package iam

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type TemplateParams struct {
	ClusterID string
	Region    string
	RegionARN string
}

func regionARN(region string) string {
	if strings.Contains(region, "cn-") {
		return "aws-cn"
	}
	return "aws"
}

func (s *IAMService) generatePolicyDocument() (string, error) {
	var t string
	if s.roleType == ControlPlaneRole {
		t = controlPlaneTemplate
	} else if s.roleType == NodesRole {
		t = nodesTemplate
	} else {
		return "", fmt.Errorf("unknown role type '%s'", s.roleType)
	}

	params := TemplateParams{
		ClusterID: s.clusterID,
		Region:    s.region,
		RegionARN: regionARN(s.region),
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
