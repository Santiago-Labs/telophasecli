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
	"github.com/santiago-labs/telophasecli/lib/awssess"
	"github.com/santiago-labs/telophasecli/lib/telophase"
)

type Client struct {
	organizationClient *organizations.Organizations
}

func New() Client {
	sess := session.Must(awssess.DefaultSession())
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

// CurrentAccounts fetches all accounts in the organization.
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

func (c Client) CurrentAccountsForParent(ctx context.Context, parentID string) ([]*organizations.Account, error) {
	var accounts []*organizations.Account

	err := c.organizationClient.ListAccountsForParentPagesWithContext(ctx, &organizations.ListAccountsForParentInput{
		ParentId: &parentID,
	},
		func(page *organizations.ListAccountsForParentOutput, lastPage bool) bool {
			accounts = append(accounts, page.Accounts...)
			return !lastPage
		},
	)
	if err != nil {
		return nil, fmt.Errorf("ListAccounts: are you using the right AWS role? err: %s", err)
	}

	return accounts, nil
}

func (c Client) CurrentOUsForParent(ctx context.Context, parentID string) ([]*organizations.OrganizationalUnit, error) {
	var accounts []*organizations.OrganizationalUnit

	err := c.organizationClient.ListOrganizationalUnitsForParentPagesWithContext(ctx, &organizations.ListOrganizationalUnitsForParentInput{
		ParentId: &parentID,
	},
		func(page *organizations.ListOrganizationalUnitsForParentOutput, lastPage bool) bool {
			accounts = append(accounts, page.OrganizationalUnits...)
			return !lastPage
		},
	)
	if err != nil {
		return nil, err
	}

	return accounts, nil
}

func (c Client) GetOrganizationUnit(ctx context.Context, OUId string) (*organizations.OrganizationalUnit, error) {
	out, err := c.organizationClient.DescribeOrganizationalUnitWithContext(ctx, &organizations.DescribeOrganizationalUnitInput{
		OrganizationalUnitId: &OUId,
	})
	if err != nil {
		return nil, fmt.Errorf("GetOrganizationalUnit: %s", err)
	}

	return out.OrganizationalUnit, nil
}

func (c Client) GetOrganizationUnitChildren(ctx context.Context, OUId string) ([]*organizations.OrganizationalUnit, error) {
	var childOUs []*organizations.OrganizationalUnit

	err := c.organizationClient.ListOrganizationalUnitsForParentPagesWithContext(ctx, &organizations.ListOrganizationalUnitsForParentInput{
		ParentId: &OUId,
	},
		func(page *organizations.ListOrganizationalUnitsForParentOutput, lastPage bool) bool {
			childOUs = append(childOUs, page.OrganizationalUnits...)
			return !lastPage
		},
	)
	if err != nil {
		return nil, fmt.Errorf("GetOrganizationUnitChildren: %s", err)
	}

	return childOUs, nil
}

func (c Client) MoveAccount(ctx context.Context, acctId, oldParentId, newParentId string) error {
	fmt.Printf("Moving Account: Account ID=%s Old Parent=%s New Parent=%s\n", acctId, oldParentId, newParentId)
	_, err := c.organizationClient.MoveAccountWithContext(ctx, &organizations.MoveAccountInput{
		AccountId:           &acctId,
		DestinationParentId: &newParentId,
		SourceParentId:      &oldParentId,
	})

	return err
}

func (c Client) CreateOrganizationUnit(ctx context.Context, ouName, newParentId string) (*organizations.OrganizationalUnit, error) {
	fmt.Printf("Creating OU: Name=%s\n", ouName)
	out, err := c.organizationClient.CreateOrganizationalUnitWithContext(ctx, &organizations.CreateOrganizationalUnitInput{
		Name:     &ouName,
		ParentId: &newParentId,
	})
	if err != nil {
		return nil, err
	}

	return out.OrganizationalUnit, nil
}

func (c Client) RecreateOU(ctx context.Context, ouID, ouName, newParentId string) error {
	newOU, err := c.CreateOrganizationUnit(ctx, ouName, newParentId)
	if err != nil {
		return err
	}

	childAccounts, err := c.ListAccountsForParent(ouID)
	if err != nil {
		return err
	}
	for _, acct := range childAccounts {
		err := c.MoveAccount(ctx, *acct.Id, ouID, *newOU.Id)
		if err != nil {
			return err
		}
	}

	childOUs, err := c.ListOrganizationalUnits(ouID)
	if err != nil {
		return err
	}
	for _, childOU := range childOUs {
		err := c.RecreateOU(ctx, *childOU.Id, *childOU.Name, *newOU.Id)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c Client) UpdateOrganizationUnit(ctx context.Context, ouID, newName string) error {
	_, err := c.organizationClient.UpdateOrganizationalUnitWithContext(ctx,
		&organizations.UpdateOrganizationalUnitInput{
			Name:                 aws.String(newName),
			OrganizationalUnitId: aws.String(ouID),
		})

	return err
}

func (c Client) CreateAccounts(ctx context.Context, accts []*organizations.Account) []error {
	var errs []error
	var createRequests []*organizations.CreateAccountStatus
	requestToAccount := make(map[string]*organizations.Account)

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
		requestToAccount[*out.CreateAccountStatus.Id] = acct
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
				requestToAccount[requestId].Id = currStatus.CreateAccountStatus.AccountId
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

func (c Client) GetRootId() (string, error) {
	rootsOutput, err := c.organizationClient.ListRoots(&organizations.ListRootsInput{})
	if err != nil {
		return "", err
	}
	if len(rootsOutput.Roots) > 0 {
		return *rootsOutput.Roots[0].Id, nil
	}
	return "", nil
}

func (c Client) ListOrganizationalUnits(parentID string) ([]*organizations.OrganizationalUnit, error) {
	var OUs []*organizations.OrganizationalUnit
	err := c.organizationClient.ListOrganizationalUnitsForParentPages(&organizations.ListOrganizationalUnitsForParentInput{
		ParentId: &parentID,
	}, func(page *organizations.ListOrganizationalUnitsForParentOutput, lastPage bool) bool {
		OUs = append(OUs, page.OrganizationalUnits...)
		return !lastPage
	})
	return OUs, err
}

func (c Client) ListAccountsForParent(parentID string) ([]*organizations.Account, error) {
	var accounts []*organizations.Account
	err := c.organizationClient.ListAccountsForParentPages(&organizations.ListAccountsForParentInput{
		ParentId: &parentID,
	}, func(page *organizations.ListAccountsForParentOutput, lastPage bool) bool {
		accounts = append(accounts, page.Accounts...)
		return !lastPage
	})

	return accounts, err
}
