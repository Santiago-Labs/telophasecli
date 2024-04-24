package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/metrics"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
	"github.com/santiago-labs/telophasecli/resource"
	"github.com/santiago-labs/telophasecli/resourceoperation"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "telophasecli",
	Short: "telophasecli - Command line interface for Telophase",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(os.Stderr, "Please pass in a command. See more with -h\n")
		os.Exit(1)
	},
}

func Execute() {
	metrics.Init()
	metrics.RegisterCommand()
	defer metrics.Close()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}

func ProcessOrgEndToEnd(consoleUI runner.ConsoleUI, cmd int, targets []string) {
	ctx := context.Background()
	orgClient := awsorgs.New()
	rootAWSOU, err := ymlparser.NewParser(orgClient).ParseOrganization(ctx, orgFile)
	if err != nil {
		consoleUI.Print(fmt.Sprintf("error: %s", err), resource.Account{AccountID: "error", AccountName: "error"})
		return
	}

	if rootAWSOU == nil {
		consoleUI.Print("Could not parse AWS Organization", resource.Account{AccountID: "error", AccountName: "error"})
		return
	}

	mgmtAcct, err := resolveMgmtAcct(ctx, orgClient, rootAWSOU)
	if err != nil {
		consoleUI.Print(fmt.Sprintf("Could not fetch AWS Management Account: %s", err), resource.Account{AccountID: "error", AccountName: "error"})
		return
	}

	var deployStacks bool
	var deploySCP bool
	var deployOrganization bool

	for _, target := range targets {
		if strings.ReplaceAll(target, " ", "") == "stacks" {
			deployStacks = true
		}
		if strings.ReplaceAll(target, " ", "") == "scp" {
			deploySCP = true
		}
		if strings.ReplaceAll(target, " ", "") == "organization" {
			deployOrganization = true
		}
	}

	if len(targets) == 0 || deployOrganization {
		orgOps := resourceoperation.CollectOrganizationUnitOps(
			ctx, consoleUI, orgClient, mgmtAcct, rootAWSOU, cmd,
		)
		for _, op := range resourceoperation.FlattenOperations(orgOps) {
			consoleUI.Print(op.ToString(), *mgmtAcct)
		}
		if len(orgOps) == 0 {
			consoleUI.Print("\033[32m No changes to AWS Organization. \033[0m", *mgmtAcct)
		}

		if cmd == resourceoperation.Deploy {
			for _, op := range orgOps {
				err := op.Call(ctx)
				if err != nil {
					consoleUI.Print(fmt.Sprintf("Error on AWS Organization Operation: %v", err), *mgmtAcct)
				}
			}
		}
	}

	if len(targets) == 0 || deployStacks {
		var accountsToApply []resource.Account
		for _, acct := range rootAWSOU.AllDescendentAccounts() {
			if contains(tag, acct.AllTags()) || tag == "" {
				accountsToApply = append(accountsToApply, *acct)
			}
		}

		if len(accountsToApply) == 0 {
			consoleUI.Print("No accounts to deploy.", *mgmtAcct)
		}

		runIAC(ctx, consoleUI, cmd, accountsToApply)
	}

	if len(targets) == 0 || deploySCP {
		// Telophasecli can be run from either the management account or
		// the delegated administrator account.
		var scpAdmin *resource.Account
		delegatedAdmin := rootAWSOU.DelegatedAdministrator()
		if delegatedAdmin != nil {
			scpAdmin = delegatedAdmin
		} else {
			scpAdmin = mgmtAcct
		}

		scpOps := resourceoperation.CollectSCPOps(ctx, orgClient, consoleUI, cmd, rootAWSOU, scpAdmin)
		for _, op := range scpOps {
			err := op.Call(ctx)
			if err != nil {
				consoleUI.Print(fmt.Sprintf("Error on SCP Operation: %v", err), *scpAdmin)
			}
		}

		if len(scpOps) == 0 {
			consoleUI.Print("No Service Control Policies to deploy.", *scpAdmin)
		}
	}
	consoleUI.Print("Done.\n", *mgmtAcct)
}

func validateTargets() error {
	if targets == "" {
		return nil
	}

	for _, target := range strings.Split(targets, ",") {
		strippedTarget := strings.ReplaceAll(target, " ", "")
		if strippedTarget != "stacks" && strippedTarget != "scp" && strippedTarget != "organization" {
			return fmt.Errorf("invalid targets: %s", targets)
		}
	}

	return nil
}

func resolveMgmtAcct(
	ctx context.Context,
	orgClient awsorgs.Client,
	rootAWSOU *resource.OrganizationUnit,
) (*resource.Account, error) {
	// We must have access to the management account so that we can create Accounts and OUs,
	// but the management account does not need to be managed by Telophase.
	parsedMgmtAcct := rootAWSOU.ManagementAccount()
	if parsedMgmtAcct != nil {
		return parsedMgmtAcct, nil
	}

	fetchedMgmtAcct, err := orgClient.FetchManagementAccount(ctx)
	if err != nil {
		return nil, err
	}
	return fetchedMgmtAcct, nil
}

func filterEmptyStrings(slice []string) []string {
	var result []string
	for _, str := range slice {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}
