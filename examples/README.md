# Examples
The examples directory contains example Cloudformation, terraform and CDK code that can be referenced in an organization.yml

## organization-config-everywhere.yml
`organization-config-everywhere.yml` stands up an example Org structure where:
- Applies delegated admin to the Audit account.
- Provisions an organization-wide config aggregator in the Audit account. 
- AWS Config is enabled in every region of every telophase managed Account.
