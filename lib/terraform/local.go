package terraform

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/lib/azureorgs"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
)

func TmpPath(acct ymlparser.Account, filePath string) string {
	hasher := sha256.New()
	hasher.Write([]byte(filePath))
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return path.Join("telophasedirs", fmt.Sprintf("tf-tmp%s-%s", acct.ID(), hashString))
}

func CopyDir(src string, dst string, acct ymlparser.Account) error {
	ignoreDir := "telophasedirs"

	abs, err := filepath.Abs(src)
	if err != nil {
		return oops.Wrapf(err, "could not get absolute file path for path: %s", src)
	}
	return filepath.Walk(abs, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.Contains(path, filepath.Join(abs, ignoreDir)) {
			return nil
		}

		relPath := strings.TrimPrefix(path, abs)
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
	updatedContent = strings.ReplaceAll(updatedContent, "${telophase.subscription_id}", acct.SubscriptionID)
	updatedContent = strings.ReplaceAll(updatedContent, "telophase.subscription_id", fmt.Sprintf("\"%s\"", acct.SubscriptionID))
	updatedContent = strings.ReplaceAll(updatedContent, "${telophase.storage_account_name}", azureorgs.StorageAccountName(acct.SubscriptionID))
	updatedContent = strings.ReplaceAll(updatedContent, "telophase.storage_account_name", fmt.Sprintf("\"%s\"", azureorgs.StorageAccountName(acct.SubscriptionID)))

	return ioutil.WriteFile(dstFile, []byte(updatedContent), 0644)
}
