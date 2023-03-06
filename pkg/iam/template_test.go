package iam

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update .golden CF template file")

type testParams struct {
	ClusterName         string
	ControlPlaneRoleARN string
	EC2ServiceDomain    string
	KIAMRoleARN         string
	AccountID           string
	CloudFrontDomain    string
	Namespace           string
	ServiceAccount      string
}

// TODO remove these tests in favor of controller tests?!
// It uses golden file as reference template and when changes to template are
// intentional, they can be updated by providing -update flag for go test.
//
//	go test ./pkg/iam -run Test_Role_Policy_Template_Render -update
func Test_Role_Policy_Template_Render(t *testing.T) {
	testCases := []struct {
		name     string
		params   testParams
		roleType string
	}{
		{
			name: "case-0-control-plane",
			params: testParams{
				ClusterName: "test",
			},
			roleType: ControlPlaneRole,
		},
		{
			name: "case-1-node",
			params: testParams{
				ClusterName: "test",
			},
			roleType: NodesRole,
		},
		{
			name: "case-2-kiam",
			params: testParams{
				ClusterName: "test",
			},
			roleType: KIAMRole,
		},
		{
			name: "case-3-route53",
			params: testParams{
				ClusterName: "test",
			},
			roleType: Route53Role,
		},
		{
			name: "case-3-cert-manager",
			params: testParams{
				ClusterName: "test",
			},
			roleType: CertManagerRole,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var err error

			template := getInlinePolicyTemplate(tc.roleType)

			templateBody, err := generatePolicyDocument(template, tc.params)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join("testdata", fmt.Sprintf("inline-role-policy-%s.golden", tc.name))

			if *update {
				err := os.WriteFile(p, []byte(templateBody), 0644) // nolint: gosec
				if err != nil {
					t.Fatal(err)
				}
			}
			goldenFile, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal([]byte(templateBody), goldenFile) {
				t.Fatalf("Rendered body does not match %s\n\n%s\n", p, cmp.Diff(string(goldenFile), templateBody))
			}
		})
	}
}

// It uses golden file as reference template and when changes to template are
// intentional, they can be updated by providing -update flag for go test.
//
//	go test ./pkg/iam -run Test_Trust_Identity_Policy_Template_Render -update
func Test_Trust_Identity_Policy_Template_Render(t *testing.T) {
	testCases := []struct {
		name     string
		params   testParams
		roleType string
	}{
		{
			name: "case-0-control-plane",
			params: testParams{
				EC2ServiceDomain: ec2ServiceDomain("eu-central-1"),
			},
			roleType: ControlPlaneRole,
		},
		{
			name: "case-1-node",
			params: testParams{
				EC2ServiceDomain: ec2ServiceDomain("eu-central-1"),
			},
			roleType: NodesRole,
		},
		{
			name: "case-2-kiam",
			params: testParams{
				ControlPlaneRoleARN: "arn:aws:iam::751852626996:role/apie1-control-plane-role",
			},
			roleType: KIAMRole,
		},
		{
			name: "case-3-route53",
			params: testParams{
				KIAMRoleARN: "arn:aws:iam::751852626996:role/apie1-kiam-role",
			},
			roleType: Route53Role,
		},
		{
			name: "case-3-route53-with-IRSA",
			params: testParams{
				KIAMRoleARN:      "arn:aws:iam::751852626996:role/apie1-kiam-role",
				AccountID:        "751852626996",
				CloudFrontDomain: "d12qpcaph79a8w.cloudfront.net",
				Namespace:        "kube-system",
				ServiceAccount:   "external-dns",
			},
			roleType: IRSARole,
		},
		{
			name: "case-3-route53-for-cert-manager",
			params: testParams{
				KIAMRoleARN:      "arn:aws:iam::751852626996:role/apie1-kiam-role",
				AccountID:        "751852626996",
				CloudFrontDomain: "d12qpcaph79a8w.cloudfront.net",
				Namespace:        "kube-system",
				ServiceAccount:   "cert-manager-controller",
			},
			roleType: CertManagerRole,
		},
		{
			name: "case-3-control-plane-china",
			params: testParams{
				EC2ServiceDomain: ec2ServiceDomain("eu-central-1"),
			},
			roleType: ControlPlaneRole,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var err error

			template := getTrustPolicyTemplate(tc.roleType)

			templateBody, err := generatePolicyDocument(template, tc.params)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join("testdata", fmt.Sprintf("trust-identity-policy-%s.golden", tc.name))

			if *update {
				err := os.WriteFile(p, []byte(templateBody), 0644) // nolint: gosec
				if err != nil {
					t.Fatal(err)
				}
			}
			goldenFile, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal([]byte(templateBody), goldenFile) {
				t.Fatalf("\n\n%s\n", cmp.Diff(string(goldenFile), templateBody))
			}
		})
	}
}
