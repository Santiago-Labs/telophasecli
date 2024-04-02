package resource

type Stack struct {
	Name            string `yaml:"Name"`
	Type            string `yaml:"Type"`
	Path            string `yaml:"Path"`
	RoleOverrideARN string `yaml:"RoleOverrideARN,omitempty"`
}
