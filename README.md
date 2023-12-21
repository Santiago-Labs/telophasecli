# telophasecli
Telophasecli is a tool to manage AWS Organizations and Azure Subscriptions as IaC.

# Features
## Manage both AWS Organization and Azure Subscription hierarchy as IaC
### AWS Organizations
Example `organization.yml`
```yml
Organization:
  AccountGroups:
      - Name: Production
        Accounts:
          - Email: safety+firmware@example.app
            AccountName: Safety Firmware
          - Email: safety+ingestion@example.app
            AccountName: Safety Ingestion Team
      - Name: Development
        Accounts:
          - Email: eng1@example.app
            AccountName: Engineer A

```
The configuration above will create
1) `Production` Organizational Unit
2) `Safety Firmware` and `Safety Ingestion Team` accounts in the `Production` OU
3) `Development` Organizational Unit
4) `Engineer A` account in the `Development` OU

### Azure Subscriptions
Example `organization.yml`
```yml
Azure:
  SubscriptionTenantID: 00000000-0000-0000-0000-000000000000
  SubscriptionOwnerID: user@company.com
  BillingAccountName: Example Billing Account
  BillingProfileName: Example Billing Profile
  InvoiceSectionName: Example Invoice Section
  Subscriptions:
    - Name: "Engineer A"
    - Name: "QA"
```
The configuration above will create
1) `Engineer A` subscription
2) `QA` subscription

Required fields: `SubscriptionTenantID`, `SubscriptionOwnerID`, `BillingAccountName`, `BillingProfileName`, `InvoiceSectionName`

## Assign IaC Stacks to Accounts/Subscriptions
Terraform and CDK (AWS Only) can be assigned at any level in the hierarchy. All child accounts/subscriptions inherit the stack.

### Example
```yml
Organization:
  AccountGroups:
      - Name: Production
        Stacks:
          - Name: SCPDisableEURegion # This stack will be applied to all accounts in the `Production` OU (`Safety Firmware` and `Safety Ingestion Team`).
            Path: go/src/cdk/scp
            Type: CDK
        Accounts:
          - Email: safety+firmware@example.app
            AccountName: Safety Firmware
            Stacks:
              - Path: tf/safety/firmware_bucket # This stack will be applied to `Safety Firmware` account only.
                Type: Terraform
          - Email: safety+ingestion@example.app
            AccountName: Safety Ingestion Team
      - Name: Development
        Stacks:
          - Name: DevAccount # This stack will be applied to all accounts in the `Development` OU (`Engineer A`).
            Path: go/src/cdk/dev
            Type: CDK
        Accounts:
          - Email: eng1@example.app
            AccountName: Engineer A
```

## Pass AccountID and AccountName as input to Terraform and CDK Stacks
`AccountID` and `AccountName` are passed as input to each stack as `telophaseAccountID` and `telophaseAccountName` respectively.

### Example: Create a subdomain using the account name
```yml
Organization:
  AccountGroups:
      - Name: Tenants
        Stacks:
          - Name: AccountZone
            Path: go/src/cdk
            Type: CDK
        Accounts:
          - Email: customers+lidl@example.org
            AccountName: Lidl
          - Email: customers+walmart@example.org
            AccountName: Walmart
          - Email: customers+costco@example.org
            AccountName: Costco
```

Define a hosted zone for the subdomain in each child account:
`go/src/cdk/account_zone.go`
```go
stack := awscdk.NewStack(scope, "childZone", props)

# Parse the account name provided by Telophasecli, `telophaseAccountName`
accountName := app.Node().TryGetContext(jsii.String("telophaseAccountName")).(string)
accountZone := fmt.Sprintf("%s.telophase.dev", accountName)

hostedZone := awsroute53.NewHostedZone(stack, jsii.String("childZone"), &awsroute53.HostedZoneProps{
    ZoneName: &accountZone,
})
```
This will create `lidl.telophase.dev` in the `Lidl` account, `walmart.telophase.dev` in the `Walmart` account, and `costco.telophase.dev` in the `Costco` account.

## Pass outputs across accounts and regions (CDK Only)
Outputs are parsed from stacks and passed as input context (ie `--context <stack_name>.<output_key>=<output_value>`) to all sibling and child stacks.

Outputs can be passed across accounts and regions! This can be used to solve the infamous “Export Cannot Be Deleted as it is in Use by Stack” error when updating a CDK output.

### Example: Point subdomain to a hosted zone in another account
```yml
Organization:
  AccountGroups:
      - Name: Production
        Accounts:
          - Email: hostedzone+owner@example.org
            AccountName: Hosted Zone Owner
            Stacks:
            - Name: MainZone
              Path: go/src/cdk
              Type: CDK
        AccountGroups:
          - Name: CustomerAccounts
            Stacks:
              - Name: ChildAccountZone
                Path: go/src/cdk
                Type: CDK
            Accounts:
              - Email: customers+walmart@example.org
                AccountName: Walmart
```

Define the main hosted zone:

`go/src/cdk/main_zone.go`
```go
func NewTelophaseHostedZoneStack(
	scope constructs.Construct, props *awscdk.StackProps,
) awscdk.Stack {

	stack := awscdk.NewStack(scope, jsii.String("MainZone"), props)

	hostedZone := awsroute53.NewHostedZone(stack, jsii.String("MainZone"), &awsroute53.HostedZoneProps{
		ZoneName: jsii.String("telophase.dev"),
	})
	hostedZone.ApplyRemovalPolicy(awscdk.RemovalPolicy_RETAIN)

	awscdk.NewCfnOutput(stack, jsii.String("HostedZoneID"), &awscdk.CfnOutputProps{  // HostedZoneID will be passed as input context to sibling and child stacks
		Value: hostedZone.HostedZoneId(),
	})

	awscdk.NewCfnOutput(stack, jsii.String("HostedZoneName"), &awscdk.CfnOutputProps{  // HostedZoneName will be passed as input context to sibling and child stacks
		Value: hostedZone.ZoneName(),
	})

	return &stack
}
```

Define a hosted zone for the child account:

`go/src/cdk/child_zone.go`
```go
func NewChildAccountHostedZoneStack(
	scope constructs.Construct, accountName string, props *awscdk.StackProps,
) *awscdk.Stack {

	stack := awscdk.NewStack(scope, jsii.String("ChildAccountZone"), props)

	hostedZone := awsroute53.NewHostedZone(stack, jsii.String(fmt.Sprintf("%s-zone", accountName)), &awsroute53.HostedZoneProps{
		ZoneName: jsii.String(fmt.Sprintf("%s.telophase.dev", accountName)),
	})

	nameServersJoined := awscdk.Fn_Join(jsii.String(","), hostedZone.HostedZoneNameServers())

	awscdk.NewCfnOutput(stack, jsii.String("HostedZoneID"), &awscdk.CfnOutputProps{  // HostedZoneID will be passed as input context to sibling and child stacks
		Value: hostedZone.ToString(),
	})

	awscdk.NewCfnOutput(stack, jsii.String("HostedZoneName"), &awscdk.CfnOutputProps{  // HostedZoneName will be passed as input context to sibling and child stacks
		Value: hostedZone.ZoneName(),
	})

	awscdk.NewCfnOutput(stack, jsii.String("HostedZoneOutput"), &awscdk.CfnOutputProps{  // HostedZoneOutput will be passed as input context to sibling and child stacks
		Value:      jsii.String(*nameServersJoined),
		ExportName: jsii.String("TenantNS"),
	})

	return &stack
}

func NewChildAccountPtrStack(
	scope constructs.Construct, accountId string, nsValues *[]string,
	ownerZoneID string, ownerZoneName string, ownerProps *awscdk.StackProps,
) awscdk.Stack {

	stackId := fmt.Sprintf("%s-ptr", accountId)
	stack := awscdk.NewStack(scope, &stackId, ownerProps)

	ownerZone := awsroute53.HostedZone_FromHostedZoneAttributes(stack, jsii.String(fmt.Sprintf("%s-hostedzone", accountId)), &awsroute53.HostedZoneAttributes{
		HostedZoneId: jsii.String(ownerZoneID),
		ZoneName:     jsii.String(ownerZoneName),
	})

	awsroute53.NewNsRecord(stack, jsii.String(fmt.Sprintf("%s-ptr-route53", accountId)), &awsroute53.NsRecordProps{
		Zone:       ownerZone,
		RecordName: jsii.String(fmt.Sprintf("%s.telophase.dev", accountId)),
		Values:     jsii.Strings(*nsValues...),
	})

	return stack
}

```

`go/src/cdk/main.go`
```go
func main() {
	app := awscdk.NewApp(nil)

	NewTelophaseHostedZoneStack(app,
		&awscdk.StackProps{
			Env: MainZoneEnv(),
		},
	)

	accountName := app.Node().TryGetContext(jsii.String("telophaseAccountName")).(string)
	NewChildAccountHostedZoneStack(app, accountName, &awscdk.StackProps{
		Env: ChildAccountEnv(),
	})

  // Output from the MainZone Stack
	ownerZoneID := app.Node().TryGetContext(jsii.String("MainZone.HostedZoneID"))
	ownerZoneName := app.Node().TryGetContext(jsii.String("MainZone.HostedZoneName"))

  // Output from the ChildAccountZone Stack
	joinedNS := app.Node().TryGetContext(jsii.String("ChildAccountZone.HostedZoneOutput"))
	tenantZoneID := app.Node().TryGetContext(jsii.String("ChildAccountZone.HostedZoneID"))
	tenantZoneName := app.Node().TryGetContext(jsii.String("ChildAccountZone.HostedZoneName"))
	if joinedNS != nil && ownerZoneID != nil && tenantZoneID != nil {
		nameservers := strings.Split(joinedNS.(string), ",")

		NewChildAccountPtrStack(
			app,
			accountName,
			&nameservers,
			ownerZoneID.(string),
			ownerZoneName.(string),
			&awscdk.StackProps{
				Env: zone_owner.TelophaseZoneOwnerEnv(),
			},
		)
	}
}

```
This will create:
1) `telophase.dev` hosted zone in `Hosted Zone Owner` account.
2) `walmart.telophase.dev` hosted zones in the `Walmart` account.
3) `walmart.telophase.dev` NS records in the `Hosted Zone Owner` account that points to the NS record in the `Walmart` account.

## Terminal UI
`telophasecli` TUI is helpful for monitoring operations in parallel

<div>
    <a href="https://www.loom.com/share/f55b9436b50a4861adc84be6e1506dbf">
      <p>Telophasecli with TUI - Watch Video</p>
    </a>
    <a href="https://www.loom.com/share/f55b9436b50a4861adc84be6e1506dbf">
      <img style="max-width:300px;" src="https://cdn.loom.com/sessions/thumbnails/f55b9436b50a4861adc84be6e1506dbf-with-play.gif">
    </a>
</div>

## Future Development
- [ ] Support for multi-cloud organizations with a unified account factory.
  - [x] Azure
  - [ ] GCP
- [ ] Drift detection/prevention
- [ ] Guardrails around account resources 
- [ ] Guardrails around new Accounts, similar to Control Tower rules.

# Getting Started 
## Requirements
- AWS Organizations
    - Follow directions from [here](https://docs.aws.amazon.com/organizations/latest/userguide/orgs_tutorials_basic.html) for setting up your Organization.

- Configure awscli credentials. You must have the following AWS managed policies from the management account for the role you will be using.
    -  `AWSOrganizationsFullAccess` - Allows organization creation and linked role creation
    - `sts:*` - Allows the CLI to assume a role in the sub-accounts to update infrastructure

## Installation
```
go install github.com/santiago-labs/telophasecli@latest
```

## Authentication
Valid `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` must be set in either your `env` or in `~/.aws/credentials`. Run `aws sts get-caller-identity` to check if your credentials are valid.

Set `AWS_SDK_LOAD_CONFIG=1` when passing env variables directly e.g. `AWS_PROFILE=<profile_name> AWS_SDK_LOAD_CONFIG=1 telophasecli account import`

### IAM Identity Center/AWS SSO (Optional)
Run `aws configure sso` and follow the directions. Make sure to choose the region where IAM Identity Center is configured!

For more details:
https://docs.aws.amazon.com/cli/latest/userguide/sso-configure-profile-token.html

## Import Existing AWS Organization (Optional)
Run from your management account:
```
telophasecli account import
```

This will output an `organization.yml` file where you can see all the accounts within your organization.

## Deploy
Once you have an `organization.yml` add `Tags` based on your account organization.

View changes
`telophasecli diff --cdk-path=$HOME/cdkapp --account-tag=dev`

## Metrics Collection
We are collecting metrics on commands run via PostHog. By default, we collect the
commands run, but this can be turned off by setting
`TELOPHASE_METRICS_DISABLED=true`

# Comparisons
## Telophase vs Control Tower
Manage Accounts via code not a UI. Telophase leaves the controls up to you and your IaC.

## Telophase vs CDK with multiple environments
Telophase wraps your usage of CDK so that you can apply the cdk to multiple
accounts in parallel. Telophase lets you focus on your actual infrastructure and
not worrying about setting up the right IAM roles for multi account management.
