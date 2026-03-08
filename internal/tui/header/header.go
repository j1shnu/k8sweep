package header

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// Model represents the header component showing cluster context and namespace.
type Model struct {
	ClusterName string
	Namespace   string
	Width       int
}

// New creates a new header model.
func New(clusterName, namespace string) Model {
	return Model{
		ClusterName: clusterName,
		Namespace:   namespace,
	}
}

// Update handles messages for the header.
func (m Model) Update(msg tea.Msg) Model {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return Model{
			ClusterName: m.ClusterName,
			Namespace:   m.Namespace,
			Width:       msg.Width,
		}
	}
	return m
}

// SetNamespace returns a new model with the updated namespace.
func (m Model) SetNamespace(ns string) Model {
	return Model{
		ClusterName: m.ClusterName,
		Namespace:   ns,
		Width:       m.Width,
	}
}

// View renders the header.
func (m Model) View() string {
	content := fmt.Sprintf("⎈ %s  │  ns: %s", m.ClusterName, m.Namespace)
	style := styles.HeaderBox
	if m.Width > 0 {
		style = style.Width(m.Width - 2)
	}
	return style.Render(content)
}
