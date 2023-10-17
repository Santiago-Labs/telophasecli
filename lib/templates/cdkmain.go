package cdktemplates

type CdkData struct {
	AwsAccountId     string
	AwsAccountRegion string
}

var CdkMainTmpl = `
package main

import (
	// "fmt"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53targets"
	// secretmgr "github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
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

	vpcCidr := "10.0.0.0/18"
	vpc := awsec2.NewVpc(stack, jsii.String("vpc"),
		&awsec2.VpcProps{
			IpAddresses: awsec2.IpAddresses_Cidr(jsii.String(vpcCidr)),
			MaxAzs:      jsii.Number(2),
		},
	)

	lambdaPath := path.Join(os.Getenv("HOME"), "sl/lambdahandler")
	lambda := awscdklambdagoalpha.NewGoFunction(stack, jsii.String("lambdaGoFunction"), &awscdklambdagoalpha.GoFunctionProps{
		Entry: jsii.String(lambdaPath),
		Environment: &map[string]*string{
			"TELOPHASE_CUSTOMER": jsii.String(id),
		},
	})

	cert := awscertificatemanager.Certificate_FromCertificateArn(
		stack,
		jsii.String("cert"),
		jsii.String("arn:aws:acm:us-west-2:011526729437:certificate/e44373c3-4b89-49c1-8da6-e05fc4694a44"),
	)
	apiG := awsapigateway.NewLambdaRestApi(stack, jsii.String("api"), &awsapigateway.LambdaRestApiProps{
		Handler: lambda,
		DomainName: &awsapigateway.DomainNameOptions{
			DomainName:  jsii.String(strings.ToLower(id) + ".example.telophase.dev"),
			Certificate: cert,
		},
	})
	hostedZone := awsroute53.HostedZone_FromLookup(
		stack,
		jsii.String("hostedZone"),
		&awsroute53.HostedZoneProviderProps{
			DomainName: jsii.String("example.telophase.dev"),
		},
	)

	apiGatewayTarget := awsroute53targets.NewApiGateway(apiG)

	awsroute53.NewARecord(stack, jsii.String("ARecord"),
		&awsroute53.ARecordProps{
			Zone: hostedZone,
			// id maps to the customer name
			RecordName: jsii.String(id),
			Target:     awsroute53.RecordTarget_FromAlias(apiGatewayTarget),
		})

	// Create MySQL 3306 inbound Security Group.
	dbSg := awsec2.NewSecurityGroup(stack, jsii.String("PGSG"), &awsec2.SecurityGroupProps{
		Vpc:               vpc,
		SecurityGroupName: jsii.String(*stack.StackName() + "-PGSG"),
		AllowAllOutbound:  jsii.Bool(true),
		Description:       jsii.String("RDS Postgres DB instances communication SG."),
	})

	dbSg.AddIngressRule(
		// Allow VPC traffic.
		awsec2.Peer_Ipv4(&vpcCidr),
		awsec2.NewPort(&awsec2.PortProps{
			Protocol:             awsec2.Protocol_TCP,
			FromPort:             jsii.Number(3306),
			ToPort:               jsii.Number(3306),
			StringRepresentation: jsii.String("Standard Postgres listen port."),
		}),
		jsii.String("Allow requests to Postgres DB instance."),
		jsii.Bool(false),
	)

	dbSecret := secretmgr.NewSecret(stack, jsii.String("DBSecret"), &secretmgr.SecretProps{
		SecretName: jsii.String(*stack.StackName() + "-Secret"),
		GenerateSecretString: &secretmgr.SecretStringGenerator{
			SecretStringTemplate: jsii.String("{\"username\":\"postgres\"}"),
			ExcludePunctuation:   jsii.Bool(true),
			IncludeSpace:         jsii.Bool(false),
			GenerateStringKey:    jsii.String("password"),
		},
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})
	fmt.Println(dbSecret)

	awsrds.NewServerlessCluster(stack, jsii.String("rdsServerless"), &awsrds.ServerlessClusterProps{
		Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(
			&awsrds.AuroraPostgresClusterEngineProps{
				Version: awsrds.AuroraPostgresEngineVersion_VER_13_9(),
			},
		),
		SecurityGroups: &[]awsec2.ISecurityGroup{dbSg},
		Vpc:            vpc,
		Credentials:    awsrds.Credentials_FromSecret(dbSecret, jsii.String("postgres")),
	})

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
	// return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//---------------------------------------------------------------------------
	return &awscdk.Environment{
		Account: jsii.String("{{.AwsAccountId}}"),
		Region:  jsii.String("{{.AwsAccountRegion}}"),
	}

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//---------------------------------------------------------------------------
	// return &awscdk.Environment{
	//  Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	//  Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
`
