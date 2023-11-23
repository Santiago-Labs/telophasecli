package cmd

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/spf13/cobra"
)

var (
	cdkPath    string
	tfPath     string
	accountTag string
	org        ymlparser.Organization
	allStacks  bool
	stacks     string

	// TUI
	useTUI   bool
	tuiIndex atomic.Int64
	tuiLock  sync.Mutex
)

func init() {
	rootCmd.AddCommand(compileCmd)
	compileCmd.Flags().StringVar(&cdkPath, "cdk-path", "", "Path to your CDK code")
	compileCmd.Flags().StringVar(&tfPath, "tf-path", "", "Path to your Terraform code")
	compileCmd.Flags().BoolVar(&allStacks, "all-stacks", false, "If all stacks should be deployed")
	compileCmd.Flags().StringVar(&stacks, "stacks", "", "List of comma separated stacks to deploy")
	compileCmd.Flags().StringVar(&accountTag, "account-tag", "", "Tag associated with the accounts to apply to a subset of account IDs, tag \"all\" to deploy all accounts.")
	compileCmd.MarkFlagRequired("account-tag")
	compileCmd.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	compileCmd.Flags().BoolVar(&useTUI, "tui", false, "use the TUI for deploy")
}

var compileCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy - Deploy a CDK and/or TF stacks to your AWS account(s). Accounts in organization.yml will be created if they do not exist.",
	Run: func(cmd *cobra.Command, args []string) {
		runIAC(deployIAC{})
	},
}

type deployIAC struct{}

func (d deployIAC) cdkCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, cdkPath string) *exec.Cmd {
	outPath := tmpPath("CDK", acct, cdkPath)
	cdkArgs := []string{"deploy", "--output", outPath, "--require-approval", "never"}
	if allStacks {
		cdkArgs = append(cdkArgs, "--all")
	}
	if stacks != "" {
		cdkArgs = append(cdkArgs, strings.Split(stacks, ",")...)
	}
	cmd := exec.Command("cdk", cdkArgs...)
	cmd.Dir = cdkPath
	cmd.Env = awssts.SetEnviron(os.Environ(),
		*result.Credentials.AccessKeyId,
		*result.Credentials.SecretAccessKey,
		*result.Credentials.SessionToken)

	return cmd
}

func (d deployIAC) tfCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, tfPath string) *exec.Cmd {
	workingPath := tmpPath("Terraform", acct, tfPath)
	args := []string{
		"apply", "-auto-approve",
	}
	cmd := exec.Command("terraform", args...)
	cmd.Dir = workingPath
	cmd.Env = awssts.SetEnviron(os.Environ(),
		*result.Credentials.AccessKeyId,
		*result.Credentials.SecretAccessKey,
		*result.Credentials.SessionToken)

	return cmd
}
