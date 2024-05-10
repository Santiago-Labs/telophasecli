package resource

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/samsarahq/go/oops"
)

type Stack struct {
	// When adding a new type to the struct, make sure you add it to the `NewForRegion` method.
	Name                      string `yaml:"Name"`
	Type                      string `yaml:"Type"`
	Path                      string `yaml:"Path"`
	Region                    string `yaml:"Region,omitempty"`
	RoleOverrideARNDeprecated string `yaml:"RoleOverrideARN,omitempty"` // Deprecated
	AssumeRoleName            string `yaml:"AssumeRoleName,omitempty"`
	Workspace                 string `yaml:"Workspace,omitempty"`

	CloudformationParameters []string `yaml:"CloudformationParameters,omitempty"`
}

func (s Stack) NewForRegion(region string) Stack {
	return Stack{
		Name:                      s.Name,
		Type:                      s.Type,
		Path:                      s.Path,
		Region:                    region,
		RoleOverrideARNDeprecated: s.RoleOverrideARNDeprecated,
		AssumeRoleName:            s.AssumeRoleName,
		Workspace:                 s.Workspace,
		CloudformationParameters:  s.CloudformationParameters,
	}
}

func (s Stack) RoleARN(acct Account) *string {
	if s.AssumeRoleName != "" {
		result := fmt.Sprintf("arn:aws:iam::%s:role/%s", acct.AccountID, s.AssumeRoleName)
		return &result
	}

	acctRole := acct.AssumeRoleARN()
	return &acctRole
}

func (s Stack) AWSRegionEnv() *string {
	if s.Region != "" {
		v := "AWS_REGION=" + s.Region
		return &v
	}
	return nil
}

func (s Stack) WorkspaceEnabled() bool {
	return s.Workspace != ""
}

func (s Stack) Validate() error {
	switch os := s.Type; os {
	case "Terraform":
		return nil

	case "CDK":
		if s.Workspace != "" {
			return oops.Errorf("Workspace: (%s) should not be set for CDK stack", s.Workspace)
		}
		return nil

	case "Cloudformation":
		if _, err := s.CloudformationParametersType(); err != nil {
			return oops.Wrapf(err, "")
		}
		return nil

	case "":
		return oops.Errorf("stack type needs to be set for stack: %+v", s)

	default:
		return oops.Errorf("only support stack types of `Terraform` and `CDK` not: %s", s.Type)
	}
}

func (s Stack) CloudformationParametersType() ([]*cloudformation.Parameter, error) {
	var params []*cloudformation.Parameter
	for _, param := range s.CloudformationParameters {
		parts := strings.Split(param, "=")
		if len(parts) != 2 {
			return nil, oops.Errorf("cloudformation parameter (%s) should be = delimited and have 2 parts it has %d parts", param, len(parts))
		}

		params = append(params, &cloudformation.Parameter{
			ParameterKey:   aws.String(parts[0]),
			ParameterValue: aws.String(parts[1]),
		})
	}

	return params, nil
}

// CloudformationStackName returns the corresponding stack name to create in cloudformation.
//
// The name needs to match [a-zA-Z][-a-zA-Z0-9]*|arn:[-a-zA-Z0-9:/._+]*
func (s Stack) CloudformationStackName() *string {
	// Replace:
	// - "/" with "-", "/" appears in the path
	// - "." with "-", "." shows up with "".yml" or ".json"
	name := strings.ReplaceAll(strings.ReplaceAll(s.Path, "/", "-"), ".", "-")
	if s.Name != "" {
		name = strings.ReplaceAll(s.Name, " ", "-") + "-" + name
	}
	if s.Region != "" {
		name = name + "-" + s.Region
	}

	// Stack name needs to start with [a-zA-Z]
	// Remove leading characters that are not alphabetic
	firstAlphaRegex := regexp.MustCompile(`^[^a-zA-Z]*`)
	name = firstAlphaRegex.ReplaceAllString(name, "")

	// Ensure the first character is alphabetic (already guaranteed by the previous step)
	// Remove or replace all characters that do not match [-a-zA-Z0-9]
	validCharsRegex := regexp.MustCompile(`[^-a-zA-Z0-9]+`)
	name = validCharsRegex.ReplaceAllString(name, "-")

	return &name
}

func (s Stack) ChangeSetName() *string {
	changeSetName := fmt.Sprintf("telophase-%s-%d", *s.CloudformationStackName(), time.Now().Unix())
	return &changeSetName
}
