package resourceoperation

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/samsarahq/go/oops"
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
	co.OutputUI.Print(fmt.Sprintf("Executing CDK stack in %s", co.Stack.Path), *co.Account)

	creds, region, err := authAWS(*co.Account, *co.Stack.RoleARN(*co.Account), co.OutputUI)
	if err != nil {
		return err
	}

	if co.Stack.Region != "" {
		region = co.Stack.Region
	}

	// We must bootstrap cdk with the account role.
	bootstrapCDK := bootstrapCDK(creds, region, *co.Account, co.Stack)
	if err := co.OutputUI.RunCmd(bootstrapCDK, *co.Account); err != nil {
		return err
	}

	synthCDK := synthCDK(creds, *co.Account, co.Stack)
	if err := co.OutputUI.RunCmd(synthCDK, *co.Account); err != nil {
		return err
	}

	var cdkArgs []string
	if co.Operation == Diff {
		cdkArgs = []string{
			"diff",
		}
	} else if co.Operation == Deploy {
		cdkArgs = []string{"deploy", "--require-approval", "never"}

		if co.Stack.Destroy {
			cdkArgs = []string{"destroy", "--require-approval", "never"}
		}
	}

	cdkArgs = append(cdkArgs, cdkDefaultArgs(*co.Account, co.Stack)...)
	// Deploy all CDK stacks every time.
	cdkArgs = append(cdkArgs, "--all")

	cmd := exec.Command(localstack.CdkCmd(), cdkArgs...)
	cmd.Dir = co.Stack.Path
	cmd.Env = awssts.SetEnvironCreds(os.Environ(),
		creds,
		co.Stack.AWSRegionEnv(),
	)
	if err := co.OutputUI.RunCmd(cmd, *co.Account); err != nil {
		return err
	}

	for _, op := range co.DependentOperations {
		if err := op.Call(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (co *cdkOperation) ToString() string {
	return ""
}

func bootstrapCDK(creds *sts.Credentials, region string, acct resource.Account, stack resource.Stack) *exec.Cmd {
	cdkArgs := append([]string{
		"bootstrap",
		fmt.Sprintf("aws://%s/%s", acct.AccountID, region),
	},
		cdkDefaultArgs(acct, stack)...,
	)

	cmd := exec.Command(localstack.CdkCmd(), cdkArgs...)
	cmd.Dir = stack.Path
	cmd.Env = awssts.SetEnvironCreds(os.Environ(),
		creds,
		stack.AWSRegionEnv(),
	)

	return cmd
}

func synthCDK(creds *sts.Credentials, acct resource.Account, stack resource.Stack) *exec.Cmd {
	cdkArgs := append(
		[]string{"synth"},
		cdkDefaultArgs(acct, stack)...,
	)

	cmd := exec.Command(localstack.CdkCmd(), cdkArgs...)
	cmd.Dir = stack.Path
	cmd.Env = awssts.SetEnvironCreds(os.Environ(),
		creds,
		stack.AWSRegionEnv(),
	)

	return cmd
}

func authAWS(acct resource.Account, arn string, consoleUI runner.ConsoleUI) (*sts.Credentials, string, error) {
	if os.Getenv("TELOPHASE_BYPASS_ASSUME_ROLE") != "" {
		return nil, "us-east-1", nil
	}

	var svc *sts.STS
	sess := session.Must(awssess.DefaultSession())
	svc = sts.New(sess)

	consoleUI.Print(fmt.Sprintf("Assuming role: %s", arn), acct)
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(arn),
		RoleSessionName: aws.String("telophase-org"),
	}

	role, err := awssess.AssumeRole(svc, input)
	if err != nil {
		// If we are in the management account the OrganizationAccountAccessRole
		// does not exist so fallback to the current credentials.
		if acct.ManagementAccount {
			// I tried to use GetSessionToken to satisfy a type and avoid
			// passing around a nil. However, GetSessionToken's output keys are
			// not allowed to make any IAM changes.
			//
			// Return nil because we don't need to assume a role.
			return nil, "us-east-1", nil
		}

		return nil, "", oops.Wrapf(err, "AssumeRole")
	}
	return role.Credentials, *sess.Config.Region, nil
}

func cdkDefaultArgs(acct resource.Account, stack resource.Stack) []string {
	return []string{
		"--context", fmt.Sprintf("telophaseAccountName=%s", acct.AccountName),
		"--context", fmt.Sprintf("telophaseAccountId=%s", acct.AccountID),
		"--output", cdk.TmpPath(acct, stack.Path),
	}
}
