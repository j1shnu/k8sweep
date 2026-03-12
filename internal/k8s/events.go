package k8s

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetPodEvents fetches recent Kubernetes events for the given pod, sorted newest first.
func GetPodEvents(ctx context.Context, client *Client, namespace, podName string) ([]PodEvent, error) {
	selector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName)
	eventList, err := client.Clientset().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list events for pod %s/%s: %w", namespace, podName, err)
	}

	events := make([]PodEvent, 0, len(eventList.Items))
	for _, e := range eventList.Items {
		events = append(events, PodEvent{
			Type:           e.Type,
			Reason:         e.Reason,
			Message:        e.Message,
			Source:         e.Source.Component,
			Count:          e.Count,
			FirstTimestamp: e.FirstTimestamp.Time,
			LastTimestamp:   e.LastTimestamp.Time,
		})
	}

	SortEventsNewestFirst(events)
	return events, nil
}

// SortEventsNewestFirst sorts events by LastTimestamp descending.
func SortEventsNewestFirst(events []PodEvent) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.After(events[j].LastTimestamp)
	})
}
