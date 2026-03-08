package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListPods fetches all pods in the given namespace and maps them to PodInfo.
func ListPods(ctx context.Context, client *Client, ns string) ([]PodInfo, error) {
	podList, err := client.Clientset().CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		if ns == AllNamespaces {
			return nil, fmt.Errorf("failed to list pods across all namespaces: %w", err)
		}
		return nil, fmt.Errorf("failed to list pods in namespace %q: %w", ns, err)
	}

	pods := make([]PodInfo, 0, len(podList.Items))
	for _, pod := range podList.Items {
		pods = append(pods, mapPodToInfo(pod))
	}
	return pods, nil
}

// FilterDirtyPods returns a new slice containing only dirty pods.
func FilterDirtyPods(pods []PodInfo) []PodInfo {
	dirty := make([]PodInfo, 0, len(pods))
	for _, p := range pods {
		if p.IsDirty() {
			dirty = append(dirty, p)
		}
	}
	return dirty
}

// DeletePods deletes the given pods and returns results for each.
// Stops early if the context is cancelled.
func DeletePods(ctx context.Context, client *Client, pods []PodInfo) []DeleteResult {
	results := make([]DeleteResult, 0, len(pods))
	for _, pod := range pods {
		if err := ctx.Err(); err != nil {
			results = append(results, DeleteResult{
				PodName:   pod.Name,
				Namespace: pod.Namespace,
				Success:   false,
				Error:     err,
			})
			continue
		}
		err := client.Clientset().CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		results = append(results, DeleteResult{
			PodName:   pod.Name,
			Namespace: pod.Namespace,
			Success:   err == nil,
			Error:     err,
		})
	}
	return results
}

// mapPodToInfo maps a Kubernetes Pod to the domain PodInfo type.
func mapPodToInfo(pod corev1.Pod) PodInfo {
	return PodInfo{
		Name:         pod.Name,
		Namespace:    pod.Namespace,
		Status:       derivePodStatus(pod),
		Age:          time.Since(pod.CreationTimestamp.Time),
		RestartCount: totalRestartCount(pod),
		NodeName:     pod.Spec.NodeName,
	}
}

// derivePodStatus determines the display status from the Kubernetes pod object.
// This mirrors kubectl's logic: check pod-level reason first, then container statuses.
func derivePodStatus(pod corev1.Pod) PodStatus {
	// Check pod-level reason first (Evicted shows up here)
	if pod.Status.Reason == "Evicted" {
		return StatusEvicted
	}

	// Check if pod is being deleted
	if pod.DeletionTimestamp != nil {
		return StatusTerminating
	}

	// Check container statuses for CrashLoopBackOff and OOMKilled
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
			return StatusCrashLoopBack
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason == "OOMKilled" {
			return StatusOOMKilled
		}
		if cs.LastTerminationState.Terminated != nil &&
			cs.LastTerminationState.Terminated.Reason == "OOMKilled" &&
			cs.State.Running == nil {
			return StatusOOMKilled
		}
	}

	// Check init container statuses
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
			return StatusCrashLoopBack
		}
		if cs.State.Terminated != nil && cs.State.Terminated.Reason == "OOMKilled" {
			return StatusOOMKilled
		}
	}

	// Map pod phase
	switch pod.Status.Phase {
	case corev1.PodRunning:
		return StatusRunning
	case corev1.PodSucceeded:
		return StatusCompleted
	case corev1.PodFailed:
		return StatusFailed
	case corev1.PodPending:
		return StatusPending
	default:
		return StatusUnknown
	}
}

// totalRestartCount sums restart counts across all containers.
func totalRestartCount(pod corev1.Pod) int32 {
	var total int32
	for _, cs := range pod.Status.ContainerStatuses {
		total += cs.RestartCount
	}
	return total
}
