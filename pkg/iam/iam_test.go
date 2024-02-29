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

	const controlPlanePolicyTemplate = "%7B%0A%09%09%22Version%22%3A%20%222012-10-17%22%2C%0A%09%09%22Statement%22%3A%20%5B%0A%09%09%20%20%7B%0A%09%09%09%22Action%22%3A%20%22elasticloadbalancing%3A%2A%22%2C%0A%09%09%09%22Resource%22%3A%20%22%2A%22%2C%0A%09%09%09%22Effect%22%3A%20%22Allow%22%0A%09%09%20%20%7D%2C%0A%09%09%20%20%7B%0A%09%09%09%22Action%22%3A%20%5B%0A%09%09%09%20%20%22autoscaling%3ADescribeAutoScalingGroups%22%2C%0A%09%09%09%20%20%22autoscaling%3ADescribeAutoScalingInstances%22%2C%0A%09%09%09%20%20%22autoscaling%3ADescribeTags%22%2C%0A%09%09%09%20%20%22autoscaling%3ADescribeLaunchConfigurations%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeLaunchTemplateVersions%22%0A%09%09%09%5D%2C%0A%09%09%09%22Resource%22%3A%20%22%2A%22%2C%0A%09%09%09%22Effect%22%3A%20%22Allow%22%0A%09%09%20%20%7D%2C%0A%09%09%20%20%7B%0A%09%09%09%22Condition%22%3A%20%7B%0A%09%09%09%20%20%22StringEquals%22%3A%20%7B%0A%09%09%09%09%22autoscaling%3AResourceTag%2Fsigs.k8s.io%2Fcluster-api-provider-aws%2Fcluster%2Ftest-cluster%22%3A%20%22owned%22%0A%09%09%09%20%20%7D%0A%09%09%09%7D%2C%0A%09%09%09%22Action%22%3A%20%5B%0A%09%09%09%20%20%22autoscaling%3ASetDesiredCapacity%22%2C%0A%09%09%09%20%20%22autoscaling%3ATerminateInstanceInAutoScalingGroup%22%0A%09%09%09%5D%2C%0A%09%09%09%22Resource%22%3A%20%22%2A%22%2C%0A%09%09%09%22Effect%22%3A%20%22Allow%22%0A%09%09%20%20%7D%2C%0A%09%09%20%20%7B%0A%09%09%09%22Action%22%3A%20%5B%0A%09%09%09%20%20%22ecr%3AGetAuthorizationToken%22%2C%0A%09%09%09%20%20%22ecr%3ABatchCheckLayerAvailability%22%2C%0A%09%09%09%20%20%22ecr%3AGetDownloadUrlForLayer%22%2C%0A%09%09%09%20%20%22ecr%3AGetRepositoryPolicy%22%2C%0A%09%09%09%20%20%22ecr%3ADescribeRepositories%22%2C%0A%09%09%09%20%20%22ecr%3AListImages%22%2C%0A%09%09%09%20%20%22ecr%3ABatchGetImage%22%0A%09%09%09%5D%2C%0A%09%09%09%22Resource%22%3A%20%22%2A%22%2C%0A%09%09%09%22Effect%22%3A%20%22Allow%22%0A%09%09%20%20%7D%2C%0A%09%09%20%20%7B%0A%09%09%09%22Action%22%3A%20%5B%0A%09%09%09%20%20%22ec2%3AAssignPrivateIpAddresses%22%2C%0A%09%09%09%20%20%22ec2%3AAttachNetworkInterface%22%2C%0A%09%09%09%20%20%22ec2%3ACreateNetworkInterface%22%2C%0A%09%09%09%20%20%22ec2%3ADeleteNetworkInterface%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeInstances%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeInstanceTypes%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeTags%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeNetworkInterfaces%22%2C%0A%09%09%09%20%20%22ec2%3ADetachNetworkInterface%22%2C%0A%09%09%09%20%20%22ec2%3AModifyNetworkInterfaceAttribute%22%2C%0A%09%09%09%20%20%22ec2%3AUnassignPrivateIpAddresses%22%0A%09%09%09%5D%2C%0A%09%09%09%22Resource%22%3A%20%22%2A%22%2C%0A%09%09%09%22Effect%22%3A%20%22Allow%22%0A%09%09%20%20%7D%2C%0A%09%09%20%20%7B%0A%09%09%09%22Action%22%3A%20%5B%0A%09%09%09%20%20%22autoscaling%3ADescribeAutoScalingGroups%22%2C%0A%09%09%09%20%20%22autoscaling%3ADescribeLaunchConfigurations%22%2C%0A%09%09%09%20%20%22autoscaling%3ADescribeTags%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeInstances%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeImages%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeRegions%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeRouteTables%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeSecurityGroups%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeSubnets%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeVolumes%22%2C%0A%09%09%09%20%20%22ec2%3ACreateSecurityGroup%22%2C%0A%09%09%09%20%20%22ec2%3ACreateTags%22%2C%0A%09%09%09%20%20%22ec2%3ACreateVolume%22%2C%0A%09%09%09%20%20%22ec2%3AModifyInstanceAttribute%22%2C%0A%09%09%09%20%20%22ec2%3AModifyVolume%22%2C%0A%09%09%09%20%20%22ec2%3AAttachVolume%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeVolumesModifications%22%2C%0A%09%09%09%20%20%22ec2%3AAuthorizeSecurityGroupIngress%22%2C%0A%09%09%09%20%20%22ec2%3ACreateRoute%22%2C%0A%09%09%09%20%20%22ec2%3ADeleteRoute%22%2C%0A%09%09%09%20%20%22ec2%3ADeleteSecurityGroup%22%2C%0A%09%09%09%20%20%22ec2%3ADeleteVolume%22%2C%0A%09%09%09%20%20%22ec2%3ADetachVolume%22%2C%0A%09%09%09%20%20%22ec2%3ARevokeSecurityGroupIngress%22%2C%0A%09%09%09%20%20%22ec2%3ADescribeVpcs%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3AAddTags%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3AAttachLoadBalancerToSubnets%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3AApplySecurityGroupsToLoadBalancer%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ACreateLoadBalancer%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ACreateLoadBalancerPolicy%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ACreateLoadBalancerListeners%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3AConfigureHealthCheck%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADeleteLoadBalancer%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADeleteLoadBalancerListeners%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADescribeLoadBalancers%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADescribeLoadBalancerAttributes%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADetachLoadBalancerFromSubnets%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADeregisterInstancesFromLoadBalancer%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3AModifyLoadBalancerAttributes%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ARegisterInstancesWithLoadBalancer%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ASetLoadBalancerPoliciesForBackendServer%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3AAddTags%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ACreateListener%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ACreateTargetGroup%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADeleteListener%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADeleteTargetGroup%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADescribeListeners%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADescribeLoadBalancerPolicies%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADescribeTargetGroups%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ADescribeTargetHealth%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3AModifyListener%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3AModifyTargetGroup%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ARegisterTargets%22%2C%0A%09%09%09%20%20%22elasticloadbalancing%3ASetLoadBalancerPoliciesOfListener%22%2C%0A%09%09%09%20%20%22iam%3ACreateServiceLinkedRole%22%2C%0A%09%09%09%20%20%22kms%3ADescribeKey%22%0A%09%09%09%5D%2C%0A%09%09%09%22Resource%22%3A%20%5B%0A%09%09%09%20%20%22%2A%22%0A%09%09%09%5D%2C%0A%09%09%09%22Effect%22%3A%20%22Allow%22%0A%09%09%20%20%7D%2C%0A%09%09%20%20%7B%0A%09%09%09%22Action%22%3A%20%5B%0A%09%09%09%20%20%22secretsmanager%3AGetSecretValue%22%2C%0A%09%09%09%20%20%22secretsmanager%3ADeleteSecret%22%0A%09%09%09%5D%2C%0A%09%09%09%22Resource%22%3A%20%22arn%3A%2A%3Asecretsmanager%3A%2A%3A%2A%3Asecret%3Aaws.cluster.x-k8s.io%2F%2A%22%2C%0A%09%09%09%22Effect%22%3A%20%22Allow%22%0A%09%09%20%20%7D%0A%09%09%5D%0A%09%20%20%7D"

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
			IAMClientFactory: func(session awsclientgo.ConfigProvider) iamiface.IAMAPI {
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
				mockIAMClient.EXPECT().GetRolePolicy(gomock.Any()).Return(&awsIAM.GetRolePolicyOutput{
					PolicyDocument: aws.String(controlPlanePolicyTemplate),
					PolicyName:     aws.String("control-plane-test-cluster-policy"),
					RoleName:       aws.String("test-role"),
				}, nil).AnyTimes()
			})
			It("should return nil", func() {
				err := iamService.ReconcileRole()
				Expect(err).To(BeNil())
			})
		})
		When("could not attach InlinePolicy", func() {
			JustBeforeEach(func() {
				mockIAMClient.EXPECT().GetRolePolicy(gomock.Any()).Return(&awsIAM.GetRolePolicyOutput{}, awserr.New(awsIAM.ErrCodeNoSuchEntityException, "test", nil)).AnyTimes()
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
			mockIAMClient.EXPECT().GetRolePolicy(gomock.Any()).Return(&awsIAM.GetRolePolicyOutput{}, awserr.New(awsIAM.ErrCodeNoSuchEntityException, "test", nil)).AnyTimes()
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
