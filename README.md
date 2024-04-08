<p align="center">
  <a href="https://telophase.dev"><img src="https://github.com/Santiago-Labs/telophasecli/assets/3019043/ff5ed6db-9e91-44e7-9feb-bcf4f608bce8" alt="Logo" height=170></a>
</p>
<h1 align="center">Telophase</h1>
<br/>

## Documentation
Full documentation here: https://docs.telophase.dev

## Why Telophase?
Automation and Compliance are key concerns when adopting a multi-account AWS setup. Telophase manages your AWS Organization as IaC, and deeply integrates with IaC providers, like Terraform or CDK. This integration allows:
1. **Workflow Automation**: Automates account creation and decommissioning, integrating with existing automation workflows, like CI or ServiceNow.
2. **IaC <> Account Binding**: Enables binding accounts to IaC blueprints for automatic provisioning of resources in a newly created account.
3. **Easier Compliance Deployment**: Enables binding Service Control Policies (SCPs) to accounts as part of your Account provisioning workflow to make sure every Account is compliant. We make it easy to test SCPs before they are deployed.

Currently, Telophase is a CLI tool only. In the future, we plan to offer a web UI.

## Install
Go is the only supported installation method. If you'd like another method, please let us know by opening an issue!
```
go install github.com/santiago-labs/telophasecli@latest
```



## Quick links

- Intro
  - [Quickstart](https://docs.telophase.dev/quickstart)
- Features
  - [Manage AWS Organization as IaC](https://docs.telophase.dev/features/Manage-AWS-Organizations)
  - [Manage Service Control Policies](https://docs.telophase.dev/features/scps)
  - [Assign IaC Blueprints to Accounts](https://docs.telophase.dev/features/Assign-IaC-Blueprints-To-Accounts)
  - [Testing](https://docs.telophase.dev/features/localstack)
  - [Terminal UI](https://docs.telophase.dev/features/tui)
- CLI
  - [`telophase diff`](https://docs.telophase.dev/commands/diff)
  - [`telophase deploy`](https://docs.telophase.dev/commands/deploy)
  - [`telophase account import`](https://docs.telophase.dev/commands/account-import)
- Organization.yml Reference
  - [Reference](https://docs.telophase.dev/config/organization)


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
