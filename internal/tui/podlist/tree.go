package podlist

import (
	"sort"
	"strings"

	"github.com/jprasad/k8sweep/internal/k8s"
)

// RowKind distinguishes controller header rows from pod rows in the tree view.
type RowKind int

const (
	// RowController is a collapsible group header showing the controller kind/name.
	RowController RowKind = iota
	// RowPod is an individual pod row indented under its controller group.
	RowPod
)

// DisplayRow represents a single row in the tree view — either a controller
// header or a pod. Pagination, cursor, and navigation all operate on a flat
// []DisplayRow slice.
type DisplayRow struct {
	Kind     RowKind
	GroupKey string           // "Deployment/nginx-app" or "Standalone"
	Header   *ControllerGroup // non-nil for RowController
	Pod      *k8s.PodInfo     // non-nil for RowPod
}

// ControllerGroup holds a controller and all its pods.
type ControllerGroup struct {
	Key        string
	Controller k8s.ControllerRef
	Pods       []k8s.PodInfo
}

// StatusCounts returns a map of status → count for the group's pods.
func (g ControllerGroup) StatusCounts() map[k8s.PodStatus]int {
	counts := make(map[k8s.PodStatus]int, len(g.Pods))
	for _, p := range g.Pods {
		counts[p.Status]++
	}
	return counts
}

// GroupPodsByController groups pods by their resolved controller, sorts pods
// within each group using the given sort column/order, and returns groups
// sorted alphabetically by key with Standalone always last.
func GroupPodsByController(pods []k8s.PodInfo, sortCol SortColumn, sortOrder SortOrder) []ControllerGroup {
	if len(pods) == 0 {
		return nil
	}

	groupMap := make(map[string]*ControllerGroup)
	var keys []string

	for i := range pods {
		p := pods[i]
		key := ControllerGroupKey(p.Controller)
		g, ok := groupMap[key]
		if !ok {
			g = &ControllerGroup{
				Key:        key,
				Controller: p.Controller,
			}
			groupMap[key] = g
			keys = append(keys, key)
		}
		g.Pods = append(g.Pods, p)
	}

	// Sort pods within each group
	for _, g := range groupMap {
		g.Pods = sortPods(g.Pods, sortCol, sortOrder)
	}

	// Sort groups alphabetically, Standalone always last
	sort.SliceStable(keys, func(i, j int) bool {
		aStandalone := keys[i] == "Standalone"
		bStandalone := keys[j] == "Standalone"
		if aStandalone != bStandalone {
			return bStandalone
		}
		return strings.Compare(keys[i], keys[j]) < 0
	})

	groups := make([]ControllerGroup, 0, len(keys))
	for _, key := range keys {
		groups = append(groups, *groupMap[key])
	}
	return groups
}

// BuildDisplayRows flattens controller groups into a display row slice.
// Collapsed groups show only the header row; expanded groups show header + pods.
func BuildDisplayRows(groups []ControllerGroup, collapsed map[string]struct{}) []DisplayRow {
	if len(groups) == 0 {
		return nil
	}

	var rows []DisplayRow
	for i := range groups {
		g := &groups[i]
		rows = append(rows, DisplayRow{
			Kind:     RowController,
			GroupKey: g.Key,
			Header:   g,
		})
		if _, isCollapsed := collapsed[g.Key]; !isCollapsed {
			for j := range g.Pods {
				rows = append(rows, DisplayRow{
					Kind:     RowPod,
					GroupKey: g.Key,
					Pod:      &g.Pods[j],
				})
			}
		}
	}
	return rows
}

// ControllerGroupKey returns the display group key for a controller reference.
// Used for tree grouping and drill-down filtering.
func ControllerGroupKey(ref k8s.ControllerRef) string {
	if ref.Kind == k8s.ControllerStandalone || ref.Kind == "" {
		return "Standalone"
	}
	return string(ref.Kind) + "/" + ref.Name
}

// firstPodRowIndex returns the index of the first RowPod in the display rows,
// or 0 if none exist.
func firstPodRowIndex(rows []DisplayRow) int {
	for i, r := range rows {
		if r.Kind == RowPod {
			return i
		}
	}
	return 0
}
