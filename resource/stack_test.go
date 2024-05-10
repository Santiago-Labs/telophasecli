package resource

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewForRegion ensures NewForRegion has an exact replica of the Stack type
// when creating a new Stack. This makes sure that we don't add a field to the
// Stack struct that doesn't get copied over.
func TestNewForRegion(t *testing.T) {
	newStack := Stack{}
	setFields(&newStack)
	newStack.Region = "us-west-2"

	valS := reflect.ValueOf(&newStack).Elem()
	valNewS := reflect.ValueOf(&newStack).Elem()

	for i := 0; i < valS.NumField(); i++ {
		valNewS.Field(i).Set(valS.Field(i))
	}

	newStackSameRegion := newStack.NewForRegion("us-west-2")

	assert.True(t, reflect.DeepEqual(newStack, newStackSameRegion), "you likely added something to the Stack struct wihtout adding it to the stack in NewForRegion")
}

func setFields(s *Stack) {
	v := reflect.ValueOf(s).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.String:
			// all strings set as an example value
			field.SetString("example value for " + v.Type().Field(i).Name)
		case reflect.Int:
			// all ints are set to 8
			field.SetInt(8)
		}
	}
}

func TestCloudformationStackName(t *testing.T) {
	tests := []struct {
		input Stack

		want string
	}{
		{
			input: Stack{
				Name:   "name",
				Path:   "path",
				Region: "us-west-2",
			},
			want: "name-path-us-west-2",
		},
		{
			input: Stack{
				Path:   "./path",
				Region: "us-west-2",
			},
			want: "path-us-west-2",
		},
		{
			input: Stack{
				Name:   "@danny",
				Path:   "./path",
				Region: "us-west-2",
			},
			want: "danny---path-us-west-2",
		},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, *tc.input.CloudformationStackName())
	}

}
