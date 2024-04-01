package resourceoperation

import (
	"context"
)

const (
	// Accounts
	UpdateParent = 1
	Create       = 2
	Update       = 3

	// IaC
	Diff   = 4
	Deploy = 5
)

type ResourceOperation interface {
	Call(context.Context) error
	ToString() string
	AddDependent(ResourceOperation)
	ListDependents() []ResourceOperation
}
