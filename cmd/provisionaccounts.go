package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/spf13/cobra"

	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssess"
	"github.com/santiago-labs/telophasecli/lib/azureorgs"
	"github.com/santiago-labs/telophasecli/lib/telophase"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
)

var orgFile string
var azure bool

func init() {
	rootCmd.AddCommand(accountProvision)
	accountProvision.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
}

func connectionAzure() (azcore.TokenCredential, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return cred, nil
}

func isValidAccountArg(arg string) bool {
	switch arg {
	case "import":
		return true
	case "diff":
		return true
	case "deploy":
		return true
	default:
		return false
	}
}

var accountProvision = &cobra.Command{
	Use:   "account",
	Short: "account - Provision and import AWS accounts",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires at least one arg")
		}
		if isValidAccountArg(args[0]) {
			return nil
		}
		return fmt.Errorf("invalid color specified: %s", args[0])
	},
	Run: func(cmd *cobra.Command, args []string) {
		orgClient := awsorgs.New()
		subscriptionClient, err := azureorgs.New()
		if err != nil {
			panic(fmt.Sprintf("error creating azure client err: %s" + err.Error()))
		}

		ctx := context.Background()
		if args[0] == "import" {
			if err := importOrgV2(orgClient); err != nil {
				panic(fmt.Sprintf("error importing organization: %s", err))
			}
		}

		if args[0] == "diff" {
			_, err := orgV2Diff(orgClient, subscriptionClient)
			if err != nil {
				panic(fmt.Sprintf("error: %s", err))
			}
		}

		if args[0] == "deploy" {
			operations, err := orgV2Diff(orgClient, subscriptionClient)
			if err != nil {
				panic(fmt.Sprintf("error: %s", err))
			}

			for _, op := range operations {
				err := op.Call(ctx, orgClient)
				if err != nil {
					panic(fmt.Sprintf("error: %s", err))
				}
			}
		}
	},
}

func orgV2Diff(orgClient awsorgs.Client, subscriptionsClient *azureorgs.Client) (ops []ymlparser.ResourceOperation, err error) {
	org, azure, err := ymlparser.ParseOrganizationV2(orgFile)
	if err != nil {
		panic(fmt.Sprintf("error: %s parsing organization", err))
	}

	var operations []ymlparser.ResourceOperation
	if org != nil {
		operations = append(operations, org.Diff(orgClient)...)
		for _, op := range ymlparser.FlattenOperations(operations) {
			fmt.Println(op.ToString())
		}
		if len(operations) == 0 {
			fmt.Println("\033[32m No changes to AWS Organization. \033[0m")
		}
	}

	if azure != nil {
		azureOps, err := azure.Diff(subscriptionsClient)
		if err != nil {
			return nil, err
		}
		for _, op := range ymlparser.FlattenOperations(azureOps) {
			fmt.Println(op.ToString())
		}
		operations = append(operations, azureOps...)

		if len(azureOps) == 0 {
			fmt.Println("\033[32m No changes to Azure Subscriptions. \033[0m")
		}
	}

	return operations, nil
}

func currentAccountID() (string, error) {
	stsClient := sts.New(session.Must(awssess.DefaultSession()))
	caller, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}

	return *caller.Account, nil
}

func importOrgV2(orgClient awsorgs.Client) error {
	accounts, err := orgClient.CurrentAccounts(context.Background())
	if err != nil {
		return fmt.Errorf("error: %s getting current accounts", err)
	}

	managingAccountID, err := currentAccountID()
	if err != nil {
		return err
	}

	for _, acct := range accounts {
		telophase.UpsertAccount(*acct.Id, *acct.Name)
	}

	rootId, err := orgClient.GetRootId()
	if err != nil {
		return err
	}
	if rootId == "" {
		return fmt.Errorf("no root ID found")
	}

	rootGroup, err := ymlparser.FetchGroupAndDescendents(context.TODO(), orgClient, rootId, managingAccountID)
	if err != nil {
		return err
	}
	org := ymlparser.AccountGroup{
		Name:        rootGroup.Name,
		ChildGroups: rootGroup.ChildGroups,
		Accounts:    rootGroup.Accounts,
	}

	if err := ymlparser.WriteOrgV2File(orgFile, &org); err != nil {
		return err
	}

	return nil
}
