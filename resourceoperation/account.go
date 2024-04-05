package resourceoperation

import (
	"bytes"
	"context"
	"html/template"
	"log"

	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/fatih/color"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/resource"
)

type accountOperation struct {
	Account             *resource.Account
	MgmtAccount         *resource.Account
	Operation           int
	NewParent           *resource.OrganizationUnit
	CurrentParent       *resource.OrganizationUnit
	DependentOperations []ResourceOperation
	ConsoleUI           runner.ConsoleUI
	OrgClient           *awsorgs.Client
}

func NewAccountOperation(
	orgClient awsorgs.Client,
	consoleUI runner.ConsoleUI,
	account, mgmtAcct *resource.Account,
	operation int,
	newParent *resource.OrganizationUnit,
	currentParent *resource.OrganizationUnit,
) ResourceOperation {

	return &accountOperation{
		Account:       account,
		Operation:     operation,
		NewParent:     newParent,
		CurrentParent: currentParent,
		ConsoleUI:     consoleUI,
		OrgClient:     &orgClient,
		MgmtAccount:   mgmtAcct,
	}
}

func CollectAccountOps(
	ctx context.Context,
	consoleUI runner.ConsoleUI,
	operation int,
	acct *resource.Account,
	stackFilter string,
) []ResourceOperation {

	var acctStacks []resource.Stack
	if stackFilter != "" && stackFilter != "*" {
		acctStacks = append(acctStacks, acct.FilterBaselineStacks(stackFilter)...)
	} else {
		acctStacks = append(acctStacks, acct.AllBaselineStacks()...)
	}

	var ops []ResourceOperation
	for _, stack := range acctStacks {
		if stack.Type == "Terraform" {
			ops = append(ops, NewTFOperation(consoleUI, acct, stack, operation))
		} else if stack.Type == "CDK" {
			ops = append(ops, NewCDKOperation(consoleUI, acct, stack, operation))
		}
	}

	return ops
}

func (ao *accountOperation) AddDependent(op ResourceOperation) {
	ao.DependentOperations = append(ao.DependentOperations, op)
}

func (ao *accountOperation) ListDependents() []ResourceOperation {
	return ao.DependentOperations
}

func (ao *accountOperation) Call(ctx context.Context) error {
	if ao.Operation == Create {
		acct := &organizations.Account{
			Email: &ao.Account.Email,
			Name:  &ao.Account.AccountName,
		}
		acctID, err := ao.OrgClient.CreateAccount(ctx, ao.ConsoleUI, *ao.MgmtAccount, acct)
		if err != nil {
			return err
		}
		ao.Account.AccountID = acctID

		rootId, err := ao.OrgClient.GetRootId()
		if err != nil {
			return err
		}

		err = ao.OrgClient.MoveAccount(ctx, ao.ConsoleUI, *ao.MgmtAccount, ao.Account.AccountID, rootId, *ao.Account.Parent.OUID)
		if err != nil {
			return err
		}

	} else if ao.Operation == UpdateParent {
		err := ao.OrgClient.MoveAccount(ctx, ao.ConsoleUI, *ao.MgmtAccount, ao.Account.AccountID, *ao.CurrentParent.OUID, *ao.NewParent.OUID)
		if err != nil {
			return err
		}
	}

	for _, op := range ao.DependentOperations {
		if err := op.Call(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (ao *accountOperation) ToString() string {
	printColor := "yellow"
	var templated string
	if ao.Operation == Create {
		printColor = "green"
		templated = "\n" + `(Create Account)
+	Name: {{ .Account.AccountName }}
+	Email: {{ .Account.Email }}
+	Parent ID: {{ if .NewParent.ID }}{{ .NewParent.ID }}{{else}}<computed>{{end}}
+	Parent Name: {{ .NewParent.Name }}

`
	} else if ao.Operation == UpdateParent {
		templated = "\n" + `(Update Account Parent)
ID: {{ .Account.AccountID }}
Name: {{ .Account.AccountName }}
Email: {{ .Account.Email }}
~	Parent ID: {{ .CurrentParent.ID }} -> {{ if .NewParent.ID }}{{ .NewParent.ID }}{{else}}<computed>{{end}}
~	Parent Name: {{ .CurrentParent.Name }} -> {{ .NewParent.Name }}

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
