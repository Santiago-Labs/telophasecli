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
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/localstack"
	"github.com/santiago-labs/telophasecli/lib/terraform"
	"github.com/santiago-labs/telophasecli/resource"
)

func CollectSCPOps(
	ctx context.Context,
	orgClient awsorgs.Client,
	consoleUI runner.ConsoleUI,
	operation int,
	rootOU *resource.OrganizationUnit,
	mgmtAcct *resource.Account,
) []ResourceOperation {

	var ops []ResourceOperation
	for _, ou := range rootOU.AllDescendentOUs() {
		for _, scp := range ou.ServiceControlPolicies {
			ops = append(ops, NewSCPOperation(
				consoleUI,
				nil,
				mgmtAcct,
				ou,
				scp,
				operation,
			))
		}
	}

	for _, acct := range rootOU.AllDescendentAccounts() {
		for _, scp := range acct.ServiceControlPolicies {
			ops = append(ops, NewSCPOperation(
				consoleUI,
				acct,
				mgmtAcct,
				nil,
				scp,
				operation,
			))
		}
	}

	return ops
}

type scpOperation struct {
	TargetAcct          *resource.Account
	TargetOU            *resource.OrganizationUnit
	MgmtAcct            *resource.Account
	Operation           int
	Stack               resource.Stack
	OutputUI            runner.ConsoleUI
	DependentOperations []ResourceOperation
}

func NewSCPOperation(
	consoleUI runner.ConsoleUI,
	targetAcct, mgmtAcct *resource.Account,
	targetOU *resource.OrganizationUnit,
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

	var acctRole *sts.AssumeRoleOutput
	if so.MgmtAcct.AssumeRoleName != "" {
		var err error
		acctRole, _, err = authAWS(*so.MgmtAcct, so.MgmtAcct.AssumeRoleARN(), so.OutputUI)
		if err != nil {
			return err
		}
	}

	initTFCmd, err := so.initTf()
	if err != nil {
		so.OutputUI.Print(fmt.Sprintf("Error initializing terraform: %s", err), *so.MgmtAcct)
		return err
	}

	if initTFCmd != nil {
		if acctRole != nil {
			initTFCmd.Env = awssts.SetEnviron(os.Environ(),
				*acctRole.Credentials.AccessKeyId,
				*acctRole.Credentials.SecretAccessKey,
				*acctRole.Credentials.SessionToken)
		}
		if err := so.OutputUI.RunCmd(initTFCmd, *so.MgmtAcct); err != nil {
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

	if acctRole != nil {
		cmd.Env = awssts.SetEnviron(os.Environ(),
			*acctRole.Credentials.AccessKeyId,
			*acctRole.Credentials.SecretAccessKey,
			*acctRole.Credentials.SessionToken)
	}

	if err := so.OutputUI.RunCmd(cmd, *so.MgmtAcct); err != nil {
		return err
	}

	for _, op := range so.DependentOperations {
		if err := op.Call(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (so *scpOperation) initTf() (*exec.Cmd, error) {
	workingPath := so.tmpPath()
	terraformDir := filepath.Join(workingPath, ".terraform")
	if terraformDir == "" || !strings.Contains(terraformDir, "telophasedirs") {
		return nil, fmt.Errorf("expected terraform dir to be set")
	}

	if err := os.RemoveAll(terraformDir); err != nil {
		return nil, fmt.Errorf("failed to remove directory %s: %v", terraformDir, err)
	}

	_, err := os.Stat(terraformDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(workingPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %v", terraformDir, err)
		}

		if err := terraform.CopyDir(so.Stack.Path, workingPath, so.targetResource()); err != nil {
			return nil, fmt.Errorf("failed to copy files from %s to %s: %v", so.Stack.Path, workingPath, err)
		}

		cmd := exec.Command(localstack.TfCmd(), "init")
		cmd.Dir = workingPath

		return cmd, nil
	}

	return nil, err
}

func (so *scpOperation) targetResource() resource.Resource {
	if so.TargetAcct != nil {
		return so.TargetAcct
	}
	return so.TargetOU
}

func (so *scpOperation) tmpPath() string {
	hasher := sha256.New()
	hasher.Write([]byte(so.Stack.Path))
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return path.Join("telophasedirs", fmt.Sprintf("tf-tmp-%s-%s-%s", so.MgmtAcct.ID(), so.targetResource().ID(), hashString))
}

func (so *scpOperation) ToString() string {
	return ""
}
