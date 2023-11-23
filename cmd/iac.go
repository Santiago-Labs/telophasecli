package cmd

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/rivo/tview"
	"github.com/santiago-labs/telophasecli/lib/awsorgs"
	"github.com/santiago-labs/telophasecli/lib/awssts"
	"github.com/santiago-labs/telophasecli/lib/colors"
	"github.com/santiago-labs/telophasecli/lib/telophase"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
)

type iacCmd interface {
	cdkCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, cdkPath string) *exec.Cmd
	tfCmd(result *sts.AssumeRoleOutput, acct ymlparser.Account, tfPath string) *exec.Cmd
}

func runIAC(cmd iacCmd) {
	orgClient := awsorgs.New()
	ctx := context.Background()
	newAccounts, _, err := accountsPlan(orgClient)
	if err != nil {
		panic(fmt.Sprintf("error: %s", err))
	}

	errs := orgClient.CreateAccounts(ctx, newAccounts)
	if errs != nil {
		panic(fmt.Sprintf("error creating accounts %v", errs))
	}

	orgs, err := ymlparser.ParseOrganization(orgFile)
	if err != nil {
		panic(fmt.Sprintf("error: %s parsing organization", err))
	}

	accountsToApply := []ymlparser.Account{}
	for _, org := range orgs.ChildAccounts {
		if contains(accountTag, org.Tags) || accountTag == "all" {
			accountsToApply = append(accountsToApply, org)
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
			if acct.AccountID == "" {
				fmt.Println(colorFunc(fmt.Sprintf("skipping account: %s because it hasn't been provisioned yet", acct.AccountName)))
				return
			}

			sess := session.Must(session.NewSession(&aws.Config{}))
			svc := sts.New(sess)
			fmt.Println("assuming role", colorFunc(acct.AssumeRoleARN()))
			input := &sts.AssumeRoleInput{
				RoleArn:         aws.String(acct.AssumeRoleARN()),
				RoleSessionName: aws.String("telophase-org"),
			}

			result, err := svc.AssumeRole(input)
			if err != nil {
				fmt.Println("Error assuming role:", err)
				return
			}

			acctStacks := make([]ymlparser.Stack, len(acct.Stacks))
			copy(acctStacks, acct.Stacks)
			if cdkPath != "" {
				acctStacks = append(acctStacks, ymlparser.Stack{
					Path: cdkPath,
					Name: "Command-line arg CDK",
					Type: "CDK",
				})
			}
			if tfPath != "" {
				acctStacks = append(acctStacks, ymlparser.Stack{
					Path: tfPath,
					Name: "Command-line arg TF",
					Type: "Terraform",
				})
			}

			coloredAccountID := colorFunc("[Account: " + acct.AccountID + "]")
			for _, stack := range acctStacks {
				if stack.Type == "CDK" {
					bootstrapCDK := bootstrapCDK(result, acct, stack.Path)
					if err := runCmd(bootstrapCDK, acct, coloredAccountID); err != nil {
						fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
						return
					}

					deployCDKCmd := cmd.cdkCmd(result, acct, stack.Path)
					if err := runCmd(deployCDKCmd, acct, coloredAccountID); err != nil {
						fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
						return
					} else {
						telophase.RecordDeploy(acct.AccountID, acct.AccountName)
					}
				} else if stack.Type == "Terraform" {
					initTFCmd := initTf(result, acct, stack.Path)
					if initTFCmd != nil {
						if err := runCmd(initTFCmd, acct, coloredAccountID); err != nil {
							fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
							return
						}
					}
					deployTFCmd := cmd.tfCmd(result, acct, stack.Path)
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

func bootstrapCDK(result *sts.AssumeRoleOutput, acct ymlparser.Account, cdkPath string) *exec.Cmd {
	outPath := tmpPath("CDK", acct, cdkPath)
	cdkArgs := []string{"bootstrap", "--output", outPath}
	cmd := exec.Command("cdk", cdkArgs...)
	cmd.Dir = cdkPath
	cmd.Env = awssts.SetEnviron(os.Environ(),
		*result.Credentials.AccessKeyId,
		*result.Credentials.SecretAccessKey,
		*result.Credentials.SessionToken)

	return cmd
}

func initTf(result *sts.AssumeRoleOutput, acct ymlparser.Account, tfPath string) *exec.Cmd {
	workingPath := tmpPath("Terraform", acct, tfPath)
	terraformDir := filepath.Join(workingPath, ".terraform")
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		if err := os.MkdirAll(workingPath, 0755); err != nil {
			panic(fmt.Errorf("failed to create directory %s: %w", workingPath, err))
		}

		if err := copyDir(tfPath, workingPath, acct); err != nil {
			panic(fmt.Errorf("failed to copy files from %s to %s: %w", tfPath, workingPath, err))
		}

		cmd := exec.Command("terraform", "init")
		cmd.Dir = workingPath
		cmd.Env = awssts.SetEnviron(os.Environ(),
			*result.Credentials.AccessKeyId,
			*result.Credentials.SecretAccessKey,
			*result.Credentials.SessionToken)

		return cmd
	}

	return nil
}

func tmpPath(iacType string, acct ymlparser.Account, filePath string) string {
	hasher := sha256.New()
	hasher.Write([]byte(tfPath))
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	if iacType == "Terraform" {
		return path.Join("telophasedirs", fmt.Sprintf("tf-tmp%s-%s", acct.AccountID, hashString))
	} else if iacType == "CDK" {
		return path.Join("telophasedirs", fmt.Sprintf("cdk-tmp%s-%s", acct.AccountID, hashString))
	}
	panic(fmt.Sprintf("unsupported iac type: %s", iacType))
}

func copyDir(src string, dst string, acct ymlparser.Account) error {
	ignoreDir := "telophasedirs"

	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.Contains(path, filepath.Join(src, ignoreDir)) {
			return nil
		}

		relPath := strings.TrimPrefix(path, src)
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		} else {
			return replaceVariablesInFile(path, targetPath, acct)
		}
	})
}

func replaceVariablesInFile(srcFile, dstFile string, acct ymlparser.Account) error {
	content, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return err
	}

	updatedContent := strings.ReplaceAll(string(content), "${telophase.account_id}", acct.AccountID)
	updatedContent = strings.ReplaceAll(updatedContent, "telophase.account_id", fmt.Sprintf("\"%s\"", acct.AccountID))
	updatedContent = strings.ReplaceAll(updatedContent, "${telophase.account_name}", acct.AccountName)
	updatedContent = strings.ReplaceAll(updatedContent, "telophase.account_name", fmt.Sprintf("\"%s\"", acct.AccountName))

	return ioutil.WriteFile(dstFile, []byte(updatedContent), 0644)
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
		go func(acct ymlparser.Account, file *os.File) {
			defer wg.Done()
			if acct.AccountID == "" {
				return
			}
			sess := session.Must(session.NewSession(&aws.Config{}))
			svc := sts.New(sess)
			input := &sts.AssumeRoleInput{
				RoleArn:         aws.String(acct.AssumeRoleARN()),
				RoleSessionName: aws.String("telophase-org"),
			}

			result, err := svc.AssumeRole(input)
			if err != nil {
				fmt.Println("Error assuming role:", err)
				return
			}

			acctStacks := make([]ymlparser.Stack, len(acct.Stacks))
			copy(acctStacks, acct.Stacks)
			if cdkPath != "" {
				acctStacks = append(acctStacks, ymlparser.Stack{
					Path: cdkPath,
					Name: "Command-line arg CDK",
					Type: "CDK",
				})
			}
			if tfPath != "" {
				acctStacks = append(acctStacks, ymlparser.Stack{
					Path: tfPath,
					Name: "Command-line arg TF",
					Type: "Terraform",
				})
			}

			for _, stack := range acctStacks {
				if stack.Type == "CDK" {
					bootstrapCDK := bootstrapCDK(result, acct, stack.Path)
					if err := runCmdWriter(bootstrapCDK, acct, file); err != nil {
						return
					}

					deployCDKCmd := cmd.cdkCmd(result, acct, stack.Path)
					if err := runCmdWriter(deployCDKCmd, acct, file); err != nil {
						return
					} else {
						telophase.RecordDeploy(acct.AccountID, acct.AccountName)
					}
				} else if stack.Type == "Terraform" {
					initTFCmd := initTf(result, acct, stack.Path)
					if initTFCmd != nil {
						if err := runCmdWriter(initTFCmd, acct, file); err != nil {
							return
						}
					}
					deployTFCmd := cmd.tfCmd(result, acct, stack.Path)
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
				tv.SetText(f())
			}
		}()
	}
}
