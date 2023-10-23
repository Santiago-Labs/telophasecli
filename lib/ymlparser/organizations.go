package ymlparser

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

type OrgData struct {
	Organizations struct {
		MasterAccount Account `yaml:"MasterAccount"`
		// Account Account `yaml:"MasterAccount"`
		ChildAccounts []Account `yaml:"ChildAccounts"`
	} `yaml:"Organizations"`
}

type Account struct {
	Email       string `yaml:"Email"`
	AccountName string `yaml:"AccountName"`
	State       string `yaml:"State"`
	Properties  struct {
		Email       string `yaml:"Email"`
		AccountName string `yaml:"AccountName"`
		Management  bool   `yaml:"Management"`
		State       string `yaml:"State"`
	} `yaml:"Properties"`
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
	validStates := []string{"delete", ""}
	for _, account := range data.Organizations.ChildAccounts {
		if ok := isOneOf(account.Properties.State,
			"delete",
			"",
		); !ok {
			return fmt.Errorf("invalid state (%s) for account %s valid states are: empty string or %v", account.Properties.State, account.AccountName, validStates)
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
