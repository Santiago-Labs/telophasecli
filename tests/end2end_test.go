package tests

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"testing"

	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
	"github.com/santiago-labs/telophasecli/resource"
	"github.com/stretchr/testify/assert"
)

type E2ETestCase struct {
	Name     string
	OrgYaml  string
	Expected *resource.OrganizationUnit
}

var tests = []E2ETestCase{
	{
		Name: "Test that we can create OUs",
		OrgYaml: `
Organization:
    Name: root
    OrganizationUnits:
      - Name: ProductionTenants
      - Name: Development 
    Accounts:
      - AccountName: master
        Email: master@example.com
`,
		Expected: &resource.OrganizationUnit{
			OUName: "root",
			ChildOUs: []*resource.OrganizationUnit{
				{
					OUName: "ProductionTenants",
				},
				{
					OUName: "Development",
				},
			},
			Accounts: []*resource.Account{
				{
					AccountName: "master",
					Email:       "master@example.com",
				},
			},
		},
	},
	{
		Name: "Test that we can create nested OUs",
		OrgYaml: `
Organization:
    Name: root
    OrganizationUnits:
      - Name: ProductionTenants
        OrganizationUnits:
          - Name: ProductionEU
          - Name: ProductionUS
      - Name: Development 
        OrganizationUnits:
          - Name: DevEU
          - Name: DevUS
    Accounts:
      - AccountName: master
        Email: master@example.com
`,
		Expected: &resource.OrganizationUnit{
			OUName: "root",
			ChildOUs: []*resource.OrganizationUnit{
				{
					OUName: "ProductionTenants",
					ChildOUs: []*resource.OrganizationUnit{
						{
							OUName: "ProductionEU",
						},
						{
							OUName: "ProductionUS",
						},
					},
				},
				{
					OUName: "Development",
					ChildOUs: []*resource.OrganizationUnit{
						{
							OUName: "DevEU",
						},
						{
							OUName: "DevUS",
						},
					},
				},
			},
			Accounts: []*resource.Account{
				{
					AccountName: "master",
					Email:       "master@example.com",
				},
			},
		},
	},
	{
		Name: "Test that we can create accounts",
		OrgYaml: `
Organization:
  Name: root
  Accounts:
    - AccountName: master
      Email: master@example.com
    - AccountName: test1
      Email: test1@example.com
    - AccountName: test2
      Email: test2@example.com
`,
		Expected: &resource.OrganizationUnit{
			OUName: "root",
			Accounts: []*resource.Account{
				{
					AccountName: "master",
					Email:       "master@example.com",
				},
				{
					AccountName: "test1",
					Email:       "test1@example.com",
				},
				{
					AccountName: "test2",
					Email:       "test2@example.com",
				},
			},
		},
	},
	{
		Name: "Test that we can create accounts in OUs",
		OrgYaml: `
Organization:
  Name: root
  OrganizationUnits:
    - Name: ProductionTenants
      Accounts:
        - AccountName: test1
          Email: test1@example.com
      OrganizationUnits:
        - Name: ProductionEU
        - Name: ProductionUS
          Accounts:
            - AccountName: test2
              Email: test2@example.com
    - Name: Development 
      OrganizationUnits:
        - Name: DevEU
          Accounts:
            - AccountName: test3
              Email: test3@example.com
            - AccountName: test4
              Email: test4@example.com
            - AccountName: test5
              Email: test5@example.com
        - Name: DevUS
          Accounts:
            - AccountName: test6
              Email: test6@example.com
            - AccountName: test7
              Email: test7@example.com
  Accounts:
    - AccountName: master
      Email: master@example.com
    - AccountName: test8
      Email: test8@example.com
    - AccountName: test9
      Email: test9@example.com
`,
		Expected: &resource.OrganizationUnit{
			OUName: "root",
			ChildOUs: []*resource.OrganizationUnit{
				{
					OUName: "ProductionTenants",
					Accounts: []*resource.Account{
						{
							AccountName: "test1",
							Email:       "test1@example.com",
						},
					},
					ChildOUs: []*resource.OrganizationUnit{
						{
							OUName: "ProductionEU",
						},
						{
							OUName: "ProductionUS",
							Accounts: []*resource.Account{
								{
									AccountName: "test2",
									Email:       "test2@example.com",
								},
							},
						},
					},
				},
				{
					OUName: "Development",
					ChildOUs: []*resource.OrganizationUnit{
						{
							OUName: "DevEU",
							Accounts: []*resource.Account{
								{
									AccountName: "test3",
									Email:       "test3@example.com",
								},
								{
									AccountName: "test4",
									Email:       "test4@example.com",
								},
								{
									AccountName: "test5",
									Email:       "test5@example.com",
								},
							},
						},
						{
							OUName: "DevUS",
							Accounts: []*resource.Account{
								{
									AccountName: "test6",
									Email:       "test6@example.com",
								},
								{
									AccountName: "test7",
									Email:       "test7@example.com",
								},
							},
						},
					},
				},
			},
			Accounts: []*resource.Account{
				{
					AccountName: "master",
					Email:       "master@example.com",
				},
				{
					AccountName: "test8",
					Email:       "test8@example.com",
				},
				{
					AccountName: "test9",
					Email:       "test9@example.com",
				},
			},
		},
	},
}

func TestEndToEnd(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Recovered from panic: %v", r)
		}
	}()

	for _, test := range tests {
		setupTest()

		fmt.Printf("Running test: %s\n", test.Name)
		err := ioutil.WriteFile("organization.yml", []byte(test.OrgYaml), 0644)
		assert.NoError(t, err, "Failed to write organization.yml")

		parsedOrg, err := ymlparser.ParseOrganizationV2("organization.yml")
		assert.NoError(t, err, "Failed to parse organization.yml")

		compareOrganizationUnits(t, test.Expected, parsedOrg)

		cmd := exec.Command("bash", "-c", "../telophasecli deploy")
		_, stderr, err := runCmd(cmd)
		assert.NoError(t, err, fmt.Sprintf("Failed to run telophasecli deploy. STDERR \n %s \n", stderr))

		ctx := context.Background()
		orgClient := awsorgs.New()
		rootId, err := orgClient.GetRootId()
		assert.NoError(t, err, "Failed to fetch rootId")

		fetchedOrg, err := orgClient.FetchOUAndDescendents(ctx, rootId, "000000000000")
		assert.NoError(t, err, "Failed to fetch rootOU")

		compareOrganizationUnits(t, test.Expected, &fetchedOrg)
	}
}
