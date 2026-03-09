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

const (
	namespaceColWidth = 20
	nameColWidth      = 45
)

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
	offset           int // current page start index
	loading          bool
	spinnerFrame     int  // current spinner animation frame index
	factIndex        int  // current fact message index
	factTicks        int  // ticks elapsed since last fact rotation
	showNamespace    bool // show namespace column (all-namespaces mode)
	metricsAvailable bool // show CPU/memory columns when metrics API is available
	sortColumn       SortColumn
	sortOrder        SortOrder
}

// New creates an empty pod list model.
func New() Model {
	return Model{
		selected:  make(map[string]struct{}),
		loading:   true,
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
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
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
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
	}
}

// SetItemsSorted returns a new model with pods sorted by the current sort column,
// resetting cursor and selection (same as SetItems but applies current sort).
func (m Model) SetItemsSorted(pods []k8s.PodInfo) Model {
	sorted := sortPods(pods, m.sortColumn, m.sortOrder)

	// Track the pod under cursor so we can follow it after re-sort
	var cursorKey string
	if len(m.items) > 0 && m.cursor < len(m.items) {
		cursorKey = podKey(m.items[m.cursor])
	}

	newCursor := 0
	for i, p := range sorted {
		if podKey(p) == cursorKey {
			newCursor = i
			break
		}
	}

	newOffset := m.pageStartForCursor(newCursor)

	// Prune selection to only pods still present
	newSelected := make(map[string]struct{})
	if len(m.selected) > 0 {
		newSelected = make(map[string]struct{}, len(m.selected))
		presentKeys := make(map[string]struct{}, len(sorted))
		for _, p := range sorted {
			presentKeys[podKey(p)] = struct{}{}
		}
		for k := range m.selected {
			if _, ok := presentKeys[k]; ok {
				newSelected[k] = struct{}{}
			}
		}
	}

	return Model{
		items:            sorted,
		cursor:           newCursor,
		selected:         newSelected,
		width:            m.width,
		height:           m.height,
		offset:           newOffset,
		loading:          false,
		showNamespace:    m.showNamespace,
		metricsAvailable: m.metricsAvailable,
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
	}
}

// SetSort applies a new sort column and order. Preserves cursor position by tracking
// the pod key at the current cursor. Selection is preserved.
func (m Model) SetSort(col SortColumn, order SortOrder) Model {
	// Track the pod under cursor so we can follow it after sort
	var cursorKey string
	if len(m.items) > 0 && m.cursor < len(m.items) {
		cursorKey = podKey(m.items[m.cursor])
	}

	sorted := sortPods(m.items, col, order)

	// Find the tracked pod in the new order
	newCursor := 0
	for i, p := range sorted {
		if podKey(p) == cursorKey {
			newCursor = i
			break
		}
	}

	newOffset := m.pageStartForCursor(newCursor)

	return Model{
		items:            sorted,
		cursor:           newCursor,
		selected:         m.selected,
		width:            m.width,
		height:           m.height,
		offset:           newOffset,
		loading:          m.loading,
		spinnerFrame:     m.spinnerFrame,
		factIndex:        m.factIndex,
		showNamespace:    m.showNamespace,
		metricsAvailable: m.metricsAvailable,
		sortColumn:       col,
		sortOrder:        order,
	}
}

// SortColumn returns the current sort column.
func (m Model) SortColumn() SortColumn {
	return m.sortColumn
}

// SortOrder returns the current sort order.
func (m Model) SortOrder() SortOrder {
	return m.sortOrder
}

// MetricsAvailable returns whether metrics columns are shown.
func (m Model) MetricsAvailable() bool {
	return m.metricsAvailable
}

// CursorItem returns the pod at the current cursor position, or nil if empty.
func (m Model) CursorItem() *k8s.PodInfo {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return nil
	}
	p := m.items[m.cursor]
	return &p
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
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
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
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
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
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
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
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
	}
}

// GoTop moves the cursor to the first item.
func (m Model) GoTop() Model {
	if len(m.items) == 0 {
		return m
	}
	newModel := m
	newModel.cursor = 0
	newModel.offset = 0
	return newModel
}

// GoBottom moves the cursor to the last item.
func (m Model) GoBottom() Model {
	if len(m.items) == 0 {
		return m
	}
	newModel := m
	newModel.cursor = len(m.items) - 1
	newModel.offset = m.pageStartForCursor(newModel.cursor)
	return newModel
}

// MoveUp moves the cursor up by one.
func (m Model) MoveUp() Model {
	if len(m.items) == 0 {
		return m
	}
	start, _ := m.currentPageBounds()
	newModel := m
	if m.cursor > start {
		newModel.cursor = m.cursor - 1
	}
	newModel.offset = start
	return newModel
}

// MoveDown moves the cursor down by one.
func (m Model) MoveDown() Model {
	if len(m.items) == 0 {
		return m
	}
	start, end := m.currentPageBounds()
	last := end - 1
	newModel := m
	if m.cursor < last {
		newModel.cursor = m.cursor + 1
	}
	newModel.offset = start
	return newModel
}

// PageUp moves the viewport and cursor up by one page with one-row overlap.
func (m Model) PageUp() Model {
	if len(m.items) == 0 {
		return m
	}
	pageSize := m.pageSize()
	currentPage := m.currentPage()
	if currentPage == 0 {
		return m
	}
	currentStart, _ := m.currentPageBounds()
	row := m.cursor - currentStart
	targetPage := currentPage - 1
	targetStart := targetPage * pageSize
	targetEnd := targetStart + pageSize
	if targetEnd > len(m.items) {
		targetEnd = len(m.items)
	}
	newModel := m
	newModel.offset = targetStart
	newModel.cursor = targetStart + row
	if newModel.cursor >= targetEnd {
		newModel.cursor = targetEnd - 1
	}
	return newModel
}

// PageDown moves the viewport and cursor down by one page with one-row overlap.
func (m Model) PageDown() Model {
	if len(m.items) == 0 {
		return m
	}
	pageSize := m.pageSize()
	currentPage := m.currentPage()
	totalPages := m.totalPages()
	if currentPage >= totalPages-1 {
		return m
	}
	currentStart, _ := m.currentPageBounds()
	row := m.cursor - currentStart
	targetPage := currentPage + 1
	targetStart := targetPage * pageSize
	targetEnd := targetStart + pageSize
	if targetEnd > len(m.items) {
		targetEnd = len(m.items)
	}
	newModel := m
	newModel.offset = targetStart
	newModel.cursor = targetStart + row
	if newModel.cursor >= targetEnd {
		newModel.cursor = targetEnd - 1
	}
	return newModel
}

func (m Model) pageSize() int {
	// page size is viewport-sized rows, excluding header + pager line
	rows := m.height
	if rows <= 0 {
		rows = 10
	}
	size := rows - 2
	if size < 1 {
		size = 1
	}
	return size
}

func (m Model) totalPages() int {
	if len(m.items) == 0 {
		return 1
	}
	size := m.pageSize()
	return (len(m.items) + size - 1) / size
}

func (m Model) currentPage() int {
	if len(m.items) == 0 {
		return 0
	}
	size := m.pageSize()
	page := m.cursor / size
	lastPage := m.totalPages() - 1
	if page > lastPage {
		return lastPage
	}
	return page
}

func (m Model) pageStartForCursor(cursor int) int {
	if len(m.items) == 0 {
		return 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(m.items) {
		cursor = len(m.items) - 1
	}
	return (cursor / m.pageSize()) * m.pageSize()
}

func (m Model) currentPageBounds() (start, end int) {
	if len(m.items) == 0 {
		return 0, 0
	}
	page := m.currentPage()
	size := m.pageSize()
	start = page * size
	end = start + size
	if end > len(m.items) {
		end = len(m.items)
	}
	return start, end
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

	var b strings.Builder

	// Render column header row
	b.WriteString(m.renderHeaderRow())
	b.WriteString("\n")
	start, end := m.currentPageBounds()

	for i := start; i < end; i++ {
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
		name := smartTruncateMiddle(pod.Name, nameColWidth)

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
			line = fmt.Sprintf("%s%s%-*s %-*s %s  %s  restarts: %d%s",
				pointer, checkbox, namespaceColWidth, pod.Namespace, nameColWidth, name, status, age, pod.RestartCount, metricsStr)
		} else {
			line = fmt.Sprintf("%s%s%-*s %s  %s  restarts: %d%s",
				pointer, checkbox, nameColWidth, name, status, age, pod.RestartCount, metricsStr)
		}

		if isCursor {
			line = styles.SelectedRow.Render(line)
		}

		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	page := m.currentPage() + 1
	totalPages := m.totalPages()
	if totalPages > 1 {
		showStart := start + 1
		showEnd := end
		pager := fmt.Sprintf("Showing %d-%d of %d Pods [%s] | %s/%s next | %s/%s previous",
			showStart, showEnd, len(m.items),
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

// renderHeaderRow renders the column header with sort indicator.
func (m Model) renderHeaderRow() string {
	indicator := func(col SortColumn) string {
		if m.sortColumn == col {
			return " " + SortIndicator(m.sortOrder)
		}
		return ""
	}

	// Pad to match the pointer + checkbox prefix ("  [ ] ")
	prefix := "      "

	var header string
	if m.showNamespace {
		header = fmt.Sprintf("%s%-*s %-*s %-16s  %-5s  %-10s",
			prefix,
			namespaceColWidth,
			"NAMESPACE"+indicator(SortByName),
			nameColWidth,
			"NAME"+indicator(SortByName),
			"STATUS"+indicator(SortByStatus),
			"AGE"+indicator(SortByAge),
			"RESTARTS"+indicator(SortByRestarts))
	} else {
		header = fmt.Sprintf("%s%-*s %-16s  %-5s  %-10s",
			prefix,
			nameColWidth,
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
