package resource

type Stack struct {
	Name            string `yaml:"Name"`
	Type            string `yaml:"Type"`
	Path            string `yaml:"Path"`
	Region          string `yaml:"Region,omitempty"`
	RoleOverrideARN string `yaml:"RoleOverrideARN,omitempty"`
}

func (s Stack) AWSRegionEnv() *string {
	if s.Region != "" {
		v := "AWS_REGION=" + s.Region
		return &v
	}
	return nil
}
