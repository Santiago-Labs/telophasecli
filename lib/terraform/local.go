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
	"github.com/santiago-labs/telophasecli/resource"
)

func TmpPath(acct resource.Account, filePath string) string {
	hasher := sha256.New()
	hasher.Write([]byte(filePath))
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return path.Join("telophasedirs", fmt.Sprintf("tf-tmp%s-%s", acct.ID(), hashString))
}

func CopyDir(stack resource.Stack, dst string, resource resource.Resource) error {
	ignoreDir := "telophasedirs"

	abs, err := filepath.Abs(stack.Path)
	if err != nil {
		return oops.Wrapf(err, "could not get absolute file path for path: %s", stack.Path)
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
			return replaceVariablesInFile(path, targetPath, resource, stack)
		}
	})
}

func replaceVariablesInFile(srcFile, dstFile string, resource resource.Resource, stack resource.Stack) error {
	fileInfo, err := os.Stat(srcFile)
	if err != nil {
		return oops.Wrapf(err, "error accessing file %s", srcFile)
	}

	content, err := ioutil.ReadFile(srcFile)
	if err != nil {
		return err
	}

	resourceType := strings.Join(strings.Split(strings.ToLower(resource.Type()), " "), "_")
	updatedContent := strings.ReplaceAll(string(content), fmt.Sprintf("${telophase.%s_id}", resourceType), resource.ID())
	updatedContent = strings.ReplaceAll(updatedContent, fmt.Sprintf("telophase.%s_id", resourceType), fmt.Sprintf("\"%s\"", resource.ID()))
	updatedContent = strings.ReplaceAll(updatedContent, fmt.Sprintf("${telophase.%s_name}", resourceType), resource.Name())
	updatedContent = strings.ReplaceAll(updatedContent, fmt.Sprintf("telophase.%s_name", resourceType), fmt.Sprintf("\"%s\"", resource.Name()))

	updatedContent = strings.ReplaceAll(updatedContent, "${telophase.resource_id}", resource.ID())
	updatedContent = strings.ReplaceAll(updatedContent, "telophase.resource_id", fmt.Sprintf("\"%s\"", resource.ID()))
	updatedContent = strings.ReplaceAll(updatedContent, "${telophase.resource_name}", resource.Name())
	updatedContent = strings.ReplaceAll(updatedContent, "telophase.resource_name", fmt.Sprintf("\"%s\"", resource.Name()))

	// Update Region
	preRegionContent := updatedContent
	updatedContent = strings.ReplaceAll(updatedContent, "${telophase.region}", stack.Region)
	updatedContent = strings.ReplaceAll(updatedContent, "telophase.region", stack.Region)
	if updatedContent != preRegionContent && stack.Region == "" {
		return oops.Errorf("Region needs to be set on stack if performing substitution")
	}

	return ioutil.WriteFile(dstFile, []byte(updatedContent), fileInfo.Mode())
}
