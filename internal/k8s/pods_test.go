package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newFakeClient(pods ...corev1.Pod) *Client {
	objects := make([]corev1.Pod, len(pods))
	copy(objects, pods)
	cs := fake.NewSimpleClientset()
	for i := range objects {
		_, _ = cs.CoreV1().Pods(objects[i].Namespace).Create(
			context.Background(), &objects[i], metav1.CreateOptions{},
		)
	}
	return NewClientFromClientset(cs, ClusterInfo{
		ContextName: "test-context",
		Namespace:   "default",
		Server:      "https://localhost:6443",
	})
}

func makePod(name, namespace string, phase corev1.PodPhase, reason string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
		},
		Status: corev1.PodStatus{
			Phase:  phase,
			Reason: reason,
		},
	}
}

func makePodWithContainerStatus(name, namespace string, phase corev1.PodPhase, waitingReason, terminatedReason string) corev1.Pod {
	pod := makePod(name, namespace, phase, "")
	cs := corev1.ContainerStatus{Name: "main"}
	if waitingReason != "" {
		cs.State = corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{Reason: waitingReason},
		}
	}
	if terminatedReason != "" {
		cs.State = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{Reason: terminatedReason},
		}
	}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{cs}
	return pod
}

func TestListPods(t *testing.T) {
	client := newFakeClient(
		makePod("running-pod", "default", corev1.PodRunning, ""),
		makePod("failed-pod", "default", corev1.PodFailed, ""),
	)

	pods, err := ListPods(context.Background(), client, "default")
	require.NoError(t, err)
	assert.Len(t, pods, 2)
}

func TestListPods_EmptyNamespace(t *testing.T) {
	client := newFakeClient()
	pods, err := ListPods(context.Background(), client, "default")
	require.NoError(t, err)
	assert.Empty(t, pods)
}

func TestFilterDirtyPods(t *testing.T) {
	pods := []PodInfo{
		{Name: "running", Status: StatusRunning},
		{Name: "failed", Status: StatusFailed},
		{Name: "completed", Status: StatusCompleted},
		{Name: "pending", Status: StatusPending},
		{Name: "evicted", Status: StatusEvicted},
		{Name: "crashloop", Status: StatusCrashLoopBack},
		{Name: "oomkilled", Status: StatusOOMKilled},
	}

	dirty := FilterDirtyPods(pods)
	assert.Len(t, dirty, 5)

	names := make([]string, len(dirty))
	for i, p := range dirty {
		names[i] = p.Name
	}
	assert.Contains(t, names, "failed")
	assert.Contains(t, names, "completed")
	assert.Contains(t, names, "evicted")
	assert.Contains(t, names, "crashloop")
	assert.Contains(t, names, "oomkilled")
}

func TestFilterDirtyPods_Empty(t *testing.T) {
	dirty := FilterDirtyPods(nil)
	assert.Empty(t, dirty)
}

func TestFilterDirtyPods_NoDirty(t *testing.T) {
	pods := []PodInfo{
		{Name: "running", Status: StatusRunning},
		{Name: "pending", Status: StatusPending},
	}
	dirty := FilterDirtyPods(pods)
	assert.Empty(t, dirty)
}

func TestDeletePods(t *testing.T) {
	client := newFakeClient(
		makePod("pod-1", "default", corev1.PodFailed, ""),
		makePod("pod-2", "default", corev1.PodSucceeded, ""),
	)

	toDelete := []PodInfo{
		{Name: "pod-1", Namespace: "default"},
		{Name: "pod-2", Namespace: "default"},
	}

	results := DeletePods(context.Background(), client, toDelete)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success, "expected pod %s deletion to succeed", r.PodName)
		assert.Nil(t, r.Error)
	}
}

func TestDeletePods_NotFound(t *testing.T) {
	client := newFakeClient()

	toDelete := []PodInfo{
		{Name: "nonexistent", Namespace: "default"},
	}

	results := DeletePods(context.Background(), client, toDelete)
	assert.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.NotNil(t, results[0].Error)
}

func TestDerivePodStatus_Evicted(t *testing.T) {
	pod := makePod("evicted", "default", corev1.PodFailed, "Evicted")
	info := mapPodToInfo(pod)
	assert.Equal(t, StatusEvicted, info.Status)
}

func TestDerivePodStatus_CrashLoopBackOff(t *testing.T) {
	pod := makePodWithContainerStatus("crashloop", "default", corev1.PodRunning, "CrashLoopBackOff", "")
	info := mapPodToInfo(pod)
	assert.Equal(t, StatusCrashLoopBack, info.Status)
}

func TestDerivePodStatus_OOMKilled(t *testing.T) {
	pod := makePodWithContainerStatus("oom", "default", corev1.PodRunning, "", "OOMKilled")
	info := mapPodToInfo(pod)
	assert.Equal(t, StatusOOMKilled, info.Status)
}

func TestDerivePodStatus_OOMKilledLastTermination_NotRunning(t *testing.T) {
	pod := makePod("oom-last", "default", corev1.PodRunning, "")
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			Name: "main",
			State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{Reason: "Error"},
			},
			LastTerminationState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled"},
			},
		},
	}
	info := mapPodToInfo(pod)
	assert.Equal(t, StatusOOMKilled, info.Status)
}

func TestDerivePodStatus_OOMKilledLastTermination_Recovered(t *testing.T) {
	// Container recovered from OOM and is now running — should show Running, not OOMKilled
	pod := makePod("oom-recovered", "default", corev1.PodRunning, "")
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			Name: "main",
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{},
			},
			LastTerminationState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled"},
			},
		},
	}
	info := mapPodToInfo(pod)
	assert.Equal(t, StatusRunning, info.Status)
}

func TestDerivePodStatus_Phases(t *testing.T) {
	tests := []struct {
		phase  corev1.PodPhase
		expect PodStatus
	}{
		{corev1.PodRunning, StatusRunning},
		{corev1.PodSucceeded, StatusCompleted},
		{corev1.PodFailed, StatusFailed},
		{corev1.PodPending, StatusPending},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			pod := makePod("test", "default", tt.phase, "")
			info := mapPodToInfo(pod)
			assert.Equal(t, tt.expect, info.Status)
		})
	}
}

func TestDerivePodStatus_Terminating(t *testing.T) {
	now := metav1.Now()
	pod := makePod("terminating", "default", corev1.PodRunning, "")
	pod.DeletionTimestamp = &now
	info := mapPodToInfo(pod)
	assert.Equal(t, StatusTerminating, info.Status)
}

func TestTotalRestartCount(t *testing.T) {
	pod := corev1.Pod{
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{RestartCount: 3},
				{RestartCount: 5},
			},
		},
	}
	assert.Equal(t, int32(8), totalRestartCount(pod))
}
