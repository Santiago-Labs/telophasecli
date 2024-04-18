package cmd

import (
	"fmt"
	"strings"

	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/resourceoperation"

	"github.com/spf13/cobra"
)

var (
	tag     string
	targets string
	stacks  string

	// TUI
	useTUI bool
)

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVar(&stacks, "stacks", "", "Filter stacks to deploy")
	deployCmd.Flags().StringVar(&tag, "tag", "", "Filter accounts and organization units to deploy")
	deployCmd.Flags().StringVar(&targets, "targets", "", "Filter resource types to deploy. Options: organization, scp, stacks")
	deployCmd.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	deployCmd.Flags().BoolVar(&useTUI, "tui", false, "use the TUI for deploy")
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy - Deploy a CDK and/or TF stacks to your AWS account(s). Accounts in organization.yml will be created if they do not exist.",
	Run: func(cmd *cobra.Command, args []string) {

		if err := validateTargets(); err != nil {
			fmt.Println(err)
			return
		}
		var consoleUI runner.ConsoleUI
		if useTUI {
			consoleUI = runner.NewTUI()
			go ProcessOrgEndToEnd(consoleUI, resourceoperation.Deploy, strings.Split(targets, ","))
		} else {
			consoleUI = runner.NewSTDOut()
			ProcessOrgEndToEnd(consoleUI, resourceoperation.Deploy, strings.Split(targets, ","))
		}

		consoleUI.Start()

	},
}
