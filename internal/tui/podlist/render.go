package podlist

import (
	"fmt"
	"strings"
	"time"

	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// View renders the pod list as a tree grouped by controller.
func (m Model) View() string {
	if m.loading {
		spinner := styles.LoadingSpinner.Render(spinnerFrames[m.spinnerFrame])
		prefix := styles.LoadingPrefix.Render(" Fetching pods...")
		fact := styles.LoadingFact.Render(" " + loadingFacts[m.factIndex])
		return fmt.Sprintf("  %s%s\n  %s", spinner, prefix, fact)
	}
	if len(m.displayRows) == 0 {
		return styles.FooterHelp.Render("  No pods found.")
	}

	var b strings.Builder
	b.WriteString(m.renderHeaderRow())
	b.WriteString("\n")

	start, end := m.currentPageBounds()
	for i := start; i < end; i++ {
		row := m.displayRows[i]
		isCursor := i == m.cursor

		switch row.Kind {
		case RowController:
			b.WriteString(m.renderControllerRow(row, isCursor))
		case RowPod:
			b.WriteString(m.renderPodRow(*row.Pod, isCursor))
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	page := m.currentPage() + 1
	totalPages := m.totalPages()
	if totalPages > 1 {
		showStart := start + 1
		showEnd := end
		pager := fmt.Sprintf("Showing %d-%d of %d rows [%s] | %s/%s next | %s/%s previous",
			showStart, showEnd, len(m.displayRows),
			styles.LabelText.Render(fmt.Sprintf("page %d/%d", page, totalPages)),
			styles.LabelText.Render("[l]"),
			styles.LabelText.Render("[→]"),
			styles.LabelText.Render("[h]"),
			styles.LabelText.Render("[←]"),
		)
		b.WriteString("\n")
		b.WriteString(styles.FooterHelp.Render("  " + pager))
	}

	return b.String()
}

// renderControllerRow renders a controller group header row.
func (m Model) renderControllerRow(row DisplayRow, isCursor bool) string {
	g := row.Header

	indicator := "▼"
	if _, collapsed := m.collapsed[row.GroupKey]; collapsed {
		indicator = "▶"
	}

	// Selection state for all pods in this group
	selectedCount := 0
	for _, p := range g.Pods {
		if _, ok := m.selected[podKey(p)]; ok {
			selectedCount++
		}
	}
	checkbox := "[ ] "
	if selectedCount == len(g.Pods) && selectedCount > 0 {
		checkbox = styles.StyleForStatus("Running").Render("[✓] ")
	} else if selectedCount > 0 {
		checkbox = styles.StyleForStatus("Pending").Render("[-] ")
	}

	pointer := "  "
	if isCursor {
		pointer = styles.Pointer.Render("> ")
	}

	summary := buildGroupSummary(g)

	metricsStr := ""
	if m.metricsAvailable {
		var totalCPU, totalMem int64
		hasMetrics := false
		for _, p := range g.Pods {
			if p.Metrics != nil {
				totalCPU += p.Metrics.CPUMillicores
				totalMem += p.Metrics.MemoryBytes
				hasMetrics = true
			}
		}
		if hasMetrics {
			cpu := styles.LoadingPrefix.Render(formatCPU(totalCPU))
			mem := styles.LoadingPrefix.Render(formatMemory(totalMem))
			metricsStr = fmt.Sprintf("  cpu: %s  mem: %s", cpu, mem)
		} else {
			metricsStr = "  cpu: ---  mem: ---"
		}
	}

	labelStyle := styles.ControllerRow
	if groupHasDirtyPod(g) {
		labelStyle = styles.ControllerRowDirty
	}
	groupLabel := labelStyle.Render(indicator + " " + row.GroupKey)
	line := fmt.Sprintf("%s%s%s  (%s)%s", pointer, checkbox, groupLabel, summary, metricsStr)

	if isCursor {
		line = styles.SelectedRow.Render(line)
	}
	return line
}

// renderPodRow renders a single pod row with tree indentation.
func (m Model) renderPodRow(pod k8s.PodInfo, isCursor bool) string {
	_, isSelected := m.selected[podKey(pod)]

	pointer := "  "
	if isCursor {
		pointer = styles.Pointer.Render("> ")
	}

	checkbox := "[ ] "
	if isSelected {
		checkbox = styles.StyleForStatus("Running").Render("[✓] ")
	}

	statusStyle := styles.StyleForStatus(string(pod.Status))
	status := statusStyle.Render(fmt.Sprintf("%-16s", pod.Status))

	age := FormatAge(pod.Age)
	// Reduce name width by 2 to account for tree indent
	nameWidth := nameColWidth - 2
	name := smartTruncateMiddle(pod.Name, nameWidth)

	metricsStr := ""
	if m.metricsAvailable {
		if pod.Metrics != nil {
			cpu := styles.LoadingPrefix.Render(formatCPU(pod.Metrics.CPUMillicores))
			mem := styles.LoadingPrefix.Render(formatMemory(pod.Metrics.MemoryBytes))
			metricsStr = fmt.Sprintf("  cpu: %s  mem: %s", cpu, mem)
		} else {
			metricsStr = "  cpu: ---  mem: ---"
		}
	}

	// 2-space indent for tree structure
	indent := "  "

	var line string
	if m.showNamespace {
		line = fmt.Sprintf("%s%s%s%-*s %-*s %s  %s  restarts: %d%s",
			pointer, checkbox, indent, namespaceColWidth, pod.Namespace, nameWidth, name, status, age, pod.RestartCount, metricsStr)
	} else {
		line = fmt.Sprintf("%s%s%s%-*s %s  %s  restarts: %d%s",
			pointer, checkbox, indent, nameWidth, name, status, age, pod.RestartCount, metricsStr)
	}

	if isCursor {
		line = styles.SelectedRow.Render(line)
	}
	return line
}

// renderHeaderRow renders the column header with sort indicator.
func (m Model) renderHeaderRow() string {
	indicator := func(col SortColumn) string {
		if m.sortColumn == col {
			return " " + SortIndicator(m.sortOrder)
		}
		return ""
	}

	// Pad to match pointer(2) + checkbox(4) + indent(2) = 8 chars
	prefix := "        "
	nameWidth := nameColWidth - 2

	var header string
	if m.showNamespace {
		header = fmt.Sprintf("%s%-*s %-*s %-16s  %-5s  %-10s",
			prefix,
			namespaceColWidth,
			"NAMESPACE"+indicator(SortByName),
			nameWidth,
			"NAME"+indicator(SortByName),
			"STATUS"+indicator(SortByStatus),
			"AGE"+indicator(SortByAge),
			"RESTARTS"+indicator(SortByRestarts))
	} else {
		header = fmt.Sprintf("%s%-*s %-16s  %-5s  %-10s",
			prefix,
			nameWidth,
			"NAME"+indicator(SortByName),
			"STATUS"+indicator(SortByStatus),
			"AGE"+indicator(SortByAge),
			"RESTARTS"+indicator(SortByRestarts))
	}

	if m.metricsAvailable {
		header += fmt.Sprintf("  %-8s  %-8s",
			"CPU"+indicator(SortByCPU),
			"MEM"+indicator(SortByMemory))
	}

	return styles.LabelText.Render(header)
}

// buildGroupSummary returns a summary string like "3 pods: 2 Running, 1 CrashLoopBackOff"
// with each status count color-coded to match pod row status colors.
func buildGroupSummary(g *ControllerGroup) string {
	counts := g.StatusCounts()
	total := len(g.Pods)

	noun := "pods"
	if total == 1 {
		noun = "pod"
	}

	var parts []string
	for _, status := range []k8s.PodStatus{
		k8s.StatusRunning, k8s.StatusCrashLoopBack, k8s.StatusFailed,
		k8s.StatusOOMKilled, k8s.StatusEvicted, k8s.StatusPending,
		k8s.StatusCompleted, k8s.StatusTerminating, k8s.StatusImagePullErr,
		k8s.StatusUnknown,
	} {
		if c, ok := counts[status]; ok && c > 0 {
			label := fmt.Sprintf("%d %s", c, status)
			parts = append(parts, styles.StyleForStatus(string(status)).Render(label))
		}
	}

	if len(parts) == 0 {
		return fmt.Sprintf("%d %s", total, noun)
	}
	return fmt.Sprintf("%d %s: %s", total, noun, strings.Join(parts, ", "))
}

// groupHasDirtyPod returns true if any pod in the group is dirty.
func groupHasDirtyPod(g *ControllerGroup) bool {
	for _, p := range g.Pods {
		if p.IsDirty() {
			return true
		}
	}
	return false
}

// smartTruncateMiddle truncates a string in the middle with "..." if it exceeds width.
func smartTruncateMiddle(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}

	keep := width - 3
	left := keep/2 + keep%2
	right := keep / 2

	return string(runes[:left]) + "..." + string(runes[len(runes)-right:])
}

// formatCPU formats CPU millicores for display (e.g., "250m", "1.5").
func formatCPU(millicores int64) string {
	if millicores >= 1000 {
		cores := float64(millicores) / 1000.0
		if cores == float64(int64(cores)) {
			return fmt.Sprintf("%d", int64(cores))
		}
		return fmt.Sprintf("%.1f", cores)
	}
	return fmt.Sprintf("%dm", millicores)
}

// formatMemory formats bytes for display (e.g., "128Mi", "2.1Gi").
func formatMemory(bytes int64) string {
	const (
		ki = 1024
		mi = 1024 * ki
		gi = 1024 * mi
	)
	switch {
	case bytes >= gi:
		val := float64(bytes) / float64(gi)
		if val == float64(int64(val)) {
			return fmt.Sprintf("%dGi", int64(val))
		}
		return fmt.Sprintf("%.1fGi", val)
	case bytes >= mi:
		val := float64(bytes) / float64(mi)
		if val == float64(int64(val)) {
			return fmt.Sprintf("%dMi", int64(val))
		}
		return fmt.Sprintf("%.1fMi", val)
	case bytes >= ki:
		return fmt.Sprintf("%dKi", bytes/ki)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// FormatAge formats a duration as a human-readable age string.
func FormatAge(d time.Duration) string {
	hours := d.Hours()
	if hours >= 24 {
		return fmt.Sprintf("%dd", int(hours/24))
	}
	if hours >= 1 {
		return fmt.Sprintf("%dh", int(hours))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
