package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"text/template"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/telophase"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
)

var orgFile string
var orgV1 bool

func init() {
	rootCmd.AddCommand(accountProvision)
	accountProvision.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	accountProvision.Flags().BoolVar(&orgV1, "orgv1", false, "Used for import only. Use this flag to import your organization in the old format.")
}

func isValidAccountArg(arg string) bool {
	switch arg {
	case "import":
		return true
	case "plan":
		return true
	case "apply":
		return true
	default:
		return false
	}
}

var accountProvision = &cobra.Command{
	Use:   "account",
	Short: "account - Provision and import AWS accounts",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires at least one arg")
		}
		if isValidAccountArg(args[0]) {
			return nil
		}
		return fmt.Errorf("invalid color specified: %s", args[0])
	},
	Run: func(cmd *cobra.Command, args []string) {
		orgClient := awsorgs.New()
		ctx := context.Background()
		if args[0] == "import" {
			if orgV1 {
				if err := importOrgV1(orgClient); err != nil {
					panic(fmt.Sprintf("error importing accounts: %s", err))
				}
			} else {
				if err := importOrgV2(orgClient); err != nil {
					panic(fmt.Sprintf("error importing organization: %s", err))
				}
			}
		}

		if args[0] == "plan" {
			if ymlparser.IsUsingOrgV1(orgFile) {
				_, _, err := orgV1Plan(orgClient)
				if err != nil {
					panic(fmt.Sprintf("error: %s", err))
				}
			} else {
				_, err := orgV2Plan(orgClient)
				if err != nil {
					panic(fmt.Sprintf("error: %s", err))
				}
			}
		}

		if args[0] == "apply" {
			if ymlparser.IsUsingOrgV1(orgFile) {
				newAccounts, _, err := orgV1Plan(orgClient)
				if err != nil {
					panic(fmt.Sprintf("error: %s", err))
				}

				errs := orgClient.CreateAccounts(ctx, newAccounts)
				if errs != nil {
					panic(fmt.Sprintf("error creating accounts %v", errs))
				}
			} else {
				operations, err := orgV2Plan(orgClient)
				if err != nil {
					panic(fmt.Sprintf("error: %s", err))
				}

				for _, op := range operations {
					err := op.Call(ctx, orgClient)
					if err != nil {
						panic(fmt.Sprintf("error: %s", err))
					}
				}
			}
		}
	},
}

func orgV1Plan(orgClient awsorgs.Client) (new []*organizations.Account, toDelete []*organizations.Account, err error) {
	org, err := ymlparser.ParseOrganizationV1(orgFile)
	if err != nil {
		panic(fmt.Sprintf("error: %s parsing organization", err))
	}

	accts, err := orgClient.CurrentAccounts(context.Background())
	if err != nil {
		return nil, nil, fmt.Errorf("error: %s getting current accounts", err)
	}
	accountsByEmail := make(map[string]*organizations.Account)
	for i, acct := range accts {
		if _, ok := accountsByEmail[*acct.Email]; ok {
			return nil, nil, fmt.Errorf("duplicate email %s", *acct.Email)
		}
		accountsByEmail[*acct.Email] = accts[i]
	}

	var newAccounts []*organizations.Account
	var deletedAccounts []*organizations.Account
	for _, account := range org.ChildAccounts {
		acct := account
		if currAcct, ok := accountsByEmail[account.Email]; !ok {
			if account.State == "" {
				newAccounts = append(newAccounts, &organizations.Account{
					Name:  &acct.AccountName,
					Email: &acct.Email,
				})
			}
		} else {
			if account.State == "delete" {
				deletedAccounts = append(deletedAccounts, currAcct)
			}
		}
	}

	if len(newAccounts) > 0 {
		const tmpl = `Account(s) to provision:{{range . }}
	+	AccountName: {{ .Name }}
	+	Email: {{ .Email }}

	{{end }}`

		t := template.Must(template.New("accounts").Parse(tmpl))
		var b bytes.Buffer
		if err := t.Execute(&b, newAccounts); err != nil {
			return nil, nil, err
		}
		fmt.Println(color.GreenString(b.String()))
	}

	if len(deletedAccounts) > 0 {
		const tmpl = `Account(s) to delete:{{range . }}
	+	AccountName: {{ .Name }}
	+	Email: {{ .Email }}

	{{end }}`

		t := template.Must(template.New("accounts").Parse(tmpl))
		var b bytes.Buffer
		if err := t.Execute(&b, deletedAccounts); err != nil {
			return nil, nil, err
		}
		fmt.Println(color.RedString(b.String()))
	}

	// Only output unchanged message if using legacy org definition.
	if len(org.ChildAccounts) > 0 {
		if len(newAccounts) == 0 && len(deletedAccounts) == 0 {
			fmt.Println("Organization.yml unchanged")
		}
	}

	return newAccounts, deletedAccounts, nil
}

func orgV2Plan(orgClient awsorgs.Client) (ops []ymlparser.ResourceOperation, err error) {
	org, err := ymlparser.ParseOrganizationV2(orgFile)
	if err != nil {
		panic(fmt.Sprintf("error: %s parsing organization", err))
	}

	operations := org.Diff(orgClient)
	for _, op := range ymlparser.FlattenOperations(operations) {
		fmt.Println(op.ToString())
	}

	return operations, nil
}

func currentAccountID() (string, error) {
	stsClient := sts.New(session.Must(session.NewSession()))
	caller, err := stsClient.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}

	return *caller.Account, nil
}

func importOrgV1(orgClient awsorgs.Client) error {
	accounts, err := orgClient.CurrentAccounts(context.Background())
	if err != nil {
		return fmt.Errorf("error: %s getting current accounts", err)
	}

	// Assume that the current role is the management account
	managingAccountID, err := currentAccountID()
	if err != nil {
		return err
	}

	var childAccounts []ymlparser.Account
	var mgmtAccount ymlparser.Account
	for _, acct := range accounts {
		telophase.UpsertAccount(*acct.Id, *acct.Name)
		if *acct.Id == managingAccountID {
			mgmtAccount = ymlparser.Account{
				AccountName: *acct.Name,
				AccountID:   *acct.Id,
			}
		} else {
			childAccounts = append(childAccounts, ymlparser.Account{
				AccountName: *acct.Name,
				AccountID:   *acct.Id,
			})
		}
	}

	org := &ymlparser.Organization{
		ManagementAccount: mgmtAccount,
		ChildAccounts:     childAccounts,
	}
	// Assume that the current role is the master account
	if err := ymlparser.WriteOrgV1File(orgFile, org); err != nil {
		return err
	}

	return nil
}

func importOrgV2(orgClient awsorgs.Client) error {
	accounts, err := orgClient.CurrentAccounts(context.Background())
	if err != nil {
		return fmt.Errorf("error: %s getting current accounts", err)
	}

	managingAccountID, err := currentAccountID()
	if err != nil {
		return err
	}

	for _, acct := range accounts {
		telophase.UpsertAccount(*acct.Id, *acct.Name)
	}

	rootId, err := orgClient.GetRootId()
	if err != nil {
		return err
	}
	if rootId == "" {
		return fmt.Errorf("no root ID found")
	}

	rootGroup, err := ymlparser.FetchGroupAndDescendents(context.TODO(), orgClient, rootId, managingAccountID)
	if err != nil {
		return err
	}
	org := ymlparser.AccountGroup{
		Name:        rootGroup.Name,
		ChildGroups: rootGroup.ChildGroups,
		Accounts:    rootGroup.Accounts,
	}

	if err := ymlparser.WriteOrgV2File(orgFile, &org); err != nil {
		return err
	}

	return nil
}
