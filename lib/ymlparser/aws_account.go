package ymlparser

import (
	"fmt"
	"strings"
)

type Account struct {
	Email       string `yaml:"Email"`
	AccountName string `yaml:"AccountName"`
	State       string `yaml:"State,omitempty"`
	// AccountID will be populated if this is an AWS Account.
	AccountID string `yaml:"-"`
	// SubscriptionID will be populated if this is an Azure Account.
	SubscriptionID         string        `yaml:"-"`
	AssumeRoleName         string        `yaml:"AssumeRoleName,omitempty"`
	Tags                   []string      `yaml:"Tags,omitempty"`
	BaselineStacks         []Stack       `yaml:"Stacks,omitempty"`
	ServiceControlPolicies []Stack       `yaml:"ServiceControlPolicies,omitempty"`
	ManagementAccount      bool          `yaml:"ManagementAccount,omitempty"`
	Parent                 *AccountGroup `yaml:"-"`
}

type Stack struct {
	Name            string `yaml:"Name"`
	Type            string `yaml:"Type"`
	Path            string `yaml:"Path"`
	RoleOverrideARN string `yaml:"RoleOverrideARN,omitempty"`
}

func (a Account) AssumeRoleARN() string {
	assumeRoleName := "OrganizationAccountAccessRole"
	if a.AssumeRoleName != "" {
		assumeRoleName = a.AssumeRoleName
	}

	return fmt.Sprintf("arn:aws:iam::%s:role/%s", a.AccountID, assumeRoleName)
}

func (a Account) ID() string {
	if a.IsAWS() {
		return a.AccountID
	}
	if a.IsAzure() {
		return a.SubscriptionID
	}

	return ""
}

func (a Account) IsAWS() bool {
	return a.AccountID != ""
}

func (a Account) IsAzure() bool {
	return a.SubscriptionID != ""
}

func (a Account) IsProvisioned() bool {
	if a.IsAWS() {
		return true
	}

	if a.IsAzure() {
		return true
	}
	return false
}

func (a Account) AllTags() []string {
	var tags []string
	tags = append(tags, a.Tags...)
	if a.Parent != nil {
		tags = append(tags, a.Parent.AllTags()...)
	}
	return tags
}

func (a Account) AllBaselineStacks() []Stack {
	var stacks []Stack
	if a.Parent != nil {
		stacks = append(stacks, a.Parent.AllBaselineStacks()...)
	}
	stacks = append(stacks, a.BaselineStacks...)
	return stacks
}

func (a Account) FilterBaselineStacks(stackNames string) []Stack {
	var matchingStacks []Stack
	targetStackNames := strings.Split(stackNames, ",")
	for _, stack := range a.AllBaselineStacks() {
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

func (a Account) FilterServiceControlPolicies(stackNames string) []Stack {
	var matchingStacks []Stack
	targetStackNames := strings.Split(stackNames, ",")
	for _, stack := range a.ServiceControlPolicies {
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
