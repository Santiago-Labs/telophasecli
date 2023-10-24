package ymlparser

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go/service/organizations"
	"gopkg.in/yaml.v3"
)

type OrgData struct {
	Organizations struct {
		MasterAccount Account   `yaml:"MasterAccount"`
		ChildAccounts []Account `yaml:"ChildAccounts"`
	} `yaml:"Organizations"`
}

type Account struct {
	Email          string `yaml:"Email"`
	AccountName    string `yaml:"AccountName"`
	State          string `yaml:"State,omitempty"`
	AccountID      string `yaml:"AccountID,omitempty"`
	AssumeRoleName string `yaml:"AssumeRoleName,omitempty"`
}

// We parse it and assume that the file is in the current directory
func ParseOrganizations(filepath string) (OrgData, error) {
	if filepath == "" {
		return OrgData{}, errors.New("filepath is empty")
	}

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return OrgData{}, errors.New(fmt.Sprintf("err: %s reading file %s", err.Error(), filepath))
	}

	var org OrgData

	if err := yaml.Unmarshal(data, &org); err != nil {
		return OrgData{}, err
	}

	if err := validOrgData(org); err != nil {
		return OrgData{}, err
	}

	return org, nil
}

func ParseOrganizationsIfExists(filepath string) (OrgData, error) {
	if filepath == "" {
		return OrgData{}, nil
	}
	_, err := os.Stat(filepath)
	if err == nil {
		return ParseOrganizations(filepath)
	}
	if os.IsNotExist(err) {
		return OrgData{}, nil
	}

	return ParseOrganizations(filepath)
}

func validOrgData(data OrgData) error {
	accountIDs := map[string]struct{}{}
	accountIDs[data.Organizations.MasterAccount.AccountID] = struct{}{}

	validStates := []string{"delete", ""}
	for _, account := range data.Organizations.ChildAccounts {
		if ok := isOneOf(account.State,
			"delete",
			"",
		); !ok {
			return fmt.Errorf("invalid state (%s) for account %s valid states are: empty string or %v", account.State, account.AccountName, validStates)
		}

		if _, ok := accountIDs[account.AccountID]; ok {
			return fmt.Errorf("duplicate account id %s", account.AccountID)
		} else {
			accountIDs[account.AccountID] = struct{}{}
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

func WriteOrgsFile(filepath, currentAccountID string, accounts []*organizations.Account) error {
	var orgData OrgData
	for _, account := range accounts {
		if *account.Id == currentAccountID {
			orgData.Organizations.MasterAccount = Account{
				Email:       *account.Email,
				AccountName: *account.Name,
				AccountID:   *account.Id,
			}
		} else {
			orgData.Organizations.ChildAccounts = append(
				orgData.Organizations.ChildAccounts,
				Account{
					Email:       *account.Email,
					AccountName: *account.Name,
					AccountID:   *account.Id,
				})
		}
	}

	result, err := yaml.Marshal(orgData)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath, result, 0644); err != nil {
		return err
	}

	return nil
}
