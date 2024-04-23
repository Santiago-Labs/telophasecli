package resource

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/account"
	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/lib/awssess"
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
	ManagementAccount      bool              `yaml:"-"`
	DelegatedAdministrator bool              `yaml:"DelegatedAdministrator,omitempty"`
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

func (a Account) AllBaselineStacks() ([]Stack, error) {
	var stacks []Stack
	if a.Parent != nil {
		stacks = append(stacks, a.Parent.AllBaselineStacks()...)
	}

	stacks = append(stacks, a.BaselineStacks...)

	// We need to rerun through the stacks after we have collected them for an
	// account because we check what regions are enabled for the specific
	// account.
	var returnStacks []Stack
	for i := range stacks {
		currStack := stacks[i]

		if currStack.Region == "all" {
			generatedStacks, err := a.GenerateStacks(currStack)
			if err != nil {
				return nil, err
			}
			returnStacks = append(returnStacks, generatedStacks...)
			continue
		}

		// Regions can be comma separated to target just a few
		splitRegionStack := strings.Split(currStack.Region, ",")
		if len(splitRegionStack) > 1 {
			for _, region := range splitRegionStack {
				returnStacks = append(returnStacks, currStack.NewForRegion(region))
			}
			continue
		}

		returnStacks = append(returnStacks, currStack)
	}

	return returnStacks, nil
}

func (a Account) GenerateStacks(stack Stack) ([]Stack, error) {
	// We only generate multiple stacks if the region is "all"
	if stack.Region != "all" {
		return []Stack{stack}, nil
	}

	sess, err := awssess.DefaultSession()
	if err != nil {
		return nil, oops.Wrapf(err, "error starting sess")
	}
	acctClient := account.New(sess)
	output, err := acctClient.ListRegions(&account.ListRegionsInput{
		AccountId:  &a.AccountID,
		MaxResults: aws.Int64(50),
	})
	if err != nil {
		return nil, oops.Wrapf(err, "listing regions for account: (%s)", a.AccountID)
	}

	var stacks []Stack
	for _, region := range output.Regions {
		if *region.RegionOptStatus == account.RegionOptStatusEnabled ||
			*region.RegionOptStatus == account.RegionOptStatusEnabling ||
			*region.RegionOptStatus == account.RegionOptStatusEnabledByDefault {

			stacks = append(stacks,
				stack.NewForRegion(*region.RegionName),
			)
		}
	}

	return stacks, nil
}

func (a Account) FilterBaselineStacks(stackNames string) ([]Stack, error) {
	var matchingStacks []Stack
	targetStackNames := strings.Split(stackNames, ",")
	baselineStacks, err := a.AllBaselineStacks()
	if err != nil {
		return nil, err
	}

	for i, stack := range baselineStacks {
		acctStackNames := strings.Split(stack.Name, ",")
		for _, name := range acctStackNames {
			for _, targetName := range targetStackNames {
				if strings.TrimSpace(name) == strings.TrimSpace(targetName) {
					matchingStacks = append(matchingStacks, baselineStacks[i])
					break
				}
			}
		}
	}
	return matchingStacks, nil
}

func (a Account) FilterServiceControlPolicies(stackNames string) []Stack {
	var matchingStacks []Stack
	targetStackNames := strings.Split(stackNames, ",")
	for i, stack := range a.ServiceControlPolicies {
		acctStackNames := strings.Split(stack.Name, ",")
		for _, name := range acctStackNames {
			for _, targetName := range targetStackNames {
				if strings.TrimSpace(name) == strings.TrimSpace(targetName) {
					matchingStacks = append(matchingStacks, a.ServiceControlPolicies[i])
					break
				}
			}
		}
	}

	return matchingStacks
}
