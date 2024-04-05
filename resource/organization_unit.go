package resource

import "sort"

type OrganizationUnit struct {
	OUID                   *string             `yaml:"-"`
	OUName                 string              `yaml:"Name,omitempty"`
	ChildGroups            []*OrganizationUnit `yaml:"AccountGroups,omitempty"` // Deprecated. Use `OrganizationUnits`
	ChildOUs               []*OrganizationUnit `yaml:"OrganizationUnits,omitempty"`
	Tags                   []string            `yaml:"Tags,omitempty"`
	Accounts               []*Account          `yaml:"Accounts,omitempty"`
	BaselineStacks         []Stack             `yaml:"Stacks,omitempty"`
	ServiceControlPolicies []Stack             `yaml:"ServiceControlPolicies,omitempty"`
	Parent                 *OrganizationUnit   `yaml:"-"`
}

func (grp OrganizationUnit) ID() string {
	if grp.OUID != nil {
		return *grp.OUID
	}
	return ""
}

func (grp OrganizationUnit) Name() string {
	return grp.OUName
}

func (grp OrganizationUnit) Type() string {
	return "Organization Unit"
}

func (grp OrganizationUnit) AllTags() []string {
	var tags []string
	tags = append(tags, grp.Tags...)
	if grp.Parent != nil {
		tags = append(tags, grp.Parent.AllTags()...)
	}
	return tags
}

func (grp OrganizationUnit) AllBaselineStacks() []Stack {
	var stacks []Stack
	if grp.Parent != nil {
		stacks = append(stacks, grp.Parent.AllBaselineStacks()...)
	}
	stacks = append(stacks, grp.BaselineStacks...)
	return stacks
}

func (grp OrganizationUnit) AllDescendentAccounts() []*Account {
	var accounts []*Account
	accounts = append(accounts, grp.Accounts...)

	for _, ou := range grp.ChildOUs {
		accounts = append(accounts, ou.AllDescendentAccounts()...)
	}

	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].Email < accounts[j].Email
	})

	return accounts
}

func (grp OrganizationUnit) AllDescendentOUs() []*OrganizationUnit {
	var OUs []*OrganizationUnit
	OUs = append(OUs, grp.ChildOUs...)

	for _, childOU := range grp.ChildOUs {
		OUs = append(OUs, childOU.AllDescendentOUs()...)
	}

	sort.Slice(OUs, func(i, j int) bool {
		return OUs[i].OUName < OUs[j].OUName
	})

	return OUs

}
