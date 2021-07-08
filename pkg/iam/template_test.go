package iam

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
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
}

// It uses golden file as reference template and when changes to template are
// intentional, they can be updated by providing -update flag for go test.
//
//  go test ./pkg/iam -run Test_Role_Policy_Template_Render -update
//
func Test_Role_Policy_Template_Render(t *testing.T) {
	testCases := []struct {
		name   string
		params testParams
		role   string
	}{
		{
			name: "case-0-control-plane",
			params: testParams{
				ClusterName: "test",
			},
			role: ControlPlaneRole,
		},
		{
			name: "case-1-node",
			params: testParams{
				ClusterName: "test",
			},
			role: NodesRole,
		},
		{
			name: "case-2-kiam",
			params: testParams{
				ClusterName: "test",
			},
			role: KIAMRole,
		},
		{
			name: "case-3-route53",
			params: testParams{
				ClusterName: "test",
			},
			role: Route53Role,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var err error

			template := getInlinePolicyTemplate(tc.role)

			templateBody, err := generatePolicyDocument(template, tc.params)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join("testdata", fmt.Sprintf("inline-role-policy-%s.golden", tc.name))

			if *update {
				err := ioutil.WriteFile(p, []byte(templateBody), 0644) // nolint: gosec
				if err != nil {
					t.Fatal(err)
				}
			}
			goldenFile, err := ioutil.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal([]byte(templateBody), goldenFile) {
				t.Fatalf("\n\n%s\n", cmp.Diff(string(goldenFile), templateBody))
			}
		})
	}
}

// It uses golden file as reference template and when changes to template are
// intentional, they can be updated by providing -update flag for go test.
//
//  go test ./pkg/iam -run Test_Trust_Identity_Policy_Template_Render -update
//
func Test_Trust_Identity_Policy_Template_Render(t *testing.T) {
	testCases := []struct {
		name   string
		params testParams
		role   string
	}{
		{
			name: "case-0-control-plane",
			params: testParams{
				EC2ServiceDomain: ec2ServiceDomain("eu-central-1"),
			},
			role: ControlPlaneRole,
		},
		{
			name: "case-1-node",
			params: testParams{
				EC2ServiceDomain: ec2ServiceDomain("eu-central-1"),
			},
			role: NodesRole,
		},
		{
			name: "case-2-kiam",
			params: testParams{
				ControlPlaneRoleARN: "arn:aws:iam::751852626996:role/apie1-control-plane-role",
			},
			role: KIAMRole,
		},
		{
			name: "case-3-route53",
			params: testParams{
				KIAMRoleARN: "arn:aws:iam::751852626996:role/apie1-kiam-role",
			},
			role: Route53Role,
		},
		{
			name: "case-3-control-plane-china",
			params: testParams{
				EC2ServiceDomain: ec2ServiceDomain("eu-central-1"),
			},
			role: ControlPlaneRole,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var err error

			template := gentTrustPolicyTemplate(tc.role)

			templateBody, err := generatePolicyDocument(template, tc.params)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join("testdata", fmt.Sprintf("trust-identity-policy-%s.golden", tc.name))

			if *update {
				err := ioutil.WriteFile(p, []byte(templateBody), 0644) // nolint: gosec
				if err != nil {
					t.Fatal(err)
				}
			}
			goldenFile, err := ioutil.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal([]byte(templateBody), goldenFile) {
				t.Fatalf("\n\n%s\n", cmp.Diff(string(goldenFile), templateBody))
			}
		})
	}
}
