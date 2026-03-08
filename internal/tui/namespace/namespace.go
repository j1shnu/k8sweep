package namespace

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// Model represents the namespace switcher component.
type Model struct {
	namespaces []string
	filtered   []string
	input      textinput.Model
	cursor     int
	selected   string
	active     bool
}

// New creates a new namespace switcher.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Filter namespaces..."
	ti.CharLimit = 64
	return Model{
		input: ti,
	}
}

// IsActive returns whether the namespace switcher is active.
func (m Model) IsActive() bool {
	return m.active
}

// SetNamespaces returns a new model with the namespace list populated.
func (m Model) SetNamespaces(ns []string) Model {
	return Model{
		namespaces: ns,
		filtered:   ns,
		input:      m.input,
		cursor:     0,
		selected:   m.selected,
		active:     m.active,
	}
}

// Activate returns a new model with the switcher active and focused.
func (m Model) Activate() Model {
	ti := m.input
	ti.Focus()
	ti.SetValue("")
	return Model{
		namespaces: m.namespaces,
		filtered:   m.namespaces,
		input:      ti,
		cursor:     0,
		selected:   "",
		active:     true,
	}
}

// Deactivate returns a new model with the switcher inactive.
func (m Model) Deactivate() Model {
	ti := m.input
	ti.Blur()
	return Model{
		namespaces: m.namespaces,
		filtered:   m.filtered,
		input:      ti,
		cursor:     m.cursor,
		selected:   m.selected,
		active:     false,
	}
}

// Update handles key events for the namespace switcher.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m.Deactivate(), nil

		case "enter":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				return Model{
					namespaces: m.namespaces,
					filtered:   m.filtered,
					input:      m.input,
					cursor:     m.cursor,
					selected:   m.filtered[m.cursor],
					active:     false,
				}, nil
			}
			return m, nil

		case "up", "ctrl+k":
			newCursor := m.cursor - 1
			if newCursor < 0 {
				newCursor = 0
			}
			return Model{
				namespaces: m.namespaces,
				filtered:   m.filtered,
				input:      m.input,
				cursor:     newCursor,
				selected:   m.selected,
				active:     m.active,
			}, nil

		case "down", "ctrl+j":
			newCursor := m.cursor + 1
			if newCursor >= len(m.filtered) {
				newCursor = len(m.filtered) - 1
			}
			if newCursor < 0 {
				newCursor = 0
			}
			return Model{
				namespaces: m.namespaces,
				filtered:   m.filtered,
				input:      m.input,
				cursor:     newCursor,
				selected:   m.selected,
				active:     m.active,
			}, nil
		}
	}

	// Pass to text input for filtering
	newInput, cmd := m.input.Update(msg)
	filtered := filterNamespaces(m.namespaces, newInput.Value())
	newCursor := m.cursor
	if newCursor >= len(filtered) {
		newCursor = len(filtered) - 1
	}
	if newCursor < 0 {
		newCursor = 0
	}

	return Model{
		namespaces: m.namespaces,
		filtered:   filtered,
		input:      newInput,
		cursor:     newCursor,
		selected:   m.selected,
		active:     m.active,
	}, cmd
}

// GetSelected returns the selected namespace and whether a selection was made.
func (m Model) GetSelected() (string, bool) {
	if m.selected != "" {
		return m.selected, true
	}
	return "", false
}

// View renders the namespace switcher.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Switch Namespace"))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	maxShow := 15
	for i, ns := range m.filtered {
		if i >= maxShow {
			break
		}
		pointer := "  "
		if i == m.cursor {
			pointer = styles.Pointer.Render("> ")
		}
		b.WriteString(pointer + ns + "\n")
	}

	if len(m.filtered) == 0 {
		b.WriteString(styles.FooterHelp.Render("  No matching namespaces"))
	}

	return styles.OverlayBox.Render(b.String())
}

func filterNamespaces(namespaces []string, query string) []string {
	if query == "" {
		return namespaces
	}
	query = strings.ToLower(query)
	filtered := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		if strings.Contains(strings.ToLower(ns), query) {
			filtered = append(filtered, ns)
		}
	}
	return filtered
}
