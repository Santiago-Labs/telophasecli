package tests

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/santiago-labs/telophasecli/resource"
	"github.com/stretchr/testify/assert"
)

func setup() error {
	fmt.Println("Running setup")

	os.Setenv("LOCALSTACK", "true")
	os.Setenv("AWS_REGION", "us-east-1")

	cmd := exec.Command("bash", "setup.sh")
	if _, stderr, err := runCmd(cmd); err != nil {
		return fmt.Errorf("Failed to run setup: %v\n %s \n", err, stderr)
	}

	cacheProvider()
	return nil
}

func setupTest() {
	resetCmd := exec.Command("curl", "-v", "--request", "POST", "http://localhost:4566/_localstack/state/reset")
	if _, stderr, err := runCmd(resetCmd); err != nil {
		fmt.Printf("Failed to reset localstack state: %v\n %s \n ", err, stderr)
		os.Exit(1)
	}

	cmd := exec.Command("awslocal", "organizations", "create-organization", "--feature-set", "ALL")
	if _, stderr, err := runCmd(cmd); err != nil {
		fmt.Printf("Failed to create localstack org: %v\n %s \n ", err, stderr)
		os.Exit(1)
	}
}

func cacheProvider() {
	version, err := getLatestProviderVersion("https://api.github.com/repos/hashicorp/terraform-provider-aws/releases/latest")
	if err != nil {
		fmt.Printf("Error fetching version: %v\n", err)
		return
	}

	if string(version[0]) == "v" {
		version = version[1:]
	}

	url := fmt.Sprintf("https://releases.hashicorp.com/terraform-provider-aws/%s/terraform-provider-aws_%s_%s_%s.zip", version, version, runtime.GOOS, runtime.GOARCH)

	destinationDir := filepath.Join(os.Getenv("HOME"), ".terraform.d", "plugins", "registry.terraform.io", "hashicorp", "aws", version, fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH))
	err = downloadAndUnzip(url, destinationDir)
	if err != nil {
		fmt.Printf("Error downloading and unzipping file: %v\n", err)
		return
	}

	fmt.Println("Provider installed successfully.")
}

func getLatestProviderVersion(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch data: %s", resp.Status)
	}

	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.TagName, nil
}

func downloadAndUnzip(url, destinationDir string) error {
	tmpFile, err := os.CreateTemp("", "provider-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	r, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}

		path := filepath.Join(destinationDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			var fdir string
			if lastIndex := strings.LastIndex(path, string(os.PathSeparator)); lastIndex > -1 {
				fdir = path[:lastIndex]
			}
			err = os.MkdirAll(fdir, os.ModePerm)
			if err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		rc.Close()
	}

	return nil
}

func teardown() error {
	fmt.Println("Running teardown")
	cmd := exec.Command("bash", "teardown.sh")
	if _, stderr, err := runCmd(cmd); err != nil {
		return fmt.Errorf("Failed to run teardown: %v\n %s \n", err, stderr)
	}
	return nil
}

func TestMain(m *testing.M) {
	err := setup()
	if err != nil {
		fmt.Println(err)
		teardown()
		os.Exit(1)
	}
	code := m.Run()
	teardown()
	os.Exit(code)
}

func compareOrganizationUnits(t *testing.T, expected, actual *resource.OrganizationUnit, ignoreStacks bool) {
	assert.Equal(t, expected.OUName, actual.OUName, "OU Name not equal")

	sort.Slice(expected.ChildOUs, func(i, j int) bool {
		return expected.ChildOUs[i].OUName < expected.ChildOUs[j].OUName
	})
	sort.Slice(actual.ChildOUs, func(i, j int) bool {
		return actual.ChildOUs[i].OUName < actual.ChildOUs[j].OUName
	})
	diff := cmp.Diff(expected.ChildOUs, actual.ChildOUs)
	assert.Equal(t, len(expected.ChildOUs), len(actual.ChildOUs), "Child OUs not equal: %v", diff)

	sort.Slice(expected.Accounts, func(i, j int) bool {
		return expected.Accounts[i].AccountName < expected.Accounts[j].AccountName
	})
	sort.Slice(actual.Accounts, func(i, j int) bool {
		return actual.Accounts[i].AccountName < actual.Accounts[j].AccountName
	})

	acctDiff := cmp.Diff(expected.Accounts, actual.Accounts)
	assert.Equal(t, len(expected.Accounts), len(actual.Accounts), "Accounts not equal: %v", acctDiff)

	if !ignoreStacks {
		assert.Equal(t, expected.BaselineStacks, actual.BaselineStacks)
		assert.Equal(t, expected.ServiceControlPolicies, actual.ServiceControlPolicies)
	}

	for i, childOU := range expected.ChildOUs {
		compareOrganizationUnits(t, childOU, actual.ChildOUs[i], ignoreStacks)
	}

	for i, account := range expected.Accounts {
		compareAccounts(t, account, actual.Accounts[i], ignoreStacks)
	}
}

func compareAccounts(t *testing.T, expected, actual *resource.Account, ignoreStacks bool) {
	assert.Equal(t, expected.Email, actual.Email, "Account Emails not equal")
	assert.Equal(t, expected.AccountName, actual.AccountName, "Account Name not equal")
	assert.Equal(t, expected.State, actual.State, "Account State not equal")
	assert.Equal(t, expected.AssumeRoleName, actual.AssumeRoleName, "Account AssumeRoleName not equal")
	assert.Equal(t, expected.ManagementAccount, actual.ManagementAccount, "Account ManagementAccount not equal")
	assert.Equal(t, expected.Tags, actual.Tags, "Account Tags not equal")

	if !ignoreStacks {
		assert.Equal(t, expected.BaselineStacks, actual.BaselineStacks, "Account BaselineStacks not equal")
		assert.Equal(t, expected.ServiceControlPolicies, actual.ServiceControlPolicies, "Account ServiceControlPolicies not equal")
	}
}

func runCmd(cmd *exec.Cmd) (string, string, error) {
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("[ERROR] %v", err)
	}

	if err := cmd.Wait(); err != nil {
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("[ERROR] %v", err)
	}

	return stdoutBuf.String(), stderrBuf.String(), nil
}
