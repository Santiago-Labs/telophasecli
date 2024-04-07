package ymlparser

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/resource"
	"gopkg.in/yaml.v3"
)

type orgDatav2 struct {
	Organization resource.OrganizationUnit `yaml:"Organization"`
}

func ParseOrganizationV2(filepath string) (*resource.OrganizationUnit, error) {
	if filepath == "" {
		return nil, errors.New("filepath is empty")
	}

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("err: %s reading file %s", err.Error(), filepath)
	}

	var org orgDatav2

	if err := yaml.Unmarshal(data, &org); err != nil {
		return nil, err
	}

	if err := validOrganizationV2(org.Organization); err != nil {
		return nil, err
	}

	orgClient := awsorgs.New()

	rootId, err := orgClient.GetRootId()
	if err != nil {
		return nil, err
	}
	rootName := "root"
	rootOU := &organizations.OrganizationalUnit{
		Id:   &rootId,
		Name: &rootName,
	}
	org.Organization.OUName = "root"
	hydrateOU(orgClient, &org.Organization, rootOU)
	hydrateAccountParent(&org.Organization)

	// Hydrate Group, then fetch all accounts (pointers) and populate ID.
	allAccounts, err := orgClient.CurrentAccounts(context.TODO())
	if err != nil {
		return nil, oops.Wrapf(err, "CurrentAccounts")
	}
	for idx := range allAccounts {
		hydrateAccount(&org.Organization, allAccounts[idx])
	}

	return &org.Organization, nil
}

func hydrateAccount(ou *resource.OrganizationUnit, acct *organizations.Account) {
	found := true
	for idx, parsedAcct := range ou.Accounts {
		if parsedAcct.Email == *acct.Email && !found {
			ou.Accounts[idx].AccountID = *acct.Id
			found = true
		}
	}

	if found {
		return
	}

	for _, childOU := range ou.ChildOUs {
		hydrateAccount(childOU, acct)
	}
}

func hydrateAccountParent(ou *resource.OrganizationUnit) {
	for idx := range ou.Accounts {
		ou.Accounts[idx].Parent = ou
	}

	for _, childOU := range ou.ChildOUs {
		hydrateAccountParent(childOU)
	}
}

func hydrateOU(orgClient awsorgs.Client, parsedOU *resource.OrganizationUnit, providerOU *organizations.OrganizationalUnit) error {
	if providerOU != nil {
		parsedOU.OUID = providerOU.Id
		children, err := orgClient.GetOrganizationUnitChildren(context.TODO(), *parsedOU.OUID)
		if err != nil {
			return err
		}

		for _, parsedChild := range parsedOU.ChildOUs {
			var found bool
			parsedChild.Parent = parsedOU
			for _, child := range children {
				if parsedChild.OUName == *child.Name {
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
		for _, parsedChild := range parsedOU.ChildOUs {
			parsedChild.Parent = parsedOU
			err := hydrateOU(orgClient, parsedChild, nil)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func WriteOrgV2File(filepath string, org *resource.OrganizationUnit) error {
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

func validOrganizationV2(data resource.OrganizationUnit) error {
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

	for _, ou := range data.AllDescendentOUs() {
		if len(ou.ChildGroups) > 0 {
			if len(ou.ChildOUs) > 0 {
				return fmt.Errorf("cannot set both AccountGroups and OrganizationUnits fields on Organization Unit: %s", ou.OUName)
			}

			// Remove this after deleting ChildGroups field.
			ou.ChildOUs = append(ou.ChildOUs, ou.ChildGroups...)
		}
	}

	return nil
}

func isOneOf(s string, valid ...string) bool {
	for _, v := range valid {
		if s == v {
			return true
		}
	}
	return false
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
