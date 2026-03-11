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
	assert.Contains(t, view, "j/k scroll")
	assert.Contains(t, view, "e shell")
	assert.Contains(t, view, "i/esc close")
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
