package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"sync"
	"sync/atomic"
	"telophasecli/lib/awsorgs"
	"telophasecli/lib/awssts"
	"telophasecli/lib/colors"
	"telophasecli/lib/ymlparser"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
)

var (
	tenant         string
	sourceCodePath string
	awsAccountID   string
	cdkPath        string
	apply          bool
	accountTag     string
	orgs           ymlparser.Organization
	allStacks      bool
	stacks         string

	// TUI
	tuiDeploy bool
	tuiIndex  atomic.Int64
	tuiLock   sync.Mutex
)

func init() {
	rootCmd.AddCommand(compileCmd)
	compileCmd.Flags().StringVar(&cdkPath, "cdk-path", "", "Path to your CDK code")
	compileCmd.Flags().BoolVar(&apply, "apply", false, "Set apply to true if you want to deploy the changes to your account")
	compileCmd.Flags().BoolVar(&allStacks, "all-stacks", false, "If all stacks should be deployed")
	compileCmd.Flags().StringVar(&stacks, "stacks", "", "List of specific stacks to deploy")
	compileCmd.Flags().StringVar(&accountTag, "account-tag", "", "Tag associated with the accounts to apply to a subset of account IDs, tag \"all\" to deploy all accounts.")
	compileCmd.MarkFlagRequired("account-tag")
	compileCmd.Flags().StringVar(&orgFile, "org", "organization.yml", "Path to the organization.yml file")
	compileCmd.Flags().BoolVar(&tuiDeploy, "tui", false, "use the TUI for deploy")
}

var compileCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy - Deploy a tenant to the Telophase platform. If there is no existing tenant the infrastructure will be stood up on its own.",
	Run: func(cmd *cobra.Command, args []string) {
		orgClient := awsorgs.New()
		ctx := context.Background()
		newAccounts, _, err := accountsPlan(orgClient)
		if err != nil {
			panic(fmt.Sprintf("error: %s", err))
		}

		if apply {
			errs := orgClient.CreateAccounts(ctx, newAccounts)
			if errs != nil {
				panic(fmt.Sprintf("error creating accounts %v", errs))
			}
		}

		fmt.Println("parsing file", orgFile)
		orgs, err := ymlparser.ParseOrganization(orgFile)
		if err != nil {
			panic(fmt.Sprintf("error: %s parsing organization", err))
		}

		orgsToApply := []ymlparser.Account{}
		for _, org := range orgs.ChildAccounts {
			if contains(accountTag, org.Tags) || accountTag == "all" {
				orgsToApply = append(orgsToApply, org)
			}
		}

		if tuiDeploy {
			deployTUI(orgsToApply)
			return
		}

		// Now for each to apply we will take that and write to stdout.
		var wg sync.WaitGroup
		for i := range orgsToApply {
			wg.Add(1)
			go func(org ymlparser.Account) {
				colorFunc := colors.DeterministicColorFunc(org.AccountID)
				defer wg.Done()
				if org.AccountID == "" {
					fmt.Println(colorFunc(fmt.Sprintf("skipping account: %s because it hasn't been provisioned yet", org.AccountName)))
					return
				}

				sess := session.Must(session.NewSession(&aws.Config{}))
				svc := sts.New(sess)
				fmt.Println("assuming role", colorFunc(org.AssumeRoleARN()))
				input := &sts.AssumeRoleInput{
					RoleArn:         aws.String(org.AssumeRoleARN()), // Change this to your role ARN
					RoleSessionName: aws.String("telophase-org"),
				}

				result, err := svc.AssumeRole(input)
				if err != nil {
					fmt.Println("Error assuming role:", err)
					return
				}
				coloredAccountID := colorFunc("[Account: " + org.AccountID + "]")
				bootstrapCDK := bootstrapCDK(result, org, cdkPath, apply)
				if err := runCmd(bootstrapCDK, org, coloredAccountID); err != nil {
					fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
					return
				}

				deployCmd := deployCDK(result, org, cdkPath, apply)
				if err := runCmd(deployCmd, org, coloredAccountID); err != nil {
					fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
					return
				}
			}(orgsToApply[i])
		}

		wg.Wait()
	},
}

// runCmd takes the command and org and runs it while prepending the
// coloredAccountID from stderr and stdout and printing it.
func runCmd(cmd *exec.Cmd, org ymlparser.Account, coloredAccountID string) error {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("[ERROR] %s %v", coloredAccountID, err)
	}
	stdoutScanner := bufio.NewScanner(stdoutPipe)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("[ERROR] %s %v", coloredAccountID, err)
	}
	stderrScanner := bufio.NewScanner(stderrPipe)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[ERROR] %s %v", coloredAccountID, err)
	}

	var scannerWg sync.WaitGroup
	scannerWg.Add(2)
	scanF := func(scanner *bufio.Scanner, name string) {
		defer scannerWg.Done()
		for scanner.Scan() {
			fmt.Printf("%s %s\n", coloredAccountID, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
			return
		}
	}

	go scanF(stdoutScanner, "stdout")
	go scanF(stderrScanner, "stderr")
	scannerWg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("[ERROR] %s %v", coloredAccountID, err)
	}

	return nil
}

func bootstrapCDK(result *sts.AssumeRoleOutput, org ymlparser.Account, cdkPath string, apply bool) *exec.Cmd {
	tmpPath := path.Join(cdkPath, "telophasedirs", fmt.Sprintf("tmp%s", org.AccountID))
	cdkArgs := []string{"bootstrap", "--output", tmpPath}
	if apply {
		cdkArgs = append(cdkArgs, "--require-approval", "never")
	}
	cmd := exec.Command("cdk", cdkArgs...)
	cmd.Dir = cdkPath
	cmd.Env = awssts.SetEnviron(os.Environ(),
		*result.Credentials.AccessKeyId,
		*result.Credentials.SecretAccessKey,
		*result.Credentials.SessionToken)

	return cmd
}

func deployCDK(result *sts.AssumeRoleOutput, org ymlparser.Account, cdkPath string, apply bool) *exec.Cmd {
	tmpPath := path.Join(cdkPath, "telophasedirs", fmt.Sprintf("tmp%s", org.AccountID))
	cdkArgs := []string{"deploy", "--output", tmpPath}
	if apply {
		cdkArgs = append(cdkArgs, "--require-approval", "never")
	}
	cmd := exec.Command("cdk", cdkArgs...)
	cmd.Dir = cdkPath
	cmd.Env = awssts.SetEnviron(os.Environ(),
		*result.Credentials.AccessKeyId,
		*result.Credentials.SecretAccessKey,
		*result.Credentials.SessionToken)

	return cmd
}

func contains(e string, s []string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func deployTUI(orgsToApply []ymlparser.Account) error {
	app := tview.NewApplication()
	tv := tview.NewTextView()

	main := tv.
		SetTextAlign(tview.AlignLeft).SetScrollable(true).
		SetChangedFunc(func() {
			tv.ScrollToEnd()
			app.Draw()
		}).SetText("Starting CDK...")

	list := tview.NewList()
	var tails []*func() string

	// Now for each to apply we will take that and write to stdout.
	var wg sync.WaitGroup
	for i := range orgsToApply {
		file, err := ioutil.TempFile("/tmp", orgsToApply[i].AccountID)
		if err != nil {
			return err
		}

		setter := func() string {
			bytes, err := ioutil.ReadFile(file.Name())
			if err != nil {
				fmt.Printf("ERR: %s", err)
				return ""
			}

			return string(bytes)
		}

		tails = append(tails, &setter)

		wrapI := i
		acctId := orgsToApply[wrapI].AccountID
		if acctId == "" {
			acctId = "Not Yet Provisioned"
		}

		list.AddItem(acctId, orgsToApply[wrapI].AccountName, runeIndex(i), func() {
			currText := *tails[wrapI]
			// And we want to call this on repeat
			tuiIndex.Swap(int64(wrapI))
			tuiLock.Lock()
			defer tuiLock.Unlock()
			main.SetText(currText())
		})

		wg.Add(1)
		go func(org ymlparser.Account, file *os.File) {
			defer wg.Done()
			if org.AccountID == "" {
				return
			}
			sess := session.Must(session.NewSession(&aws.Config{}))
			svc := sts.New(sess)
			colorFunc := colors.DeterministicColorFunc(org.AccountID)
			input := &sts.AssumeRoleInput{
				RoleArn:         aws.String(org.AssumeRoleARN()), // Change this to your role ARN
				RoleSessionName: aws.String("telophase-org"),
			}

			result, err := svc.AssumeRole(input)
			if err != nil {
				fmt.Println("Error assuming role:", err)
				return
			}

			coloredAccountID := colorFunc("[Account: " + org.AccountID + "]")
			bootstrapCDK := bootstrapCDK(result, org, cdkPath, apply)
			if err := runCmdWriter(bootstrapCDK, org, file); err != nil {
				fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
				return
			}

			deployCmd := deployCDK(result, org, cdkPath, apply)
			if err := runCmdWriter(deployCmd, org, file); err != nil {
				fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
				return
			}
		}(orgsToApply[i], file)
	}

	list.AddItem("Quit", "Press to exit", 'q', func() {
		app.Stop()
	})

	// Start index at 0 for the first account.
	tuiIndex.Swap(0)

	go liveTextSetter(main, tails)

	grid := tview.NewGrid().
		SetColumns(-1, -3).
		SetRows(-1).
		SetBorders(true)

	// Layout for screens wider than 100 cells.
	grid.AddItem(list, 0, 0, 1, 1, 0, 100, false).
		AddItem(main, 0, 1, 1, 1, 0, 100, false)

	if err := app.SetRoot(grid, true).SetFocus(list).Run(); err != nil {
		panic(err)
	}

	wg.Wait()
	return nil
}
