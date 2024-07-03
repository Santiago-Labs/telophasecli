package resourceoperation

import (
	"fmt"
	"testing"

	"github.com/santiago-labs/telophasecli/resource"
	"github.com/stretchr/testify/assert"
)

func TestDiffTags(t *testing.T) {
	tests := []struct {
		description string
		input       resource.OrganizationUnit

		wantAdded   []string
		wantRemoved []string
	}{
		{
			description: "adding basic tag",
			input: resource.OrganizationUnit{
				Accounts: []*resource.Account{
					{
						AccountName: "mgmt",
						Tags: []string{
							"ou=mgmt",
						},
					},
				},
			},
			wantAdded: []string{"ou=mgmt"},
		},
		{
			description: "removing basic tag",
			input: resource.OrganizationUnit{
				Accounts: []*resource.Account{
					{
						AccountName: "mgmt",
						AWSTags: []string{
							"ou=mgmt",
						},
					},
				},
			},
			wantRemoved: []string{"ou=mgmt"},
		},
		{
			description: "no tag diff",
			input: resource.OrganizationUnit{
				Accounts: []*resource.Account{
					{
						AccountName: "mgmt",
						Tags: []string{
							"ou=mgmt",
						},
						AWSTags: []string{
							"ou=mgmt",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		added, removed := diffTags(tc.input.Accounts[0])
		fmt.Println("added", added)
		fmt.Println("removed", removed)
		assert.Equal(t, tc.wantAdded, added, "added: "+tc.description)
		assert.Equal(t, tc.wantRemoved, removed, "removed: "+tc.description)
	}
}
