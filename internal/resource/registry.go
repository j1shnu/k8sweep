package resource

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds registered resource types. It is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	resources map[string]Resource
}

// NewRegistry creates an empty resource registry.
func NewRegistry() *Registry {
	return &Registry{
		resources: make(map[string]Resource),
	}
}

// Register adds a resource type to the registry.
func (r *Registry) Register(res Resource) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := res.Type()
	if _, exists := r.resources[name]; exists {
		return fmt.Errorf("resource type %q already registered", name)
	}
	r.resources[name] = res
	return nil
}

// Get returns a resource by type name.
func (r *Registry) Get(typeName string) (Resource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	res, ok := r.resources[typeName]
	return res, ok
}

// ListTypes returns all registered resource type names.
func (r *Registry) ListTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.resources))
	for k := range r.resources {
		types = append(types, k)
	}
	sort.Strings(types)
	return types
}
