package confirm

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNew_DefaultsToNo(t *testing.T) {
	m := New([]string{"pod-1", "pod-2"})
	assert.Equal(t, 1, m.cursor) // No is default
	assert.False(t, m.IsConfirmed())
	assert.False(t, m.IsCancelled())
}

func TestConfirm_Yes(t *testing.T) {
	m := New([]string{"pod-1"})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	assert.True(t, m.IsConfirmed())
	assert.False(t, m.IsCancelled())
}

func TestConfirm_Escape(t *testing.T) {
	m := New([]string{"pod-1"})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, m.IsConfirmed())
	assert.True(t, m.IsCancelled())
}

func TestConfirm_NavigateAndEnter(t *testing.T) {
	m := New([]string{"pod-1"})
	// Navigate to Yes
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 0, m.cursor)
	// Confirm
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, m.IsConfirmed())
}

func TestConfirm_EnterOnNo(t *testing.T) {
	m := New([]string{"pod-1"})
	// Default is No, press enter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, m.IsConfirmed())
	assert.True(t, m.IsCancelled())
}

func TestConfirm_View(t *testing.T) {
	m := New([]string{"pod-1", "pod-2"})
	view := m.View()
	assert.Contains(t, view, "pod-1")
	assert.Contains(t, view, "pod-2")
	assert.Contains(t, view, "Delete these pods?")
}
