package awsorgs

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/santiago-labs/telophasecli/lib/telophase"
)

type Client struct {
	organizationClient *organizations.Organizations
}

func New() Client {
	sess := session.Must(session.NewSession())
	orgsClient := organizations.New(sess)

	stsClient := sts.New(sess)
	_, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "UnrecognizedClientException", "InvalidClientTokenId", "AccessDenied":
				fmt.Println("Error fetching caller identity. Ensure your awscli credentials are valid.\nError:", awsErr.Message())
				panic(err)
			}
		}
	}
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
	if err != nil {
		return nil, fmt.Errorf("ListAccounts: are you using the right AWS role? err: %s", err)
	}

	return accounts, nil
}

func (c Client) CreateAccounts(ctx context.Context, accts []*organizations.Account) []error {
	var errs []error
	var createRequests []*organizations.CreateAccountStatus
	for _, acct := range accts {
		fmt.Printf("Creating Account: Name=%s Email=%s\n", *acct.Name, *acct.Email)
		out, err := c.organizationClient.CreateAccount(&organizations.CreateAccountInput{
			AccountName: acct.Name,
			Email:       acct.Email,
			Tags: []*organizations.Tag{
				{
					Key:   aws.String("TelophaseManaged"),
					Value: aws.String("true"),
				},
			},
		})
		if err != nil {
			fmt.Printf("Error creating account: %s.\n", err.Error())
			errs = append(errs, err)
		}
		createRequests = append(createRequests, out.CreateAccountStatus)
	}

	requestsInProgress := len(createRequests)
	for requestsInProgress > 0 {
		time.Sleep(15 * time.Second)

		for _, request := range createRequests {
			requestId := *request.Id
			currStatus, err := c.organizationClient.DescribeCreateAccountStatus(&organizations.DescribeCreateAccountStatusInput{
				CreateAccountRequestId: &requestId,
			})
			if err != nil {
				fmt.Printf("error fetching create status: %s\n", err)
				continue
			}

			state := *currStatus.CreateAccountStatus.State
			accountName := *currStatus.CreateAccountStatus.AccountName

			switch state {
			case "IN_PROGRESS":
				fmt.Printf("Still creating %s...\n", accountName)
			case "FAILED":
				fmt.Printf("Failed to create account %s. Error: %s\n", accountName, *currStatus.CreateAccountStatus.FailureReason)
				requestsInProgress -= 1

			case "SUCCEEDED":
				fmt.Printf("Successfully created account %s.\n", accountName)
				requestsInProgress -= 1
				telophase.UpsertAccount(*currStatus.CreateAccountStatus.AccountId, accountName)
			}
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
