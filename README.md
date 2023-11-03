# telophasecli
Open-source AWS Control Tower.

# Why
Manage your Account Factory with code. One place to provision new accounts and apply your CDK stacks across as many AWS accounts as you have.

Tag AWS accounts to apply changes to a subset of your global infrastructure.

## Future Development
Support for multi-cloud organizations with a unified account factory.
Guardrails around new Accounts.

# Features
## Provision AWS accounts under one Management Account via code
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
`telophase deploy --account-tag="dev" --apply` in your CDK repository `telophase` will:
- provision the new AWS account
- Apply your CDK stack to all accounts with the tag `dev` in parallel

## Terminal UI for deploying to multiple AWS accounts 
`telophase` TUI is helpful when applying your CDK code to multiple Accounts.

https://github.com/Santiago-Labs/telophasecli/assets/22655472/aa1080d5-d763-4d41-b040-7827d341c384

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
## Telophase vs StackSets
Telophase is a CLI program that can be used alongside your IaC. Telophase allows you to manage your Account settings via one shared file and limit IAM permissions.

## Telophase vs CDK with multiple environments
Telophase wraps your usage of CDK so that you can apply the cdk to multiple accounts in parallel. Telophase lets you focus on your actual infrastructure and not worrying about setting up the right IAM roles for multi account management.

