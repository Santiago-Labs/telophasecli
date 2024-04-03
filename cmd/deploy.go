package cmd

import (
	"context"
	"fmt"

	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
	"github.com/santiago-labs/telophasecli/resource"
	"github.com/santiago-labs/telophasecli/resourceoperation"

	"github.com/spf13/cobra"
)

var (
	tag    string
	stacks string

	// TUI
	useTUI bool
)

func init() {
	rootCmd.AddCommand(compileCmd)
	compileCmd.Flags().StringVar(&stacks, "stacks", "", "Filter stacks to deploy")
	compileCmd.Flags().StringVar(&tag, "tag", "", "Filter accounts and account groups to deploy")
	compileCmd.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	compileCmd.Flags().BoolVar(&useTUI, "tui", false, "use the TUI for deploy")
}

var compileCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy - Deploy a CDK and/or TF stacks to your AWS account(s). Accounts in organization.yml will be created if they do not exist.",
	Run: func(cmd *cobra.Command, args []string) {
		orgClient := awsorgs.New()
		ctx := context.Background()

		var consoleUI runner.ConsoleUI
		if useTUI {
			consoleUI = runner.NewTUI()
		} else {
			consoleUI = runner.NewSTDOut()
		}

		var accountsToApply []resource.Account
		rootAWSGroup, err := ymlparser.ParseOrganizationV2(orgFile)
		if err != nil {
			panic(fmt.Sprintf("error: %s", err))
		}

		ops := orgV2Diff(ctx, consoleUI, orgClient, rootAWSGroup, resourceoperation.Deploy)
		for _, op := range ops {
			op.Call(ctx)
		}

		if rootAWSGroup != nil {
			for _, acct := range rootAWSGroup.AllDescendentAccounts() {
				if contains(tag, acct.AllTags()) || tag == "" {
					accountsToApply = append(accountsToApply, *acct)
				}
			}
		}

		if len(accountsToApply) == 0 {
			fmt.Println("No accounts to deploy")
		}

		runIAC(ctx, consoleUI, resourceoperation.Deploy, accountsToApply)

		scpOps := resourceoperation.CollectSCPOps(ctx, orgClient, consoleUI, resourceoperation.Diff, rootAWSGroup)
		for _, op := range scpOps {
			op.Call(ctx)
		}
	},
}
