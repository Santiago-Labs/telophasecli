package ymlparser

import (
	"bytes"
	"context"
	"html/template"
	"log"

	"github.com/fatih/color"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/azureorgs"
)

type AzureSubscriptionOperation struct {
	Operation           int
	Subscription        *Subscription
	AzureGroup          AzureAccountGroup
	DependentOperations []ResourceOperation
}

func (ao *AzureSubscriptionOperation) AddDependent(op ResourceOperation) {
	ao.DependentOperations = append(ao.DependentOperations, op)
}

func (ao *AzureSubscriptionOperation) ListDependents() []ResourceOperation {
	return ao.DependentOperations
}

func (ao *AzureSubscriptionOperation) Call(ctx context.Context, orgsClient awsorgs.Client) error {
	subsClient, err := azureorgs.New()
	if err != nil {
		return err
	}

	if ao.Operation == Create {
		err := subsClient.CreateSubscription(ctx, azureorgs.CreateSubscriptionArgs{
			SubscriptionAliasName: ao.Subscription.SubscriptionName,

			SubscriptionTenantID: ao.AzureGroup.SubscriptionTenantID,
			SubscriptionOwnerID:  ao.AzureGroup.SubscriptionOwnerID,

			BillingAccountName: ao.AzureGroup.BillingAccountName,
			BillingProfileName: ao.AzureGroup.BillingProfileName,
			InvoiceSectionName: ao.AzureGroup.InvoiceSectionName,
		})
		if err != nil {
			return err
		}
	}

	for _, op := range ao.DependentOperations {
		op.Call(ctx, orgsClient)
	}

	return nil
}

func (ao *AzureSubscriptionOperation) ToString() string {
	printColor := "yellow"
	var templated string
	if ao.Operation == Create {
		printColor = "green"
		templated = `(Create Subscription)
+	Name: {{ .Subscription.SubscriptionName }}
`
	}

	tpl, err := template.New("operation").Parse(templated)
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ao); err != nil {
		log.Fatal(err)
	}
	if printColor == "yellow" {
		return color.YellowString(buf.String())
	}
	return color.GreenString(buf.String())
}
