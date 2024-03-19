## Commands
### `telophasecli diff`
This command will read `organization.yml` and **output**:
1) Changes required to AWS Organization.
2) Changes required to Azure Subscriptions.
3) Output of `cdk diff`.
4) Output of `terraform plan`.

### `telophasecli deploy`
This command will read `organization.yml` and **perform**:
1) Changes required to AWS Organization.
2) Changes required to Azure Subscriptions.
3) CDK deploy. `telophasecli diff` does _not_ need to be run before this. Telophase runs `cdk bootstrap` and `cdk synth` on every deploy.
4) Terraform apply. `telophasecli diff` does _not_ need to be run before this. Telophase automatically runs `terraform plan` if no plan exists.

### `telophasecli account import`
This command reads your AWS Organization and outputs `organization.yml`.

### `telophasecli account deploy`
This command will read `organization.yml` and **perform** changes required to your AWS Organization. This is useful if you only want to update your AWS Organization. `telophasecli deploy` encapsulates this call.

### `telophasecli account diff`
This command will read `organization.yml` and **output** changes required to your AWS Organization. This is useful if you only want to see changes to your AWS Organization. `telophasecli diff` encapsulates this call.

### `telophasecli account deploy`
This command will read `organization.yml` and **perform** changes required to your AWS Organization. This is useful if you only want to update your AWS Organization. `telophasecli deploy` encapsulates this call.

### `telophasecli subscription diff`
This command will read `organization.yml` and **output** changes required to your Azure Subscriptions. This is useful if you only want to see changes to your Azure Subscriptions. `telophasecli deploy` encapsulates this call.

### `telophasecli subscription deploy`
This command will read `organization.yml` and **perform** changes required to your Azure Subscriptions. This is useful if you only want to update your Azure Subscriptions. `telophasecli deploy` encapsulates this call.