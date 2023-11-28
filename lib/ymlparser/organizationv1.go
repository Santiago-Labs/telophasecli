package ymlparser

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"gopkg.in/yaml.v3"
)

// Deprecated: use orgDatav2 and AccountGroup instead.
type orgDatav1 struct {
	Organization Organization `yaml:"Organization"`
}

type Organization struct {
	ManagementAccount Account   `yaml:"ManagementAccount"`
	ChildAccounts     []Account `yaml:"ChildAccounts,omitempty"`
}

type Account struct {
	Email             string        `yaml:"Email"`
	AccountName       string        `yaml:"AccountName"`
	State             string        `yaml:"State,omitempty"`
	AccountID         string        `yaml:"-"`
	AssumeRoleName    string        `yaml:"AssumeRoleName,omitempty"`
	Tags              []string      `yaml:"Tags,omitempty"`
	Stacks            []Stack       `yaml:"Stacks,omitempty"`
	ManagementAccount bool          `yaml:"ManagementAccount,omitempty"`
	Parent            *AccountGroup `yaml:"-"`
}

type Stack struct {
	Name string `yaml:"Name"`
	Type string `yaml:"Type"`
	Path string `yaml:"Path"`
}

func (a Account) AssumeRoleARN() string {
	assumeRoleName := "OrganizationAccountAccessRole"
	if a.AssumeRoleName != "" {
		assumeRoleName = a.AssumeRoleName
	}

	return fmt.Sprintf("arn:aws:iam::%s:role/%s", a.AccountID, assumeRoleName)
}

func (a Account) AllTags() []string {
	var tags []string
	tags = append(tags, a.Tags...)
	if a.Parent != nil {
		tags = append(tags, a.Parent.AllTags()...)
	}
	return tags
}

func (a Account) AllStacks() []Stack {
	var stacks []Stack
	stacks = append(stacks, a.Stacks...)
	if a.Parent != nil {
		stacks = append(stacks, a.Parent.AllStacks()...)
	}
	return stacks
}

func (a Account) FilterStacks(stackNames string) []Stack {
	var matchingStacks []Stack
	targetStackNames := strings.Split(stackNames, ",")
	for _, stack := range a.AllStacks() {
		acctStackNames := strings.Split(stack.Name, ",")
		var matchingStackNames []string
		for _, name := range acctStackNames {
			for _, targetName := range targetStackNames {
				if strings.TrimSpace(name) == strings.TrimSpace(targetName) {
					matchingStackNames = append(matchingStackNames, name)
					break
				}
			}
		}
		if len(matchingStackNames) > 0 {
			matchingStacks = append(matchingStacks, Stack{
				Path: stack.Path,
				Type: stack.Type,
				Name: strings.Join(matchingStackNames, ","),
			})
		}
	}
	return matchingStacks
}

func IsUsingOrgV1(filepath string) bool {
	if filepath == "" {
		return true
	}

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return true
	}

	if strings.Contains(string(data), "AccountGroups") {
		return false
	}
	return true
}

// We parse it and assume that the file is in the current directory
func ParseOrganizationV1(filepath string) (Organization, error) {
	if filepath == "" {
		return Organization{}, errors.New("filepath is empty")
	}

	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return Organization{}, fmt.Errorf("err: %s reading file %s", err.Error(), filepath)
	}

	var org orgDatav1

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
		// Deprecated logic.
		for idx, parsedAcct := range org.Organization.ChildAccounts {
			if parsedAcct.AccountName == *acct.Name {
				org.Organization.ChildAccounts[idx].AccountID = *acct.Id
			}
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
		return ParseOrganizationV1(filepath)
	}
	if os.IsNotExist(err) {
		return Organization{}, nil
	}

	return ParseOrganizationV1(filepath)
}

func validOrganization(data Organization) error {
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

func WriteOrgV1File(filepath string, org *Organization) error {
	orgData := orgDatav1{
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

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
