package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-cdk-go/awscdk"
	"github.com/aws/aws-cdk-go/awscdk/awss3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/jsii-runtime-go"
)

type TerraformStateBucketStackProps struct {
	awscdk.StackProps
}

func fetchAccountID() string {
	cfg := aws.NewConfig()
	if os.Getenv("LOCALSTACK") != "" {
		cfg.Endpoint = aws.String("http://localhost:4566")
	}
	sess := session.Must(session.NewSession(
		cfg,
	))
	svc := sts.New(sess)
	result, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		panic(fmt.Sprintf("Failed to get caller identity: %s", err))
	}

	return *result.Account
}

func NewTerraformStateBucketStack(scope awscdk.Construct, id string, props *TerraformStateBucketStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	awss3.NewBucket(stack, jsii.String("TerraformStateBucket"), &awss3.BucketProps{
		Versioned:  jsii.Bool(true),
		BucketName: jsii.String(fmt.Sprintf("tfstate-%s", fetchAccountID())),
	})

	return stack
}

func main() {
	app := awscdk.NewApp(nil)

	NewTerraformStateBucketStack(app, "TerraformStateBucketStackExample", &TerraformStateBucketStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return nil
}
