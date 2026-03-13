package podlist

import (
	"testing"
	"time"

	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func treeTestPods() []k8s.PodInfo {
	return []k8s.PodInfo{
		{Name: "nginx-abc-1", Namespace: "default", Status: k8s.StatusRunning, Age: 2 * time.Hour,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "nginx"}},
		{Name: "nginx-abc-2", Namespace: "default", Status: k8s.StatusCrashLoopBack, Age: 1 * time.Hour,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "nginx"}},
		{Name: "redis-0", Namespace: "default", Status: k8s.StatusRunning, Age: 5 * time.Hour,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerStatefulSet, Name: "redis"}},
		{Name: "debug-pod", Namespace: "default", Status: k8s.StatusRunning, Age: 5 * time.Minute,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerStandalone}},
		{Name: "worker-job-xyz", Namespace: "default", Status: k8s.StatusCompleted, Age: 30 * time.Minute,
			Controller: k8s.ControllerRef{Kind: k8s.ControllerJob, Name: "worker-job"}},
	}
}

func TestGroupPodsByController(t *testing.T) {
	groups := GroupPodsByController(treeTestPods(), SortByName, SortAsc)

	require.Len(t, groups, 4)
	// Alphabetical: Deployment/nginx, Job/worker-job, StatefulSet/redis, Standalone (last)
	assert.Equal(t, "Deployment/nginx", groups[0].Key)
	assert.Equal(t, "Job/worker-job", groups[1].Key)
	assert.Equal(t, "StatefulSet/redis", groups[2].Key)
	assert.Equal(t, "Standalone", groups[3].Key)
}

func TestGroupPodsByController_StandaloneAlwaysLast(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "standalone-1", Controller: k8s.ControllerRef{Kind: k8s.ControllerStandalone}},
		{Name: "deploy-1", Controller: k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "app"}},
	}
	groups := GroupPodsByController(pods, SortByName, SortAsc)

	require.Len(t, groups, 2)
	assert.Equal(t, "Deployment/app", groups[0].Key)
	assert.Equal(t, "Standalone", groups[1].Key)
}

func TestGroupPodsByController_SortsPodsWithinGroup(t *testing.T) {
	groups := GroupPodsByController(treeTestPods(), SortByName, SortAsc)

	// Deployment/nginx group should have pods sorted by name
	require.Len(t, groups[0].Pods, 2)
	assert.Equal(t, "nginx-abc-1", groups[0].Pods[0].Name)
	assert.Equal(t, "nginx-abc-2", groups[0].Pods[1].Name)
}

func TestGroupPodsByController_Empty(t *testing.T) {
	groups := GroupPodsByController(nil, SortByName, SortAsc)
	assert.Nil(t, groups)
}

func TestGroupPodsByController_EmptyControllerKind(t *testing.T) {
	pods := []k8s.PodInfo{
		{Name: "pod-a", Controller: k8s.ControllerRef{}},
		{Name: "pod-b", Controller: k8s.ControllerRef{Kind: k8s.ControllerStandalone}},
	}
	groups := GroupPodsByController(pods, SortByName, SortAsc)

	// Both should end up in Standalone group
	require.Len(t, groups, 1)
	assert.Equal(t, "Standalone", groups[0].Key)
	assert.Len(t, groups[0].Pods, 2)
}

func TestBuildDisplayRows_AllExpanded(t *testing.T) {
	groups := GroupPodsByController(treeTestPods(), SortByName, SortAsc)
	rows := BuildDisplayRows(groups, map[string]struct{}{})

	// 4 headers + 5 pods = 9 rows
	assert.Len(t, rows, 9)
	assert.Equal(t, RowController, rows[0].Kind) // Deployment/nginx header
	assert.Equal(t, RowPod, rows[1].Kind)         // nginx-abc-1
	assert.Equal(t, RowPod, rows[2].Kind)         // nginx-abc-2
	assert.Equal(t, RowController, rows[3].Kind)  // Job/worker-job header
	assert.Equal(t, RowPod, rows[4].Kind)         // worker-job-xyz
	assert.Equal(t, RowController, rows[5].Kind)  // StatefulSet/redis header
	assert.Equal(t, RowPod, rows[6].Kind)         // redis-0
	assert.Equal(t, RowController, rows[7].Kind)  // Standalone header
	assert.Equal(t, RowPod, rows[8].Kind)         // debug-pod
}

func TestBuildDisplayRows_SomeCollapsed(t *testing.T) {
	groups := GroupPodsByController(treeTestPods(), SortByName, SortAsc)
	collapsed := map[string]struct{}{"Deployment/nginx": {}}
	rows := BuildDisplayRows(groups, collapsed)

	// nginx collapsed: 1 header, no pods. Others expanded: 3 headers + 3 pods = 7
	assert.Len(t, rows, 7)
	assert.Equal(t, RowController, rows[0].Kind)
	assert.Equal(t, "Deployment/nginx", rows[0].GroupKey)
	// Next should be Job header (nginx pods hidden)
	assert.Equal(t, RowController, rows[1].Kind)
	assert.Equal(t, "Job/worker-job", rows[1].GroupKey)
}

func TestBuildDisplayRows_AllCollapsed(t *testing.T) {
	groups := GroupPodsByController(treeTestPods(), SortByName, SortAsc)
	collapsed := map[string]struct{}{
		"Deployment/nginx":  {},
		"Job/worker-job":    {},
		"StatefulSet/redis": {},
		"Standalone":        {},
	}
	rows := BuildDisplayRows(groups, collapsed)

	// Only 4 header rows
	assert.Len(t, rows, 4)
	for _, r := range rows {
		assert.Equal(t, RowController, r.Kind)
	}
}

func TestBuildDisplayRows_Empty(t *testing.T) {
	rows := BuildDisplayRows(nil, map[string]struct{}{})
	assert.Nil(t, rows)
}

func TestControllerGroupStatusCounts(t *testing.T) {
	groups := GroupPodsByController(treeTestPods(), SortByName, SortAsc)
	counts := groups[0].StatusCounts() // Deployment/nginx
	assert.Equal(t, 1, counts[k8s.StatusRunning])
	assert.Equal(t, 1, counts[k8s.StatusCrashLoopBack])
}

func TestFirstPodRowIndex(t *testing.T) {
	groups := GroupPodsByController(treeTestPods(), SortByName, SortAsc)
	rows := BuildDisplayRows(groups, map[string]struct{}{})

	// First row is a controller header, first pod is at index 1
	assert.Equal(t, 1, firstPodRowIndex(rows))
}

func TestFirstPodRowIndex_AllCollapsed(t *testing.T) {
	groups := GroupPodsByController(treeTestPods(), SortByName, SortAsc)
	collapsed := map[string]struct{}{
		"Deployment/nginx":  {},
		"Job/worker-job":    {},
		"StatefulSet/redis": {},
		"Standalone":        {},
	}
	rows := BuildDisplayRows(groups, collapsed)

	// No pod rows, should return 0
	assert.Equal(t, 0, firstPodRowIndex(rows))
}

func TestFirstPodRowIndex_Empty(t *testing.T) {
	assert.Equal(t, 0, firstPodRowIndex(nil))
}

func TestControllerGroupKey(t *testing.T) {
	tests := []struct {
		ref  k8s.ControllerRef
		want string
	}{
		{k8s.ControllerRef{Kind: k8s.ControllerDeployment, Name: "nginx"}, "Deployment/nginx"},
		{k8s.ControllerRef{Kind: k8s.ControllerStatefulSet, Name: "redis"}, "StatefulSet/redis"},
		{k8s.ControllerRef{Kind: k8s.ControllerStandalone}, "Standalone"},
		{k8s.ControllerRef{}, "Standalone"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, controllerGroupKey(tt.ref))
	}
}
