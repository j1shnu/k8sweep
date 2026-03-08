package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergePodMetrics_WithMetrics(t *testing.T) {
	pods := []PodInfo{
		{Name: "pod-a", Namespace: "ns1"},
		{Name: "pod-b", Namespace: "ns1"},
		{Name: "pod-c", Namespace: "ns2"},
	}
	metrics := map[string]PodMetrics{
		"ns1/pod-a": {CPUMillicores: 100, MemoryBytes: 1024 * 1024},
		"ns2/pod-c": {CPUMillicores: 250, MemoryBytes: 512 * 1024 * 1024},
	}

	merged := MergePodMetrics(pods, metrics)

	// Original slice unmodified
	assert.Nil(t, pods[0].Metrics)
	assert.Nil(t, pods[2].Metrics)

	// Merged slice has metrics
	assert.NotNil(t, merged[0].Metrics)
	assert.Equal(t, int64(100), merged[0].Metrics.CPUMillicores)
	assert.Equal(t, int64(1024*1024), merged[0].Metrics.MemoryBytes)

	// Pod without metrics
	assert.Nil(t, merged[1].Metrics)

	// Pod in different namespace
	assert.NotNil(t, merged[2].Metrics)
	assert.Equal(t, int64(250), merged[2].Metrics.CPUMillicores)
}

func TestMergePodMetrics_NilMetrics(t *testing.T) {
	pods := []PodInfo{{Name: "pod-a", Namespace: "ns1"}}
	result := MergePodMetrics(pods, nil)
	// Should return original slice when metrics is nil
	assert.Equal(t, pods, result)
}

func TestMergePodMetrics_EmptyPods(t *testing.T) {
	metrics := map[string]PodMetrics{
		"ns1/pod-a": {CPUMillicores: 100, MemoryBytes: 1024},
	}
	merged := MergePodMetrics(nil, metrics)
	assert.Empty(t, merged)
}

func TestMergePodMetrics_EmptyMetrics(t *testing.T) {
	pods := []PodInfo{{Name: "pod-a", Namespace: "ns1"}}
	merged := MergePodMetrics(pods, map[string]PodMetrics{})
	assert.Len(t, merged, 1)
	assert.Nil(t, merged[0].Metrics)
}
