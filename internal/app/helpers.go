package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/header"
)

const deleteTimeout = 30 * time.Second

func buildPodCountLabel(filterActive bool, shown, total int) string {
	if !filterActive {
		return ""
	}
	return fmt.Sprintf("%d/%d dirty pods", shown, total)
}

// activeSearchQuery returns the current search text — from the input field
// when actively searching, or from the confirmed query otherwise.
func (m Model) activeSearchQuery() string {
	if m.state == stateSearching {
		return m.searchInput.Value()
	}
	return m.searchQuery
}

// applyFilters applies dirty filter and name search to a pod list.
func applyFilters(pods []k8s.PodInfo, filter k8s.ResourceFilter, searchQuery string) []k8s.PodInfo {
	result := pods
	if filter.ShowDirtyOnly {
		result = k8s.FilterDirtyPods(result)
	}
	if searchQuery != "" {
		result = filterByName(result, searchQuery)
	}
	return result
}

// filterByName returns pods whose name contains the query (case-insensitive).
func filterByName(pods []k8s.PodInfo, query string) []k8s.PodInfo {
	q := strings.ToLower(query)
	filtered := make([]k8s.PodInfo, 0, len(pods))
	for _, p := range pods {
		name := p.NameLower
		if name == "" {
			name = strings.ToLower(p.Name)
		}
		if strings.Contains(name, q) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func buildStatusSummary(pods []k8s.PodInfo) header.StatusSummary {
	s := header.StatusSummary{}
	for _, p := range pods {
		switch p.Status {
		case k8s.StatusCrashLoopBack, k8s.StatusFailed:
			s.CritCrash++
		case k8s.StatusImagePullErr:
			s.CritImgErr++
		case k8s.StatusOOMKilled:
			s.CritOOM++
		case k8s.StatusEvicted:
			s.CritEvicted++
		case k8s.StatusPending:
			s.WarnPending++
		case k8s.StatusTerminating:
			s.WarnTerminating++
		case k8s.StatusRunning:
			s.OKRunning++
		case k8s.StatusCompleted:
			s.OKCompleted++
		}
	}
	return s
}

func (m Model) handleMetricsProbed(msg MetricsProbedMsg) (Model, tea.Cmd) {
	if !msg.Available {
		return m, nil
	}
	if err := m.client.EnableMetrics(); err != nil {
		return m, nil
	}
	newModel := m
	newModel.metricsAvailable = true
	newModel.podList = m.podList.SetMetricsAvailable(true)
	return newModel, newModel.fetchMetricsCmd()
}

func isShellEligibleStatus(status k8s.PodStatus) bool {
	switch status {
	case k8s.StatusCompleted, k8s.StatusFailed, k8s.StatusEvicted,
		k8s.StatusTerminating, k8s.StatusPending:
		return false
	default:
		return true
	}
}
