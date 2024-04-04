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
	MgmtAccount         *resource.Account
	Operation           int
	NewParent           *resource.AccountGroup
	CurrentParent       *resource.AccountGroup
	NewName             *string
	OrgClient           awsorgs.Client
	ConsoleUI           runner.ConsoleUI
	DependentOperations []ResourceOperation
}

func NewOrganizationUnitOperation(
	orgClient awsorgs.Client,
	consoleUI runner.ConsoleUI,
	organizationUnit *resource.AccountGroup,
	operation int,
	newParent *resource.AccountGroup,
	currentParent *resource.AccountGroup,
	newName *string,
) ResourceOperation {

	mgmtAcct, err := orgClient.FetchManagementAccount(context.TODO())
	if err != nil {
		panic(err)
	}
	return &organizationUnitOperation{
		OrgClient:        orgClient,
		ConsoleUI:        consoleUI,
		OrganizationUnit: organizationUnit,
		Operation:        operation,
		NewParent:        newParent,
		CurrentParent:    currentParent,
		NewName:          newName,
		MgmtAccount:      mgmtAcct,
	}
}

func CollectOrganizationUnitOps(
	ctx context.Context,
	consoleUI runner.ConsoleUI,
	orgClient awsorgs.Client,
	rootOU *resource.AccountGroup,
	op int,
) []ResourceOperation {

	// Order of operations matters. Groups must be Created first, followed by account creation,
	// and finally (re)parenting groups and accounts.
	var operations []ResourceOperation

	stsClient := sts.New(session.Must(awssess.DefaultSession()))
	caller, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		panic(err)
	}

	providerRootGroup, err := orgClient.FetchGroupAndDescendents(context.TODO(), *rootOU.GroupID, *caller.Account)
	if err != nil {
		panic(err)
	}

	providerGroups := providerRootGroup.AllDescendentGroups()
	for _, parsedGroup := range rootOU.AllDescendentGroups() {
		var found bool
		for _, providerGroup := range providerGroups {
			if parsedGroup.GroupID != nil && *providerGroup.GroupID == *parsedGroup.GroupID {
				found = true
				if parsedGroup.Parent.GroupID == nil {
					for _, newGroup := range FlattenOperations(operations) {
						newGroupOperation, ok := newGroup.(*organizationUnitOperation)
						if !ok {
							continue
						}

						if newGroupOperation.OrganizationUnit == parsedGroup.Parent {
							newGroup.AddDependent(NewOrganizationUnitOperation(
								orgClient,
								consoleUI,
								parsedGroup,
								UpdateParent,
								parsedGroup.Parent,
								providerGroup.Parent,
								nil,
							))
						}
					}

				} else if *parsedGroup.Parent.GroupID != *providerGroup.Parent.GroupID {
					operations = append(operations,
						NewOrganizationUnitOperation(
							orgClient,
							consoleUI,
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
			if parsedGroup.Parent.GroupID == nil {
				for _, newGroup := range FlattenOperations(operations) {
					newGroupOperation, ok := newGroup.(*organizationUnitOperation)
					if !ok {
						continue
					}
					if newGroupOperation.OrganizationUnit == parsedGroup.Parent {
						newGroup.AddDependent(NewOrganizationUnitOperation(
							orgClient,
							consoleUI,
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
						consoleUI,
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
				if parsedAcct.Parent.GroupID == nil {
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
				} else if *providerAcct.Parent.GroupID != *parsedAcct.Parent.GroupID {
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
			if parsedAcct.Parent.GroupID == nil {
				for _, newGroup := range FlattenOperations(operations) {
					newGroupOperation, ok := newGroup.(*organizationUnitOperation)
					if !ok {
						continue
					}
					if newGroupOperation.OrganizationUnit == parsedAcct.Parent {
						newAcct := NewAccountOperation(
							orgClient,
							consoleUI,
							parsedAcct,
							Create,
							parsedAcct.Parent,
							nil,
						)
						newGroup.AddDependent(newAcct)

						for _, acctOp := range CollectAccountOps(ctx, consoleUI, op, parsedAcct, "") {
							newAcct.AddDependent(acctOp)
						}
					}
				}
			} else {
				newAcct := NewAccountOperation(
					orgClient,
					consoleUI,
					parsedAcct,
					Create,
					parsedAcct.Parent,
					nil,
				)
				operations = append(operations, newAcct)

				for _, acctOp := range CollectAccountOps(ctx, consoleUI, op, parsedAcct, "") {
					newAcct.AddDependent(acctOp)
				}
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
		newOrg, err := ou.OrgClient.CreateOrganizationUnit(ctx, ou.ConsoleUI, *ou.MgmtAccount, ou.OrganizationUnit.GroupName, *ou.OrganizationUnit.Parent.GroupID)
		if err != nil {
			return err
		}
		ou.OrganizationUnit.GroupID = newOrg.Id
	} else if ou.Operation == UpdateParent {
		err := ou.OrgClient.RecreateOU(ctx, ou.ConsoleUI, *ou.MgmtAccount, *ou.OrganizationUnit.GroupID, ou.OrganizationUnit.GroupName, *ou.OrganizationUnit.Parent.GroupID)
		if err != nil {
			return err
		}
	} else if ou.Operation == Update {
		err := ou.OrgClient.UpdateOrganizationUnit(ctx, *ou.OrganizationUnit.GroupID, ou.OrganizationUnit.GroupName)
		if err != nil {
			return err
		}
	}

	for _, op := range ou.DependentOperations {
		if err := op.Call(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (ou *organizationUnitOperation) ToString() string {
	printColor := "yellow"
	var templated string
	if ou.Operation == Create {
		printColor = "green"
		templated = "\n" + `(Create Organizational Unit)
+	Name: {{ .OrganizationUnit.Name }}
+	Parent ID: {{ if .NewParent.ID }}{{ .NewParent.ID }}{{else}}<computed>{{end}}
+	Parent Name: {{ .NewParent.Name }}

`
	} else if ou.Operation == UpdateParent {
		templated = "\n" + `(Update Organizational Unit Parent)
ID: {{ .OrganizationUnit.ID }}
Name: {{ .OrganizationUnit.Name }}
~	Parent ID: {{ .CurrentParent.ID }} -> {{ if .NewParent.ID }}{{ .NewParent.ID }}{{else}}<computed>{{end}}
~	Parent Name: {{ .CurrentParent.Name }} -> {{ .NewParent.Name }}

`
	} else if ou.Operation == Update {
		templated = "\n" + `(Update Organizational Unit)
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
