<p align="center">
  <a href="https://telophase.dev"><img src="https://github.com/Santiago-Labs/telophasecli/assets/3019043/ff5ed6db-9e91-44e7-9feb-bcf4f608bce8" alt="Logo" height=170></a>
</p>
<h1 align="center">Telophase</h1>
<br/>

## Why Telophase?
Automation and Compliance are key concerns when adopting multi-account AWS. Telophase orchestrates the management of AWS Organizations alongside your infrastructure-as-code (IaC) provider, like Terraform or CDK. Using a single tool for these allows:
1. **Workflow Automation**: Automates account creation and decommissioning, integrating with existing automation workflows, like CI or ServiceNow.
2. **IaC <> Account Binding**: Enables binding accounts to specific IaC stacks for automatic provisioning of baseline resources.
3. **Easier Compliance Deployment**: Enables binding Service Control Policies (SCPs) to accounts as part of your Account provisioning workflow to make sure every Account is compliant. We make it easy to test SCPs before they are deployed.

Currently, Telophase is a CLI tool only. In the future, we plan to offer a web UI.

## Install
Go is the only supported installation method. If you'd like another method, please let us know by opening an issue!
```
go install github.com/santiago-labs/telophasecli@latest
```

## Quick links

- Intro
  - [Quickstart](mintlifydocs/quickstart.md)
- Features
  - [Manage AWS Organization as IaC](mintlifydocs/features.md#aws-organization)
  - [Manage Service Control Policies](mintlifydocs/features.md#service-control-policies)
  - [Assign IaC Blueprints to Accounts](mintlifydocs/features.md#assign-iac-blueprints-to-accounts)
  - [Testing](mintlifydocs/features.md#testing)
  - [Terminal UI](mintlifydocs/features/tui.mdx)
- CLI
  - [`telophase diff`](mintlifydocs/commands/diff.mdx)
  - [`telophase deploy`](mintlifydocs/commands/deploy.mdx)
  - [`telophase account import`](mintlifydocs/commands/account-import.mdx)
- Organization.yml Reference
  - [Reference](mintlifydocs/config/organizationyml.mdx)


### Future Development
- [ ] Support for multi-cloud organizations with a unified account factory.
  - [ ] Azure
  - [ ] GCP
- [ ] Drift detection/prevention
- [ ] Guardrails around account resources 
- [ ] Guardrails around new Accounts, similar to Control Tower rules.

### Comparisons
#### Telophase vs Control Tower
Manage Accounts via code not a UI. Telophase leaves the controls up to you and your IaC.

#### Telophase vs CDK with multiple environments
Telophase wraps your usage of CDK so that you can apply the cdk to multiple
accounts in parallel. Telophase lets you focus on your actual infrastructure and
not worrying about setting up the right IAM roles for multi account management.
