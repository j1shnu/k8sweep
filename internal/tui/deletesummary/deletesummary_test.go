package deletesummary

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/common"
	"github.com/stretchr/testify/assert"
)

func testResults() []k8s.DeleteResult {
	return []k8s.DeleteResult{
		{PodName: "pod-1", Namespace: "default", Success: true},
		{PodName: "pod-2", Namespace: "kube-system", Success: true},
		{PodName: "pod-3", Namespace: "default", Success: false, Error: fmt.Errorf("forbidden: pod is protected")},
	}
}

func TestNew_NotDismissed(t *testing.T) {
	m := New(testResults(), common.DeleteNormal)
	assert.False(t, m.IsDismissed())
}

func TestDismiss_Enter(t *testing.T) {
	m := New(testResults(), common.DeleteNormal).SetSize(120, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, m.IsDismissed())
}

func TestDismiss_Escape(t *testing.T) {
	m := New(testResults(), common.DeleteNormal).SetSize(120, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	assert.True(t, m.IsDismissed())
}

func TestView_DeleteSummary(t *testing.T) {
	m := New(testResults(), common.DeleteNormal).SetSize(120, 40)
	view := m.View()
	assert.Contains(t, view, "Delete Summary")
	assert.Contains(t, view, "Total:     3")
	assert.Contains(t, view, "Succeeded: 2")
	assert.Contains(t, view, "Failed:    1")
	assert.Contains(t, view, "pod-1")
	assert.Contains(t, view, "pod-2")
	assert.Contains(t, view, "pod-3")
	assert.Contains(t, view, "forbidden")
}

func TestView_ForceDeleteSummary(t *testing.T) {
	m := New(testResults(), common.DeleteForce).SetSize(120, 40)
	view := m.View()
	assert.Contains(t, view, "Force Delete Summary")
}

func TestView_AllSuccess(t *testing.T) {
	results := []k8s.DeleteResult{
		{PodName: "pod-1", Namespace: "default", Success: true},
	}
	m := New(results, common.DeleteNormal).SetSize(120, 40)
	view := m.View()
	assert.Contains(t, view, "Succeeded: 1")
	assert.NotContains(t, view, "Failed:")
}

func TestView_AllFailed(t *testing.T) {
	results := []k8s.DeleteResult{
		{PodName: "pod-1", Namespace: "default", Success: false, Error: fmt.Errorf("timeout")},
	}
	m := New(results, common.DeleteNormal).SetSize(120, 40)
	view := m.View()
	assert.NotContains(t, view, "Succeeded:")
	assert.Contains(t, view, "Failed:    1")
	assert.Contains(t, view, "timeout")
}

func TestScrollDown(t *testing.T) {
	// Create enough results to require scrolling
	results := make([]k8s.DeleteResult, 50)
	for i := range results {
		results[i] = k8s.DeleteResult{PodName: fmt.Sprintf("pod-%d", i), Namespace: "ns", Success: true}
	}
	m := New(results, common.DeleteNormal).SetSize(120, 25)
	assert.Equal(t, 0, m.scroll)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, m.scroll)
}

func TestScrollUp_AtTop(t *testing.T) {
	m := New(testResults(), common.DeleteNormal).SetSize(120, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 0, m.scroll)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 6))
}

func TestCounts(t *testing.T) {
	m := New(testResults(), common.DeleteNormal)
	s, f := m.counts()
	assert.Equal(t, 2, s)
	assert.Equal(t, 1, f)
}
