package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVar(&cdkPath, "cdk-path", "", "Path to your CDK code")
	diffCmd.Flags().StringVar(&tfPath, "tf-path", "", "Path to your TF code")
	diffCmd.Flags().BoolVar(&allStacks, "all-stacks", true, "If all stacks should be deployed")
	diffCmd.Flags().StringVar(&stacks, "stacks", "", "List of specific stacks to deploy")
	diffCmd.Flags().StringVar(&tag, "account-tag", "all", "Tag associated with the accounts to apply to a subset of account IDs, tag \"all\" to deploy all accounts.")
	diffCmd.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	diffCmd.Flags().BoolVar(&useTUI, "tui", false, "use the TUI for diff")
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "diff - Show accounts to create/update and CDK and/or TF changes for each account.",
	Run: func(cmd *cobra.Command, args []string) {
		runIAC(diffIAC{})
	},
}

type diffIAC struct{}

func (d diffIAC) cdkCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, cdkPath string) *exec.Cmd {
	outPath := tmpPath("CDK", acct, cdkPath)
	cdkArgs := []string{"diff", "--output", outPath}
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

func (d diffIAC) tfCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, tfPath string) *exec.Cmd {
	workingPath := tmpPath("Terraform", acct, tfPath)
	args := []string{
		"plan",
	}
	cmd := exec.Command("terraform", args...)
	cmd.Dir = workingPath
	cmd.Env = awssts.SetEnviron(os.Environ(),
		*result.Credentials.AccessKeyId,
		*result.Credentials.SecretAccessKey,
		*result.Credentials.SessionToken)

	return cmd
}

func (d diffIAC) orgV1Cmd(ctx context.Context, orgClient awsorgs.Client) {
	_, _, err := orgV1Plan(orgClient)
	if err != nil {
		panic(fmt.Sprintf("error: %s", err))
	}
}

func (d diffIAC) orgV2Cmd(ctx context.Context, orgClient awsorgs.Client) {
	_, err := orgV2Plan(orgClient)
	if err != nil {
		panic(fmt.Sprintf("error: %s", err))
	}
}
