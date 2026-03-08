package k8s

import "time"

// PodStatus represents the derived status of a pod.
type PodStatus string

const (
	StatusRunning        PodStatus = "Running"
	StatusCompleted      PodStatus = "Completed"
	StatusFailed         PodStatus = "Failed"
	StatusEvicted        PodStatus = "Evicted"
	StatusCrashLoopBack  PodStatus = "CrashLoopBackOff"
	StatusOOMKilled      PodStatus = "OOMKilled"
	StatusPending        PodStatus = "Pending"
	StatusTerminating    PodStatus = "Terminating"
	StatusUnknown        PodStatus = "Unknown"
)

// dirtyStatuses is the set of pod statuses considered "dirty" and eligible for cleanup.
var dirtyStatuses = map[PodStatus]struct{}{
	StatusCompleted:     {},
	StatusFailed:        {},
	StatusEvicted:       {},
	StatusCrashLoopBack: {},
	StatusOOMKilled:     {},
}

// PodInfo holds the display-relevant information for a single pod.
type PodInfo struct {
	Name         string
	Namespace    string
	Status       PodStatus
	Age          time.Duration
	RestartCount int32
	NodeName     string
}

// IsDirty returns true if the pod's status is in the dirty set.
func (p PodInfo) IsDirty() bool {
	_, ok := dirtyStatuses[p.Status]
	return ok
}

// GetName returns the pod name (implements ResourceItem).
func (p PodInfo) GetName() string { return p.Name }

// GetNamespace returns the pod namespace (implements ResourceItem).
func (p PodInfo) GetNamespace() string { return p.Namespace }

// GetStatus returns the pod status string (implements ResourceItem).
func (p PodInfo) GetStatus() string { return string(p.Status) }

// ClusterInfo holds the current cluster connection details.
type ClusterInfo struct {
	ContextName string
	Namespace   string
	Server      string
}

// ResourceFilter controls which pods are displayed.
type ResourceFilter struct {
	ShowDirtyOnly bool
}

// DeleteResult holds the outcome of a single pod deletion.
type DeleteResult struct {
	PodName   string
	Namespace string
	Success   bool
	Error     error
}
