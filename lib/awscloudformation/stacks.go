package awscloudformation

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/jsii-runtime-go"
	"gopkg.in/yaml.v3"

	"github.com/santiago-labs/telophasecli/lib/awssess"
	"github.com/santiago-labs/telophasecli/lib/cdk/template"
)

type Client struct {
	client *cloudformation.CloudFormation
}

func New(creds *sts.Credentials) Client {
	sess := session.Must(awssess.DefaultSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			*creds.AccessKeyId,
			*creds.SecretAccessKey,
			*creds.SessionToken,
		),
	}))
	cfClient := cloudformation.New(sess)

	stsClient := sts.New(sess)
	_, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "UnrecognizedClientException", "InvalidClientTokenId", "AccessDenied":
				fmt.Println("Error fetching caller identity. Ensure your awscli credentials are valid.\nError:", awsErr.Message())
				panic(err)
			}
		}
	}
	return Client{
		client: cfClient,
	}
}

func (c Client) FetchStackOutputs(ctx context.Context, stackName string) ([]map[string]string, error) {
	stack, err := c.client.DescribeStacksWithContext(ctx, &cloudformation.DescribeStacksInput{
		StackName: jsii.String(stackName),
	})
	if err != nil {
		return nil, err
	}

	if len(stack.Stacks) != 1 {
		return nil, fmt.Errorf("expected 1 stack (found: %d)", len(stack.Stacks))
	}

	var outputs []map[string]string
	for _, output := range stack.Stacks[0].Outputs {
		outputs = append(outputs, map[string]string{
			"OutputKey":   *output.OutputKey,
			"OutputValue": *output.OutputValue,
		})
	}

	return outputs, nil
}

func (c Client) FetchTemplateOutputs(ctx context.Context, stackName string) (*template.CDKOutputs, error) {
	var templateOutputs template.CDKOutputs
	template, err := c.client.GetTemplateWithContext(ctx, &cloudformation.GetTemplateInput{
		StackName: jsii.String(stackName),
	})
	if err != nil {
		return &templateOutputs, err
	}

	err = yaml.Unmarshal([]byte(*template.TemplateBody), &templateOutputs)
	if err != nil {
		return &templateOutputs, err
	}

	return &templateOutputs, nil
}

func (c Client) IsStackDeployed(ctx context.Context, stackName string) (bool, error) {
	stacks, err := c.client.DescribeStacksWithContext(ctx, &cloudformation.DescribeStacksInput{
		StackName: jsii.String(stackName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == cloudformation.ErrCodeStackInstanceNotFoundException {
				return false, nil
			}
		}
		return false, err
	}

	if len(stacks.Stacks) == 0 {
		return false, nil
	}

	return true, nil
}
