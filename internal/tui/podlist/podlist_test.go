package podlist

import (
	"testing"

	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/stretchr/testify/assert"
)

func samplePods() []k8s.PodInfo {
	return []k8s.PodInfo{
		{Name: "pod-1", Namespace: "default", Status: k8s.StatusRunning},
		{Name: "pod-2", Namespace: "default", Status: k8s.StatusFailed},
		{Name: "pod-3", Namespace: "default", Status: k8s.StatusCompleted},
		{Name: "pod-4", Namespace: "default", Status: k8s.StatusEvicted},
		{Name: "pod-5", Namespace: "default", Status: k8s.StatusCrashLoopBack},
	}
}

func TestNew(t *testing.T) {
	m := New()
	assert.Equal(t, 0, m.Len())
	assert.Equal(t, 0, m.SelectedCount())
}

func TestSetItems(t *testing.T) {
	m := New().SetItems(samplePods())
	assert.Equal(t, 5, m.Len())
	assert.Equal(t, 0, m.SelectedCount())
}

func TestMoveDown(t *testing.T) {
	m := New().SetItems(samplePods())
	m = m.MoveDown()
	assert.Equal(t, 1, m.cursor)
	m = m.MoveDown()
	assert.Equal(t, 2, m.cursor)
}

func TestMoveDown_AtEnd(t *testing.T) {
	m := New().SetItems(samplePods())
	for i := 0; i < 10; i++ {
		m = m.MoveDown()
	}
	assert.Equal(t, 4, m.cursor) // clamped to last item
}

func TestMoveUp(t *testing.T) {
	m := New().SetItems(samplePods())
	m = m.MoveDown().MoveDown().MoveUp()
	assert.Equal(t, 1, m.cursor)
}

func TestMoveUp_AtStart(t *testing.T) {
	m := New().SetItems(samplePods())
	m = m.MoveUp()
	assert.Equal(t, 0, m.cursor)
}

func TestToggleSelect(t *testing.T) {
	m := New().SetItems(samplePods())
	m = m.ToggleSelect() // select pod-1
	assert.Equal(t, 1, m.SelectedCount())

	m = m.ToggleSelect() // deselect pod-1
	assert.Equal(t, 0, m.SelectedCount())
}

func TestSelectAll(t *testing.T) {
	m := New().SetItems(samplePods())
	m = m.SelectAll()
	assert.Equal(t, 5, m.SelectedCount())
}

func TestDeselectAll(t *testing.T) {
	m := New().SetItems(samplePods()).SelectAll().DeselectAll()
	assert.Equal(t, 0, m.SelectedCount())
}

func TestGetSelected(t *testing.T) {
	m := New().SetItems(samplePods())
	m = m.ToggleSelect() // pod-1
	m = m.MoveDown().MoveDown().ToggleSelect() // pod-3

	selected := m.GetSelected()
	assert.Len(t, selected, 2)

	names := make([]string, len(selected))
	for i, p := range selected {
		names[i] = p.Name
	}
	assert.Contains(t, names, "pod-1")
	assert.Contains(t, names, "pod-3")
}

func TestEmptyList_Operations(t *testing.T) {
	m := New()
	m = m.MoveUp()
	assert.Equal(t, 0, m.cursor)
	m = m.MoveDown()
	assert.Equal(t, 0, m.cursor)
	m = m.ToggleSelect()
	assert.Equal(t, 0, m.SelectedCount())
}

func TestView_Empty(t *testing.T) {
	m := New()
	view := m.View()
	assert.Contains(t, view, "No pods found")
}

func TestView_WithPods(t *testing.T) {
	m := New().SetItems(samplePods()).SetSize(120, 10)
	view := m.View()
	assert.Contains(t, view, "pod-1")
	assert.Contains(t, view, "Running")
	assert.Contains(t, view, "Failed")
}

func TestScrolling(t *testing.T) {
	m := New().SetItems(samplePods()).SetSize(120, 3) // only 3 visible rows
	// Move to last item
	for i := 0; i < 4; i++ {
		m = m.MoveDown()
	}
	assert.Equal(t, 4, m.cursor)
	assert.Equal(t, 2, m.offset) // offset should adjust

	// Move back to top
	for i := 0; i < 4; i++ {
		m = m.MoveUp()
	}
	assert.Equal(t, 0, m.cursor)
	assert.Equal(t, 0, m.offset)
}

func TestLen(t *testing.T) {
	m := New()
	assert.Equal(t, 0, m.Len())
	m = m.SetItems(samplePods())
	assert.Equal(t, 5, m.Len())
}
