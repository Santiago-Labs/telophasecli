package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/azureiam"
	"github.com/santiago-labs/telophasecli/lib/azureorgs"
	"github.com/santiago-labs/telophasecli/lib/cdk"
	"github.com/santiago-labs/telophasecli/lib/localstack"
	"github.com/santiago-labs/telophasecli/lib/terraform"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"

	"github.com/aws/aws-sdk-go/service/sts"
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
		runIAC(diffIAC{})
	},
}

type diffIAC struct{}

func (d diffIAC) cdkCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack) *exec.Cmd {
	cdkArgs := []string{
		"diff",
		"--context", fmt.Sprintf("telophaseAccountName=%s", acct.AccountName),
		"--output", cdk.TmpPath(acct, stack.Path),
	}
	if stack.Name == "" {
		cdkArgs = append(cdkArgs, "--all")
	} else {
		cdkArgs = append(cdkArgs, strings.Split(stack.Name, ",")...)
	}
	cmd := exec.Command(localstack.CdkCmd(), cdkArgs...)
	cmd.Dir = stack.Path
	if result != nil {
		cmd.Env = awssts.SetEnviron(os.Environ(),
			*result.Credentials.AccessKeyId,
			*result.Credentials.SecretAccessKey,
			*result.Credentials.SessionToken)
	}

	return cmd
}

func (d diffIAC) tfCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack) *exec.Cmd {
	workingPath := terraform.TmpPath(acct, stack.Path)
	args := []string{
		"plan",
	}
	cmd := exec.Command(localstack.TfCmd(), args...)
	cmd.Dir = workingPath
	if result != nil {
		cmd.Env = awssts.SetEnviron(os.Environ(),
			*result.Credentials.AccessKeyId,
			*result.Credentials.SecretAccessKey,
			*result.Credentials.SessionToken)
	}
	if acct.SubscriptionID != "" {
		resultEnv, err := azureiam.SetEnviron(os.Environ(), acct.SubscriptionID)
		if err != nil {
			panic(oops.Wrapf(err, "setting azure subscription id %s", acct.SubscriptionID))
		}
		cmd.Env = resultEnv
	}

	return cmd
}

func (d diffIAC) orgV2Cmd(ctx context.Context, orgClient awsorgs.Client, subsClient *azureorgs.Client) {
	_, err := orgV2Diff(orgClient, subsClient)
	if err != nil {
		panic(fmt.Sprintf("error: %s", err))
	}
}
