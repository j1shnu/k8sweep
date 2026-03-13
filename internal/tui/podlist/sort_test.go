package podlist

import (
	"testing"
	"time"

	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/stretchr/testify/assert"
)

func sortTestPods() []k8s.PodInfo {
	return []k8s.PodInfo{
		{Name: "charlie", Namespace: "default", Status: k8s.StatusRunning, Age: 2 * time.Hour, RestartCount: 3,
			Metrics: &k8s.PodMetrics{CPUMillicores: 500, MemoryBytes: 256 * 1024 * 1024}},
		{Name: "alpha", Namespace: "default", Status: k8s.StatusCrashLoopBack, Age: 48 * time.Hour, RestartCount: 10,
			Metrics: &k8s.PodMetrics{CPUMillicores: 100, MemoryBytes: 64 * 1024 * 1024}},
		{Name: "bravo", Namespace: "default", Status: k8s.StatusFailed, Age: 1 * time.Hour, RestartCount: 0},
		{Name: "delta", Namespace: "default", Status: k8s.StatusPending, Age: 5 * time.Minute, RestartCount: 0,
			Metrics: &k8s.PodMetrics{CPUMillicores: 50, MemoryBytes: 32 * 1024 * 1024}},
	}
}

func TestSortPods_ByNameAsc(t *testing.T) {
	pods := sortTestPods()
	sorted := sortPods(pods, SortByName, SortAsc)

	assert.Equal(t, "alpha", sorted[0].Name)
	assert.Equal(t, "bravo", sorted[1].Name)
	assert.Equal(t, "charlie", sorted[2].Name)
	assert.Equal(t, "delta", sorted[3].Name)
}

func TestSortPods_ByNameDesc(t *testing.T) {
	sorted := sortPods(sortTestPods(), SortByName, SortDesc)

	assert.Equal(t, "delta", sorted[0].Name)
	assert.Equal(t, "charlie", sorted[1].Name)
}

func TestSortPods_ByStatusAsc(t *testing.T) {
	sorted := sortPods(sortTestPods(), SortByStatus, SortAsc)

	// Running(0) < Pending(2) < Failed(5) < CrashLoop(8)
	assert.Equal(t, k8s.StatusRunning, sorted[0].Status)
	assert.Equal(t, k8s.StatusPending, sorted[1].Status)
	assert.Equal(t, k8s.StatusFailed, sorted[2].Status)
	assert.Equal(t, k8s.StatusCrashLoopBack, sorted[3].Status)
}

func TestSortPods_ByStatusDesc(t *testing.T) {
	sorted := sortPods(sortTestPods(), SortByStatus, SortDesc)

	assert.Equal(t, k8s.StatusCrashLoopBack, sorted[0].Status)
	assert.Equal(t, k8s.StatusFailed, sorted[1].Status)
}

func TestSortPods_ByAgeAsc(t *testing.T) {
	sorted := sortPods(sortTestPods(), SortByAge, SortAsc)

	assert.Equal(t, "delta", sorted[0].Name) // 5m
	assert.Equal(t, "bravo", sorted[1].Name) // 1h
}

func TestSortPods_ByAgeDesc(t *testing.T) {
	sorted := sortPods(sortTestPods(), SortByAge, SortDesc)

	assert.Equal(t, "alpha", sorted[0].Name) // 48h
}

func TestSortPods_ByRestartsDesc(t *testing.T) {
	sorted := sortPods(sortTestPods(), SortByRestarts, SortDesc)

	assert.Equal(t, "alpha", sorted[0].Name)   // 10
	assert.Equal(t, "charlie", sorted[1].Name)  // 3
}

func TestSortPods_ByCPUDesc(t *testing.T) {
	sorted := sortPods(sortTestPods(), SortByCPU, SortDesc)

	assert.Equal(t, "charlie", sorted[0].Name) // 500m
	assert.Equal(t, "alpha", sorted[1].Name)   // 100m
	assert.Equal(t, "delta", sorted[2].Name)   // 50m
	assert.Equal(t, "bravo", sorted[3].Name)   // nil → bottom
}

func TestSortPods_ByMemoryAsc(t *testing.T) {
	sorted := sortPods(sortTestPods(), SortByMemory, SortAsc)

	assert.Equal(t, "delta", sorted[0].Name)  // 32Mi
	assert.Equal(t, "alpha", sorted[1].Name)  // 64Mi
	assert.Equal(t, "charlie", sorted[2].Name) // 256Mi
	assert.Equal(t, "bravo", sorted[3].Name)  // nil → bottom
}

func TestSortPods_NilMetricsToBottom(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "no-metrics", Namespace: "default"},
		{Name: "has-metrics", Namespace: "default", Metrics: &k8s.PodMetrics{CPUMillicores: 100, MemoryBytes: 64}},
	}

	sortedCPU := sortPods(pods, SortByCPU, SortAsc)
	assert.Equal(t, "has-metrics", sortedCPU[0].Name)
	assert.Equal(t, "no-metrics", sortedCPU[1].Name)

	sortedMem := sortPods(pods, SortByMemory, SortDesc)
	assert.Equal(t, "has-metrics", sortedMem[0].Name)
	assert.Equal(t, "no-metrics", sortedMem[1].Name)
}

func TestSortPods_EmptySlice(t *testing.T) {
	sorted := sortPods(nil, SortByName, SortAsc)
	assert.Nil(t, sorted)
}

func TestSortPods_SingleItem(t *testing.T) {
	pods := []k8s.PodInfo{{Name: "solo"}}
	sorted := sortPods(pods, SortByName, SortAsc)
	assert.Len(t, sorted, 1)
	assert.Equal(t, "solo", sorted[0].Name)
}

func TestSortPods_DoesNotMutateOriginal(t *testing.T) {
	pods := sortTestPods()
	firstName := pods[0].Name
	_ = sortPods(pods, SortByName, SortAsc)
	assert.Equal(t, firstName, pods[0].Name) // original unchanged
}

func TestNextSortColumn_WithMetrics(t *testing.T) {
	col := SortByName
	col = NextSortColumn(col, true) // → Status
	assert.Equal(t, SortByStatus, col)
	col = NextSortColumn(col, true) // → Age
	assert.Equal(t, SortByAge, col)
	col = NextSortColumn(col, true) // → Restarts
	assert.Equal(t, SortByRestarts, col)
	col = NextSortColumn(col, true) // → Owner
	assert.Equal(t, SortByOwner, col)
	col = NextSortColumn(col, true) // → CPU
	assert.Equal(t, SortByCPU, col)
	col = NextSortColumn(col, true) // → Memory
	assert.Equal(t, SortByMemory, col)
	col = NextSortColumn(col, true) // → Name (wraps)
	assert.Equal(t, SortByName, col)
}

func TestNextSortColumn_WithoutMetrics(t *testing.T) {
	col := SortByName
	col = NextSortColumn(col, false)
	assert.Equal(t, SortByStatus, col)
	col = NextSortColumn(col, false)
	assert.Equal(t, SortByAge, col)
	col = NextSortColumn(col, false)
	assert.Equal(t, SortByRestarts, col)
	col = NextSortColumn(col, false) // → Owner
	assert.Equal(t, SortByOwner, col)
	col = NextSortColumn(col, false) // wraps to Name, skipping CPU/Mem
	assert.Equal(t, SortByName, col)
}

func TestSortColumnLabel(t *testing.T) {
	assert.Equal(t, "NAME", SortColumnLabel(SortByName))
	assert.Equal(t, "STATUS", SortColumnLabel(SortByStatus))
	assert.Equal(t, "AGE", SortColumnLabel(SortByAge))
	assert.Equal(t, "RESTARTS", SortColumnLabel(SortByRestarts))
	assert.Equal(t, "OWNER", SortColumnLabel(SortByOwner))
	assert.Equal(t, "CPU", SortColumnLabel(SortByCPU))
	assert.Equal(t, "MEM", SortColumnLabel(SortByMemory))
}

func TestSortIndicator(t *testing.T) {
	assert.Equal(t, "▲", SortIndicator(SortAsc))
	assert.Equal(t, "▼", SortIndicator(SortDesc))
}
