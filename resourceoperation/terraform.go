package resourceoperation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/localstack"
	"github.com/santiago-labs/telophasecli/lib/terraform"
	"github.com/santiago-labs/telophasecli/resource"
)

type tfOperation struct {
	Account             *resource.Account
	Operation           int
	Stack               resource.Stack
	OutputUI            runner.ConsoleUI
	DependentOperations []ResourceOperation
}

func NewTFOperation(consoleUI runner.ConsoleUI, acct *resource.Account, stack resource.Stack, op int) ResourceOperation {
	return &tfOperation{
		Account:   acct,
		Operation: op,
		Stack:     stack,
		OutputUI:  consoleUI,
	}
}

func (to *tfOperation) AddDependent(op ResourceOperation) {
	to.DependentOperations = append(to.DependentOperations, op)
}

func (to *tfOperation) ListDependents() []ResourceOperation {
	return to.DependentOperations
}

func (to *tfOperation) Call(ctx context.Context) error {
	to.OutputUI.Print(fmt.Sprintf("Executing Terraform stack in %s", to.Stack.Path), *to.Account)

	var stackRole *sts.AssumeRoleOutput
	var assumeRoleErr error
	if to.Account.AccountID != "" {
		if roleArn := to.Stack.RoleARN(*to.Account); roleArn != nil {
			stackRole, _, assumeRoleErr = authAWS(*to.Account, *roleArn, to.OutputUI)
		} else {
			stackRole, _, assumeRoleErr = authAWS(*to.Account, to.Account.AssumeRoleARN(), to.OutputUI)
		}

		if assumeRoleErr != nil {
			return assumeRoleErr
		}
	}

	initTFCmd := to.initTf(stackRole)
	if initTFCmd != nil {
		if err := to.OutputUI.RunCmd(initTFCmd, *to.Account); err != nil {
			return err
		}
	}

	// Set workspace if we are using it.
	setWorkspace, err := to.setWorkspace(stackRole)
	if err != nil {
		return err
	}
	if setWorkspace != nil {
		if err := to.OutputUI.RunCmd(setWorkspace, *to.Account); err != nil {
			return err
		}
	}

	var args []string
	if to.Operation == Diff {
		args = []string{
			"plan",
		}
	} else if to.Operation == Deploy {
		args = []string{
			"apply", "-auto-approve",
		}
	}

	workingPath := terraform.TmpPath(*to.Account, to.Stack.Path)
	cmd := exec.Command(localstack.TfCmd(), args...)
	cmd.Dir = workingPath

	cmd.Env = awssts.SetEnviron(os.Environ(),
		*stackRole.Credentials.AccessKeyId,
		*stackRole.Credentials.SecretAccessKey,
		*stackRole.Credentials.SessionToken,
		to.Stack.AWSRegionEnv(),
	)

	if err := to.OutputUI.RunCmd(cmd, *to.Account); err != nil {
		return err
	}

	for _, op := range to.DependentOperations {
		if err := op.Call(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (to *tfOperation) initTf(role *sts.AssumeRoleOutput) *exec.Cmd {
	workingPath := terraform.TmpPath(*to.Account, to.Stack.Path)
	terraformDir := filepath.Join(workingPath, ".terraform")
	if terraformDir == "" || !strings.Contains(terraformDir, "telophasedirs") {
		to.OutputUI.Print("expected terraform dir to be set", *to.Account)
		return nil
	}
	// Clean the directory
	if err := os.RemoveAll(terraformDir); err != nil {
		to.OutputUI.Print(fmt.Sprintf("Error: failed to remove directory %s: %v", terraformDir, err), *to.Account)
		return nil
	}

	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		if err := os.MkdirAll(workingPath, 0755); err != nil {
			to.OutputUI.Print(fmt.Sprintf("Error: failed to create directory %s: %v", terraformDir, err), *to.Account)
			return nil
		}

		if err := terraform.CopyDir(to.Stack, workingPath, *to.Account); err != nil {
			to.OutputUI.Print(fmt.Sprintf("Error: failed to copy files from %s to %s: %v", to.Stack.Path, workingPath, err), *to.Account)
			return nil
		}

		cmd := exec.Command(localstack.TfCmd(), "init")
		cmd.Dir = workingPath

		cmd.Env = awssts.SetEnviron(os.Environ(),
			*role.Credentials.AccessKeyId,
			*role.Credentials.SecretAccessKey,
			*role.Credentials.SessionToken,
			to.Stack.AWSRegionEnv(),
		)

		return cmd
	}

	return nil
}

func replaceVals(workspace, AccountID, Region string) (string, error) {
	currentContent := workspace
	// Bracketed needs to be checked before non-bracketed otherwise {telophase.account_id} will replaced with {11111}.
	currentContent = strings.ReplaceAll(currentContent, "${telophase.account_id}", AccountID)
	currentContent = strings.ReplaceAll(currentContent, "telophase.account_id", AccountID)

	preRegionContent := currentContent
	currentContent = strings.ReplaceAll(currentContent, "${telophase.region}", Region)
	currentContent = strings.ReplaceAll(currentContent, "telophase.region", Region)
	if currentContent != preRegionContent && Region == "" {
		return "", oops.Errorf("Region needs to be set on stack if performing substitution")
	}

	return currentContent, nil
}

func (to *tfOperation) setWorkspace(role *sts.AssumeRoleOutput) (*exec.Cmd, error) {
	if !to.Stack.WorkspaceEnabled() {
		return nil, nil
	}

	workingPath := terraform.TmpPath(*to.Account, to.Stack.Path)

	rewrittenWorkspace, err := replaceVals(to.Stack.Workspace, to.Account.AccountID, to.Stack.Region)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(localstack.TfCmd(), "workspace", "select", "-or-create", rewrittenWorkspace)
	cmd.Dir = workingPath

	cmd.Env = awssts.SetEnviron(os.Environ(),
		*role.Credentials.AccessKeyId,
		*role.Credentials.SecretAccessKey,
		*role.Credentials.SessionToken,
		to.Stack.AWSRegionEnv(),
	)

	return cmd, nil
}

func (to *tfOperation) ToString() string {
	return ""
}
