package footer

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// Model represents the footer help bar.
type Model struct {
	Bindings []key.Binding
	Width    int
}

// New creates a new footer model.
func New(bindings []key.Binding) Model {
	return Model{
		Bindings: bindings,
	}
}

// SetWidth returns a new model with the updated width.
func (m Model) SetWidth(w int) Model {
	return Model{
		Bindings: m.Bindings,
		Width:    w,
	}
}

// View renders the footer help bar.
func (m Model) View() string {
	var parts []string
	for _, b := range m.Bindings {
		if !b.Enabled() {
			continue
		}
		help := b.Help()
		parts = append(parts, "["+help.Key+"] "+help.Desc)
	}
	line := strings.Join(parts, "  ")
	return styles.FooterHelp.Render(line)
}
