package resource

import (
	"github.com/samsarahq/go/oops"
)

type Stack struct {
	Name            string `yaml:"Name"`
	Type            string `yaml:"Type"`
	Path            string `yaml:"Path"`
	Region          string `yaml:"Region,omitempty"`
	RoleOverrideARN string `yaml:"RoleOverrideARN,omitempty"`
	Workspace       string `yaml:"Workspace,omitempty"`
}

func (s Stack) NewForRegion(region string) Stack {
	return Stack{
		Name:            s.Name,
		Type:            s.Type,
		Path:            s.Path,
		Region:          region,
		RoleOverrideARN: s.RoleOverrideARN,
		Workspace:       s.Workspace,
	}
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

	case "":
		return oops.Errorf("stack type needs to be set for stack: %+v", s)

	default:
		return oops.Errorf("only support stack types of `Terraform` and `CDK` not: %s", s.Type)
	}
}
