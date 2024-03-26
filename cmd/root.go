package cmd

import (
	"fmt"
	"os"

	"github.com/santiago-labs/telophasecli/lib/metrics"
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
