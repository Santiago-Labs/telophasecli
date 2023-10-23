package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	cdktemplates "telophasecli/lib/templates"
	"telophasecli/lib/ymlparser"
	"text/template"

	"github.com/spf13/cobra"
)

var tenant string
var sourceCodePath string
var awsAccountID string
var orgs ymlparser.OrgData

func init() {
	rootCmd.AddCommand(compileCmd)
	compileCmd.Flags().StringVar(&tenant, "tenant", "", "Name of the tenant to provision/deploy.")
	compileCmd.MarkFlagRequired("tenant")
	compileCmd.Flags().StringVar(&sourceCodePath, "source", "", "Path to the source code that will be deployed for the lambda")
	compileCmd.MarkFlagRequired("source")
	compileCmd.Flags().StringVar(&awsAccountID, "accountID", "", "AWS account ID")
	compileCmd.MarkFlagRequired("accountID")
}

var compileCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy - Deploy a tenant to the Telophase platform. If there is no existing tenant the infrastructure will be stood up on its own.",
	Run: func(cmd *cobra.Command, args []string) {
		cmdStr := "cdk"
		cdkArgs := []string{"deploy", "--require-approval=never"}

		tmpDir, err := writeCdkFiles()
		if err != nil {
			fmt.Println("Error writing CDK files:", err)
			return
		}
		fmt.Println("Wrote temp directory:", tmpDir)

		goCmd := exec.Command("go", "get")
		goCmd.Dir = tmpDir
		goCmd.Stdout = os.Stdout
		goCmd.Stderr = os.Stderr
		if err := goCmd.Run(); err != nil {
			fmt.Println("Error running go get:", err)
			return
		}

		cmdCdk := exec.Command(cmdStr, cdkArgs...)
		cmdCdk.Dir = tmpDir
		cmdCdk.Stdout = os.Stdout
		cmdCdk.Stderr = os.Stderr
		cmdCdk.Env = append(os.Environ(), fmt.Sprintf("TELOPHASE_CUSTOMER=%s", tenant))

		err = cmdCdk.Run()
		if err != nil {
			fmt.Printf("Command finished with error: %v\n", err)
		}
	},
}

// writeCdkFiles writes the setup for the CDK files.
func writeCdkFiles() (string, error) {
	// Create a new temporary directory
	tmpDir, err := ioutil.TempDir("", "cdkproj")
	if err != nil {
		fmt.Println("Error creating temp directory:", err)
		return "", err
	}

	fmt.Println("Created temporary directory:", tmpDir)
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
