package app

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) enterSearchMode() (Model, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = "type to filter pods by name..."
	ti.Prompt = "/ "
	ti.CharLimit = 100
	if m.searchQuery != "" {
		ti.SetValue(m.searchQuery)
	}
	cmd := ti.Focus()

	newModel := m
	newModel.state = stateSearching
	newModel.searchInput = ti
	return newModel, cmd
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newModel := m
		newModel.state = stateBrowsing
		newModel.searchQuery = m.searchInput.Value()
		newModel.searchSeq++
		// Re-apply filters with confirmed search
		if m.allPods != nil {
			displayPods := applyFilters(m.allPods, m.filter, newModel.searchQuery, m.controllerDrillDown)
			newModel.podList = m.podList.SetItemsSorted(displayPods)
			newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), len(m.allPods))).
				SetStatusSummary(buildStatusSummary(m.allPods))
		}
		if newModel.searchQuery != "" {
			newModel.statusMsg = "Search: " + newModel.searchQuery
		} else {
			newModel.statusMsg = ""
		}
		return newModel, newModel.savePrefsCmd()

	case "esc":
		newModel := m
		newModel.state = stateBrowsing
		newModel.searchQuery = ""
		newModel.searchSeq++
		// Clear search filter
		if m.allPods != nil {
			displayPods := applyFilters(m.allPods, m.filter, "", m.controllerDrillDown)
			newModel.podList = m.podList.SetItemsSorted(displayPods)
			newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), len(m.allPods))).
				SetStatusSummary(buildStatusSummary(m.allPods))
		}
		newModel.statusMsg = ""
		return newModel, newModel.savePrefsCmd()
	}

	// Pass to textinput
	newInput, cmd := m.searchInput.Update(msg)
	newModel := m
	newModel.searchInput = newInput
	query := newInput.Value()
	newModel.searchSeq = m.searchSeq + 1

	return newModel, tea.Batch(cmd, searchDebounceCmd(newModel.searchSeq, query))
}

// handleSearchMsg handles non-key messages (like blink) while in search mode.
func (m Model) handleSearchMsg(msg tea.Msg) (Model, tea.Cmd) {
	newInput, cmd := m.searchInput.Update(msg)
	newModel := m
	newModel.searchInput = newInput
	return newModel, cmd
}

func (m Model) handleSearchDebounced(msg SearchDebouncedMsg) Model {
	if m.state != stateSearching || msg.Seq != m.searchSeq {
		return m
	}
	if m.allPods == nil {
		return m
	}
	displayPods := applyFilters(m.allPods, m.filter, msg.Query, m.controllerDrillDown)
	newModel := m
	newModel.podList = m.podList.SetItemsSorted(displayPods)
	newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), len(m.allPods))).
		SetStatusSummary(buildStatusSummary(m.allPods))
	return newModel
}
