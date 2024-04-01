package resourceoperation

import (
	"bytes"
	"context"
	"html/template"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/fatih/color"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssess"
	"github.com/santiago-labs/telophasecli/resource"
)

type OrganizationUnitOperation struct {
	OrganizationUnit    *resource.AccountGroup
	Operation           int
	NewParent           *resource.AccountGroup
	CurrentParent       *resource.AccountGroup
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

func AccountGroupDiff(grp *resource.AccountGroup, orgClient awsorgs.Client) []ResourceOperation {
	// Order of operations matters. Groups must be created first, followed by account creation,
	// and finally (re)parenting groups and accounts.
	var operations []ResourceOperation

	stsClient := sts.New(session.Must(awssess.DefaultSession()))
	caller, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		panic(err)
	}

	providerRootGroup, err := FetchGroupAndDescendents(context.TODO(), orgClient, *grp.ID, *caller.Account)
	if err != nil {
		panic(err)
	}

	providerGroups := providerRootGroup.AllDescendentGroups()
	for _, parsedGroup := range grp.AllDescendentGroups() {
		var found bool
		for _, providerGroup := range providerGroups {
			if parsedGroup.ID != nil && *providerGroup.ID == *parsedGroup.ID {
				found = true
				if parsedGroup.Parent.ID == nil {
					for _, newGroup := range FlattenOperations(operations) {
						newGroupOperation, ok := newGroup.(*OrganizationUnitOperation)
						if !ok {
							continue
						}
						if newGroupOperation.OrganizationUnit == parsedGroup.Parent {
							newGroup.AddDependent(&OrganizationUnitOperation{
								OrganizationUnit: parsedGroup,
								NewParent:        parsedGroup.Parent,
								CurrentParent:    providerGroup.Parent,
								Operation:        UpdateParent,
							})
						}
					}

				} else if *parsedGroup.Parent.ID != *providerGroup.Parent.ID {
					operations = append(operations, &OrganizationUnitOperation{
						OrganizationUnit: parsedGroup,
						NewParent:        parsedGroup.Parent,
						CurrentParent:    providerGroup.Parent,
						Operation:        UpdateParent,
					})
				}
				break
			}
		}

		if !found {
			if parsedGroup.Parent.ID == nil {
				for _, newGroup := range FlattenOperations(operations) {
					newGroupOperation, ok := newGroup.(*OrganizationUnitOperation)
					if !ok {
						continue
					}
					if newGroupOperation.OrganizationUnit == parsedGroup.Parent {
						newGroup.AddDependent(&OrganizationUnitOperation{
							OrganizationUnit: parsedGroup,
							NewParent:        parsedGroup.Parent,
							Operation:        Create,
						})
					}
				}
			} else {
				operations = append(operations, &OrganizationUnitOperation{
					OrganizationUnit: parsedGroup,
					NewParent:        parsedGroup.Parent,
					Operation:        Create,
				})
			}
		}
	}

	providerAccounts := providerRootGroup.AllDescendentAccounts()
	for _, parsedAcct := range grp.AllDescendentAccounts() {
		var found bool
		for _, providerAcct := range providerAccounts {
			if providerAcct.Email == parsedAcct.Email {
				found = true
				if parsedAcct.Parent.ID == nil {
					for _, newGroup := range FlattenOperations(operations) {
						newGroupOperation, ok := newGroup.(*OrganizationUnitOperation)
						if !ok {
							continue
						}
						if newGroupOperation.OrganizationUnit == parsedAcct.Parent {
							newGroup.AddDependent(&AccountOperation{
								Account:       parsedAcct,
								Operation:     UpdateParent,
								CurrentParent: providerAcct.Parent,
								NewParent:     parsedAcct.Parent,
							})
						}
					}
				} else if *providerAcct.Parent.ID != *parsedAcct.Parent.ID {
					operations = append(operations, &AccountOperation{
						Account:       parsedAcct,
						NewParent:     parsedAcct.Parent,
						CurrentParent: providerAcct.Parent,
						Operation:     UpdateParent,
					})
				}
				break
			}
		}

		if !found {
			if parsedAcct.Parent.ID == nil {
				for _, newGroup := range FlattenOperations(operations) {
					newGroupOperation, ok := newGroup.(*OrganizationUnitOperation)
					if !ok {
						continue
					}
					if newGroupOperation.OrganizationUnit == parsedAcct.Parent {
						newGroup.AddDependent(&AccountOperation{
							Account:   parsedAcct,
							Operation: Create,
							NewParent: parsedAcct.Parent,
						})
					}
				}
			} else {
				operations = append(operations, &AccountOperation{
					Account:   parsedAcct,
					Operation: Create,
					NewParent: parsedAcct.Parent,
				})
			}
		}
	}

	return operations
}

func FlattenOperations(topList []ResourceOperation) []ResourceOperation {
	var finalOperations []ResourceOperation

	for _, op := range topList {
		finalOperations = append(finalOperations, op)
		finalOperations = append(finalOperations, FlattenOperations(op.ListDependents())...)
	}

	return finalOperations
}

func FetchGroupAndDescendents(ctx context.Context, orgClient awsorgs.Client, ouID, mgmtAccountID string) (resource.AccountGroup, error) {
	var group resource.AccountGroup

	var providerGroup *organizations.OrganizationalUnit

	// we treat the root group as an OU, but AWS does not consider root as an OU.
	if strings.HasPrefix(ouID, "r-") {
		name := "root"
		providerGroup = &organizations.OrganizationalUnit{
			Id:   &ouID,
			Name: &name,
		}
	} else {
		var err error
		providerGroup, err = orgClient.GetOrganizationUnit(ctx, ouID)
		if err != nil {
			return group, err
		}
	}

	group.ID = &ouID
	group.Name = *providerGroup.Name

	groupAccounts, err := orgClient.CurrentAccountsForParent(ctx, *group.ID)
	if err != nil {
		return group, err
	}

	for _, providerAcct := range groupAccounts {
		acct := resource.Account{
			AccountID:   *providerAcct.Id,
			Email:       *providerAcct.Email,
			Parent:      &group,
			AccountName: *providerAcct.Name,
		}
		if providerAcct.Id == &mgmtAccountID {
			acct.ManagementAccount = true
		}
		group.Accounts = append(group.Accounts, &acct)
	}

	children, err := orgClient.GetOrganizationUnitChildren(ctx, ouID)
	if err != nil {
		return group, err
	}

	for _, providerChild := range children {
		child, err := FetchGroupAndDescendents(ctx, orgClient, *providerChild.Id, mgmtAccountID)
		if err != nil {
			return group, err
		}
		child.Parent = &group
		group.ChildGroups = append(group.ChildGroups, &child)
	}

	return group, nil
}
