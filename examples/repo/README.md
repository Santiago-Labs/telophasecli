# Example Repo Setup
This Directory contains an example repo for IaC using remote S3 buckets and telophasecli and is intended to work with localstack.

# Getting Started with Localstack + Telophase
1. Install Dependencies
```bash
./setup.sh
```

2. Start up Localstack in the background. Ensure your pro license is set. Learn more [here](https://docs.localstack.cloud/getting-started/auth-token/)
```bash
localstack start -d
```

3. Localstack Setup
Create your root organization in Localstack
```bash
awslocal organizations create-organization --feature-set ALL
```

4. Setup an AWS_REGION if not set
```bash
export AWS_REGION=us-east-1
```
5. Run your first account diff. This assumes that you have an `organization.yml` setup. If using this repo you can use the example [`organization.yml`](./organization.yml) in this directory.

```bash
LOCALSTACK=true AWS_REGION=us-east-1 telophasecli account diff
```

6. Create your first accounts!
```bash
LOCALSTACK=true AWS_REGION=us-east-1 telophasecli account deploy
```

7. Deploy Infra in the Accounts

Optionally use the TUI via --tui. The [`organization.yml`](./organization.yml) in this account assumes you are using the [`s3-remote-state`](./s3-remote-state/) for CDK and the IAM role stack in [`tf/ci_iam`](./tf/ci_iam) for Terraform. This will deploy the associated stacks with each account.
```bash
LOCALSTACK=true AWS_REGION=us-east-1 telophasecli deploy
```

8. Inspect the accounts using `awslocal`

Learn how Localstack handles multi-account auth [here](https://docs.localstack.cloud/references/multi-account-setups/)
```bash
# View the organizations you have created
awslocal organizations list-accounts

# Each account will have its own tfstate bucket 
AWS_ACCESS_KEY_ID=$AN_ORG_FROM_ABOVE awslocal s3 ls

# New roles will be created in each account
AWS_ACCESS_KEY_ID=$AN_ORG_FROM_ABOVE awslocal iam list-roles
```
