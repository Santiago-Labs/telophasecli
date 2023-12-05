package ymlparser

import (
	"bytes"
	"context"
	"html/template"
	"log"

	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/fatih/color"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
)

const (
	UpdateParent = 1
	Create       = 2
	Update       = 3
)

type ResourceOperation interface {
	Call(context.Context, awsorgs.Client) error
	ToString() string
	AddDependent(ResourceOperation)
	ListDependents() []ResourceOperation
}

type AccountOperation struct {
	Account             *Account
	Operation           int
	NewParent           *AccountGroup
	CurrentParent       *AccountGroup
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

type OrganizationUnitOperation struct {
	OrganizationUnit    *AccountGroup
	Operation           int
	NewParent           *AccountGroup
	CurrentParent       *AccountGroup
	NewName             *string
	DependentOperations []ResourceOperation
}

func (ou *OrganizationUnitOperation) AddDependent(op ResourceOperation) {
	ou.DependentOperations = append(ou.DependentOperations, op)
}

func (ou *OrganizationUnitOperation) ListDependents() []ResourceOperation {
	return ou.DependentOperations
}

func (ou *OrganizationUnitOperation) Call(ctx context.Context, orgsClient awsorgs.Client) error {
	if ou.Operation == Create {
		newOrg, err := orgsClient.CreateOrganizationUnit(ctx, ou.OrganizationUnit.Name, *ou.OrganizationUnit.Parent.ID)
		if err != nil {
			return err
		}
		ou.OrganizationUnit.ID = newOrg.Id
	} else if ou.Operation == UpdateParent {
		err := orgsClient.RecreateOU(ctx, *ou.OrganizationUnit.ID, ou.OrganizationUnit.Name, *ou.OrganizationUnit.Parent.ID)
		if err != nil {
			return err
		}
	} else if ou.Operation == Update {
		err := orgsClient.UpdateOrganizationUnit(ctx, *ou.OrganizationUnit.ID, ou.OrganizationUnit.Name)
		if err != nil {
			return err
		}
	}

	for _, op := range ou.DependentOperations {
		op.Call(ctx, orgsClient)
	}

	return nil
}

func (ou *OrganizationUnitOperation) ToString() string {
	printColor := "yellow"
	var templated string
	if ou.Operation == Create {
		printColor = "green"
		templated = `(Create Organizational Unit)
+	Name: {{ .OrganizationUnit.Name }}
+	Parent ID: {{ if .NewParent.ID }}{{ .NewParent.ID }}{{else}}<computed>{{end}}
+	Parent Name: {{ .NewParent.Name }}

`
	} else if ou.Operation == UpdateParent {
		templated = `(Update Organizational Unit Parent)
ID: {{ .OrganizationUnit.ID }}
Name: {{ .OrganizationUnit.Name }}
~	Parent ID: {{ .CurrentParent.ID }} -> {{ if .NewParent.ID }}{{ .NewParent.ID }}{{else}}<computed>{{end}}
~	Parent Name: {{ .CurrentParent.Name }} -> {{ .NewParent.Name }}

`
	} else if ou.Operation == Update {
		templated = `(Update Organizational Unit)
ID: {{ .OrganizationUnit.ID }}
~	Name: {{ .OrganizationUnit.Name }} -> {{ .NewName }}

`
	}

	tpl, err := template.New("operation").Parse(templated)
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ou); err != nil {
		log.Fatal(err)
	}
	if printColor == "yellow" {
		return color.YellowString(buf.String())
	}
	return color.GreenString(buf.String())
}
