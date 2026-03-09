package app

import "github.com/jprasad/k8sweep/internal/k8s"

// PodsLoadedMsg is sent when pod data has been fetched from the cluster.
type PodsLoadedMsg struct {
	Pods    []k8s.PodInfo
	Err     error
	FetchID uint64
}

// WatchPodsMsg is sent when the pod watcher delivers a debounced update.
type WatchPodsMsg struct {
	Pods    []k8s.PodInfo
	WatchID uint64
}

// WatchStoppedMsg is sent when the pod watcher's channel closes.
type WatchStoppedMsg struct {
	WatchID uint64
}

// PodsDeletedMsg is sent after a batch delete operation completes.
// Individual errors are captured per-DeleteResult.
type PodsDeletedMsg struct {
	Results []k8s.DeleteResult
}

// NamespacesLoadedMsg is sent when the namespace list has been fetched.
type NamespacesLoadedMsg struct {
	Namespaces []string
	Err        error
}

// MetricsLoadedMsg is sent when pod metrics have been fetched from the cluster.
type MetricsLoadedMsg struct {
	Metrics   map[string]k8s.PodMetrics
	Namespace string // discard if namespace has changed
}

// PodDetailLoadedMsg is sent when pod detail has been fetched.
type PodDetailLoadedMsg struct {
	Detail *k8s.PodDetail
	Err    error
	PodKey string // "namespace/name" to detect stale responses
}

// TickMsg triggers periodic metrics refresh.
type TickMsg struct{}

// LoadingTickMsg triggers a spinner/fact rotation while loading.
type LoadingTickMsg struct{}

// SearchDebouncedMsg applies search filtering after a short debounce delay.
// Seq is used to discard stale debounce ticks.
type SearchDebouncedMsg struct {
	Seq   uint64
	Query string
}
