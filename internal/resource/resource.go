package resource

import "context"

// ResourceItem is the interface for any displayable Kubernetes resource.
type ResourceItem interface {
	GetName() string
	GetNamespace() string
	GetStatus() string
	IsDirty() bool
}

// DeleteResult holds the outcome of deleting a single resource.
type DeleteResult struct {
	Name      string
	Namespace string
	Success   bool
	Error     error
}

// Resource is the interface for a Kubernetes resource type that can be listed and deleted.
type Resource interface {
	Type() string
	List(ctx context.Context, namespace string) ([]ResourceItem, error)
	Delete(ctx context.Context, items []ResourceItem) ([]DeleteResult, error)
}
