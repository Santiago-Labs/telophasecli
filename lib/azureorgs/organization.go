package azureorgs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/managementgroups/armmanagementgroups"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/aws/smithy-go/ptr"
	"github.com/samsarahq/go/oops"
)

type Client struct {
	subscriptionClient      *armsubscriptions.Client
	subscriptionAliasClient *armsubscription.AliasClient
	managementClient        *armmanagementgroups.Client
}

func New() (*Client, error) {
	creds, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		// If we can't load Azure credentials, assume we don't have azure setup.
		return nil, nil
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

func (args CreateSubscriptionArgs) Valid() error {
	if args.SubscriptionAliasName == "" {
		return fmt.Errorf("missing SubscriptionAliasName")
	}
	if args.SubscriptionTenantID == "" {
		return fmt.Errorf("missing SubscriptionTenantID")
	}
	if args.SubscriptionOwnerID == "" {
		return fmt.Errorf("missing SubscriptionOwnerID")
	}
	if args.BillingAccountName == "" {
		return fmt.Errorf("missing BillingAccountName")
	}
	if args.BillingProfileName == "" {
		return fmt.Errorf("missing BillingProfileName")
	}
	if args.InvoiceSectionName == "" {
		return fmt.Errorf("missing InvoiceSectionName")
	}
	return nil
}

func (c *Client) CreateSubscription(ctx context.Context, args CreateSubscriptionArgs) error {
	if err := args.Valid(); err != nil {
		return oops.Wrapf(err, "")
	}

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
		return oops.Wrapf(err, "")
	}

	// wait for the operation to complete
	result, err := r.PollUntilDone(context.Background(), &runtime.PollUntilDoneOptions{
		Frequency: 5 * time.Second,
	})
	if err != nil {
		return oops.Wrapf(err, "")
	}
	_ = result

	_, err = c.CreateStateStorage(ctx, *result.ID)
	if err != nil {
		return oops.Wrapf(err, "")
	}

	return nil
}

func createResourceGroup(ctx context.Context, subscriptionId string, credential azcore.TokenCredential) (armresources.ResourceGroupsClientCreateOrUpdateResponse, error) {
	rgClient, err := armresources.NewResourceGroupsClient(subscriptionId, credential, nil)
	if err != nil {
		return armresources.ResourceGroupsClientCreateOrUpdateResponse{}, oops.Wrapf(err, "")
	}

	name := to.StringPtr("telophasetfstate")
	param := armresources.ResourceGroup{
		Location: to.StringPtr("eastus"),
		Name:     name,
	}

	output, err := rgClient.CreateOrUpdate(ctx, *name, param, nil)
	if err != nil {
		return armresources.ResourceGroupsClientCreateOrUpdateResponse{}, oops.Wrapf(err, "")
	}
	return output, nil
}

// sanitizeSubscriptionID takes a subscription ID and returns a sanitized version removing the dashes.
func sanitizeSubscriptionID(subscriptionID string) string {
	return strings.ReplaceAll(subscriptionID, "-", "")[:12]
}

func StorageAccountName(subscriptionID string) string {
	return "telophase" + sanitizeSubscriptionID(subscriptionID)
}

// CreateStateStorage creates a resource group and storage account to store the
// terraform state for the given subscription ID.
func (c *Client) CreateStateStorage(ctx context.Context, subscriptionID string) (string, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", oops.Errorf("authentication failure: %+v", err)
	}

	// Call your function to create an Azure resource group.
	resourceGroup, err := createResourceGroup(ctx, subscriptionID, cred)
	if err != nil {
		return "", oops.Errorf("creation of resource group failed: %+v", err)
	}
	storageClient, err := armstorage.NewAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return "", oops.Errorf("creation of storage client failed: %+v", err)
	}

	storageAccountName := StorageAccountName(subscriptionID)
	n := armstorage.SKUNameStandardLRS
	kind := armstorage.KindStorageV2
	properties := armstorage.AccountPropertiesCreateParameters{}
	parameters := armstorage.AccountCreateParameters{
		SKU: &armstorage.SKU{
			Name: &n,
		},
		Kind:       &kind,
		Location:   to.StringPtr("eastus"),
		Properties: &properties,
	}

	result, err := storageClient.BeginCreate(ctx,
		*resourceGroup.Name,
		storageAccountName,
		parameters,
		nil,
	)
	if err != nil {
		return "", oops.Wrapf(err, "")
	}

	postCreate, err := result.PollUntilDone(ctx, nil)
	if err != nil {
		return "", oops.Wrapf(err, "")
	}

	containerClient, err := armstorage.NewBlobContainersClient(subscriptionID, cred, nil)
	if err != nil {
		return "", oops.Wrapf(err, "NewBlobContainersClient")
	}

	_, err = containerClient.Create(ctx, *resourceGroup.Name, storageAccountName, "tfstate", armstorage.BlobContainer{}, nil)
	if err != nil {
		return "", oops.Wrapf(err, "containerClient.Create")
	}

	return *postCreate.Name, nil
}
