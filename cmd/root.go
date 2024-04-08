package cmd

import (
	"context"
	"fmt"
	"os"

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

func ProcessOrgEndToEnd(consoleUI runner.ConsoleUI, cmd int) {
	ctx := context.Background()
	orgClient := awsorgs.New()
	mgmtAcct, err := orgClient.FetchManagementAccount(ctx)
	if err != nil {
		consoleUI.Print(fmt.Sprintf("Error: %v", err), resource.Account{})
		return
	}

	rootAWSOU, err := ymlparser.ParseOrganizationV2(orgFile)
	if err != nil {
		consoleUI.Print(fmt.Sprintf("error: %s", err), *mgmtAcct)
		return
	}

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

	var accountsToApply []resource.Account
	if rootAWSOU != nil {
		for _, acct := range rootAWSOU.AllDescendentAccounts() {
			if contains(tag, acct.AllTags()) || tag == "" {
				accountsToApply = append(accountsToApply, *acct)
			}
		}
	}

	if len(accountsToApply) == 0 {
		consoleUI.Print("No accounts to deploy.", *mgmtAcct)
	}

	runIAC(ctx, consoleUI, cmd, accountsToApply)

	scpOps := resourceoperation.CollectSCPOps(ctx, orgClient, consoleUI, cmd, rootAWSOU, mgmtAcct)
	for _, op := range scpOps {
		err := op.Call(ctx)
		if err != nil {
			consoleUI.Print(fmt.Sprintf("Error on SCP Operation: %v", err), *mgmtAcct)
		}
	}

	if len(scpOps) == 0 {
		consoleUI.Print("No Service Control Policies to deploy.", *mgmtAcct)
	}
	consoleUI.Print("Done.\n", *mgmtAcct)
}
