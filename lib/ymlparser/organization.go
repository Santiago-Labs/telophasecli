package ymlparser

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go/service/organizations"
	"gopkg.in/yaml.v3"

	"github.com/santiago-labs/telophasecli/lib/awsorgs"
)

type orgData struct {
	Organization Organization `yaml:"Organization"`
}

type Organization struct {
	ManagementAccount Account   `yaml:"ManagementAccount"`
	ChildAccounts     []Account `yaml:"ChildAccounts"`
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
func ParseOrganization(filepath string) (Organization, error) {
	if filepath == "" {
		return Organization{}, errors.New("filepath is empty")
	}

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		fmt.Println("read file")
		return Organization{}, fmt.Errorf("err: %s reading file %s", err.Error(), filepath)
	}

	var org orgData

	if err := yaml.Unmarshal(data, &org); err != nil {
		return Organization{}, err
	}

	if err := validOrganization(org.Organization); err != nil {
		return Organization{}, err
	}

	orgClient := awsorgs.New()
	allAccounts, err := orgClient.CurrentAccounts(context.TODO())
	if err != nil {
		return Organization{}, err
	}

	for _, acct := range allAccounts {
		for idx, parsedAcct := range org.Organization.ChildAccounts {
			if parsedAcct.AccountName == *acct.Name {
				org.Organization.ChildAccounts[idx].AccountID = *acct.Id
			}
		}

		if org.Organization.ManagementAccount.AccountName == *acct.Name {
			org.Organization.ManagementAccount.AccountID = *acct.Id
		}
	}

	return org.Organization, nil
}

func ParseOrganizationIfExists(filepath string) (Organization, error) {
	if filepath == "" {
		return Organization{}, nil
	}
	_, err := os.Stat(filepath)
	if err == nil {
		return ParseOrganization(filepath)
	}
	if os.IsNotExist(err) {
		return Organization{}, nil
	}

	return ParseOrganization(filepath)
}

func validOrganization(data Organization) error {
	// accountNames aren't enforced to be unique. But we believe unique account
	// names is a good pattern to follow.
	accountNames := map[string]struct{}{}
	accountNames[data.ManagementAccount.AccountName] = struct{}{}

	accountEmails := map[string]struct{}{}
	accountEmails[data.ManagementAccount.Email] = struct{}{}

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

func WriteOrgFile(filepath, masterAccountID string, accounts []*organizations.Account) error {
	var orgData orgData
	for _, account := range accounts {
		if *account.Id == masterAccountID {
			orgData.Organization.ManagementAccount = Account{
				Email:       *account.Email,
				AccountName: *account.Name,
			}
		} else {
			orgData.Organization.ChildAccounts = append(
				orgData.Organization.ChildAccounts,
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
