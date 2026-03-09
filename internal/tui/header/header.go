package header

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// StatusSummary contains grouped pod health counts for header display.
type StatusSummary struct {
	CritCrash       int
	CritImgErr      int
	CritOOM         int
	CritEvicted     int
	WarnPending     int
	WarnTerminating int
	OKRunning       int
	OKCompleted     int
}

// Model represents the header component showing cluster context and namespace.
type Model struct {
	clusterName   string
	namespace     string
	filterActive  bool
	podCountLabel string
	statusSummary StatusSummary
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
			statusSummary: m.statusSummary,
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
		statusSummary: m.statusSummary,
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
		statusSummary: m.statusSummary,
		width:         m.width,
	}
}

// SetStatusSummary returns a new model with updated health summary counts.
func (m Model) SetStatusSummary(summary StatusSummary) Model {
	return Model{
		clusterName:   m.clusterName,
		namespace:     m.namespace,
		filterActive:  m.filterActive,
		podCountLabel: m.podCountLabel,
		statusSummary: summary,
		width:         m.width,
	}
}

// View renders the header.
func (m Model) View() string {
	nsLabel := m.namespace
	if nsLabel == "" {
		nsLabel = "All Namespaces"
	}

	content := fmt.Sprintf("%s %s  │  %s %s",
		styles.LabelText.Render("Cluster:"),
		m.clusterName,
		styles.LabelText.Render("Namespace:"),
		nsLabel,
	)

	summary := m.renderSummary()
	if summary != "" && !m.filterActive {
		content += "  │  " + summary
	}

	if m.filterActive {
		content += "  │  " + styles.FilterBadge.Render(" FILTERED ")
		if m.podCountLabel != "" {
			content += "  " + m.podCountLabel
		}
	}
	if summary != "" && m.filterActive {
		content += "  │  " + summary
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

func (m Model) renderSummary() string {
	var parts []string

	var critItems []string
	if m.statusSummary.CritCrash > 0 {
		critItems = append(critItems, fmt.Sprintf("%d Crash", m.statusSummary.CritCrash))
	}
	if m.statusSummary.CritImgErr > 0 {
		critItems = append(critItems, fmt.Sprintf("%d ImgErr", m.statusSummary.CritImgErr))
	}
	if m.statusSummary.CritOOM > 0 {
		critItems = append(critItems, fmt.Sprintf("%d OOM", m.statusSummary.CritOOM))
	}
	if m.statusSummary.CritEvicted > 0 {
		critItems = append(critItems, fmt.Sprintf("%d Evict", m.statusSummary.CritEvicted))
	}
	if len(critItems) > 0 {
		parts = append(parts, styles.CritSummary.Render("Crit: "+strings.Join(critItems, ", ")))
	}

	var warnItems []string
	if m.statusSummary.WarnPending > 0 {
		warnItems = append(warnItems, fmt.Sprintf("%d Pend", m.statusSummary.WarnPending))
	}
	if m.statusSummary.WarnTerminating > 0 {
		warnItems = append(warnItems, fmt.Sprintf("%d Term", m.statusSummary.WarnTerminating))
	}
	if len(warnItems) > 0 {
		parts = append(parts, styles.WarnSummary.Render("Warn: "+strings.Join(warnItems, ", ")))
	}

	var okItems []string
	if m.statusSummary.OKRunning > 0 {
		okItems = append(okItems, fmt.Sprintf("%d Run", m.statusSummary.OKRunning))
	}
	if m.statusSummary.OKCompleted > 0 {
		okItems = append(okItems, fmt.Sprintf("%d Comp", m.statusSummary.OKCompleted))
	}
	if len(okItems) > 0 {
		parts = append(parts, styles.OKSummary.Render("OK: "+strings.Join(okItems, ", ")))
	}

	return strings.Join(parts, " | ")
}
