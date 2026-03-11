package containerpicker

import (
	"testing"

	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleContainers() []k8s.ContainerDetail {
	return []k8s.ContainerDetail{
		{Name: "main", Image: "nginx:1.25"},
		{Name: "sidecar", Image: "busybox:1.36"},
	}
}

func TestSelected(t *testing.T) {
	m := New().SetContainers(sampleContainers())
	selected := m.Selected()
	require.NotNil(t, selected)
	assert.Equal(t, "main", selected.Name)
}

func TestMoveDownAndUp(t *testing.T) {
	m := New().SetContainers(sampleContainers())
	m = m.MoveDown()
	selected := m.Selected()
	require.NotNil(t, selected)
	assert.Equal(t, "sidecar", selected.Name)

	m = m.MoveUp()
	selected = m.Selected()
	require.NotNil(t, selected)
	assert.Equal(t, "main", selected.Name)
}

func TestView(t *testing.T) {
	m := New().SetContainers(sampleContainers())
	view := m.View()
	assert.Contains(t, view, "Select Container Shell Target")
	assert.Contains(t, view, "main")
	assert.Contains(t, view, "nginx:1.25")
	assert.Contains(t, view, "enter select")
}
