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

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVar(&stacks, "stacks", "", "Filter stacks to deploy")
	diffCmd.Flags().StringVar(&tag, "tag", "", "Filter accounts and organization units to deploy.")
	diffCmd.Flags().StringVar(&targets, "targets", "", "Filter resource types to deploy. Options: organization, scp, stacks")
	diffCmd.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	diffCmd.Flags().BoolVar(&useTUI, "tui", false, "use the TUI for diff")
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "diff - Show accounts to create/update and CDK and/or TF changes for each account.",
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
				return ProcessOrgEndToEnd(consoleUI, resourceoperation.Diff, parsedTargets)
			})
		} else {
			consoleUI = runner.NewSTDOut()
			if err := ProcessOrgEndToEnd(consoleUI, resourceoperation.Diff, parsedTargets); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}

		consoleUI.Start()
		if err := g.Wait(); err != nil {
			os.Exit(1)
		}
	},
}
