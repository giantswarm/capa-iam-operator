package iam

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	awsclientgo "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	awsiam "github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
)

const (
	BastionRole      = "bastion"
	ControlPlaneRole = "control-plane" // also used as part of finalizer name
	NodesRole        = "nodes"         // also used as part of finalizer name
	Route53Role      = "route53-role"
	KIAMRole         = "kiam-role"
	IRSARole         = "irsa-role"
	CertManagerRole  = "cert-manager-role"
	ALBConrollerRole = "ALBController-Role"

	IAMControllerOwnedTag = "capi-iam-controller/owned"
	ClusterIDTag          = "sigs.k8s.io/cluster-api-provider-aws/cluster/%s"
)

type IAMServiceConfig struct {
	AWSSession       awsclientgo.ConfigProvider
	ClusterName      string
	MainRoleName     string
	Log              logr.Logger
	RoleType         string
	Region           string
	PrincipalRoleARN string

	IAMClientFactory func(awsclientgo.ConfigProvider) iamiface.IAMAPI
}

type IAMService struct {
	clusterName      string
	iamClient        iamiface.IAMAPI
	eksClient        eksiface.EKSAPI
	mainRoleName     string
	log              logr.Logger
	region           string
	roleType         string
	principalRoleARN string
}

type Route53RoleParams struct {
	EC2ServiceDomain string
	AccountID        string
	CloudFrontDomain string
	Namespace        string
	ServiceAccount   string
	PrincipalRoleARN string
}

func New(config IAMServiceConfig) (*IAMService, error) {
	if config.AWSSession == nil {
		return nil, errors.New("cannot create IAMService with AWSSession equal to nil")
	}
	if config.IAMClientFactory == nil {
		return nil, errors.New("cannot create IAMService with IAMClientFactory equal to nil")
	}
	if config.ClusterName == "" {
		return nil, errors.New("cannot create IAMService with empty ClusterName")
	}
	if config.MainRoleName == "" {
		return nil, errors.New("cannot create IAMService with empty MainRoleName")
	}
	if !(config.RoleType == ControlPlaneRole || config.RoleType == NodesRole || config.RoleType == BastionRole || config.RoleType == IRSARole) {
		return nil, fmt.Errorf("cannot create IAMService with invalid RoleType '%s'", config.RoleType)
	}
	iamClient := config.IAMClientFactory(config.AWSSession)
	eksClient := eks.New(config.AWSSession, &aws.Config{Region: aws.String(config.Region)})

	l := config.Log.WithValues("clusterName", config.ClusterName, "iam-role", config.RoleType)
	s := &IAMService{
		clusterName:      config.ClusterName,
		iamClient:        iamClient,
		eksClient:        eksClient,
		mainRoleName:     config.MainRoleName,
		log:              l,
		roleType:         config.RoleType,
		region:           config.Region,
		principalRoleARN: config.PrincipalRoleARN,
	}

	return s, nil
}

func (s *IAMService) ReconcileRole() error {
	s.log.Info("reconciling IAM role")

	params := struct {
		ClusterName      string
		EC2ServiceDomain string
	}{
		ClusterName:      s.clusterName,
		EC2ServiceDomain: ec2ServiceDomain(s.region),
	}
	err := s.reconcileRole(s.mainRoleName, s.roleType, params)
	if err != nil {
		return err
	}

	s.log.Info("finished reconciling IAM role")
	return nil
}

func (s *IAMService) ReconcileKiamRole() error {
	s.log.Info("reconciling KIAM IAM role")

	var controlPlaneRoleARN string
	{

		i := &awsiam.GetRoleInput{
			RoleName: aws.String(s.mainRoleName),
		}

		o, err := s.iamClient.GetRole(i)
		if err != nil {
			s.log.Error(err, "failed to fetch ControlPlane role")
			return err
		}

		controlPlaneRoleARN = *o.Role.Arn
	}

	params := struct {
		ControlPlaneRoleARN string
		EC2ServiceDomain    string
	}{
		ControlPlaneRoleARN: controlPlaneRoleARN,
		EC2ServiceDomain:    ec2ServiceDomain(s.region),
	}

	err := s.reconcileRole(roleName(KIAMRole, s.clusterName), KIAMRole, params)
	if err != nil {
		return err
	}

	s.log.Info("finished reconciling KIAM IAM role")
	return nil
}

func (s *IAMService) ReconcileRolesForIRSA(awsAccountID string, cloudFrontDomain string) error {
	s.log.Info("reconciling IAM roles for IRSA")

	for _, roleTypeToReconcile := range []string{Route53Role, CertManagerRole, ALBConrollerRole} {
		var params Route53RoleParams
		params, err := s.generateRoute53RoleParams(roleTypeToReconcile, awsAccountID, cloudFrontDomain)
		if err != nil {
			s.log.Error(err, "failed to generate Route53 role parameters")
			return err
		}

		err = s.reconcileRole(roleName(roleTypeToReconcile, s.clusterName), roleTypeToReconcile, params)
		if err != nil {
			return err
		}
	}

	s.log.Info("finished reconciling IAM roles for IRSA")
	return nil
}

func (s *IAMService) generateRoute53RoleParams(roleTypeToReconcile string, awsAccountID string, cloudFrontDomain string) (Route53RoleParams, error) {
	namespace := "kube-system"
	serviceAccount, err := getServiceAccount(roleTypeToReconcile)

	if err != nil {
		s.log.Error(err, "failed to get service account for role")
		return Route53RoleParams{}, err
	}

	params := Route53RoleParams{
		EC2ServiceDomain: ec2ServiceDomain(s.region),
		AccountID:        awsAccountID,
		CloudFrontDomain: cloudFrontDomain,
		Namespace:        namespace,
		ServiceAccount:   serviceAccount,
	}

	return params, nil
}

func (s *IAMService) reconcileRole(roleName string, roleType string, params interface{}) error {
	l := s.log.WithValues("role_name", roleName, "role_type", roleType)
	err := s.createRole(roleName, roleType, params)
	if err != nil {
		return err
	}

	if s.roleType == IRSARole || s.roleType == CertManagerRole {
		if err = s.applyAssumePolicyRole(roleName, roleType, params); err != nil {
			l.Error(err, "Failed to apply assume role policy to role")
			return err
		}
	}

	// we only attach the inline policy to a role that is owned (and was created) by iam controller
	owned, err := isOwnedByIAMController(roleName, s.iamClient)
	if err != nil {
		l.Error(err, "Failed to fetch IAM Role")
		return err
	}
	// check if the policy is created by this controller, if its not than we skip adding inline policy
	if !owned {
		l.Info("IAM role is not owned by IAM controller, skipping adding inline policy", "role_name", roleName)
	} else {
		err = s.attachInlinePolicy(roleName, roleType, params)
		if err != nil {
			return err
		}
	}
	return nil
}

// createRole will create requested IAM role
func (s *IAMService) createRole(roleName string, roleType string, params interface{}) error {
	l := s.log.WithValues("role_name", roleName, "role_type", roleType)

	_, err := s.iamClient.GetRole(&awsiam.GetRoleInput{
		RoleName: aws.String(roleName),
	})

	// create new IAMRole if it does not exist yet
	if err == nil {
		l.Info("IAM Role already exists, skipping creation")
		return nil
	}
	if !IsNotFound(err) {
		l.Error(err, "Failed to fetch IAM Role")
		return err
	}

	tmpl := getTrustPolicyTemplate(roleType)

	assumeRolePolicyDocument, err := generatePolicyDocument(tmpl, params)
	if err != nil {
		l.Error(err, "failed to generate assume policy document from template for IAM role")
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

	_, err = s.iamClient.CreateRole(&awsiam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicyDocument),
		Tags:                     tags,
	})
	if err != nil {
		l.Error(err, "failed to create IAM Role")
		return err
	}

	i2 := &awsiam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
		Tags:                tags,
	}

	_, err = s.iamClient.CreateInstanceProfile(i2)
	if IsAlreadyExists(err) {
		// fall thru
	} else if err != nil {
		l.Error(err, "failed to create instance profile")
		return err
	}

	i3 := &awsiam.AddRoleToInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
		RoleName:            aws.String(roleName),
	}

	_, err = s.iamClient.AddRoleToInstanceProfile(i3)
	if IsAlreadyExists(err) {
		// fall thru
	} else if err != nil {
		l.Error(err, "failed to add role to instance profile")
		return err
	}

	l.Info("successfully created a new IAM role")

	return nil
}

func (s *IAMService) applyAssumePolicyRole(roleName string, roleType string, params interface{}) error {
	log := s.log.WithValues("role_name", roleName)
	i := &awsiam.GetRoleInput{
		RoleName: aws.String(roleName),
	}

	_, err := s.iamClient.GetRole(i)

	if IsNotFound(err) {
		log.Info("role doesn't exist. Skipping application of assume policy")
		return nil
	}

	if !IsNotFound(err) && err != nil {
		log.Error(err, "failed to fetch IAM role")
		return err
	}

	log.Info("applying assume policy role to role")

	tmpl := getTrustPolicyTemplate(roleType)
	assumeRolePolicyDocument, err := generatePolicyDocument(tmpl, params)
	if err != nil {
		log.Error(err, "failed to generate assume policy document from template for IAM role")
		return err
	}

	updateInput := &awsiam.UpdateAssumeRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyDocument: aws.String(assumeRolePolicyDocument),
	}

	_, err = s.iamClient.UpdateAssumeRolePolicy(updateInput)

	return err
}

// attachInlinePolicy  will attach inline policy to the main IAM role
func (s *IAMService) attachInlinePolicy(roleName string, roleType string, params interface{}) error {
	l := s.log.WithValues("role_name", roleName)
	i := &awsiam.ListRolePoliciesInput{
		RoleName: aws.String(roleName),
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
		tmpl := getInlinePolicyTemplate(roleType)

		policyDocument, err := generatePolicyDocument(tmpl, params)
		if err != nil {
			l.Error(err, "failed to generate inline policy document from template for IAM role")
			return err
		}

		i := &awsiam.PutRolePolicyInput{
			PolicyName:     aws.String(policyName(s.roleType, s.clusterName)),
			PolicyDocument: aws.String(policyDocument),
			RoleName:       aws.String(roleName),
		}

		_, err = s.iamClient.PutRolePolicy(i)
		if err != nil {
			l.Error(err, "failed to add inline policy to IAM Role")
			return err
		}
		l.Info("successfully added inline policy to IAM role")
	} else {
		l.Info("inline policy for IAM role already added, skipping")
	}

	return nil
}

func (s *IAMService) DeleteRole() error {
	s.log.Info("deleting IAM resources")

	// delete main role
	err := s.deleteRole(s.mainRoleName)
	if err != nil {
		return err
	}

	s.log.Info("finished deleting IAM resources")
	return nil
}

func (s *IAMService) DeleteKiamRole() error {
	s.log.Info("deleting KIAM IAM resources")

	// delete kiam role
	err := s.deleteRole(roleName(KIAMRole, s.clusterName))
	if err != nil {
		return err
	}

	s.log.Info("finished deleting KIAM IAM resources")
	return nil
}

func (s *IAMService) DeleteRoute53Role() error {
	s.log.Info("deleting Route53 IAM resources")

	// delete route3 role
	err := s.deleteRole(roleName(Route53Role, s.clusterName))
	if err != nil {
		return err
	}

	s.log.Info("finished deleting Route53 IAM resources")
	return nil
}

func (s *IAMService) DeleteRolesForIRSA() error {
	s.log.Info("deleting IAM roles for IRSA")

	// delete cert-manager role
	err := s.deleteRole(roleName(CertManagerRole, s.clusterName))
	if err != nil {
		return err
	}

	// delete route53 role
	err = s.deleteRole(roleName(Route53Role, s.clusterName))
	if err != nil {
		return err
	}

	// delete AWS Load Balancer Controller role
	err = s.deleteRole(roleName(ALBConrollerRole, s.clusterName))
	if err != nil {
		return err
	}

	s.log.Info("finished deleting IAM roles for IRSA")
	return nil
}

func (s *IAMService) deleteRole(roleName string) error {
	l := s.log.WithValues("role_name", roleName)

	owned, err := isOwnedByIAMController(roleName, s.iamClient)
	if IsNotFound(err) {
		// role do not exists, nothing to delete, lets just finish
		return nil
	} else if err != nil {
		l.Error(err, "Failed to fetch IAM Role")
		return err
	}
	// check if the policy is created by this controller, if its not than we skip deletion
	if !owned {
		l.Info("IAM role is not owned by IAM controller, skipping deletion")
		return nil
	}

	// clean any attached policies, otherwise deletion of role will not work
	err = s.cleanAttachedPolicies(roleName)
	if err != nil {
		return err
	}

	i := &awsiam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
		RoleName:            aws.String(roleName),
	}

	_, err = s.iamClient.RemoveRoleFromInstanceProfile(i)
	if err != nil && !IsNotFound(err) {
		l.Error(err, "failed to remove role from instance profile")
		return err
	}

	i2 := &awsiam.DeleteInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
	}

	_, err = s.iamClient.DeleteInstanceProfile(i2)
	if err != nil && !IsNotFound(err) {
		l.Error(err, "failed to delete instance profile")
		return err
	}

	// delete the role
	i3 := &awsiam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	}

	_, err = s.iamClient.DeleteRole(i3)
	if err != nil {
		l.Error(err, "failed to delete role")
		return err
	}

	return nil
}

func (s *IAMService) cleanAttachedPolicies(roleName string) error {
	l := s.log.WithValues("role_name", roleName)
	// clean attached policies
	{
		i := &awsiam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(roleName),
		}

		o, err := s.iamClient.ListAttachedRolePolicies(i)
		if err != nil {
			l.Error(err, "failed to list attached policies")
			return err
		} else {
			for _, p := range o.AttachedPolicies {
				l.Info(fmt.Sprintf("detaching policy %s", *p.PolicyName))

				i := &awsiam.DetachRolePolicyInput{
					PolicyArn: p.PolicyArn,
					RoleName:  aws.String(roleName),
				}

				_, err := s.iamClient.DetachRolePolicy(i)
				if err != nil {
					l.Error(err, fmt.Sprintf("failed to detach policy %s", *p.PolicyName))
					return err
				}

				l.Info(fmt.Sprintf("detached policy %s", *p.PolicyName))
			}
		}
	}

	// clean inline policies
	{
		i := &awsiam.ListRolePoliciesInput{
			RoleName: aws.String(roleName),
		}

		o, err := s.iamClient.ListRolePolicies(i)
		if err != nil {
			l.Error(err, "failed to list inline policies")
			return err
		}

		for _, p := range o.PolicyNames {
			l.Info(fmt.Sprintf("deleting inline policy %s", *p))

			i := &awsiam.DeleteRolePolicyInput{
				RoleName:   aws.String(roleName),
				PolicyName: p,
			}

			_, err := s.iamClient.DeleteRolePolicy(i)
			if err != nil {
				l.Error(err, fmt.Sprintf("failed to delete inline policy %s", *p))
				return err
			}
			l.Info(fmt.Sprintf("deleted inline policy %s", *p))
		}
	}

	l.Info("cleaned attached and inline policies from IAM Role")
	return nil
}

func (s *IAMService) GetRoleARN(roleName string) (string, error) {
	o, err := s.iamClient.GetRole(&awsiam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return "", microerror.Mask(err)
	}

	return *o.Role.Arn, nil
}

func (s *IAMService) SetPrincipalRoleARN(arn string) {
	s.principalRoleARN = arn
}

func (s *IAMService) GetIRSAOpenIDForEKS(clusterName string) (string, error) {
	i := &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}
	cluster, err := s.eksClient.DescribeCluster(i)
	if err != nil {
		return "", microerror.Mask(err)
	}
	if cluster == nil || cluster.Cluster == nil || cluster.Cluster.Identity == nil || cluster.Cluster.Identity.Oidc == nil || cluster.Cluster.Identity.Oidc.Issuer == nil {
		return "", microerror.Maskf(invalidClusterError, "cluster %s does not have OIDC identity", clusterName)
	}

	id := strings.TrimPrefix(*cluster.Cluster.Identity.Oidc.Issuer, "https://")

	return id, nil
}

func isOwnedByIAMController(iamRoleName string, iamClient iamiface.IAMAPI) (bool, error) {
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

func roleName(role string, clusterID string) string {
	if role == Route53Role {
		return fmt.Sprintf("%s-Route53Manager-Role", clusterID)
	} else if role == KIAMRole {
		return fmt.Sprintf("%s-IAMManager-Role", clusterID)
	} else if role == CertManagerRole {
		return fmt.Sprintf("%s-CertManager-Role", clusterID)
	} else {
		return fmt.Sprintf("%s-%s", clusterID, role)
	}
}

func policyName(role string, clusterID string) string {
	return fmt.Sprintf("%s-%s-policy", role, clusterID)
}

func getServiceAccount(role string) (string, error) {
	if role == CertManagerRole {
		return "cert-manager-controller", nil
	} else if role == IRSARole {
		return "external-dns", nil
	} else if role == Route53Role {
		return "external-dns", nil
	} else if role == ALBConrollerRole {
		return "aws-load-balancer-controller", nil
	}

	return "", fmt.Errorf("Cannot get service account for specified role - %s", role)
}
