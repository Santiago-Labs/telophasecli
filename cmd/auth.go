package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

const TELOPHASE_URL = "https://app.telophase.dev"

func init() {
	rootCmd.AddCommand(authCommand)
}

func isValidAuthArg(arg string) bool {
	switch arg {
	case "signup":
		return true
	default:
		return false
	}
}

var authCommand = &cobra.Command{
	Use:   "auth",
	Short: "auth - Signup for Telophase (Optional, but we would love your feedback!)",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires at least one arg")
		}
		if isValidAuthArg(args[0]) {
			return nil
		}
		return fmt.Errorf("invalid color specified: %s", args[0])
	},
	Run: func(cmd *cobra.Command, args []string) {
		if args[0] == "signup" {
			if err := openSignup(); err != nil {
				panic(fmt.Sprintf("error opening signup page: %s. Please visit https://app.telophade.dev", err))
			}
		}
	},
}

// https://gist.github.com/sevkin/9798d67b2cb9d07cb05f89f14ba682f8
func openSignup() error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, TELOPHASE_URL)
	return exec.Command(cmd, args...).Start()
}
