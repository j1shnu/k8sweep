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

func TestPodWatcher_InitialList(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	cs := fake.NewSimpleClientset(pod)
	w := NewPodWatcher(cs, "default")
	w.Start()
	defer w.Stop()

	select {
	case pods := <-w.Events():
		require.Len(t, pods, 1)
		assert.Equal(t, "test-pod", pods[0].Name)
		assert.Equal(t, StatusRunning, pods[0].Status)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for initial pod list")
	}
}

func TestPodWatcher_DynamicUpdate(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "initial-pod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	cs := fake.NewSimpleClientset(pod)
	w := NewPodWatcher(cs, "default")
	w.Start()
	defer w.Stop()

	// Wait for initial list
	<-w.Events()

	// Add a new pod
	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new-pod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{Phase: corev1.PodPending},
	}
	_, err := cs.CoreV1().Pods("default").Create(context.Background(), newPod, metav1.CreateOptions{})
	require.NoError(t, err)

	// Wait for watch event
	select {
	case pods := <-w.Events():
		assert.Len(t, pods, 2)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for watch update")
	}
}

func TestPodWatcher_Stop_ClosesChannel(t *testing.T) {
	cs := fake.NewSimpleClientset()
	w := NewPodWatcher(cs, "default")
	w.Start()

	// Drain initial event if any
	select {
	case <-w.Events():
	case <-time.After(2 * time.Second):
	}

	w.Stop()

	select {
	case _, ok := <-w.Events():
		assert.False(t, ok, "expected channel to be closed")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}

func TestPodWatcher_ListPods_FromCache(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cached-pod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	cs := fake.NewSimpleClientset(pod)
	w := NewPodWatcher(cs, "default")
	w.Start()
	defer w.Stop()

	// Wait for cache sync via first event
	<-w.Events()

	pods := w.ListPods()
	require.Len(t, pods, 1)
	assert.Equal(t, "cached-pod", pods[0].Name)
}

func TestPodWatcher_StopIsIdempotent(t *testing.T) {
	cs := fake.NewSimpleClientset()
	w := NewPodWatcher(cs, "default")
	w.Start()

	// Should not panic
	w.Stop()
	w.Stop()
	w.Stop()
}
