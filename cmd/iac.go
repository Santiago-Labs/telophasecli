package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awssess"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/azureiam"
	"github.com/santiago-labs/telophasecli/lib/cdk"
	"github.com/santiago-labs/telophasecli/lib/localstack"
	"github.com/santiago-labs/telophasecli/lib/terraform"
	"github.com/santiago-labs/telophasecli/resource"
)

type iacCmd interface {
	cdkCmd(result *sts.AssumeRoleOutput, acct resource.Account, stack resource.Stack) *exec.Cmd
	tfCmd(result *sts.AssumeRoleOutput, acct resource.Account, stack resource.Stack) *exec.Cmd
}

func runIAC(cmd iacCmd, accountsToApply []resource.Account, consoleUI runner.ConsoleUI) {
	var wg sync.WaitGroup
	for i := range accountsToApply {
		wg.Add(1)
		go func(acct resource.Account) {
			defer wg.Done()
			if !acct.IsProvisioned() {
				consoleUI.Print(fmt.Sprintf("skipping account: %s because it hasn't been provisioned yet", acct.AccountName), acct)
				return
			}

			var acctStacks []resource.Stack
			if stacks != "" && stacks != "*" {
				acctStacks = append(acctStacks, acct.FilterBaselineStacks(stacks)...)
			} else {
				acctStacks = append(acctStacks, acct.AllBaselineStacks()...)
			}

			if len(acctStacks) == 0 {
				consoleUI.Print("No stacks to deploy\n", acct)
				return
			}

			var bootstrappedCDK bool
			for _, stack := range acctStacks {
				consoleUI.Print(fmt.Sprintf("executing stack: %s (empty string means all stacks) \n", stack.Name), acct)
				var stackRole *sts.AssumeRoleOutput

				if acct.AccountID != "" {
					var err error
					stackRole, err = authAWS(acct, stack, consoleUI)
					if err != nil {
						consoleUI.Print(fmt.Sprintf("[ERROR] %v\n", err), acct)
						return
					}
				}
				if stack.Type == "CDK" {
					if !bootstrappedCDK {
						bootstrapCDK := bootstrapCDK(stackRole, acct, stack)
						if err := consoleUI.RunCmd(bootstrapCDK, acct); err != nil {
							consoleUI.Print(fmt.Sprintf("[ERROR] %v\n", err), acct)
							return
						}
						bootstrappedCDK = true
					}

					synthCDK := synthCDK(stackRole, acct, stack)
					if err := consoleUI.RunCmd(synthCDK, acct); err != nil {
						consoleUI.Print(fmt.Sprintf("[ERROR] %v\n", err), acct)
						return
					}
					deployCDKCmd := cmd.cdkCmd(stackRole, acct, stack)
					if err := consoleUI.RunCmd(deployCDKCmd, acct); err != nil {
						consoleUI.Print(fmt.Sprintf("[ERROR] %v\n", err), acct)
						return
					}

				} else if stack.Type == "Terraform" {
					initTFCmd := initTf(stackRole, acct, stack)
					if initTFCmd != nil {
						if err := consoleUI.RunCmd(initTFCmd, acct); err != nil {
							consoleUI.Print(fmt.Sprintf("[ERROR] %v\n", err), acct)
							return
						}
					}
					deployTFCmd := cmd.tfCmd(stackRole, acct, stack)
					if err := consoleUI.RunCmd(deployTFCmd, acct); err != nil {
						consoleUI.Print(fmt.Sprintf("[ERROR] %v\n", err), acct)
						return
					}
				} else {
					panic(fmt.Errorf("unsupported stack type: %s", stack.Type))
				}
			}
		}(accountsToApply[i])
	}

	consoleUI.PostProcess()

	wg.Wait()
}

func authAWS(acct resource.Account, stack resource.Stack, consoleUI runner.ConsoleUI) (*sts.AssumeRoleOutput, error) {
	var svc *sts.STS
	sess := session.Must(awssess.DefaultSession())
	svc = sts.New(sess)

	if stack.RoleOverrideARN != "" {
		consoleUI.Print(fmt.Sprintf("Assuming role: %s", stack.RoleOverrideARN), acct)
		input := &sts.AssumeRoleInput{
			RoleArn:         aws.String(stack.RoleOverrideARN),
			RoleSessionName: aws.String("telophase-org"),
		}

		return awssess.AssumeRole(svc, input)
	}

	consoleUI.Print(fmt.Sprintf("Assuming role: %s", acct.AssumeRoleARN()), acct)
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(acct.AssumeRoleARN()),
		RoleSessionName: aws.String("telophase-org"),
	}

	return awssess.AssumeRole(svc, input)
}

func bootstrapCDK(result *sts.AssumeRoleOutput, acct resource.Account, stack resource.Stack) *exec.Cmd {
	cdkArgs := []string{
		"bootstrap",
		"--context", fmt.Sprintf("telophaseAccountName=%s", acct.AccountName),
		"--output", cdk.TmpPath(acct, stack.Path),
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

func synthCDK(result *sts.AssumeRoleOutput, acct resource.Account, stack resource.Stack) *exec.Cmd {
	cdkArgs := []string{
		"synth",
		"--context", fmt.Sprintf("telophaseAccountName=%s", acct.AccountName),
		"--output", cdk.TmpPath(acct, stack.Path),
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

func initTf(result *sts.AssumeRoleOutput, acct resource.Account, stack resource.Stack) *exec.Cmd {
	workingPath := terraform.TmpPath(acct, stack.Path)
	terraformDir := filepath.Join(workingPath, ".terraform")
	if terraformDir == "" || !strings.Contains(terraformDir, "telophasedirs") {
		panic(fmt.Errorf("expected terraform dir to be set"))
	}
	// Clean the directory
	if err := os.RemoveAll(terraformDir); err != nil {
		panic(fmt.Errorf("failed to remove directory %s: %w", terraformDir, err))
	}

	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		if err := os.MkdirAll(workingPath, 0755); err != nil {
			panic(fmt.Errorf("failed to create directory %s: %w", workingPath, err))
		}

		if err := terraform.CopyDir(stack.Path, workingPath, acct); err != nil {
			panic(fmt.Errorf("failed to copy files from %s to %s: %w", stack.Path, workingPath, err))
		}

		cmd := exec.Command(localstack.TfCmd(), "init")
		cmd.Dir = workingPath

		// If result is nil then we are not using AWS
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

	return nil
}

func contains(e string, s []string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
