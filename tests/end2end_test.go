package tests

import (
	"context"
	"fmt"
	"io/ioutil"
	"runtime/debug"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/santiago-labs/telophasecli/cmd"
	"github.com/santiago-labs/telophasecli/cmd/runner"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
	"github.com/santiago-labs/telophasecli/resource"
	"github.com/santiago-labs/telophasecli/resourceoperation"
	"github.com/stretchr/testify/assert"
)

type E2ETestCase struct {
	Name              string
	OrgInitialState   *resource.OrganizationUnit
	OrgYaml           string
	Expected          *resource.OrganizationUnit
	Targets           string
	ExpectedResources func(t *testing.T)
}

var tests = []E2ETestCase{
	// 	{
	// 		Name: "Test that we can create OUs",
	// 		OrgYaml: `
	// Organization:
	//     Name: root
	//     OrganizationUnits:
	//       - Name: ProductionTenants
	//       - Name: Development
	//     Accounts:
	//       - AccountName: master
	//         Email: master@example.com
	// `,
	// 		Expected: &resource.OrganizationUnit{
	// 			OUName: "root",
	// 			ChildOUs: []*resource.OrganizationUnit{
	// 				{
	// 					OUName: "ProductionTenants",
	// 				},
	// 				{
	// 					OUName: "Development",
	// 				},
	// 			},
	// 			Accounts: []*resource.Account{
	// 				{
	// 					AccountName:       "master",
	// 					Email:             "master@example.com",
	// 					ManagementAccount: true,
	// 				},
	// 			},
	// 		},
	// 		ExpectedResources: func(*testing.T) {},
	// 	},
	// 	{
	// 		Name: "Test that we can create nested OUs",
	// 		OrgYaml: `
	// Organization:
	//     Name: root
	//     OrganizationUnits:
	//       - Name: ProductionTenants
	//         OrganizationUnits:
	//           - Name: ProductionEU
	//           - Name: ProductionUS
	//       - Name: Development
	//         OrganizationUnits:
	//           - Name: DevEU
	//           - Name: DevUS
	//     Accounts:
	//       - AccountName: master
	//         Email: master@example.com
	// `,
	// 		Expected: &resource.OrganizationUnit{
	// 			OUName: "root",
	// 			ChildOUs: []*resource.OrganizationUnit{
	// 				{
	// 					OUName: "ProductionTenants",
	// 					ChildOUs: []*resource.OrganizationUnit{
	// 						{
	// 							OUName: "ProductionEU",
	// 						},
	// 						{
	// 							OUName: "ProductionUS",
	// 						},
	// 					},
	// 				},
	// 				{
	// 					OUName: "Development",
	// 					ChildOUs: []*resource.OrganizationUnit{
	// 						{
	// 							OUName: "DevEU",
	// 						},
	// 						{
	// 							OUName: "DevUS",
	// 						},
	// 					},
	// 				},
	// 			},
	// 			Accounts: []*resource.Account{
	// 				{
	// 					AccountName:       "master",
	// 					Email:             "master@example.com",
	// 					ManagementAccount: true,
	// 				},
	// 			},
	// 		},
	// 		ExpectedResources: func(*testing.T) {},
	// 	},
	// 	{
	// 		Name: "Test that we can create accounts",
	// 		OrgYaml: `
	// Organization:
	//   Name: root
	//   Accounts:
	//     - AccountName: master
	//       Email: master@example.com
	//     - AccountName: test1
	//       Email: test1@example.com
	//     - AccountName: test2
	//       Email: test2@example.com
	// `,
	// 		Expected: &resource.OrganizationUnit{
	// 			OUName: "root",
	// 			Accounts: []*resource.Account{
	// 				{
	// 					AccountName:       "master",
	// 					Email:             "master@example.com",
	// 					ManagementAccount: true,
	// 				},
	// 				{
	// 					AccountName: "test1",
	// 					Email:       "test1@example.com",
	// 				},
	// 				{
	// 					AccountName: "test2",
	// 					Email:       "test2@example.com",
	// 				},
	// 			},
	// 		},
	// 		ExpectedResources: func(*testing.T) {},
	// 	},
	// 	{
	// 		Name: "Test that we can create accounts in OUs",
	// 		OrgYaml: `
	// Organization:
	//   Name: root
	//   OrganizationUnits:
	//     - Name: ProductionTenants
	//       Accounts:
	//         - AccountName: test1
	//           Email: test1@example.com
	//       OrganizationUnits:
	//         - Name: ProductionEU
	//         - Name: ProductionUS
	//           Accounts:
	//             - AccountName: test2
	//               Email: test2@example.com
	//     - Name: Development
	//       OrganizationUnits:
	//         - Name: DevEU
	//           Accounts:
	//             - AccountName: test3
	//               Email: test3@example.com
	//             - AccountName: test4
	//               Email: test4@example.com
	//             - AccountName: test5
	//               Email: test5@example.com
	//         - Name: DevUS
	//           Accounts:
	//             - AccountName: test6
	//               Email: test6@example.com
	//             - AccountName: test7
	//               Email: test7@example.com
	//   Accounts:
	//     - AccountName: master
	//       Email: master@example.com
	//     - AccountName: test8
	//       Email: test8@example.com
	//     - AccountName: test9
	//       Email: test9@example.com
	// `,
	// 		Expected: &resource.OrganizationUnit{
	// 			OUName: "root",
	// 			ChildOUs: []*resource.OrganizationUnit{
	// 				{
	// 					OUName: "ProductionTenants",
	// 					Accounts: []*resource.Account{
	// 						{
	// 							AccountName: "test1",
	// 							Email:       "test1@example.com",
	// 						},
	// 					},
	// 					ChildOUs: []*resource.OrganizationUnit{
	// 						{
	// 							OUName: "ProductionEU",
	// 						},
	// 						{
	// 							OUName: "ProductionUS",
	// 							Accounts: []*resource.Account{
	// 								{
	// 									AccountName: "test2",
	// 									Email:       "test2@example.com",
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 				{
	// 					OUName: "Development",
	// 					ChildOUs: []*resource.OrganizationUnit{
	// 						{
	// 							OUName: "DevEU",
	// 							Accounts: []*resource.Account{
	// 								{
	// 									AccountName: "test3",
	// 									Email:       "test3@example.com",
	// 								},
	// 								{
	// 									AccountName: "test4",
	// 									Email:       "test4@example.com",
	// 								},
	// 								{
	// 									AccountName: "test5",
	// 									Email:       "test5@example.com",
	// 								},
	// 							},
	// 						},
	// 						{
	// 							OUName: "DevUS",
	// 							Accounts: []*resource.Account{
	// 								{
	// 									AccountName: "test6",
	// 									Email:       "test6@example.com",
	// 								},
	// 								{
	// 									AccountName: "test7",
	// 									Email:       "test7@example.com",
	// 								},
	// 							},
	// 						},
	// 					},
	// 				},
	// 			},
	// 			Accounts: []*resource.Account{
	// 				{
	// 					AccountName:       "master",
	// 					Email:             "master@example.com",
	// 					ManagementAccount: true,
	// 				},
	// 				{
	// 					AccountName: "test8",
	// 					Email:       "test8@example.com",
	// 				},
	// 				{
	// 					AccountName: "test9",
	// 					Email:       "test9@example.com",
	// 				},
	// 			},
	// 		},
	// 		ExpectedResources: func(*testing.T) {},
	// 	},
	// 	{
	// 		Name: "Test that we can move Accounts between OUs",
	// 		OrgInitialState: &resource.OrganizationUnit{
	// 			OUName: "root",
	// 			ChildOUs: []*resource.OrganizationUnit{
	// 				{
	// 					OUName: "US Engineers",
	// 					Accounts: []*resource.Account{
	// 						{
	// 							AccountName: "Engineer A",
	// 							Email:       "engineerA@example.com",
	// 						},
	// 						{
	// 							AccountName: "Engineer B",
	// 							Email:       "engineerB@example.com",
	// 						},
	// 					},
	// 				},
	// 				{
	// 					OUName: "EU Engineers",
	// 					Accounts: []*resource.Account{
	// 						{
	// 							AccountName: "Engineer C",
	// 							Email:       "engineerC@example.com",
	// 						},
	// 						{
	// 							AccountName: "Engineer D",
	// 							Email:       "engineerD@example.com",
	// 						},
	// 					},
	// 				},
	// 			},
	// 			Accounts: []*resource.Account{
	// 				{
	// 					AccountName: "master",
	// 					Email:       "master@example.com",
	// 				},
	// 			},
	// 		},
	// 		OrgYaml: `
	// Organization:
	//     Name: root
	//     OrganizationUnits:
	//       - Name: US Engineers
	//         Accounts:
	//           - AccountName: Engineer A
	//             Email: engineerA@example.com
	//       - Name: EU Engineers
	//         Accounts:
	//           - AccountName: Engineer C
	//             Email: engineerC@example.com
	//           - AccountName: Engineer D
	//             Email: engineerD@example.com
	//           - AccountName: Engineer B
	//             Email: engineerB@example.com
	//     Accounts:
	//       - AccountName: master
	//         Email: master@example.com
	// `,
	// 		Expected: &resource.OrganizationUnit{
	// 			OUName: "root",
	// 			ChildOUs: []*resource.OrganizationUnit{
	// 				{
	// 					OUName: "US Engineers",
	// 					Accounts: []*resource.Account{
	// 						{
	// 							AccountName: "Engineer A",
	// 							Email:       "engineerA@example.com",
	// 						},
	// 					},
	// 				},
	// 				{
	// 					OUName: "EU Engineers",
	// 					Accounts: []*resource.Account{
	// 						{
	// 							AccountName: "Engineer B",
	// 							Email:       "engineerB@example.com",
	// 						},
	// 						{
	// 							AccountName: "Engineer C",
	// 							Email:       "engineerC@example.com",
	// 						},
	// 						{
	// 							AccountName: "Engineer D",
	// 							Email:       "engineerD@example.com",
	// 						},
	// 					},
	// 				},
	// 			},
	// 			Accounts: []*resource.Account{
	// 				{
	// 					AccountName:       "master",
	// 					Email:             "master@example.com",
	// 					ManagementAccount: true,
	// 				},
	// 			},
	// 		},
	// 		ExpectedResources: func(*testing.T) {},
	// 	},
	{
		Name: "Test that we can apply SCPs",
		OrgYaml: `
Organization:
    Name: root
    OrganizationUnits:
      - Name: ProductionTenants
        ServiceControlPolicies:
          - Type: Terraform
            Path: tf/scp-test
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
					ServiceControlPolicies: []resource.Stack{
						{
							Type: "Terraform",
							Path: "tf/scp-test",
						},
					},
				},
				{
					OUName: "Development",
				},
			},
			Accounts: []*resource.Account{
				{
					AccountName:       "master",
					Email:             "master@example.com",
					ManagementAccount: true,
				},
			},
		},
		ExpectedResources: func(t *testing.T) {
			sess, err := session.NewSession(&aws.Config{
				Region:   aws.String("us-east-1"),
				Endpoint: aws.String("http://localhost:4566"),
			})
			if err != nil {
				t.Fatalf("Failed to create session: %v", err)
			}

			orgSvc := organizations.New(sess)
			listRootsOutput, err := orgSvc.ListRoots(&organizations.ListRootsInput{})
			if err != nil {
				t.Fatalf("Failed to list roots: %v", err)
			}

			var rootId string
			if len(listRootsOutput.Roots) > 0 {
				rootId = *listRootsOutput.Roots[0].Id
			} else {
				t.Fatalf("No roots found")
			}

			listOUsOutput, err := orgSvc.ListOrganizationalUnitsForParent(&organizations.ListOrganizationalUnitsForParentInput{
				ParentId: &rootId,
			})
			if err != nil {
				t.Fatalf("Failed to list OUs for root: %v", err)
			}

			var productionTenantId string
			for _, ou := range listOUsOutput.OrganizationalUnits {
				if *ou.Name == "ProductionTenants" {
					productionTenantId = *ou.Id
					break
				}
			}

			if productionTenantId == "" {
				t.Fatalf("OU 'ProductionTenants' not found")
			}

			listPoliciesOutput, err := orgSvc.ListPoliciesForTarget(&organizations.ListPoliciesForTargetInput{
				TargetId: &productionTenantId,
				Filter:   aws.String("SERVICE_CONTROL_POLICY"),
			})
			if err != nil {
				t.Fatalf("Failed to list policies for 'ProductionTenants': %v", err)
			}

			var scpFound bool
			for _, policy := range listPoliciesOutput.Policies {
				if *policy.Name == "restrict_regions" {
					scpFound = true
					break
				}
			}

			if !scpFound {
				t.Fatalf("SCP 'restrict_regions' is not attached to 'ProductionTenants'")
			}
		},
	},
	{
		Name: "Test that we can apply stacks at root",
		OrgYaml: `
Organization:
    Name: root
    Stacks:
      - Type: Terraform
        Path: tf/s3-test
    Accounts:
      - AccountName: master
        Email: master@example.com
`,
		Expected: &resource.OrganizationUnit{
			OUName: "root",
			BaselineStacks: []resource.Stack{
				{
					Type: "Terraform",
					Path: "tf/s3-test",
				},
			},
			Accounts: []*resource.Account{
				{
					AccountName:       "master",
					Email:             "master@example.com",
					ManagementAccount: true,
				},
			},
		},
		ExpectedResources: func(t *testing.T) {
			sess, err := session.NewSession(&aws.Config{
				Region:           aws.String("us-east-1"),
				Endpoint:         aws.String("http://localhost:4566"),
				S3ForcePathStyle: aws.Bool(true),
			})
			if err != nil {
				t.Fatalf("Failed to create session: %v", err)
			}

			svc := s3.New(sess)
			result, err := svc.ListBuckets(nil)
			if err != nil {
				t.Fatalf("Failed to list buckets: %v", err)
			}

			var bucketNames []string
			for _, b := range result.Buckets {
				bucketNames = append(bucketNames, *b.Name)
			}

			if len(bucketNames) != 1 || bucketNames[0] != "test" {
				t.Fatalf("Test failed, expected only 'test' bucket, found buckets: %v", bucketNames)
			}
		},
	},
	{
		Name: "Test that we can target new accounts only",
		OrgYaml: `
Organization:
    Name: root
    Stacks:
      - Type: Terraform
        Path: tf/s3-test
    OrganizationUnits:
      - Name: ProductionTenants
        ServiceControlPolicies:
          - Type: Terraform
            Path: tf/scp-test
    Accounts:
      - AccountName: master
        Email: master@example.com
      - AccountName: test
        Email: test@example.com
`,
		Expected: &resource.OrganizationUnit{
			OUName: "root",
			ChildOUs: []*resource.OrganizationUnit{
				{
					OUName: "ProductionTenants",
					ServiceControlPolicies: []resource.Stack{
						{
							Type: "Terraform",
							Path: "tf/scp-test",
						},
					},
				},
			},
			BaselineStacks: []resource.Stack{
				{
					Type: "Terraform",
					Path: "tf/s3-test",
				},
			},
			Accounts: []*resource.Account{
				{
					AccountName:       "master",
					Email:             "master@example.com",
					ManagementAccount: true,
				},
				{
					AccountName: "test",
					Email:       "test@example.com",
				},
			},
		},
		ExpectedResources: func(t *testing.T) {
			sess, err := session.NewSession(&aws.Config{
				Region:           aws.String("us-east-1"),
				Endpoint:         aws.String("http://localhost:4566"),
				S3ForcePathStyle: aws.Bool(true),
			})
			if err != nil {
				t.Fatalf("Failed to create session: %v", err)
			}

			// Check S3 bucket
			svc := s3.New(sess)
			result, err := svc.ListBuckets(nil)
			if err != nil {
				t.Fatalf("Failed to list buckets: %v", err)
			}

			if len(result.Buckets) > 0 {
				t.Fatalf("Buckets found in account: %v", result.Buckets)
			}

			// Check Organization Service
			orgSvc := organizations.New(sess)
			listRootsOutput, err := orgSvc.ListRoots(&organizations.ListRootsInput{})
			if err != nil {
				t.Fatalf("Failed to list roots: %v", err)
			}

			var rootId string
			if len(listRootsOutput.Roots) > 0 {
				rootId = *listRootsOutput.Roots[0].Id
			} else {
				t.Fatalf("No roots found")
			}

			listOUsOutput, err := orgSvc.ListOrganizationalUnitsForParent(&organizations.ListOrganizationalUnitsForParentInput{
				ParentId: &rootId,
			})
			if err != nil {
				t.Fatalf("Failed to list OUs for root: %v", err)
			}

			var productionTenantId string
			for _, ou := range listOUsOutput.OrganizationalUnits {
				if *ou.Name == "ProductionTenants" {
					productionTenantId = *ou.Id
					break
				}
			}

			if productionTenantId == "" {
				t.Fatalf("OU 'ProductionTenants' not found")
			}

			listPoliciesOutput, err := orgSvc.ListPoliciesForTarget(&organizations.ListPoliciesForTargetInput{
				TargetId: &productionTenantId,
				Filter:   aws.String("SERVICE_CONTROL_POLICY"),
			})
			if err != nil {
				t.Fatalf("Failed to list policies for 'ProductionTenants': %v", err)
			}

			if len(listPoliciesOutput.Policies) > 0 {
				t.Fatalf("Policies found on OU: %v", listPoliciesOutput.Policies)
			}

		},
		Targets: "organization",
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

			ymlparser.HydrateParsedOrg(ctx, test.OrgInitialState)
			orgOps := resourceoperation.CollectOrganizationUnitOps(
				ctx, consoleUI, orgClient, mgmtAcct, test.OrgInitialState, resourceoperation.Deploy,
			)
			for _, op := range orgOps {
				err := op.Call(ctx)
				if err != nil {
					assert.NoError(t, err, "Error creating organization initial state")
				}
			}

			// Ignore stacks because we do not know which stacks were deployed to the org in AWS.
			fetchedOrg, err := orgClient.FetchOUAndDescendents(ctx, rootId, mgmtAcct.AccountID)
			assert.NoError(t, err, "Failed to fetch rootOU")

			compareOrganizationUnits(t, test.OrgInitialState, &fetchedOrg, true)
		}

		err = ioutil.WriteFile("organization.yml", []byte(test.OrgYaml), 0644)
		assert.NoError(t, err, "Failed to write organization.yml")

		parsedOrg, err := ymlparser.ParseOrganizationV2(ctx, "organization.yml")
		assert.NoError(t, err, "Failed to parse organization.yml")

		compareOrganizationUnits(t, test.Expected, parsedOrg, false)

		cmd.ProcessOrgEndToEnd(consoleUI, resourceoperation.Deploy, test.Targets)

		fetchedOrg, err := orgClient.FetchOUAndDescendents(ctx, rootId, mgmtAcct.AccountID)
		assert.NoError(t, err, "Failed to fetch rootOU")

		// Ignore stacks because we do not know which stacks were deployed to the org in AWS.
		compareOrganizationUnits(t, test.Expected, &fetchedOrg, true)

		test.ExpectedResources(t)
	}
}
