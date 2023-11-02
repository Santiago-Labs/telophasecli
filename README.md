# telophasecli

# Features
## Provision AWS accounts under one Management Account via code
Example `organization.yml`
```yml
Organization:
    MasterAccount:
        Email: management@telophase.dev
        AccountName: Acme 

    ChildAccounts:
        - Email: eng1@telophase.dev
          AccountName: Engineer 1 
          Tags:
            - "dev"

        - Email: eng2@telophase.dev
          AccountName: Engineer 2 
          Tags:
            - "dev"

+        - Email: eng3@telophase.dev
+          AccountName: Engineer 3
+          Tags:
+            - "dev"
```

In the above example adding account "Engineer 3" then running:
`telophase deploy --account-tag="dev" --apply` in your CDK repository `telophase` will:
- provision the new AWS account
- Apply your CDK stack to all accounts with the tag `dev` in parallel

## Terminal UI for deploying to multiple AWS accounts 
`telophase` TUI is helpful when applying your CDK code to multiple Accounts.

https://github.com/Santiago-Labs/telophasecli/assets/22655472/525b4c71-3f42-41b3-9c5c-4b8ddb1a3482


# Requirements
- Setup AWS Organizations. 
    - Follow directions from [here](https://docs.aws.amazon.com/organizations/latest/userguide/orgs_tutorials_basic.html) for setting up your Organization.
- Configure awscli credentials. You must have the following privileges from the management account.
    - `accounts:*`
        - We need to modify sub AWS accounts.
    - `sts:*`
        - From the management account we assume roles in the sub-accounts to manage their infrastructure.

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

