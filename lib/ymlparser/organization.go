package ymlparser

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/resource"
	"gopkg.in/yaml.v3"
)

type orgDatav2 struct {
	Organization resource.OrganizationUnit `yaml:"Organization"`
}

type Parser struct {
	orgClient awsorgs.Client
}

func NewParser(orgClient awsorgs.Client) Parser {
	return Parser{
		orgClient: orgClient,
	}
}

func (o Parser) ParseOrganization(ctx context.Context, filepath string) (*resource.OrganizationUnit, error) {
	if filepath == "" {
		return nil, errors.New("filepath is empty")
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("err: %s reading file %s", err.Error(), filepath)
	}

	var org orgDatav2

	if err := yaml.Unmarshal(data, &org); err != nil {
		return nil, err
	}

	// We hydrate the OU filepaths before validating the organization because we
	// need every org from all the file branches to be populated so we can get
	// their corresponding accountIDs.
	if err := o.hydrateOUFilepaths(ctx, &org.Organization); err != nil {
		return nil, err
	}

	if err := validOrganization(org.Organization); err != nil {
		return nil, err
	}

	if err = o.HydrateParsedOrg(ctx, &org.Organization); err != nil {
		return nil, err
	}

	return &org.Organization, nil
}

func (p Parser) hydrateOUFilepaths(ctx context.Context, ou *resource.OrganizationUnit) error {
	if ou.OUFilepath != nil {
		data, err := os.ReadFile(*ou.OUFilepath)
		if err != nil {
			return fmt.Errorf("err: %s reading file for OUFilepath %s", err.Error(), *ou.OUFilepath)
		}

		var childOU resource.OrganizationUnit
		if err := yaml.Unmarshal(data, &childOU); err != nil {
			return oops.Wrapf(err, "UnmarshalChildOU")
		}
		// Now we replace where the ou is pointing to with this parsed OU.
		*ou = childOU
	}

	for _, childOU := range ou.ChildOUs {
		err := p.hydrateOUFilepaths(ctx, childOU)
		if err != nil {
			return oops.Wrapf(err, "HydrateFilepaths")
		}
	}

	return nil
}

func (p Parser) HydrateParsedOrg(ctx context.Context, parsedOrg *resource.OrganizationUnit) error {
	rootId, err := p.orgClient.GetRootId()
	if err != nil {
		return err
	}
	rootName := "root"
	rootOU := &organizations.OrganizationalUnit{
		Id:   &rootId,
		Name: &rootName,
	}
	parsedOrg.OUName = "root"
	p.hydrateOUID(parsedOrg, rootOU)
	hydrateOUParent(parsedOrg)
	hydrateAccountParent(parsedOrg)

	mgmtAcct, err := p.orgClient.FetchManagementAccount(ctx)
	if err != nil {
		return oops.Wrapf(err, "error fetching management account")
	}

	// Hydrate Group, then fetch all accounts (pointers) and populate ID.
	providerAccts, err := p.orgClient.CurrentAccounts(ctx)
	if err != nil {
		return oops.Wrapf(err, "CurrentAccounts")
	}
	for _, providerAcct := range providerAccts {
		for _, parsedAcct := range parsedOrg.AllDescendentAccounts() {
			if parsedAcct.Email == *providerAcct.Email {
				parsedAcct.AccountID = *providerAcct.Id
				parsedAcct.Status = aws.StringValue(providerAcct.Status)
			}
			if parsedAcct.Email == mgmtAcct.Email {
				parsedAcct.ManagementAccount = true
			}
		}
	}

	// We have to hydrate tags after account ID to get the right tags.
	p.hydrateTags(ctx, parsedOrg)

	return nil
}

func hydrateAccountParent(ou *resource.OrganizationUnit) {
	for idx := range ou.Accounts {
		ou.Accounts[idx].Parent = ou
	}

	for _, childOU := range ou.ChildOUs {
		hydrateAccountParent(childOU)
	}
}

func (p Parser) hydrateOUID(parsedOU *resource.OrganizationUnit, providerOU *organizations.OrganizationalUnit) error {
	if providerOU != nil {
		parsedOU.OUID = providerOU.Id
		providerChildren, err := p.orgClient.GetOrganizationUnitChildren(context.TODO(), *parsedOU.OUID)
		if err != nil {
			return oops.Wrapf(err, "GetOrganizationUnitChildren for OUID: %s", *parsedOU.OUID)
		}

		for _, parsedChild := range parsedOU.ChildOUs {
			var found bool
			for _, providerChild := range providerChildren {
				if parsedChild.OUName == *providerChild.Name {
					found = true
					err = p.hydrateOUID(parsedChild, providerChild)
					if err != nil {
						return err
					}
				}
			}

			if !found {
				err := p.hydrateOUID(parsedChild, nil)
				if err != nil {
					return err
				}
			}
		}
	} else {
		for _, parsedChild := range parsedOU.ChildOUs {
			err := p.hydrateOUID(parsedChild, nil)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p Parser) hydrateTags(ctx context.Context, ou *resource.OrganizationUnit) error {
	tags, err := p.orgClient.GetTags(ctx, ou.ID())
	if err != nil {
		return oops.Wrapf(err, "GetTags OUID: %s", ou.ID())
	}
	ou.AWSTags = tags

	for _, parsedChild := range ou.AllDescendentOUs() {
		p.hydrateTags(ctx, parsedChild)
	}

	for _, acct := range ou.AllDescendentAccounts() {
		tags, err := p.orgClient.GetTags(ctx, acct.ID())
		if err != nil {
			return oops.Wrapf(err, "GetTags Account ID: %s", acct.ID())
		}
		acct.AWSTags = tags
	}

	return nil
}

func hydrateOUParent(parsedOU *resource.OrganizationUnit) {
	for _, parsedChild := range parsedOU.ChildOUs {
		parsedChild.Parent = parsedOU
		hydrateOUParent(parsedChild)
	}
}

func WriteOrgFile(filepath string, org *resource.OrganizationUnit) error {
	orgData := orgDatav2{
		Organization: *org,
	}
	result, err := yaml.Marshal(orgData)
	if err != nil {
		return err
	}

	if fileExists(filepath) {
		return fmt.Errorf("file %s already exists we will not overwrite it", filepath)
	}

	if err := os.WriteFile(filepath, result, 0644); err != nil {
		return err
	}

	return nil
}

func validOrganization(data resource.OrganizationUnit) error {
	accountEmails := map[string]struct{}{}

	for _, account := range data.AllDescendentAccounts() {
		if _, ok := accountEmails[account.Email]; ok {
			return fmt.Errorf("duplicate account email %s", account.Email)
		} else {
			accountEmails[account.Email] = struct{}{}
		}

	}

	for _, ou := range data.AllDescendentOUs() {
		if len(ou.ChildGroups) > 0 {
			if len(ou.ChildOUs) > 0 {
				return fmt.Errorf("cannot set both AccountGroups and OrganizationUnits fields on Organization Unit: %s", ou.OUName)
			}

			// Remove this after deleting ChildGroups field.
			ou.ChildOUs = append(ou.ChildOUs, ou.ChildGroups...)
		}
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
