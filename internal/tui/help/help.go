package help

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// Section group names corresponding to FullHelp() groups.
var sectionNames = []string{"Navigation", "Actions", "Other"}

// Model renders a full help overlay with grouped keybindings.
type Model struct {
	groups [][]key.Binding
	width  int
}

// New creates a help model from keybinding groups.
func New(groups [][]key.Binding) Model {
	return Model{groups: groups}
}

// SetWidth returns a new Model with the given width.
func (m Model) SetWidth(w int) Model {
	m.width = w
	return m
}

// View renders the help overlay.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("k8sweep — Keyboard Shortcuts"))
	b.WriteString("\n")

	for i, group := range m.groups {
		b.WriteString("\n")
		name := "Keys"
		if i < len(sectionNames) {
			name = sectionNames[i]
		}
		b.WriteString(styles.Title.Render(name))
		b.WriteString("\n")
		for _, binding := range group {
			h := binding.Help()
			keyStr := styles.LoadingPrefix.Render("[" + h.Key + "]")
			desc := styles.FooterHelp.Render(h.Desc)
			b.WriteString("  " + keyStr + "  " + desc + "\n")
		}
		// gg is raw key logic, not a key.Binding — append to Navigation section
		if i == 0 {
			b.WriteString("  " + styles.LoadingPrefix.Render("[gg]") + "  " + styles.FooterHelp.Render("go to first pod") + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.Title.Render("In Search"))
	b.WriteString("\n")
	b.WriteString("  " + styles.LoadingPrefix.Render("[type]") + "  " + styles.FooterHelp.Render("filter pods by name (real-time)") + "\n")
	b.WriteString("  " + styles.LoadingPrefix.Render("[enter]") + "  " + styles.FooterHelp.Render("confirm search filter") + "\n")
	b.WriteString("  " + styles.LoadingPrefix.Render("[esc]") + "  " + styles.FooterHelp.Render("cancel and clear search") + "\n")

	b.WriteString("\n")
	b.WriteString(styles.Title.Render("In Confirm Dialog"))
	b.WriteString("\n")
	b.WriteString("  " + styles.LoadingPrefix.Render("[y/n]") + "  " + styles.FooterHelp.Render("confirm or cancel") + "\n")
	b.WriteString("  " + styles.LoadingPrefix.Render("[esc]") + "  " + styles.FooterHelp.Render("cancel") + "\n")
	b.WriteString("  " + styles.FooterHelp.Render("force delete (x) bypasses graceful shutdown") + "\n")
	b.WriteString("  " + styles.FooterHelp.Render("standalone pods (no controller) show a warning") + "\n")

	b.WriteString("\n")
	b.WriteString(styles.Title.Render("In Pod Detail"))
	b.WriteString("\n")
	b.WriteString("  " + styles.LoadingPrefix.Render("[j/k]") + "  " + styles.FooterHelp.Render("scroll up/down") + "\n")
	b.WriteString("  " + styles.LoadingPrefix.Render("[i/esc]") + "  " + styles.FooterHelp.Render("close detail view") + "\n")

	b.WriteString("\n")
	b.WriteString(styles.Title.Render("In Namespace Switcher"))
	b.WriteString("\n")
	b.WriteString("  " + styles.FooterHelp.Render("type to filter, [enter] select, [esc] cancel") + "\n")

	b.WriteString("\n")
	b.WriteString(styles.FooterHelp.Render("Press ? to close"))

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorAccent).
		Padding(1, 2)

	if m.width > 0 {
		boxStyle = boxStyle.Width(m.width - 4)
	}

	return boxStyle.Render(b.String())
}
