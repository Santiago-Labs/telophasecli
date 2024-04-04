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
	Organization resource.AccountGroup `yaml:"Organization"`
}

func ParseOrganizationV2(filepath string) (*resource.AccountGroup, error) {
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
	org.Organization.GroupName = "root"
	hydrateOU(orgClient, &org.Organization, rootOU)

	// Hydrate Group, then fetch all accounts (pointers) and populate ID.
	allAccounts, err := orgClient.CurrentAccounts(context.TODO())
	if err != nil {
		return nil, oops.Wrapf(err, "CurrentAccounts")
	}
	for _, acct := range allAccounts {
		hydrateAccount(&org.Organization, acct)
	}

	return &org.Organization, nil
}

func hydrateAccount(group *resource.AccountGroup, acct *organizations.Account) {
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

func hydrateOU(orgClient awsorgs.Client, group *resource.AccountGroup, ou *organizations.OrganizationalUnit) error {
	if ou != nil {
		group.GroupID = ou.Id
		children, err := orgClient.GetOrganizationUnitChildren(context.TODO(), *group.GroupID)
		if err != nil {
			return err
		}

		for _, parsedChild := range group.ChildGroups {
			var found bool
			parsedChild.Parent = group
			for _, child := range children {
				if parsedChild.GroupName == *child.Name {
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

func WriteOrgV2File(filepath string, org *resource.AccountGroup) error {
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

func validOrganizationV2(data resource.AccountGroup) error {
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
