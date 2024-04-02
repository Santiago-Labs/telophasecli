package resourceoperation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awssess"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/cdk"
	"github.com/santiago-labs/telophasecli/lib/localstack"
	"github.com/santiago-labs/telophasecli/resource"
)

type cdkOperation struct {
	Account             *resource.Account
	Operation           int
	Stack               resource.Stack
	OutputUI            runner.ConsoleUI
	DependentOperations []ResourceOperation
}

func NewCDKOperation(consoleUI runner.ConsoleUI, acct *resource.Account, stack resource.Stack, op int) ResourceOperation {
	return &cdkOperation{
		Account:   acct,
		Operation: op,
		Stack:     stack,
		OutputUI:  consoleUI,
	}
}

func (co *cdkOperation) AddDependent(op ResourceOperation) {
	co.DependentOperations = append(co.DependentOperations, op)
}

func (co *cdkOperation) ListDependents() []ResourceOperation {
	return co.DependentOperations
}

func (co *cdkOperation) Call(ctx context.Context) error {
	co.OutputUI.Print(fmt.Sprintf("Executing CDK stack in %s", co.Stack.Name), *co.Account)
	stackRole, err := authAWS(*co.Account, co.Stack, co.OutputUI)
	if err != nil {
		panic(err)
	}

	bootstrapCDK := bootstrapCDK(stackRole, *co.Account, co.Stack)
	if err := co.OutputUI.RunCmd(bootstrapCDK, *co.Account); err != nil {
		return err
	}

	synthCDK := synthCDK(stackRole, *co.Account, co.Stack)
	if err := co.OutputUI.RunCmd(synthCDK, *co.Account); err != nil {
		return err
	}

	var cdkArgs []string
	if co.Operation == Diff {
		cdkArgs = []string{
			"diff",
			"--context", fmt.Sprintf("telophaseAccountName=%s", co.Account.AccountName),
			"--output", cdk.TmpPath(*co.Account, co.Stack.Path),
		}
	} else if co.Operation == Deploy {
		cdkArgs = []string{"deploy", "--require-approval", "never", "--output", cdk.TmpPath(*co.Account, co.Stack.Path)}
	}

	cdkArgs = append(cdkArgs, "--context", fmt.Sprintf("telophaseAccountName=%s", co.Account.AccountName))
	if co.Stack.Name == "" {
		cdkArgs = append(cdkArgs, "--all")
	} else {
		cdkArgs = append(cdkArgs, strings.Split(co.Stack.Name, ",")...)
	}
	cmd := exec.Command(localstack.CdkCmd(), cdkArgs...)
	cmd.Dir = co.Stack.Path
	if stackRole != nil {
		cmd.Env = awssts.SetEnviron(os.Environ(),
			*stackRole.Credentials.AccessKeyId,
			*stackRole.Credentials.SecretAccessKey,
			*stackRole.Credentials.SessionToken)
	}
	if err := co.OutputUI.RunCmd(cmd, *co.Account); err != nil {
		return err
	}

	for _, op := range co.DependentOperations {
		op.Call(ctx)
	}

	return nil
}

func (co *cdkOperation) ToString() string {
	return ""
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

	if stack.Name != "" {
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