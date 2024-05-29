package resourceoperation

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"text/template"

	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/fatih/color"
	"github.com/samsarahq/go/oops"
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
	TagsDiff            *TagsDiff
	AllowDelete         bool
}

func NewAccountOperation(
	orgClient awsorgs.Client,
	consoleUI runner.ConsoleUI,
	account, mgmtAcct *resource.Account,
	operation int,
	newParent *resource.OrganizationUnit,
	currentParent *resource.OrganizationUnit,
	tagsDiff *TagsDiff,
) *accountOperation {

	return &accountOperation{
		Account:       account,
		Operation:     operation,
		NewParent:     newParent,
		CurrentParent: currentParent,
		ConsoleUI:     consoleUI,
		OrgClient:     &orgClient,
		MgmtAccount:   mgmtAcct,
		TagsDiff:      tagsDiff,
	}
}

func (ao *accountOperation) SetAllowDelete(allowDelete bool) {
	ao.AllowDelete = allowDelete
}

func CollectAccountOps(
	ctx context.Context,
	consoleUI runner.ConsoleUI,
	operation int,
	acct *resource.Account,
	stackFilter string,
) ([]ResourceOperation, error) {

	var acctStacks []resource.Stack
	if stackFilter != "" && stackFilter != "*" {
		baselineStacks, err := acct.FilterBaselineStacks(stackFilter)
		if err != nil {
			return nil, err
		}
		acctStacks = append(acctStacks, baselineStacks...)
	} else {
		baselineStacks, err := acct.AllBaselineStacks()
		if err != nil {
			return nil, err
		}
		acctStacks = append(acctStacks, baselineStacks...)
	}

	var ops []ResourceOperation
	for _, stack := range acctStacks {
		if stack.Type == "Terraform" {
			ops = append(ops, NewTFOperation(consoleUI, acct, stack, operation))
		} else if stack.Type == "CDK" {
			ops = append(ops, NewCDKOperation(consoleUI, acct, stack, operation))
		} else if stack.Type == "Cloudformation" {
			ops = append(ops, NewCloudformationOperation(consoleUI, acct, stack, operation))
		}
	}

	return ops, nil
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
		acctID, err := ao.OrgClient.CreateAccount(ctx, ao.ConsoleUI, *ao.MgmtAccount, acct, ao.Account.AllTags())
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
	} else if ao.Operation == UpdateTags {
		err := ao.OrgClient.TagResource(ctx, ao.Account.AccountID, ao.Account.AllTags())
		if err != nil {
			return oops.Wrapf(err, "UpdateTags")
		}
		err = ao.OrgClient.UntagResources(ctx, ao.Account.AccountID, ao.TagsDiff.Removed)
		if err != nil {
			return oops.Wrapf(err, "UntagResources")
		}

		ao.ConsoleUI.Print("Updated Tags", *ao.Account)
	} else if ao.Operation == Delete {
		if !ao.AllowDelete {
			return fmt.Errorf("attempting to delete account: (name:%s email:%s id:%s) stopping because --allow-account-delete is not passed into telophasecli", ao.Account.AccountName, ao.Account.Email, ao.Account.AccountID)
		}

		// Stacks need to be cleaned up from an AWS account before its closed.
		err := ao.OrgClient.CloseAccount(ctx, ao.Account.AccountID, ao.Account.AccountName, ao.Account.Email)
		if err != nil {
			return oops.Wrapf(err, "CloseAccounts")
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

		if len(ao.Account.AllTags()) > 0 {
			templated = templated +
				`+	Tags: {{ range AllTags }}
+	- {{ . }}{{ end }}
`

		}
	} else if ao.Operation == UpdateParent {
		templated = "\n" + `(Update Account Parent)
ID: {{ .Account.AccountID }}
Name: {{ .Account.AccountName }}
Email: {{ .Account.Email }}
~	Parent ID: {{ .CurrentParent.ID }} -> {{ if .NewParent.ID }}{{ .NewParent.ID }}{{else}}<computed>{{end}}
~	Parent Name: {{ .CurrentParent.Name }} -> {{ .NewParent.Name }}

`
	} else if ao.Operation == Delete {
		printColor = "red"
		includeDeleteStr := ""
		if !ao.AllowDelete {
			includeDeleteStr = " To ensure deletion run telophasecli with --allow-account-delete flag"
		}
		templated = "\n" + fmt.Sprintf(`(DELETE ACCOUNT)%s
-	Name: {{ .Account.AccountName }}
-	Email: {{ .Account.Email }}
-	ID: {{ .Account.ID }}
`, includeDeleteStr)
	} else if ao.Operation == UpdateTags {
		// We need to compute which tags have changed
		templated = "\n" + `(Updating Account Tags)
ID: {{ .Account.AccountID }}
Name: {{ .Account.AccountName }}
Tags: `

		if ao.TagsDiff.Added != nil {
			templated = templated + `(Added Tags){{ range .TagsDiff.Added }}
+	{{ . }}{{ end }}
`
			if ao.TagsDiff.Removed == nil {
				printColor = "green"
			}
		}

		if ao.TagsDiff.Removed != nil {
			templated = templated + `(Removed Tags){{ range .TagsDiff.Removed}}
-	{{ . }}{{end}}
`
			if ao.TagsDiff.Added == nil {
				printColor = "red"
			}
		}
	}

	tpl, err := template.New("operation").Funcs(template.FuncMap{
		"AllTags": func() []string {
			tags := ao.Account.AllTags()

			return tags
		},
	}).Parse(templated)
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
	if printColor == "red" {
		return color.RedString(buf.String())
	}
	return color.GreenString(buf.String())
}
