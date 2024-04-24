package resource_test

import (
	"fmt"
	"testing"

	"github.com/santiago-labs/telophasecli/resource"
	"github.com/stretchr/testify/assert"
)

var (
	rootOU = &resource.OrganizationUnit{
		BaselineStacks: []resource.Stack{
			{
				Name: "Tf1",
				Type: "Terraform",
				Path: "tf/example",
			},
			{
				Name: "cdk1",
				Type: "CDK",
				Path: "cdk/example",
			},
		},
		Accounts: []*resource.Account{
			{
				Email:       "example1@example.com",
				AccountName: "Example1",
				AccountID:   "1",
				BaselineStacks: []resource.Stack{
					{
						Name: "cdk3",
						Type: "CDK",
						Path: "cdk/example3",
					},
				},
			},
			{
				Email:       "example2@example.com",
				AccountName: "Example2",
				AccountID:   "2",
			},
		},
		ChildOUs: []*resource.OrganizationUnit{
			{
				Accounts: []*resource.Account{
					{
						Email:       "childou1@example.com",
						AccountName: "ChildOU1",
						AccountID:   "childou1",
						BaselineStacks: []resource.Stack{
							{
								Name:      "tf4",
								Type:      "Terraform",
								Path:      "tf/example4",
								Workspace: "${telophase.account_id}_${telophase.region}",
								Region:    "us-west-2,us-west-1",
							},
						},
					},
					{
						Email:       "childou2@example.com",
						AccountName: "ChildOU2",
						AccountID:   "childou2",
					},
				},
			},
		},
	}
)

func TestAllBaselineStacks(t *testing.T) {
	hydrateOUParent(rootOU)
	hydrateAccountParent(rootOU)

	type test struct {
		rootOU             *resource.OrganizationUnit
		targetAccountEmail string
		wantStacks         []resource.Stack
	}

	tests := []test{
		{
			rootOU:             rootOU,
			targetAccountEmail: "example1@example.com",
			wantStacks: []resource.Stack{
				{
					Name: "Tf1",
					Type: "Terraform",
					Path: "tf/example",
				},
				{
					Name: "cdk1",
					Type: "CDK",
					Path: "cdk/example",
				},
				{
					Name: "cdk3",
					Type: "CDK",
					Path: "cdk/example3",
				},
			},
		},
		{
			rootOU:             rootOU,
			targetAccountEmail: "example2@example.com",
			wantStacks: []resource.Stack{
				{
					Name: "Tf1",
					Type: "Terraform",
					Path: "tf/example",
				},
				{
					Name: "cdk1",
					Type: "CDK",
					Path: "cdk/example",
				},
			},
		},
		{
			rootOU:             rootOU,
			targetAccountEmail: "childou1@example.com",
			wantStacks: []resource.Stack{
				{
					Name: "Tf1",
					Type: "Terraform",
					Path: "tf/example",
				},
				{
					Name: "cdk1",
					Type: "CDK",
					Path: "cdk/example",
				},
				{
					Name:      "tf4",
					Type:      "Terraform",
					Path:      "tf/example4",
					Workspace: "${telophase.account_id}_${telophase.region}",
					Region:    "us-west-2",
				},
				{
					Name:      "tf4",
					Type:      "Terraform",
					Path:      "tf/example4",
					Workspace: "${telophase.account_id}_${telophase.region}",
					Region:    "us-west-1",
				},
			},
		},
		{
			rootOU:             rootOU,
			targetAccountEmail: "childou2@example.com",
			wantStacks: []resource.Stack{
				{
					Name: "Tf1",
					Type: "Terraform",
					Path: "tf/example",
				},
				{
					Name: "cdk1",
					Type: "CDK",
					Path: "cdk/example",
				},
			},
		},
	}

	for _, tc := range tests {
		acct := findAcctByEmail(tc.rootOU, tc.targetAccountEmail)
		baselineStacks, err := acct.AllBaselineStacks()
		assert.NoError(t, err, "shouldn't be an error on AllBaselineStacks")
		assert.Equal(t, baselineStacks, tc.wantStacks, fmt.Sprintf("stacks should be equal for account with email: %s", tc.targetAccountEmail))
	}
}

func TestFilterBaselineStacks(t *testing.T) {
	hydrateOUParent(rootOU)
	hydrateAccountParent(rootOU)

	type test struct {
		rootOU             *resource.OrganizationUnit
		description        string
		targetAccountEmail string
		filter             string
		wantStacks         []resource.Stack
	}

	tests := []test{
		{
			rootOU:             rootOU,
			targetAccountEmail: "example1@example.com",
			description:        "base case where we match all stacks",
			wantStacks: []resource.Stack{
				{
					Name: "Tf1",
					Type: "Terraform",
					Path: "tf/example",
				},
				{
					Name: "cdk3",
					Type: "CDK",
					Path: "cdk/example3",
				},
			},
			filter: "Tf1,cdk3",
		},
		{
			rootOU:             rootOU,
			targetAccountEmail: "example2@example.com",
			filter:             "cdk1",
			description:        "filter to one stack",
			wantStacks: []resource.Stack{
				{
					Name: "cdk1",
					Type: "CDK",
					Path: "cdk/example",
				},
			},
		},
		{
			rootOU:             rootOU,
			targetAccountEmail: "childou1@example.com",
			filter:             "cdk1,tf4",
			wantStacks: []resource.Stack{
				{
					Name: "cdk1",
					Type: "CDK",
					Path: "cdk/example",
				},
				{
					Name:      "tf4",
					Type:      "Terraform",
					Path:      "tf/example4",
					Workspace: "${telophase.account_id}_${telophase.region}",
					Region:    "us-west-2",
				},
				{
					Name:      "tf4",
					Type:      "Terraform",
					Path:      "tf/example4",
					Workspace: "${telophase.account_id}_${telophase.region}",
					Region:    "us-west-1",
				},
			},
		},
		{
			rootOU:             rootOU,
			targetAccountEmail: "childou2@example.com",
			filter:             "tf1,cdk1",
			wantStacks: []resource.Stack{
				{
					Name: "cdk1",
					Type: "CDK",
					Path: "cdk/example",
				},
			},
		},
	}

	for _, tc := range tests {
		acct := findAcctByEmail(tc.rootOU, tc.targetAccountEmail)
		baselineStacks, err := acct.FilterBaselineStacks(tc.filter)
		assert.NoError(t, err, "shouldn't be an error on AllBaselineStacks")
		assert.Equal(t, baselineStacks, tc.wantStacks, fmt.Sprintf("stacks should be equal for account with email: %s", tc.targetAccountEmail))
	}
}

func hydrateOUParent(parsedOU *resource.OrganizationUnit) {
	for _, parsedChild := range parsedOU.ChildOUs {
		parsedChild.Parent = parsedOU
		hydrateOUParent(parsedChild)
	}
}

func hydrateAccountParent(ou *resource.OrganizationUnit) {
	for idx := range ou.Accounts {
		ou.Accounts[idx].Parent = ou
	}

	for _, childOU := range ou.ChildOUs {
		hydrateAccountParent(childOU)
	}
}

func findAcctByEmail(ou *resource.OrganizationUnit, email string) *resource.Account {
	for _, acct := range ou.Accounts {
		if acct.Email == email {
			return acct
		}
	}

	var result *resource.Account
	for _, childOU := range ou.ChildOUs {
		result = findAcctByEmail(childOU, email)
		if result != nil {
			return result
		}
	}

	return nil
}
