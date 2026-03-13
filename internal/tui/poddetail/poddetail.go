package poddetail

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// State represents the overlay state.
type State int

const (
	StateHidden State = iota
	StateLoading
	StateReady
	StateError
)

// Subview represents which tab is active within the detail overlay.
type Subview int

const (
	SubviewDetail Subview = iota
	SubviewEvents
	SubviewLogs
)

// Model is the pod detail overlay component.
type Model struct {
	detail  *k8s.PodDetail
	errMsg  string
	state   State
	subview Subview
	scroll  int
	width   int
	height  int
	lines   []string // pre-rendered content lines

	// Events subview
	events    []k8s.PodEvent
	eventsErr string

	// Logs subview
	logLines     []string
	logContainer string
	logsErr      string
}

// New creates an empty hidden detail model.
func New() Model {
	return Model{state: StateHidden}
}

// SetLoading returns a new model in loading state.
func (m Model) SetLoading() Model {
	return Model{
		state:  StateLoading,
		width:  m.width,
		height: m.height,
	}
}

// SetDetail returns a new model displaying the given pod detail.
func (m Model) SetDetail(detail *k8s.PodDetail) Model {
	newModel := Model{
		detail: detail,
		state:  StateReady,
		scroll: 0,
		width:  m.width,
		height: m.height,
	}
	newModel.lines = newModel.renderLines()
	return newModel
}

// SetError returns a new model displaying an error.
func (m Model) SetError(msg string) Model {
	return Model{
		errMsg: msg,
		state:  StateError,
		width:  m.width,
		height: m.height,
	}
}

// SetSize returns a new model with updated dimensions.
func (m Model) SetSize(width, height int) Model {
	newModel := m
	newModel.width = width
	newModel.height = height
	return newModel
}

// Hide returns a hidden model.
func (m Model) Hide() Model {
	return Model{
		width:  m.width,
		height: m.height,
	}
}

// Subview returns the current subview.
func (m Model) Subview() Subview {
	return m.subview
}

// ShowDetail switches back to the detail subview.
func (m Model) ShowDetail() Model {
	newModel := m
	newModel.subview = SubviewDetail
	newModel.scroll = 0
	newModel.lines = newModel.renderLines()
	return newModel
}

// SetEventsLoading returns a model in the events subview with loading state.
func (m Model) SetEventsLoading() Model {
	newModel := m
	newModel.subview = SubviewEvents
	newModel.events = nil
	newModel.eventsErr = ""
	newModel.scroll = 0
	newModel.state = StateLoading
	newModel.lines = nil
	return newModel
}

// SetEvents returns a model displaying pod events.
func (m Model) SetEvents(events []k8s.PodEvent) Model {
	newModel := m
	newModel.subview = SubviewEvents
	newModel.events = events
	newModel.eventsErr = ""
	newModel.scroll = 0
	newModel.state = StateReady
	newModel.lines = newModel.renderLines()
	return newModel
}

// SetEventsError returns a model displaying an events error.
func (m Model) SetEventsError(msg string) Model {
	newModel := m
	newModel.subview = SubviewEvents
	newModel.eventsErr = msg
	newModel.scroll = 0
	newModel.state = StateError
	newModel.lines = nil
	return newModel
}

// SetLogsLoading returns a model in the logs subview with loading state.
func (m Model) SetLogsLoading() Model {
	newModel := m
	newModel.subview = SubviewLogs
	newModel.logLines = nil
	newModel.logContainer = ""
	newModel.logsErr = ""
	newModel.scroll = 0
	newModel.state = StateLoading
	newModel.lines = nil
	return newModel
}

// SetLogs returns a model displaying pod logs.
func (m Model) SetLogs(lines []string, container string) Model {
	newModel := m
	newModel.subview = SubviewLogs
	newModel.logLines = lines
	newModel.logContainer = container
	newModel.logsErr = ""
	newModel.scroll = 0
	newModel.state = StateReady
	newModel.lines = newModel.renderLines()
	return newModel
}

// SetLogsError returns a model displaying a logs error.
func (m Model) SetLogsError(msg string) Model {
	newModel := m
	newModel.subview = SubviewLogs
	newModel.logsErr = msg
	newModel.scroll = 0
	newModel.state = StateError
	newModel.lines = nil
	return newModel
}

// IsVisible returns true if the overlay is showing.
func (m Model) IsVisible() bool {
	return m.state != StateHidden
}

// ScrollUp scrolls the content up by one line.
func (m Model) ScrollUp() Model {
	if m.scroll <= 0 {
		return m
	}
	newModel := m
	newModel.scroll = m.scroll - 1
	return newModel
}

// ScrollDown scrolls the content down by one line.
func (m Model) ScrollDown() Model {
	maxScroll := m.maxScroll()
	if m.scroll >= maxScroll {
		return m
	}
	newModel := m
	newModel.scroll = m.scroll + 1
	return newModel
}

// ScrollToTop scrolls to the top of the content.
func (m Model) ScrollToTop() Model {
	if m.scroll == 0 {
		return m
	}
	newModel := m
	newModel.scroll = 0
	return newModel
}

// ScrollToBottom scrolls to the bottom of the content.
func (m Model) ScrollToBottom() Model {
	max := m.maxScroll()
	if m.scroll >= max {
		return m
	}
	newModel := m
	newModel.scroll = max
	return newModel
}

func (m Model) maxScroll() int {
	contentHeight := m.height - 6 // borders + padding + footer hint
	if contentHeight <= 0 {
		contentHeight = 10
	}
	max := len(m.lines) - contentHeight
	if max < 0 {
		return 0
	}
	return max
}

// View renders the overlay.
func (m Model) View() string {
	var content string

	switch m.state {
	case StateHidden:
		return ""
	case StateLoading:
		label := " Loading pod details..."
		switch m.subview {
		case SubviewEvents:
			label = " Loading events..."
		case SubviewLogs:
			label = " Loading logs..."
		}
		content = styles.LoadingSpinner.Render("⠹") + styles.LoadingPrefix.Render(label)
	case StateError:
		errMsg := m.errMsg
		switch m.subview {
		case SubviewEvents:
			errMsg = m.eventsErr
		case SubviewLogs:
			errMsg = m.logsErr
		}
		content = styles.ErrorMessage.Render("Error: " + errMsg)
	case StateReady:
		contentHeight := m.height - 6
		if contentHeight <= 0 {
			contentHeight = 10
		}

		end := m.scroll + contentHeight
		if end > len(m.lines) {
			end = len(m.lines)
		}
		start := m.scroll
		if start > end {
			start = end
		}

		visible := m.lines[start:end]
		content = strings.Join(visible, "\n")
	}

	footer := m.footerHint()
	content += "\n\n" + footer

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorAccent).
		Padding(1, 2)

	if m.width > 0 {
		boxStyle = boxStyle.Width(m.width - 4)
	}

	return boxStyle.Render(content)
}

func (m Model) footerHint() string {
	k := func(s string) string { return styles.LabelText.Render("[" + s + "]") }
	d := func(s string) string { return styles.FooterHelp.Render(s) }
	sep := "  "

	switch m.subview {
	case SubviewEvents, SubviewLogs:
		return k("j/k") + d(" scroll") + sep +
			k("gg") + d(" top") + sep +
			k("G") + d(" bottom") + sep +
			k("esc") + d(" back to detail")
	default:
		return k("j/k") + d(" scroll") + sep +
			k("gg") + d(" top") + sep +
			k("G") + d(" bottom") + sep +
			k("v") + d(" events") + sep +
			k("o") + d(" logs") + sep +
			k("e") + d(" shell") + sep +
			k("i/esc") + d(" close")
	}
}

// renderLines pre-renders content into lines for scrolling based on the active subview.
func (m Model) renderLines() []string {
	switch m.subview {
	case SubviewEvents:
		return m.renderEventLines()
	case SubviewLogs:
		return m.renderLogLines()
	default:
		return m.renderDetailLines()
	}
}

func (m Model) renderDetailLines() []string {
	d := m.detail
	if d == nil {
		return nil
	}

	var lines []string

	title := styles.Title.Render("Pod Detail: " + d.Name)
	lines = append(lines, title)
	lines = append(lines, strings.Repeat("─", 50))

	lines = append(lines, field("Namespace", d.Namespace))
	lines = append(lines, field("Status", string(d.Status)))
	lines = append(lines, field("Node", d.Node))
	lines = append(lines, field("Age", formatDetailAge(d.Age)))
	lines = append(lines, field("Pod IP", d.PodIP))
	lines = append(lines, field("Host IP", d.HostIP))
	lines = append(lines, field("QoS Class", d.QoSClass))
	if d.ResolvedController != "" {
		lines = append(lines, field("Controller", d.ResolvedController))
	}
	if d.Owner != "" {
		lines = append(lines, field("Owner", d.Owner))
	}

	if len(d.Labels) > 0 {
		lines = append(lines, "")
		lines = append(lines, styles.Title.Render("Labels:"))
		keys := sortedKeys(d.Labels)
		for _, k := range keys {
			lines = append(lines, "  "+styles.LoadingPrefix.Render(k)+"="+d.Labels[k])
		}
	}

	if len(d.Annotations) > 0 {
		lines = append(lines, "")
		lines = append(lines, styles.Title.Render("Annotations:"))
		keys := sortedKeys(d.Annotations)
		for _, k := range keys {
			v := d.Annotations[k]
			if len(v) > 60 {
				v = v[:57] + "..."
			}
			lines = append(lines, "  "+styles.LoadingPrefix.Render(k)+"="+v)
		}
	}

	if len(d.Containers) > 0 {
		lines = append(lines, "")
		lines = append(lines, styles.Title.Render("Containers:"))
		for i, c := range d.Containers {
			portsStr := ""
			if len(c.Ports) > 0 {
				parts := make([]string, len(c.Ports))
				for j, p := range c.Ports {
					parts[j] = fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol)
				}
				portsStr = " (ports: " + strings.Join(parts, ", ") + ")"
			}
			lines = append(lines, fmt.Sprintf("  [%d] %s | image: %s%s",
				i+1, styles.LoadingPrefix.Render(c.Name), styles.LoadingPrefix.Render(c.Image), portsStr))

			readyStr := "no"
			if c.Ready {
				readyStr = "yes"
			}
			lines = append(lines, fmt.Sprintf("      State: %s | Ready: %s", c.State, readyStr))
			lines = append(lines, fmt.Sprintf("      Restarts: %d", c.RestartCount))

			if c.Requests.CPU != "" || c.Requests.Memory != "" {
				lines = append(lines, fmt.Sprintf("      Requests: cpu=%s, mem=%s", c.Requests.CPU, c.Requests.Memory))
			}
			if c.Limits.CPU != "" || c.Limits.Memory != "" {
				lines = append(lines, fmt.Sprintf("      Limits:   cpu=%s, mem=%s", c.Limits.CPU, c.Limits.Memory))
			}
		}
	}

	if len(d.Conditions) > 0 {
		lines = append(lines, "")
		lines = append(lines, styles.Title.Render("Conditions:"))
		for _, c := range d.Conditions {
			line := fmt.Sprintf("  %s=%s", c.Type, c.Status)
			if c.Reason != "" {
				line += " (" + c.Reason + ")"
			}
			lines = append(lines, line)
		}
	}

	return lines
}

func field(label, value string) string {
	return fmt.Sprintf("  %-12s %s", label+":", value)
}

func formatDetailAge(d interface{ Hours() float64 }) string {
	hours := d.Hours()
	if hours >= 24 {
		return fmt.Sprintf("%dd", int(hours/24))
	}
	if hours >= 1 {
		return fmt.Sprintf("%dh", int(hours))
	}
	return fmt.Sprintf("%dm", int(hours*60))
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (m Model) renderEventLines() []string {
	podName := ""
	if m.detail != nil {
		podName = m.detail.Name
	}

	var lines []string
	lines = append(lines, styles.Title.Render("Events: "+podName))
	lines = append(lines, strings.Repeat("─", 50))

	if len(m.events) == 0 {
		lines = append(lines, "")
		lines = append(lines, styles.FooterHelp.Render("  No events found for this pod."))
		return lines
	}

	for _, e := range m.events {
		typeStyle := styles.FooterHelp
		if e.Type == "Warning" {
			typeStyle = lipgloss.NewStyle().Foreground(styles.ColorEvicted)
		}

		age := formatEventAge(e.LastTimestamp)
		countStr := ""
		if e.Count > 1 {
			countStr = fmt.Sprintf(" (x%d)", e.Count)
		}

		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %s  %s  %s%s",
			typeStyle.Render(e.Type),
			styles.LoadingPrefix.Render(e.Reason),
			styles.FooterHelp.Render(age),
			countStr,
		))
		// Wrap long messages
		msg := e.Message
		if len(msg) > 80 {
			msg = msg[:77] + "..."
		}
		lines = append(lines, "    "+msg)
		if e.Source != "" {
			lines = append(lines, "    "+styles.FooterHelp.Render("source: "+e.Source))
		}
	}

	return lines
}

func (m Model) renderLogLines() []string {
	podName := ""
	if m.detail != nil {
		podName = m.detail.Name
	}

	title := "Logs: " + podName
	if m.logContainer != "" {
		title += "/" + m.logContainer
	}
	title += " (last 100 lines)"

	var lines []string
	lines = append(lines, styles.Title.Render(title))
	lines = append(lines, strings.Repeat("─", 50))

	if len(m.logLines) == 0 {
		lines = append(lines, "")
		lines = append(lines, styles.FooterHelp.Render("  No logs available."))
		return lines
	}

	for _, l := range m.logLines {
		lines = append(lines, "  "+l)
	}

	return lines
}

func formatEventAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	hours := d.Hours()
	if hours >= 24 {
		return fmt.Sprintf("%dd ago", int(hours/24))
	}
	if hours >= 1 {
		return fmt.Sprintf("%dh ago", int(hours))
	}
	mins := int(d.Minutes())
	if mins >= 1 {
		return fmt.Sprintf("%dm ago", mins)
	}
	return fmt.Sprintf("%ds ago", int(d.Seconds()))
}
