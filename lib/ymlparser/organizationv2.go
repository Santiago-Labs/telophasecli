package ymlparser

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"gopkg.in/yaml.v3"
)

type orgDatav2 struct {
	Organization AccountGroup `yaml:"Organization"`
}

type AccountGroup struct {
	ID          *string         `yaml:"-"`
	Name        string          `yaml:"Name,omitempty"`
	ChildGroups []*AccountGroup `yaml:"AccountGroups,omitempty"`
	Tags        []string        `yaml:"Tags,omitempty"`
	Accounts    []*Account      `yaml:"Accounts,omitempty"`
	Stacks      []Stack         `yaml:"Stacks,omitempty"`
	Parent      *AccountGroup   `yaml:"-"`
}

func (grp AccountGroup) AllStacks() []Stack {
	var stacks []Stack
	stacks = append(stacks, grp.Stacks...)
	if grp.Parent != nil {
		stacks = append(stacks, grp.Parent.AllStacks()...)
	}
	return stacks
}

// grp == configuration in organization.yml.
// other == configuration in cloud provider.
func (grp AccountGroup) Diff(orgClient awsorgs.Client) []ResourceOperation {
	// Order of operations matters. Groups must be created first, followed by account creation,
	// and finally (re)parenting groups and accounts.
	var operations []ResourceOperation

	stsClient := sts.New(session.Must(session.NewSession()))
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
			fmt.Printf("%v\n", parsedGroup)
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

func (grp AccountGroup) AllDescendentAccounts() []*Account {
	var accounts []*Account
	accounts = append(accounts, grp.Accounts...)

	for _, group := range grp.ChildGroups {
		accounts = append(accounts, group.AllDescendentAccounts()...)
	}

	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].Email < accounts[j].Email
	})

	return accounts
}

func (grp AccountGroup) AllDescendentGroups() []*AccountGroup {
	var groups []*AccountGroup
	groups = append(groups, grp.ChildGroups...)

	for _, group := range grp.ChildGroups {
		groups = append(groups, group.AllDescendentGroups()...)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups

}

func ParseOrganizationV2(filepath string) (AccountGroup, error) {
	if filepath == "" {
		return AccountGroup{}, errors.New("filepath is empty")
	}

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return AccountGroup{}, fmt.Errorf("err: %s reading file %s", err.Error(), filepath)
	}

	var org orgDatav2

	if err := yaml.Unmarshal(data, &org); err != nil {
		return AccountGroup{}, err
	}

	if err := validOrganizationV2(org.Organization); err != nil {
		return AccountGroup{}, err
	}

	orgClient := awsorgs.New()

	rootId, err := orgClient.GetRootId()
	if err != nil {
		return AccountGroup{}, err
	}
	rootName := "root"
	rootOU := &organizations.OrganizationalUnit{
		Id:   &rootId,
		Name: &rootName,
	}
	org.Organization.Name = "root"
	hydrateOU(orgClient, &org.Organization, rootOU)

	// Hydrate Group, then fetch all accounts (pointers) and populate ID.
	allAccounts, err := orgClient.CurrentAccounts(context.TODO())
	if err != nil {
		return AccountGroup{}, err
	}
	for _, acct := range allAccounts {
		hydrateAccount(&org.Organization, acct)
	}

	return org.Organization, nil
}

func hydrateAccount(group *AccountGroup, acct *organizations.Account) {
	for idx, parsedAcct := range group.Accounts {
		group.Accounts[idx].Parent = group
		if parsedAcct.Email == *acct.Email {
			group.Accounts[idx].AccountID = *acct.Id
			return
		}
	}

	for _, childGroup := range group.ChildGroups {
		hydrateAccount(childGroup, acct)
	}
}

func hydrateOU(orgClient awsorgs.Client, group *AccountGroup, ou *organizations.OrganizationalUnit) error {
	if ou != nil {
		group.ID = ou.Id
		children, err := orgClient.GetOrganizationUnitChildren(context.TODO(), *group.ID)
		if err != nil {
			return err
		}

		for _, parsedChild := range group.ChildGroups {
			var found bool
			parsedChild.Parent = group
			for _, child := range children {
				if parsedChild.Name == *child.Name {
					found = true
					err = hydrateOU(orgClient, parsedChild, child)
					if err != nil {
						return err
					}
				}
			}

			// Iterate over children to hydrate parentID
			if !found {
				err := hydrateOU(orgClient, parsedChild, nil)
				if err != nil {
					return err
				}
			}
		}
	} else {
		for _, parsedChild := range group.ChildGroups {
			parsedChild.Parent = group
			err := hydrateOU(orgClient, parsedChild, nil)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func FetchGroupAndDescendents(ctx context.Context, orgClient awsorgs.Client, ouID, mgmtAccountID string) (AccountGroup, error) {
	var group AccountGroup

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
		acct := Account{
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

func ParseOrganizationV2IfExists(filepath string) (AccountGroup, error) {
	if filepath == "" {
		return AccountGroup{}, nil
	}
	_, err := os.Stat(filepath)
	if err == nil {
		return ParseOrganizationV2(filepath)
	}
	if os.IsNotExist(err) {
		return AccountGroup{}, nil
	}

	return ParseOrganizationV2(filepath)
}

func WriteOrgV2File(filepath string, org *AccountGroup) error {
	orgData := orgDatav2{
		Organization: *org,
	}
	result, err := yaml.Marshal(orgData)
	if err != nil {
		return err
	}

	if fileExists(filepath) {
		return fmt.Errorf("file %s already exists we will not overwrite it", filepath)
	}

	if err := ioutil.WriteFile(filepath, result, 0644); err != nil {
		return err
	}

	return nil
}

func validOrganizationV2(data AccountGroup) error {
	accountEmails := map[string]struct{}{}

	validStates := []string{"delete", ""}
	for _, account := range data.AllDescendentAccounts() {
		if ok := isOneOf(account.State,
			"delete",
			"",
		); !ok {
			return fmt.Errorf("invalid state (%s) for account %s valid states are: empty string or %v", account.State, account.AccountName, validStates)
		}

		if _, ok := accountEmails[account.Email]; ok {
			return fmt.Errorf("duplicate account email %s", account.Email)
		} else {
			accountEmails[account.Email] = struct{}{}
		}

	}

	return nil
}

func FlattenOperations(topList []ResourceOperation) []ResourceOperation {
	var finalOperations []ResourceOperation

	for _, op := range topList {
		finalOperations = append(finalOperations, op)
		finalOperations = append(finalOperations, FlattenOperations(op.ListDependents())...)
	}

	return finalOperations
}
