package iam

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type TemplateParams struct {
	ClusterName      string
	EC2ServiceDomain string
	Region           string
	RegionARN        string
}

func generatePolicyDocument(t string, params interface{}) (string, error) {
	tmpl, err := template.New("policy").Parse(t)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, params)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func xgeneratePolicyDocument(clusterName string, roleType string, region string) (string, error) {
	var t string
	if roleType == ControlPlaneRole {
		t = controlPlanePolicyTemplate
	} else if roleType == NodesRole {
		t = nodesTemplate
	} else {
		return "", fmt.Errorf("unknown role type '%s'", roleType)
	}

	params := TemplateParams{
		ClusterName: clusterName,
		Region:      region,
		RegionARN:   regionARN(region),
	}

	tmpl, err := template.New("policy").Parse(t)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, params)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func regionARN(region string) string {
	if isChinaRegion(region) {
		return "aws-cn"
	}
	return "aws"
}

func ec2ServiceDomain(region string) string {
	domain := "ec2.amazonaws.com"

	if isChinaRegion(region) {
		domain += ".cn"
	}

	return domain
}

func isChinaRegion(region string) bool {
	return strings.Contains(region, "cn-")
}

func getInlinePolicyTemplate(roleName string) string {
	switch roleName {
	case ControlPlaneRole:
		return controlPlanePolicyTemplate
	case NodesRole:
		return nodesTemplate
	case Route53Role:
		return route53RolePolicyTemplate
	case KIAMRole:
		return kiamRolePolicyTemplate
	default:
		return ""
	}
}

func gentTrustPolicyTemplate(roleName string) string {
	switch roleName {
	case ControlPlaneRole:
		return ec2TrustIdentityPolicyTemplate
	case NodesRole:
		return ec2TrustIdentityPolicyTemplate
	case Route53Role:
		return route53TrustIdentityPolicy
	case KIAMRole:
		return kiamTrustIdentityPolicy
	default:
		return ""
	}
}
