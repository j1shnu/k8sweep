package confirm

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// Model represents the deletion confirmation overlay.
type Model struct {
	podNames  []string
	cursor    int // 0 = Yes, 1 = No
	confirmed bool
	cancelled bool
	width     int
}

// New creates a new confirmation overlay for the given pod names.
func New(podNames []string) Model {
	return Model{
		podNames: podNames,
		cursor:   1, // Default to "No" for safety
	}
}

// Update handles key events for the confirmation overlay.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "right", "h", "l", "tab":
			newCursor := 1 - m.cursor
			return Model{
				podNames:  m.podNames,
				cursor:    newCursor,
				confirmed: m.confirmed,
				cancelled: m.cancelled,
				width:     m.width,
			}, nil

		case "enter":
			return Model{
				podNames:  m.podNames,
				cursor:    m.cursor,
				confirmed: m.cursor == 0,
				cancelled: m.cursor == 1,
				width:     m.width,
			}, nil

		case "y":
			return Model{
				podNames:  m.podNames,
				cursor:    0,
				confirmed: true,
				cancelled: false,
				width:     m.width,
			}, nil

		case "esc", "n":
			return Model{
				podNames:  m.podNames,
				cursor:    m.cursor,
				confirmed: false,
				cancelled: true,
				width:     m.width,
			}, nil
		}
	}
	return m, nil
}

// IsConfirmed returns true if the user confirmed deletion.
func (m Model) IsConfirmed() bool { return m.confirmed }

// IsCancelled returns true if the user cancelled.
func (m Model) IsCancelled() bool { return m.cancelled }

// View renders the confirmation overlay.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(styles.ErrorMessage.Render("Delete these pods?"))
	b.WriteString("\n\n")

	maxShow := 10
	for i, name := range m.podNames {
		if i >= maxShow {
			b.WriteString(fmt.Sprintf("  ... and %d more\n", len(m.podNames)-maxShow))
			break
		}
		b.WriteString(fmt.Sprintf("  • %s\n", name))
	}

	b.WriteString("\n")

	yes := "  Yes  "
	no := "  No  "
	if m.cursor == 0 {
		yes = styles.SelectedRow.Render(" [Yes] ")
		no = "  No  "
	} else {
		yes = "  Yes  "
		no = styles.SelectedRow.Render(" [No] ")
	}

	b.WriteString(fmt.Sprintf("    %s    %s", yes, no))

	return styles.OverlayBox.Render(b.String())
}
