package controllers_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsclientupstream "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/giantswarm/capa-iam-operator/controllers"
	"github.com/giantswarm/capa-iam-operator/pkg/test/fakes/awssdkfakes"
)

func TestAWSMachineTemplateReconciler(t *testing.T) {
	g := NewWithT(t)

	ctrl.SetLogger(klog.Background())
	scheme := runtime.NewScheme()
	utilruntime.Must(capi.AddToScheme(scheme))
	utilruntime.Must(capa.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	ctx := context.TODO()

	mockIAMClient := new(awssdkfakes.FakeIAMAPI)

	reconciler := controllers.AWSMachineTemplateReconciler{
		Client:            fakeClient,
		EnableKiamRole:    true,
		EnableRoute53Role: true,
		Log:               ctrl.Log,

		IAMClientAndRegionFactory: func(session awsclientupstream.ConfigProvider) (iamiface.IAMAPI, string) {
			return mockIAMClient, "fakeregion"
		},
	}

	err := fakeClient.Create(ctx, &capa.AWSMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": "test-cluster",
				"cluster.x-k8s.io/role":         "control-plane",
				"cluster.x-k8s.io/watch-filter": "capi",
			},
			Name:      "my-awsmt",
			Namespace: "my-ns",
		},
		Spec: capa.AWSMachineTemplateSpec{
			Template: capa.AWSMachineTemplateResource{
				Spec: capa.AWSMachineSpec{
					IAMInstanceProfile: "the-profile",
				},
			},
		},
	})
	g.Expect(err).To(BeNil())

	err = fakeClient.Create(ctx, &capa.AWSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": "test-cluster",
			},
			Name:      "my-awsc",
			Namespace: "my-ns",
		},
	})
	g.Expect(err).To(BeNil())

	err = fakeClient.Create(ctx, &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "my-ns",
		},
	})
	g.Expect(err).To(BeNil())

	// expectedIAMTags := []*iam.Tag{
	// 	{
	// 		Key:   aws.String("capi-iam-controller/owned"),
	// 		Value: aws.String(""),
	// 	},
	// 	{
	// 		Key:   aws.String("sigs.k8s.io/cluster-api-provider-aws/cluster/test-cluster"),
	// 		Value: aws.String("owned"),
	// 	},
	// }

	mockIAMClient.GetRoleReturnsOnCall(0, nil, awserr.New(iam.ErrCodeNoSuchEntityException, "unit test", nil))
	defer func() {
		input := mockIAMClient.GetRoleArgsForCall(0)
		g.Expect(input.RoleName).To(Equal(aws.String("the-profile")))
	}()

	// 	mockIAMClient.EXPECT().CreateRole(&iam.CreateRoleInput{
	// 		AssumeRolePolicyDocument: aws.String("{\n  \"Version\": \"2012-10-17\",\n  \"Statement\": [\n    {\n      \"Effect\": \"Allow\",\n      \"Principal\": {\n        \"Service\": \"ec2.amazonaws.com\"\n      },\n      \"Action\": \"sts:AssumeRole\"\n    }\n  ]\n}"),
	// 		RoleName:                 aws.String("the-profile"),
	// 		Tags:                     expectedIAMTags,
	// 	}).Return(&iam.CreateRoleOutput{}, nil)

	// 	mockIAMClient.EXPECT().CreateInstanceProfile(&iam.CreateInstanceProfileInput{
	// 		InstanceProfileName: aws.String("the-profile"),
	// 		Tags:                expectedIAMTags,
	// 	}).Return(&iam.CreateInstanceProfileOutput{}, nil)

	// 	mockIAMClient.EXPECT().AddRoleToInstanceProfile(&iam.AddRoleToInstanceProfileInput{
	// 		InstanceProfileName: aws.String("the-profile"),
	// 		RoleName:            aws.String("the-profile"),
	// 	}).Return(&iam.AddRoleToInstanceProfileOutput{}, nil)

	// 	// Implementation detail: instead of storing the ARN, the controller calls `GetRole` multiple times
	// 	// from different places
	// 	mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
	// 		RoleName: aws.String("the-profile"),
	// 	}).MinTimes(1).Return(&iam.GetRoleOutput{
	// 		Role: &iam.Role{
	// 			Arn:  aws.String("arn:aws:iam::12345678:role/the-profile"),
	// 			Tags: expectedIAMTags,
	// 		},
	// 	}, nil)

	// 	mockIAMClient.EXPECT().ListRolePolicies(&iam.ListRolePoliciesInput{
	// 		RoleName: aws.String("the-profile"),
	// 	}).Return(&iam.ListRolePoliciesOutput{}, nil)

	// 	mockIAMClient.EXPECT().PutRolePolicy(&iam.PutRolePolicyInput{
	// 		PolicyName: aws.String("control-plane-test-cluster-policy"),
	// 		PolicyDocument: aws.String(`{
	//     "Version": "2012-10-17",
	//     "Statement": [
	//         {
	//             "Action": "elasticloadbalancing:*",
	//             "Resource": "*",
	//             "Effect": "Allow"
	//         },
	//         {
	//             "Action": [
	//                 "autoscaling:DescribeAutoScalingGroups",
	//                 "autoscaling:DescribeAutoScalingInstances",
	//                 "autoscaling:DescribeTags",
	//                 "autoscaling:DescribeLaunchConfigurations",
	//                 "ec2:DescribeLaunchTemplateVersions"
	//             ],
	//             "Resource": "*",
	//             "Effect": "Allow"
	//         },
	//         {
	//             "Condition": {
	//                 "StringEquals": {
	//                     "autoscaling:ResourceTag/sigs.k8s.io/cluster-api-provider-aws/cluster/test-cluster": "owned"
	//                 }
	//             },
	//             "Action": [
	//                 "autoscaling:SetDesiredCapacity",
	//                 "autoscaling:TerminateInstanceInAutoScalingGroup"
	//             ],
	//             "Resource": "*",
	//             "Effect": "Allow"
	//         },
	//         {
	//             "Action": [
	//                 "ecr:GetAuthorizationToken",
	//                 "ecr:BatchCheckLayerAvailability",
	//                 "ecr:GetDownloadUrlForLayer",
	//                 "ecr:GetRepositoryPolicy",
	//                 "ecr:DescribeRepositories",
	//                 "ecr:ListImages",
	//                 "ecr:BatchGetImage"
	//             ],
	//             "Resource": "*",
	//             "Effect": "Allow"
	//         },
	//         {
	//             "Action": [
	//                 "ec2:AssignPrivateIpAddresses",
	//                 "ec2:AttachNetworkInterface",
	//                 "ec2:CreateNetworkInterface",
	//                 "ec2:DeleteNetworkInterface",
	//                 "ec2:DescribeInstances",
	//                 "ec2:DescribeInstanceTypes",
	//                 "ec2:DescribeTags",
	//                 "ec2:DescribeNetworkInterfaces",
	//                 "ec2:DetachNetworkInterface",
	//                 "ec2:ModifyNetworkInterfaceAttribute",
	//                 "ec2:UnassignPrivateIpAddresses"
	//             ],
	//             "Resource": "*",
	//             "Effect": "Allow"
	//         },
	//         {
	//             "Action": [
	//                 "autoscaling:DescribeAutoScalingGroups",
	//                 "autoscaling:DescribeLaunchConfigurations",
	//                 "autoscaling:DescribeTags",
	//                 "ec2:DescribeInstances",
	//                 "ec2:DescribeImages",
	//                 "ec2:DescribeRegions",
	//                 "ec2:DescribeRouteTables",
	//                 "ec2:DescribeSecurityGroups",
	//                 "ec2:DescribeSubnets",
	//                 "ec2:DescribeVolumes",
	//                 "ec2:CreateSecurityGroup",
	//                 "ec2:CreateTags",
	//                 "ec2:CreateVolume",
	//                 "ec2:ModifyInstanceAttribute",
	//                 "ec2:ModifyVolume",
	//                 "ec2:AttachVolume",
	//                 "ec2:DescribeVolumesModifications",
	//                 "ec2:AuthorizeSecurityGroupIngress",
	//                 "ec2:CreateRoute",
	//                 "ec2:DeleteRoute",
	//                 "ec2:DeleteSecurityGroup",
	//                 "ec2:DeleteVolume",
	//                 "ec2:DetachVolume",
	//                 "ec2:RevokeSecurityGroupIngress",
	//                 "ec2:DescribeVpcs",
	//                 "elasticloadbalancing:AddTags",
	//                 "elasticloadbalancing:AttachLoadBalancerToSubnets",
	//                 "elasticloadbalancing:ApplySecurityGroupsToLoadBalancer",
	//                 "elasticloadbalancing:CreateLoadBalancer",
	//                 "elasticloadbalancing:CreateLoadBalancerPolicy",
	//                 "elasticloadbalancing:CreateLoadBalancerListeners",
	//                 "elasticloadbalancing:ConfigureHealthCheck",
	//                 "elasticloadbalancing:DeleteLoadBalancer",
	//                 "elasticloadbalancing:DeleteLoadBalancerListeners",
	//                 "elasticloadbalancing:DescribeLoadBalancers",
	//                 "elasticloadbalancing:DescribeLoadBalancerAttributes",
	//                 "elasticloadbalancing:DetachLoadBalancerFromSubnets",
	//                 "elasticloadbalancing:DeregisterInstancesFromLoadBalancer",
	//                 "elasticloadbalancing:ModifyLoadBalancerAttributes",
	//                 "elasticloadbalancing:RegisterInstancesWithLoadBalancer",
	//                 "elasticloadbalancing:SetLoadBalancerPoliciesForBackendServer",
	//                 "elasticloadbalancing:AddTags",
	//                 "elasticloadbalancing:CreateListener",
	//                 "elasticloadbalancing:CreateTargetGroup",
	//                 "elasticloadbalancing:DeleteListener",
	//                 "elasticloadbalancing:DeleteTargetGroup",
	//                 "elasticloadbalancing:DescribeListeners",
	//                 "elasticloadbalancing:DescribeLoadBalancerPolicies",
	//                 "elasticloadbalancing:DescribeTargetGroups",
	//                 "elasticloadbalancing:DescribeTargetHealth",
	//                 "elasticloadbalancing:ModifyListener",
	//                 "elasticloadbalancing:ModifyTargetGroup",
	//                 "elasticloadbalancing:RegisterTargets",
	//                 "elasticloadbalancing:SetLoadBalancerPoliciesOfListener",
	//                 "iam:CreateServiceLinkedRole",
	//                 "kms:DescribeKey"
	//             ],
	//             "Resource": [
	//                 "*"
	//             ],
	//             "Effect": "Allow"
	//         },
	//         {
	//             "Action": [
	//                 "secretsmanager:GetSecretValue",
	//                 "secretsmanager:DeleteSecret"
	//             ],
	//             "Resource": "arn:*:secretsmanager:*:*:secret:aws.cluster.x-k8s.io/*",
	//             "Effect": "Allow"
	//         }
	//     ]
	// }
	// `),
	// 		RoleName: aws.String("the-profile"),
	// 	}).Return(&iam.PutRolePolicyOutput{}, nil)

	// 	// KIAM

	// 	mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
	// 		RoleName: aws.String("test-cluster-IAMManager-Role"),
	// 	}).Return(nil, awserr.New(iam.ErrCodeNoSuchEntityException, "unit test", nil))

	// 	mockIAMClient.EXPECT().CreateRole(&iam.CreateRoleInput{
	// 		AssumeRolePolicyDocument: aws.String("{\n  \"Version\": \"2012-10-17\",\n  \"Statement\": [\n    {\n      \"Effect\": \"Allow\",\n      \"Principal\": {\n        \"AWS\": \"arn:aws:iam::12345678:role/the-profile\"\n      },\n      \"Action\": \"sts:AssumeRole\"\n    }\n  ]\n}"),
	// 		RoleName:                 aws.String("test-cluster-IAMManager-Role"),
	// 		Tags:                     expectedIAMTags,
	// 	}).Return(&iam.CreateRoleOutput{}, nil)

	// 	mockIAMClient.EXPECT().CreateInstanceProfile(&iam.CreateInstanceProfileInput{
	// 		InstanceProfileName: aws.String("test-cluster-IAMManager-Role"),
	// 		Tags:                expectedIAMTags,
	// 	}).Return(&iam.CreateInstanceProfileOutput{}, nil)

	// 	mockIAMClient.EXPECT().AddRoleToInstanceProfile(&iam.AddRoleToInstanceProfileInput{
	// 		InstanceProfileName: aws.String("test-cluster-IAMManager-Role"),
	// 		RoleName:            aws.String("test-cluster-IAMManager-Role"),
	// 	}).Return(&iam.AddRoleToInstanceProfileOutput{}, nil)

	// 	mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
	// 		RoleName: aws.String("test-cluster-IAMManager-Role"),
	// 	}).MinTimes(1).Return(&iam.GetRoleOutput{
	// 		Role: &iam.Role{
	// 			Arn:  aws.String("arn:aws:iam::999666333:role/test-cluster-IAMManager-Role"),
	// 			Tags: expectedIAMTags,
	// 		},
	// 	}, nil)

	// 	mockIAMClient.EXPECT().ListRolePolicies(&iam.ListRolePoliciesInput{
	// 		RoleName: aws.String("test-cluster-IAMManager-Role"),
	// 	}).Return(&iam.ListRolePoliciesOutput{}, nil)

	// 	mockIAMClient.EXPECT().PutRolePolicy(&iam.PutRolePolicyInput{
	// 		PolicyName: aws.String("control-plane-test-cluster-policy"),
	// 		PolicyDocument: aws.String(`{
	//     "Version": "2012-10-17",
	//     "Statement": {
	//         "Action": "sts:AssumeRole",
	//         "Resource": "*",
	//         "Effect": "Allow"
	//     }
	// }`),
	// 		RoleName: aws.String("test-cluster-IAMManager-Role"),
	// 	}).Return(&iam.PutRolePolicyOutput{}, nil)

	// 	// external-dns (called "Route53" in our code)

	// 	mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
	// 		RoleName: aws.String("test-cluster-Route53Manager-Role"),
	// 	}).Return(nil, awserr.New(iam.ErrCodeNoSuchEntityException, "unit test", nil))

	// 	mockIAMClient.EXPECT().CreateRole(&iam.CreateRoleInput{
	// 		AssumeRolePolicyDocument: aws.String("{\n  \"Version\": \"2012-10-17\",\n  \"Statement\": [\n    {\n      \"Effect\": \"Allow\",\n      \"Principal\": {\n        \"AWS\": \"arn:aws:iam::999666333:role/test-cluster-IAMManager-Role\"\n      },\n      \"Action\": \"sts:AssumeRole\"\n    }\n  ]\n}"),
	// 		RoleName:                 aws.String("test-cluster-Route53Manager-Role"),
	// 		Tags:                     expectedIAMTags,
	// 	}).Return(&iam.CreateRoleOutput{}, nil)

	// 	mockIAMClient.EXPECT().CreateInstanceProfile(&iam.CreateInstanceProfileInput{
	// 		InstanceProfileName: aws.String("test-cluster-Route53Manager-Role"),
	// 		Tags:                expectedIAMTags,
	// 	}).Return(&iam.CreateInstanceProfileOutput{}, nil)

	// 	mockIAMClient.EXPECT().AddRoleToInstanceProfile(&iam.AddRoleToInstanceProfileInput{
	// 		InstanceProfileName: aws.String("test-cluster-Route53Manager-Role"),
	// 		RoleName:            aws.String("test-cluster-Route53Manager-Role"),
	// 	}).Return(&iam.AddRoleToInstanceProfileOutput{}, nil)

	// 	mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
	// 		RoleName: aws.String("test-cluster-Route53Manager-Role"),
	// 	}).Return(&iam.GetRoleOutput{
	// 		Role: &iam.Role{
	// 			Arn:  aws.String("arn:aws:iam::55554444:role/test-cluster-Route53Manager-Role"),
	// 			Tags: expectedIAMTags,
	// 		},
	// 	}, nil)

	// 	mockIAMClient.EXPECT().ListRolePolicies(&iam.ListRolePoliciesInput{
	// 		RoleName: aws.String("test-cluster-Route53Manager-Role"),
	// 	}).Return(&iam.ListRolePoliciesOutput{}, nil)

	// 	mockIAMClient.EXPECT().PutRolePolicy(&iam.PutRolePolicyInput{
	// 		PolicyName: aws.String("control-plane-test-cluster-policy"),
	// 		PolicyDocument: aws.String(`{
	//     "Version": "2012-10-17",
	//     "Statement": [
	//         {
	//             "Action": "route53:ChangeResourceRecordSets",
	//             "Resource": [
	//                 "arn:aws:route53:::hostedzone/*"
	//             ],
	//             "Effect": "Allow"
	//         },
	//         {
	//             "Action": [
	//                 "route53:ListHostedZones",
	//                 "route53:ListResourceRecordSets"
	//             ],
	//             "Resource": "*",
	//             "Effect": "Allow"
	//         }
	//     ]
	// }
	// `),
	// 		RoleName: aws.String("test-cluster-Route53Manager-Role"),
	// 	}).Return(&iam.PutRolePolicyOutput{}, nil)

	// 	// cert-manager

	// 	mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
	// 		RoleName: aws.String("test-cluster-CertManager-Role"),
	// 	}).Return(nil, awserr.New(iam.ErrCodeNoSuchEntityException, "unit test", nil))

	// 	mockIAMClient.EXPECT().CreateRole(&iam.CreateRoleInput{
	// 		AssumeRolePolicyDocument: aws.String("{\n  \"Version\": \"2012-10-17\",\n  \"Statement\": [\n    {\n      \"Effect\": \"Allow\",\n      \"Principal\": {\n        \"AWS\": \"arn:aws:iam::999666333:role/test-cluster-IAMManager-Role\"\n      },\n      \"Action\": \"sts:AssumeRole\"\n    },\n    {\n      \"Effect\": \"Allow\",\n      \"Principal\": {\n        \"Federated\": \"arn:aws:iam:::oidc-provider/\"\n      },\n      \"Action\": \"sts:AssumeRoleWithWebIdentity\",\n      \"Condition\": {\n        \"StringEquals\": {\n          \":sub\": \"system:serviceaccount:kube-system:cert-manager-controller\"\n        }\n      }\n    }\n  ]\n}\n"),
	// 		RoleName:                 aws.String("test-cluster-CertManager-Role"),
	// 		Tags:                     expectedIAMTags,
	// 	}).Return(&iam.CreateRoleOutput{}, nil)

	// 	mockIAMClient.EXPECT().CreateInstanceProfile(&iam.CreateInstanceProfileInput{
	// 		InstanceProfileName: aws.String("test-cluster-CertManager-Role"),
	// 		Tags:                expectedIAMTags,
	// 	}).Return(&iam.CreateInstanceProfileOutput{}, nil)

	// 	mockIAMClient.EXPECT().AddRoleToInstanceProfile(&iam.AddRoleToInstanceProfileInput{
	// 		InstanceProfileName: aws.String("test-cluster-CertManager-Role"),
	// 		RoleName:            aws.String("test-cluster-CertManager-Role"),
	// 	}).Return(&iam.AddRoleToInstanceProfileOutput{}, nil)

	// 	mockIAMClient.EXPECT().GetRole(&iam.GetRoleInput{
	// 		RoleName: aws.String("test-cluster-CertManager-Role"),
	// 	}).Return(&iam.GetRoleOutput{
	// 		Role: &iam.Role{
	// 			Arn:  aws.String("arn:aws:iam::121245456767:role/test-cluster-CertManager-Role"),
	// 			Tags: expectedIAMTags,
	// 		},
	// 	}, nil)

	// 	mockIAMClient.EXPECT().ListRolePolicies(&iam.ListRolePoliciesInput{
	// 		RoleName: aws.String("test-cluster-CertManager-Role"),
	// 	}).Return(&iam.ListRolePoliciesOutput{}, nil)

	// 	mockIAMClient.EXPECT().PutRolePolicy(&iam.PutRolePolicyInput{
	// 		PolicyName: aws.String("control-plane-test-cluster-policy"),
	// 		PolicyDocument: aws.String(`{
	//     "Version": "2012-10-17",
	//     "Statement": [
	//         {
	//             "Effect": "Allow",
	//             "Action": "route53:GetChange",
	//             "Resource": "arn:aws:route53:::change/*"
	//         },
	//         {
	//             "Effect": "Allow",
	//             "Action": [
	//                 "route53:ChangeResourceRecordSets",
	//                 "route53:ListResourceRecordSets"
	//             ],
	//             "Resource": "arn:aws:route53:::hostedzone/*"
	//         },
	//         {
	//             "Effect": "Allow",
	//             "Action": "route53:ListHostedZonesByName",
	//             "Resource": "*"
	//         }
	//     ]
	// }
	// `),
	// 		RoleName: aws.String("test-cluster-CertManager-Role"),
	// 	}).Return(&iam.PutRolePolicyOutput{}, nil)

	// --------------

	_, err = reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: client.ObjectKey{
			Name:      "my-awsmt",
			Namespace: "my-ns",
		},
	})
	g.Expect(err).To(BeNil())
}
