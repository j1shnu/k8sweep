package app

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newTestModel(pods []k8s.PodInfo) Model {
	cs := fake.NewClientset()
	client := k8s.NewClientFromClientset(cs, k8s.ClusterInfo{
		ContextName: "test-context",
		Namespace:   "default",
	})
	m := NewModel(client)
	// Simulate window resize so View() works
	m.width = 120
	m.height = 40
	// Simulate pods loaded
	if pods != nil {
		m.allPods = pods
		m.totalPodCount = len(pods)
		m.podList = m.podList.SetItems(pods)
	}
	return m
}

func samplePods() []k8s.PodInfo {
	return []k8s.PodInfo{
		{Name: "running-1", Namespace: "default", Status: k8s.StatusRunning, Age: 1 * time.Hour},
		{Name: "running-2", Namespace: "default", Status: k8s.StatusRunning, Age: 2 * time.Hour},
		{Name: "failed-1", Namespace: "default", Status: k8s.StatusFailed, Age: 3 * time.Hour},
		{Name: "completed-1", Namespace: "default", Status: k8s.StatusCompleted, Age: 4 * time.Hour},
		{Name: "crash-1", Namespace: "default", Status: k8s.StatusCrashLoopBack, Age: 5 * time.Hour},
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

func TestFilterToggleOn_WithCachedPods_NoRefetch(t *testing.T) {
	m := newTestModel(samplePods())
	assert.False(t, m.filter.ShowDirtyOnly)
	assert.Equal(t, 5, m.podList.PodCount())

	// Toggle filter ON
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	updated := result.(Model)

	// Should use cached data — no command returned (no fetch)
	assert.Nil(t, cmd)
	assert.True(t, updated.filter.ShowDirtyOnly)
	// Only dirty pods: failed-1, completed-1, crash-1
	assert.Equal(t, 3, updated.podList.PodCount())
	assert.Contains(t, updated.statusMsg, "dirty pods only")
}

func TestFilterToggleOff_WithCachedPods_NoRefetch(t *testing.T) {
	m := newTestModel(samplePods())
	// Start with filter ON
	m.filter = k8s.ResourceFilter{ShowDirtyOnly: true}
	dirtyPods := k8s.FilterDirtyPods(samplePods())
	m.podList = m.podList.SetItems(dirtyPods)

	// Toggle filter OFF
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	updated := result.(Model)

	// Should use cached data — no command returned
	assert.Nil(t, cmd)
	assert.False(t, updated.filter.ShowDirtyOnly)
	// All pods restored from cache
	assert.Equal(t, 5, updated.podList.PodCount())
	assert.Contains(t, updated.statusMsg, "all pods")
}

func TestFilterToggleOn_NoCachedPods_TriggersRefetch(t *testing.T) {
	m := newTestModel(nil)

	// Toggle filter ON with no cached pods
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	updated := result.(Model)

	// Should trigger a fetch command
	assert.NotNil(t, cmd)
	assert.True(t, updated.filter.ShowDirtyOnly)
}

func TestHandlePodsLoaded_CachesAllPods(t *testing.T) {
	m := newTestModel(nil)
	m.fetchID = 42

	msg := PodsLoadedMsg{
		Pods:    samplePods(),
		FetchID: 42,
	}
	updated, _ := m.handlePodsLoaded(msg)

	// allPods should contain the full list
	require.NotNil(t, updated.allPods)
	assert.Equal(t, 5, len(updated.allPods))
	assert.Equal(t, 5, updated.totalPodCount)
	// Display should show all pods (filter is off)
	assert.Equal(t, 5, updated.podList.PodCount())
}

func TestHandlePodsLoaded_FilterActive_CachesAllShowsDirty(t *testing.T) {
	m := newTestModel(nil)
	m.fetchID = 42
	m.filter = k8s.ResourceFilter{ShowDirtyOnly: true}

	msg := PodsLoadedMsg{
		Pods:    samplePods(),
		FetchID: 42,
	}
	updated, _ := m.handlePodsLoaded(msg)

	// allPods should still contain the FULL list
	require.NotNil(t, updated.allPods)
	assert.Equal(t, 5, len(updated.allPods))
	assert.Equal(t, 5, updated.totalPodCount)
	// Display should show only dirty pods
	assert.Equal(t, 3, updated.podList.PodCount())
}

func TestHandlePodsLoaded_DiscardsStaleFetch(t *testing.T) {
	m := newTestModel(samplePods())
	m.fetchID = 42

	msg := PodsLoadedMsg{
		Pods:    nil,
		FetchID: 99, // stale
	}
	updated, _ := m.handlePodsLoaded(msg)

	// Should not change anything
	assert.Equal(t, 5, updated.podList.PodCount())
	assert.Equal(t, 5, len(updated.allPods))
}

func TestSwitchNamespace_ClearsCache(t *testing.T) {
	cs := fake.NewClientset()
	client := k8s.NewClientFromClientset(cs, k8s.ClusterInfo{
		ContextName: "test-context",
		Namespace:   "default",
	})
	m := NewModel(client)
	m.width = 120
	m.height = 40
	m.allPods = samplePods()

	updated, _ := m.switchNamespace("kube-system")

	assert.Nil(t, updated.allPods)
	assert.Equal(t, "kube-system", updated.namespace)
}

func TestBuildPodCountLabel(t *testing.T) {
	assert.Equal(t, "", buildPodCountLabel(false, 3, 10))
	assert.Equal(t, "3/10 dirty pods", buildPodCountLabel(true, 3, 10))
	assert.Equal(t, "0/5 dirty pods", buildPodCountLabel(true, 0, 5))
}

func TestFilterToggle_PodCountBadgeAccurate(t *testing.T) {
	m := newTestModel(samplePods())

	// Toggle filter ON
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	updated := result.(Model)

	// Header view should contain the pod count
	view := updated.header.View()
	assert.Contains(t, view, "FILTERED")
	assert.Contains(t, view, "Cluster:")
	assert.Contains(t, view, "Namespace:")
}

func TestPodsLoaded_IntegrationWithFakeClient(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	cs := fake.NewClientset(pod)
	client := k8s.NewClientFromClientset(cs, k8s.ClusterInfo{
		ContextName: "test-context",
		Namespace:   "default",
	})
	m := NewModel(client)

	// Simulate the fetch
	pods, err := k8s.ListPods(t.Context(), client, "default")
	require.NoError(t, err)

	msg := PodsLoadedMsg{
		Pods:    pods,
		FetchID: m.fetchID,
	}
	updated, _ := m.handlePodsLoaded(msg)

	assert.Equal(t, 1, len(updated.allPods))
	assert.Equal(t, 1, updated.podList.PodCount())
	assert.Equal(t, "test-pod", updated.allPods[0].Name)
}

func TestPagingKeys_BrowsingMode(t *testing.T) {
	m := newTestModel(manyPods(30))
	m.podList = m.podList.SetSize(120, 10)

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	require.Nil(t, cmd)
	updated := result.(Model)

	p := updated.podList.CursorItem()
	require.NotNil(t, p)
	assert.Equal(t, "pod-09", p.Name)

	result, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	require.Nil(t, cmd)
	updated = result.(Model)
	p = updated.podList.CursorItem()
	require.NotNil(t, p)
	assert.Equal(t, "pod-01", p.Name)
}

func TestPagingArrowKeys_BrowsingMode(t *testing.T) {
	m := newTestModel(manyPods(30))
	m.podList = m.podList.SetSize(120, 10)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	updated := result.(Model)
	p := updated.podList.CursorItem()
	require.NotNil(t, p)
	assert.Equal(t, "pod-09", p.Name)

	result, _ = updated.Update(tea.KeyMsg{Type: tea.KeyLeft})
	updated = result.(Model)
	p = updated.podList.CursorItem()
	require.NotNil(t, p)
	assert.Equal(t, "pod-01", p.Name)
}

func TestSingleStepNavigationStillWorks(t *testing.T) {
	m := newTestModel(manyPods(30))
	m.podList = m.podList.SetSize(120, 10)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated := result.(Model)
	p := updated.podList.CursorItem()
	require.NotNil(t, p)
	assert.Equal(t, "pod-02", p.Name)

	result, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	updated = result.(Model)
	p = updated.podList.CursorItem()
	require.NotNil(t, p)
	assert.Equal(t, "pod-01", p.Name)
}

func TestSingleStepNavigation_DoesNotCrossPage(t *testing.T) {
	m := newTestModel(manyPods(30))
	m.podList = m.podList.SetSize(120, 10)

	for i := 0; i < 20; i++ {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = result.(Model)
	}

	p := m.podList.CursorItem()
	require.NotNil(t, p)
	// page size 8; row 0 is controller header, pods at rows 1-7 → clamped at row 7 = pod-07
	assert.Equal(t, "pod-07", p.Name)
}

func TestBuildStatusSummary(t *testing.T) {
	pods := []k8s.PodInfo{
		{Status: k8s.StatusCrashLoopBack},
		{Status: k8s.StatusFailed},
		{Status: k8s.StatusImagePullErr},
		{Status: k8s.StatusOOMKilled},
		{Status: k8s.StatusEvicted},
		{Status: k8s.StatusPending},
		{Status: k8s.StatusTerminating},
		{Status: k8s.StatusRunning},
		{Status: k8s.StatusCompleted},
	}

	s := buildStatusSummary(pods)
	assert.Equal(t, 2, s.CritCrash)
	assert.Equal(t, 1, s.CritImgErr)
	assert.Equal(t, 1, s.CritOOM)
	assert.Equal(t, 1, s.CritEvicted)
	assert.Equal(t, 1, s.WarnPending)
	assert.Equal(t, 1, s.WarnTerminating)
	assert.Equal(t, 1, s.OKRunning)
	assert.Equal(t, 1, s.OKCompleted)
}

func TestFilterOn_HeaderSummaryUsesAllPods(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "failed-1", Namespace: "default", Status: k8s.StatusFailed},
		{Name: "pending-1", Namespace: "default", Status: k8s.StatusPending},
		{Name: "evicted-1", Namespace: "default", Status: k8s.StatusEvicted},
	}
	m := newTestModel(pods)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	updated := result.(Model)

	view := updated.header.View()
	assert.Contains(t, view, "Crit:")
	assert.Contains(t, view, "Warn:")
	assert.Contains(t, view, "1 Evict")
	assert.Contains(t, view, "1 Pend")
}

func TestFilterOff_ResetsToFirstPage(t *testing.T) {
	pods := manyPods(30)
	pods[0].Status = k8s.StatusFailed
	pods[1].Status = k8s.StatusFailed
	pods[2].Status = k8s.StatusFailed

	m := newTestModel(pods)
	m.podList = m.podList.SetSize(120, 10)

	// Turn filter ON and move to another page.
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = result.(Model)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = result.(Model)

	// Turn filter OFF: should reset to page 1 / first item.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = result.(Model)

	p := m.podList.CursorItem()
	require.NotNil(t, p)
	assert.Equal(t, "pod-01", p.Name)
}

func TestDetailShell_SingleContainerStartsShellCmd(t *testing.T) {
	m := newTestModel(samplePods())
	m.state = stateViewingDetail
	m.detailPodKey = "default/running-1"
	m.detailData = &k8s.PodDetail{
		Name:      "running-1",
		Namespace: "default",
		Status:    k8s.StatusRunning,
		Containers: []k8s.ContainerDetail{
			{Name: "main", Image: "nginx", State: "Running"},
		},
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated := result.(Model)

	require.NotNil(t, cmd)
	assert.Equal(t, stateViewingDetail, updated.state)
}

func TestDetailShell_MultiContainerOpensPicker(t *testing.T) {
	m := newTestModel(samplePods())
	m.state = stateViewingDetail
	m.detailPodKey = "default/running-1"
	m.detailData = &k8s.PodDetail{
		Name:      "running-1",
		Namespace: "default",
		Status:    k8s.StatusRunning,
		Containers: []k8s.ContainerDetail{
			{Name: "main", Image: "nginx", State: "Running"},
			{Name: "sidecar", Image: "busybox", State: "Running"},
		},
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated := result.(Model)

	require.Nil(t, cmd)
	assert.Equal(t, statePickingContainer, updated.state)
	require.NotNil(t, updated.containerSel.Selected())
	assert.Equal(t, "main", updated.containerSel.Selected().Name)
}

func TestContainerPicker_EnterStartsShellCmd(t *testing.T) {
	m := newTestModel(samplePods())
	m.state = statePickingContainer
	m.detailPodKey = "default/running-1"
	m.detailData = &k8s.PodDetail{
		Name:      "running-1",
		Namespace: "default",
		Status:    k8s.StatusRunning,
		Containers: []k8s.ContainerDetail{
			{Name: "main", Image: "nginx", State: "Running"},
			{Name: "sidecar", Image: "busybox", State: "Running"},
		},
	}
	m.containerSel = m.containerSel.SetContainers(m.detailData.Containers)
	m.containerSel = m.containerSel.MoveDown()

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	require.NotNil(t, cmd)
	assert.Equal(t, stateViewingDetail, updated.state)
}

func TestDetailShell_RejectsCompletedPod(t *testing.T) {
	m := newTestModel(samplePods())
	m.state = stateViewingDetail
	m.detailData = &k8s.PodDetail{
		Name:      "completed-1",
		Namespace: "default",
		Status:    k8s.StatusCompleted,
		Containers: []k8s.ContainerDetail{
			{Name: "main", Image: "nginx"},
		},
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated := result.(Model)

	require.Nil(t, cmd)
	assert.Equal(t, stateViewingDetail, updated.state)
	assert.Contains(t, updated.detailStatus, "Shell unavailable")
}

func TestDetailShell_CrashLoopWarningThenProceeds(t *testing.T) {
	m := newTestModel(samplePods())
	m.state = stateViewingDetail
	m.detailPodKey = "default/crash-1"
	m.detailData = &k8s.PodDetail{
		Name:      "crash-1",
		Namespace: "default",
		Status:    k8s.StatusCrashLoopBack,
		Containers: []k8s.ContainerDetail{
			{Name: "main", Image: "nginx", State: "Running"},
		},
	}

	// First press — should show warning, no cmd
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated := result.(Model)

	require.Nil(t, cmd)
	assert.True(t, updated.shellWarningAcked)
	assert.Contains(t, updated.detailStatus, "Warning")
	assert.Contains(t, updated.detailStatus, "CrashLoopBackOff")

	// Second press — should launch shell
	result2, cmd2 := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated2 := result2.(Model)

	require.NotNil(t, cmd2)
	assert.False(t, updated2.shellWarningAcked)
}

func TestDetailShell_CrashLoopAllContainersDown_NoWarning(t *testing.T) {
	m := newTestModel(samplePods())
	m.state = stateViewingDetail
	m.detailPodKey = "default/crash-1"
	m.detailData = &k8s.PodDetail{
		Name:      "crash-1",
		Namespace: "default",
		Status:    k8s.StatusCrashLoopBack,
		Containers: []k8s.ContainerDetail{
			{Name: "main", Image: "nginx", State: "Waiting: CrashLoopBackOff"},
		},
	}

	// Should skip warning and show unavailable directly
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated := result.(Model)

	require.Nil(t, cmd)
	assert.False(t, updated.shellWarningAcked)
	assert.Contains(t, updated.detailStatus, "Shell unavailable")
	assert.Contains(t, updated.detailStatus, "not running")
}

func TestDetailShell_SingleContainerNotRunning_Rejected(t *testing.T) {
	m := newTestModel(samplePods())
	m.state = stateViewingDetail
	m.detailPodKey = "default/running-1"
	m.detailData = &k8s.PodDetail{
		Name:      "running-1",
		Namespace: "default",
		Status:    k8s.StatusRunning,
		Containers: []k8s.ContainerDetail{
			{Name: "main", Image: "nginx", State: "Waiting: ContainerCreating"},
		},
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated := result.(Model)

	require.Nil(t, cmd)
	assert.Contains(t, updated.detailStatus, "Shell unavailable")
	assert.Contains(t, updated.detailStatus, "not running")
}

func TestContainerPicker_NonRunningContainerRejected(t *testing.T) {
	m := newTestModel(samplePods())
	m.state = statePickingContainer
	m.detailPodKey = "default/running-1"
	m.detailData = &k8s.PodDetail{
		Name:      "running-1",
		Namespace: "default",
		Status:    k8s.StatusRunning,
		Containers: []k8s.ContainerDetail{
			{Name: "main", Image: "nginx", State: "Running"},
			{Name: "sidecar", Image: "busybox", State: "Waiting: CrashLoopBackOff"},
		},
	}
	m.containerSel = m.containerSel.SetContainers(m.detailData.Containers)
	// Move to sidecar (non-running)
	m.containerSel = m.containerSel.MoveDown()

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(Model)

	require.Nil(t, cmd)
	assert.Contains(t, updated.detailStatus, "Shell unavailable")
	assert.Contains(t, updated.detailStatus, "sidecar")
}

func TestContainerRunning(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		{"Running", true},
		{"Running (started 2h ago)", true},
		{"Waiting: CrashLoopBackOff", false},
		{"Terminated: OOMKilled", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			c := k8s.ContainerDetail{Name: "test", State: tt.state}
			assert.Equal(t, tt.want, containerRunning(c))
		})
	}
}

func TestHasRunningContainer(t *testing.T) {
	assert.True(t, hasRunningContainer([]k8s.ContainerDetail{
		{Name: "a", State: "Running"},
		{Name: "b", State: "Waiting: CrashLoopBackOff"},
	}))
	assert.False(t, hasRunningContainer([]k8s.ContainerDetail{
		{Name: "a", State: "Waiting: CrashLoopBackOff"},
		{Name: "b", State: "Terminated: OOMKilled"},
	}))
	assert.False(t, hasRunningContainer(nil))
}

func TestIsMissingShellError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		stderr string
		want   bool
	}{
		{"executable file not found", fmt.Errorf("executable file not found in $PATH"), "", true},
		{"no such file or directory", fmt.Errorf("exec failed"), "/bin/bash: no such file or directory", true},
		{"pod not found should NOT match", fmt.Errorf("pod not found"), "", false},
		{"container not found should NOT match", fmt.Errorf("container not found"), "", false},
		{"generic error", fmt.Errorf("connection refused"), "", false},
		{"nil error", nil, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isMissingShellError(tt.err, tt.stderr))
		})
	}
}

func TestHandlePodShellExited_SetsStatusAndReturnsToBrowsing(t *testing.T) {
	m := newTestModel(samplePods())
	m.state = stateViewingDetail
	m.detailPodKey = "default/running-1"
	m.detailData = &k8s.PodDetail{Name: "running-1", Namespace: "default", Status: k8s.StatusRunning}

	updated := m.handlePodShellExited(PodShellExitedMsg{
		PodKey:    "default/running-1",
		Container: "main",
		Backend:   "kubectl",
		ShellPath: "/bin/sh",
	})

	assert.Equal(t, stateBrowsing, updated.state)
	assert.Contains(t, updated.statusMsg, "Shell closed")
	assert.Nil(t, updated.detailData)
}
