# Azure Terraform

In this example, we use Telophase to manage multiple Azure Subscriptions in
parallel with Terraform.

<div>
    <a href="https://www.loom.com/share/4fa1999de25c4dbeb60431a8c823ef50">
      <p>Telophase Azure/Terraform</p>
    </a>
    <a href="https://www.loom.com/share/4fa1999de25c4dbeb60431a8c823ef50">
      <img style="max-width:300px;" src="https://cdn.loom.com/sessions/thumbnails/4fa1999de25c4dbeb60431a8c823ef50-with-play.gif">
    </a>
</div>

# Organization File
Check out the [organization file](./organization.yaml) to see how we define the organization's subscriptions.

Example `organization.yml`
```yaml
Azure:
  # az account list | jq '.[] | select(.isDefault == true) | .id'
  SubscriptionTenantID: SubscriptionTenantID 
  # Owner Email Address
  SubscriptionOwnerID: subscriptionowner@example.com 

  # az billing account list | jq '.[] | select(.displayName == "<YOUR-BILLING-ACCOUNT-DISPLAY-NAME>") | .name'
  BillingAccountName: Billing-Account-Name 
  # az billing profile list --account-name <billingAccountName> | jq '.[] | select(.displayName == "<YOUR-BILLING-PROFILE-DISPLAY-NAME>") | .name'
  BillingProfileName: XXXX-XXXX-XXX-XXX 
  # az billing invoice section list --account-name <billingAccountName> --profile-name <billingProfileName> | jq '.[] | select(.displayName == "<YOUR-INVOICE-SECTION-DISPLAY-NAME>") | .name'
  InvoiceSectionName: XXXX-XXXX-XXX-XXX 

  Subscriptions:
    - Name: "Azure subscription 1"
      Stacks:
        - Type: "Terraform"
          Path: "./examples/tf/azure"

    - Name: "SubscriptionOrgcli"
      Stacks:
        - Type: "Terraform"
          Path: "./examples/tf/azure"

    - Name: "Subscription 2"
      Stacks:
        - Type: "Terraform"
          Path: "./examples/tf/azure"

    - Name: "Subscription 3"
      Stacks:
        - Type: "Terraform"
          Path: "./examples/tf/azure"
```
# Root README
Check out the [root README](../../../README.md) to see telophase's full set of features.