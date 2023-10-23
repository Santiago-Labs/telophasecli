package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"telophasecli/lib/awsorgs"
	"telophasecli/lib/ymlparser"
	"text/template"

	"github.com/aws/aws-sdk-go/service/organizations"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(accountProvision)
}

func isValidAccountArg(arg string) bool {
	switch arg {
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
	Short: "account - Provision new accounts",
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
		newAccounts, _, err := accountsPlan(orgClient)
		if err != nil {
			panic(fmt.Sprintf("error: %s", err))
		}

		if args[0] == "apply" {
			errs := orgClient.CreateAccounts(ctx, newAccounts)
			if errs != nil {
				panic(fmt.Sprintf("error creating accounts %v", errs))
			}
		}
	},
}

func accountsPlan(orgClient awsorgs.Client) (new []ymlparser.Account, toDelete []*organizations.Account, err error) {
	// With accountsPlan we want to look at the current accounts and see if we
	// can add any accounts.
	orgs, err = ymlparser.ParseOrganizations("organizations.yml")
	if err != nil {
		panic(fmt.Sprintf("error: %s parsing organizations", err))
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

	var newAccounts []ymlparser.Account
	var deletedAccounts []*organizations.Account
	for _, account := range orgs.Organizations.ChildAccounts {
		acct := account
		if currAcct, ok := accountsByEmail[account.Email]; !ok {
			if account.Properties.State == "" {
				newAccounts = append(newAccounts, acct)
			}
		} else {
			if account.Properties.State == "delete" {
				deletedAccounts = append(deletedAccounts, currAcct)
			}
		}
	}

	if len(newAccounts) > 0 {
		const tmpl = `Account(s) to provision:{{range . }}
	+	AccountName: {{ .Properties.AccountName }}
	+	Email: {{ .Properties.Email }}

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
	+	AccountName: {{ .Properties.AccountName }}
	+	Email: {{ .Properties.Email }}

	{{end }}`

		t := template.Must(template.New("accounts").Parse(tmpl))
		var b bytes.Buffer
		if err := t.Execute(&b, deletedAccounts); err != nil {
			return nil, nil, err
		}
		fmt.Println(color.RedString(b.String()))
	}

	if len(newAccounts) == 0 && len(deletedAccounts) == 0 {
		fmt.Println("No accounts changed.")
	}

	return newAccounts, deletedAccounts, nil
}
