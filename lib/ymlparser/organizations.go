package ymlparser

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go/service/organizations"
	"gopkg.in/yaml.v3"

	"telophasecli/lib/awsorgs"
)

type orgData struct {
	Organizations Organizations `yaml:"Organizations"`
}

type Organizations struct {
	MasterAccount Account   `yaml:"MasterAccount"`
	ChildAccounts []Account `yaml:"ChildAccounts"`
}

type Account struct {
	Email          string   `yaml:"Email"`
	AccountName    string   `yaml:"AccountName"`
	State          string   `yaml:"State,omitempty"`
	AccountID      string   `yaml:"AccountID,omitempty"`
	AssumeRoleName string   `yaml:"AssumeRoleName,omitempty"`
	Tags           []string `yaml:"Tags,omitempty"`
}

func (a Account) AssumeRoleARN() string {
	assumeRoleName := "OrganizationAccountAccessRole"
	if a.AssumeRoleName != "" {
		assumeRoleName = a.AssumeRoleName
	}

	return fmt.Sprintf("arn:aws:iam::%s:role/%s", a.AccountID, assumeRoleName)
}

// We parse it and assume that the file is in the current directory
func ParseOrganizations(filepath string) (Organizations, error) {
	if filepath == "" {
		return Organizations{}, errors.New("filepath is empty")
	}

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return Organizations{}, fmt.Errorf("err: %s reading file %s", err.Error(), filepath)
	}

	var org orgData

	if err := yaml.Unmarshal(data, &org); err != nil {
		return Organizations{}, err
	}

	if err := validOrganizations(org.Organizations); err != nil {
		return Organizations{}, err
	}

	orgClient := awsorgs.New()
	allAccounts, err := orgClient.CurrentAccounts(context.TODO())
	if err != nil {
		return Organizations{}, err
	}

	for _, acct := range allAccounts {
		for idx, parsedAcct := range org.Organizations.ChildAccounts {
			if parsedAcct.AccountName == *acct.Name {
				org.Organizations.ChildAccounts[idx].AccountID = *acct.Id
			}
		}

		if org.Organizations.MasterAccount.AccountName == *acct.Name {
			org.Organizations.MasterAccount.AccountID = *acct.Id
		}
	}

	return org.Organizations, nil
}

func ParseOrganizationsIfExists(filepath string) (Organizations, error) {
	if filepath == "" {
		return Organizations{}, nil
	}
	_, err := os.Stat(filepath)
	if err == nil {
		return ParseOrganizations(filepath)
	}
	if os.IsNotExist(err) {
		return Organizations{}, nil
	}

	return ParseOrganizations(filepath)
}

func validOrganizations(data Organizations) error {
	accountNames := map[string]struct{}{}
	accountNames[data.MasterAccount.AccountName] = struct{}{}

	validStates := []string{"delete", ""}
	for _, account := range data.ChildAccounts {
		if ok := isOneOf(account.State,
			"delete",
			"",
		); !ok {
			return fmt.Errorf("invalid state (%s) for account %s valid states are: empty string or %v", account.State, account.AccountName, validStates)
		}

		if _, ok := accountNames[account.AccountName]; ok {
			return fmt.Errorf("duplicate account name %s", account.AccountName)
		} else {
			accountNames[account.AccountName] = struct{}{}
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

func WriteOrgsFile(filepath, masterAccountID string, accounts []*organizations.Account) error {
	var orgData orgData
	for _, account := range accounts {
		if *account.Id == masterAccountID {
			orgData.Organizations.MasterAccount = Account{
				Email:       *account.Email,
				AccountName: *account.Name,
			}
		} else {
			orgData.Organizations.ChildAccounts = append(
				orgData.Organizations.ChildAccounts,
				Account{
					Email:       *account.Email,
					AccountName: *account.Name,
				})
		}
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

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
