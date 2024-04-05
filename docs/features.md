## Features
### Manage AWS Organization as IaC
#### AWS Organization
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

### Service Control Policies
Service Control Policies defined in Terraform or CDK can be applied to Organization Units and Accounts in `organization.yml`.

#### Example
```yml
Organization:
  AccountGroups:
      - Name: Production
        ServiceControlPolicies:
          - Name: DisableEURegion # This SCP will be applied to the `Production` Organization Unit.
            Path: path/to/scp
            Type: Terraform
        Accounts:
          - Email: safety+firmware@example.app
            AccountName: Safety Firmware
          - Email: safety+ingestion@example.app
            AccountName: Safety Ingestion Team
      - Name: Development
        Accounts:
          - Email: eng1@example.app
            AccountName: Engineer A
            ServiceControlPolicies:
              - Name: DisableGPUInstances # This SCP will be applied to `Engineer A` account only.
                Path: path/to/scp
                Type: Terraform
```

### Assign IaC Blueprints to Accounts
Terraform and CDK can be assigned at any level in the hierarchy. All child accounts inherit the stack.

#### Example
```yml
Organization:
  AccountGroups:
      - Name: Production
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

### Testing
Telophase integrates with [localstack](https://www.localstack.cloud/) to test AWS Organization and TF/CDK changes locally. Set `LOCALSTACK=true` in your env to use localstack instead of your AWS account. For a detailed example, see the [LocalStack Example](https://github.com/Santiago-Labs/telophasecli/tree/main/examples/localstack).

### Pass AccountID and AccountName as input to Terraform and CDK Stacks
`AccountID` and `AccountName` are passed as input to each stack as `telophaseAccountID` and `telophaseAccountName` respectively.

#### Example: Create a subdomain using the account name
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

// Parse the account name provided by Telophasecli, `telophaseAccountName`
accountName := app.Node().TryGetContext(jsii.String("telophaseAccountName")).(string)
accountZone := fmt.Sprintf("%s.telophase.dev", accountName)

hostedZone := awsroute53.NewHostedZone(stack, jsii.String("childZone"), &awsroute53.HostedZoneProps{
    ZoneName: &accountZone,
})
```
This will create `lidl.telophase.dev` in the `Lidl` account, `walmart.telophase.dev` in the `Walmart` account, and `costco.telophase.dev` in the `Costco` account.

### Terminal UI
Pass `--tui` option to use our Terminal UI

<img width="1272" alt="Screenshot 2024-04-05 at 12 20 39 PM" src="https://github.com/Santiago-Labs/telophasecli/assets/3019043/4cb86f97-bf59-4a80-adb0-0323b7005934">