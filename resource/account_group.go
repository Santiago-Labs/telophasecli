package resource

import "sort"

type AccountGroup struct {
	ID                     *string         `yaml:"-"`
	Name                   string          `yaml:"Name,omitempty"`
	ChildGroups            []*AccountGroup `yaml:"AccountGroups,omitempty"`
	Tags                   []string        `yaml:"Tags,omitempty"`
	Accounts               []*Account      `yaml:"Accounts,omitempty"`
	BaselineStacks         []Stack         `yaml:"Stacks,omitempty"`
	ServiceControlPolicies []Stack         `yaml:"ServiceControlPolicies,omitempty"`
	Parent                 *AccountGroup   `yaml:"-"`
}

func (grp AccountGroup) AllTags() []string {
	var tags []string
	tags = append(tags, grp.Tags...)
	if grp.Parent != nil {
		tags = append(tags, grp.Parent.AllTags()...)
	}
	return tags
}

func (grp AccountGroup) AllBaselineStacks() []Stack {
	var stacks []Stack
	if grp.Parent != nil {
		stacks = append(stacks, grp.Parent.AllBaselineStacks()...)
	}
	stacks = append(stacks, grp.BaselineStacks...)
	return stacks
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
