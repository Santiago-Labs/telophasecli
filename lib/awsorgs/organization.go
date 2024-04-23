package awsorgs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/organizations/organizationsiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awssess"
	"github.com/santiago-labs/telophasecli/resource"
)

type Client struct {
	organizationClient organizationsiface.OrganizationsAPI
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

func (c Client) FetchManagementAccount(ctx context.Context) (*resource.Account, error) {
	org, err := c.organizationClient.DescribeOrganization(&organizations.DescribeOrganizationInput{})
	if err != nil {
		return nil, fmt.Errorf("DescribeOrganization: %s", err)
	}

	// The management account ID is available in the Organization.MasterAccountId field.
	managementAccountID := *org.Organization.MasterAccountId

	// Fetch the details of the management account.
	var managementAccount *organizations.Account
	accounts, err := c.CurrentAccounts(ctx)
	if err != nil {
		return nil, err
	}

	for _, account := range accounts {
		if *account.Id == managementAccountID {
			managementAccount = account
			break
		}
	}

	if managementAccount == nil {
		return nil, fmt.Errorf("management account with ID %s not found", managementAccountID)
	}

	return &resource.Account{
		Email:             *managementAccount.Email,
		AccountID:         managementAccountID,
		AccountName:       *managementAccount.Name,
		ManagementAccount: true,
	}, nil
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

func (c Client) MoveAccount(
	ctx context.Context,
	consoleUI runner.ConsoleUI,
	mgmtAcct resource.Account,
	acctId, oldParentId, newParentId string,
) error {
	if oldParentId == newParentId {
		return nil
	}
	consoleUI.Print(fmt.Sprintf("Moving Account: %s Old Parent: %s New Parent: %s\n", acctId, oldParentId, newParentId), mgmtAcct)
	_, err := c.organizationClient.MoveAccountWithContext(ctx, &organizations.MoveAccountInput{
		AccountId:           &acctId,
		DestinationParentId: &newParentId,
		SourceParentId:      &oldParentId,
	})

	if err == nil {
		consoleUI.Print(fmt.Sprintf("Successfully moved account: Account: %s Old Parent=%s New Parent=%s\n", acctId, oldParentId, newParentId), mgmtAcct)
		return nil
	}

	consoleUI.Print(fmt.Sprintf("Error moving account: Account: %s err: %v", acctId, err), mgmtAcct)
	return err
}

func (c Client) CreateOrganizationUnit(
	ctx context.Context,
	consoleUI runner.ConsoleUI,
	mgmtAcct resource.Account,
	ouName, newParentId string,
) (*organizations.OrganizationalUnit, error) {
	consoleUI.Print(fmt.Sprintf("Creating OU: Name=%s\n", ouName), mgmtAcct)
	out, err := c.organizationClient.CreateOrganizationalUnitWithContext(ctx, &organizations.CreateOrganizationalUnitInput{
		Name:     &ouName,
		ParentId: &newParentId,
	})
	if err != nil {
		return nil, err
	}

	return out.OrganizationalUnit, nil
}

func (c Client) RecreateOU(
	ctx context.Context,
	consoleUI runner.ConsoleUI,
	mgmtAcct resource.Account,
	ouID, ouName, newParentId string,
) error {
	newOU, err := c.CreateOrganizationUnit(ctx, consoleUI, mgmtAcct, ouName, newParentId)
	if err != nil {
		return err
	}

	childAccounts, err := c.ListAccountsForParent(ouID)
	if err != nil {
		return err
	}
	for _, acct := range childAccounts {
		err := c.MoveAccount(ctx, consoleUI, mgmtAcct, *acct.Id, ouID, *newOU.Id)
		if err != nil {
			return err
		}
	}

	childOUs, err := c.ListOrganizationalUnits(ouID)
	if err != nil {
		return err
	}
	for _, childOU := range childOUs {
		err := c.RecreateOU(ctx, consoleUI, mgmtAcct, *childOU.Id, *childOU.Name, *newOU.Id)
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

func (c Client) CreateAccount(
	ctx context.Context,
	consoleUI runner.ConsoleUI,
	mgmtAcct resource.Account,
	acct *organizations.Account,
) (string, error) {
	consoleUI.Print(fmt.Sprintf("Creating Account: Name=%s Email=%s\n", *acct.Name, *acct.Email), mgmtAcct)
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
		consoleUI.Print(fmt.Sprintf("Error creating account: %s.\n", err.Error()), mgmtAcct)
		return "", err
	}

	for {

		requestId := *out.CreateAccountStatus.Id
		currStatus, err := c.organizationClient.DescribeCreateAccountStatus(&organizations.DescribeCreateAccountStatusInput{
			CreateAccountRequestId: &requestId,
		})
		if err != nil {
			consoleUI.Print(fmt.Sprintf("Error fetching create status: %s\n", err), mgmtAcct)
			continue
		}

		state := *currStatus.CreateAccountStatus.State
		accountName := *currStatus.CreateAccountStatus.AccountName

		switch state {
		case "IN_PROGRESS":
			consoleUI.Print(fmt.Sprintf("Still creating %s...\n", accountName), mgmtAcct)
		case "FAILED":
			consoleUI.Print(fmt.Sprintf("Failed to create account %s. Error: %s\n", accountName, *currStatus.CreateAccountStatus.FailureReason), mgmtAcct)
			return "", err

		case "SUCCEEDED":
			consoleUI.Print(fmt.Sprintf("Successfully created account %s.\n", accountName), mgmtAcct)
			return *currStatus.CreateAccountStatus.AccountId, nil
		default:
			return "", fmt.Errorf("unexpected state: %s", state)
		}

		time.Sleep(5 * time.Second)
	}
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
		return "", oops.Wrapf(err, "organizaqtions.ListRootsInput, make sure you have access to organizations from this role")
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

	return accounts, oops.Wrapf(err, "organizations.ListAccountsForParent")
}

func (c Client) FetchOUAndDescendents(ctx context.Context, ouID, mgmtAccountID string) (resource.OrganizationUnit, error) {
	var ou resource.OrganizationUnit

	var providerOU *organizations.OrganizationalUnit

	// AWS does not represent the root as an OU, but we do for simplicity.
	if strings.HasPrefix(ouID, "r-") {
		name := "root"
		providerOU = &organizations.OrganizationalUnit{
			Id:   &ouID,
			Name: &name,
		}
	} else {
		var err error
		providerOU, err = c.GetOrganizationUnit(ctx, ouID)
		if err != nil {
			return ou, err
		}
	}

	ou.OUID = &ouID
	ou.OUName = *providerOU.Name

	groupAccounts, err := c.CurrentAccountsForParent(ctx, *ou.OUID)
	if err != nil {
		return ou, err
	}

	for _, providerAcct := range groupAccounts {
		acct := resource.Account{
			AccountID:   *providerAcct.Id,
			Email:       *providerAcct.Email,
			Parent:      &ou,
			AccountName: *providerAcct.Name,
		}
		if *providerAcct.Id == mgmtAccountID {
			acct.ManagementAccount = true
		}
		ou.Accounts = append(ou.Accounts, &acct)
	}

	children, err := c.GetOrganizationUnitChildren(ctx, ouID)
	if err != nil {
		return ou, err
	}

	for _, providerChild := range children {
		child, err := c.FetchOUAndDescendents(ctx, *providerChild.Id, mgmtAccountID)
		if err != nil {
			return ou, err
		}
		child.Parent = &ou
		ou.ChildOUs = append(ou.ChildOUs, &child)
	}

	return ou, nil
}
