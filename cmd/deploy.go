package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/cmd/runner"
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

var (
	tag    string
	stacks string

	// TUI
	useTUI bool
)

func init() {
	rootCmd.AddCommand(compileCmd)
	compileCmd.Flags().StringVar(&stacks, "stacks", "", "Filter stacks to deploy")
	compileCmd.Flags().StringVar(&tag, "tag", "", "Filter accounts and account groups to deploy")
	compileCmd.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	compileCmd.Flags().BoolVar(&useTUI, "tui", false, "use the TUI for deploy")
}

var compileCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy - Deploy a CDK and/or TF stacks to your AWS account(s). Accounts in organization.yml will be created if they do not exist.",
	Run: func(cmd *cobra.Command, args []string) {

		orgClient := awsorgs.New()
		subsClient, err := azureorgs.New()
		if err != nil {
			panic(fmt.Sprintf("error: %s", err))
		}
		ctx := context.Background()

		var accountsToApply []ymlparser.Account

		ops, err := orgV2Diff(orgClient, subsClient)
		if err != nil {
			panic(fmt.Sprintf("error: %s", err))
		}

		for _, op := range ops {
			op.Call(ctx, orgClient)
		}
		awsGroup, azureGroup, err := ymlparser.ParseOrganizationV2(orgFile)
		if err != nil {
			panic(fmt.Sprintf("error: %s parsing organization", err))
		}
		if awsGroup != nil {
			for _, acct := range awsGroup.AllDescendentAccounts() {
				if contains(tag, acct.AllTags()) || tag == "" {
					accountsToApply = append(accountsToApply, *acct)
				}
			}
		}

		if azureGroup != nil {
			for _, acct := range azureGroup.AllDescendentAccounts() {
				if contains(tag, acct.AllTags()) || tag == "" {
					accountsToApply = append(accountsToApply, *acct)
				}
			}
		}

		if len(accountsToApply) == 0 {
			fmt.Println("No accounts to deploy")
		}

		var consoleUI runner.ConsoleUI
		if useTUI {
			consoleUI = runner.NewTUI()
		} else {
			consoleUI = runner.NewSTDOut()
		}
		runIAC(deployIAC{}, accountsToApply, consoleUI)
	},
}

type deployIAC struct{}

func (d deployIAC) cdkCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack) *exec.Cmd {
	cdkArgs := []string{"deploy", "--require-approval", "never", "--output", cdk.TmpPath(acct, stack.Path)}
	cdkArgs = append(cdkArgs, "--context", fmt.Sprintf("telophaseAccountName=%s", acct.AccountName))
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

func (d deployIAC) tfCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack) *exec.Cmd {
	workingPath := terraform.TmpPath(acct, stack.Path)
	args := []string{
		"apply", "-auto-approve",
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
