package resourceoperation

import (
	"bytes"
	"context"
	"fmt"
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
	OrganizationUnit    *resource.OrganizationUnit
	MgmtAccount         *resource.Account
	Operation           int
	NewParent           *resource.OrganizationUnit
	CurrentParent       *resource.OrganizationUnit
	NewName             *string
	OrgClient           awsorgs.Client
	ConsoleUI           runner.ConsoleUI
	DependentOperations []ResourceOperation
}

func NewOrganizationUnitOperation(
	orgClient awsorgs.Client,
	consoleUI runner.ConsoleUI,
	organizationUnit *resource.OrganizationUnit,
	mgmtAcct *resource.Account,
	operation int,
	newParent *resource.OrganizationUnit,
	currentParent *resource.OrganizationUnit,
	newName *string,
) ResourceOperation {

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
	mgmtAcct *resource.Account,
	rootOU *resource.OrganizationUnit,
	op int,
) []ResourceOperation {

	// Order of operations matters. Groups must be Created first, followed by account creation,
	// and finally (re)parenting groups and accounts.
	var operations []ResourceOperation

	stsClient := sts.New(session.Must(awssess.DefaultSession()))
	caller, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		consoleUI.Print(fmt.Sprintf("Error: %v", err), *mgmtAcct)
		return []ResourceOperation{}
	}

	providerRootOU, err := orgClient.FetchOUAndDescendents(context.TODO(), *rootOU.OUID, *caller.Account)
	if err != nil {
		consoleUI.Print(fmt.Sprintf("Error: %v", err), *mgmtAcct)
		return []ResourceOperation{}
	}

	providerOUs := providerRootOU.AllDescendentOUs()
	for _, parsedOU := range rootOU.AllDescendentOUs() {
		var found bool
		for _, providerOU := range providerOUs {
			if parsedOU.OUID != nil && *providerOU.OUID == *parsedOU.OUID {
				found = true
				if parsedOU.Parent.OUID == nil {
					for _, newOU := range FlattenOperations(operations) {
						newOUOperation, ok := newOU.(*organizationUnitOperation)
						if !ok {
							continue
						}

						if newOUOperation.OrganizationUnit == parsedOU.Parent {
							newOU.AddDependent(NewOrganizationUnitOperation(
								orgClient,
								consoleUI,
								parsedOU,
								mgmtAcct,
								UpdateParent,
								parsedOU.Parent,
								providerOU.Parent,
								nil,
							))
						}
					}

				} else if *parsedOU.Parent.OUID != *providerOU.Parent.OUID {
					operations = append(operations,
						NewOrganizationUnitOperation(
							orgClient,
							consoleUI,
							parsedOU,
							mgmtAcct,
							UpdateParent,
							parsedOU.Parent,
							providerOU.Parent,
							nil,
						),
					)
				}
				break
			}
		}

		if !found {
			if parsedOU.Parent.OUID == nil {
				for _, newOU := range FlattenOperations(operations) {
					newOUOperation, ok := newOU.(*organizationUnitOperation)
					if !ok {
						continue
					}
					if newOUOperation.OrganizationUnit == parsedOU.Parent {
						newOU.AddDependent(NewOrganizationUnitOperation(
							orgClient,
							consoleUI,
							parsedOU,
							mgmtAcct,
							Create,
							parsedOU.Parent,
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
						parsedOU,
						mgmtAcct,
						Create,
						parsedOU.Parent,
						nil,
						nil,
					),
				)
			}
		}
	}

	providerAccounts := providerRootOU.AllDescendentAccounts()
	for _, parsedAcct := range rootOU.AllDescendentAccounts() {
		var found bool
		for _, providerAcct := range providerAccounts {
			if providerAcct.Email == parsedAcct.Email {
				found = true
				if parsedAcct.Parent.OUID == nil {
					for _, newOU := range FlattenOperations(operations) {
						newOUOperation, ok := newOU.(*organizationUnitOperation)
						if !ok {
							continue
						}
						if newOUOperation.OrganizationUnit == parsedAcct.Parent {
							newOU.AddDependent(NewAccountOperation(
								orgClient,
								consoleUI,
								parsedAcct,
								mgmtAcct,
								UpdateParent,
								parsedAcct.Parent,
								providerAcct.Parent,
							))

						}
					}
				} else if *providerAcct.Parent.OUID != *parsedAcct.Parent.OUID {
					operations = append(operations, NewAccountOperation(
						orgClient,
						consoleUI,
						parsedAcct,
						mgmtAcct,
						UpdateParent,
						parsedAcct.Parent,
						providerAcct.Parent,
					))
				}
				break
			}
		}

		if !found {
			if parsedAcct.Parent.OUID == nil {
				for _, newOU := range FlattenOperations(operations) {
					newOUOperation, ok := newOU.(*organizationUnitOperation)
					if !ok {
						continue
					}
					if newOUOperation.OrganizationUnit == parsedAcct.Parent {
						newAcct := NewAccountOperation(
							orgClient,
							consoleUI,
							parsedAcct,
							mgmtAcct,
							Create,
							parsedAcct.Parent,
							nil,
						)
						newOU.AddDependent(newAcct)
					}
				}
			} else {
				newAcct := NewAccountOperation(
					orgClient,
					consoleUI,
					parsedAcct,
					mgmtAcct,
					Create,
					parsedAcct.Parent,
					nil,
				)
				operations = append(operations, newAcct)
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
		newOrg, err := ou.OrgClient.CreateOrganizationUnit(ctx, ou.ConsoleUI, *ou.MgmtAccount, ou.OrganizationUnit.OUName, *ou.OrganizationUnit.Parent.OUID)
		if err != nil {
			return err
		}
		ou.OrganizationUnit.OUID = newOrg.Id
	} else if ou.Operation == UpdateParent {
		err := ou.OrgClient.RecreateOU(ctx, ou.ConsoleUI, *ou.MgmtAccount, *ou.OrganizationUnit.OUID, ou.OrganizationUnit.OUName, *ou.OrganizationUnit.Parent.OUID)
		if err != nil {
			return err
		}
	} else if ou.Operation == Update {
		err := ou.OrgClient.UpdateOrganizationUnit(ctx, *ou.OrganizationUnit.OUID, ou.OrganizationUnit.OUName)
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
