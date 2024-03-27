## Features
### Manage both AWS Organization and Azure Subscription hierarchy as IaC
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

#### Azure Subscriptions
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

### Assign IaC Stacks to Accounts/Subscriptions
Terraform and CDK (AWS Only) can be assigned at any level in the hierarchy. All child accounts/subscriptions inherit the stack.

#### Example
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
