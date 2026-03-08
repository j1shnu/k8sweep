package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetPodDetail(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels:    map[string]string{"app": "test", "version": "v1"},
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet", Name: "test-rs-abc123"},
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
			Containers: []corev1.Container{
				{
					Name:  "main",
					Image: "nginx:1.21",
					Ports: []corev1.ContainerPort{
						{ContainerPort: 80, Protocol: corev1.ProtocolTCP},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase:  corev1.PodRunning,
			PodIP:  "10.0.0.5",
			HostIP: "192.168.1.1",
			QOSClass: corev1.PodQOSBurstable,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "main",
					Ready:        true,
					RestartCount: 2,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
			},
		},
	}

	cs := fake.NewSimpleClientset(pod)
	client := NewClientFromClientset(cs, ClusterInfo{Namespace: "default"})

	detail, err := GetPodDetail(context.Background(), client, "default", "test-pod")
	require.NoError(t, err)

	assert.Equal(t, "test-pod", detail.Name)
	assert.Equal(t, "default", detail.Namespace)
	assert.Equal(t, StatusRunning, detail.Status)
	assert.Equal(t, "node-1", detail.Node)
	assert.Equal(t, "10.0.0.5", detail.PodIP)
	assert.Equal(t, "192.168.1.1", detail.HostIP)
	assert.Equal(t, "Burstable", detail.QoSClass)
	assert.Equal(t, "ReplicaSet/test-rs-abc123", detail.Owner)

	assert.Equal(t, "test", detail.Labels["app"])
	assert.Equal(t, "v1", detail.Labels["version"])

	require.Len(t, detail.Containers, 1)
	c := detail.Containers[0]
	assert.Equal(t, "main", c.Name)
	assert.Equal(t, "nginx:1.21", c.Image)
	assert.Equal(t, "Running", c.State)
	assert.True(t, c.Ready)
	assert.Equal(t, int32(2), c.RestartCount)
	require.Len(t, c.Ports, 1)
	assert.Equal(t, int32(80), c.Ports[0].ContainerPort)
	assert.Equal(t, "100m", c.Requests.CPU)
	assert.Equal(t, "128Mi", c.Requests.Memory)
	assert.Equal(t, "500m", c.Limits.CPU)
	assert.Equal(t, "256Mi", c.Limits.Memory)

	require.Len(t, detail.Conditions, 2)
	assert.Equal(t, "Ready", detail.Conditions[0].Type)
	assert.Equal(t, "True", detail.Conditions[0].Status)
}

func TestGetPodDetail_NotFound(t *testing.T) {
	cs := fake.NewSimpleClientset()
	client := NewClientFromClientset(cs, ClusterInfo{Namespace: "default"})

	_, err := GetPodDetail(context.Background(), client, "default", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get pod")
}

func TestGetPodDetail_NoOwner(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orphan-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "busybox"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	cs := fake.NewSimpleClientset(pod)
	client := NewClientFromClientset(cs, ClusterInfo{Namespace: "default"})

	detail, err := GetPodDetail(context.Background(), client, "default", "orphan-pod")
	require.NoError(t, err)
	assert.Empty(t, detail.Owner)
}

func TestGetPodDetail_WaitingContainer(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "waiting-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "busybox"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "main",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"},
					},
				},
			},
		},
	}

	cs := fake.NewSimpleClientset(pod)
	client := NewClientFromClientset(cs, ClusterInfo{Namespace: "default"})

	detail, err := GetPodDetail(context.Background(), client, "default", "waiting-pod")
	require.NoError(t, err)
	assert.Equal(t, "Waiting: ImagePullBackOff", detail.Containers[0].State)
}
