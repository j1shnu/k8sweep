package podlist

import (
	"sort"
	"strings"

	"github.com/jprasad/k8sweep/internal/k8s"
)

// SortColumn identifies which column to sort by.
type SortColumn int

const (
	SortByName SortColumn = iota
	SortByStatus
	SortByAge
	SortByRestarts
	SortByOwner
	SortByCPU
	SortByMemory
)

// SortOrder indicates ascending or descending sort.
type SortOrder int

const (
	SortAsc SortOrder = iota
	SortDesc
)

// sortColumnCount is the total number of sort columns (used for cycling).
const sortColumnCount = 7

// sortColumnCountNoMetrics is the count when metrics are unavailable.
const sortColumnCountNoMetrics = 5

// statusSeverity maps pod statuses to a severity rank for sorting.
// Lower values = healthier; higher values = more problematic (sorted first in desc).
var statusSeverity = map[k8s.PodStatus]int{
	k8s.StatusRunning:       0,
	k8s.StatusCompleted:     1,
	k8s.StatusPending:       2,
	k8s.StatusTerminating:   3,
	k8s.StatusUnknown:       4,
	k8s.StatusFailed:        5,
	k8s.StatusEvicted:       6,
	k8s.StatusOOMKilled:     7,
	k8s.StatusImagePullErr:  8,
	k8s.StatusCrashLoopBack: 9,
}

// NextSortColumn cycles to the next sort column. CPU/Mem are skipped when
// metricsAvailable is false.
func NextSortColumn(current SortColumn, metricsAvailable bool) SortColumn {
	max := sortColumnCount
	if !metricsAvailable {
		max = sortColumnCountNoMetrics
	}
	next := SortColumn((int(current) + 1) % max)
	return next
}

// SortColumnLabel returns a human-readable label for the sort column.
func SortColumnLabel(col SortColumn) string {
	switch col {
	case SortByName:
		return "NAME"
	case SortByStatus:
		return "STATUS"
	case SortByAge:
		return "AGE"
	case SortByRestarts:
		return "RESTARTS"
	case SortByOwner:
		return "OWNER"
	case SortByCPU:
		return "CPU"
	case SortByMemory:
		return "MEM"
	default:
		return "NAME"
	}
}

// ParseSortColumn converts a label string (e.g. "NAME", "STATUS") back to a SortColumn.
// Returns SortByName for unrecognized labels.
func ParseSortColumn(label string) SortColumn {
	switch label {
	case "NAME":
		return SortByName
	case "STATUS":
		return SortByStatus
	case "AGE":
		return SortByAge
	case "RESTARTS":
		return SortByRestarts
	case "OWNER":
		return SortByOwner
	case "CPU":
		return SortByCPU
	case "MEM":
		return SortByMemory
	default:
		return SortByName
	}
}

// ParseSortOrder converts a string ("asc" or "desc") back to a SortOrder.
// Returns SortAsc for unrecognized values.
func ParseSortOrder(s string) SortOrder {
	if s == "desc" {
		return SortDesc
	}
	return SortAsc
}

// SortIndicator returns ▲ for ascending or ▼ for descending.
func SortIndicator(order SortOrder) string {
	if order == SortAsc {
		return "▲"
	}
	return "▼"
}

// sortPods returns a sorted copy of the given pod slice. The original is not modified.
// For CPU/Mem columns, pods with nil Metrics sort to the bottom regardless of order.
func sortPods(pods []k8s.PodInfo, col SortColumn, order SortOrder) []k8s.PodInfo {
	if len(pods) <= 1 {
		return pods
	}

	sorted := make([]k8s.PodInfo, len(pods))
	copy(sorted, pods)

	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]

		switch col {
		case SortByName:
			cmp := strings.Compare(a.Name, b.Name)
			if order == SortDesc {
				return cmp > 0
			}
			return cmp < 0

		case SortByStatus:
			sa := statusSeverity[a.Status]
			sb := statusSeverity[b.Status]
			if order == SortDesc {
				return sa > sb
			}
			return sa < sb

		case SortByAge:
			if order == SortDesc {
				return a.Age > b.Age
			}
			return a.Age < b.Age

		case SortByRestarts:
			if order == SortDesc {
				return a.RestartCount > b.RestartCount
			}
			return a.RestartCount < b.RestartCount

		case SortByOwner:
			aKey := string(a.Controller.Kind) + "/" + a.Controller.Name
			bKey := string(b.Controller.Kind) + "/" + b.Controller.Name
			cmp := strings.Compare(aKey, bKey)
			if order == SortDesc {
				return cmp > 0
			}
			return cmp < 0

		case SortByCPU:
			aNil := a.Metrics == nil
			bNil := b.Metrics == nil
			if aNil != bNil {
				return bNil // nil sorts to bottom
			}
			if aNil {
				return false
			}
			if order == SortDesc {
				return a.Metrics.CPUMillicores > b.Metrics.CPUMillicores
			}
			return a.Metrics.CPUMillicores < b.Metrics.CPUMillicores

		case SortByMemory:
			aNil := a.Metrics == nil
			bNil := b.Metrics == nil
			if aNil != bNil {
				return bNil // nil sorts to bottom
			}
			if aNil {
				return false
			}
			if order == SortDesc {
				return a.Metrics.MemoryBytes > b.Metrics.MemoryBytes
			}
			return a.Metrics.MemoryBytes < b.Metrics.MemoryBytes
		}

		return false
	})

	return sorted
}
