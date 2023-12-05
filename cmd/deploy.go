package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/spf13/cobra"
)

var (
	cdkPath string
	tfPath  string
	tag     string
	stacks  string

	// TUI
	useTUI   bool
	tuiIndex atomic.Int64
	tuiLock  sync.Mutex
)

func init() {
	rootCmd.AddCommand(compileCmd)
	compileCmd.Flags().StringVar(&cdkPath, "cdk-path", "", "Path to your CDK code")
	compileCmd.Flags().StringVar(&tfPath, "tf-path", "", "Path to your Terraform code")
	compileCmd.Flags().StringVar(&stacks, "stacks", "", "Filter stacks to deploy")
	compileCmd.Flags().StringVar(&tag, "tag", "", "Filter accounts and account groups to deploy")
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

func (d deployIAC) cdkCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack) *exec.Cmd {
	outPath := tmpPath("CDK", acct, stack.Path)
	cdkArgs := []string{"deploy", "--output", outPath, "--require-approval", "never"}
	if stack.Name == "" {
		cdkArgs = append(cdkArgs, "--all")
	} else {
		cdkArgs = append(cdkArgs, strings.Split(stack.Name, ",")...)
	}
	cmd := exec.Command("cdk", cdkArgs...)
	cmd.Dir = stack.Path
	cmd.Env = awssts.SetEnviron(os.Environ(),
		*result.Credentials.AccessKeyId,
		*result.Credentials.SecretAccessKey,
		*result.Credentials.SessionToken)

	return cmd
}

func (d deployIAC) tfCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack) *exec.Cmd {
	workingPath := tmpPath("Terraform", acct, stack.Path)
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

func (d deployIAC) orgV1Cmd(ctx context.Context, orgClient awsorgs.Client) {
	newAccounts, _, err := orgV1Diff(orgClient)
	if err != nil {
		panic(fmt.Sprintf("error: %s", err))
	}

	errs := orgClient.CreateAccounts(ctx, newAccounts)
	if errs != nil {
		panic(fmt.Sprintf("error creating accounts %v", errs))
	}
}

func (d deployIAC) orgV2Cmd(ctx context.Context, orgClient awsorgs.Client) {
	ops, err := orgV2Diff(orgClient)
	if err != nil {
		panic(fmt.Sprintf("error: %s", err))
	}

	for _, op := range ops {
		op.Call(ctx, orgClient)
	}
}
