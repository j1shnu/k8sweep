package podlist

import (
	"math/rand"

	"github.com/jprasad/k8sweep/internal/k8s"
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

// Model represents the interactive pod list component with tree grouping.
type Model struct {
	items            []k8s.PodInfo
	displayRows      []DisplayRow
	collapsed        map[string]struct{}
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
		collapsed: make(map[string]struct{}),
		loading:   true,
		factIndex: randomFactIndex(),
	}
}

// Len returns the number of display rows (for pagination).
func (m Model) Len() int {
	return len(m.displayRows)
}

// PodCount returns the number of pods (for header/selection checks).
func (m Model) PodCount() int {
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
		displayRows:      m.displayRows,
		collapsed:        m.collapsed,
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

// SetItems returns a new model with the given pods, resetting cursor, selection,
// and collapse state.
func (m Model) SetItems(pods []k8s.PodInfo) Model {
	collapsed := make(map[string]struct{})
	groups := GroupPodsByController(pods, m.sortColumn, m.sortOrder)
	displayRows := BuildDisplayRows(groups, collapsed)
	cursor := firstPodRowIndex(displayRows)

	return Model{
		items:            pods,
		displayRows:      displayRows,
		collapsed:        collapsed,
		cursor:           cursor,
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

// SetItemsSorted returns a new model with updated pods, preserving cursor position,
// collapse state, and selection (pruned). Collapse state is carried forward without
// pruning — stale keys for absent groups are harmless no-ops in BuildDisplayRows,
// and this ensures collapse state survives filter toggles where groups are temporarily
// hidden. Real cleanup happens in SetItems (namespace switch), which resets to empty.
func (m Model) SetItemsSorted(pods []k8s.PodInfo) Model {
	podTarget, groupTarget := m.cursorTarget()

	groups := GroupPodsByController(pods, m.sortColumn, m.sortOrder)
	displayRows := BuildDisplayRows(groups, m.collapsed)

	newCursor := findRowIndex(displayRows, podTarget, groupTarget)
	if podTarget == "" && groupTarget == "" {
		newCursor = firstPodRowIndex(displayRows)
	}
	ps := m.pageSize()
	newOffset := 0
	if ps > 0 {
		newOffset = (newCursor / ps) * ps
	}

	newSelected := pruneSelected(m.selected, pods)

	return Model{
		items:            pods,
		displayRows:      displayRows,
		collapsed:        m.collapsed,
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
	podTarget, groupTarget := m.cursorTarget()

	groups := GroupPodsByController(m.items, col, order)
	displayRows := BuildDisplayRows(groups, m.collapsed)

	newCursor := findRowIndex(displayRows, podTarget, groupTarget)
	if podTarget == "" && groupTarget == "" {
		newCursor = firstPodRowIndex(displayRows)
	}
	ps := m.pageSize()
	newOffset := 0
	if ps > 0 {
		newOffset = (newCursor / ps) * ps
	}

	return Model{
		items:            m.items,
		displayRows:      displayRows,
		collapsed:        m.collapsed,
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

// CursorItem returns the pod at the current cursor position, or nil if cursor
// is on a controller row or the list is empty.
func (m Model) CursorItem() *k8s.PodInfo {
	if len(m.displayRows) == 0 || m.cursor >= len(m.displayRows) {
		return nil
	}
	row := m.displayRows[m.cursor]
	if row.Kind == RowPod && row.Pod != nil {
		p := *row.Pod
		return &p
	}
	return nil
}

// CursorRow returns the display row at the current cursor position.
func (m Model) CursorRow() *DisplayRow {
	if len(m.displayRows) == 0 || m.cursor >= len(m.displayRows) {
		return nil
	}
	r := m.displayRows[m.cursor]
	return &r
}

// SetSize returns a new model with the updated dimensions.
func (m Model) SetSize(width, height int) Model {
	return Model{
		items:            m.items,
		displayRows:      m.displayRows,
		collapsed:        m.collapsed,
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

// ToggleSelect toggles selection. On a controller row, toggles all pods in the
// group. On a pod row, toggles that single pod.
func (m Model) ToggleSelect() Model {
	if len(m.displayRows) == 0 || m.cursor >= len(m.displayRows) {
		return m
	}
	row := m.displayRows[m.cursor]

	newSelected := make(map[string]struct{}, len(m.selected))
	for k, v := range m.selected {
		newSelected[k] = v
	}

	if row.Kind == RowController && row.Header != nil {
		// Toggle all pods in this group
		allSelected := true
		for _, p := range row.Header.Pods {
			if _, ok := newSelected[podKey(p)]; !ok {
				allSelected = false
				break
			}
		}
		if allSelected {
			for _, p := range row.Header.Pods {
				delete(newSelected, podKey(p))
			}
		} else {
			for _, p := range row.Header.Pods {
				newSelected[podKey(p)] = struct{}{}
			}
		}
	} else if row.Kind == RowPod && row.Pod != nil {
		key := podKey(*row.Pod)
		if _, ok := newSelected[key]; ok {
			delete(newSelected, key)
		} else {
			newSelected[key] = struct{}{}
		}
	}

	return Model{
		items:            m.items,
		displayRows:      m.displayRows,
		collapsed:        m.collapsed,
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
		displayRows:      m.displayRows,
		collapsed:        m.collapsed,
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
		displayRows:      m.displayRows,
		collapsed:        m.collapsed,
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

// ToggleCollapse toggles expand/collapse of the controller group at cursor.
// No-op if cursor is not on a controller row.
func (m Model) ToggleCollapse() Model {
	if len(m.displayRows) == 0 || m.cursor >= len(m.displayRows) {
		return m
	}
	row := m.displayRows[m.cursor]
	if row.Kind != RowController {
		return m
	}

	newCollapsed := make(map[string]struct{}, len(m.collapsed))
	for k, v := range m.collapsed {
		newCollapsed[k] = v
	}
	if _, ok := newCollapsed[row.GroupKey]; ok {
		delete(newCollapsed, row.GroupKey)
	} else {
		newCollapsed[row.GroupKey] = struct{}{}
	}

	groups := GroupPodsByController(m.items, m.sortColumn, m.sortOrder)
	displayRows := BuildDisplayRows(groups, newCollapsed)

	// Keep cursor at the same controller row
	newCursor := findRowIndex(displayRows, "", row.GroupKey)
	ps := m.pageSize()
	newOffset := 0
	if ps > 0 {
		newOffset = (newCursor / ps) * ps
	}

	return Model{
		items:            m.items,
		displayRows:      displayRows,
		collapsed:        newCollapsed,
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
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
	}
}

// CollapseAll collapses all controller groups.
func (m Model) CollapseAll() Model {
	groups := GroupPodsByController(m.items, m.sortColumn, m.sortOrder)
	newCollapsed := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		newCollapsed[g.Key] = struct{}{}
	}
	displayRows := BuildDisplayRows(groups, newCollapsed)

	// If cursor was on a pod, move to its group header
	_, groupTarget := m.cursorTarget()
	if groupTarget == "" && len(m.displayRows) > 0 && m.cursor < len(m.displayRows) {
		groupTarget = m.displayRows[m.cursor].GroupKey
	}
	newCursor := findRowIndex(displayRows, "", groupTarget)

	ps := m.pageSize()
	newOffset := 0
	if ps > 0 {
		newOffset = (newCursor / ps) * ps
	}

	return Model{
		items:            m.items,
		displayRows:      displayRows,
		collapsed:        newCollapsed,
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
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
	}
}

// ExpandAll expands all controller groups.
func (m Model) ExpandAll() Model {
	newCollapsed := make(map[string]struct{})
	groups := GroupPodsByController(m.items, m.sortColumn, m.sortOrder)
	displayRows := BuildDisplayRows(groups, newCollapsed)

	podTarget, groupTarget := m.cursorTarget()
	newCursor := findRowIndex(displayRows, podTarget, groupTarget)

	ps := m.pageSize()
	newOffset := 0
	if ps > 0 {
		newOffset = (newCursor / ps) * ps
	}

	return Model{
		items:            m.items,
		displayRows:      displayRows,
		collapsed:        newCollapsed,
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
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
	}
}

// SmartCollapse collapses controller groups that have no dirty pods, leaving
// groups with at least one dirty pod expanded. Used on startup to surface
// problematic pods without visual clutter from healthy groups.
func (m Model) SmartCollapse() Model {
	groups := GroupPodsByController(m.items, m.sortColumn, m.sortOrder)
	newCollapsed := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		hasDirty := false
		for _, p := range g.Pods {
			if p.IsDirty() {
				hasDirty = true
				break
			}
		}
		if !hasDirty {
			newCollapsed[g.Key] = struct{}{}
		}
	}
	displayRows := BuildDisplayRows(groups, newCollapsed)
	cursor := firstPodRowIndex(displayRows)

	ps := m.pageSize()
	newOffset := 0
	if ps > 0 {
		newOffset = (cursor / ps) * ps
	}

	return Model{
		items:            m.items,
		displayRows:      displayRows,
		collapsed:        newCollapsed,
		cursor:           cursor,
		selected:         m.selected,
		width:            m.width,
		height:           m.height,
		offset:           newOffset,
		loading:          m.loading,
		spinnerFrame:     m.spinnerFrame,
		factIndex:        m.factIndex,
		showNamespace:    m.showNamespace,
		metricsAvailable: m.metricsAvailable,
		sortColumn:       m.sortColumn,
		sortOrder:        m.sortOrder,
	}
}

// AnyExpanded returns true if any controller group is expanded.
func (m Model) AnyExpanded() bool {
	for _, r := range m.displayRows {
		if r.Kind == RowController {
			if _, ok := m.collapsed[r.GroupKey]; !ok {
				return true
			}
		}
	}
	return false
}

// GoTop moves the cursor to the first item.
func (m Model) GoTop() Model {
	if len(m.displayRows) == 0 {
		return m
	}
	newModel := m
	newModel.cursor = 0
	newModel.offset = 0
	return newModel
}

// GoFirstPod moves the cursor to the first pod row, skipping any leading
// controller headers. Falls back to GoTop if no pod rows exist.
func (m Model) GoFirstPod() Model {
	idx := firstPodRowIndex(m.displayRows)
	if len(m.displayRows) == 0 {
		return m
	}
	newModel := m
	newModel.cursor = idx
	newModel.offset = m.pageStartForCursor(idx)
	return newModel
}

// GoBottom moves the cursor to the last item.
func (m Model) GoBottom() Model {
	if len(m.displayRows) == 0 {
		return m
	}
	newModel := m
	newModel.cursor = len(m.displayRows) - 1
	newModel.offset = m.pageStartForCursor(newModel.cursor)
	return newModel
}

// MoveUp moves the cursor up by one.
func (m Model) MoveUp() Model {
	if len(m.displayRows) == 0 {
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
	if len(m.displayRows) == 0 {
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
	if len(m.displayRows) == 0 {
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
	if targetEnd > len(m.displayRows) {
		targetEnd = len(m.displayRows)
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
	if len(m.displayRows) == 0 {
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
	if targetEnd > len(m.displayRows) {
		targetEnd = len(m.displayRows)
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
	count := len(m.displayRows)
	if count == 0 {
		return 1
	}
	size := m.pageSize()
	return (count + size - 1) / size
}

func (m Model) currentPage() int {
	count := len(m.displayRows)
	if count == 0 {
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
	count := len(m.displayRows)
	if count == 0 {
		return 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= count {
		cursor = count - 1
	}
	return (cursor / m.pageSize()) * m.pageSize()
}

func (m Model) currentPageBounds() (start, end int) {
	count := len(m.displayRows)
	if count == 0 {
		return 0, 0
	}
	page := m.currentPage()
	size := m.pageSize()
	start = page * size
	end = start + size
	if end > count {
		end = count
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

// --- Tree helpers ---

// cursorTarget returns identifiers for tracking the cursor across display row rebuilds.
func (m Model) cursorTarget() (podTarget, groupTarget string) {
	if len(m.displayRows) == 0 || m.cursor >= len(m.displayRows) {
		return "", ""
	}
	row := m.displayRows[m.cursor]
	if row.Kind == RowPod && row.Pod != nil {
		return podKey(*row.Pod), ""
	}
	return "", row.GroupKey
}

// findRowIndex finds the display row index matching the given target.
// Prefers pod target over group target (two-pass). Returns 0 if no match.
func findRowIndex(rows []DisplayRow, podTarget, groupTarget string) int {
	if podTarget != "" {
		for i, r := range rows {
			if r.Kind == RowPod && r.Pod != nil && podKey(*r.Pod) == podTarget {
				return i
			}
		}
	}
	if groupTarget != "" {
		for i, r := range rows {
			if r.Kind == RowController && r.GroupKey == groupTarget {
				return i
			}
		}
	}
	return 0
}

// pruneSelected carries forward selection, removing keys for pods that no
// longer exist.
func pruneSelected(old map[string]struct{}, pods []k8s.PodInfo) map[string]struct{} {
	if len(old) == 0 {
		return make(map[string]struct{})
	}
	present := make(map[string]struct{}, len(pods))
	for _, p := range pods {
		present[podKey(p)] = struct{}{}
	}
	result := make(map[string]struct{}, len(old))
	for k := range old {
		if _, ok := present[k]; ok {
			result[k] = struct{}{}
		}
	}
	return result
}
