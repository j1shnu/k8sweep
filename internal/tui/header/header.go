package header

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// Model represents the header component showing cluster context and namespace.
type Model struct {
	clusterName   string
	namespace     string
	filterActive  bool
	podCountLabel string
	width         int
}

// New creates a new header model.
func New(clusterName, namespace string) Model {
	return Model{
		clusterName: clusterName,
		namespace:   namespace,
	}
}

// Update handles messages for the header.
func (m Model) Update(msg tea.Msg) Model {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return Model{
			clusterName:   m.clusterName,
			namespace:     m.namespace,
			filterActive:  m.filterActive,
			podCountLabel: m.podCountLabel,
			width:         msg.Width,
		}
	}
	return m
}

// SetNamespace returns a new model with the updated namespace.
func (m Model) SetNamespace(ns string) Model {
	return Model{
		clusterName:   m.clusterName,
		namespace:     ns,
		filterActive:  m.filterActive,
		podCountLabel: m.podCountLabel,
		width:         m.width,
	}
}

// SetFilter returns a new model with the updated filter state and pod count label.
func (m Model) SetFilter(active bool, countLabel string) Model {
	return Model{
		clusterName:   m.clusterName,
		namespace:     m.namespace,
		filterActive:  active,
		podCountLabel: countLabel,
		width:         m.width,
	}
}

// View renders the header.
func (m Model) View() string {
	nsLabel := m.namespace
	if nsLabel == "" {
		nsLabel = "All Namespaces"
	}

	content := fmt.Sprintf("⎈ %s  │  ns: %s", m.clusterName, nsLabel)

	if m.filterActive {
		content += "  │  " + styles.FilterBadge.Render(" FILTERED ")
		if m.podCountLabel != "" {
			content += "  " + m.podCountLabel
		}
	}

	style := styles.HeaderBox
	if m.filterActive {
		style = styles.FilterActiveHeaderBox
	}
	if m.width > 0 {
		style = style.Width(m.width - 2)
	}
	return style.Render(content)
}
