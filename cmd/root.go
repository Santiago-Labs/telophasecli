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

func processOrgEndToEnd(consoleUI runner.ConsoleUI, cmd int) {
	ctx := context.Background()
	orgClient := awsorgs.New()
	mgmtAcct, err := orgClient.FetchManagementAccount(ctx)
	if err != nil {
		panic(err)
	}

	var accountsToApply []resource.Account
	rootAWSGroup, err := ymlparser.ParseOrganizationV2(orgFile)
	if err != nil {
		panic(fmt.Sprintf("error: %s", err))
	}
	orgV2Diff(ctx, consoleUI, orgClient, rootAWSGroup, mgmtAcct, cmd)

	if rootAWSGroup != nil {
		for _, acct := range rootAWSGroup.AllDescendentAccounts() {
			if contains(tag, acct.AllTags()) || tag == "" {
				accountsToApply = append(accountsToApply, *acct)
			}
		}
	}

	if len(accountsToApply) == 0 {
		consoleUI.Print("No accounts to deploy.", *mgmtAcct)
	}

	runIAC(ctx, consoleUI, cmd, accountsToApply)

	scpOps := resourceoperation.CollectSCPOps(ctx, orgClient, consoleUI, cmd, rootAWSGroup, mgmtAcct)
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
