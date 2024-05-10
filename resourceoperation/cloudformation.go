package resourceoperation

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awssess"
	"github.com/santiago-labs/telophasecli/resource"
)

type cloudformationOp struct {
	Account              *resource.Account
	Operation            int
	Stack                resource.Stack
	OutputUI             runner.ConsoleUI
	DependentOperations  []ResourceOperation
	CloudformationClient cloudformationiface.CloudFormationAPI
}

func NewCloudformationOperation(consoleUI runner.ConsoleUI, acct *resource.Account, stack resource.Stack, op int) ResourceOperation {
	creds, _, err := authAWS(*acct, *stack.RoleARN(*acct), consoleUI)
	if err != nil {
		panic(oops.Wrapf(err, "authAWS"))
	}

	var newCreds *credentials.Credentials
	if creds != nil {
		newCreds = credentials.NewStaticCredentials(*creds.AccessKeyId, *creds.SecretAccessKey, *creds.SessionToken)
	}

	cloudformationClient := cloudformation.New(session.Must(awssess.DefaultSession(&aws.Config{
		Credentials: newCreds,
		Region:      &stack.Region,
	})))

	return &cloudformationOp{
		Account:              acct,
		Operation:            op,
		Stack:                stack,
		OutputUI:             consoleUI,
		CloudformationClient: cloudformationClient,
	}
}

func (co *cloudformationOp) AddDependent(op ResourceOperation) {
	co.DependentOperations = append(co.DependentOperations, op)
}

func (co *cloudformationOp) ListDependents() []ResourceOperation {
	return co.DependentOperations
}

func (co *cloudformationOp) Call(ctx context.Context) error {
	co.OutputUI.Print(fmt.Sprintf("Executing Cloudformation stack in %s", co.Stack.Path), *co.Account)

	cs, err := co.createChangeSet(ctx)
	if err != nil {
		return err
	}
	if aws.StringValue(cs.Status) == cloudformation.ChangeSetStatusFailed {
		if strings.Contains(aws.StringValue(cs.StatusReason), "The submitted information didn't contain changes") {
			co.OutputUI.Print(fmt.Sprintf("change set (%s) resulted in no diff, skipping", *co.Stack.ChangeSetName()), *co.Account)
			return nil
		} else {
			return oops.Errorf("change set failed, reason (%s)", aws.StringValue(cs.StatusReason))
		}
	} else {
		co.OutputUI.Print("Created change set with changes:"+cs.String(), *co.Account)
	}

	// End call if we aren't deploying
	if co.Operation != Deploy {
		return nil
	}

	_, err = co.executeChangeSet(ctx, cs.ChangeSetId)
	if err != nil {
		return oops.Wrapf(err, "executing change set")
	}
	co.OutputUI.Print("Executed change set", *co.Account)

	return nil
}

func (co *cloudformationOp) createChangeSet(ctx context.Context) (*cloudformation.DescribeChangeSetOutput, error) {
	params, err := co.Stack.CloudformationParametersType()
	if err != nil {
		return nil, oops.Wrapf(err, "CloudformationParameters")
	}

	// If we can find the stack then we just update. If not then we continue on
	changeSetType := cloudformation.ChangeSetTypeUpdate

	stack, err := co.CloudformationClient.DescribeStacksWithContext(ctx,
		&cloudformation.DescribeStacksInput{
			StackName: co.Stack.CloudformationStackName(),
		})
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			changeSetType = cloudformation.ChangeSetTypeCreate
			// Reset err in case it is re-referenced somewhere else
			err = nil
		} else {
			return nil, oops.Wrapf(err, "describe stack with name: (%s)", *co.Stack.CloudformationStackName())
		}
	} else {
		if len(stack.Stacks) == 1 && aws.StringValue(stack.Stacks[0].StackStatus) == cloudformation.StackStatusReviewInProgress {
			// If we reset to Create the change set the same change set will be reused.
			changeSetType = cloudformation.ChangeSetTypeCreate
		}
	}
	template, err := ioutil.ReadFile(co.Stack.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read stack template at path: (%s) should be a path to one file", co.Stack.Path)
	}

	strTemplate := string(template)
	changeSet, err := co.CloudformationClient.CreateChangeSetWithContext(ctx,
		&cloudformation.CreateChangeSetInput{
			Parameters:    params,
			StackName:     co.Stack.CloudformationStackName(),
			ChangeSetName: co.Stack.ChangeSetName(),
			TemplateBody:  &strTemplate,
			ChangeSetType: &changeSetType,
			Capabilities:  co.Stack.CloudformationCapabilitiesArg(),
		},
	)
	if err != nil {
		return nil, oops.Wrapf(err, "createChangeSet for stack: %s", co.Stack.Name)
	}

	for {
		cs, err := co.CloudformationClient.DescribeChangeSetWithContext(ctx,
			&cloudformation.DescribeChangeSetInput{
				StackName:     changeSet.StackId,
				ChangeSetName: changeSet.Id,
			})
		if err != nil {
			return nil, oops.Wrapf(err, "DescribeChangeSet")
		}

		state := aws.StringValue(cs.Status)
		switch state {
		case cloudformation.ChangeSetStatusCreateInProgress:
			co.OutputUI.Print(fmt.Sprintf("Still creating change set for stack: %s", *co.Stack.CloudformationStackName()), *co.Account)

		case cloudformation.ChangeSetStatusCreateComplete:
			co.OutputUI.Print(fmt.Sprintf("Successfully created change set for stack: %s", *co.Stack.CloudformationStackName()), *co.Account)
			return cs, nil

		case cloudformation.ChangeSetStatusFailed:
			return cs, nil
		}

		time.Sleep(5 * time.Second)
	}
}

func (co *cloudformationOp) executeChangeSet(ctx context.Context, changeSetID *string) (*cloudformation.DescribeChangeSetOutput, error) {
	_, err := co.CloudformationClient.ExecuteChangeSetWithContext(ctx,
		&cloudformation.ExecuteChangeSetInput{
			ChangeSetName: changeSetID,
		})
	if err != nil {
		return nil, oops.Wrapf(err, "executing change set")
	}

	for {
		cs, err := co.CloudformationClient.DescribeChangeSetWithContext(ctx,
			&cloudformation.DescribeChangeSetInput{
				ChangeSetName: changeSetID,
			})
		if err != nil {
			return nil, oops.Wrapf(err, "DescribeChangeSet")
		}

		state := aws.StringValue(cs.ExecutionStatus)
		switch state {
		case cloudformation.ExecutionStatusExecuteInProgress:
			co.OutputUI.Print(fmt.Sprintf("Still executing change set for stack: (%s) for path: %s", *co.Stack.CloudformationStackName(), co.Stack.Path), *co.Account)

		case cloudformation.ExecutionStatusExecuteComplete:
			co.OutputUI.Print(fmt.Sprintf("Successfully executed change set for stack: (%s) for path: %s", *co.Stack.CloudformationStackName(), co.Stack.Path), *co.Account)
			return cs, nil

		case cloudformation.ExecutionStatusExecuteFailed:
			co.OutputUI.Print(fmt.Sprintf("Failed to execute change set: (%s) for path: %s Reason: %s", *co.Stack.CloudformationStackName(), co.Stack.Path, *cs.StatusReason), *co.Account)
			return cs, oops.Errorf("ExecuteChangeSet failed")
		}

		time.Sleep(5 * time.Second)
	}
}

func (co *cloudformationOp) ToString() string {
	return ""
}
