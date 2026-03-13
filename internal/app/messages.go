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
// Err is non-nil when the watcher stopped due to a non-retriable error (e.g. auth failure).
type WatchStoppedMsg struct {
	WatchID uint64
	Err     error
}

// PodsDeletedMsg is sent after a batch delete operation completes.
// Individual errors are captured per-DeleteResult.
type PodsDeletedMsg struct {
	Results    []k8s.DeleteResult
	ForceDelete bool
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

// PodShellExitedMsg is sent when an interactive pod shell session exits.
type PodShellExitedMsg struct {
	PodKey    string
	Container string
	Backend   string
	ShellPath string
	Err       error
}

// MetricsProbedMsg is sent after the async metrics API probe completes.
type MetricsProbedMsg struct {
	Available bool
}

// TickMsg triggers periodic metrics refresh.
type TickMsg struct{}

// LoadingTickMsg triggers a spinner/fact rotation while loading.
type LoadingTickMsg struct{}

// PodEventsLoadedMsg is sent when pod events have been fetched.
type PodEventsLoadedMsg struct {
	Events []k8s.PodEvent
	Err    error
	PodKey string // "namespace/name" to detect stale responses
}

// PodLogsLoadedMsg is sent when pod logs have been fetched.
type PodLogsLoadedMsg struct {
	Lines     []string
	Container string
	Err       error
	PodKey    string // "namespace/name" to detect stale responses
}

// SearchDebouncedMsg applies search filtering after a short debounce delay.
// Seq is used to discard stale debounce ticks.
type SearchDebouncedMsg struct {
	Seq   uint64
	Query string
}

// OwnerResolvedMsg is sent when async ownership resolution completes.
type OwnerResolvedMsg struct {
	Pods    []k8s.PodInfo
	FetchID uint64 // matches fetchID or watchID to discard stale results
}

// PrefsSavedMsg is sent after preferences have been written to disk.
// Err is non-nil if the save failed (silently ignored by the UI).
type PrefsSavedMsg struct {
	Err error
}
