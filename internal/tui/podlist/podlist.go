package podlist

import (
	"fmt"
	"strings"
	"time"

	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// Model represents the interactive pod list component.
type Model struct {
	items         []k8s.PodInfo
	cursor        int
	selected      map[string]struct{} // key: "namespace/name"
	width         int
	height        int
	offset        int // viewport scroll offset
	loading       bool
	showNamespace bool // show namespace column (all-namespaces mode)
}

// New creates an empty pod list model.
func New() Model {
	return Model{
		selected: make(map[string]struct{}),
		loading:  true,
	}
}

// Len returns the number of items in the list.
func (m Model) Len() int {
	return len(m.items)
}

// SetLoading returns a new model in the loading state.
func (m Model) SetLoading() Model {
	return Model{
		items:         m.items,
		cursor:        m.cursor,
		selected:      m.selected,
		width:         m.width,
		height:        m.height,
		offset:        m.offset,
		loading:       true,
		showNamespace: m.showNamespace,
	}
}

// SetShowNamespace returns a new model that shows or hides the namespace column.
func (m Model) SetShowNamespace(show bool) Model {
	newModel := m
	newModel.showNamespace = show
	return newModel
}

// SetItems returns a new model with the given pods, resetting cursor and selection.
func (m Model) SetItems(pods []k8s.PodInfo) Model {
	return Model{
		items:         pods,
		cursor:        0,
		selected:      make(map[string]struct{}),
		width:         m.width,
		height:        m.height,
		offset:        0,
		loading:       false,
		showNamespace: m.showNamespace,
	}
}

// SetSize returns a new model with the updated dimensions.
func (m Model) SetSize(width, height int) Model {
	return Model{
		items:         m.items,
		cursor:        m.cursor,
		selected:      m.selected,
		width:         width,
		height:        height,
		offset:        m.offset,
		loading:       m.loading,
		showNamespace: m.showNamespace,
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
		items:         m.items,
		cursor:        m.cursor,
		selected:      newSelected,
		width:         m.width,
		height:        m.height,
		offset:        m.offset,
		loading:       m.loading,
		showNamespace: m.showNamespace,
	}
}

// SelectAll selects all items.
func (m Model) SelectAll() Model {
	newSelected := make(map[string]struct{}, len(m.items))
	for _, p := range m.items {
		newSelected[podKey(p)] = struct{}{}
	}
	return Model{
		items:         m.items,
		cursor:        m.cursor,
		selected:      newSelected,
		width:         m.width,
		height:        m.height,
		offset:        m.offset,
		loading:       m.loading,
		showNamespace: m.showNamespace,
	}
}

// DeselectAll deselects all items.
func (m Model) DeselectAll() Model {
	return Model{
		items:         m.items,
		cursor:        m.cursor,
		selected:      make(map[string]struct{}),
		width:         m.width,
		height:        m.height,
		offset:        m.offset,
		loading:       m.loading,
		showNamespace: m.showNamespace,
	}
}

// MoveUp moves the cursor up by one.
func (m Model) MoveUp() Model {
	if len(m.items) == 0 {
		return m
	}
	newCursor := m.cursor - 1
	if newCursor < 0 {
		newCursor = 0
	}
	newOffset := m.offset
	if newCursor < newOffset {
		newOffset = newCursor
	}
	return Model{
		items:         m.items,
		cursor:        newCursor,
		selected:      m.selected,
		width:         m.width,
		height:        m.height,
		offset:        newOffset,
		showNamespace: m.showNamespace,
	}
}

// MoveDown moves the cursor down by one.
func (m Model) MoveDown() Model {
	if len(m.items) == 0 {
		return m
	}
	newCursor := m.cursor + 1
	if newCursor >= len(m.items) {
		newCursor = len(m.items) - 1
	}
	newOffset := m.offset
	visibleRows := m.height
	if visibleRows <= 0 {
		visibleRows = 10
	}
	if newCursor >= newOffset+visibleRows {
		newOffset = newCursor - visibleRows + 1
	}
	return Model{
		items:         m.items,
		cursor:        newCursor,
		selected:      m.selected,
		width:         m.width,
		height:        m.height,
		offset:        newOffset,
		showNamespace: m.showNamespace,
	}
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
		return styles.FooterHelp.Render("  Fetching pods...")
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

		var line string
		if m.showNamespace {
			line = fmt.Sprintf("%s%s%-20s %-45s %s  %s  restarts: %d",
				pointer, checkbox, pod.Namespace, name, status, age, pod.RestartCount)
		} else {
			line = fmt.Sprintf("%s%s%-45s %s  %s  restarts: %d",
				pointer, checkbox, name, status, age, pod.RestartCount)
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
