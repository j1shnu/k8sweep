package deletesummary

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/common"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// Model represents the post-delete summary screen.
type Model struct {
	results    []k8s.DeleteResult
	actionType common.DeleteAction
	scroll     int
	width      int
	height     int
	dismissed  bool
}

// New creates a delete summary model from results.
func New(results []k8s.DeleteResult, action common.DeleteAction) Model {
	return Model{
		results:    results,
		actionType: action,
	}
}

// SetSize returns a new Model with the given dimensions.
func (m Model) SetSize(w, h int) Model {
	m.width = w
	m.height = h
	m.scroll = clampScroll(m.scroll, m.maxScroll())
	return m
}

// Update handles key events for the summary screen.
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
		case "enter", "esc":
			newModel := m
			newModel.dismissed = true
			return newModel, nil
		}
	}
	return m, nil
}

// IsDismissed returns true if the user acknowledged the summary.
func (m Model) IsDismissed() bool { return m.dismissed }

// View renders the delete summary screen.
func (m Model) View() string {
	var b strings.Builder

	successCount, failCount := m.counts()
	total := len(m.results)

	// Header
	actionLabel := "Delete"
	if m.actionType == common.DeleteForce {
		actionLabel = "Force Delete"
	}
	b.WriteString(styles.Title.Render(fmt.Sprintf("%s Summary", actionLabel)))
	b.WriteString("\n\n")

	// Counts
	b.WriteString(fmt.Sprintf("  Total:     %d\n", total))
	if successCount > 0 {
		b.WriteString(styles.OKSummary.Render(fmt.Sprintf("  Succeeded: %d", successCount)))
		b.WriteString("\n")
	}
	if failCount > 0 {
		b.WriteString(styles.CritSummary.Render(fmt.Sprintf("  Failed:    %d", failCount)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Result rows — scrollable
	visibleRows := m.visibleRowCount()
	start := m.scroll
	end := start + visibleRows
	if end > total {
		end = total
	}

	nsCol := 20
	nameCol := 35
	for i := start; i < end; i++ {
		r := m.results[i]
		ns := truncate(r.Namespace, nsCol)
		name := truncate(r.PodName, nameCol)
		if r.Success {
			icon := styles.OKSummary.Render("✓")
			row := fmt.Sprintf("  %s %-*s %-*s", icon, nsCol, ns, nameCol, name)
			b.WriteString(row)
		} else {
			icon := styles.CritSummary.Render("✗")
			errMsg := "unknown error"
			if r.Error != nil {
				errMsg = r.Error.Error()
			}
			row := fmt.Sprintf("  %s %-*s %-*s", icon, nsCol, ns, nameCol, name)
			b.WriteString(row)
			b.WriteString("\n")
			b.WriteString(styles.ErrorMessage.Render(fmt.Sprintf("    → %s", truncate(errMsg, 70))))
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if total > visibleRows {
		shown := fmt.Sprintf("  showing %d-%d of %d (j/k to scroll)", start+1, end, total)
		b.WriteString(styles.FooterHelp.Render(shown))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.FooterHelp.Render("  Press [enter] or [esc] to return"))

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.borderColor()).
		Padding(1, 2)
	if m.width > 0 {
		boxStyle = boxStyle.Width(m.width - 4)
	}

	return boxStyle.Render(b.String())
}

func (m Model) counts() (success, fail int) {
	for _, r := range m.results {
		if r.Success {
			success++
		} else {
			fail++
		}
	}
	return
}

func (m Model) borderColor() lipgloss.Color {
	_, failCount := m.counts()
	if failCount > 0 {
		return lipgloss.Color("#FF6666")
	}
	return styles.ColorAccent
}

func (m Model) visibleRowCount() int {
	// Reserve lines for header (2), counts (~4), scroll hint (1), footer (2), padding/border (~4)
	overhead := 13
	avail := m.height - overhead
	if avail < 3 {
		avail = 3
	}
	if avail > len(m.results) {
		avail = len(m.results)
	}
	return avail
}

func (m Model) maxScroll() int {
	max := len(m.results) - m.visibleRowCount()
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
