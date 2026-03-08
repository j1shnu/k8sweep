package app

import (
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
	cs := fake.NewSimpleClientset()
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

func TestFilterToggleOn_WithCachedPods_NoRefetch(t *testing.T) {
	m := newTestModel(samplePods())
	assert.False(t, m.filter.ShowDirtyOnly)
	assert.Equal(t, 5, m.podList.Len())

	// Toggle filter ON
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	updated := result.(Model)

	// Should use cached data — no command returned (no fetch)
	assert.Nil(t, cmd)
	assert.True(t, updated.filter.ShowDirtyOnly)
	// Only dirty pods: failed-1, completed-1, crash-1
	assert.Equal(t, 3, updated.podList.Len())
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
	assert.Equal(t, 5, updated.podList.Len())
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
	updated := m.handlePodsLoaded(msg)

	// allPods should contain the full list
	require.NotNil(t, updated.allPods)
	assert.Equal(t, 5, len(updated.allPods))
	assert.Equal(t, 5, updated.totalPodCount)
	// Display should show all pods (filter is off)
	assert.Equal(t, 5, updated.podList.Len())
}

func TestHandlePodsLoaded_FilterActive_CachesAllShowsDirty(t *testing.T) {
	m := newTestModel(nil)
	m.fetchID = 42
	m.filter = k8s.ResourceFilter{ShowDirtyOnly: true}

	msg := PodsLoadedMsg{
		Pods:    samplePods(),
		FetchID: 42,
	}
	updated := m.handlePodsLoaded(msg)

	// allPods should still contain the FULL list
	require.NotNil(t, updated.allPods)
	assert.Equal(t, 5, len(updated.allPods))
	assert.Equal(t, 5, updated.totalPodCount)
	// Display should show only dirty pods
	assert.Equal(t, 3, updated.podList.Len())
}

func TestHandlePodsLoaded_DiscardsStaleFetch(t *testing.T) {
	m := newTestModel(samplePods())
	m.fetchID = 42

	msg := PodsLoadedMsg{
		Pods:    nil,
		FetchID: 99, // stale
	}
	updated := m.handlePodsLoaded(msg)

	// Should not change anything
	assert.Equal(t, 5, updated.podList.Len())
	assert.Equal(t, 5, len(updated.allPods))
}

func TestSwitchNamespace_ClearsCache(t *testing.T) {
	cs := fake.NewSimpleClientset()
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
	cs := fake.NewSimpleClientset(pod)
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
	updated := m.handlePodsLoaded(msg)

	assert.Equal(t, 1, len(updated.allPods))
	assert.Equal(t, 1, updated.podList.Len())
	assert.Equal(t, "test-pod", updated.allPods[0].Name)
}
