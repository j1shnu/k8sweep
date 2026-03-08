package app

import "github.com/jprasad/k8sweep/internal/k8s"

// PodsLoadedMsg is sent when pod data has been fetched from the cluster.
type PodsLoadedMsg struct {
	Pods    []k8s.PodInfo
	Err     error
	FetchID uint64
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

// TickMsg triggers a periodic pod refresh.
type TickMsg struct{}

// LoadingTickMsg triggers a spinner/fact rotation while loading.
type LoadingTickMsg struct{}
