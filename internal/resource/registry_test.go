package resource

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockResource struct {
	typeName string
}

func (m mockResource) Type() string { return m.typeName }
func (m mockResource) List(_ context.Context, _ string) ([]ResourceItem, error) {
	return nil, nil
}
func (m mockResource) Delete(_ context.Context, _ []ResourceItem) ([]DeleteResult, error) {
	return nil, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	err := reg.Register(mockResource{typeName: "pods"})
	require.NoError(t, err)

	res, ok := reg.Get("pods")
	assert.True(t, ok)
	assert.Equal(t, "pods", res.Type())
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := NewRegistry()
	_, ok := reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	reg := NewRegistry()
	err := reg.Register(mockResource{typeName: "pods"})
	require.NoError(t, err)

	err = reg.Register(mockResource{typeName: "pods"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_ListTypes(t *testing.T) {
	reg := NewRegistry()
	_ = reg.Register(mockResource{typeName: "pods"})
	_ = reg.Register(mockResource{typeName: "jobs"})

	types := reg.ListTypes()
	assert.Len(t, types, 2)
	assert.Contains(t, types, "pods")
	assert.Contains(t, types, "jobs")
}

func TestRegistry_Empty(t *testing.T) {
	reg := NewRegistry()
	assert.Empty(t, reg.ListTypes())
}
