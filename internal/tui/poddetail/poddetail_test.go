package poddetail

import (
	"testing"
	"time"

	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/stretchr/testify/assert"
)

func sampleDetail() *k8s.PodDetail {
	return &k8s.PodDetail{
		Name:      "test-pod",
		Namespace: "default",
		Status:    k8s.StatusRunning,
		Node:      "node-1",
		Age:       48 * time.Hour,
		PodIP:     "10.0.0.5",
		HostIP:    "192.168.1.1",
		QoSClass:  "Burstable",
		Owner:     "ReplicaSet/test-rs-abc123",
		Labels:    map[string]string{"app": "test", "version": "v1"},
		Containers: []k8s.ContainerDetail{
			{
				Name:         "main",
				Image:        "nginx:1.21",
				Ports:        []k8s.ContainerPort{{ContainerPort: 80, Protocol: "TCP"}},
				State:        "Running",
				Ready:        true,
				RestartCount: 2,
				Requests:     k8s.ResourceList{CPU: "100m", Memory: "128Mi"},
				Limits:       k8s.ResourceList{CPU: "500m", Memory: "256Mi"},
			},
		},
		Conditions: []k8s.PodCondition{
			{Type: "Ready", Status: "True"},
			{Type: "PodScheduled", Status: "True"},
		},
	}
}

func TestNew_Hidden(t *testing.T) {
	m := New()
	assert.False(t, m.IsVisible())
	assert.Equal(t, StateHidden, m.state)
	assert.Empty(t, m.View())
}

func TestSetLoading(t *testing.T) {
	m := New().SetLoading()
	assert.True(t, m.IsVisible())
	assert.Equal(t, StateLoading, m.state)
	view := m.View()
	assert.Contains(t, view, "Loading pod details")
}

func TestSetDetail(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail())
	assert.True(t, m.IsVisible())
	assert.Equal(t, StateReady, m.state)

	view := m.View()
	assert.Contains(t, view, "test-pod")
	assert.Contains(t, view, "default")
	assert.Contains(t, view, "node-1")
	assert.Contains(t, view, "10.0.0.5")
	assert.Contains(t, view, "Burstable")
	assert.Contains(t, view, "ReplicaSet/test-rs-abc123")
	assert.Contains(t, view, "main")
	assert.Contains(t, view, "nginx:1.21")
	assert.Contains(t, view, "image:")
	assert.Contains(t, view, "80/TCP")
	assert.Contains(t, view, "Ready")
}

func TestSetError(t *testing.T) {
	m := New().SetError("pod not found")
	assert.True(t, m.IsVisible())
	assert.Equal(t, StateError, m.state)
	view := m.View()
	assert.Contains(t, view, "pod not found")
}

func TestHide(t *testing.T) {
	m := New().SetLoading().Hide()
	assert.False(t, m.IsVisible())
}

func TestScroll(t *testing.T) {
	// Create a detail with many lines, small height
	m := New().SetSize(100, 15).SetDetail(sampleDetail())
	assert.Equal(t, 0, m.scroll)

	// Scroll down
	m = m.ScrollDown()
	assert.Equal(t, 1, m.scroll)

	// Scroll back up
	m = m.ScrollUp()
	assert.Equal(t, 0, m.scroll)

	// Scroll up at top = no-op
	m = m.ScrollUp()
	assert.Equal(t, 0, m.scroll)
}

func TestScrollDown_Bounded(t *testing.T) {
	// Very tall height, few lines → maxScroll should be 0
	m := New().SetSize(100, 200).SetDetail(sampleDetail())
	m = m.ScrollDown()
	assert.Equal(t, 0, m.scroll) // can't scroll past content
}

func TestSetSize_Preserved(t *testing.T) {
	m := New().SetSize(120, 40)
	assert.Equal(t, 120, m.width)
	assert.Equal(t, 40, m.height)

	m = m.SetLoading()
	assert.Equal(t, 120, m.width)
}

func TestView_Footer(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail())
	view := m.View()
	assert.Contains(t, view, "[j/k]")
	assert.Contains(t, view, "scroll")
	assert.Contains(t, view, "[e]")
	assert.Contains(t, view, "shell")
	assert.Contains(t, view, "[i/esc]")
	assert.Contains(t, view, "close")
}

func TestView_Labels(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail())
	view := m.View()
	assert.Contains(t, view, "app")
	assert.Contains(t, view, "test")
	assert.Contains(t, view, "version")
}

func TestView_ContainerResources(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail())
	view := m.View()
	assert.Contains(t, view, "Requests:")
	assert.Contains(t, view, "Limits:")
	assert.Contains(t, view, "cpu=100m")
	assert.Contains(t, view, "mem=256Mi")
}

func TestView_Conditions(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail())
	view := m.View()
	assert.Contains(t, view, "Ready=True")
	assert.Contains(t, view, "PodScheduled=True")
}

// --- Events subview tests ---

func TestSetEvents(t *testing.T) {
	events := []k8s.PodEvent{
		{Type: "Warning", Reason: "BackOff", Message: "Back-off restarting", Count: 3, LastTimestamp: time.Now()},
		{Type: "Normal", Reason: "Scheduled", Message: "Assigned to node", Count: 1, LastTimestamp: time.Now().Add(-5 * time.Minute)},
	}
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetEvents(events)
	assert.Equal(t, SubviewEvents, m.Subview())
	assert.Equal(t, StateReady, m.state)

	view := m.View()
	assert.Contains(t, view, "Events:")
	assert.Contains(t, view, "BackOff")
	assert.Contains(t, view, "Scheduled")
	assert.Contains(t, view, "[esc]")
	assert.Contains(t, view, "back to detail")
}

func TestSetEventsEmpty(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetEvents(nil)
	view := m.View()
	assert.Contains(t, view, "No events found")
}

func TestSetEventsError(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetEventsError("forbidden")
	assert.Equal(t, SubviewEvents, m.Subview())
	assert.Equal(t, StateError, m.state)
	view := m.View()
	assert.Contains(t, view, "forbidden")
}

func TestSetEventsLoading(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetEventsLoading()
	assert.Equal(t, SubviewEvents, m.Subview())
	assert.Equal(t, StateLoading, m.state)
	view := m.View()
	assert.Contains(t, view, "Loading events")
}

// --- Logs subview tests ---

func TestSetLogs(t *testing.T) {
	lines := []string{"line 1", "line 2", "line 3"}
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetLogs(lines, "main")
	assert.Equal(t, SubviewLogs, m.Subview())
	assert.Equal(t, StateReady, m.state)

	view := m.View()
	assert.Contains(t, view, "Logs:")
	assert.Contains(t, view, "main")
	assert.Contains(t, view, "line 1")
	assert.Contains(t, view, "line 3")
	assert.Contains(t, view, "[esc]")
	assert.Contains(t, view, "back to detail")
}

func TestSetLogsEmpty(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetLogs(nil, "main")
	view := m.View()
	assert.Contains(t, view, "No logs available")
}

func TestSetLogsError(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetLogsError("unauthorized")
	assert.Equal(t, SubviewLogs, m.Subview())
	assert.Equal(t, StateError, m.state)
	view := m.View()
	assert.Contains(t, view, "unauthorized")
}

func TestSetLogsLoading(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetLogsLoading()
	assert.Equal(t, SubviewLogs, m.Subview())
	assert.Equal(t, StateLoading, m.state)
	view := m.View()
	assert.Contains(t, view, "Loading logs")
}

// --- Subview navigation tests ---

func TestShowDetail_FromEvents(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetEvents(nil)
	assert.Equal(t, SubviewEvents, m.Subview())

	m = m.ShowDetail()
	assert.Equal(t, SubviewDetail, m.Subview())
	assert.Equal(t, 0, m.scroll)
}

func TestShowDetail_FromLogs(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetLogs([]string{"log"}, "c")
	assert.Equal(t, SubviewLogs, m.Subview())

	m = m.ShowDetail()
	assert.Equal(t, SubviewDetail, m.Subview())
}

func TestHide_ResetsSubview(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail()).SetEvents(nil)
	m = m.Hide()
	assert.False(t, m.IsVisible())
	assert.Equal(t, SubviewDetail, m.Subview())
}

func TestView_DetailFooter_ShowsEventsAndLogs(t *testing.T) {
	m := New().SetSize(100, 40).SetDetail(sampleDetail())
	view := m.View()
	assert.Contains(t, view, "[v]")
	assert.Contains(t, view, "events")
	assert.Contains(t, view, "[o]")
	assert.Contains(t, view, "logs")
}
