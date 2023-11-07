package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"sync"

	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/colors"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVar(&cdkPath, "cdk-path", "", "Path to your CDK code")
	diffCmd.Flags().BoolVar(&allStacks, "all-stacks", false, "If all stacks should be deployed")
	diffCmd.Flags().StringVar(&stacks, "stacks", "", "List of specific stacks to deploy")
	diffCmd.Flags().StringVar(&accountTag, "account-tag", "", "Tag associated with the accounts to apply to a subset of account IDs, tag \"all\" to deploy all accounts.")
	diffCmd.MarkFlagRequired("account-tag")
	diffCmd.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	diffCmd.Flags().BoolVar(&useTUI, "tui", false, "use the TUI for diff")
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "diff - Show accounts to create/update and CDK changes for each account.",
	Run: func(cmd *cobra.Command, args []string) {
		orgClient := awsorgs.New()
		_, _, err := accountsPlan(orgClient)
		if err != nil {
			panic(fmt.Sprintf("error: %s", err))
		}

		orgs, err := ymlparser.ParseOrganization(orgFile)
		if err != nil {
			panic(fmt.Sprintf("error: %s parsing organization", err))
		}

		accountsToDiff := []ymlparser.Account{}
		for _, org := range orgs.ChildAccounts {
			if contains(accountTag, org.Tags) || accountTag == "all" {
				accountsToDiff = append(accountsToDiff, org)
			}
		}

		if useTUI {
			deployTUI(accountsToDiff)
			return
		}

		// Now for each to apply we will take that and write to stdout.
		var wg sync.WaitGroup
		for i := range accountsToDiff {
			wg.Add(1)
			go func(acct ymlparser.Account) {
				colorFunc := colors.DeterministicColorFunc(acct.AccountID)
				defer wg.Done()
				if acct.AccountID == "" {
					fmt.Println(colorFunc(fmt.Sprintf("skipping account: %s because it hasn't been provisioned yet", acct.AccountName)))
					return
				}

				sess := session.Must(session.NewSession(&aws.Config{}))
				svc := sts.New(sess)
				fmt.Println("assuming role", colorFunc(acct.AssumeRoleARN()))
				input := &sts.AssumeRoleInput{
					RoleArn:         aws.String(acct.AssumeRoleARN()), // Change this to your role ARN
					RoleSessionName: aws.String("telophase-org"),
				}

				result, err := svc.AssumeRole(input)
				if err != nil {
					fmt.Println("Error assuming role:", err)
					return
				}
				coloredAccountID := colorFunc("[Account: " + acct.AccountID + "]")
				bootstrapCDK := bootstrapCDK(result, acct, cdkPath)
				if err := runCmd(bootstrapCDK, acct, coloredAccountID); err != nil {
					fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
					return
				}

				diffCmd := diffCDK(result, acct, cdkPath)
				if err := runCmd(diffCmd, acct, coloredAccountID); err != nil {
					fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
					return
				}
			}(accountsToDiff[i])
		}

		wg.Wait()
	},
}

func diffCDK(result *sts.AssumeRoleOutput, acct ymlparser.Account, cdkPath string) *exec.Cmd {
	tmpPath := path.Join(cdkPath, "telophasedirs", fmt.Sprintf("tmp%s", acct.AccountID))
	cdkArgs := []string{"diff", "--output", tmpPath}
	cmd := exec.Command("cdk", cdkArgs...)
	cmd.Dir = cdkPath
	cmd.Env = awssts.SetEnviron(os.Environ(),
		*result.Credentials.AccessKeyId,
		*result.Credentials.SecretAccessKey,
		*result.Credentials.SessionToken)

	return cmd
}
