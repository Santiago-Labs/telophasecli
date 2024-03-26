package cdk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/samsarahq/go/oops"
	"github.com/santiago-labs/telophasecli/lib/awscloudformation"
	"github.com/santiago-labs/telophasecli/lib/cdk/template"
	"github.com/santiago-labs/telophasecli/lib/localstack"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
)

func getStackNames(acct ymlparser.Account, stack ymlparser.Stack) ([]string, error) {
	cmd := exec.Command(localstack.CdkCmd(), "ls", "--output", TmpPath(acct, stack.Name))
	cmd.Dir = stack.Path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return []string{}, oops.Wrapf(err, "%s ls path: (%s), output: (%s)", localstack.CdkCmd(), stack.Path, output)
	}

	// cdk ls can result in a few empty lines. Make sure we remove those to avoid returning a stack with an empty name.
	outputStr := string(output)
	results := strings.Split(outputStr, "\n")
	var stackNames []string
	for _, result := range results {
		trimmed := strings.TrimSpace(result)
		if trimmed == "" {
			continue
		}

		stackNames = append(stackNames, trimmed)
	}

	return stackNames, nil
}

func templateOutput(cfnClient awscloudformation.Client, acct ymlparser.Account, stack ymlparser.Stack) (*template.CDKOutputs, error) {
	var localTemplate template.CDKOutputs

	tmpPath := TmpPath(acct, stack.Path)
	var stackPathPrefix string
	if stack.Path != "" {
		stackPathPrefix += fmt.Sprintf("%s/", stack.Path)
	}
	rawFile, err := os.ReadFile(fmt.Sprintf("%s%s/%s.template.json", stackPathPrefix, tmpPath, stack.Name))
	if err != nil {
		return nil, oops.Wrapf(err, "read template")
	}

	err = json.Unmarshal(rawFile, &localTemplate)
	if err != nil {
		return nil, oops.Wrapf(err, "unmarshal template")
	}

	// We have to manually set the name because its not in the stack template.
	localTemplate.StackName = stack.Name

	return &localTemplate, nil
}

func StackLocalOutput(cfnClient awscloudformation.Client, acct ymlparser.Account, stack ymlparser.Stack) ([]*template.CDKOutputs, error) {
	var stackTemplateOutputs []*template.CDKOutputs

	if stack.Name == "" || stack.Name == "*" {
		names, err := getStackNames(acct, stack)
		if err != nil {
			return nil, oops.Wrapf(err, "getting stack names")
		}
		for _, stackName := range names {
			if stackName == "" {
				continue
			}
			specificStack := ymlparser.Stack{
				Path:            stack.Path,
				RoleOverrideARN: stack.RoleOverrideARN,
				Type:            stack.Type,
				Name:            stackName,
			}
			output, err := templateOutput(cfnClient, acct, specificStack)
			if err != nil {
				return nil, oops.Wrapf(err, "error getting local outputs from stack: %s", stackName)
			}
			stackTemplateOutputs = append(stackTemplateOutputs, output)
		}
	} else {
		output, err := templateOutput(cfnClient, acct, stack)
		if err != nil {
			return nil, oops.Wrapf(err, "error getting templateOutput")
		}
		stackTemplateOutputs = append(stackTemplateOutputs, output)
	}

	return stackTemplateOutputs, nil
}

func StackRemoteOutput(cfnClient awscloudformation.Client, acct ymlparser.Account, stack ymlparser.Stack) ([]*template.CDKOutputs, error) {
	localOutputs, err := StackLocalOutput(cfnClient, acct, stack)
	if err != nil {
		return []*template.CDKOutputs{}, oops.Wrapf(err, "error getting local outputs")
	}

	// Mark deployed outputs that have changed
	deployedTemplateOutputs, err := cfnClient.FetchTemplateOutputs(context.TODO(), stack.Name)
	if err != nil {
		return nil, oops.Wrapf(err, "FetchTemplateOutputs stack (%s)", stack.Name)
	}

	for idx, output := range localOutputs {
		templateOutputDiff := deployedTemplateOutputs.Diff(output)

		outputVals, err := cfnClient.FetchStackOutputs(context.TODO(), output.StackName)
		if err != nil {
			return []*template.CDKOutputs{}, oops.Wrapf(err, "getting output: %s", output.StackName)
		}

		for _, outValue := range outputVals {
			for varName := range output.Outputs {
				if outValue["OutputKey"] == varName {
					if updatedEval, ok := templateOutputDiff[template.Updated][varName]; ok {
						localOutputs[idx].Outputs[varName] = updatedEval.(map[string]interface{})
					} else {
						localOutputs[idx].Outputs[varName]["Value"] = outValue["OutputValue"]
					}
				}
			}
		}
	}

	return localOutputs, nil
}

func TmpPath(acct ymlparser.Account, filePath string) string {
	hasher := sha256.New()
	hasher.Write([]byte(filePath))
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return path.Join("telophasedirs", fmt.Sprintf("cdk-tmp%s-%s", acct.AccountID, hashString))
}
