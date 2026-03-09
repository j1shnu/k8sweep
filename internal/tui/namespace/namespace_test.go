package namespace

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestView_ScrollsWithCursor(t *testing.T) {
	namespaces := make([]string, 0, 30)
	for i := 1; i <= 30; i++ {
		namespaces = append(namespaces, fmt.Sprintf("ns-%02d", i))
	}

	m := New().SetNamespaces(namespaces).Activate()

	for i := 0; i < 20; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated
	}

	view := m.View()
	assert.Contains(t, view, "ns-20")
	assert.NotContains(t, view, "ns-01")
}
