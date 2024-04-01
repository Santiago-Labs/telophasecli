package resources

type AzureAccountGroup struct {
	// Required Fields for managing from a root subscription.
	SubscriptionTenantID string `yaml:"SubscriptionTenantID"`
	SubscriptionOwnerID  string `yaml:"SubscriptionOwnerID"`

	BaselineStacks []Stack `yaml:"Stacks,omitempty"`

	// The Billing fields are combined to resourceoperations.Create a billing scope like:
	// fmt.Sprintf("/providers/Microsoft.Billing/billingAccounts/%s/billingProfiles/%s/invoiceSections/%s",
	// 	args.BillingAccountName,
	// 	args.BillingProfileName,
	// 	args.InvoiceSectionName),

	// az billing account list | jq '.[] | select(.displayName == "<YOUR-BILLING-ACCOUNT-DISPLAY-NAME>") | .name'
	BillingAccountName string `yaml:"BillingAccountName"`

	// az billing profile list --account-name <billingAccountName> | jq '.[] | select(.displayName == "<YOUR-BILLING-PROFILE-DISPLAY-NAME>") | .name'
	BillingProfileName string `yaml:"BillingProfileName"`

	// az billing invoice section list --account-name <billingAccountName> --profile-name <billingProfileName> | jq '.[] | select(.displayName == "<YOUR-INVOICE-SECTION-DISPLAY-NAME>") | .name'
	InvoiceSectionName string             `yaml:"InvoiceSectionName"`
	Subscriptions      []Subscription     `yaml:"Subscriptions,omitempty"`
	Parent             *AzureAccountGroup `yaml:"-"`
}

func (az AzureAccountGroup) AllDescendentAccounts() []*Account {
	var accounts []*Account

	for i := range az.Subscriptions {
		accounts = append(accounts, az.Subscriptions[i].Account)
	}

	return accounts
}

func (az AzureAccountGroup) AllBaselineStacks() []Stack {
	var stacks []Stack
	if az.Parent != nil {
		stacks = append(stacks, az.Parent.AllBaselineStacks()...)
	}
	stacks = append(stacks, az.BaselineStacks...)
	return stacks
}
