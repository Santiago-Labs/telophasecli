package resourceoperation

import (
	"context"
	"fmt"

	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/azureorgs"
	"github.com/santiago-labs/telophasecli/resource"
)

type AzureAccountGroupOperation struct {
	Operation           int
	AzureGroup          resource.AzureAccountGroup
	DependentOperations []ResourceOperation
	SubscriptionClient  azureorgs.Client
	OutputUI            runner.ConsoleUI
}

func CollectAzureAcctGroupOps(
	ctx context.Context,
	outputUI runner.ConsoleUI,
	subClient azureorgs.Client,
	grp *resource.AzureAccountGroup,
) []ResourceOperation {

	subscriptions, err := subClient.CurrentSubscriptions(ctx)
	if err != nil {
		fmt.Printf("[ERROR]: %v\n", err)
		return []ResourceOperation{}
	}

	var operations []ResourceOperation

	liveSubs := map[string]struct{}{}
	for _, sub := range subscriptions {
		liveSubs[*sub.DisplayName] = struct{}{}
	}

	subsToCreate := map[string]resource.Subscription{}
	for _, iacSub := range grp.Subscriptions {
		if _, ok := liveSubs[iacSub.SubscriptionName]; !ok {
			subsToCreate[iacSub.SubscriptionName] = iacSub
		}
	}

	for i := range subsToCreate {
		toCreate := subsToCreate[i]

		operations = append(operations, &AzureSubscriptionOperation{
			Operation:    Create,
			Subscription: &toCreate,
			AzureGroup:   *grp,
		})
	}

	return operations
}

func (ao *AzureAccountGroupOperation) AddDependent(op ResourceOperation) {
	ao.DependentOperations = append(ao.DependentOperations, op)
}

func (ao *AzureAccountGroupOperation) ListDependents() []ResourceOperation {
	return ao.DependentOperations
}

func (ao *AzureAccountGroupOperation) Call(ctx context.Context) error {
	return nil
}
