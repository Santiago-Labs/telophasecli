package resourceoperation

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"text/template"

	"github.com/fatih/color"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
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
	TagsDiff            *TagsDiff
}

// TagsDiff needs to be exported so it can be read by the template.
type TagsDiff struct {
	Added   []string
	Removed []string
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
	tagsDiff *TagsDiff,
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
		TagsDiff:         tagsDiff,
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

	providerRootOU, err := orgClient.FetchOUAndDescendents(ctx, *rootOU.OUID, mgmtAcct.AccountID)
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
							nil,
						),
					)
				}

				added, removed := diffTags(parsedOU)
				if len(added) > 0 || len(removed) > 0 {
					fmt.Println("adding new", removed, added)
					operations = append(operations, NewOrganizationUnitOperation(
						orgClient,
						consoleUI,
						parsedOU,
						mgmtAcct,
						UpdateTags,
						parsedOU.Parent,
						providerOU.Parent,
						nil,
						&TagsDiff{
							Added:   added,
							Removed: removed,
						},
					))
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
								nil,
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
						nil,
					))
				}

				added, removed := diffTags(parsedAcct)
				if len(added) > 0 || len(removed) > 0 {
					fmt.Println("added, removed ", added, removed)
					operations = append(operations, NewAccountOperation(
						orgClient,
						consoleUI,
						parsedAcct,
						mgmtAcct,
						UpdateTags,
						parsedAcct.Parent,
						providerAcct.Parent,
						&TagsDiff{
							Added:   added,
							Removed: removed,
						},
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
		newOrg, err := ou.OrgClient.CreateOrganizationUnit(ctx, ou.ConsoleUI, *ou.MgmtAccount, ou.OrganizationUnit.OUName, *ou.OrganizationUnit.Parent.OUID, ou.OrganizationUnit.AllTags())
		if err != nil {
			return err
		}
		ou.OrganizationUnit.OUID = newOrg.Id
	} else if ou.Operation == UpdateParent {
		err := ou.OrgClient.RecreateOU(ctx, ou.ConsoleUI, *ou.MgmtAccount, *ou.OrganizationUnit.OUID, ou.OrganizationUnit.OUName, *ou.OrganizationUnit.Parent.OUID, ou.OrganizationUnit.AllTags())
		if err != nil {
			return err
		}
	} else if ou.Operation == Update {
		err := ou.OrgClient.UpdateOrganizationUnit(ctx, *ou.OrganizationUnit.OUID, ou.OrganizationUnit.OUName)
		if err != nil {
			return err
		}
	} else if ou.Operation == UpdateTags {
		err := ou.OrgClient.TagResource(ctx, *ou.OrganizationUnit.OUID, ou.OrganizationUnit.AllTags())
		if err != nil {
			return err
		}
		err = ou.OrgClient.UntagResources(ctx, *ou.OrganizationUnit.OUID, ou.TagsDiff.Removed)
		if err != nil {
			return err
		}

		runner.ConsoleUI.Print(ou.ConsoleUI, "updated tags", *ou.MgmtAccount)
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
+	Parent Name: {{ .NewParent.Name }}`
		if len(ou.OrganizationUnit.AllTags()) > 0 {
			templated = templated + "\n" + `
+	Tags: {{ range AWSTags }}
+		- {{ . }}{{ end }}
`

		}

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
	} else if ou.Operation == UpdateTags {
		templated = "\n" + `(Update OU Tags)
ID: {{ .OrganizationUnit.ID }}
Tags: `
		if ou.TagsDiff.Added != nil {
			// TODO: ensure that just the tag line is green/red to more clearly denote the diff. Do this in account as well.
			templated = templated + `(Adding Tags){{ range .TagsDiff.Added}}
+ 	{{ . }}{{ end }}
`
			if ou.TagsDiff.Removed == nil {
				printColor = "green"
			}
		}
		if ou.TagsDiff.Removed != nil {
			templated = templated + `(Removing Tags){{ range .TagsDiff.Removed }}
-	{{ . }}{{ end }}
`
			if ou.TagsDiff.Added == nil {
				printColor = "red"
			}
		}
	}

	tpl, err := template.New("operation").Funcs(template.FuncMap{
		"AWSTags": func() []string {
			return ou.OrganizationUnit.AllTags()
		},
	}).Parse(templated)
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
	if printColor == "red" {
		return color.RedString(buf.String())
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

type Taggable interface {
	AllTags() []string
	AllAWSTags() []string
}

func diffTags(taggable Taggable) (added, removed []string) {
	oldMap := make(map[string]struct{})
	for _, tag := range taggable.AllAWSTags() {
		if ignorableTag(tag) {
			continue
		}
		oldMap[tag] = struct{}{}
	}

	taggableMap := make(map[string]struct{})
	for _, tag := range taggable.AllTags() {
		taggableMap[tag] = struct{}{}
	}

	for _, tag := range taggable.AllTags() {
		if _, ok := oldMap[tag]; !ok {
			if contains(added, tag) {
				// There can be duplicates when tags are inherited from an OU
				continue
			}
			// All the added tags don't exist in the
			added = append(added, tag)
			continue
		}
	}

	for _, tag := range taggable.AllAWSTags() {
		if ignorableTag(tag) {
			continue
		}
		if _, ok := taggableMap[tag]; !ok {
			if contains(removed, tag) {
				// There can be duplicates when tags are inherited from an OU
				continue
			}
			// All the removed tags don't exist on the taggable
			removed = append(removed, tag)
			continue
		}
	}

	return added, removed
}

func contains(slc []string, check string) bool {
	for _, s := range slc {
		if s == check {
			return true
		}
	}
	return false
}

func ignorableTag(tag string) bool {
	ignorableTags := map[string]struct{}{
		"TelophaseManaged=true": {},
	}

	_, ok := ignorableTags[tag]
	return ok
}
