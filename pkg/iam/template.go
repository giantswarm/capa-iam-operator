package iam

import (
	"bytes"
	"strings"
	"text/template"
)

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
	case BastionRole:
		return bastionPolicyTemplate
	case ControlPlaneRole:
		return controlPlanePolicyTemplate
	case NodesRole:
		return nodesTemplate
	case Route53Role:
		return route53RolePolicyTemplate
	case KIAMRole:
		return kiamRolePolicyTemplate
	case IRSARole:
		return route53RolePolicyTemplate
	default:
		return ""
	}
}

func gentTrustPolicyTemplate(roleName string) string {
	switch roleName {
	case BastionRole:
		return ec2TrustIdentityPolicyTemplate
	case ControlPlaneRole:
		return ec2TrustIdentityPolicyTemplate
	case NodesRole:
		return ec2TrustIdentityPolicyTemplate
	case Route53Role:
		return route53TrustIdentityPolicy
	case KIAMRole:
		return kiamTrustIdentityPolicy
	case IRSARole:
		return route53TrustIdentityPolicyWithIRSA
	default:
		return ""
	}
}
