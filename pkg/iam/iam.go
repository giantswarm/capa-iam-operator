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
)

type IAMServiceConfig struct {
	AWSSession  awsclient.ConfigProvider
	ClusterID   string
	IAMRoleName string
	Log         logr.Logger
	RoleType    string
}

type IAMService struct {
	clusterID   string
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
	if config.ClusterID == "" {
		return nil, errors.New("cannot create IAMService with empty ClusterID")
	}
	if config.IAMRoleName == "" {
		return nil, errors.New("cannot create IAMService with empty IAMRoleName")
	}
	if config.Log == nil {
		return nil, errors.New("cannot create IAMService with Log equal to nil")
	}
	if !(config.RoleType == ControlPlaneRole || config.RoleType == NodesRole) {
		return nil, errors.New(fmt.Sprintf("cannot create IAMService with invalid RoleType '%s'", config.RoleType))
	}
	client := awsiam.New(config.AWSSession)

	s := &IAMService{
		clusterID:   config.ClusterID,
		iamClient:   client,
		iamRoleName: config.IAMRoleName,
		log:         config.Log,
		roleType:    config.RoleType,
		region:      client.SigningRegion,
	}

	return s, nil
}

func (s *IAMService) Reconcile() error {
	i := &awsiam.GetRoleInput{
		RoleName: aws.String(s.iamRoleName),
	}

	_, err := s.iamClient.GetRole(i)

	// create new IAMRole if it does not exists yet
	if IsNotFound(err) {
		err := s.create()
		if err != nil {
			s.log.Error(err, "Failed to create IAMRole")
			return err
		}

	} else if err != nil {
		s.log.Error(err, "Failed to fetch IAMRole")
		return err
	} else {
		s.log.Info("IAM Role already exists, skipping creation")
	}

	return nil

}

func (s *IAMService) create() error {
	policyDocument, err := s.generatePolicyDocument()
	if err != nil {
		s.log.Error(err, fmt.Sprintf("failed to generate policy document from template %s", s.roleType))
	}

	tags := []*awsiam.Tag{
		{
			Key:   aws.String(IAMControllerOwnedTag),
			Value: aws.String(""),
		},
	}

	i := &awsiam.CreateRoleInput{
		RoleName:                 aws.String(s.iamRoleName),
		AssumeRolePolicyDocument: aws.String(policyDocument),
		Tags:                     tags,
	}

	_, err = s.iamClient.CreateRole(i)
	if err != nil {
		s.log.Error(err, "failed to create IAMRole")
	}
	return nil
}

func (s *IAMService) Delete() error {
	i := &awsiam.GetRoleInput{
		RoleName: aws.String(s.iamRoleName),
	}

	o, err := s.iamClient.GetRole(i)

	if IsNotFound(err) {
		// role do not exists, nothing to delete, lets just finish
		return nil
	} else if err != nil {
		s.log.Error(err, "Failed to fetch IAMRole")
		return err
	}
	// check if the policy is created by this controller, if its not than we skip deletion
	if !isOwnedByIAMController(o.Role.Tags) {
		s.log.Info("IAM role is not owned by IAM controller, skipping deletion")
		return nil
	}

	// clean any attached policies, otherwise deletion of role will not work
	err = s.cleanAttachedPolicies()
	if err != nil {
		return err
	}

	// delete the role
	i2 := &awsiam.DeleteRoleInput{
		RoleName: aws.String(s.iamRoleName),
	}

	_, err = s.iamClient.DeleteRole(i2)
	if err != nil {
		s.log.Error(err, fmt.Sprintf("Failed to delete role %s", s.iamRoleName))
		return err
	}
	return nil
}

func (s *IAMService) cleanAttachedPolicies() error {
	var policies []string
	{
		s.log.Info("finding all policies")

		i := &awsiam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(s.iamRoleName),
		}

		o, err := s.iamClient.ListAttachedRolePolicies(i)
		if IsNotFound(err) {
			s.log.Info("No attached policies")
		} else if err != nil {
			s.log.Error(err, "failed to list attached policies")
			return err
		} else {
			for _, p := range o.AttachedPolicies {
				policies = append(policies, *p.PolicyArn)
			}
			s.log.Info(fmt.Sprintf("Found %d attached policies", len(policies)))

			for _, p := range policies {
				s.log.Info(fmt.Sprintf("detaching policy %s", p))

				i := &awsiam.DetachRolePolicyInput{
					PolicyArn: aws.String(p),
					RoleName:  aws.String(s.iamRoleName),
				}

				_, err := s.iamClient.DetachRolePolicy(i)
				if err != nil {
					s.log.Error(err, fmt.Sprintf("failed to detach policy %s", p))
					return err
				}

				s.log.Info(fmt.Sprintf("detached policy %s", p))
			}
		}
	}
	return nil
}

func isOwnedByIAMController(tags []*awsiam.Tag) bool {
	for _, tag := range tags {
		if *tag.Key == IAMControllerOwnedTag {
			return true
		}
	}

	return false
}
