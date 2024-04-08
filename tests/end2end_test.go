package tests

import (
	"context"
	"fmt"
	"io/ioutil"
	"runtime/debug"
	"testing"

	"github.com/santiago-labs/telophasecli/cmd"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
	"github.com/santiago-labs/telophasecli/resource"
	"github.com/santiago-labs/telophasecli/resourceoperation"
	"github.com/stretchr/testify/assert"
)

type E2ETestCase struct {
	Name            string
	OrgInitialState *resource.OrganizationUnit
	OrgYaml         string
	Expected        *resource.OrganizationUnit
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
	{
		Name: "Test that we can move Accounts between OUs",
		OrgInitialState: &resource.OrganizationUnit{
			OUName: "root",
			ChildOUs: []*resource.OrganizationUnit{
				{
					OUName: "US Engineers",
					Accounts: []*resource.Account{
						{
							AccountName: "Engineer A",
							Email:       "engineerA@example.com",
						},
						{
							AccountName: "Engineer B",
							Email:       "engineerB@example.com",
						},
					},
				},
				{
					OUName: "EU Engineers",
					Accounts: []*resource.Account{
						{
							AccountName: "Engineer C",
							Email:       "engineerC@example.com",
						},
						{
							AccountName: "Engineer D",
							Email:       "engineerD@example.com",
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
		OrgYaml: `
Organization:
    Name: root
    OrganizationUnits:
      - Name: US Engineers
        Accounts:
          - AccountName: Engineer A
            Email: engineerA@example.com
      - Name: EU Engineers 
        Accounts:
          - AccountName: Engineer C
            Email: engineerC@example.com
          - AccountName: Engineer D
            Email: engineerD@example.com
          - AccountName: Engineer B
            Email: engineerB@example.com
    Accounts:
      - AccountName: master
        Email: master@example.com
`,
		Expected: &resource.OrganizationUnit{
			OUName: "root",
			ChildOUs: []*resource.OrganizationUnit{
				{
					OUName: "US Engineers",
					Accounts: []*resource.Account{
						{
							AccountName: "Engineer A",
							Email:       "engineerA@example.com",
						},
					},
				},
				{
					OUName: "EU Engineers",
					Accounts: []*resource.Account{
						{
							AccountName: "Engineer B",
							Email:       "engineerB@example.com",
						},
						{
							AccountName: "Engineer C",
							Email:       "engineerC@example.com",
						},
						{
							AccountName: "Engineer D",
							Email:       "engineerD@example.com",
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
}

func TestEndToEnd(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			t.Errorf("Recovered from panic: %v\n%s", r, stack)
		}
	}()

	for _, test := range tests {
		fmt.Printf("Running test: %s\n", test.Name)
		setupTest()

		ctx := context.Background()
		orgClient := awsorgs.New()

		rootId, err := orgClient.GetRootId()
		assert.NoError(t, err, "Failed to fetch rootId")

		consoleUI := runner.NewSTDOut()
		mgmtAcct, err := orgClient.FetchManagementAccount(ctx)
		assert.NoError(t, err, "Error fetching management account")

		if test.OrgInitialState != nil {
			rootId, err := orgClient.GetRootId()
			assert.NoError(t, err, "Error fetching root OU ID")
			test.OrgInitialState.OUID = &rootId

			ymlparser.HydrateParsedOrg(test.OrgInitialState)
			orgOps := resourceoperation.CollectOrganizationUnitOps(
				ctx, consoleUI, orgClient, mgmtAcct, test.OrgInitialState, resourceoperation.Deploy,
			)
			for _, op := range orgOps {
				err := op.Call(ctx)
				if err != nil {
					assert.NoError(t, err, "Error creating organization initial state")
				}
			}

			fetchedOrg, err := orgClient.FetchOUAndDescendents(ctx, rootId, "000000000000")
			assert.NoError(t, err, "Failed to fetch rootOU")

			compareOrganizationUnits(t, test.OrgInitialState, &fetchedOrg)
		}

		err = ioutil.WriteFile("organization.yml", []byte(test.OrgYaml), 0644)
		assert.NoError(t, err, "Failed to write organization.yml")

		parsedOrg, err := ymlparser.ParseOrganizationV2("organization.yml")
		assert.NoError(t, err, "Failed to parse organization.yml")

		compareOrganizationUnits(t, test.Expected, parsedOrg)

		cmd.ProcessOrgEndToEnd(consoleUI, resourceoperation.Deploy)

		fetchedOrg, err := orgClient.FetchOUAndDescendents(ctx, rootId, "000000000000")
		assert.NoError(t, err, "Failed to fetch rootOU")

		compareOrganizationUnits(t, test.Expected, &fetchedOrg)
	}
}
