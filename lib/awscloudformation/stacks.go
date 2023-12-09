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
)

type CFOutput struct {
	Outputs []map[string]string
}

type Client struct {
	client *cloudformation.CloudFormation
}

func New(creds *sts.Credentials) Client {
	sess := session.Must(session.NewSession(&aws.Config{
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
