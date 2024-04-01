package resourceoperation

import (
	"bytes"
	"context"
	"html/template"
	"log"

	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/fatih/color"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/resource"
)

const (
	UpdateParent = 1
	Create       = 2
	Update       = 3
)

type AccountOperation struct {
	Account             *resource.Account
	Operation           int
	NewParent           *resource.AccountGroup
	CurrentParent       *resource.AccountGroup
	DependentOperations []ResourceOperation
}

func (ao *AccountOperation) AddDependent(op ResourceOperation) {
	ao.DependentOperations = append(ao.DependentOperations, op)
}

func (ao *AccountOperation) ListDependents() []ResourceOperation {
	return ao.DependentOperations
}

func (ao *AccountOperation) Call(ctx context.Context, orgsClient awsorgs.Client) error {
	if ao.Operation == Create {
		acct := &organizations.Account{
			Email: &ao.Account.Email,
			Name:  &ao.Account.AccountName,
		}
		errs := orgsClient.CreateAccounts(ctx, []*organizations.Account{acct})
		if len(errs) > 0 {
			// TODO
			return errs[0]
		}
		rootId, err := orgsClient.GetRootId()
		if err != nil {
			return err
		}

		err = orgsClient.MoveAccount(ctx, *acct.Id, rootId, *ao.Account.Parent.ID)
		if err != nil {
			return err
		}

	} else if ao.Operation == UpdateParent {
		err := orgsClient.MoveAccount(ctx, ao.Account.AccountID, *ao.CurrentParent.ID, *ao.NewParent.ID)
		if err != nil {
			return err
		}
	}

	for _, op := range ao.DependentOperations {
		op.Call(ctx, orgsClient)
	}

	return nil
}

func (ao *AccountOperation) ToString() string {
	printColor := "yellow"
	var templated string
	if ao.Operation == Create {
		printColor = "green"
		templated = `(Create Account)
+	Name: {{ .Account.AccountName }}
+	Email: {{ .Account.Email }}
+	Parent ID: {{ if .NewParent.ID }}{{ .NewParent.ID }}{{else}}<computed>{{end}}
+	Parent Name: {{ .NewParent.Name }}

`
	} else if ao.Operation == UpdateParent {
		templated = `(Update Account Parent)
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
