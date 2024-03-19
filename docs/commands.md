## Commands
### `telophasecli diff`
```sh
Usage:
  telophasecli diff [flags]

Flags:
      --cdk-path string   Path to your CDK code
  -h, --help              help for diff
      --org string        Path to the organization.yml file (default "organization.yml")
      --stacks string     Filter stacks to deploy
      --tag string        Filter accounts and account groups to deploy.
      --tf-path string    Path to your TF code
      --tui               use the TUI for diff
```

This command will read `organization.yml` and **output**:
1) Changes required to AWS Organization.
2) Changes required to Azure Subscriptions.
3) Output of `cdk diff`.
4) Output of `terraform plan`.

### `telophasecli deploy`
```sh
Usage:
  telophasecli deploy [flags]

Flags:
      --cdk-path string   Path to your CDK code
  -h, --help              help for deploy
      --org string        Path to the organization.yml file (default "organization.yml")
      --stacks string     Filter stacks to deploy
      --tag string        Filter accounts and account groups to deploy
      --tf-path string    Path to your Terraform code
      --tui               use the TUI for deploy
```

This command will read `organization.yml` and **perform**:
1) Changes required to AWS Organization.
2) Changes required to Azure Subscriptions.
3) CDK deploy. `telophasecli diff` does _not_ need to be run before this. Telophase runs `cdk bootstrap` and `cdk synth` on every deploy.
4) Terraform apply. `telophasecli diff` does _not_ need to be run before this. Telophase automatically runs `terraform plan` if no plan exists.

### `telophasecli account import`
```sh
Usage:
  telophasecli account [flags]

Flags:
  -h, --help         help for account
      --org string   Path to the organization.yml file (default "organization.yml")
```

This command reads your AWS Organization and outputs `organization.yml`. We do not yet support import Azure Subscriptions.

### `telophasecli account diff`
```sh
Usage:
  telophasecli account [flags]

Flags:
  -h, --help         help for account
      --org string   Path to the organization.yml file (default "organization.yml")
```

This command will read `organization.yml` and **output** changes required to your AWS Organization and Azure Subscriptions. This is useful to view changes before they are run. `telophasecli diff` encapsulates this call.

### `telophasecli account deploy`
```sh
Usage:
  telophasecli account [flags]

Flags:
  -h, --help         help for account
      --org string   Path to the organization.yml file (default "organization.yml")
```

This command will read `organization.yml` and **perform** changes required to your AWS Organization and Azure Subscriptions. This is useful if you want to modify AWS Organization/Azure Subscriptions without deploying CDK or Terraform. `telophasecli deploy` encapsulates this call.
