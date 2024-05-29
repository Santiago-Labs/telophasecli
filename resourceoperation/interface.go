package resourceoperation

import (
	"context"
)

const (
	// Accounts
	UpdateParent = 1
	Create       = 2
	Update       = 3
	UpdateTags   = 6
	Delete       = 7

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
