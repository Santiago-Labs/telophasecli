package awsorgs

import (
	"context"
	"fmt"
	"telophasecli/lib/ymlparser"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/organizations"
)

type Client struct {
	organizationClient *organizations.Organizations
}

func New() Client {
	orgsClient := organizations.New(session.Must(session.NewSession()))
	return Client{
		organizationClient: orgsClient,
	}
}

// CurrentAccounts fetches all the current accounts owned in the current OU.
func (c Client) CurrentAccounts(ctx context.Context) ([]*organizations.Account, error) {
	var accounts []*organizations.Account

	err := c.organizationClient.ListAccountsPagesWithContext(ctx, &organizations.ListAccountsInput{},
		func(page *organizations.ListAccountsOutput, lastPage bool) bool {
			accounts = append(accounts, page.Accounts...)
			return !lastPage
		},
	)

	return accounts, err
}

func (c Client) CreateAccounts(ctx context.Context, accts []ymlparser.Account) []error {
	var errs []error
	for _, acct := range accts {
		fmt.Printf("Creating Account: %s Email: %s\n", acct.AccountName, acct.Email)
		_, err := c.organizationClient.CreateAccount(&organizations.CreateAccountInput{
			AccountName: &acct.AccountName,
			Email:       &acct.Email,
			Tags: []*organizations.Tag{
				{
					Key:   aws.String("TelophaseManaged"),
					Value: aws.String("true"),
				},
			},
		})
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (c Client) CloseAccounts(ctx context.Context, accts []*organizations.Account) []error {
	var errs []error
	for _, acct := range accts {
		fmt.Printf("Closing Account: %s Email: %s\n", *acct.Name, *acct.Email)
		_, err := c.organizationClient.CloseAccountWithContext(ctx, &organizations.CloseAccountInput{
			AccountId: acct.Id,
		})
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
