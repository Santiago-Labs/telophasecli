package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/resourceoperation"
	"golang.org/x/sync/errgroup"

	"github.com/spf13/cobra"
)

var (
	tag                string
	targets            string
	stacks             string
	allowDeleteAccount bool

	// TUI
	useTUI bool
)

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringVar(&stacks, "stacks", "", "Filter stacks to deploy")
	deployCmd.Flags().StringVar(&tag, "tag", "", "Filter accounts and organization units to deploy with a comma separated list")
	deployCmd.Flags().StringVar(&targets, "targets", "", "Filter resource types to deploy. Options: organization, scp, stacks")
	deployCmd.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	deployCmd.Flags().BoolVar(&useTUI, "tui", false, "use the TUI for deploy")
	deployCmd.Flags().BoolVar(&allowDeleteAccount, "allow-account-delete", false, "Allow closing an AWS account")
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy - Deploy a CDK and/or TF stacks to your AWS account(s). Accounts in organization.yml will be created if they do not exist.",
	Run: func(cmd *cobra.Command, args []string) {

		if err := validateTargets(); err != nil {
			log.Fatal("error validating targets err:", err)
		}
		var consoleUI runner.ConsoleUI
		parsedTargets := filterEmptyStrings(strings.Split(targets, ","))
		var g errgroup.Group

		if useTUI {
			consoleUI = runner.NewTUI()
			g.Go(func() error {
				return ProcessOrgEndToEnd(consoleUI, resourceoperation.Deploy, parsedTargets)
			})
		} else {
			consoleUI = runner.NewSTDOut()
			if err := ProcessOrgEndToEnd(consoleUI, resourceoperation.Deploy, parsedTargets); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		consoleUI.Start()
		if err := g.Wait(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}
