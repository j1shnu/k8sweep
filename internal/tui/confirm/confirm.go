package confirm

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// ActionType distinguishes normal delete from force delete.
type ActionType int

const (
	ActionDelete ActionType = iota
	ActionForceDelete
)

// Model represents the deletion confirmation overlay.
type Model struct {
	podNames   []string
	warnings   []string // e.g., "pod-x is standalone (no controller)"
	actionType ActionType
	cursor     int // 0 = Yes, 1 = No
	confirmed  bool
	cancelled  bool
	width      int
}

// New creates a new confirmation overlay for the given pod names.
func New(podNames []string) Model {
	return Model{
		podNames: podNames,
		cursor:   1, // Default to "No" for safety
	}
}

// NewWithAction creates a confirmation overlay with action type and warnings.
func NewWithAction(podNames []string, action ActionType, warnings []string) Model {
	return Model{
		podNames:   podNames,
		warnings:   warnings,
		actionType: action,
		cursor:     1,
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
				podNames:   m.podNames,
				warnings:   m.warnings,
				actionType: m.actionType,
				cursor:     newCursor,
				confirmed:  m.confirmed,
				cancelled:  m.cancelled,
				width:      m.width,
			}, nil

		case "enter":
			return Model{
				podNames:   m.podNames,
				warnings:   m.warnings,
				actionType: m.actionType,
				cursor:     m.cursor,
				confirmed:  m.cursor == 0,
				cancelled:  m.cursor == 1,
				width:      m.width,
			}, nil

		case "y":
			return Model{
				podNames:   m.podNames,
				warnings:   m.warnings,
				actionType: m.actionType,
				cursor:     0,
				confirmed:  true,
				cancelled:  false,
				width:      m.width,
			}, nil

		case "esc", "n":
			return Model{
				podNames:   m.podNames,
				warnings:   m.warnings,
				actionType: m.actionType,
				cursor:     m.cursor,
				confirmed:  false,
				cancelled:  true,
				width:      m.width,
			}, nil
		}
	}
	return m, nil
}

// IsConfirmed returns true if the user confirmed deletion.
func (m Model) IsConfirmed() bool { return m.confirmed }

// IsCancelled returns true if the user cancelled.
func (m Model) IsCancelled() bool { return m.cancelled }

// ActionType returns the action type for this confirmation.
func (m Model) Action() ActionType { return m.actionType }

// View renders the confirmation overlay.
func (m Model) View() string {
	var b strings.Builder

	if m.actionType == ActionForceDelete {
		b.WriteString(styles.ErrorMessage.Render("FORCE delete these pods? (bypasses graceful shutdown)"))
	} else {
		b.WriteString(styles.ErrorMessage.Render("Delete these pods?"))
	}
	b.WriteString("\n\n")

	maxShow := 10
	for i, name := range m.podNames {
		if i >= maxShow {
			fmt.Fprintf(&b, "  ... and %d more\n", len(m.podNames)-maxShow)
			break
		}
		fmt.Fprintf(&b, "  • %s\n", name)
	}

	if len(m.warnings) > 0 {
		b.WriteString("\n")
		b.WriteString(styles.StatusMessage.Render("Warnings:"))
		b.WriteString("\n")
		for _, w := range m.warnings {
			fmt.Fprintf(&b, "  ⚠ %s\n", w)
		}
	}

	b.WriteString("\n")

	yes, no := "  Yes  ", "  No  "
	if m.cursor == 0 {
		yes = styles.SelectedRow.Render(" [Yes] ")
	} else {
		no = styles.SelectedRow.Render(" [No] ")
	}

	fmt.Fprintf(&b, "    %s    %s", yes, no)

	return styles.OverlayBox.Render(b.String())
}
