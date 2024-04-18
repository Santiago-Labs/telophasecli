package main

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk"
	"github.com/aws/aws-cdk-go/awscdk/awss3"
	"github.com/aws/jsii-runtime-go"
)

type TerraformStateBucketStackProps struct {
	awscdk.StackProps
}

func NewTerraformStateBucketStack(scope awscdk.Construct, id string, props *TerraformStateBucketStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	accountId := stack.Node().TryGetContext(jsii.String("telophaseAccountId")).(string)
	awss3.NewBucket(stack, jsii.String("TerraformStateBucket"), &awss3.BucketProps{
		Versioned:  jsii.Bool(true),
		BucketName: jsii.String(fmt.Sprintf("tfstate-%s", accountId)),
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
