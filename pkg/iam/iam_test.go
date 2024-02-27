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

	const controlPlanePolicyTemplate = `{
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
	  `

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
