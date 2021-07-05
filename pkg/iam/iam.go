package iam

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awsclient "github.com/aws/aws-sdk-go/aws/client"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	"github.com/go-logr/logr"
)

const (
	ControlPlaneRole = "control-plane"
	NodesRole        = "nodes"

	IAMControllerOwnedTag = "capi-iam-controller/owned"
	ClusterIDTag          = "sigs.k8s.io/cluster-api-provider-aws/cluster/%s"
)

type IAMServiceConfig struct {
	AWSSession  awsclient.ConfigProvider
	ClusterName string
	IAMRoleName string
	Log         logr.Logger
	RoleType    string
}

type IAMService struct {
	clusterName string
	iamClient   *awsiam.IAM
	iamRoleName string
	log         logr.Logger
	region      string
	roleType    string
}

func New(config IAMServiceConfig) (*IAMService, error) {
	if config.AWSSession == nil {
		return nil, errors.New("cannot create IAMService with AWSSession equal to nil")
	}
	if config.ClusterName == "" {
		return nil, errors.New("cannot create IAMService with empty ClusterName")
	}
	if config.IAMRoleName == "" {
		return nil, errors.New("cannot create IAMService with empty IAMRoleName")
	}
	if config.Log == nil {
		return nil, errors.New("cannot create IAMService with Log equal to nil")
	}
	if !(config.RoleType == ControlPlaneRole || config.RoleType == NodesRole) {
		return nil, fmt.Errorf("cannot create IAMService with invalid RoleType '%s'", config.RoleType)
	}
	client := awsiam.New(config.AWSSession)

	l := config.Log.WithValues("clusterName", config.ClusterName, "iam-role", config.RoleType)

	s := &IAMService{
		clusterName: config.ClusterName,
		iamClient:   client,
		iamRoleName: config.IAMRoleName,
		log:         l,
		roleType:    config.RoleType,
		region:      client.SigningRegion,
	}

	return s, nil
}

func (s *IAMService) Reconcile() error {
	s.log.Info("reconciling IAM role")

	err := s.createMainRole()
	if err != nil {
		return err
	}

	// we only attach the inline policy to a role that is owned (and was created) by iam controller
	owned, err := isOwnedByIAMController(s.iamRoleName, s.iamClient)
	if err != nil {
		s.log.Error(err, "Failed to fetch IAM Role")
		return err
	}
	// check if the policy is created by this controller, if its not than we skip adding inline policy
	if !owned {
		s.log.Info("IAM role is not owned by IAM controller, skipping adding inline policy")
	} else {
		err = s.attachInlinePolicy()
		if err != nil {
			return err
		}
	}

	s.log.Info("finished reconciling IAM role")
	return nil
}

// createMainRole will create the main IAM role that will be attached to EC2 instances
func (s *IAMService) createMainRole() error {
	i := &awsiam.GetRoleInput{
		RoleName: aws.String(s.iamRoleName),
	}

	_, err := s.iamClient.GetRole(i)

	// create new IAMRole if it does not exists yet
	if IsNotFound(err) {
		assumeRolePolicyDocument, err := generateAssumeRolePolicyDocument(s.region)
		if err != nil {
			s.log.Error(err, "failed to generate assume policy document from template for IAM role")
			return err
		}

		tags := []*awsiam.Tag{
			{
				Key:   aws.String(IAMControllerOwnedTag),
				Value: aws.String(""),
			},
			{
				Key:   aws.String(fmt.Sprintf(ClusterIDTag, s.clusterName)),
				Value: aws.String("owned"),
			},
		}

		i := &awsiam.CreateRoleInput{
			RoleName:                 aws.String(s.iamRoleName),
			AssumeRolePolicyDocument: aws.String(assumeRolePolicyDocument),
			Tags:                     tags,
		}

		_, err = s.iamClient.CreateRole(i)
		if err != nil {
			s.log.Error(err, "failed to create main IAM Role")
			return err
		}

		s.log.Info("successfully created a new main IAM role")
	} else if err != nil {
		s.log.Error(err, "Failed to fetch IAM Role")
		return err
	} else {
		s.log.Info("IAM Role already exists, skipping creation")
	}

	return nil
}

// attachInlinePolicy  will attach inline policy to the main IAM role
func (s *IAMService) attachInlinePolicy() error {
	i := &awsiam.ListRolePoliciesInput{
		RoleName: aws.String(s.iamRoleName),
	}

	alreadyExists := false

	// check if the inline policy already exists
	o, err := s.iamClient.ListRolePolicies(i)
	if err == nil {
		for _, p := range o.PolicyNames {
			if *p == policyName(s.roleType, s.clusterName) {
				alreadyExists = true
				break
			}
		}
	}

	// add inline policy to the main IAM role if it do not exist yet
	if !alreadyExists {
		policyDocument, err := generatePolicyDocument(s.clusterName, s.roleType, s.region)
		if err != nil {
			s.log.Error(err, "failed to generate inline policy document from template for IAM role")
			return err
		}

		i3 := &awsiam.PutRolePolicyInput{
			PolicyName:     aws.String(policyName(s.roleType, s.clusterName)),
			PolicyDocument: aws.String(policyDocument),
			RoleName:       aws.String(s.iamRoleName),
		}

		_, err = s.iamClient.PutRolePolicy(i3)
		if err != nil {
			s.log.Error(err, "failed to add inline policy to IAM Role")
			return err
		}
		s.log.Info("successfully added inline policy to IAM role")
	} else {
		s.log.Info("inline policy for IAM role already added, skipping")
	}

	return nil
}

func (s *IAMService) Delete() error {
	s.log.Info("deleting IAM resources")

	owned, err := isOwnedByIAMController(s.iamRoleName, s.iamClient)
	if IsNotFound(err) {
		// role do not exists, nothing to delete, lets just finish
		return nil
	} else if err != nil {
		s.log.Error(err, "Failed to fetch IAM Role")
		return err
	}
	// check if the policy is created by this controller, if its not than we skip deletion
	if !owned {
		s.log.Info("IAM role is not owned by IAM controller, skipping deletion")
		return nil
	}

	// clean any attached policies, otherwise deletion of role will not work
	err = s.cleanAttachedPolicies()
	if err != nil {
		return err
	}

	// delete the role
	i := &awsiam.DeleteRoleInput{
		RoleName: aws.String(s.iamRoleName),
	}

	_, err = s.iamClient.DeleteRole(i)
	if err != nil {
		s.log.Error(err, "failed to delete role")
		return err
	}

	s.log.Info("finished deleting IAM resources")
	return nil
}

func (s *IAMService) cleanAttachedPolicies() error {
	s.log.Info("finding all policies")

	// clean attached policies
	{
		i := &awsiam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(s.iamRoleName),
		}

		o, err := s.iamClient.ListAttachedRolePolicies(i)
		if err != nil {
			s.log.Error(err, "failed to list attached policies")
			return err
		} else {
			for _, p := range o.AttachedPolicies {
				s.log.Info(fmt.Sprintf("detaching policy %s", *p.PolicyName))

				i := &awsiam.DetachRolePolicyInput{
					PolicyArn: p.PolicyArn,
					RoleName:  aws.String(s.iamRoleName),
				}

				_, err := s.iamClient.DetachRolePolicy(i)
				if err != nil {
					s.log.Error(err, fmt.Sprintf("failed to detach policy %s", *p.PolicyName))
					return err
				}

				s.log.Info(fmt.Sprintf("detached policy %s", *p.PolicyName))
			}
		}
	}

	// clean inline policies
	{
		i := &awsiam.ListRolePoliciesInput{
			RoleName: aws.String(s.iamRoleName),
		}

		o, err := s.iamClient.ListRolePolicies(i)
		if err != nil {
			s.log.Error(err, "failed to list inline policies")
			return err
		}

		for _, p := range o.PolicyNames {
			s.log.Info(fmt.Sprintf("deleting inline policy %s", *p))

			i := &awsiam.DeleteRolePolicyInput{
				RoleName:   aws.String(s.iamRoleName),
				PolicyName: p,
			}

			_, err := s.iamClient.DeleteRolePolicy(i)
			if err != nil {
				s.log.Error(err, fmt.Sprintf("failed to delete inline policy %s", *p))
				return err
			}
			s.log.Info(fmt.Sprintf("deleted inline policy %s", *p))
		}
	}

	s.log.Info("cleaned attached and inline policies from IAM Role")
	return nil
}

func isOwnedByIAMController(iamRoleName string, iamClient *awsiam.IAM) (bool, error) {
	i := &awsiam.GetRoleInput{
		RoleName: aws.String(iamRoleName),
	}

	o, err := iamClient.GetRole(i)
	if err != nil {
		return false, err
	}

	if hasIAMControllerTag(o.Role.Tags) {
		return true, nil
	} else {
		return false, nil
	}
}

func hasIAMControllerTag(tags []*awsiam.Tag) bool {
	for _, tag := range tags {
		if *tag.Key == IAMControllerOwnedTag {
			return true
		}
	}

	return false
}

func policyName(role string, clusterID string) string {
	return fmt.Sprintf("%s-%s-policy", role, clusterID)
}
