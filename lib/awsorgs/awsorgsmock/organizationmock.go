// package awsorgsmock provides a basic mocked implementation of the
// organizations interface for telophase.
//
// The structure fo the mock org is that account 0 is the root account.
// Account 1 - Child account
// Account 2 - Another Child account
// Account 3 - Orphan account within the organization.
// Other methods are mocked, but don't perform any functions to avoid nil pointer exceptions.
package awsorgsmock

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/organizations/organizationsiface"
)

func New() organizationsiface.OrganizationsAPI {
	return &mockedOrganizations{}
}

type mockedOrganizations struct {
	organizationsiface.OrganizationsAPI
}

// ListAccountsPagesWithContext mocks the ListAccountsPagesWithContext method
func (m *mockedOrganizations) ListAccountsPagesWithContext(ctx aws.Context, input *organizations.ListAccountsInput, fn func(*organizations.ListAccountsOutput, bool) bool, opts ...request.Option) error {
	return m.ListAccountsPagesWithContextFunc(ctx, input, fn, opts...)
}

func mockAccount(index int) *organizations.Account {
	return &organizations.Account{
		Id:    aws.String(fmt.Sprintf("%d0000000000", index)),
		Name:  aws.String(fmt.Sprintf("test%d", index)),
		Email: aws.String(fmt.Sprintf("test%d@example.com", index)),
	}
}

func (m *mockedOrganizations) ListAccountsPagesWithContextFunc(ctx aws.Context, input *organizations.ListAccountsInput, fn func(*organizations.ListAccountsOutput, bool) bool, opts ...request.Option) error {
	fn(&organizations.ListAccountsOutput{
		Accounts: []*organizations.Account{
			mockAccount(0),
			mockAccount(1),
			mockAccount(2),
			mockAccount(3),
		},
	}, true)

	return nil
}

func (m mockedOrganizations) DescribeOrganization(org *organizations.DescribeOrganizationInput) (*organizations.DescribeOrganizationOutput, error) {
	return &organizations.DescribeOrganizationOutput{
		Organization: &organizations.Organization{
			MasterAccountId: mockAccount(0).Id,
		},
	}, nil
}

func (m mockedOrganizations) DescribeOrganizationWithContext(ctx aws.Context, org *organizations.DescribeOrganizationInput, opts ...request.Option) (*organizations.DescribeOrganizationOutput, error) {
	return &organizations.DescribeOrganizationOutput{
		Organization: &organizations.Organization{
			MasterAccountId: mockAccount(0).Id,
		},
	}, nil
}

func (m mockedOrganizations) ListRoots(org *organizations.ListRootsInput) (*organizations.ListRootsOutput, error) {
	return &organizations.ListRootsOutput{
		Roots: []*organizations.Root{
			{
				Id:   aws.String("r-0000"),
				Name: aws.String("root"),
			},
		},
	}, nil
}

func (m *mockedOrganizations) ListAccountsForParentPagesWithContext(ctx aws.Context, input *organizations.ListAccountsForParentInput, fn func(*organizations.ListAccountsForParentOutput, bool) bool, opts ...request.Option) error {
	return m.ListAccountsForParentPagesWithContextFunc(ctx, input, fn, opts...)
}

func (m *mockedOrganizations) ListAccountsForParentPagesWithContextFunc(ctx aws.Context, input *organizations.ListAccountsForParentInput, fn func(*organizations.ListAccountsForParentOutput, bool) bool, opts ...request.Option) error {
	if aws.StringValue(input.ParentId) == "1ou" {
		fn(&organizations.ListAccountsForParentOutput{
			Accounts: []*organizations.Account{
				mockAccount(1),
				mockAccount(2),
			},
		}, true)
	}

	return nil
}

func (m *mockedOrganizations) ListOrganizationalUnitsForParentPagesWithContext(ctx aws.Context, input *organizations.ListOrganizationalUnitsForParentInput, fn func(*organizations.ListOrganizationalUnitsForParentOutput, bool) bool, opts ...request.Option) error {
	return m.ListOrganizationalUnitsForParentPagesWithContextFunc(ctx, input, fn, opts...)
}

func (m *mockedOrganizations) ListOrganizationalUnitsForParentPagesWithContextFunc(ctx aws.Context, input *organizations.ListOrganizationalUnitsForParentInput, fn func(*organizations.ListOrganizationalUnitsForParentOutput, bool) bool, opts ...request.Option) error {
	fn(&organizations.ListOrganizationalUnitsForParentOutput{
		OrganizationalUnits: []*organizations.OrganizationalUnit{},
	}, true)

	return nil
}

func (m *mockedOrganizations) ListTagsForResourcePagesWithContext(ctx aws.Context, input *organizations.ListTagsForResourceInput, fn func(*organizations.ListTagsForResourceOutput, bool) bool, opts ...request.Option) error {
	return m.ListTagsForResourcePagesWithContextFunc(ctx, input, fn, opts...)
}

func (m *mockedOrganizations) ListTagsForResourcePagesWithContextFunc(ctx aws.Context, input *organizations.ListTagsForResourceInput, fn func(*organizations.ListTagsForResourceOutput, bool) bool, opts ...request.Option) error {
	fn(&organizations.ListTagsForResourceOutput{}, true)

	return nil
}
