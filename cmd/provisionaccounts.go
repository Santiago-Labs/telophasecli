package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
	"github.com/santiago-labs/telophasecli/resource"
	"github.com/santiago-labs/telophasecli/resourceoperation"
)

var orgFile string

func init() {
	rootCmd.AddCommand(accountProvision)
	accountProvision.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	accountProvision.Flags().BoolVar(&useTUI, "tui", false, "use the TUI for diff")
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

		var consoleUI runner.ConsoleUI
		if useTUI {
			consoleUI = runner.NewTUI()
			go processOrg(consoleUI, args[0])
		} else {
			consoleUI = runner.NewSTDOut()
			processOrg(consoleUI, args[0])
		}
		consoleUI.Start()

	},
}

func processOrg(consoleUI runner.ConsoleUI, cmd string) {
	orgClient := awsorgs.New()
	ctx := context.Background()

	if cmd == "import" {
		mgmtAcct, err := orgClient.FetchManagementAccount(ctx)
		if err != nil {
			consoleUI.Print(fmt.Sprintf("Error: %v", err), *mgmtAcct)
			return
		}
		consoleUI.Print("Importing AWS Organization", *mgmtAcct)
		if err := importOrgV2(ctx, consoleUI, orgClient, mgmtAcct); err != nil {
			consoleUI.Print(fmt.Sprintf("error importing organization: %s", err), *mgmtAcct)
		}
	}

	rootAWSOU, err := ymlparser.NewParser(orgClient).ParseOrganizationV2(ctx, orgFile)
	if err != nil {
		consoleUI.Print(fmt.Sprintf("error parsing organization: %s", err), resource.Account{AccountID: "error", AccountName: "error"})
	}

	mgmtAcct, err := resolveMgmtAcct(ctx, orgClient, rootAWSOU)
	if err != nil {
		consoleUI.Print(fmt.Sprintf("Could not fetch AWS Management Account: %s", err), resource.Account{AccountID: "error", AccountName: "error"})
		return
	}

	if cmd == "diff" {
		consoleUI.Print("Diffing AWS Organization", *mgmtAcct)
		orgOps := resourceoperation.CollectOrganizationUnitOps(
			ctx, consoleUI, orgClient, mgmtAcct, rootAWSOU, resourceoperation.Diff,
		)
		for _, op := range resourceoperation.FlattenOperations(orgOps) {
			consoleUI.Print(op.ToString(), *mgmtAcct)
		}
		if len(orgOps) == 0 {
			consoleUI.Print("\033[32m No changes to AWS Organization. \033[0m", *mgmtAcct)
		}
	}

	if cmd == "deploy" {
		consoleUI.Print("Diffing AWS Organization", *mgmtAcct)
		orgOps := resourceoperation.CollectOrganizationUnitOps(
			ctx, consoleUI, orgClient, mgmtAcct, rootAWSOU, resourceoperation.Deploy,
		)

		for _, op := range resourceoperation.FlattenOperations(orgOps) {
			consoleUI.Print(op.ToString(), *mgmtAcct)
		}
		if len(orgOps) == 0 {
			consoleUI.Print("\033[32m No changes to AWS Organization. \033[0m", *mgmtAcct)
		}
		for _, op := range orgOps {
			err := op.Call(ctx)
			if err != nil {
				consoleUI.Print(fmt.Sprintf("Error: %v", err), *mgmtAcct)
				return
			}
		}
	}

	consoleUI.Print("Done.\n", *mgmtAcct)
}

func importOrgV2(ctx context.Context, consoleUI runner.ConsoleUI, orgClient awsorgs.Client, mgmtAcct *resource.Account) error {

	rootId, err := orgClient.GetRootId()
	if err != nil {
		return err
	}
	if rootId == "" {
		return fmt.Errorf("no root ID found")
	}

	rootOU, err := orgClient.FetchOUAndDescendents(ctx, rootId, mgmtAcct.AccountID)
	if err != nil {
		return err
	}
	org := resource.OrganizationUnit{
		OUName:   rootOU.OUName,
		ChildOUs: rootOU.ChildOUs,
		Accounts: rootOU.Accounts,
	}

	if err := ymlparser.WriteOrgV2File(orgFile, &org); err != nil {
		return err
	}

	consoleUI.Print(fmt.Sprintf("Successfully wrote file to: %s", orgFile), *mgmtAcct)
	return nil
}
