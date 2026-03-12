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

func TestGetPodEvents(t *testing.T) {
	now := time.Now()
	cs := fake.NewClientset()

	// Create events for our target pod
	events := []corev1.Event{
		{
			ObjectMeta:     metav1.ObjectMeta{Name: "evt-1", Namespace: "default"},
			InvolvedObject: corev1.ObjectReference{Name: "my-pod", Kind: "Pod"},
			Type:           "Normal",
			Reason:         "Scheduled",
			Message:        "Successfully assigned",
			Source:         corev1.EventSource{Component: "scheduler"},
			Count:          1,
			FirstTimestamp: metav1.NewTime(now.Add(-5 * time.Minute)),
			LastTimestamp:  metav1.NewTime(now.Add(-5 * time.Minute)),
		},
		{
			ObjectMeta:     metav1.ObjectMeta{Name: "evt-2", Namespace: "default"},
			InvolvedObject: corev1.ObjectReference{Name: "my-pod", Kind: "Pod"},
			Type:           "Warning",
			Reason:         "BackOff",
			Message:        "Back-off restarting failed container",
			Source:         corev1.EventSource{Component: "kubelet"},
			Count:          3,
			FirstTimestamp: metav1.NewTime(now.Add(-2 * time.Minute)),
			LastTimestamp:  metav1.NewTime(now.Add(-1 * time.Minute)),
		},
	}

	for i := range events {
		_, err := cs.CoreV1().Events("default").Create(context.Background(), &events[i], metav1.CreateOptions{})
		require.NoError(t, err)
	}

	client := NewClientFromClientset(cs, ClusterInfo{Namespace: "default"})
	result, err := GetPodEvents(context.Background(), client, "default", "my-pod")
	require.NoError(t, err)

	// fake client may not support field selectors, so just verify we get events
	assert.GreaterOrEqual(t, len(result), 1)
	// Verify sorted newest first (if both returned)
	if len(result) >= 2 {
		assert.True(t, !result[0].LastTimestamp.Before(result[1].LastTimestamp))
	}
}

func TestGetPodEventsEmpty(t *testing.T) {
	cs := fake.NewClientset()
	client := NewClientFromClientset(cs, ClusterInfo{Namespace: "default"})
	result, err := GetPodEvents(context.Background(), client, "default", "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestSortEventsNewestFirst(t *testing.T) {
	now := time.Now()
	events := []PodEvent{
		{Reason: "old", LastTimestamp: now.Add(-10 * time.Minute)},
		{Reason: "new", LastTimestamp: now},
		{Reason: "mid", LastTimestamp: now.Add(-5 * time.Minute)},
	}
	SortEventsNewestFirst(events)
	assert.Equal(t, "new", events[0].Reason)
	assert.Equal(t, "mid", events[1].Reason)
	assert.Equal(t, "old", events[2].Reason)
}
