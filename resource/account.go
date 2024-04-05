package resource

import (
	"fmt"
	"strings"
)

type Account struct {
	Email       string `yaml:"Email"`
	AccountName string `yaml:"AccountName"`
	State       string `yaml:"State,omitempty"`
	AccountID   string `yaml:"-"`

	AssumeRoleName         string            `yaml:"AssumeRoleName,omitempty"`
	Tags                   []string          `yaml:"Tags,omitempty"`
	BaselineStacks         []Stack           `yaml:"Stacks,omitempty"`
	ServiceControlPolicies []Stack           `yaml:"ServiceControlPolicies,omitempty"`
	ManagementAccount      bool              `yaml:"ManagementAccount,omitempty"`
	Parent                 *OrganizationUnit `yaml:"-"`
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

	return ""
}

func (a Account) Name() string {
	return a.AccountName
}

func (a Account) Type() string {
	return "Account"
}

func (a Account) IsAWS() bool {
	return a.AccountID != ""
}

func (a Account) IsProvisioned() bool {
	return a.IsAWS()
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
