package controllers_test

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsclientupstream "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/giantswarm/capa-iam-operator/controllers"
	"github.com/giantswarm/capa-iam-operator/pkg/test/mocks"
)

var (
	ctx           context.Context
	mockCtrl      *gomock.Controller
	mockIAMClient *mocks.MockIAMAPI
	reconcileErr  error
	reconciler    *controllers.AWSMachineTemplateReconciler
)

var _ = Describe("AWSMachineTemplateReconciler", func() {
	var req ctrl.Request

	BeforeEach(func() {
		logger := zap.New(zap.WriteTo(GinkgoWriter))
		ctx = log.IntoContext(context.Background(), logger)

		mockCtrl = gomock.NewController(GinkgoT())

		ctx := context.TODO()

		mockIAMClient = mocks.NewMockIAMAPI(mockCtrl)

		reconciler = &controllers.AWSMachineTemplateReconciler{
			Client:            k8sClient,
			EnableKiamRole:    true,
			EnableRoute53Role: true,
			Log:               ctrl.Log,

			IAMClientAndRegionFactory: func(session awsclientupstream.ConfigProvider) (iamiface.IAMAPI, string) {
				return mockIAMClient, "fakeregion"
			},
		}

		err := k8sClient.Create(ctx, &capa.AWSMachineTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"cluster.x-k8s.io/cluster-name": "test-cluster",
					"cluster.x-k8s.io/role":         "control-plane",
					"cluster.x-k8s.io/watch-filter": "capi",
				},
				Name:      "my-awsmt",
				Namespace: namespace,
			},
			Spec: capa.AWSMachineTemplateSpec{
				Template: capa.AWSMachineTemplateResource{
					Spec: capa.AWSMachineSpec{
						IAMInstanceProfile: "the-profile",
						InstanceType:       "unittest.4xlarge",
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Create(ctx, &capa.AWSCluster{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"cluster.x-k8s.io/cluster-name": "test-cluster",
				},
				Name:      "my-awsc",
				Namespace: namespace,
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

		err = k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-irsa-cloudfront",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"domain": []byte("foobar.cloudfront.net"),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(namespace).NotTo(BeEmpty())
		req = ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      "my-awsmt",
				Namespace: namespace,
			},
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	type RoleInfo struct {
		ExpectedName                     string
		ExpectedAssumeRolePolicyDocument string
		ExpectedPolicyName               string
		ExpectedPolicyDocument           string
		ReturnRoleArn                    string
	}
	expectedRoleStatusesOnSuccess := []RoleInfo{
		// Control plane node
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

			ExpectedPolicyName: "control-plane-test-cluster-policy",
			ExpectedPolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
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
        "ec2:DescribeVolumesModifications",
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

		// KIAM
		{
			ExpectedName: "test-cluster-IAMManager-Role",

			ExpectedAssumeRolePolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::12345678:role/the-profile"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
`,

			ExpectedPolicyName: "control-plane-test-cluster-policy",
			ExpectedPolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": {
    "Action": "sts:AssumeRole",
    "Resource": "*",
    "Effect": "Allow"
  }
}
`,

			ReturnRoleArn: "arn:aws:iam::999666333:role/test-cluster-IAMManager-Role",
		},

		// external-dns (called "Route53" in our code)
		{
			ExpectedName: "test-cluster-Route53Manager-Role",

			ExpectedAssumeRolePolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::999666333:role/test-cluster-IAMManager-Role"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
`,

			ExpectedPolicyName: "control-plane-test-cluster-policy",
			ExpectedPolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "route53:ChangeResourceRecordSets",
      "Resource": [
        "arn:aws:route53:::hostedzone/*"
      ],
      "Effect": "Allow"
    },
    {
      "Action": [
        "route53:ListHostedZones",
        "route53:ListResourceRecordSets"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
`,

			ReturnRoleArn: "arn:aws:iam::55554444:role/test-cluster-Route53Manager-Role",
		},

		// cert-manager
		{
			ExpectedName: "test-cluster-CertManager-Role",

			ExpectedAssumeRolePolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::999666333:role/test-cluster-IAMManager-Role"
      },
      "Action": "sts:AssumeRole"
    },
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam:::oidc-provider/foobar.cloudfront.net"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "foobar.cloudfront.net:sub": "system:serviceaccount:kube-system:cert-manager-controller"
        }
      }
    }
  ]
}
`,

			ExpectedPolicyName: "control-plane-test-cluster-policy",
			ExpectedPolicyDocument: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "route53:GetChange",
      "Resource": "arn:aws:route53:::change/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "route53:ChangeResourceRecordSets",
        "route53:ListResourceRecordSets"
      ],
      "Resource": "arn:aws:route53:::hostedzone/*"
    },
    {
      "Effect": "Allow",
      "Action": "route53:ListHostedZonesByName",
      "Resource": "*"
    }
  ]
}
`,

			ReturnRoleArn: "arn:aws:iam::121245456767:role/test-cluster-CertManager-Role",
		},
	}

	expectedIAMTags := []*iam.Tag{
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
			for _, info := range expectedRoleStatusesOnSuccess {
				mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
					RoleName: aws.String(info.ExpectedName),
				}).Return(nil, awserr.New(iam.ErrCodeNoSuchEntityException, "unit test", nil))
			}
		})

		It("creates the role", func() {
			for _, info := range expectedRoleStatusesOnSuccess {
				mockIAMClient.EXPECT().CreateRole(&iam.CreateRoleInput{
					AssumeRolePolicyDocument: aws.String(info.ExpectedAssumeRolePolicyDocument),
					RoleName:                 aws.String(info.ExpectedName),
					Tags:                     expectedIAMTags,
				}).Return(&iam.CreateRoleOutput{}, nil)

				mockIAMClient.EXPECT().CreateInstanceProfile(&iam.CreateInstanceProfileInput{
					InstanceProfileName: aws.String(info.ExpectedName),
					Tags:                expectedIAMTags,
				}).Return(&iam.CreateInstanceProfileOutput{}, nil)

				mockIAMClient.EXPECT().AddRoleToInstanceProfile(&iam.AddRoleToInstanceProfileInput{
					InstanceProfileName: aws.String(info.ExpectedName),
					RoleName:            aws.String(info.ExpectedName),
				}).Return(&iam.AddRoleToInstanceProfileOutput{}, nil)

				// Implementation detail: instead of storing the ARN, the controller calls `GetRole` multiple times
				// from different places. Remove once we don't do this anymore (hence the `MinTimes` call so we
				// would notice).
				mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
					RoleName: aws.String(info.ExpectedName),
				}).MinTimes(1).Return(&iam.GetRoleOutput{
					Role: &iam.Role{
						Arn:  aws.String(info.ReturnRoleArn),
						Tags: expectedIAMTags,
					},
				}, nil)

				mockIAMClient.EXPECT().ListRolePolicies(&iam.ListRolePoliciesInput{
					RoleName: aws.String(info.ExpectedName),
				}).Return(&iam.ListRolePoliciesOutput{}, nil)

				mockIAMClient.EXPECT().PutRolePolicy(&iam.PutRolePolicyInput{
					PolicyName:     aws.String(info.ExpectedPolicyName),
					PolicyDocument: aws.String(info.ExpectedPolicyDocument),
					RoleName:       aws.String(info.ExpectedName),
				}).Return(&iam.PutRolePolicyOutput{}, nil)
			}

			_, reconcileErr = reconciler.Reconcile(ctx, req)
			Expect(reconcileErr).To(BeNil())
		})
	})
})
