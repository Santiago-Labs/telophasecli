# telophasecli

# Requirements
- Setup AWS Organizations
- Configure awscli credentials. You must have admin privileges for the management account.

## Authentication
Valid `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` must be set in either your `env` or in `~/.aws/credentials`. Run `aws sts get-caller-identity` to check if your credentials are valid.

Make sure to set `AWS_SDK_LOAD_CONFIG=1` when passing env variables e.g. `AWS_PROFILE=<profile_name> AWS_SDK_LOAD_CONFIG=1 telophasecli account import`

### IAM Identity Center/AWS SSO (Optional)
Run `aws configure sso` and follow the directions. Make sure to choose the region where IAM Identity Center is configured!

For more details:
https://docs.aws.amazon.com/cli/latest/userguide/sso-configure-profile-token.html

