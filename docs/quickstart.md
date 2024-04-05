## Quick Start
### Requirements
1. You must have AWS Organizations enabled.
    - Follow directions from [here](https://docs.aws.amazon.com/organizations/latest/userguide/orgs_tutorials_basic.html) to set up your AWS Organization.
2. AWS CLI must be configured. See [Authentication](#authentication) below

### Installation
```go
go install github.com/santiago-labs/telophasecli@latest
```

### Authentication
Note: Set `AWS_SDK_LOAD_CONFIG=1` when passing env variables directly e.g. `AWS_PROFILE=<profile_name> AWS_SDK_LOAD_CONFIG=1 telophasecli account import`

#### Option 1: IAM Identity Center/AWS SSO (Recommended)
1. Navigate to Identity Center in the *Management Account*
2. Create a group and add the users who will manage accounts and apply IaC changes
3. Navigate to the `AWS accounts` tab in Identity Center
4. Assign the group to all accounts you want telophase to manage (note: you must include your management account)
5. Assign these permission sets to the group:
    -  `AWSOrganizationsFullAccess` - This policy allows the creation of organizations and linked roles.
    - `sts:*` - This policy allows the AWS CLI to assume roles in sub-accounts to update infrastructure.
6. Configure AWS CLI using `aws configure sso`. Make sure to choose the region where IAM Identity Center is configured!

For more details, visit the [Identity Center CLI Guide](https://docs.aws.amazon.com/cli/latest/userguide/sso-configure-profile-token.html)

#### Option 2: IAM
1. Navigate to IAM in the *Management Account*
2. Create a role and attach the following policies:
    -  `AWSOrganizationsFullAccess` - This policy allows the creation of organizations and linked roles.
    - `sts:*` - This policy allows the AWS CLI to assume roles in sub-accounts to update infrastructure.
3. Configure AWS CLI to use the role you just created.
    - Follow the instructions [here](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-role.html) to configure the CLI with role-based access.

### Configure `organization.yml`
Telophase uses a file called `organization.yml` to manage your AWS Organization and IaC. See [organization.yml](#organization.yml) for configuration options.

#### Option 1: Import Existing AWS Organization
Telophase can import your AWS Organization (including OU structure):
```sh
telophasecli account import
```

This command will output an `organization.yml` file containing all the accounts in your AWS Organization. You can remove any accounts you don't want Telophase to manage from this file.

#### Option 2: Start From Scratch
If you prefer to start fresh and not have Telophase manage any of your existing accounts, create the organization.yml file with the following content:

```yaml
Organization:
    Name: root
```

### You're ready!
Here's a few examples of what you can do. Visit [Features](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/features.md) for a detailed guide.

#### Example: Create account
Create an account by adding a new entry to `organization.yml`:
```yaml
Organization:
  Name: root
  Accounts:
    - Email: ethan+ci@telophase.dev
      AccountName: CI
```
Then run `telophasecli account diff` and `telophasecli account apply`

#### Example: Apply Terraform
You can apply IaC by assigning a stack to the account in `organization.yml`:
```yaml
Organization:
  Name: root
  Accounts:
    - Email: ethan+ci@telophase.dev
      AccountName: CI
      Stacks:
        - Path: tf/ci_blueprint
          Type: Terraform
```
Then run `telophasecli diff` and `telophasecli apply`