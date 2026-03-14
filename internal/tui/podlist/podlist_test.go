package podlist

import (
	"fmt"
	"strings"
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

func manyPods(n int) []k8s.PodInfo {
	pods := make([]k8s.PodInfo, 0, n)
	for i := 1; i <= n; i++ {
		pods = append(pods, k8s.PodInfo{
			Name:      fmt.Sprintf("pod-%02d", i),
			Namespace: "default",
			Status:    k8s.StatusRunning,
		})
	}
	return pods
}

func TestNew(t *testing.T) {
	m := New()
	assert.Equal(t, 0, m.Len())
	assert.Equal(t, 0, m.SelectedCount())
}

func TestSetItems(t *testing.T) {
	m := New().SetItems(samplePods())
	assert.Equal(t, 5, m.PodCount())
	// Len includes the Standalone group header
	assert.Equal(t, 6, m.Len())
	assert.Equal(t, 0, m.SelectedCount())
}

func TestMoveDown(t *testing.T) {
	m := New().SetItems(samplePods())
	// cursor starts at 1 (first pod; row 0 is Standalone header)
	m = m.MoveDown()
	assert.Equal(t, 2, m.cursor)
	m = m.MoveDown()
	assert.Equal(t, 3, m.cursor)
}

func TestMoveDown_AtEnd(t *testing.T) {
	m := New().SetItems(samplePods())
	for i := 0; i < 10; i++ {
		m = m.MoveDown()
	}
	// 6 display rows (1 header + 5 pods), clamped to last = 5
	assert.Equal(t, 5, m.cursor)
}

func TestMoveUp(t *testing.T) {
	m := New().SetItems(samplePods())
	m = m.MoveDown().MoveDown().MoveUp()
	// 1→2→3→2
	assert.Equal(t, 2, m.cursor)
}

func TestMoveUp_AtStart(t *testing.T) {
	m := New().SetItems(samplePods())
	m = m.MoveUp()
	// Cursor starts at 1 (first pod), clamped at page start (0)
	assert.Equal(t, 0, m.cursor)
}

func TestToggleSelect(t *testing.T) {
	m := New().SetItems(samplePods())
	m = m.ToggleSelect() // select pod-1 (cursor at first pod)
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
	m = m.ToggleSelect()                       // pod-1 (cursor at 1)
	m = m.MoveDown().MoveDown().ToggleSelect() // pod-3 (cursor at 3)

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

func TestRowNavigation_ClampedWithinPage(t *testing.T) {
	m := New().SetItems(manyPods(30)).SetSize(120, 10) // page size = 8

	for i := 0; i < 20; i++ {
		m = m.MoveDown()
	}
	assert.Equal(t, 7, m.cursor) // last row of page 0
	assert.Equal(t, 0, m.offset)

	m = m.MoveUp()
	assert.Equal(t, 6, m.cursor)
	assert.Equal(t, 0, m.offset)
}

func TestLen(t *testing.T) {
	m := New()
	assert.Equal(t, 0, m.Len())
	m = m.SetItems(samplePods())
	// Len returns display row count (1 header + 5 pods)
	assert.Equal(t, 6, m.Len())
	assert.Equal(t, 5, m.PodCount())
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
	// Namespace may appear in controller header but not in pod data columns
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

func TestSortPreservedAcrossOperations(t *testing.T) {
	m := New().SetItems(samplePods()).SetSort(SortByStatus, SortDesc)
	assert.Equal(t, SortByStatus, m.sortColumn)
	assert.Equal(t, SortDesc, m.sortOrder)

	m = m.MoveDown()
	assert.Equal(t, SortByStatus, m.sortColumn)
	m = m.ToggleSelect()
	assert.Equal(t, SortByStatus, m.sortColumn)
	m = m.SelectAll()
	assert.Equal(t, SortByStatus, m.sortColumn)
	m = m.DeselectAll()
	assert.Equal(t, SortByStatus, m.sortColumn)
	m = m.SetSize(120, 10)
	assert.Equal(t, SortByStatus, m.sortColumn)
	m = m.SetLoading()
	assert.Equal(t, SortByStatus, m.sortColumn)
}

func TestSetSort_CursorTracking(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "alpha", Namespace: "default", Status: k8s.StatusRunning},
		{Name: "bravo", Namespace: "default", Status: k8s.StatusFailed},
		{Name: "charlie", Namespace: "default", Status: k8s.StatusPending},
	}
	m := New().SetItems(pods).SetSize(120, 20)

	// cursor starts at 1 (first pod = alpha); move to bravo (index 2)
	m = m.MoveDown()
	assert.Equal(t, 2, m.cursor)

	// Sort by name desc → charlie, bravo, alpha (all under Standalone)
	m = m.SetSort(SortByName, SortDesc)
	// Cursor should follow "bravo" to its new position (row 2: header=0, charlie=1, bravo=2)
	assert.Equal(t, 2, m.cursor)
	p := m.CursorItem()
	assert.NotNil(t, p)
	assert.Equal(t, "bravo", p.Name)
}

func TestSetSort_PreservesSelection(t *testing.T) {
	pods := samplePods()
	m := New().SetItems(pods).SetSize(120, 20)
	m = m.ToggleSelect() // select pod-1
	assert.Equal(t, 1, m.SelectedCount())

	m = m.SetSort(SortByStatus, SortDesc)
	assert.Equal(t, 1, m.SelectedCount())
}

func TestSetItemsSorted_AppliesCurrentSort(t *testing.T) {
	m := New().SetSort(SortByName, SortDesc)

	pods := []k8s.PodInfo{
		{Name: "alpha", Namespace: "default"},
		{Name: "charlie", Namespace: "default"},
		{Name: "bravo", Namespace: "default"},
	}
	m = m.SetItemsSorted(pods)

	p := m.CursorItem()
	assert.NotNil(t, p)
	// desc order → charlie first pod row (after Standalone header)
	assert.Equal(t, "charlie", p.Name)
}

func TestView_HeaderRow(t *testing.T) {
	m := New().SetItems(samplePods()).SetSize(120, 10)
	view := m.View()
	assert.Contains(t, view, "NAME")
	assert.Contains(t, view, "STATUS")
	assert.Contains(t, view, "AGE")
	assert.Contains(t, view, "RESTARTS")
}

func TestView_HeaderRow_SortIndicator(t *testing.T) {
	m := New().SetItems(samplePods()).SetSize(120, 10).SetSort(SortByStatus, SortDesc)
	view := m.View()
	assert.Contains(t, view, "STATUS ▼")
}

func TestCursorItem_EmptyList(t *testing.T) {
	m := New().SetItems(nil)
	assert.Nil(t, m.CursorItem())
}

func TestCursorItem_WithPods(t *testing.T) {
	m := New().SetItems(samplePods())
	p := m.CursorItem()
	assert.NotNil(t, p)
	assert.Equal(t, "pod-1", p.Name)
}

func TestPageDown_PageUp(t *testing.T) {
	m := New().SetItems(manyPods(30)).SetSize(120, 10)

	// cursor starts at 1 (first pod). PageDown preserves relative row.
	m = m.PageDown()
	assert.Equal(t, 9, m.cursor)
	assert.Equal(t, 8, m.offset)
	p := m.CursorItem()
	assert.NotNil(t, p)
	assert.Equal(t, "pod-09", p.Name)

	m = m.PageUp()
	assert.Equal(t, 1, m.cursor)
	assert.Equal(t, 0, m.offset)
}

func TestPageDown_ClampedAtEnd(t *testing.T) {
	m := New().SetItems(manyPods(12)).SetSize(120, 10)

	for i := 0; i < 5; i++ {
		m = m.PageDown()
	}

	assert.Equal(t, 9, m.cursor)
	assert.Equal(t, 8, m.offset)
	p := m.CursorItem()
	assert.NotNil(t, p)
	assert.Equal(t, "pod-09", p.Name)
}

func TestPageUp_ClampedAtStart(t *testing.T) {
	m := New().SetItems(manyPods(30)).SetSize(120, 10)
	m = m.GoBottom()
	m = m.PageUp()
	m = m.PageUp()
	m = m.PageUp()
	m = m.PageUp()
	m = m.PageUp()

	assert.Equal(t, 0, m.offset)
	assert.Equal(t, 6, m.cursor)
}

func TestView_ShowsPaginationFooter(t *testing.T) {
	m := New().SetItems(manyPods(100)).SetSize(120, 10)

	view := m.View()
	// 101 display rows = 1 Standalone header + 100 pods
	assert.Contains(t, view, "Showing 1-8 of 101 rows")
	assert.Contains(t, view, "page 1/13")
	assert.Contains(t, view, "[l]/[→] next | [h]/[←] previous")

	m = m.PageDown()
	view = m.View()
	assert.Contains(t, view, "Showing 9-16 of 101 rows")
	assert.Contains(t, view, "page 2/13")
}

func TestView_HidesPaginationFooter_ForSinglePage(t *testing.T) {
	m := New().SetItems(manyPods(2)).SetSize(120, 10)
	view := m.View()
	assert.NotContains(t, view, "Showing")
	assert.NotContains(t, view, "[l]/[→] next | [h]/[←] previous")
}

func TestSmartTruncateMiddle(t *testing.T) {
	in := "checkout-api-worker-payments-us-east-1-canary-76f8dfc4f8-r49xm"
	got := smartTruncateMiddle(in, 18)
	assert.Equal(t, 18, len([]rune(got)))
	assert.Contains(t, got, "...")
	assert.True(t, strings.HasPrefix(got, "checkout"))
	assert.True(t, strings.HasSuffix(got, "r49xm"))
}

func TestSmartTruncateMiddle_ShortValueUnchanged(t *testing.T) {
	in := "short-pod"
	assert.Equal(t, in, smartTruncateMiddle(in, 45))
}

func TestView_LongNamesRemainDistinguishable(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "checkout-api-worker-payments-us-east-1-canary-76f8dfc4f8-r49xm", Namespace: "default", Status: k8s.StatusRunning},
		{Name: "checkout-api-worker-payments-us-east-1-canary-76f8dfc4f8-v8n2q", Namespace: "default", Status: k8s.StatusRunning},
	}

	m := New().SetItems(pods).SetSize(120, 10)
	view := m.View()

	assert.Contains(t, view, "checkout-api-worker")
	assert.Contains(t, view, "...")
	assert.Contains(t, view, "4f8-r49xm")
	assert.Contains(t, view, "4f8-v8n2q")
}

// --- Tree-specific tests ---

func TestToggleCollapse(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "nginx-1", Namespace: "default", Status: k8s.StatusRunning,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "nginx"}},
		{Name: "nginx-2", Namespace: "default", Status: k8s.StatusRunning,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "nginx"}},
	}
	m := New().SetItems(pods).SetSize(120, 20)
	// 3 rows: header + 2 pods
	assert.Equal(t, 3, m.Len())

	// Move to header (row 0) and collapse
	m = m.GoTop()
	m = m.ToggleCollapse()
	assert.Equal(t, 1, m.Len()) // only header

	// Expand again
	m = m.ToggleCollapse()
	assert.Equal(t, 3, m.Len())
}

func TestCollapseAll_ExpandAll(t *testing.T) {
	pods := treeTestPods()
	m := New().SetItems(pods).SetSize(120, 30)
	// 4 groups + 5 pods = 9 rows
	assert.Equal(t, 9, m.Len())

	m = m.CollapseAll()
	assert.Equal(t, 4, m.Len()) // 4 headers only

	m = m.ExpandAll()
	assert.Equal(t, 9, m.Len())
}

func TestSmartCollapse(t *testing.T) {
	t.Run("mixed dirty and healthy groups", func(t *testing.T) {
		// treeTestPods: Deployment/nginx (has CrashLoopBack), Job/worker-job (Completed),
		// StatefulSet/redis (Running), Standalone (Running)
		pods := treeTestPods()
		m := New().SetItems(pods).SetSize(120, 30)
		m = m.SmartCollapse()

		// Dirty groups expanded: Deployment/nginx (2 pods) + Job/worker-job (1 pod) = 2 headers + 3 pods
		// Healthy groups collapsed: StatefulSet/redis + Standalone = 2 headers only
		// Total: 4 headers + 3 pods = 7 rows
		assert.Equal(t, 7, m.Len())

		// Cursor should be on the first pod row
		row := m.CursorRow()
		assert.NotNil(t, row)
		assert.Equal(t, RowPod, row.Kind)
	})

	t.Run("all pods healthy", func(t *testing.T) {
		pods := []k8s.PodInfo{
			{Name: "web-1", Namespace: "default", Status: k8s.StatusRunning,
				Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "web"}},
			{Name: "api-1", Namespace: "default", Status: k8s.StatusRunning,
				Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "api"}},
		}
		m := New().SetItems(pods).SetSize(120, 30)
		m = m.SmartCollapse()

		// All groups collapsed: 2 headers only
		assert.Equal(t, 2, m.Len())
		assert.False(t, m.AnyExpanded())
	})

	t.Run("all pods dirty", func(t *testing.T) {
		pods := []k8s.PodInfo{
			{Name: "crash-1", Namespace: "default", Status: k8s.StatusCrashLoopBack,
				Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "crash"}},
			{Name: "evicted-1", Namespace: "default", Status: k8s.StatusEvicted,
				Controller: k8s.ControllerRef{Kind: k8s.ControllerStatefulSet, Name: "evicted"}},
		}
		m := New().SetItems(pods).SetSize(120, 30)
		m = m.SmartCollapse()

		// All groups expanded: 2 headers + 2 pods = 4 rows
		assert.Equal(t, 4, m.Len())
		assert.True(t, m.AnyExpanded())
	})

	t.Run("empty pod list", func(t *testing.T) {
		m := New().SetItems(nil).SetSize(120, 30)
		m = m.SmartCollapse()
		assert.Equal(t, 0, m.Len())
	})

	t.Run("standalone group with dirty pod", func(t *testing.T) {
		pods := []k8s.PodInfo{
			{Name: "healthy-deploy", Namespace: "default", Status: k8s.StatusRunning,
				Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "app"}},
			{Name: "standalone-crash", Namespace: "default", Status: k8s.StatusCrashLoopBack,
				Controller: k8s.ControllerRef{Kind: k8s.ControllerStandalone}},
		}
		m := New().SetItems(pods).SetSize(120, 30)
		m = m.SmartCollapse()

		// Deployment/app collapsed (1 header), Standalone expanded (1 header + 1 pod) = 3 rows
		assert.Equal(t, 3, m.Len())
	})
}

func TestToggleSelect_ControllerRow(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "nginx-1", Namespace: "default", Status: k8s.StatusRunning,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "nginx"}},
		{Name: "nginx-2", Namespace: "default", Status: k8s.StatusRunning,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "nginx"}},
	}
	m := New().SetItems(pods).SetSize(120, 20)

	// Move to header row and toggle select → should select all pods in group
	m = m.GoTop()
	m = m.ToggleSelect()
	assert.Equal(t, 2, m.SelectedCount())

	// Toggle again → deselect all
	m = m.ToggleSelect()
	assert.Equal(t, 0, m.SelectedCount())
}

func TestCursorRow(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "nginx-1", Namespace: "default", Status: k8s.StatusRunning,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "nginx"}},
	}
	m := New().SetItems(pods).SetSize(120, 20)

	// Cursor starts on first pod (row 1)
	row := m.CursorRow()
	assert.NotNil(t, row)
	assert.Equal(t, RowPod, row.Kind)

	// Move to header
	m = m.GoTop()
	row = m.CursorRow()
	assert.NotNil(t, row)
	assert.Equal(t, RowController, row.Kind)
	assert.Nil(t, m.CursorItem()) // CursorItem returns nil for controller rows
}

func TestView_TreeStructure(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "nginx-1", Namespace: "default", Status: k8s.StatusRunning,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "nginx"}},
		{Name: "debug", Namespace: "default", Status: k8s.StatusRunning,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerStandalone}},
	}
	m := New().SetItems(pods).SetSize(120, 20)
	view := m.View()

	// Should contain controller group headers
	assert.Contains(t, view, "Deployment/nginx")
	assert.Contains(t, view, "Standalone")
	// Should contain expand/collapse indicators
	assert.Contains(t, view, "▼")
}
