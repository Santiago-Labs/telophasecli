package ymlparser

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awsorgs/awsorgsmock"
	"github.com/santiago-labs/telophasecli/resource"
	"github.com/stretchr/testify/require"
)

func basicOU() resource.OrganizationUnit {
	return resource.OrganizationUnit{
		OUName: "root",
		OUID:   aws.String("r-0000"),
		ChildOUs: []*resource.OrganizationUnit{
			{
				OUName: "ExampleOU",
				BaselineStacks: []resource.Stack{
					{
						Type: "CDK",
						Path: "examples/localstack/s3-remote-state",
						Name: "example",
					},
				},
				Tags: []string{
					"ou=ExampleTenants",
				},
				Accounts: []*resource.Account{
					{
						Email:       "test1@example.com",
						AccountName: "test1",
						BaselineStacks: []resource.Stack{
							{
								Type:   "CDK",
								Path:   "examples/cdk/sqs",
								Name:   "example",
								Region: "us-west-2,us-east-1",
							},
						},
						AccountID: "10000000000",
					},
					{
						Email:       "test2@example.com",
						AccountName: "test2",
						AccountID:   "20000000000",
					},
				},
			},
			{
				OUName: "ExampleOU2",
				Accounts: []*resource.Account{
					{
						Email:       "test3@example.com",
						AccountName: "test3",
						AccountID:   "30000000000",
					},
				},
			},
		},
	}
}

func TestParseOrganization(t *testing.T) {
	tests := []struct {
		name    string
		orgPath string
		want    resource.OrganizationUnit
	}{
		{
			name:    "basic OU",
			orgPath: "./testdata/organization-basic.yml",
			want:    basicOU(),
		},
		{
			name:    "OU with child filepaths",
			orgPath: "./testdata/organization-with-filepath.yml",
			want:    basicOU(),
		},
	}
	for _, tc := range tests {
		mockClient := awsorgs.New(&awsorgs.Config{
			OrganizationClient: awsorgsmock.New(),
		})

		parser := NewParser(mockClient)

		actual, err := parser.ParseOrganization(context.Background(), tc.orgPath)
		require.NoError(t, err)
		ignoreFields := []cmp.Option{
			cmpopts.IgnoreFields(resource.OrganizationUnit{}, "Parent"),
			cmpopts.IgnoreFields(resource.Account{}, "Parent"),
		}

		if diff := cmp.Diff(tc.want.ChildOUs, actual.ChildOUs, ignoreFields...); diff != "" {
			t.Errorf(fmt.Sprintf("expected no diff for %s got diff: %+v", tc.name, diff))
		}
	}
}
