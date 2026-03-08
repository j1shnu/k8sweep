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

func TestView_Loading(t *testing.T) {
	m := New()
	view := m.View()
	assert.Contains(t, view, "Fetching pods...")
}

func TestView_Empty(t *testing.T) {
	m := New().SetItems(nil) // loading done, no pods
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

func TestShowNamespace_Preserved(t *testing.T) {
	m := New().SetShowNamespace(true).SetItems(samplePods())
	assert.True(t, m.showNamespace)

	// Verify preserved through all operations
	m = m.MoveDown()
	assert.True(t, m.showNamespace)
	m = m.MoveUp()
	assert.True(t, m.showNamespace)
	m = m.ToggleSelect()
	assert.True(t, m.showNamespace)
	m = m.SelectAll()
	assert.True(t, m.showNamespace)
	m = m.DeselectAll()
	assert.True(t, m.showNamespace)
	m = m.SetSize(120, 10)
	assert.True(t, m.showNamespace)
	m = m.SetLoading()
	assert.True(t, m.showNamespace)
}

func TestView_WithNamespaceColumn(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "pod-1", Namespace: "kube-system", Status: k8s.StatusRunning},
		{Name: "pod-2", Namespace: "default", Status: k8s.StatusFailed},
	}
	m := New().SetShowNamespace(true).SetItems(pods).SetSize(150, 10)
	view := m.View()
	assert.Contains(t, view, "kube-system")
	assert.Contains(t, view, "default")
	assert.Contains(t, view, "pod-1")
	assert.Contains(t, view, "pod-2")
}

func TestView_WithoutNamespaceColumn(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "pod-1", Namespace: "kube-system", Status: k8s.StatusRunning},
	}
	m := New().SetItems(pods).SetSize(120, 10)
	view := m.View()
	assert.Contains(t, view, "pod-1")
	assert.NotContains(t, view, "kube-system")
}

func TestLoadingPreservedAcrossOperations(t *testing.T) {
	m := New() // loading=true by default
	m = m.ToggleSelect()
	assert.True(t, m.loading)
	m = m.SelectAll()
	assert.True(t, m.loading)
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0m"},
		{1, "1m"},
		{250, "250m"},
		{999, "999m"},
		{1000, "1"},
		{1500, "1.5"},
		{2000, "2"},
		{2345, "2.3"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatCPU(tt.input), "formatCPU(%d)", tt.input)
	}
}

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1Ki"},
		{1024 * 1024, "1Mi"},
		{128 * 1024 * 1024, "128Mi"},
		{512 * 1024 * 1024, "512Mi"},
		{1024 * 1024 * 1024, "1Gi"},
		{int64(1.5 * 1024 * 1024 * 1024), "1.5Gi"},
		{int64(2 * 1024 * 1024 * 1024), "2Gi"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatMemory(tt.input), "formatMemory(%d)", tt.input)
	}
}

func TestMetricsAvailable_Preserved(t *testing.T) {
	m := New().SetMetricsAvailable(true).SetItems(samplePods())
	assert.True(t, m.metricsAvailable)

	m = m.MoveDown()
	assert.True(t, m.metricsAvailable)
	m = m.ToggleSelect()
	assert.True(t, m.metricsAvailable)
	m = m.SelectAll()
	assert.True(t, m.metricsAvailable)
	m = m.DeselectAll()
	assert.True(t, m.metricsAvailable)
	m = m.SetSize(120, 10)
	assert.True(t, m.metricsAvailable)
	m = m.SetLoading()
	assert.True(t, m.metricsAvailable)
}

func TestView_WithMetrics(t *testing.T) {
	pods := []k8s.PodInfo{
		{
			Name: "pod-1", Namespace: "default", Status: k8s.StatusRunning,
			Metrics: &k8s.PodMetrics{CPUMillicores: 250, MemoryBytes: 128 * 1024 * 1024},
		},
		{
			Name: "pod-2", Namespace: "default", Status: k8s.StatusFailed,
		},
	}
	m := New().SetMetricsAvailable(true).SetItems(pods).SetSize(150, 10)
	view := m.View()
	assert.Contains(t, view, "cpu:")
	assert.Contains(t, view, "mem:")
	assert.Contains(t, view, "250m")
	assert.Contains(t, view, "128Mi")
	assert.Contains(t, view, "---")
}

func TestView_WithoutMetrics(t *testing.T) {
	m := New().SetItems(samplePods()).SetSize(120, 10)
	view := m.View()
	assert.NotContains(t, view, "cpu:")
	assert.NotContains(t, view, "mem:")
}
