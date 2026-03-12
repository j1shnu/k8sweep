package deletepreview

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/common"
	"github.com/stretchr/testify/assert"
)

func testPods() []k8s.PodInfo {
	return []k8s.PodInfo{
		{Name: "pod-1", Namespace: "default", Status: k8s.StatusCrashLoopBack, Age: 2 * time.Hour},
		{Name: "pod-2", Namespace: "kube-system", Status: k8s.StatusFailed, Age: 48 * time.Hour},
		{Name: "pod-3", Namespace: "default", Status: k8s.StatusEvicted, Age: 10 * time.Minute, OwnerRef: ""},
	}
}

func TestNew_DefaultsToNo(t *testing.T) {
	m := New(testPods(), common.DeleteNormal, nil)
	assert.Equal(t, 1, m.cursor)
	assert.False(t, m.IsConfirmed())
	assert.False(t, m.IsCancelled())
	assert.Equal(t, common.DeleteNormal, m.Action())
}

func TestConfirm_Yes(t *testing.T) {
	m := New(testPods(), common.DeleteNormal, nil).SetSize(120, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	assert.True(t, m.IsConfirmed())
	assert.False(t, m.IsCancelled())
}

func TestConfirm_Escape(t *testing.T) {
	m := New(testPods(), common.DeleteNormal, nil).SetSize(120, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, m.IsConfirmed())
	assert.True(t, m.IsCancelled())
}

func TestConfirm_N(t *testing.T) {
	m := New(testPods(), common.DeleteNormal, nil).SetSize(120, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	assert.True(t, m.IsCancelled())
}

func TestNavigateAndEnter(t *testing.T) {
	m := New(testPods(), common.DeleteNormal, nil).SetSize(120, 40)
	// Navigate to Yes
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 0, m.cursor)
	// Confirm
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, m.IsConfirmed())
}

func TestEnterOnNo(t *testing.T) {
	m := New(testPods(), common.DeleteNormal, nil).SetSize(120, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, m.IsConfirmed())
	assert.True(t, m.IsCancelled())
}

func TestToggleCursor(t *testing.T) {
	m := New(testPods(), common.DeleteNormal, nil).SetSize(120, 40)
	assert.Equal(t, 1, m.cursor) // starts at No
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 0, m.cursor) // now Yes
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, 1, m.cursor) // back to No
}

func TestView_DeletePreview(t *testing.T) {
	pods := testPods()
	m := New(pods, common.DeleteNormal, nil).SetSize(120, 40)
	view := m.View()
	assert.Contains(t, view, "Delete Preview")
	assert.Contains(t, view, "3 pod(s) selected")
	assert.Contains(t, view, "pod-1")
	assert.Contains(t, view, "pod-2")
	assert.Contains(t, view, "pod-3")
	assert.Contains(t, view, "default")
	assert.Contains(t, view, "kube-system")
	assert.Contains(t, view, "CrashLoopBackOff")
	assert.Contains(t, view, "Failed")
}

func TestView_ForceDelete(t *testing.T) {
	m := New(testPods(), common.DeleteForce, nil).SetSize(120, 40)
	view := m.View()
	assert.Contains(t, view, "FORCE DELETE")
	assert.Contains(t, view, "GracePeriodSeconds=0")
}

func TestView_Warnings(t *testing.T) {
	warnings := []string{"pod-3 is standalone (no controller — delete is permanent)"}
	m := New(testPods(), common.DeleteNormal, warnings).SetSize(120, 40)
	view := m.View()
	assert.Contains(t, view, "Warnings:")
	assert.Contains(t, view, "standalone")
}

func TestView_AgeFormatting(t *testing.T) {
	m := New(testPods(), common.DeleteNormal, nil).SetSize(120, 40)
	view := m.View()
	assert.Contains(t, view, "2h")  // 2 hours
	assert.Contains(t, view, "2d")  // 48 hours = 2 days
	assert.Contains(t, view, "10m") // 10 minutes
}

func TestScrollDown(t *testing.T) {
	// Create enough pods to require scrolling
	pods := make([]k8s.PodInfo, 50)
	for i := range pods {
		pods[i] = k8s.PodInfo{Name: "pod-" + strings.Repeat("x", 3), Namespace: "ns", Status: k8s.StatusFailed}
	}
	m := New(pods, common.DeleteNormal, nil).SetSize(120, 25) // small height forces scrolling
	assert.Equal(t, 0, m.scroll)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, m.scroll)
}

func TestScrollUp(t *testing.T) {
	m := New(testPods(), common.DeleteNormal, nil).SetSize(120, 40)
	// Already at top, should not go negative
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 0, m.scroll)
}

func TestPods_ReturnsDefensiveCopy(t *testing.T) {
	pods := testPods()
	m := New(pods, common.DeleteForce, nil)
	got := m.Pods()
	assert.Equal(t, pods, got)
	// Mutating the returned slice should not affect the model
	got[0].Name = "mutated"
	assert.NotEqual(t, "mutated", m.Pods()[0].Name)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 6))
	assert.Equal(t, "ab", truncate("abc", 2))
}
