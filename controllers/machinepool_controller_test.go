package controllers_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	awsiamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capa "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	expcapa "sigs.k8s.io/cluster-api-provider-aws/v2/exp/api/v1beta2"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	expcapi "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/giantswarm/capa-iam-operator/controllers"
	"github.com/giantswarm/capa-iam-operator/pkg/iam"
	"github.com/giantswarm/capa-iam-operator/pkg/test/mocks"
)

var _ = Describe("MachinePoolReconciler", func() {
	var (
		ctx           context.Context
		mockCtrl      *gomock.Controller
		mockAwsClient *mocks.MockAwsClientInterface
		mockIAMClient *mocks.MockIAMClient
		reconcileErr  error
		reconciler    *controllers.MachinePoolReconciler
		req           ctrl.Request
		namespace     string
		cfg           *aws.Config
	)

	SetupNamespaceBeforeAfterEach(&namespace)

	BeforeEach(func() {
		logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
		ctx = log.IntoContext(context.Background(), logger)

		mockCtrl = gomock.NewController(GinkgoT())

		ctx := context.TODO()

		mockAwsClient = mocks.NewMockAwsClientInterface(mockCtrl)
		mockIAMClient = mocks.NewMockIAMClient(mockCtrl)

		reconciler = &controllers.MachinePoolReconciler{
			Client:    k8sClient,
			AWSClient: mockAwsClient,
			IAMClientFactory: func(_ aws.Config, _ string) iam.IAMClient {
				return mockIAMClient
			},
		}

		err := k8sClient.Create(ctx, &expcapa.AWSMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"cluster.x-k8s.io/cluster-name": "test-cluster",
					"cluster.x-k8s.io/watch-filter": "capi",
				},
				Name:      "my-awsmp",
				Namespace: namespace,
			},
			Spec: expcapa.AWSMachinePoolSpec{
				AWSLaunchTemplate: expcapa.AWSLaunchTemplate{
					IamInstanceProfile: "the-profile",
				},
				MaxSize: 3,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Create(ctx, &expcapi.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"cluster.x-k8s.io/cluster-name": "test-cluster",
					"cluster.x-k8s.io/watch-filter": "capi",
				},
				Name:      "my-mp",
				Namespace: namespace,
			},
			Spec: expcapi.MachinePoolSpec{
				ClusterName: "test-cluster",
				Template: capi.MachineTemplateSpec{
					Spec: capi.MachineSpec{
						ClusterName: "test-cluster",
						InfrastructureRef: corev1.ObjectReference{
							Kind:       "AWSMachinePool",
							Namespace:  namespace,
							Name:       "my-awsmp",
							APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
						},
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		_ = k8sClient.Create(ctx, &capa.AWSClusterRoleIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-3",
			},
			Spec: capa.AWSClusterRoleIdentitySpec{
				AWSRoleSpec: capa.AWSRoleSpec{
					RoleArn: "arn:aws:iam::012345678901:role/giantswarm-test-capa-controller",
				},
				AWSClusterIdentitySpec: capa.AWSClusterIdentitySpec{
					AllowedNamespaces: &capa.AllowedNamespaces{},
				},
			},
		})
		err = k8sClient.Create(ctx, &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"cluster.x-k8s.io/cluster-name":                                "test-cluster",
					"alpha.aws.giantswarm.io/reduced-instance-permissions-workers": "true",
				},
				Name:      "my-awsc",
				Namespace: namespace,
			},
			Spec: capa.AWSClusterSpec{
				IdentityRef: &capa.AWSIdentityReference{
					Name: "test-3",
					Kind: "AWSClusterRoleIdentity",
				},
				Region: "eu-west-1",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Create(ctx, &capi.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: namespace,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-cluster-values",
				Namespace: namespace,
			},
			Data: map[string]string{
				"values": "baseDomain: test.gaws.gigantic.io\n",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(namespace).NotTo(BeEmpty())
		req = ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      "my-mp",
				Namespace: namespace,
			},
		}

		cfg = aws.NewConfig()
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	expectedRoleStatusesOnSuccess := []RoleInfo{
		// Worker node
		{
			ExpectedName: "the-profile",

			ExpectedAssumeRolePolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
`,

			ExpectedPolicyName: "nodes-test-cluster-policy",
			ExpectedPolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "ec2:*",
      "Resource": "*",
      "Effect": "Allow"
    },
    {
      "Action": "elasticloadbalancing:*",
      "Resource": "*",
      "Effect": "Allow"
    },
    {
      "Action": [
        "autoscaling:DescribeAutoScalingGroups",
        "autoscaling:DescribeAutoScalingInstances",
        "autoscaling:DescribeTags",
        "autoscaling:DescribeLaunchConfigurations",
        "ec2:DescribeLaunchTemplateVersions"
      ],
      "Resource": "*",
      "Effect": "Allow"
    },
    {
      "Condition": {
        "StringEquals": {
          "autoscaling:ResourceTag/sigs.k8s.io/cluster-api-provider-aws/cluster/test-cluster": "owned"
        }
      },
      "Action": [
        "autoscaling:SetDesiredCapacity",
        "autoscaling:TerminateInstanceInAutoScalingGroup"
      ],
      "Resource": "*",
      "Effect": "Allow"
    },
    {
      "Action": [
        "ec2:DescribeInstances",
        "ec2:DescribeRegions",
        "ecr:GetAuthorizationToken",
        "ecr:BatchCheckLayerAvailability",
        "ecr:GetDownloadUrlForLayer",
        "ecr:GetRepositoryPolicy",
        "ecr:DescribeRepositories",
        "ecr:ListImages",
        "ecr:BatchGetImage"
      ],
      "Resource": "*",
      "Effect": "Allow"
    },
    {
      "Action": [
        "ec2:AssignPrivateIpAddresses",
        "ec2:AttachNetworkInterface",
        "ec2:CreateNetworkInterface",
        "ec2:DeleteNetworkInterface",
        "ec2:DescribeInstances",
        "ec2:DescribeInstanceTypes",
        "ec2:DescribeTags",
        "ec2:DescribeNetworkInterfaces",
        "ec2:DetachNetworkInterface",
        "ec2:ModifyNetworkInterfaceAttribute",
        "ec2:UnassignPrivateIpAddresses"
      ],
      "Resource": "*",
      "Effect": "Allow"
    },
    {
      "Action": [
        "autoscaling:DescribeAutoScalingGroups",
        "autoscaling:DescribeLaunchConfigurations",
        "autoscaling:DescribeTags",
        "ec2:DescribeInstances",
        "ec2:DescribeImages",
        "ec2:DescribeRegions",
        "ec2:DescribeRouteTables",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeSubnets",
        "ec2:DescribeVolumes",
        "ec2:CreateSecurityGroup",
        "ec2:CreateTags",
        "ec2:CreateVolume",
        "ec2:ModifyInstanceAttribute",
        "ec2:ModifyVolume",
        "ec2:AttachVolume",
        "ec2:AuthorizeSecurityGroupIngress",
        "ec2:CreateRoute",
        "ec2:DeleteRoute",
        "ec2:DeleteSecurityGroup",
        "ec2:DeleteVolume",
        "ec2:DetachVolume",
        "ec2:RevokeSecurityGroupIngress",
        "ec2:DescribeVpcs",
        "elasticloadbalancing:AddTags",
        "elasticloadbalancing:AttachLoadBalancerToSubnets",
        "elasticloadbalancing:ApplySecurityGroupsToLoadBalancer",
        "elasticloadbalancing:CreateLoadBalancer",
        "elasticloadbalancing:CreateLoadBalancerPolicy",
        "elasticloadbalancing:CreateLoadBalancerListeners",
        "elasticloadbalancing:ConfigureHealthCheck",
        "elasticloadbalancing:DeleteLoadBalancer",
        "elasticloadbalancing:DeleteLoadBalancerListeners",
        "elasticloadbalancing:DescribeLoadBalancers",
        "elasticloadbalancing:DescribeLoadBalancerAttributes",
        "elasticloadbalancing:DetachLoadBalancerFromSubnets",
        "elasticloadbalancing:DeregisterInstancesFromLoadBalancer",
        "elasticloadbalancing:ModifyLoadBalancerAttributes",
        "elasticloadbalancing:RegisterInstancesWithLoadBalancer",
        "elasticloadbalancing:SetLoadBalancerPoliciesForBackendServer",
        "elasticloadbalancing:AddTags",
        "elasticloadbalancing:CreateListener",
        "elasticloadbalancing:CreateTargetGroup",
        "elasticloadbalancing:DeleteListener",
        "elasticloadbalancing:DeleteTargetGroup",
        "elasticloadbalancing:DescribeListeners",
        "elasticloadbalancing:DescribeLoadBalancerPolicies",
        "elasticloadbalancing:DescribeTargetGroups",
        "elasticloadbalancing:DescribeTargetHealth",
        "elasticloadbalancing:ModifyListener",
        "elasticloadbalancing:ModifyTargetGroup",
        "elasticloadbalancing:RegisterTargets",
        "elasticloadbalancing:SetLoadBalancerPoliciesOfListener",
        "iam:CreateServiceLinkedRole",
        "kms:DescribeKey"
      ],
      "Resource": [
        "*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:DeleteSecret"
      ],
      "Resource": "arn:*:secretsmanager:*:*:secret:aws.cluster.x-k8s.io/*",
      "Effect": "Allow"
    }
  ]
}
`,

			ReturnRoleArn: "arn:aws:iam::12345678:role/the-profile",
		},
	}

	expectedIAMTags := []awsiamtypes.Tag{
		{
			Key:   aws.String("capi-iam-controller/owned"),
			Value: aws.String(""),
		},
		{
			Key:   aws.String("sigs.k8s.io/cluster-api-provider-aws/cluster/test-cluster"),
			Value: aws.String("owned"),
		},
	}

	When("a role does not exist", func() {
		BeforeEach(func() {
			mockAwsClient.EXPECT().GetAWSClientConfig("arn:aws:iam::012345678901:role/giantswarm-test-capa-controller", "eu-west-1").Return(*cfg, nil)
			for _, info := range expectedRoleStatusesOnSuccess {
				mockIAMClient.EXPECT().GetRole(context.TODO(), &awsiam.GetRoleInput{
					RoleName: aws.String(info.ExpectedName),
				}).Return(nil, &awsiamtypes.NoSuchEntityException{})
			}
		})

		It("creates the role", func() {
			for _, info := range expectedRoleStatusesOnSuccess {
				mockIAMClient.EXPECT().CreateRole(context.TODO(), &awsiam.CreateRoleInput{
					AssumeRolePolicyDocument: aws.String(info.ExpectedAssumeRolePolicyDocument),
					RoleName:                 aws.String(info.ExpectedName),
					Tags:                     expectedIAMTags,
				}).Return(&awsiam.CreateRoleOutput{}, nil)

				mockIAMClient.EXPECT().CreateInstanceProfile(context.TODO(), &awsiam.CreateInstanceProfileInput{
					InstanceProfileName: aws.String(info.ExpectedName),
					Tags:                expectedIAMTags,
				}).Return(&awsiam.CreateInstanceProfileOutput{}, nil)

				mockIAMClient.EXPECT().AddRoleToInstanceProfile(context.TODO(), &awsiam.AddRoleToInstanceProfileInput{
					InstanceProfileName: aws.String(info.ExpectedName),
					RoleName:            aws.String(info.ExpectedName),
				}).Return(&awsiam.AddRoleToInstanceProfileOutput{}, nil)

				mockIAMClient.EXPECT().GetRolePolicy(context.TODO(),
					&awsiam.GetRolePolicyInput{
						PolicyName: aws.String(info.ExpectedPolicyName),
						RoleName:   aws.String(info.ExpectedName),
					},
				).Return(&awsiam.GetRolePolicyOutput{}, &awsiamtypes.NoSuchEntityException{})

				mockIAMClient.EXPECT().PutRolePolicy(context.TODO(), &awsiam.PutRolePolicyInput{
					PolicyName:     aws.String(info.ExpectedPolicyName),
					PolicyDocument: aws.String(info.ExpectedPolicyDocument),
					RoleName:       aws.String(info.ExpectedName),
				}).Return(&awsiam.PutRolePolicyOutput{}, nil)
			}

			_, reconcileErr = reconciler.Reconcile(ctx, req)
			Expect(reconcileErr).To(BeNil())
		})
	})

	When("a role already exists", func() {
		BeforeEach(func() {
			for _, info := range expectedRoleStatusesOnSuccess {
				mockIAMClient.EXPECT().GetRole(context.TODO(), &awsiam.GetRoleInput{
					RoleName: aws.String(info.ExpectedName),
				}).MinTimes(1).Return(&awsiam.GetRoleOutput{
					Role: &awsiamtypes.Role{
						Arn:  aws.String(info.ReturnRoleArn),
						Tags: expectedIAMTags,
					},
				}, nil)
			}
		})

		It("works on the existing role", func() {
			Skip("TODO The controller is not idempotent to this extent, but should be. Once this is implemented, we should also add test cases for failures in each AWS SDK call")

			for _, info := range expectedRoleStatusesOnSuccess {
				mockIAMClient.EXPECT().CreateInstanceProfile(context.TODO(), &awsiam.CreateInstanceProfileInput{
					InstanceProfileName: aws.String(info.ExpectedName),
					Tags:                expectedIAMTags,
				}).Return(&awsiam.CreateInstanceProfileOutput{}, nil)

				mockIAMClient.EXPECT().AddRoleToInstanceProfile(context.TODO(), &awsiam.AddRoleToInstanceProfileInput{
					InstanceProfileName: aws.String(info.ExpectedName),
					RoleName:            aws.String(info.ExpectedName),
				}).Return(&awsiam.AddRoleToInstanceProfileOutput{}, nil)

				// Implementation detail: instead of storing the ARN, the controller calls `GetRole` multiple times
				// from different places. Remove once we don't do this anymore (hence the `MinTimes` call so we
				// would notice).
				mockIAMClient.EXPECT().GetRole(context.TODO(), &awsiam.GetRoleInput{
					RoleName: aws.String(info.ExpectedName),
				}).MinTimes(1).Return(&awsiam.GetRoleOutput{
					Role: &awsiamtypes.Role{
						Arn:  aws.String(info.ReturnRoleArn),
						Tags: expectedIAMTags,
					},
				}, nil)

				mockIAMClient.EXPECT().ListRolePolicies(context.TODO(), &awsiam.ListRolePoliciesInput{
					RoleName: aws.String(info.ExpectedName),
				}).Return(&awsiam.ListRolePoliciesOutput{}, nil)

				mockIAMClient.EXPECT().PutRolePolicy(context.TODO(), &awsiam.PutRolePolicyInput{
					PolicyName:     aws.String(info.ExpectedPolicyName),
					PolicyDocument: aws.String(info.ExpectedPolicyDocument),
					RoleName:       aws.String(info.ExpectedName),
				}).Return(&awsiam.PutRolePolicyOutput{}, nil)
			}

			_, reconcileErr = reconciler.Reconcile(ctx, req)
			Expect(reconcileErr).To(BeNil())
		})
	})
})
