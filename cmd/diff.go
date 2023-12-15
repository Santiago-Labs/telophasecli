package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"github.com/santiago-labs/telophasecli/lib/awscloudformation"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/azureorgs"
	"github.com/santiago-labs/telophasecli/lib/cdk"
	"github.com/santiago-labs/telophasecli/lib/cdk/template"
	"github.com/santiago-labs/telophasecli/lib/terraform"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVar(&cdkPath, "cdk-path", "", "Path to your CDK code")
	diffCmd.Flags().StringVar(&tfPath, "tf-path", "", "Path to your TF code")
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

func (d diffIAC) cdkCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack, prevOutputs []*template.CDKOutputs) *exec.Cmd {
	outPath := cdk.TmpPath(acct, stack.Path)
	cdkArgs := []string{"diff", "--output", outPath}
	cdkArgs = append(cdkArgs, "--context", fmt.Sprintf("telophaseAccountName=%s", acct.AccountName))
	for _, prevOutput := range prevOutputs {
		for key, val := range prevOutput.Outputs {
			if reflect.TypeOf(val["Value"]).Kind() == reflect.Map {
				serializedVal, err := json.Marshal(val)
				if err != nil {
					panic(err)
				}
				cdkArgs = append(cdkArgs, "--context", fmt.Sprintf("%s.%s=%s", prevOutput.StackName, key, serializedVal))
			} else {
				cdkArgs = append(cdkArgs, "--context", fmt.Sprintf("%s.%s=%s", prevOutput.StackName, key, val["Value"]))
			}
		}
	}
	if stack.Name == "" {
		cdkArgs = append(cdkArgs, "--all")
	} else {
		cdkArgs = append(cdkArgs, strings.Split(stack.Name, ",")...)
	}
	cmd := exec.Command("cdk", cdkArgs...)
	cmd.Dir = stack.Path
	if result != nil {
		cmd.Env = awssts.SetEnviron(os.Environ(),
			*result.Credentials.AccessKeyId,
			*result.Credentials.SecretAccessKey,
			*result.Credentials.SessionToken)
	}

	return cmd
}

func (d diffIAC) cdkOutputs(cfnClient awscloudformation.Client, acct ymlparser.Account, stack ymlparser.Stack) []*template.CDKOutputs {
	isDeployed, err := cfnClient.IsStackDeployed(context.TODO(), stack.Name)
	if err != nil {
		panic(err)
	}

	if !isDeployed {
		outputs, err := cdk.StackLocalOutput(cfnClient, acct, stack)
		if err != nil {
			panic(err)
		}
		return outputs
	}

	outputs, err := cdk.StackRemoteOutput(cfnClient, acct, stack)
	if err != nil {
		panic(err)
	}
	return outputs
}

func (d diffIAC) tfCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack) *exec.Cmd {
	workingPath := terraform.TmpPath(acct, stack.Path)
	args := []string{
		"plan",
	}
	cmd := exec.Command("terraform", args...)
	cmd.Dir = workingPath
	if result != nil {
		cmd.Env = awssts.SetEnviron(os.Environ(),
			*result.Credentials.AccessKeyId,
			*result.Credentials.SecretAccessKey,
			*result.Credentials.SessionToken)
	}

	return cmd
}

func (d diffIAC) orgV1Cmd(ctx context.Context, orgClient awsorgs.Client) {
	_, _, err := orgV1Diff(orgClient)
	if err != nil {
		panic(fmt.Sprintf("error: %s", err))
	}
}

func (d diffIAC) orgV2Cmd(ctx context.Context, orgClient awsorgs.Client, subsClient *azureorgs.Client) {
	_, err := orgV2Diff(orgClient, subsClient)
	if err != nil {
		panic(fmt.Sprintf("error: %s", err))
	}
}
