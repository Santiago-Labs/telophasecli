# telophasecli
Open-source AWS Control Tower.

# Why
Manage your Account Factory with code. One place to provision new accounts and
apply CDK stacks across all your AWS accounts.

We developed this tool because we have experienced the pain of managing multiple
AWS accounts with Control Tower and Cloudformation Templates. Amazon
forces you to login to their UI and manage all your infrastructure from within
the portal where changing accounts while using SSO is a pain. 

That's is why we developed `telopahsecli`. We wanted a way to apply our CDK code
across many AWS accounts with code and with a great UX.

## Future Development
Support for multi-cloud organizations with a unified account factory.

Drift detection/prevention

Guardrails around account resources 
Guardrails around new Accounts similar to Control Tower rules.

# Features
## Provision AWS accounts via code
Example `organization.yml`
```yml
Organization:
    MasterAccount:
        Email: management@telophase.dev
        AccountName: Telophase 

    ChildAccounts:
        - Email: production+us0@telophase.dev
            AccountName: Production US0 
            Tags:
                - "prod"
            Env:
                - "TELOPHASE_CELL=us0"
                - "AWS_REGION=us-west-2"

        - Email: production+us1@telophase.dev
            AccountName: Production US1
            Tags:
                - "prod"
            Env:
                - "TELOPHASE_CELL=us1"
                - "AWS_REGION=us-east-2"

        - Email: eng1@telophase.dev
            AccountName: Engineer 1 
            Tags:
                - "dev"

        - Email: eng2@telophase.dev
            AccountName: Engineer 2 
            Tags:
                - "dev"

# Adds a new AWS account for a new dev
+        - Email: eng3@telophase.dev
+          AccountName: Engineer 3
+          Tags:
+               - "dev"
```

In the above example adding account "Engineer 3" then running:
`telophasecli deploy --account-tag="dev" --apply` in your CDK repository `telophase` will:
- provision the new AWS account
- Apply your CDK stack to all accounts with the tag `dev` in parallel

## Terminal UI for deploying to multiple AWS accounts 
`telophasecli` TUI is helpful when applying your CDK code to multiple Accounts.

https://github.com/Santiago-Labs/telophasecli/assets/22655472/aa1080d5-d763-4d41-b040-7827d341c384

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

1. View expected change 
`telophasecli deploy --cdk-path=$HOME/cdkapp --account-tag=dev`

2. Apply changes by including `--apply`


# Requirements
- Setup AWS Organizations. 
    - Follow directions from [here](https://docs.aws.amazon.com/organizations/latest/userguide/orgs_tutorials_basic.html) for setting up your Organization.

- Configure awscli credentials. You must have the following AWS managed policies from the management account for the role you will be using.
    -  `AWSOrganizationsFullAccess` - Allows organizatoin creation and linked role creation
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
