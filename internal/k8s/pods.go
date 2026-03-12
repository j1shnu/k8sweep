package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/pager"
)

// listPageSize is the number of pods to fetch per API call.
// Matches kubectl's default chunk size.
const listPageSize = 500

const deleteWorkerCount = 12

// ListPods fetches all pods in the given namespace using the client-go pager,
// the same pagination mechanism kubectl uses internally. The pager handles
// continue tokens, resource expiration fallback, and background page buffering.
func ListPods(ctx context.Context, client *Client, ns string) ([]PodInfo, error) {
	p := pager.New(func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		return client.Clientset().CoreV1().Pods(ns).List(ctx, opts)
	})
	p.PageSize = listPageSize

	var pods []PodInfo
	err := p.EachListItem(ctx, metav1.ListOptions{}, func(obj runtime.Object) error {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return fmt.Errorf("unexpected object type: %T", obj)
		}
		pods = append(pods, mapPodToInfo(*pod))
		return nil
	})
	if err != nil {
		if ns == AllNamespaces {
			return nil, fmt.Errorf("failed to list pods across all namespaces: %w", err)
		}
		return nil, fmt.Errorf("failed to list pods in namespace %q: %w", ns, err)
	}

	return pods, nil
}

// ListPodsAllNamespaces fetches pods across all namespaces using the client-go
// pager with Pods("").List — the exact same approach kubectl uses for `get po -A`.
// No custom parallelism or QPS tuning needed.
func ListPodsAllNamespaces(ctx context.Context, client *Client) ([]PodInfo, error) {
	pods, err := ListPods(ctx, client, AllNamespaces)
	if err != nil {
		return nil, err
	}

	// Copy before sorting to avoid mutating the original slice
	sorted := make([]PodInfo, len(pods))
	copy(sorted, pods)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Namespace != sorted[j].Namespace {
			return sorted[i].Namespace < sorted[j].Namespace
		}
		return sorted[i].Name < sorted[j].Name
	})

	return sorted, nil
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
	return deletePodsWithOptions(ctx, client, pods, metav1.DeleteOptions{})
}

// mapPodToInfo maps a Kubernetes Pod to the domain PodInfo type.
func mapPodToInfo(pod corev1.Pod) PodInfo {
	owner := ""
	if len(pod.OwnerReferences) > 0 {
		ref := pod.OwnerReferences[0]
		owner = ref.Kind + "/" + ref.Name
	}
	var deletionTime *time.Time
	if pod.DeletionTimestamp != nil {
		t := pod.DeletionTimestamp.Time
		deletionTime = &t
	}
	return PodInfo{
		Name:         pod.Name,
		NameLower:    strings.ToLower(pod.Name),
		Namespace:    pod.Namespace,
		Status:       derivePodStatus(pod),
		Age:          time.Since(pod.CreationTimestamp.Time),
		RestartCount: totalRestartCount(pod),
		NodeName:     pod.Spec.NodeName,
		OwnerRef:     owner,
		DeletionTime: deletionTime,
	}
}

// ForceDeletePods deletes the given pods with GracePeriodSeconds=0,
// bypassing graceful shutdown. Used for stuck Terminating pods.
func ForceDeletePods(ctx context.Context, client *Client, pods []PodInfo) []DeleteResult {
	var zero int64
	return deletePodsWithOptions(ctx, client, pods, metav1.DeleteOptions{
		GracePeriodSeconds: &zero,
	})
}

func deletePodsWithOptions(ctx context.Context, client *Client, pods []PodInfo, opts metav1.DeleteOptions) []DeleteResult {
	if len(pods) == 0 {
		return nil
	}

	type deleteTask struct {
		index int
		pod   PodInfo
	}
	type indexedResult struct {
		index  int
		result DeleteResult
	}

	workerCount := deleteWorkerCount
	if len(pods) < workerCount {
		workerCount = len(pods)
	}

	tasks := make(chan deleteTask, len(pods))
	resultsCh := make(chan indexedResult, len(pods))

	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				err := ctx.Err()
				if err == nil {
					err = client.Clientset().CoreV1().Pods(task.pod.Namespace).Delete(ctx, task.pod.Name, opts)
				}
				resultsCh <- indexedResult{
					index: task.index,
					result: DeleteResult{
						PodName:   task.pod.Name,
						Namespace: task.pod.Namespace,
						Success:   err == nil,
						Error:     err,
					},
				}
			}
		}()
	}

	for i, pod := range pods {
		tasks <- deleteTask{index: i, pod: pod}
	}
	close(tasks)

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	ordered := make([]DeleteResult, len(pods))
	for result := range resultsCh {
		ordered[result.index] = result.result
	}
	return ordered
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
		if cs.State.Waiting != nil &&
			(cs.State.Waiting.Reason == "ImagePullBackOff" || cs.State.Waiting.Reason == "ErrImagePull") {
			return StatusImagePullErr
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
		if cs.State.Waiting != nil &&
			(cs.State.Waiting.Reason == "ImagePullBackOff" || cs.State.Waiting.Reason == "ErrImagePull") {
			return StatusImagePullErr
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
