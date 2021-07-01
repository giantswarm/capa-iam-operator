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

// It uses golden file as reference template and when changes to template are
// intentional, they can be updated by providing -update flag for go test.
//
//  go test ./pkg/iam -run Test_Assume_Role_Template_Render -update
//
func Test_Assume_Role_Template_Render(t *testing.T) {
	testCases := []struct {
		name   string
		params TemplateParams
	}{
		{
			name: "case-0",
			params: TemplateParams{
				Region: "eu-west-1",
			},
		},
		{
			name: "case-1-china",
			params: TemplateParams{
				Region: "cn-north-1",
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var err error

			templateBody, err := generateAssumeRolePolicyDocument(tc.params.Region)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join("testdata", fmt.Sprintf("assume-role-policy-%s.golden", tc.name))

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
//  go test ./pkg/iam -run Test_Role_Policy_Template_Render -update
//
func Test_Role_Policy_Template_Render(t *testing.T) {
	testCases := []struct {
		name     string
		params   TemplateParams
		roleType string
	}{
		{
			name:     "case-0",
			roleType: ControlPlaneRole,
			params: TemplateParams{
				ClusterID: "test1",
				Region:    "eu-west-1",
				RegionARN: "aws",
			},
		},
		{
			name:     "case-1",
			roleType: NodesRole,
			params: TemplateParams{
				ClusterID: "test2",
				Region:    "eu-west-1",
				RegionARN: "aws",
			},
		},
		{
			name:     "case-2-china",
			roleType: ControlPlaneRole,
			params: TemplateParams{
				ClusterID: "test3",
				Region:    "cn-north-1",
				RegionARN: regionARN("cn-north-1"),
			},
		},
		{
			name:     "case-3-china",
			roleType: NodesRole,
			params: TemplateParams{
				ClusterID: "test4",
				Region:    "cn-northeast-1",
				RegionARN: regionARN("cn-north-1"),
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var err error

			templateBody, err := generatePolicyDocument(tc.params.ClusterID, tc.roleType, tc.params.Region)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join("testdata", fmt.Sprintf("inline-policy-%s.golden", tc.name))

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
