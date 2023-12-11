package template

import "encoding/json"

const (
	Created = 1
	Updated = 2
	Deleted = 3
)

type CDKOutputs struct {
	StackName string                            `yaml:"StackName" json:"StackName"`
	Outputs   map[string]map[string]interface{} `yaml:"Outputs" json:"Outputs"`
}

func (c *CDKOutputs) Diff(other *CDKOutputs) map[int]map[string]interface{} {
	diff := make(map[int]map[string]interface{})
	diff[Created] = make(map[string]interface{})
	diff[Updated] = make(map[string]interface{})
	diff[Deleted] = make(map[string]interface{})

	for key := range c.Outputs {
		otherVal, ok := other.Outputs[key]
		if !ok {
			diff[Deleted][key] = c.Outputs[key]
		} else {
			otherValueSer, err := json.Marshal(otherVal["Value"])
			if err != nil {
				panic(err)
			}

			cValueSer, err := json.Marshal(c.Outputs[key]["Value"])
			if err != nil {
				panic(err)
			}
			if string(otherValueSer) != string(cValueSer) {
				diff[Updated][key] = otherVal
			}
		}
	}

	for key := range other.Outputs {
		_, ok := c.Outputs[key]
		if !ok {
			diff[Created][key] = other.Outputs[key]
		}
	}

	return diff
}
