package resourceoperation

import (
	"context"

	"github.com/santiago-labs/telophasecli/lib/azureorgs"
	"github.com/santiago-labs/telophasecli/resource"
)

func AzureAccountGroupDiff(az *resource.AzureAccountGroup, subscriptionClient *azureorgs.Client) ([]ResourceOperation, error) {
	if subscriptionClient == nil {
		return nil, nil
	}

	ctx := context.TODO()
	subscriptions, err := subscriptionClient.CurrentSubscriptions(ctx)
	if err != nil {
		return nil, err
	}

	var operations []ResourceOperation

	liveSubs := map[string]struct{}{}
	for _, sub := range subscriptions {
		liveSubs[*sub.DisplayName] = struct{}{}
	}

	subsToCreate := map[string]resource.Subscription{}
	for _, iacSub := range az.Subscriptions {
		if _, ok := liveSubs[iacSub.SubscriptionName]; !ok {
			subsToCreate[iacSub.SubscriptionName] = iacSub
		}
	}

	for i := range subsToCreate {
		toCreate := subsToCreate[i]

		operations = append(operations, &AzureSubscriptionOperation{
			Operation:    Create,
			Subscription: &toCreate,
			AzureGroup:   *az,
		})
	}

	return operations, nil
}
