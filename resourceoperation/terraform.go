package resourceoperation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/service/sts"
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
		if to.Stack.RoleOverrideARN != "" {
			stackRole, _, assumeRoleErr = authAWS(*to.Account, to.Stack.RoleOverrideARN, to.OutputUI)
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
		*stackRole.Credentials.SessionToken)

	if err := to.OutputUI.RunCmd(cmd, *to.Account); err != nil {
		return err
	}

	for _, op := range to.DependentOperations {
		op.Call(ctx)
	}

	return nil
}

func (to *tfOperation) initTf(role *sts.AssumeRoleOutput) *exec.Cmd {
	workingPath := terraform.TmpPath(*to.Account, to.Stack.Path)
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

		if err := terraform.CopyDir(to.Stack.Path, workingPath, *to.Account); err != nil {
			panic(fmt.Errorf("failed to copy files from %s to %s: %w", to.Stack.Path, workingPath, err))
		}

		cmd := exec.Command(localstack.TfCmd(), "init")
		cmd.Dir = workingPath

		cmd.Env = awssts.SetEnviron(os.Environ(),
			*role.Credentials.AccessKeyId,
			*role.Credentials.SecretAccessKey,
			*role.Credentials.SessionToken)

		return cmd
	}

	return nil
}

func (to *tfOperation) ToString() string {
	return ""
}
