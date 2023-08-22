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
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/giantswarm/capa-iam-operator/controllers"
	"github.com/giantswarm/capa-iam-operator/pkg/test/mocks"
)

var _ = Describe("AWSClusterReconciler", func() {
	var (
		ctx           context.Context
		mockCtrl      *gomock.Controller
		mockAwsClient *mocks.MockAwsClientInterface
		mockIAMClient *mocks.MockIAMAPI
		reconcileErr  error
		reconciler    *controllers.AWSClusterReconciler
		req           ctrl.Request
		namespace     string
		sess          *session.Session
	)

	SetupNamespaceBeforeAfterEach(&namespace)

	BeforeEach(func() {
		logger := zap.New(zap.WriteTo(GinkgoWriter))
		ctx = log.IntoContext(context.Background(), logger)

		mockCtrl = gomock.NewController(GinkgoT())

		ctx := context.TODO()

		mockAwsClient = mocks.NewMockAwsClientInterface(mockCtrl)
		mockIAMClient = mocks.NewMockIAMAPI(mockCtrl)

		reconciler = &controllers.AWSClusterReconciler{
			Client:    k8sClient,
			Log:       ctrl.Log,
			AWSClient: mockAwsClient,
			IAMClientFactory: func(session awsclientupstream.ConfigProvider) iamiface.IAMAPI {
				return mockIAMClient
			},
		}

		err := k8sClient.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster-cluster-values",
				Namespace: namespace,
			},
			Data: map[string]string{
				"values": "baseDomain: test.gaws.gigantic.io\n",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		_ = k8sClient.Create(ctx, &capa.AWSClusterRoleIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-2",
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
					"cluster.x-k8s.io/cluster-name": "test-cluster",
				},
				Name:      "test-cluster",
				Namespace: namespace,
			},
			Spec: capa.AWSClusterSpec{
				IdentityRef: &capa.AWSIdentityReference{
					Name: "test-2",
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
			Spec: capi.ClusterSpec{
				ControlPlaneEndpoint: capi.APIEndpoint{
					Host: "api.testcluster-apiserver-123456789.eu-west-2.elb.amazonaws.com",
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(namespace).NotTo(BeEmpty())
		req = ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      "test-cluster",
				Namespace: namespace,
			},
		}

		sess, err = session.NewSession(&aws.Config{
			Region: aws.String("eu-west-1")},
		)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	// TODO We create multiple equal policies (`control-plane-test-cluster-policy`
	//      vs. `irsa-role-test-cluster-policy`. Until we fix that, the test checks
	//      the current, wrong behavior :(
	externalDnsRoleInfoCopy := externalDnsRoleInfo
	Expect(externalDnsRoleInfoCopy.ExpectedPolicyName).To(Equal("control-plane-test-cluster-policy"))
	externalDnsRoleInfoCopy.ExpectedPolicyName = irsaRoleName
	certManagerRoleInfoCopy := certManagerRoleInfo
	Expect(certManagerRoleInfoCopy.ExpectedPolicyName).To(Equal("control-plane-test-cluster-policy"))
	certManagerRoleInfoCopy.ExpectedPolicyName = irsaRoleName
	ALBControllerRoleInfoCopy := ALBControllerRoleInfo
	Expect(ALBControllerRoleInfoCopy.ExpectedPolicyName).To(Equal("control-plane-test-cluster-policy"))
	ALBControllerRoleInfoCopy.ExpectedPolicyName = irsaRoleName

	expectedRoleStatusesOnSuccess := []RoleInfo{
		certManagerRoleInfoCopy,
		externalDnsRoleInfoCopy,
		ALBControllerRoleInfoCopy,
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

	When("KIAM role was already created by other controller", func() {
		BeforeEach(func() {
			mockAwsClient.EXPECT().GetAWSClientSession("arn:aws:iam::012345678901:role/giantswarm-test-capa-controller", "eu-west-1").Return(sess, nil)
			// Implementation detail: KIAM role gets looked up for each role, therefore `MinTimes(1)`
			mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
				RoleName: aws.String("test-cluster-IAMManager-Role"),
			}).MinTimes(1).Return(&iam.GetRoleOutput{
				Role: &iam.Role{
					Arn:  aws.String("arn:aws:iam::999666333:role/test-cluster-IAMManager-Role"),
					Tags: expectedIAMTags,
				},
			}, nil)
		})

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

					// TODO This is different from `AWSMachineTemplateReconciler`. We should update the policy
					//      document if and only if it differs from the desired one.
					mockIAMClient.EXPECT().UpdateAssumeRolePolicy(&iam.UpdateAssumeRolePolicyInput{
						PolicyDocument: aws.String(info.ExpectedAssumeRolePolicyDocument),
						RoleName:       aws.String(info.ExpectedName),
					}).Return(&iam.UpdateAssumeRolePolicyOutput{}, nil)

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
})
