package resourceoperation

import (
	"context"

	"github.com/santiago-labs/telophasecli/lib/awsorgs"
)

type ResourceOperation interface {
	Call(context.Context, awsorgs.Client) error
	ToString() string
	AddDependent(ResourceOperation)
	ListDependents() []ResourceOperation
}