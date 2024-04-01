package cmd

import (
	"context"
	"fmt"

	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/azureorgs"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
	"github.com/santiago-labs/telophasecli/resource"
	"github.com/santiago-labs/telophasecli/resourceoperation"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVar(&stacks, "stacks", "", "Filter stacks to deploy")
	diffCmd.Flags().StringVar(&tag, "tag", "", "Filter accounts and account groups to deploy.")
	diffCmd.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	diffCmd.Flags().BoolVar(&useTUI, "tui", false, "use the TUI for diff")
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "diff - Show accounts to create/update and CDK and/or TF changes for each account.",
	Run: func(cmd *cobra.Command, args []string) {
		orgClient := awsorgs.New()
		subsClient, err := azureorgs.New()
		if err != nil {
			panic(fmt.Sprintf("error: %s", err))
		}

		var consoleUI runner.ConsoleUI
		if useTUI {
			consoleUI = runner.NewTUI()
		} else {
			consoleUI = runner.NewSTDOut()
		}

		var accountsToApply []resource.Account

		ctx := context.Background()

		rootAWSGroup, rootAzureGroup, err := ymlparser.ParseOrganizationV2(orgFile)
		if err != nil {
			panic(fmt.Sprintf("error: %s parsing organization", err))
		}
		orgV2Diff(ctx, consoleUI, orgClient, *subsClient, rootAWSGroup, rootAzureGroup)

		if rootAWSGroup != nil {
			for _, acct := range rootAWSGroup.AllDescendentAccounts() {
				if contains(tag, acct.AllTags()) || tag == "" {
					accountsToApply = append(accountsToApply, *acct)
				}
			}
		}

		if rootAzureGroup != nil {
			for _, acct := range rootAzureGroup.AllDescendentAccounts() {
				if contains(tag, acct.AllTags()) || tag == "" {
					accountsToApply = append(accountsToApply, *acct)
				}
			}
		}

		if len(accountsToApply) == 0 {
			fmt.Println("No accounts to deploy")
		}

		runIAC(ctx, consoleUI, resourceoperation.Diff, accountsToApply)
	},
}
