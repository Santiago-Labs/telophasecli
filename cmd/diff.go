package cmd

import (
	"github.com/santiago-labs/telophasecli/cmd/runner"
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

		var consoleUI runner.ConsoleUI
		if useTUI {
			consoleUI = runner.NewTUI()
			go processOrgEndToEnd(consoleUI, resourceoperation.Diff)
		} else {
			consoleUI = runner.NewSTDOut()
			processOrgEndToEnd(consoleUI, resourceoperation.Diff)
		}

		consoleUI.Start()
	},
}
