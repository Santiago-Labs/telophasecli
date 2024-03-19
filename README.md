<p align="center">
  <a href="https://telophase.dev"><img src="https://github.com/Santiago-Labs/telophasecli/assets/3019043/ff5ed6db-9e91-44e7-9feb-bcf4f608bce8" alt="Logo" height=170></a>
</p>
<h1 align="center">Telophase</h1>
<br/>

## What is Telophase?
Telophase is a tool designed to manage AWS Organizations and Azure Subscriptions using infrastructure-as-code (IaC) principles. It uses a configuration file, `organization.yml`, as a source of truth and makes calls to your IaC provider (`terraform` or `cdk`) and your cloud provider(s).

Currently, Telophase is a CLI tool only. In the future, we plan to offer a web UI.

## Install
Go is the only supported installation method. If you'd like another method, please let us know by opening an issue!
```
go install github.com/santiago-labs/telophasecli@latest
```

## Quick links

- Intro
  - [Quickstart](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/quickstart.md)
- Features
  - [Manage AWS Organization](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/features.md#aws-organization)
  - [Manage Azure Subscriptions](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/features.md#azure-subscriptions)
  - [Assign IaC to Accounts/Subscriptions](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/features.md#assign-iac-stacks-to-accountssubscriptions)
  - [Pass Outputs Across Stacks](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/features.md#pass-outputs-across-accounts-and-regions-cdk-only)
  - [Terminal UI](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/features.md#terminal-ui)
- CLI
  - [`telophase account`](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/commands.md#account)
  - [`telophase diff`](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/commands.md#diff)
  - [`telophase deploy`](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/commands.md#deploy)
- Organization.yml Reference
  - [Reference](https://github.com/Santiago-Labs/telophasecli/blob/main/docs/organizationyml.md)


### Future Development
- [ ] Support for multi-cloud organizations with a unified account factory.
  - [x] Azure
  - [ ] GCP
- [ ] Drift detection/prevention
- [ ] Guardrails around account resources 
- [ ] Guardrails around new Accounts, similar to Control Tower rules.

### Metrics Collection
We are collecting metrics on commands run via PostHog. By default, we collect the
commands run, but this can be turned off by setting
`TELOPHASE_METRICS_DISABLED=true`

### Comparisons
#### Telophase vs Control Tower
Manage Accounts via code not a UI. Telophase leaves the controls up to you and your IaC.

#### Telophase vs CDK with multiple environments
Telophase wraps your usage of CDK so that you can apply the cdk to multiple
accounts in parallel. Telophase lets you focus on your actual infrastructure and
not worrying about setting up the right IAM roles for multi account management.
