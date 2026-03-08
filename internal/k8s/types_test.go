package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPodInfo_IsDirty(t *testing.T) {
	tests := []struct {
		name   string
		status PodStatus
		want   bool
	}{
		{"Running is not dirty", StatusRunning, false},
		{"Pending is not dirty", StatusPending, false},
		{"Terminating is not dirty", StatusTerminating, false},
		{"Unknown is not dirty", StatusUnknown, false},
		{"Completed is dirty", StatusCompleted, true},
		{"Failed is dirty", StatusFailed, true},
		{"Evicted is dirty", StatusEvicted, true},
		{"CrashLoopBackOff is dirty", StatusCrashLoopBack, true},
		{"OOMKilled is dirty", StatusOOMKilled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := PodInfo{Name: "test", Status: tt.status}
			assert.Equal(t, tt.want, pod.IsDirty())
		})
	}
}

func TestPodInfo_GetName(t *testing.T) {
	pod := PodInfo{Name: "my-pod", Namespace: "default"}
	assert.Equal(t, "my-pod", pod.GetName())
	assert.Equal(t, "default", pod.GetNamespace())
	assert.Equal(t, "", pod.GetStatus()) // empty status
}

func TestPodInfo_GetStatus(t *testing.T) {
	pod := PodInfo{Status: StatusRunning}
	assert.Equal(t, "Running", pod.GetStatus())
}
