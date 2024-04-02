package resourceoperation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/localstack"
	"github.com/santiago-labs/telophasecli/lib/terraform"
	"github.com/santiago-labs/telophasecli/resource"
)

func CollectSCPOps(
	ctx context.Context,
	consoleUI runner.ConsoleUI,
	operation int,
	rootOU *resource.AccountGroup,
) []ResourceOperation {

	var ops []ResourceOperation

	return ops
}

type scpOperation struct {
	TargetAcct          *resource.Account
	TargetOU            *resource.AccountGroup
	MgmtAcct            *resource.Account
	Operation           int
	Stack               resource.Stack
	OutputUI            runner.ConsoleUI
	DependentOperations []ResourceOperation
}

func NewSCPOperation(
	consoleUI runner.ConsoleUI,
	targetAcct, mgmtAcct *resource.Account,
	targetOU *resource.AccountGroup,
	stack resource.Stack,
	op int,
) ResourceOperation {
	return &scpOperation{
		TargetAcct: targetAcct,
		TargetOU:   targetOU,
		MgmtAcct:   mgmtAcct,
		Operation:  op,
		Stack:      stack,
		OutputUI:   consoleUI,
	}
}

func (so *scpOperation) AddDependent(op ResourceOperation) {
	so.DependentOperations = append(so.DependentOperations, op)
}

func (so *scpOperation) ListDependents() []ResourceOperation {
	return so.DependentOperations
}

func (so *scpOperation) Call(ctx context.Context) error {
	so.OutputUI.Print(fmt.Sprintf("Executing SCP Terraform stack in %s", so.Stack.Path), *so.MgmtAcct)

	mgmtRole, _, assumeRoleErr := authAWS(*so.MgmtAcct, so.roleARN(), so.OutputUI)
	if assumeRoleErr != nil {
		return assumeRoleErr
	}

	initTFCmd := so.initTf(mgmtRole)
	if initTFCmd != nil {
		if err := so.OutputUI.RunCmd(initTFCmd, *so.TargetAcct); err != nil {
			return err
		}
	}

	var args []string
	if so.Operation == Diff {
		args = []string{
			"plan",
		}
	} else if so.Operation == Deploy {
		args = []string{
			"apply", "-auto-approve",
		}
	}

	workingPath := so.tmpPath()
	cmd := exec.Command(localstack.TfCmd(), args...)
	cmd.Dir = workingPath

	cmd.Env = awssts.SetEnviron(os.Environ(),
		*mgmtRole.Credentials.AccessKeyId,
		*mgmtRole.Credentials.SecretAccessKey,
		*mgmtRole.Credentials.SessionToken)

	if err := so.OutputUI.RunCmd(cmd, *so.TargetAcct); err != nil {
		return err
	}

	for _, op := range so.DependentOperations {
		op.Call(ctx)
	}

	return nil
}

func (so *scpOperation) roleARN() string {
	if so.Stack.RoleOverrideARN != "" {
		return so.Stack.RoleOverrideARN
	}
	return so.MgmtAcct.AssumeRoleARN()
}

func (so *scpOperation) initTf(role *sts.AssumeRoleOutput) *exec.Cmd {
	workingPath := so.tmpPath()
	terraformDir := filepath.Join(workingPath, ".terraform")
	if terraformDir == "" || !strings.Contains(terraformDir, "telophasedirs") {
		panic(fmt.Errorf("expected terraform dir to be set"))
	}

	if err := os.RemoveAll(terraformDir); err != nil {
		panic(fmt.Errorf("failed to remove directory %s: %w", terraformDir, err))
	}

	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		if err := os.MkdirAll(workingPath, 0755); err != nil {
			panic(fmt.Errorf("failed to create directory %s: %w", workingPath, err))
		}

		if err := terraform.CopyDir(so.Stack.Path, workingPath, *so.TargetAcct); err != nil {
			panic(fmt.Errorf("failed to copy files from %s to %s: %w", so.Stack.Path, workingPath, err))
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

func (so *scpOperation) tmpPath() string {
	hasher := sha256.New()
	hasher.Write([]byte(so.Stack.Path))
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return path.Join("telophasedirs", fmt.Sprintf("tf-tmp-%s-%s-%s", so.MgmtAcct.ID(), so.MgmtAcct.ID(), hashString))
}

func (so *scpOperation) ToString() string {
	return ""
}
