package iam

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

const assumeRolePolicyDocumentTemplate = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "{{.EC2ServiceDomain}}"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}`

type TemplateParams struct {
	ClusterID        string
	EC2ServiceDomain string
	Region           string
	RegionARN        string
}

func generateAssumeRolePolicyDocument(region string) (string, error) {
	params := TemplateParams{
		EC2ServiceDomain: ec2ServiceDomain(region),
	}

	tmpl, err := template.New("policy").Parse(assumeRolePolicyDocumentTemplate)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, params)
	if err != nil {
		return "", err
	}
	fmt.Printf("generated template \n%s\n", buf.String())

	return buf.String(), nil
}

func generatePolicyDocument(clusterID string, roleType string, region string) (string, error) {
	var t string
	if roleType == ControlPlaneRole {
		t = controlPlaneTemplate
	} else if roleType == NodesRole {
		t = nodesTemplate
	} else {
		return "", fmt.Errorf("unknown role type '%s'", roleType)
	}

	params := TemplateParams{
		ClusterID: clusterID,
		Region:    region,
		RegionARN: regionARN(region),
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
