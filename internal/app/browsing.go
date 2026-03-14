package app

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/common"
	"github.com/jprasad/k8sweep/internal/tui/deletepreview"
	"github.com/jprasad/k8sweep/internal/tui/header"
	"github.com/jprasad/k8sweep/internal/tui/podlist"
)

func (m Model) handleBrowsingKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Help):
		newModel := m
		newModel.state = stateHelp
		newModel.help = m.help.SetSize(m.width, m.height-common.HeaderHeight-common.FooterHeight-1)
		return newModel, nil

	// Navigation
	case key.Matches(msg, m.keys.Up):
		newModel := m
		newModel.podList = m.podList.MoveUp()
		newModel.pendingG = false
		return newModel, nil
	case key.Matches(msg, m.keys.Down):
		newModel := m
		newModel.podList = m.podList.MoveDown()
		newModel.pendingG = false
		return newModel, nil
	case key.Matches(msg, m.keys.PageUp):
		newModel := m
		newModel.podList = m.podList.PageUp()
		newModel.pendingG = false
		return newModel, nil
	case key.Matches(msg, m.keys.PageDown):
		newModel := m
		newModel.podList = m.podList.PageDown()
		newModel.pendingG = false
		return newModel, nil
	case key.Matches(msg, m.keys.GoBottom):
		newModel := m
		newModel.podList = m.podList.GoBottom()
		newModel.pendingG = false
		return newModel, nil
	case msg.String() == "g":
		return m.handleGG()

	// Selection & tree
	case key.Matches(msg, m.keys.Select):
		newModel := m
		newModel.podList = m.podList.ToggleSelect()
		return newModel, nil
	case key.Matches(msg, m.keys.Toggle):
		newModel := m
		newModel.podList = m.podList.ToggleCollapse()
		return newModel, nil
	case key.Matches(msg, m.keys.ToggleAllGroups):
		newModel := m
		if m.podList.AnyExpanded() {
			newModel.podList = m.podList.CollapseAll()
			newModel.savedAllCollapsed = true
		} else {
			newModel.podList = m.podList.ExpandAll()
			newModel.savedAllCollapsed = false
		}
		return newModel, newModel.savePrefsCmd()
	case key.Matches(msg, m.keys.SelectAll):
		newModel := m
		if m.podList.SelectedCount() == m.podList.PodCount() {
			newModel.podList = m.podList.DeselectAll()
		} else {
			newModel.podList = m.podList.SelectAll()
		}
		return newModel, nil

	// Actions
	case key.Matches(msg, m.keys.Delete):
		return m.enterDeletePreview(common.DeleteNormal)
	case key.Matches(msg, m.keys.ForceDelete):
		return m.enterDeletePreview(common.DeleteForce)
	case key.Matches(msg, m.keys.Namespace):
		return m.handleNamespaceKey()
	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefreshKey()
	case key.Matches(msg, m.keys.Info):
		return m.handleInfoKey()
	case key.Matches(msg, m.keys.Sort):
		return m.handleSortKey()
	case key.Matches(msg, m.keys.Search):
		return m.enterSearchMode()
	case key.Matches(msg, m.keys.Filter):
		return m.handleFilterKey()
	case key.Matches(msg, m.keys.ControllerFilter):
		return m.handleControllerFilterKey()
	case msg.String() == "esc":
		if m.controllerDrillDown != "" {
			return m.exitControllerDrillDown()
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleGG() (Model, tea.Cmd) {
	if m.pendingG && time.Since(m.pendingGTime) < 500*time.Millisecond {
		newModel := m
		newModel.podList = m.podList.GoTop()
		newModel.pendingG = false
		return newModel, nil
	}
	newModel := m
	newModel.pendingG = true
	newModel.pendingGTime = time.Now()
	return newModel, nil
}

func (m Model) handleNamespaceKey() (Model, tea.Cmd) {
	newModel := m
	newModel.nsLoading = true
	newModel.nsSpinnerFrame = 0
	newModel.statusMsg = ""

	// If pods are still loading, stop the watcher to free API bandwidth
	// so the namespace list fetch gets priority. Bump watchID to discard
	// any in-flight watch results. switchNamespace() creates a fresh watcher.
	if m.podList.IsLoading() && m.watcher != nil {
		m.watcher.Stop()
		newModel.watcher = nil
		newModel.watchID = m.watchID + 1
	}

	return newModel, tea.Batch(newModel.fetchNamespacesCmd(), loadingTickCmd())
}

func (m Model) handleRefreshKey() (Model, tea.Cmd) {
	newModel := m
	if m.watcher != nil {
		pods := m.watcher.ListPods()
		allPods := pods
		if m.lastMetrics != nil {
			allPods = k8s.MergePodMetrics(allPods, m.lastMetrics)
		}
		displayPods := applyFilters(allPods, m.filter, m.activeSearchQuery(), m.controllerDrillDown)
		newModel.allPods = allPods
		newModel.totalPodCount = len(allPods)
		newModel.podList = m.podList.SetItemsSorted(displayPods)
		newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), len(allPods))).
			SetStatusSummary(buildStatusSummary(allPods))
		newModel.statusMsg = "Refreshed"
		return newModel, m.fetchMetricsCmd()
	}
	newModel.podList = m.podList.SetLoading()
	newModel.statusMsg = "Refreshing..."
	return newModel, tea.Batch(newModel.fetchPodsCmd(), newModel.fetchMetricsCmd(), loadingTickCmd())
}

func (m Model) handleInfoKey() (Model, tea.Cmd) {
	row := m.podList.CursorRow()
	if row == nil {
		return m, nil
	}

	// Controller row: drill down into this controller's pods
	if row.Kind == podlist.RowController && row.Header != nil {
		// Toggle: if already drilled into this controller, exit drill-down
		if m.controllerDrillDown == row.Header.Key {
			return m.exitControllerDrillDown()
		}
		return m.enterControllerDrillDown(row.Header.Key)
	}

	// Pod row: open pod detail overlay
	pod := m.podList.CursorItem()
	if pod == nil {
		return m, nil
	}
	newModel := m
	newModel.state = stateViewingDetail
	newModel.detailPodKey = pod.Namespace + "/" + pod.Name
	newModel.detailData = nil
	newModel.detailStatus = ""
	newModel.podDetail = m.podDetail.SetSize(m.width, m.height-common.HeaderHeight-1).SetLoading()
	return newModel, newModel.fetchPodDetailCmd(pod.Namespace, pod.Name)
}

func (m Model) enterControllerDrillDown(groupKey string) (Model, tea.Cmd) {
	newModel := m
	newModel.controllerDrillDown = groupKey
	newModel.statusMsg = ""
	if m.allPods != nil {
		displayPods := applyFilters(m.allPods, m.filter, m.activeSearchQuery(), groupKey)
		newModel.podList = m.podList.SetItemsSorted(displayPods).GoTop()
		newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), len(m.allPods))).
			SetStatusSummary(buildStatusSummary(m.allPods))
	}
	return newModel, newModel.savePrefsCmd()
}

func (m Model) exitControllerDrillDown() (Model, tea.Cmd) {
	newModel := m
	newModel.controllerDrillDown = ""
	newModel.statusMsg = ""
	if m.allPods != nil {
		displayPods := applyFilters(m.allPods, m.filter, m.activeSearchQuery(), "")
		newModel.podList = m.podList.SetItemsSorted(displayPods).GoTop()
		newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), len(m.allPods))).
			SetStatusSummary(buildStatusSummary(m.allPods))
	}
	return newModel, newModel.savePrefsCmd()
}

func (m Model) handleSortKey() (Model, tea.Cmd) {
	col := m.podList.SortColumn()
	order := m.podList.SortOrder()
	if order == podlist.SortAsc {
		order = podlist.SortDesc
	} else {
		col = podlist.NextSortColumn(col, m.podList.MetricsAvailable())
		order = podlist.SortAsc
	}
	newModel := m
	newModel.podList = m.podList.SetSort(col, order)
	newModel.statusMsg = "Sort: " + podlist.SortColumnLabel(col) + " " + podlist.SortIndicator(order)
	return newModel, newModel.savePrefsCmd()
}

func (m Model) handleFilterKey() (Model, tea.Cmd) {
	newFilter := !m.filter.ShowDirtyOnly
	newModel := m
	newModel.filter = k8s.ResourceFilter{ShowDirtyOnly: newFilter, ControllerKindFilter: m.filter.ControllerKindFilter}
	if newFilter {
		newModel.statusMsg = "Filter: showing dirty pods only"
	} else {
		newModel.statusMsg = "Filter: showing all pods"
	}
	if m.allPods != nil {
		displayPods := applyFilters(m.allPods, newModel.filter, m.activeSearchQuery(), m.controllerDrillDown)
		newModel.podList = m.podList.SetItemsSorted(displayPods)
		// Turning filter OFF should reset pagination/cursor to page 1.
		if !newFilter {
			newModel.podList = newModel.podList.GoFirstPod()
		}
		newModel.header = m.header.SetFilter(newFilter, buildPodCountLabel(newFilter, len(displayPods), len(m.allPods))).
			SetStatusSummary(buildStatusSummary(m.allPods))
		return newModel, newModel.savePrefsCmd()
	}
	newModel.podList = m.podList.SetLoading()
	newModel.header = m.header.SetFilter(newFilter, "").
		SetStatusSummary(header.StatusSummary{})
	return newModel, tea.Batch(newModel.fetchPodsCmd(), newModel.fetchMetricsCmd(), loadingTickCmd(), newModel.savePrefsCmd())
}

func (m Model) handleControllerFilterKey() (Model, tea.Cmd) {
	nextKind := k8s.NextControllerFilter(m.filter.ControllerKindFilter)
	newModel := m
	newModel.filter = k8s.ResourceFilter{ShowDirtyOnly: m.filter.ShowDirtyOnly, ControllerKindFilter: nextKind}
	if nextKind == "" {
		newModel.statusMsg = "Controller filter: all"
	} else {
		newModel.statusMsg = "Controller filter: " + string(nextKind)
	}
	if m.allPods != nil {
		displayPods := applyFilters(m.allPods, newModel.filter, m.activeSearchQuery(), m.controllerDrillDown)
		newModel.podList = m.podList.SetItemsSorted(displayPods).GoTop()
		newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), len(m.allPods))).
			SetStatusSummary(buildStatusSummary(m.allPods))
		return newModel, newModel.savePrefsCmd()
	}
	return newModel, nil
}

// enterDeletePreview enters the delete preview screen.
func (m Model) enterDeletePreview(action common.DeleteAction) (Model, tea.Cmd) {
	selected := m.podList.GetSelected()
	if len(selected) == 0 {
		newModel := m
		newModel.statusMsg = "No pods selected"
		return newModel, nil
	}

	var warnings []string
	for _, p := range selected {
		if p.IsStandalone() {
			warnings = append(warnings, p.Name+" is standalone (no controller — delete is permanent)")
		}
		if p.Status == k8s.StatusRunning {
			warnings = append(warnings, p.Name+" is Running — deletion will interrupt active workload")
		}
	}

	newModel := m
	newModel.deletePreview = deletepreview.New(selected, action, warnings).
		SetSize(m.width, m.height-common.HeaderHeight-1)
	newModel.state = stateDeletePreview
	return newModel, nil
}
