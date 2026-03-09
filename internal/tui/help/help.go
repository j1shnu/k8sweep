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
	height int
	scroll int
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

// SetSize returns a new Model with the given width/height.
func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h
	if m.scroll < 0 {
		m.scroll = 0
	}
	max := m.maxScroll()
	if m.scroll > max {
		m.scroll = max
	}
	return m
}

// ScrollUp scrolls help content up by one line.
func (m Model) ScrollUp() Model {
	if m.scroll <= 0 {
		return m
	}
	m.scroll--
	return m
}

// ScrollDown scrolls help content down by one line.
func (m Model) ScrollDown() Model {
	max := m.maxScroll()
	if m.scroll >= max {
		return m
	}
	m.scroll++
	return m
}

// View renders the help overlay.
func (m Model) View() string {
	lines := m.renderLines()
	start := 0
	end := len(lines)
	if m.height > 0 {
		contentHeight := m.height - 6 // account for box border/padding + fixed footer hint
		if contentHeight < 3 {
			contentHeight = 3
		}
		if m.scroll > 0 {
			start = m.scroll
		}
		end = start + contentHeight
		if end > len(lines) {
			end = len(lines)
		}
		if start > end {
			start = end
		}
	}

	var b strings.Builder
	b.WriteString(strings.Join(lines[start:end], "\n"))
	b.WriteString("\n\n")
	b.WriteString(styles.FooterHelp.Render("[j/k or ↑/↓ scroll, ?/esc close]"))

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorAccent).
		Padding(1, 2)

	if m.width > 0 {
		boxStyle = boxStyle.Width(m.width - 4)
	}

	return boxStyle.Render(b.String())
}

func (m Model) renderLines() []string {
	var lines []string

	lines = append(lines, styles.Title.Render("k8sweep — Keyboard Shortcuts"))
	for i, group := range m.groups {
		lines = append(lines, "")
		name := "Keys"
		if i < len(sectionNames) {
			name = sectionNames[i]
		}
		lines = append(lines, styles.Title.Render(name))
		for _, binding := range group {
			h := binding.Help()
			keyStr := styles.LabelText.Render("[" + h.Key + "]")
			desc := styles.FooterHelp.Render(h.Desc)
			lines = append(lines, "  "+keyStr+"  "+desc)
		}
		// gg is raw key logic, not a key.Binding — append to Navigation section
		if i == 0 {
			lines = append(lines, "  "+styles.LabelText.Render("[gg]")+"  "+styles.FooterHelp.Render("go to first pod"))
		}
	}

	lines = append(lines, "")
	lines = append(lines, styles.Title.Render("In Search"))
	lines = append(lines, "  "+styles.LabelText.Render("[type]")+"  "+styles.FooterHelp.Render("filter pods by name (real-time)"))
	lines = append(lines, "  "+styles.LabelText.Render("[enter]")+"  "+styles.FooterHelp.Render("confirm search filter"))
	lines = append(lines, "  "+styles.LabelText.Render("[esc]")+"  "+styles.FooterHelp.Render("cancel and clear search"))

	lines = append(lines, "")
	lines = append(lines, styles.Title.Render("In Confirm Dialog"))
	lines = append(lines, "  "+styles.LabelText.Render("[y/n]")+"  "+styles.FooterHelp.Render("confirm or cancel"))
	lines = append(lines, "  "+styles.LabelText.Render("[esc]")+"  "+styles.FooterHelp.Render("cancel"))
	lines = append(lines, "  "+styles.FooterHelp.Render("force delete (x) bypasses graceful shutdown"))
	lines = append(lines, "  "+styles.FooterHelp.Render("standalone pods (no controller) show a warning"))

	lines = append(lines, "")
	lines = append(lines, styles.Title.Render("In Pod Detail"))
	lines = append(lines, "  "+styles.LabelText.Render("[j/k]")+"  "+styles.FooterHelp.Render("scroll up/down"))
	lines = append(lines, "  "+styles.LabelText.Render("[i/esc]")+"  "+styles.FooterHelp.Render("close detail view"))

	lines = append(lines, "")
	lines = append(lines, styles.Title.Render("In Namespace Switcher"))
	lines = append(lines, "  "+styles.FooterHelp.Render("type to filter, [enter] select, [esc] cancel"))

	return lines
}

func (m Model) maxScroll() int {
	if m.height <= 0 {
		return 0
	}
	contentHeight := m.height - 6
	if contentHeight < 3 {
		contentHeight = 3
	}
	max := len(m.renderLines()) - contentHeight
	if max < 0 {
		return 0
	}
	return max
}
