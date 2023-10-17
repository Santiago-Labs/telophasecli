package main

import (
	"fmt"
	"os"
	"path"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"
	"github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"

	// "github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type Props struct {
	awscdk.StackProps
}

var cidr string

func New(scope constructs.Construct, id string, props *Props) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	vpc := awsec2.NewVpc(stack, jsii.String("vpc"),
		&awsec2.VpcProps{
			Cidr: jsii.String("10.0.0.0/18"),
		},
	)

	lambdaPath := path.Join(os.Getenv("HOME"), "sl/lambdahandler")
	lambda := awscdklambdagoalpha.NewGoFunction(stack, jsii.String("lambdaGoFunction"), &awscdklambdagoalpha.GoFunctionProps{
		Entry: jsii.String(lambdaPath),
		Environment: &map[string]*string{
			"TELOPHASE_CUSTOMER": jsii.String(id),
		},
	})

	apiGateway := awsapigateway.NewLambdaRestApi(stack, jsii.String("api"), &awsapigateway.LambdaRestApiProps{
		Handler: lambda,
	})

	rds := awsrds.NewServerlessCluster(stack, jsii.String("rdsServerless"), &awsrds.ServerlessClusterProps{
		Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(
			&awsrds.AuroraPostgresClusterEngineProps{
				Version: awsrds.AuroraPostgresEngineVersion_VER_13_9(),
			},
		),
		Vpc: vpc,
	})
	fmt.Println(lambda, apiGateway, rds)

	return stack
}

func main() {
	defer jsii.Close()

	cidr = os.Getenv("TELOPHASE_VPC_CIDR")
	if cidr == "" {
		cidr = "10.0.0.0/16"
	}

	customerName := os.Getenv("TELOPHASE_CUSTOMER")
	if customerName == "" {
		customerName = "cdkapp"
	}

	app := awscdk.NewApp(nil)

	New(app, customerName, &Props{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String("123456789012"),
	//  Region:  jsii.String("us-east-1"),
	// }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
