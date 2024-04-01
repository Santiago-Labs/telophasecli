package resourceoperation

import (
	"bytes"
	"context"
	"html/template"
	"log"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/fatih/color"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssess"
	"github.com/santiago-labs/telophasecli/resource"
)

type organizationUnitOperation struct {
	OrganizationUnit    *resource.AccountGroup
	Operation           int
	NewParent           *resource.AccountGroup
	CurrentParent       *resource.AccountGroup
	NewName             *string
	OrgClient           awsorgs.Client
	DependentOperations []ResourceOperation
}

func NewOrganizationUnitOperation(
	orgClient awsorgs.Client,
	organizationUnit *resource.AccountGroup,
	operation int,
	newParent *resource.AccountGroup,
	currentParent *resource.AccountGroup,
	newName *string,
) ResourceOperation {

	return &organizationUnitOperation{
		OrgClient:        orgClient,
		OrganizationUnit: organizationUnit,
		Operation:        operation,
		NewParent:        newParent,
		CurrentParent:    currentParent,
		NewName:          newName,
	}
}

func CollectOrganizationUnitOps(
	ctx context.Context,
	consoleUI runner.ConsoleUI,
	orgClient awsorgs.Client,
	rootOU *resource.AccountGroup,
) []ResourceOperation {

	// Order of operations matters. Groups must be Created first, followed by account creation,
	// and finally (re)parenting groups and accounts.
	var operations []ResourceOperation

	stsClient := sts.New(session.Must(awssess.DefaultSession()))
	caller, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		panic(err)
	}

	providerRootGroup, err := orgClient.FetchGroupAndDescendents(context.TODO(), *rootOU.ID, *caller.Account)
	if err != nil {
		panic(err)
	}

	providerGroups := providerRootGroup.AllDescendentGroups()
	for _, parsedGroup := range rootOU.AllDescendentGroups() {
		var found bool
		for _, providerGroup := range providerGroups {
			if parsedGroup.ID != nil && *providerGroup.ID == *parsedGroup.ID {
				found = true
				if parsedGroup.Parent.ID == nil {
					for _, newGroup := range FlattenOperations(operations) {
						newGroupOperation, ok := newGroup.(*organizationUnitOperation)
						if !ok {
							continue
						}

						if newGroupOperation.OrganizationUnit == parsedGroup.Parent {
							newGroup.AddDependent(NewOrganizationUnitOperation(
								orgClient,
								parsedGroup,
								UpdateParent,
								parsedGroup.Parent,
								providerGroup.Parent,
								nil,
							))
						}
					}

				} else if *parsedGroup.Parent.ID != *providerGroup.Parent.ID {
					operations = append(operations,
						NewOrganizationUnitOperation(
							orgClient,
							parsedGroup,
							UpdateParent,
							parsedGroup.Parent,
							providerGroup.Parent,
							nil,
						),
					)
				}
				break
			}
		}

		if !found {
			if parsedGroup.Parent.ID == nil {
				for _, newGroup := range FlattenOperations(operations) {
					newGroupOperation, ok := newGroup.(*organizationUnitOperation)
					if !ok {
						continue
					}
					if newGroupOperation.OrganizationUnit == parsedGroup.Parent {
						newGroup.AddDependent(NewOrganizationUnitOperation(
							orgClient,
							parsedGroup,
							Create,
							parsedGroup.Parent,
							nil,
							nil,
						))
					}
				}
			} else {
				operations = append(operations,
					NewOrganizationUnitOperation(
						orgClient,
						parsedGroup,
						Create,
						parsedGroup.Parent,
						nil,
						nil,
					),
				)
			}
		}
	}

	providerAccounts := providerRootGroup.AllDescendentAccounts()
	for _, parsedAcct := range rootOU.AllDescendentAccounts() {
		var found bool
		for _, providerAcct := range providerAccounts {
			if providerAcct.Email == parsedAcct.Email {
				found = true
				if parsedAcct.Parent.ID == nil {
					for _, newGroup := range FlattenOperations(operations) {
						newGroupOperation, ok := newGroup.(*organizationUnitOperation)
						if !ok {
							continue
						}
						if newGroupOperation.OrganizationUnit == parsedAcct.Parent {
							newGroup.AddDependent(NewAccountOperation(
								orgClient,
								consoleUI,
								parsedAcct,
								UpdateParent,
								parsedAcct.Parent,
								providerAcct.Parent,
							))

						}
					}
				} else if *providerAcct.Parent.ID != *parsedAcct.Parent.ID {
					operations = append(operations, NewAccountOperation(
						orgClient,
						consoleUI,
						parsedAcct,
						UpdateParent,
						parsedAcct.Parent,
						providerAcct.Parent,
					))
				}
				break
			}
		}

		if !found {
			if parsedAcct.Parent.ID == nil {
				for _, newGroup := range FlattenOperations(operations) {
					newGroupOperation, ok := newGroup.(*organizationUnitOperation)
					if !ok {
						continue
					}
					if newGroupOperation.OrganizationUnit == parsedAcct.Parent {
						newGroup.AddDependent(NewAccountOperation(
							orgClient,
							consoleUI,
							parsedAcct,
							Create,
							parsedAcct.Parent,
							nil,
						))
					}
				}
			} else {
				operations = append(operations, NewAccountOperation(
					orgClient,
					consoleUI,
					parsedAcct,
					Create,
					parsedAcct.Parent,
					nil,
				))
			}
		}
	}

	return operations
}

func (ou *organizationUnitOperation) AddDependent(op ResourceOperation) {
	ou.DependentOperations = append(ou.DependentOperations, op)
}

func (ou *organizationUnitOperation) ListDependents() []ResourceOperation {
	return ou.DependentOperations
}

func (ou *organizationUnitOperation) Call(ctx context.Context) error {
	if ou.Operation == Create {
		newOrg, err := ou.OrgClient.CreateOrganizationUnit(ctx, ou.OrganizationUnit.Name, *ou.OrganizationUnit.Parent.ID)
		if err != nil {
			return err
		}
		ou.OrganizationUnit.ID = newOrg.Id
	} else if ou.Operation == UpdateParent {
		err := ou.OrgClient.RecreateOU(ctx, *ou.OrganizationUnit.ID, ou.OrganizationUnit.Name, *ou.OrganizationUnit.Parent.ID)
		if err != nil {
			return err
		}
	} else if ou.Operation == Update {
		err := ou.OrgClient.UpdateOrganizationUnit(ctx, *ou.OrganizationUnit.ID, ou.OrganizationUnit.Name)
		if err != nil {
			return err
		}
	}

	for _, op := range ou.DependentOperations {
		op.Call(ctx)
	}

	return nil
}

func (ou *organizationUnitOperation) ToString() string {
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

func FlattenOperations(topList []ResourceOperation) []ResourceOperation {
	var finalOperations []ResourceOperation

	for _, op := range topList {
		finalOperations = append(finalOperations, op)
		finalOperations = append(finalOperations, FlattenOperations(op.ListDependents())...)
	}

	return finalOperations
}
