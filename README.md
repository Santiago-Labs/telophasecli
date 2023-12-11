# telophasecli
Open-source AWS Control Tower.

# Why
Manage your Account Factory with code. One place to provision new accounts and
apply CDK stacks across all your AWS accounts.

We developed this tool because we have experienced the pain of managing multiple
AWS accounts with Control Tower and Cloudformation Templates. Amazon
forces you to log in to their UI and manage all your infrastructure from within
the portal where changing accounts while using SSO is a pain. 

We wanted a way to apply our CDK code across many AWS accounts with code and
with a great UX.

## Future Development
[] Support for multi-cloud organizations with a unified account factory.
  [x] Azure
  [] GCP
[] Drift detection/prevention
[] Guardrails around account resources 
[] Guardrails around new Accounts, similar to Control Tower rules.

# Features
## Provision AWS accounts via code
Example `organization.yml`
```yml
Organization:
  AccountGroups:
      - Name: Production
        AccountGroups:
          - Name: Safety Team
            AccountGroups:
              - Name: Firmware Team
                Accounts:
                  - Email: safety+firmware@example.app
                    AccountName: Safety Firmware
              - Name: Safety Ingestion
                Accounts:
                  - Email: safety+ingestion@example.app
                    AccountName: Safety Ingestion Team
      - Name: Security
        Accounts:
          - Email: ethan+audit@example.app
            AccountName: Audit
          - Email: ethan+logs@example.app
            AccountName: Log Archive
      - Name: Development
        Accounts:
          - Email: eng1@example.app
            AccountName: Engineer 1
          - Email: eng2@example.app
            AccountName: Engineer 2
# Adds a new AWS account for a new dev
+         - Email: eng3@example.app
+           AccountName: Engineer 3


# Azure accounts differ in that they require top level configuration then a
# Subscription Name.
Azure:
	# az account list | jq '.[] | select(.isDefault == true) | .id'
  SubscriptionTenantID: 00000000-0000-0000-0000-000000000000

  # Owner Email Address
  SubscriptionOwnerID: user@company.com

	# az billing account list | jq '.[] | select(.displayName == "<YOUR-BILLING-ACCOUNT-DISPLAY-NAME>") | .name'
  BillingAccountName: Example Billing Account

	# az billing profile list --account-name <billingAccountName> | jq '.[] | select(.displayName == "<YOUR-BILLING-PROFILE-DISPLAY-NAME>") | .name'
  BillingProfileName: Example Billing Profile

	# az billing invoice section list --account-name <billingAccountName> --profile-name <billingProfileName> | jq '.[] | select(.displayName == "<YOUR-INVOICE-SECTION-DISPLAY-NAME>") | .name'
  InvoiceSectionName: Example Invoice Section
```

In the above example adding account "Engineer 3" then running:
`telophasecli deploy` in your CDK repository `telophase` will:
- provision the new AWS account
- Apply your CDK stack to all accounts in parallel

## Terminal UI for deploying to multiple AWS accounts 
`telophasecli` TUI is helpful when applying your CDK code to multiple Accounts.

<div>
    <a href="https://www.loom.com/share/f55b9436b50a4861adc84be6e1506dbf">
      <p>Telophasecli with TUI - Watch Video</p>
    </a>
    <a href="https://www.loom.com/share/f55b9436b50a4861adc84be6e1506dbf">
      <img style="max-width:300px;" src="https://cdn.loom.com/sessions/thumbnails/f55b9436b50a4861adc84be6e1506dbf-with-play.gif">
    </a>
</div>

# Getting Started 
## Installation
```
go install github.com/santiago-labs/telophasecli@latest
```
## Import Current AWS accounts
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

# Requirements
- Setup AWS Organizations. 
    - Follow directions from [here](https://docs.aws.amazon.com/organizations/latest/userguide/orgs_tutorials_basic.html) for setting up your Organization.

- Configure awscli credentials. You must have the following AWS managed policies from the management account for the role you will be using.
    -  `AWSOrganizationsFullAccess` - Allows organization creation and linked role creation
    - `sts:*` - Allows the CLI to assume a role in the sub-accounts to update infrastructure

## Authentication
Valid `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` must be set in either your `env` or in `~/.aws/credentials`. Run `aws sts get-caller-identity` to check if your credentials are valid.

Make sure to set `AWS_SDK_LOAD_CONFIG=1` when passing env variables e.g. `AWS_PROFILE=<profile_name> AWS_SDK_LOAD_CONFIG=1 telophase account import`

### IAM Identity Center/AWS SSO (Optional)
Run `aws configure sso` and follow the directions. Make sure to choose the region where IAM Identity Center is configured!

For more details:
https://docs.aws.amazon.com/cli/latest/userguide/sso-configure-profile-token.html

# Comparisons
## Telophase vs Control Tower
Manage Accounts via code not a UI. Telophase leaves the controls up to you and your IaC.

## Telophase vs CDK with multiple environments
Telophase wraps your usage of CDK so that you can apply the cdk to multiple
accounts in parallel. Telophase lets you focus on your actual infrastructure and
not worrying about setting up the right IAM roles for multi account management.
