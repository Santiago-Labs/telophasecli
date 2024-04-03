package resource

type Resource interface {
	ID() string
	Name() string
	Type() string
}
