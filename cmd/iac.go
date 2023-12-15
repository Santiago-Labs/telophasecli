package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/rivo/tview"
	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/lib/awscloudformation"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/azureiam"
	"github.com/santiago-labs/telophasecli/lib/azureorgs"
	"github.com/santiago-labs/telophasecli/lib/cdk"
	"github.com/santiago-labs/telophasecli/lib/cdk/template"
	"github.com/santiago-labs/telophasecli/lib/colors"
	"github.com/santiago-labs/telophasecli/lib/telophase"
	"github.com/santiago-labs/telophasecli/lib/terraform"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
)

type iacCmd interface {
	cdkCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack, prevOutputs []*template.CDKOutputs) *exec.Cmd
	tfCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack) *exec.Cmd
	cdkOutputs(cfnClient awscloudformation.Client, acct ymlparser.Account, stack ymlparser.Stack) []*template.CDKOutputs
	orgV1Cmd(ctx context.Context, orgClient awsorgs.Client) // Deprecated
	orgV2Cmd(ctx context.Context, orgClient awsorgs.Client, subsClient *azureorgs.Client)
}

func runIAC(cmd iacCmd) {
	orgClient := awsorgs.New()
	subsClient, err := azureorgs.New()
	if err != nil {
		panic(fmt.Sprintf("error: %s", err))
	}
	ctx := context.Background()

	var accountsToApply []ymlparser.Account
	if ymlparser.IsUsingOrgV1(orgFile) {
		cmd.orgV1Cmd(ctx, orgClient)
		org, err := ymlparser.ParseOrganizationV1(orgFile)
		if err != nil {
			panic(fmt.Sprintf("error: %s parsing organization", err))
		}

		if contains(tag, org.ManagementAccount.Tags) || tag == "" {
			accountsToApply = append(accountsToApply, org.ManagementAccount)
		}
		for _, acct := range org.ChildAccounts {
			if contains(tag, acct.AllTags()) || tag == "" {
				accountsToApply = append(accountsToApply, acct)
			}
		}
	} else {
		cmd.orgV2Cmd(ctx, orgClient, subsClient)
		rootGroup, azureGroup, err := ymlparser.ParseOrganizationV2(orgFile)
		if err != nil {
			panic(fmt.Sprintf("error: %s parsing organization", err))
		}
		for _, acct := range rootGroup.AllDescendentAccounts() {
			if contains(tag, acct.AllTags()) || tag == "" {
				accountsToApply = append(accountsToApply, *acct)
			}
		}
		for _, acct := range azureGroup.AllDescendentAccounts() {
			if contains(tag, acct.AllTags()) || tag == "" {
				accountsToApply = append(accountsToApply, *acct)
			}
		}
	}

	if len(accountsToApply) == 0 {
		fmt.Println("No accounts to deploy")
	}

	if useTUI {
		deployTUI(cmd, accountsToApply)
		return
	}

	// Now for each to apply we will take that and write to stdout.
	var wg sync.WaitGroup
	for i := range accountsToApply {
		wg.Add(1)
		go func(acct ymlparser.Account) {
			colorFunc := colors.DeterministicColorFunc(acct.AccountID)
			defer wg.Done()
			if acct.AccountID == "" && acct.SubscriptionID == "" {
				fmt.Println(colorFunc(fmt.Sprintf("skipping account: %s because it hasn't been provisioned yet", acct.AccountName)))
				return
			}
			var accountRole *sts.AssumeRoleOutput
			var svc *sts.STS
			if acct.AccountID != "" {
				sess := session.Must(session.NewSession(&aws.Config{}))
				svc := sts.New(sess)
				fmt.Println("assuming role", colorFunc(acct.AssumeRoleARN()))
				input := &sts.AssumeRoleInput{
					RoleArn:         aws.String(acct.AssumeRoleARN()),
					RoleSessionName: aws.String("telophase-org"),
				}

				var err error
				accountRole, err = svc.AssumeRole(input)
				if err != nil {
					fmt.Println("Error assuming role:", err)
					return
				}
			}

			var acctStacks []ymlparser.Stack
			if cdkPath == "" && tfPath == "" {
				if stacks != "" && stacks != "*" {
					acctStacks = append(acctStacks, acct.FilterStacks(stacks)...)
				} else {
					acctStacks = append(acctStacks, acct.AllStacks()...)
				}
			} else {
				if cdkPath != "" {
					acctStacks = append(acctStacks, ymlparser.Stack{
						Path: cdkPath,
						Name: stacks,
						Type: "CDK",
					})
				}
				if tfPath != "" {
					acctStacks = append(acctStacks, ymlparser.Stack{
						Path: tfPath,
						Name: stacks,
						Type: "Terraform",
					})
				}
			}

			coloredAccountID := colorFunc("[Account: " + acct.ID() + "]")

			if len(acctStacks) == 0 {
				fmt.Printf("%s No stacks to deploy\n", coloredAccountID)
				return
			}
			var cdkOutputs []*template.CDKOutputs

			var bootstrappedCDK bool
			for _, stack := range acctStacks {
				fmt.Printf("%s executing stack: %s (empty string means all stacks) \n", coloredAccountID, stack.Name)
				stackRole := accountRole
				if stack.RoleOverrideARN != "" {
					fmt.Printf("%s assuming role: %s \n", coloredAccountID, colorFunc(stack.RoleOverrideARN))
					input := &sts.AssumeRoleInput{
						RoleArn:         aws.String(stack.RoleOverrideARN),
						RoleSessionName: aws.String("telophase-org"),
					}

					stackRole, err = svc.AssumeRole(input)
					if err != nil {
						fmt.Println("Error assuming role:", err)
						return
					}
				}
				if stack.Type == "CDK" {
					cfnClient := awscloudformation.New(stackRole.Credentials)
					if !bootstrappedCDK {
						bootstrapCDK := bootstrapCDK(stackRole, acct, stack)
						if err := runCmd(bootstrapCDK, acct, coloredAccountID); err != nil {
							fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
							return
						}
						bootstrappedCDK = true
					}

					synthCDK := synthCDK(stackRole, acct, stack, cdkOutputs)
					if err := runCmd(synthCDK, acct, coloredAccountID); err != nil {
						fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
						return
					}
					deployCDKCmd := cmd.cdkCmd(stackRole, acct, stack, cdkOutputs)
					if err := runCmd(deployCDKCmd, acct, coloredAccountID); err != nil {
						fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
						return
					} else {
						telophase.RecordDeploy(acct.AccountID, acct.AccountName)
					}
					cdkOutputs = append(cdkOutputs, cmd.cdkOutputs(cfnClient, acct, stack)...)

				} else if stack.Type == "Terraform" {
					initTFCmd := initTf(stackRole, acct, stack)
					if initTFCmd != nil {
						if err := runCmd(initTFCmd, acct, coloredAccountID); err != nil {
							fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
							return
						}
					}
					deployTFCmd := cmd.tfCmd(stackRole, acct, stack)
					if err := runCmd(deployTFCmd, acct, coloredAccountID); err != nil {
						fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
						return
					} else {
						telophase.RecordDeploy(acct.AccountID, acct.AccountName)
					}
				} else {
					panic(fmt.Errorf("unsupported stack type: %s", stack.Type))
				}
			}
		}(accountsToApply[i])
	}

	wg.Wait()
}

// runCmd takes the command and acct and runs it while prepending the
// coloredAccountID from stderr and stdout and printing it.
func runCmd(cmd *exec.Cmd, acct ymlparser.Account, coloredAccountID string) error {
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

func bootstrapCDK(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack) *exec.Cmd {
	outPath := cdk.TmpPath(acct, stack.Path)
	cdkArgs := []string{"bootstrap", "--output", outPath}
	cdkArgs = append(cdkArgs, "--context", fmt.Sprintf("telophaseAccountName=%s", acct.AccountName))
	cmd := exec.Command("cdk", cdkArgs...)
	cmd.Dir = stack.Path
	if result != nil {
		cmd.Env = awssts.SetEnviron(os.Environ(),
			*result.Credentials.AccessKeyId,
			*result.Credentials.SecretAccessKey,
			*result.Credentials.SessionToken)
	}

	return cmd
}

func synthCDK(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack, prevOutputs []*template.CDKOutputs) *exec.Cmd {
	outPath := cdk.TmpPath(acct, stack.Path)
	cdkArgs := []string{"synth", "--output", outPath}
	cdkArgs = append(cdkArgs, "--context", fmt.Sprintf("telophaseAccountName=%s", acct.AccountName))
	for _, prevOutput := range prevOutputs {
		for key, val := range prevOutput.Outputs {
			if reflect.TypeOf(val["Value"]).Kind() == reflect.Map {
				serializedVal, err := json.Marshal(val)
				if err != nil {
					panic(err)
				}
				cdkArgs = append(cdkArgs, "--context", fmt.Sprintf("%s.%s=%s", prevOutput.StackName, key, serializedVal))
			} else {
				cdkArgs = append(cdkArgs, "--context", fmt.Sprintf("%s.%s=%s", prevOutput.StackName, key, val["Value"]))
			}
		}
	}
	cmd := exec.Command("cdk", cdkArgs...)
	cmd.Dir = stack.Path
	if result != nil {
		cmd.Env = awssts.SetEnviron(os.Environ(),
			*result.Credentials.AccessKeyId,
			*result.Credentials.SecretAccessKey,
			*result.Credentials.SessionToken)
	}

	return cmd
}

func initTf(result *sts.AssumeRoleOutput, acct ymlparser.Account, stack ymlparser.Stack) *exec.Cmd {
	workingPath := terraform.TmpPath(acct, stack.Path)
	terraformDir := filepath.Join(workingPath, ".terraform")
	if terraformDir == "" || !strings.Contains(terraformDir, "telophasedirs") {
		panic(fmt.Errorf("expected terraform dir to be set"))
	}
	// Clean the directory
	if err := os.RemoveAll(terraformDir); err != nil {
		panic(fmt.Errorf("failed to remove directory %s: %w", terraformDir, err))
	}

	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		if err := os.MkdirAll(workingPath, 0755); err != nil {
			panic(fmt.Errorf("failed to create directory %s: %w", workingPath, err))
		}

		if err := terraform.CopyDir(stack.Path, workingPath, acct); err != nil {
			panic(fmt.Errorf("failed to copy files from %s to %s: %w", stack.Path, workingPath, err))
		}

		cmd := exec.Command("terraform", "init")
		cmd.Dir = workingPath

		// If result is nil then we are not using AWS
		if result != nil {
			cmd.Env = awssts.SetEnviron(os.Environ(),
				*result.Credentials.AccessKeyId,
				*result.Credentials.SecretAccessKey,
				*result.Credentials.SessionToken)
		}

		if acct.SubscriptionID != "" {
			resultEnv, err := azureiam.SetEnviron(os.Environ(), acct.SubscriptionID)
			if err != nil {
				panic(oops.Wrapf(err, "setting azure subscription id %s", acct.SubscriptionID))
			}

			cmd.Env = resultEnv
		}
		return cmd
	}

	return nil
}

func contains(e string, s []string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func deployTUI(cmd iacCmd, orgsToApply []ymlparser.Account) error {
	app := tview.NewApplication()
	tv := tview.NewTextView().SetDynamicColors(true)

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
		acctId := orgsToApply[wrapI].ID()
		if acctId == "" {
			acctId = "Not Yet Provisioned"
		}

		list.AddItem(acctId, orgsToApply[wrapI].AccountName, runeIndex(i), func() {
			currText := *tails[wrapI]
			// And we want to call this on repeat
			tuiIndex.Swap(int64(wrapI))
			tuiLock.Lock()
			defer tuiLock.Unlock()
			main.SetText(tview.TranslateANSI(currText()))
		})

		wg.Add(1)
		go func(acct ymlparser.Account, file *os.File) {
			defer wg.Done()
			if !acct.IsProvisioned() {
				return
			}

			var accountRole *sts.AssumeRoleOutput
			var svc *sts.STS
			if acct.AccountID != "" {
				sess := session.Must(session.NewSession(&aws.Config{}))
				svc = sts.New(sess)
				input := &sts.AssumeRoleInput{
					RoleArn:         aws.String(acct.AssumeRoleARN()),
					RoleSessionName: aws.String("telophase-org"),
				}
				var err error
				accountRole, err = svc.AssumeRole(input)
				if err != nil {
					fmt.Fprint(file, "Error assuming role:", err)
					return
				}
			}

			var acctStacks []ymlparser.Stack
			if cdkPath == "" && tfPath == "" {
				if stacks != "" && stacks != "*" {
					acctStacks = append(acctStacks, acct.FilterStacks(stacks)...)
				} else {
					acctStacks = append(acctStacks, acct.AllStacks()...)
				}
			} else {
				if cdkPath != "" {
					acctStacks = append(acctStacks, ymlparser.Stack{
						Path: cdkPath,
						Name: stacks,
						Type: "CDK",
					})
				}
				if tfPath != "" {
					acctStacks = append(acctStacks, ymlparser.Stack{
						Path: tfPath,
						Name: stacks,
						Type: "Terraform",
					})
				}
			}

			if len(acctStacks) == 0 {
				fmt.Fprint(file, "No stacks to deploy\n")
				return
			}

			var cdkOutputs []*template.CDKOutputs
			var bootstrappedCDK bool
			for _, stack := range acctStacks {
				fmt.Fprintf(file, "executing stack: %s (empty means all) \n", stack.Name)
				stackRole := accountRole
				if stack.RoleOverrideARN != "" {
					fmt.Fprintf(file, "assuming role: %s", stack.RoleOverrideARN)
					input := &sts.AssumeRoleInput{
						RoleArn:         aws.String(stack.RoleOverrideARN),
						RoleSessionName: aws.String("telophase-org"),
					}

					stackRole, err = svc.AssumeRole(input)
					if err != nil {
						fmt.Println("Error assuming role:", err)
						return
					}
				}
				if stack.Type == "CDK" {
					cfnClient := awscloudformation.New(stackRole.Credentials)
					if !bootstrappedCDK {
						bootstrapCDK := bootstrapCDK(stackRole, acct, stack)
						if err := runCmdWriter(bootstrapCDK, acct, file); err != nil {
							return
						}
						bootstrappedCDK = true
					}

					synthCDK := synthCDK(stackRole, acct, stack, cdkOutputs)
					if err := runCmdWriter(synthCDK, acct, file); err != nil {
						return
					}

					deployCDKCmd := cmd.cdkCmd(stackRole, acct, stack, cdkOutputs)
					if err := runCmdWriter(deployCDKCmd, acct, file); err != nil {
						return
					} else {
						telophase.RecordDeploy(acct.AccountID, acct.AccountName)
					}
					cdkOutputs = append(cdkOutputs, cmd.cdkOutputs(cfnClient, acct, stack)...)
				} else if stack.Type == "Terraform" {
					initTFCmd := initTf(stackRole, acct, stack)
					if initTFCmd != nil {
						if err := runCmdWriter(initTFCmd, acct, file); err != nil {
							return
						}
					}
					deployTFCmd := cmd.tfCmd(stackRole, acct, stack)
					if err := runCmdWriter(deployTFCmd, acct, file); err != nil {
						return
					} else {
						telophase.RecordDeploy(acct.AccountID, acct.AccountName)
					}
				} else {
					panic(fmt.Errorf("unsupported stack type: %s", stack.Type))
				}
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

func runeIndex(i int) rune {
	j := 0
	for r := 'a'; r <= 'p'; r++ {
		if j == i {
			return r
		}
		j++
	}

	return 'z'
}

func runCmdWriter(cmd *exec.Cmd, org ymlparser.Account, writer io.Writer) error {
	cmd.Stderr = writer
	cmd.Stdout = writer

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

// liveTextSetter updates the current tui view with the current tail's text.
func liveTextSetter(tv *tview.TextView, tails []*func() string) {
	for {
		func() {
			time.Sleep(200 * time.Millisecond)
			tuiLock.Lock()
			defer tuiLock.Unlock()
			f := *tails[tuiIndex.Load()]

			curr := tv.GetText(true)
			newText := f()
			if newText != curr && newText != "" {
				tv.SetText(tview.TranslateANSI(f()))
			}
		}()
	}
}
