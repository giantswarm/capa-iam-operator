package iam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/giantswarm/microerror"
	"github.com/go-logr/logr"
)

const (
	BastionRole           = "bastion"
	ControlPlaneRole      = "control-plane" // also used as part of finalizer name
	NodesRole             = "nodes"         // also used as part of finalizer name
	Route53Role           = "route53-role"
	KIAMRole              = "kiam-role"
	IRSARole              = "irsa-role"
	CertManagerRole       = "cert-manager-role"
	ALBConrollerRole      = "ALBController-Role"
	EBSCSIDriverRole      = "ebs-csi-driver-role"
	EFSCSIDriverRole      = "efs-csi-driver-role"
	ClusterAutoscalerRole = "cluster-autoscaler-role"

	GiantSwarmReleaseCrossplaneNodesIAMRoles = "34.0.0"

	IAMControllerOwnedTag = "capi-iam-controller/owned"
	ClusterIDTag          = "sigs.k8s.io/cluster-api-provider-aws/cluster/%s"
)

// IAMClient defines all of the methods that we use of the IAM service.
// The AWS SDK used to defined this, but not anymore since v2.
// I hate this.
type IAMClient interface {
	iam.GetRoleAPIClient
	iam.ListRolePoliciesAPIClient
	iam.ListAttachedRolePoliciesAPIClient

	AddRoleToInstanceProfile(ctx context.Context, params *iam.AddRoleToInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.AddRoleToInstanceProfileOutput, error)
	CreateInstanceProfile(ctx context.Context, params *iam.CreateInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.CreateInstanceProfileOutput, error)
	CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error)
	DeleteInstanceProfile(ctx context.Context, params *iam.DeleteInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.DeleteInstanceProfileOutput, error)
	DeleteRole(ctx context.Context, params *iam.DeleteRoleInput, optFns ...func(*iam.Options)) (*iam.DeleteRoleOutput, error)
	DeleteRolePolicy(ctx context.Context, params *iam.DeleteRolePolicyInput, optFns ...func(*iam.Options)) (*iam.DeleteRolePolicyOutput, error)
	DetachRolePolicy(ctx context.Context, params *iam.DetachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.DetachRolePolicyOutput, error)
	GetRolePolicy(ctx context.Context, params *iam.GetRolePolicyInput, optFns ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error)
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
	RemoveRoleFromInstanceProfile(ctx context.Context, params *iam.RemoveRoleFromInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.RemoveRoleFromInstanceProfileOutput, error)
	UpdateAssumeRolePolicy(ctx context.Context, params *iam.UpdateAssumeRolePolicyInput, optFns ...func(*iam.Options)) (*iam.UpdateAssumeRolePolicyOutput, error)
}

// EKSClient defines all the methods that we use of the EKS service.
// I hate this less.
type EKSClient interface {
	eks.DescribeClusterAPIClient
}

type IAMServiceConfig struct {
	ObjectLabels     map[string]string // not always filled
	AWSConfig        *aws.Config
	ClusterName      string
	ClusterRelease   string
	MainRoleName     string
	Log              logr.Logger
	RoleType         string
	Region           string
	PrincipalRoleARN string
	CustomTags       map[string]string

	IAMClientFactory func(aws.Config, string) IAMClient
}

type IAMService struct {
	objectLabels     map[string]string // not always filled
	clusterName      string
	clusterRelease   string
	iamClient        IAMClient
	eksClient        EKSClient
	mainRoleName     string
	log              logr.Logger
	region           string
	roleType         string
	principalRoleARN string
	customTags       map[string]string
}

type Route53RoleParams struct {
	AWSPartition     string
	EC2ServiceDomain string
	AccountID        string
	IRSATrustDomains []string
	Namespace        string
	ServiceAccount   string
	PrincipalRoleARN string
}

func New(config IAMServiceConfig) (*IAMService, error) {
	if config.AWSConfig == nil {
		return nil, errors.New("cannot create IAMService with AWSConfig equal to nil")
	}
	if config.IAMClientFactory == nil {
		return nil, errors.New("cannot create IAMService with IAMClientFactory equal to nil")
	}
	if config.ClusterName == "" {
		return nil, errors.New("cannot create IAMService with empty ClusterName")
	}
	if config.ClusterRelease == "" {
		return nil, errors.New("cannot create IAMService with empty ClusterRelease")
	}
	if config.MainRoleName == "" {
		return nil, errors.New("cannot create IAMService with empty MainRoleName")
	}
	if config.RoleType != ControlPlaneRole && config.RoleType != NodesRole && config.RoleType != BastionRole && config.RoleType != IRSARole {
		return nil, fmt.Errorf("cannot create IAMService with invalid RoleType '%s'", config.RoleType)
	}
	if config.ObjectLabels == nil {
		config.ObjectLabels = map[string]string{}
	}
	iamClient := config.IAMClientFactory(*config.AWSConfig, config.Region)
	eksClient := eks.NewFromConfig(*config.AWSConfig)

	l := config.Log.WithValues("clusterName", config.ClusterName, "iam-role", config.RoleType)
	s := &IAMService{
		objectLabels:     config.ObjectLabels,
		clusterName:      config.ClusterName,
		clusterRelease:   config.ClusterRelease,
		iamClient:        iamClient,
		eksClient:        eksClient,
		mainRoleName:     config.MainRoleName,
		log:              l,
		roleType:         config.RoleType,
		region:           config.Region,
		principalRoleARN: config.PrincipalRoleARN,
		customTags:       config.CustomTags,
	}

	return s, nil
}

func (s *IAMService) ReconcileRole() error {
	s.log.Info("reconciling IAM role")

	params := struct {
		ClusterName      string
		EC2ServiceDomain string
		AWSPartition     string
		ObjectLabels     map[string]string
	}{
		ClusterName:      s.clusterName,
		EC2ServiceDomain: ec2ServiceDomain(s.region),
		AWSPartition:     awsPartition(s.region),
		ObjectLabels:     s.objectLabels,
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

		i := &iam.GetRoleInput{
			RoleName: aws.String(s.mainRoleName),
		}

		o, err := s.iamClient.GetRole(context.TODO(), i)
		if err != nil {
			s.log.Error(err, "failed to fetch ControlPlane role")
			return err
		}

		controlPlaneRoleARN = *o.Role.Arn
	}

	params := struct {
		AWSPartition        string
		ControlPlaneRoleARN string
		EC2ServiceDomain    string
	}{
		AWSPartition:        awsPartition(s.region),
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

func (s *IAMService) ReconcileRolesForIRSA(awsAccountID string, irsaTrustDomains []string) error {
	s.log.Info("reconciling IAM roles for IRSA")

	for _, roleTypeToReconcile := range getIRSARoles() {
		var params Route53RoleParams
		params, err := s.generateRoute53RoleParams(roleTypeToReconcile, awsAccountID, irsaTrustDomains)
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

func (s *IAMService) generateRoute53RoleParams(roleTypeToReconcile string, awsAccountID string, irsaTrustDomains []string) (Route53RoleParams, error) {
	if len(irsaTrustDomains) == 0 || slices.ContainsFunc(irsaTrustDomains, func(irsaTrustDomain string) bool { return irsaTrustDomain == "" }) {
		return Route53RoleParams{}, fmt.Errorf("irsaTrustDomains cannot be empty or have empty values: %v", irsaTrustDomains)
	}

	namespace := "kube-system"
	serviceAccount, err := getServiceAccount(roleTypeToReconcile)
	if err != nil {
		s.log.Error(err, "failed to get service account for role")
		return Route53RoleParams{}, err
	}

	params := Route53RoleParams{
		AWSPartition:     awsPartition(s.region),
		EC2ServiceDomain: ec2ServiceDomain(s.region),
		AccountID:        awsAccountID,
		IRSATrustDomains: irsaTrustDomains,
		Namespace:        namespace,
		ServiceAccount:   serviceAccount,
	}

	return params, nil
}

func (s *IAMService) reconcileRole(roleName string, roleType string, params any) error {
	l := s.log.WithValues("role_name", roleName, "role_type", roleType)

	// In a certain GiantSwarm release we changed how the IAM Roles are managed within `cluster-aws`. Crossplane will manage the roles from now on.
	// This means that we no longer need to manage the IAM Roles for nodes (workers, control-plane) from this controller.
	// If a cluster is using a release equal or greater than the release containing these changes, we skip the nodes IAM Role creation.
	if roleType == ControlPlaneRole || roleType == NodesRole {
		// Parse the current cluster release version
		currentVersion, err := semver.NewVersion(s.clusterRelease)
		if err != nil {
			return err
		}

		// Parse the threshold version
		thresholdVersion, err := semver.NewVersion(GiantSwarmReleaseCrossplaneNodesIAMRoles)
		if err != nil {
			// This should never happen as we control this constant
			return err
		}

		// Check if the current version is equal or greater than threshold version
		if currentVersion.GreaterThanEqual(thresholdVersion) {
			return nil
		}
	}

	err := s.createRole(roleName, roleType, params)
	if err != nil {
		return err
	}

	if roleType == IRSARole || roleType == CertManagerRole || roleType == Route53Role || roleType == ALBConrollerRole || roleType == EBSCSIDriverRole || roleType == EFSCSIDriverRole || roleType == ClusterAutoscalerRole {
		if err = s.applyAssumePolicyRole(roleName, roleType, params); err != nil {
			l.Error(err, "Failed to apply assume role policy to role")
			return err
		}
	}

	// we only attach the inline policy to a role that is owned (and was created) by iam controller
	err = s.attachInlinePolicy(roleName, roleType, params)
	if err != nil {
		return err
	}

	return nil
}

// createRole will create requested IAM role
func (s *IAMService) createRole(roleName string, roleType string, params any) error {
	l := s.log.WithValues("role_name", roleName, "role_type", roleType)

	_, err := s.iamClient.GetRole(context.TODO(), &iam.GetRoleInput{
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

	tags := []iamtypes.Tag{
		{
			Key:   aws.String(IAMControllerOwnedTag),
			Value: aws.String(""),
		},
		{
			Key:   aws.String(fmt.Sprintf(ClusterIDTag, s.clusterName)),
			Value: aws.String("owned"),
		},
	}
	for k, v := range s.customTags {
		tags = append(tags, iamtypes.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	_, err = s.iamClient.CreateRole(context.TODO(), &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(assumeRolePolicyDocument),
		Tags:                     tags,
	})
	if err != nil {
		l.Error(err, "failed to create IAM Role")
		return err
	}

	i2 := &iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
		Tags:                tags,
	}

	_, err = s.iamClient.CreateInstanceProfile(context.TODO(), i2)
	if IsAlreadyExists(err) {
		// fall thru
	} else if err != nil {
		l.Error(err, "failed to create instance profile")
		return err
	}

	i3 := &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
		RoleName:            aws.String(roleName),
	}

	_, err = s.iamClient.AddRoleToInstanceProfile(context.TODO(), i3)
	if IsAlreadyExists(err) {
		// fall thru
	} else if err != nil {
		l.Error(err, "failed to add role to instance profile")
		return err
	}

	l.Info("successfully created a new IAM role")

	return nil
}

func (s *IAMService) applyAssumePolicyRole(roleName string, roleType string, params any) error {
	log := s.log.WithValues("role_name", roleName)
	i := &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	}

	_, err := s.iamClient.GetRole(context.TODO(), i)

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

	updateInput := &iam.UpdateAssumeRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyDocument: aws.String(assumeRolePolicyDocument),
	}

	_, err = s.iamClient.UpdateAssumeRolePolicy(context.TODO(), updateInput)

	return err
}

// attachInlinePolicy  will attach inline policy to the main IAM role
func (s *IAMService) attachInlinePolicy(roleName string, roleType string, params any) error {
	l := s.log.WithValues("role_name", roleName)
	tmpl := getInlinePolicyTemplate(roleType, s.objectLabels)

	policyDocument, err := generatePolicyDocument(tmpl, params)
	if err != nil {
		l.Error(err, "failed to generate inline policy document from template for IAM role")
		return err
	}

	// check if the inline policy already exists
	output, err := s.iamClient.GetRolePolicy(context.TODO(), &iam.GetRolePolicyInput{
		RoleName:   aws.String(roleName),
		PolicyName: aws.String(policyName(s.roleType, s.clusterName)),
	})
	if err != nil && !IsNotFound(err) {
		l.Error(err, "failed to fetch inline policy for IAM role")
		return err
	}

	if err == nil {
		// Policy already exists

		isEqual, err := areEqualPolicy(*output.PolicyDocument, policyDocument)
		if err != nil {
			l.Error(err, "failed to compare inline policy documents")
			return err
		}
		if isEqual {
			l.Info("inline policy for IAM role already exists, skipping")
			return nil
		}

		_, err = s.iamClient.DeleteRolePolicy(context.TODO(), &iam.DeleteRolePolicyInput{
			PolicyName: aws.String(policyName(s.roleType, s.clusterName)),
			RoleName:   aws.String(roleName),
		})
		if err != nil {
			l.Error(err, "failed to delete inline policy from IAM Role")
			return err
		}
	}

	i := &iam.PutRolePolicyInput{
		PolicyName:     aws.String(policyName(s.roleType, s.clusterName)),
		PolicyDocument: aws.String(policyDocument),
		RoleName:       aws.String(roleName),
	}

	_, err = s.iamClient.PutRolePolicy(context.TODO(), i)
	if err != nil {
		l.Error(err, "failed to add inline policy to IAM Role")
		return err
	}
	l.Info("successfully added inline policy to IAM role")

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
	defer s.log.Info("finished deleting IAM roles for IRSA")

	for _, roleTypeToReconcile := range getIRSARoles() {
		err := s.deleteRole(roleName(roleTypeToReconcile, s.clusterName))
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *IAMService) deleteRole(roleName string) error {
	l := s.log.WithValues("role_name", roleName)

	// clean any attached policies, otherwise deletion of role will not work
	err := s.cleanRolePolicies(roleName)
	if err != nil {
		return err
	}

	i := &iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
		RoleName:            aws.String(roleName),
	}

	_, err = s.iamClient.RemoveRoleFromInstanceProfile(context.TODO(), i)
	if err != nil && !IsNotFound(err) {
		l.Error(err, "failed to remove role from instance profile")
		return err
	}

	i2 := &iam.DeleteInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
	}

	_, err = s.iamClient.DeleteInstanceProfile(context.TODO(), i2)
	if err != nil && !IsNotFound(err) {
		l.Error(err, "failed to delete instance profile")
		return err
	}

	// delete the role
	i3 := &iam.DeleteRoleInput{
		RoleName: aws.String(roleName),
	}

	_, err = s.iamClient.DeleteRole(context.TODO(), i3)
	if err != nil && !IsNotFound(err) {
		l.Error(err, "failed to delete role")
		return err
	}

	return nil
}

func (s *IAMService) cleanRolePolicies(roleName string) error {
	l := s.log.WithValues("role_name", roleName)

	err := s.cleanAttachedPolicies(roleName)
	if err != nil {
		l.Error(err, "failed to clean attached policies from IAM Role")
		return err
	}

	err = s.cleanInlinePolicies(roleName)
	if err != nil {
		l.Error(err, "failed to clean inline policies from IAM Role")
		return err
	}

	l.Info("cleaned attached and inline policies from IAM Role")
	return nil
}

func (s *IAMService) cleanAttachedPolicies(roleName string) error {
	l := s.log.WithValues("role_name", roleName)
	i := &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	}

	o, err := s.iamClient.ListAttachedRolePolicies(context.TODO(), i)
	if IsNotFound(err) {
		l.Info("role not found")
		return nil
	}
	if err != nil {
		l.Error(err, "failed to list attached policies")
		return err
	}

	for _, p := range o.AttachedPolicies {
		l.Info(fmt.Sprintf("detaching policy %s", *p.PolicyName))

		i := &iam.DetachRolePolicyInput{
			PolicyArn: p.PolicyArn,
			RoleName:  aws.String(roleName),
		}

		_, err := s.iamClient.DetachRolePolicy(context.TODO(), i)
		if err != nil {
			l.Error(err, fmt.Sprintf("failed to detach policy %s", *p.PolicyName))
			return err
		}

		l.Info(fmt.Sprintf("detached policy %s", *p.PolicyName))
	}
	return nil
}

func (s *IAMService) cleanInlinePolicies(roleName string) error {
	l := s.log.WithValues("role_name", roleName)
	i := &iam.ListRolePoliciesInput{
		RoleName: aws.String(roleName),
	}

	o, err := s.iamClient.ListRolePolicies(context.TODO(), i)
	if IsNotFound(err) {
		l.Info("role not found")
		return nil
	}
	if err != nil {
		l.Error(err, "failed to list inline policies")
		return err
	}

	for _, p := range o.PolicyNames {
		l.Info(fmt.Sprintf("deleting inline policy %s", p))

		i := &iam.DeleteRolePolicyInput{
			RoleName:   aws.String(roleName),
			PolicyName: aws.String(p),
		}

		_, err := s.iamClient.DeleteRolePolicy(context.TODO(), i)
		if err != nil && !IsNotFound(err) {
			l.Error(err, fmt.Sprintf("failed to delete inline policy %s", p))
			return err
		}
		l.Info(fmt.Sprintf("deleted inline policy %s", p))
	}

	return nil
}

func (s *IAMService) GetRoleARN(roleName string) (string, error) {
	o, err := s.iamClient.GetRole(context.TODO(), &iam.GetRoleInput{
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
	cluster, err := s.eksClient.DescribeCluster(context.TODO(), i)
	if err != nil {
		return "", microerror.Mask(err)
	}
	if cluster == nil || cluster.Cluster == nil || cluster.Cluster.Identity == nil || cluster.Cluster.Identity.Oidc == nil || cluster.Cluster.Identity.Oidc.Issuer == nil {
		return "", microerror.Maskf(invalidClusterError, "cluster %s does not have OIDC identity", clusterName)
	}

	id := strings.TrimPrefix(*cluster.Cluster.Identity.Oidc.Issuer, "https://")

	return id, nil
}

func roleName(role string, clusterID string) string {
	switch role {
	case Route53Role:
		return fmt.Sprintf("%s-Route53Manager-Role", clusterID)
	case KIAMRole:
		return fmt.Sprintf("%s-IAMManager-Role", clusterID)
	case CertManagerRole:
		return fmt.Sprintf("%s-CertManager-Role", clusterID)
	default:
		return fmt.Sprintf("%s-%s", clusterID, role)
	}
}

func policyName(role string, clusterID string) string {
	return fmt.Sprintf("%s-%s-policy", role, clusterID)
}

func getServiceAccount(role string) (string, error) {
	switch role {
	case CertManagerRole:
		return "cert-manager-app", nil
	case IRSARole:
		return "external-dns", nil
	case Route53Role:
		return "external-dns", nil
	case ALBConrollerRole:
		return "aws-load-balancer-controller", nil
	case EBSCSIDriverRole:
		return "ebs-csi-controller-sa", nil
	case EFSCSIDriverRole:
		return "efs-csi-sa", nil
	case ClusterAutoscalerRole:
		return "cluster-autoscaler", nil
	default:
		return "", fmt.Errorf("cannot get service account for specified role - %s", role)
	}
}

func getIRSARoles() []string {
	return []string{
		Route53Role,
		CertManagerRole,
		ALBConrollerRole,
		EBSCSIDriverRole,
		EFSCSIDriverRole,
		ClusterAutoscalerRole,
	}
}

func areEqualPolicy(encodedPolicy, expectedPolicy string) (bool, error) {
	decodedPolicy, err := urlDecode(encodedPolicy)
	if err != nil {
		return false, err
	}
	return areEqualJSON(decodedPolicy, expectedPolicy)
}

func areEqualJSON(s1, s2 string) (bool, error) {
	var o1 any
	var o2 any

	var err error
	err = json.Unmarshal([]byte(s1), &o1)
	if err != nil {
		return false, err
	}
	err = json.Unmarshal([]byte(s2), &o2)
	if err != nil {
		return false, err
	}

	return reflect.DeepEqual(o1, o2), nil
}

func urlDecode(encodedValue string) (string, error) {
	decodedValue, err := url.QueryUnescape(encodedValue)
	if err != nil {
		return "", err
	}

	return decodedValue, nil
}
