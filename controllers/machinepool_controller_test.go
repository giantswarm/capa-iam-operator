package controllers_test

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsclientupstream "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
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
	"github.com/giantswarm/capa-iam-operator/pkg/test/mocks"
)

var _ = Describe("MachinePoolReconciler", func() {
	var (
		ctx           context.Context
		mockCtrl      *gomock.Controller
		mockAwsClient *mocks.MockAwsClientInterface
		mockIAMClient *mocks.MockIAMAPI
		reconcileErr  error
		reconciler    *controllers.MachinePoolReconciler
		req           ctrl.Request
		namespace     string
		sess          *session.Session
	)

	SetupNamespaceBeforeAfterEach(&namespace)

	BeforeEach(func() {
		logger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
		ctx = log.IntoContext(context.Background(), logger)

		mockCtrl = gomock.NewController(GinkgoT())

		ctx := context.TODO()

		mockAwsClient = mocks.NewMockAwsClientInterface(mockCtrl)
		mockIAMClient = mocks.NewMockIAMAPI(mockCtrl)

		reconciler = &controllers.MachinePoolReconciler{
			Client:    k8sClient,
			AWSClient: mockAwsClient,
			IAMClientFactory: func(session awsclientupstream.ConfigProvider, region string) iamiface.IAMAPI {
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

		sess, err = session.NewSession(&aws.Config{
			Region: aws.String("eu-west-1"),
		},
		)
		Expect(err).NotTo(HaveOccurred())
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
      "Action": [
        "ecr:BatchCheckLayerAvailability",
        "ecr:BatchGetImage",
        "ecr:DescribeRepositories",
        "ecr:GetAuthorizationToken",
        "ecr:GetDownloadUrlForLayer",
        "ecr:GetRepositoryPolicy",
        "ecr:ListImages"
      ],
      "Resource": "*",
      "Effect": "Allow"
    }
  ]
}
`,

			ReturnRoleArn: "arn:aws:iam::12345678:role/the-profile",
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
			mockAwsClient.EXPECT().GetAWSClientSession("arn:aws:iam::012345678901:role/giantswarm-test-capa-controller", "eu-west-1").Return(sess, nil)
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

				mockIAMClient.EXPECT().GetRolePolicy(
					&iam.GetRolePolicyInput{
						PolicyName: aws.String(info.ExpectedPolicyName),
						RoleName:   aws.String(info.ExpectedName),
					},
				).Return(&iam.GetRolePolicyOutput{}, awserr.New(iam.ErrCodeNoSuchEntityException, "unit test", nil))

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

	When("a role already exists", func() {
		BeforeEach(func() {
			for _, info := range expectedRoleStatusesOnSuccess {
				mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
					RoleName: aws.String(info.ExpectedName),
				}).MinTimes(1).Return(&iam.GetRoleOutput{
					Role: &iam.Role{
						Arn:  aws.String(info.ReturnRoleArn),
						Tags: expectedIAMTags,
					},
				}, nil)
			}
		})

		It("works on the existing role", func() {
			Skip("TODO The controller is not idempotent to this extent, but should be. Once this is implemented, we should also add test cases for failures in each AWS SDK call")

			for _, info := range expectedRoleStatusesOnSuccess {
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
