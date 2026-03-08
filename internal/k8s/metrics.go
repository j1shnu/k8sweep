package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// MetricsClient wraps access to the Kubernetes Metrics API.
type MetricsClient struct {
	clientset metricsclient.Interface
}

// CheckMetricsAvailable probes whether the metrics.k8s.io API group is registered.
// Handles partial errors from ServerGroupsAndResources, which are common when some
// API groups are unavailable — the returned resource list is still checked.
func CheckMetricsAvailable(ctx context.Context, client *Client) bool {
	// ServerGroupsAndResources may return a partial list alongside a non-nil error
	// when some API groups are unreachable. We still check the partial results.
	_, resources, _ := client.clientset.Discovery().ServerGroupsAndResources()
	for _, r := range resources {
		if r.GroupVersion == "metrics.k8s.io/v1beta1" {
			return true
		}
	}
	return false
}

// NewMetricsClient creates a MetricsClient from the same rest config used by the main client.
func NewMetricsClient(cs metricsclient.Interface) *MetricsClient {
	return &MetricsClient{clientset: cs}
}

// FetchPodMetrics fetches pod metrics for a namespace (or all namespaces when ns is "").
// Returns a map keyed by "namespace/name" with aggregated CPU/memory across all containers.
// Returns an empty map (not error) on failure for graceful degradation.
func FetchPodMetrics(ctx context.Context, mc *MetricsClient, ns string) map[string]PodMetrics {
	if mc == nil {
		return nil
	}

	var podMetricsList *metricsv1beta1.PodMetricsList
	var err error

	if ns == AllNamespaces {
		podMetricsList, err = mc.clientset.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	} else {
		podMetricsList, err = mc.clientset.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil
	}

	result := make(map[string]PodMetrics, len(podMetricsList.Items))
	for _, pm := range podMetricsList.Items {
		var cpuTotal, memTotal int64
		for _, c := range pm.Containers {
			cpuTotal += c.Usage.Cpu().MilliValue()
			memTotal += c.Usage.Memory().Value()
		}
		key := pm.Namespace + "/" + pm.Name
		result[key] = PodMetrics{
			CPUMillicores: cpuTotal,
			MemoryBytes:   memTotal,
		}
	}
	return result
}

// MergePodMetrics returns a new slice with Metrics populated from the metrics map.
// Pods without a matching entry get nil Metrics. The original slice is not modified.
func MergePodMetrics(pods []PodInfo, metrics map[string]PodMetrics) []PodInfo {
	if metrics == nil {
		return pods
	}
	merged := make([]PodInfo, len(pods))
	for i, p := range pods {
		merged[i] = p
		key := p.Namespace + "/" + p.Name
		if m, ok := metrics[key]; ok {
			merged[i].Metrics = &PodMetrics{
				CPUMillicores: m.CPUMillicores,
				MemoryBytes:   m.MemoryBytes,
			}
		}
	}
	return merged
}
