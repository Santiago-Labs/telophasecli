package azureorgs

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/managementgroups/armmanagementgroups"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/aws/smithy-go/ptr"
)

type Client struct {
	subscriptionClient      *armsubscriptions.Client
	subscriptionAliasClient *armsubscription.AliasClient
	managementClient        *armmanagementgroups.Client
}

func New() (*Client, error) {
	creds, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}

	client := &Client{}
	subscriptionsClient, err := armsubscriptions.NewClient(creds, nil)
	if err != nil {
		return nil, err
	}
	client.subscriptionClient = subscriptionsClient

	managementClient, err := armmanagementgroups.NewClient(creds, nil)
	if err != nil {
		return nil, err
	}
	client.managementClient = managementClient

	subAliasClient, err := armsubscription.NewAliasClient(creds, nil)
	if err != nil {
		return nil, err
	}
	client.subscriptionAliasClient = subAliasClient

	return client, nil
}

// CurrentSubscriptions fetches all subscriptions in the organization.
func (c *Client) CurrentSubscriptions(ctx context.Context) ([]*armsubscriptions.Subscription, error) {
	var response []*armsubscriptions.Subscription

	pager := c.subscriptionClient.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		response = append(response, page.Value...)
	}

	return response, nil
}

type CreateSubscriptionArgs struct {
	SubscriptionAliasName string

	SubscriptionTenantID string
	SubscriptionOwnerID  string

	BillingAccountName string
	BillingProfileName string
	InvoiceSectionName string
}

func (c *Client) CreateSubscription(ctx context.Context, args CreateSubscriptionArgs) error {
	workload := armsubscription.WorkloadProduction // important! does not work with WorkloadDevTest

	r, err := c.subscriptionAliasClient.BeginCreate(ctx, args.SubscriptionAliasName, armsubscription.PutAliasRequest{
		Properties: &armsubscription.PutAliasRequestProperties{
			DisplayName: ptr.String(args.SubscriptionAliasName),
			AdditionalProperties: &armsubscription.PutAliasRequestAdditionalProperties{
				SubscriptionTenantID: ptr.String(args.SubscriptionTenantID),
				SubscriptionOwnerID:  ptr.String(args.SubscriptionOwnerID),
			},
			BillingScope: ptr.String(
				fmt.Sprintf("/providers/Microsoft.Billing/billingAccounts/%s/billingProfiles/%s/invoiceSections/%s",
					args.BillingAccountName,
					args.BillingProfileName,
					args.InvoiceSectionName),
			),
			Workload: &workload,
		},
	}, &armsubscription.AliasClientBeginCreateOptions{})
	if err != nil {
		return err
	}

	// wait for the operation to complete
	result, err := r.PollUntilDone(context.Background(), &runtime.PollUntilDoneOptions{
		Frequency: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	_ = result

	return nil
}
