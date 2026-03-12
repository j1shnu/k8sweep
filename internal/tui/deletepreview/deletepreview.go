package deletepreview

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/common"
	"github.com/jprasad/k8sweep/internal/tui/podlist"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// Model represents the delete preview/confirmation screen.
type Model struct {
	pods       []k8s.PodInfo
	warnings   []string
	actionType common.DeleteAction
	scroll     int // scroll offset for pod list
	cursor     int // 0 = Yes, 1 = No
	confirmed  bool
	cancelled  bool
	width      int
	height     int
}

// New creates a delete preview model.
func New(pods []k8s.PodInfo, action common.DeleteAction, warnings []string) Model {
	return Model{
		pods:       pods,
		warnings:   warnings,
		actionType: action,
		cursor:     1, // default to No for safety
	}
}

// SetSize returns a new Model with the given dimensions.
func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h
	m.scroll = clampScroll(m.scroll, m.maxScroll())
	return m
}

// Update handles key events for the delete preview screen.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			newModel := m
			max := newModel.maxScroll()
			if newModel.scroll < max {
				newModel.scroll++
			}
			return newModel, nil
		case "k", "up":
			newModel := m
			if newModel.scroll > 0 {
				newModel.scroll--
			}
			return newModel, nil
		case "left", "right", "h", "l", "tab":
			newModel := m
			newModel.cursor = 1 - m.cursor
			return newModel, nil
		case "enter":
			newModel := m
			newModel.confirmed = m.cursor == 0
			newModel.cancelled = m.cursor == 1
			return newModel, nil
		case "y":
			newModel := m
			newModel.cursor = 0
			newModel.confirmed = true
			return newModel, nil
		case "esc", "n":
			newModel := m
			newModel.cancelled = true
			return newModel, nil
		}
	}
	return m, nil
}

// IsConfirmed returns true if the user confirmed the deletion.
func (m Model) IsConfirmed() bool { return m.confirmed }

// IsCancelled returns true if the user cancelled.
func (m Model) IsCancelled() bool { return m.cancelled }

// Action returns the action type.
func (m Model) Action() common.DeleteAction { return m.actionType }

// Pods returns a copy of the pods being previewed.
func (m Model) Pods() []k8s.PodInfo { return append([]k8s.PodInfo(nil), m.pods...) }

// View renders the delete preview screen.
func (m Model) View() string {
	var b strings.Builder

	// Header with action type
	count := len(m.pods)
	if m.actionType == common.DeleteForce {
		b.WriteString(styles.ErrorMessage.Render(
			fmt.Sprintf("⚠  FORCE DELETE — %d pod(s) selected", count)))
		b.WriteString("\n")
		b.WriteString(styles.ErrorMessage.Render(
			"   Bypasses graceful shutdown (GracePeriodSeconds=0)"))
	} else {
		b.WriteString(styles.Title.Render(
			fmt.Sprintf("Delete Preview — %d pod(s) selected", count)))
	}
	b.WriteString("\n\n")

	// Column headers
	nsCol := 20
	nameCol := 35
	statusCol := 18
	ageCol := 8
	headerLine := fmt.Sprintf("  %-*s %-*s %-*s %-*s",
		nsCol, "NAMESPACE", nameCol, "NAME", statusCol, "STATUS", ageCol, "AGE")
	b.WriteString(styles.FooterHelp.Render(headerLine))
	b.WriteString("\n")

	// Pod rows — scrollable
	visibleRows := m.visibleRowCount()
	start := m.scroll
	end := start + visibleRows
	if end > len(m.pods) {
		end = len(m.pods)
	}

	for i := start; i < end; i++ {
		p := m.pods[i]
		ns := truncate(p.Namespace, nsCol)
		name := truncate(p.Name, nameCol)
		status := string(p.Status)
		age := podlist.FormatAge(p.Age)

		styledStatus := styles.StyleForStatus(status).Render(fmt.Sprintf("%-*s", statusCol, truncate(status, statusCol)))
		row := fmt.Sprintf("  %-*s %-*s %s %-*s", nsCol, ns, nameCol, name, styledStatus, ageCol, age)
		b.WriteString(row)
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.pods) > visibleRows {
		shown := fmt.Sprintf("  showing %d-%d of %d (j/k to scroll)", start+1, end, len(m.pods))
		b.WriteString(styles.FooterHelp.Render(shown))
		b.WriteString("\n")
	}

	// Warnings
	if len(m.warnings) > 0 {
		b.WriteString("\n")
		b.WriteString(styles.StatusMessage.Render("Warnings:"))
		b.WriteString("\n")
		for _, w := range m.warnings {
			fmt.Fprintf(&b, "  ⚠ %s\n", w)
		}
	}

	// Yes/No buttons with color coding
	b.WriteString("\n")
	var yes, no string
	if m.cursor == 0 {
		yes = styles.ButtonDanger.Render(" [Yes] ")
		no = styles.ButtonSafeDim.Render("  No  ")
	} else {
		yes = styles.ButtonDangerDim.Render("  Yes  ")
		no = styles.ButtonSafe.Render(" [No] ")
	}
	fmt.Fprintf(&b, "    %s    %s", yes, no)
	b.WriteString("\n")
	b.WriteString(styles.FooterHelp.Render("  [y] confirm  [n/esc] cancel  [←/→] toggle"))

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.borderColor()).
		Padding(1, 2)
	if m.width > 0 {
		boxStyle = boxStyle.Width(m.width - 4)
	}

	return boxStyle.Render(b.String())
}

func (m Model) borderColor() lipgloss.Color {
	if m.actionType == common.DeleteForce {
		return lipgloss.Color("#FF0000")
	}
	return styles.ColorAccent
}

func (m Model) visibleRowCount() int {
	// Reserve lines for header (3), column header (1), scroll hint (1),
	// warnings (~3), buttons (3), padding/border (~4)
	overhead := 15 + len(m.warnings)
	avail := m.height - overhead
	if avail < 3 {
		avail = 3
	}
	if avail > len(m.pods) {
		avail = len(m.pods)
	}
	return avail
}

func (m Model) maxScroll() int {
	max := len(m.pods) - m.visibleRowCount()
	if max < 0 {
		return 0
	}
	return max
}

func clampScroll(scroll, max int) int {
	if scroll < 0 {
		return 0
	}
	if scroll > max {
		return max
	}
	return scroll
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
