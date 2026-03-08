package podlist

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// spinnerFrames are the animation frames for the loading spinner.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// loadingFacts are fun facts shown while waiting for pods to load.
var loadingFacts = []string{
	"Did you know? A pod of whales can have up to 1,000 members.",
	"The name 'Kubernetes' comes from Greek, meaning 'helmsman'.",
	"A container in the wild can run for years without a restart. Yours won't.",
	"The first Kubernetes commit was on June 6, 2014.",
	"Fun fact: etcd stores your entire cluster state in a single Raft log.",
	"Pods are ephemeral. Like this loading message.",
	"There are mass bird die-offs called 'pod events' too. Unrelated, probably.",
	"The average pod lives shorter than a mayfly in production.",
	"kubectl was almost named 'kubecfg'. Dodged a bullet there.",
	"CrashLoopBackOff is just your pod taking a power nap.",
	"Somewhere, a DevOps engineer is also waiting for pods right now.",
	"Your cluster has more YAML than a library has books.",
	"OOMKilled: when your pod's eyes are bigger than its memory limits.",
	"'It works on my machine' is why we have containers.",
	"The 'k' in k8s stands for... well, 'k'.",
}

func randomFactIndex() int {
	return rand.Intn(len(loadingFacts))
}

// factRotateInterval is the number of loading ticks before rotating the fact.
// At 80ms per tick, 125 ticks = 10 seconds per fact.
const factRotateInterval = 125

// Model represents the interactive pod list component.
type Model struct {
	items            []k8s.PodInfo
	cursor           int
	selected         map[string]struct{} // key: "namespace/name"
	width            int
	height           int
	offset           int  // viewport scroll offset
	loading          bool
	spinnerFrame     int  // current spinner animation frame index
	factIndex        int  // current fact message index
	factTicks        int  // ticks elapsed since last fact rotation
	showNamespace    bool // show namespace column (all-namespaces mode)
	metricsAvailable bool // show CPU/memory columns when metrics API is available
}

// New creates an empty pod list model.
func New() Model {
	return Model{
		selected: make(map[string]struct{}),
		loading:  true,
		factIndex: randomFactIndex(),
	}
}

// Len returns the number of items in the list.
func (m Model) Len() int {
	return len(m.items)
}

// IsLoading returns whether the model is in the loading state.
func (m Model) IsLoading() bool {
	return m.loading
}

// SetLoading returns a new model in the loading state with a random fun fact.
func (m Model) SetLoading() Model {
	return Model{
		items:            m.items,
		cursor:           m.cursor,
		selected:         m.selected,
		width:            m.width,
		height:           m.height,
		offset:           m.offset,
		loading:          true,
		spinnerFrame:     0,
		factIndex:        randomFactIndex(),
		showNamespace:    m.showNamespace,
		metricsAvailable: m.metricsAvailable,
	}
}

// TickLoading advances the spinner frame and rotates the fact message every
// factRotateInterval ticks (~10s). Returns a new model (only meaningful when loading).
func (m Model) TickLoading() Model {
	if !m.loading {
		return m
	}
	newModel := m
	newModel.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
	newModel.factTicks = m.factTicks + 1
	if newModel.factTicks >= factRotateInterval {
		newModel.factIndex = (m.factIndex + 1) % len(loadingFacts)
		newModel.factTicks = 0
	}
	return newModel
}

// SetShowNamespace returns a new model that shows or hides the namespace column.
func (m Model) SetShowNamespace(show bool) Model {
	newModel := m
	newModel.showNamespace = show
	return newModel
}

// SetMetricsAvailable returns a new model that shows or hides the metrics columns.
func (m Model) SetMetricsAvailable(available bool) Model {
	newModel := m
	newModel.metricsAvailable = available
	return newModel
}

// SetItems returns a new model with the given pods, resetting cursor and selection.
func (m Model) SetItems(pods []k8s.PodInfo) Model {
	return Model{
		items:            pods,
		cursor:           0,
		selected:         make(map[string]struct{}),
		width:            m.width,
		height:           m.height,
		offset:           0,
		loading:          false,
		showNamespace:    m.showNamespace,
		metricsAvailable: m.metricsAvailable,
	}
}

// SetSize returns a new model with the updated dimensions.
func (m Model) SetSize(width, height int) Model {
	return Model{
		items:            m.items,
		cursor:           m.cursor,
		selected:         m.selected,
		width:            width,
		height:           height,
		offset:           m.offset,
		loading:          m.loading,
		spinnerFrame:     m.spinnerFrame,
		factIndex:        m.factIndex,
		showNamespace:    m.showNamespace,
		metricsAvailable: m.metricsAvailable,
	}
}

func podKey(p k8s.PodInfo) string {
	return p.Namespace + "/" + p.Name
}

// ToggleSelect toggles selection of the pod at the current cursor.
func (m Model) ToggleSelect() Model {
	if len(m.items) == 0 {
		return m
	}
	newSelected := make(map[string]struct{}, len(m.selected))
	for k, v := range m.selected {
		newSelected[k] = v
	}
	key := podKey(m.items[m.cursor])
	if _, ok := newSelected[key]; ok {
		delete(newSelected, key)
	} else {
		newSelected[key] = struct{}{}
	}
	return Model{
		items:            m.items,
		cursor:           m.cursor,
		selected:         newSelected,
		width:            m.width,
		height:           m.height,
		offset:           m.offset,
		loading:          m.loading,
		spinnerFrame:     m.spinnerFrame,
		factIndex:        m.factIndex,
		showNamespace:    m.showNamespace,
		metricsAvailable: m.metricsAvailable,
	}
}

// SelectAll selects all items.
func (m Model) SelectAll() Model {
	newSelected := make(map[string]struct{}, len(m.items))
	for _, p := range m.items {
		newSelected[podKey(p)] = struct{}{}
	}
	return Model{
		items:            m.items,
		cursor:           m.cursor,
		selected:         newSelected,
		width:            m.width,
		height:           m.height,
		offset:           m.offset,
		loading:          m.loading,
		spinnerFrame:     m.spinnerFrame,
		factIndex:        m.factIndex,
		showNamespace:    m.showNamespace,
		metricsAvailable: m.metricsAvailable,
	}
}

// DeselectAll deselects all items.
func (m Model) DeselectAll() Model {
	return Model{
		items:            m.items,
		cursor:           m.cursor,
		selected:         make(map[string]struct{}),
		width:            m.width,
		height:           m.height,
		offset:           m.offset,
		loading:          m.loading,
		spinnerFrame:     m.spinnerFrame,
		factIndex:        m.factIndex,
		showNamespace:    m.showNamespace,
		metricsAvailable: m.metricsAvailable,
	}
}

// MoveUp moves the cursor up by one.
func (m Model) MoveUp() Model {
	if len(m.items) == 0 {
		return m
	}
	newModel := m
	newModel.cursor = m.cursor - 1
	if newModel.cursor < 0 {
		newModel.cursor = 0
	}
	if newModel.cursor < newModel.offset {
		newModel.offset = newModel.cursor
	}
	return newModel
}

// MoveDown moves the cursor down by one.
func (m Model) MoveDown() Model {
	if len(m.items) == 0 {
		return m
	}
	newModel := m
	newModel.cursor = m.cursor + 1
	if newModel.cursor >= len(m.items) {
		newModel.cursor = len(m.items) - 1
	}
	visibleRows := m.height
	if visibleRows <= 0 {
		visibleRows = 10
	}
	if newModel.cursor >= newModel.offset+visibleRows {
		newModel.offset = newModel.cursor - visibleRows + 1
	}
	return newModel
}

// GetSelected returns copies of all selected pods.
func (m Model) GetSelected() []k8s.PodInfo {
	result := make([]k8s.PodInfo, 0, len(m.selected))
	for _, p := range m.items {
		if _, ok := m.selected[podKey(p)]; ok {
			result = append(result, p)
		}
	}
	return result
}

// SelectedCount returns the number of selected pods.
func (m Model) SelectedCount() int {
	return len(m.selected)
}

// View renders the pod list.
func (m Model) View() string {
	if m.loading {
		spinner := styles.LoadingSpinner.Render(spinnerFrames[m.spinnerFrame])
		prefix := styles.LoadingPrefix.Render(" Fetching pods...")
		fact := styles.LoadingFact.Render(" " + loadingFacts[m.factIndex])
		return fmt.Sprintf("  %s%s\n  %s", spinner, prefix, fact)
	}
	if len(m.items) == 0 {
		return styles.FooterHelp.Render("  No pods found.")
	}

	visibleRows := m.height
	if visibleRows <= 0 {
		visibleRows = 10
	}

	var b strings.Builder

	end := m.offset + visibleRows
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := m.offset; i < end; i++ {
		pod := m.items[i]
		isCursor := i == m.cursor
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

		age := formatAge(pod.Age)
		name := pod.Name

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

		var line string
		if m.showNamespace {
			line = fmt.Sprintf("%s%s%-20s %-45s %s  %s  restarts: %d%s",
				pointer, checkbox, pod.Namespace, name, status, age, pod.RestartCount, metricsStr)
		} else {
			line = fmt.Sprintf("%s%s%-45s %s  %s  restarts: %d%s",
				pointer, checkbox, name, status, age, pod.RestartCount, metricsStr)
		}

		if isCursor {
			line = styles.SelectedRow.Render(line)
		}

		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
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

func formatAge(d time.Duration) string {
	hours := d.Hours()
	if hours >= 24 {
		return fmt.Sprintf("%dd", int(hours/24))
	}
	if hours >= 1 {
		return fmt.Sprintf("%dh", int(hours))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
