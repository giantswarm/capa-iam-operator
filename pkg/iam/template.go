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

func getInlinePolicyTemplate(roleType string) string {
	switch roleType {
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
	case CertManagerRole:
		return route53RolePolicyTemplateForCertManager
	case ALBConrollerRole:
		return ALBControllerPolicyTemplate
	default:
		return ""
	}
}

func getTrustPolicyTemplate(roleType string) string {
	switch roleType {
	case BastionRole:
		return ec2TrustIdentityPolicyTemplate
	case ControlPlaneRole:
		return ec2TrustIdentityPolicyTemplate
	case NodesRole:
		return ec2TrustIdentityPolicyTemplate
	case Route53Role:
		return trustIdentityPolicyKIAMAndIRSA
	case KIAMRole:
		return kiamTrustIdentityPolicy
	case IRSARole:
		return trustIdentityPolicyKIAMAndIRSA
	case CertManagerRole:
		return trustIdentityPolicyKIAMAndIRSA
	case ALBConrollerRole:
		return trustIdentityPolicyKIAMAndIRSA
	default:
		return ""
	}
}
