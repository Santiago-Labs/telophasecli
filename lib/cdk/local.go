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

	"github.com/santiago-labs/telophasecli/lib/awscloudformation"
	"github.com/santiago-labs/telophasecli/lib/cdk/template"
	"github.com/santiago-labs/telophasecli/lib/localstack"
	"github.com/santiago-labs/telophasecli/lib/ymlparser"
)

func getStackNames(stack ymlparser.Stack) ([]string, error) {
	cmd := exec.Command(localstack.CdkCmd(), "ls")
	output, err := cmd.Output()
	if err != nil {
		return []string{}, err
	}

	outputStr := string(output)
	return strings.Split(outputStr, "\n"), nil
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
		return nil, err
	}

	err = json.Unmarshal(rawFile, &localTemplate)
	if err != nil {
		return nil, err
	}

	// We have to manually set the name because its not in the stack template.
	localTemplate.StackName = stack.Name

	return &localTemplate, nil
}

func StackLocalOutput(cfnClient awscloudformation.Client, acct ymlparser.Account, stack ymlparser.Stack) ([]*template.CDKOutputs, error) {
	var stackTemplateOutputs []*template.CDKOutputs

	if stack.Name == "" || stack.Name == "*" {
		names, err := getStackNames(stack)
		if err != nil {
			return nil, err
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
				return nil, err
			}
			stackTemplateOutputs = append(stackTemplateOutputs, output)
		}
	} else {
		output, err := templateOutput(cfnClient, acct, stack)
		if err != nil {
			return nil, err
		}
		stackTemplateOutputs = append(stackTemplateOutputs, output)
	}

	return stackTemplateOutputs, nil
}

func StackRemoteOutput(cfnClient awscloudformation.Client, acct ymlparser.Account, stack ymlparser.Stack) ([]*template.CDKOutputs, error) {
	localOutputs, err := StackLocalOutput(cfnClient, acct, stack)
	if err != nil {
		return []*template.CDKOutputs{}, err
	}

	// Mark deployed outputs that have changed
	deployedTemplateOutputs, err := cfnClient.FetchTemplateOutputs(context.TODO(), stack.Name)
	if err != nil {
		return nil, err
	}

	for idx, output := range localOutputs {
		templateOutputDiff := deployedTemplateOutputs.Diff(output)

		outputVals, err := cfnClient.FetchStackOutputs(context.TODO(), output.StackName)
		if err != nil {
			return []*template.CDKOutputs{}, err
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
