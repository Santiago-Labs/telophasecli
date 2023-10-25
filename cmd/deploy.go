package cmd

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"telophasecli/lib/awssts"
	"telophasecli/lib/colors"
	cdktemplates "telophasecli/lib/templates"
	"telophasecli/lib/ymlparser"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/spf13/cobra"
)

var (
	tenant          string
	sourceCodePath  string
	awsAccountID    string
	cdkPath         string
	requireApproval string
	accountTag      string
	orgs            ymlparser.Organizations
)

func init() {
	rootCmd.AddCommand(compileCmd)
	// compileCmd.Flags().StringVar(&tenant, "tenant", "", "Name of the tenant to provision/deploy.")
	// compileCmd.MarkFlagRequired("tenant")
	// compileCmd.Flags().StringVar(&sourceCodePath, "source", "", "Path to the source code that will be deployed for the lambda")
	// compileCmd.MarkFlagRequired("source")
	// compileCmd.Flags().StringVar(&awsAccountID, "account-id", "", "AWS account ID")
	// compileCmd.MarkFlagRequired("account-id")
	compileCmd.Flags().StringVar(&cdkPath, "cdk-path", "", "Path to your CDK code")
	compileCmd.MarkFlagRequired("cdk-path")
	compileCmd.Flags().StringVar(&requireApproval, "require-approval", "", "Set require-approval to never to apply your changes")
	compileCmd.Flags().StringVar(&accountTag, "account-tag", "", "Tag associated with the accounts to apply to a subset of account IDs")
	compileCmd.MarkFlagRequired("account-tag")
	compileCmd.Flags().StringVar(&orgsFile, "orgs", "organizations.yml", "Path to the organizations.yml file")
}

var compileCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy - Deploy a tenant to the Telophase platform. If there is no existing tenant the infrastructure will be stood up on its own.",
	Run: func(cmd *cobra.Command, args []string) {
		// cmdStr := "cdk"
		// cdkArgs := []string{"deploy", "--require-approval=never"}

		orgs, err := ymlparser.ParseOrganizations(orgsFile)
		if err != nil {
			panic(fmt.Sprintf("error: %s parsing organizations", err))
		}

		orgsToApply := []ymlparser.Account{}
		for _, org := range orgs.ChildAccounts {
			if contains(accountTag, org.Tags) {
				orgsToApply = append(orgsToApply, org)
			}
		}
		fmt.Printf("%+v", orgsToApply)
		fmt.Println("orgs to apply", len(orgsToApply))

		// Now for each to apply we will take that and write to stdout.
		var wg sync.WaitGroup
		for i := range orgsToApply {
			wg.Add(1)
			go func(org ymlparser.Account) {
				defer wg.Done()
				sess := session.Must(session.NewSession(&aws.Config{}))
				svc := sts.New(sess)
				colorFunc := colors.DeterministicColorFunc(org.AccountID)
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

				tmpPath := path.Join(cdkPath, "telophasedirs", fmt.Sprintf("tmp%s", org.AccountID))
				cdkArgs := []string{"deploy", "--output", tmpPath}
				cmd := exec.Command("cdk", cdkArgs...)
				cmd.Dir = cdkPath
				cmd.Env = awssts.SetEnviron(os.Environ(),
					*result.Credentials.AccessKeyId,
					*result.Credentials.SecretAccessKey,
					*result.Credentials.SessionToken)

				coloredAccountID := colorFunc("[Account: " + org.AccountID + "]")

				stdoutPipe, err := cmd.StdoutPipe()
				if err != nil {
					fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
					return
				}
				stdoutScanner := bufio.NewScanner(stdoutPipe)

				stderrPipe, err := cmd.StderrPipe()
				if err != nil {
					fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
					return
				}
				stderrScanner := bufio.NewScanner(stderrPipe)

				if err := cmd.Start(); err != nil {
					fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
					return
				}

				var scannerWg sync.WaitGroup
				scannerWg.Add(2)
				scanF := func(scanner *bufio.Scanner, name string) {
					defer scannerWg.Done()
					for scanner.Scan() {
						// fmt.Printf("%s %s\n %s\n", coloredAccountID, scanner.Text(), colorFunc("end "+org.AccountID))
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

				// Wait for the command to finish
				if err := cmd.Wait(); err != nil {
					fmt.Printf("[ERROR] %s %v\n", coloredAccountID, err)
					return
				}
			}(orgsToApply[i])
		}

		wg.Wait()
	},
}

func contains(e string, s []string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// writeCdkFiles writes the setup for the CDK files.
func writeCdkFiles() (string, error) {
	// Create a new temporary directory
	tmpDir, err := ioutil.TempDir("", "cdkproj")
	if err != nil {
		fmt.Println("Error creating temp directory:", err)
		return "", err
	}

	files := map[string]string{
		"cdkapp.go": cdktemplates.CdkMainTmpl,
		"go.mod":    cdktemplates.GoModTmpl,
		"cdk.json":  cdktemplates.CdkJSONTmpl,
	}

	data := cdktemplates.CdkData{
		AwsAccountId:     awsAccountID,
		AwsAccountRegion: "us-west-2",
	}

	for fileName, tmplContent := range files {
		tmpl, err := template.New(fileName).Parse(tmplContent)
		if err != nil {
			fmt.Printf("Error parsing template for file %s: %v\n", fileName, err)
			return "", err
		}

		filePath := filepath.Join(tmpDir, fileName)
		file, err := os.Create(filePath)
		if err != nil {
			fmt.Printf("Error creating file %s: %v\n", filePath, err)
			return "", err
		}

		if err := tmpl.Execute(file, data); err != nil {
			fmt.Printf("Error executing template for file %s: %v\n", fileName, err)
			return "", err
		}
		file.Close()
	}
	return tmpDir, nil
}
