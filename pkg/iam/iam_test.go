package iam_test

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsclientgo "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	awsIAM "github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/giantswarm/capa-iam-operator/pkg/iam"
	"github.com/giantswarm/capa-iam-operator/pkg/test/mocks"
)

var _ = Describe("ReconcileRole", func() {

	var (
		mockCtrl      *gomock.Controller
		mockIAMClient *mocks.MockIAMAPI
		iamService    *iam.IAMService
		err           error
		sess          awsclientgo.ConfigProvider
	)

	BeforeEach(func() {
		//create me new iam service config with mocks
		sess, err = session.NewSession(&aws.Config{
			Region: aws.String("eu-west-1")},
		)
		Expect(err).NotTo(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())
		mockIAMClient = mocks.NewMockIAMAPI(mockCtrl)

		iamConfig := iam.IAMServiceConfig{
			ClusterName:      "test-cluster",
			MainRoleName:     "test-role",
			Region:           "test-region",
			RoleType:         "control-plane",
			PrincipalRoleARN: "test-principal-role-arn",
			Log:              ctrl.Log,
			AWSSession:       sess,
			IAMClientFactory: func(session awsclientgo.ConfigProvider, region string) iamiface.IAMAPI {
				return mockIAMClient
			},
		}
		iamService, err = iam.New(iamConfig)
		Expect(err).To(BeNil())
	})

	When("role is present", func() {
		BeforeEach(func() {
			mockIAMClient.EXPECT().GetRole(gomock.Any()).Return(&awsIAM.GetRoleOutput{Role: &awsIAM.Role{
				Tags: []*awsIAM.Tag{{Key: aws.String("capi-iam-controller/owned"), Value: aws.String("test-cluster")}},
			}}, nil).AnyTimes()
		})
		When("inline policy is already attached", func() {
			BeforeEach(func() {
				mockIAMClient.EXPECT().ListRolePolicies(gomock.Any()).Return(&awsIAM.ListRolePoliciesOutput{PolicyNames: aws.StringSlice([]string{"control-plane-test-cluster-policy"})}, nil)
			})
			It("should return nil", func() {
				err := iamService.ReconcileRole()
				Expect(err).To(BeNil())
			})
		})
		When("could not attach InlinePolicy", func() {
			JustBeforeEach(func() {
				mockIAMClient.EXPECT().ListRolePolicies(gomock.Any()).Return(&awsIAM.ListRolePoliciesOutput{}, nil).AnyTimes()
				mockIAMClient.EXPECT().PutRolePolicy(gomock.Any()).Return(&awsIAM.PutRolePolicyOutput{}, errors.New("test error")).AnyTimes()
			})
			It("should return error", func() {
				err := iamService.ReconcileRole()
				Expect(err).NotTo(BeNil())
			})
		})
	})

	When("role is not present", func() {
		BeforeEach(func() {
			mockIAMClient.EXPECT().GetRole(gomock.Any()).Return(&awsIAM.GetRoleOutput{}, awserr.New(awsIAM.ErrCodeNoSuchEntityException, "test", nil)).Times(1)
			mockIAMClient.EXPECT().CreateRole(gomock.Any()).Return(&awsIAM.CreateRoleOutput{}, nil)
			mockIAMClient.EXPECT().CreateInstanceProfile(gomock.Any()).Return(&awsIAM.CreateInstanceProfileOutput{}, nil)
			mockIAMClient.EXPECT().AddRoleToInstanceProfile(gomock.Any()).Return(&awsIAM.AddRoleToInstanceProfileOutput{}, nil)
			mockIAMClient.EXPECT().GetRole(gomock.Any()).Return(&awsIAM.GetRoleOutput{Role: &awsIAM.Role{
				Tags: []*awsIAM.Tag{{Key: aws.String("capi-iam-controller/owned"), Value: aws.String("test-cluster")}},
			}}, nil).AnyTimes()
			mockIAMClient.EXPECT().ListRolePolicies(gomock.Any()).Return(&awsIAM.ListRolePoliciesOutput{}, nil).AnyTimes()
			mockIAMClient.EXPECT().PutRolePolicy(gomock.Any()).Return(&awsIAM.PutRolePolicyOutput{}, nil).AnyTimes()
		})
		It("should create the role", func() {
			err := iamService.ReconcileRole()
			Expect(err).To(BeNil())
		})
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})
})
