package k8s

import "time"

// PodStatus represents the derived status of a pod.
type PodStatus string

const (
	StatusRunning       PodStatus = "Running"
	StatusCompleted     PodStatus = "Completed"
	StatusFailed        PodStatus = "Failed"
	StatusEvicted       PodStatus = "Evicted"
	StatusCrashLoopBack PodStatus = "CrashLoopBackOff"
	StatusImagePullErr  PodStatus = "ImagePullError"
	StatusOOMKilled     PodStatus = "OOMKilled"
	StatusPending       PodStatus = "Pending"
	StatusTerminating   PodStatus = "Terminating"
	StatusUnknown       PodStatus = "Unknown"
)

// dirtyStatuses is the set of pod statuses considered "dirty" and eligible for cleanup.
var dirtyStatuses = map[PodStatus]struct{}{
	StatusCompleted:     {},
	StatusFailed:        {},
	StatusEvicted:       {},
	StatusCrashLoopBack: {},
	StatusImagePullErr:  {},
	StatusOOMKilled:     {},
}

// ControllerKind identifies the type of workload controller that owns a pod.
type ControllerKind string

const (
	ControllerDeployment  ControllerKind = "Deployment"
	ControllerStatefulSet ControllerKind = "StatefulSet"
	ControllerDaemonSet   ControllerKind = "DaemonSet"
	ControllerJob         ControllerKind = "Job"
	ControllerCronJob     ControllerKind = "CronJob"
	ControllerReplicaSet  ControllerKind = "ReplicaSet"
	ControllerStandalone  ControllerKind = "Standalone"
)

// controllerFilterOrder defines the cycle order for the controller filter toggle.
var controllerFilterOrder = []ControllerKind{
	"", // All (no filter)
	ControllerDeployment,
	ControllerStatefulSet,
	ControllerDaemonSet,
	ControllerJob,
	ControllerCronJob,
	ControllerStandalone,
}

// NextControllerFilter returns the next controller kind in the cycle order.
func NextControllerFilter(current ControllerKind) ControllerKind {
	for i, k := range controllerFilterOrder {
		if k == current {
			return controllerFilterOrder[(i+1)%len(controllerFilterOrder)]
		}
	}
	return ""
}

// ControllerRef identifies the resolved top-level controller for a pod.
type ControllerRef struct {
	Kind ControllerKind
	Name string
}

// String returns "Kind/Name" or "Standalone" for display.
func (c ControllerRef) String() string {
	if c.Kind == ControllerStandalone || c.Kind == "" {
		return string(c.Kind)
	}
	return string(c.Kind) + "/" + c.Name
}

// PodMetrics holds CPU and memory usage for a single pod.
type PodMetrics struct {
	CPUMillicores int64
	MemoryBytes   int64
}

// StuckTerminatingThreshold is the duration after which a Terminating pod
// is considered "stuck" and eligible for cleanup via the dirty filter.
const StuckTerminatingThreshold = 5 * time.Minute

// PodInfo holds the display-relevant information for a single pod.
type PodInfo struct {
	Name         string
	NameLower    string
	Namespace    string
	Status       PodStatus
	Age          time.Duration
	RestartCount int32
	NodeName     string
	OwnerRef     string        // e.g. "ReplicaSet/my-app-abc123", empty for standalone pods
	Controller   ControllerRef // resolved top-level controller (Deployment, not ReplicaSet)
	Metrics      *PodMetrics   // nil when metrics are unavailable
	DeletionTime *time.Time    // non-nil when pod is terminating (from DeletionTimestamp)
}

// IsStandalone returns true if the pod has no owning controller.
func (p PodInfo) IsStandalone() bool {
	return p.Controller.Kind == ControllerStandalone
}

// IsDirty returns true if the pod's status is in the dirty set,
// or if the pod has been stuck in Terminating state beyond the threshold.
func (p PodInfo) IsDirty() bool {
	if _, ok := dirtyStatuses[p.Status]; ok {
		return true
	}
	if p.Status == StatusTerminating && p.DeletionTime != nil {
		return time.Since(*p.DeletionTime) >= StuckTerminatingThreshold
	}
	return false
}

// GetName returns the pod name (implements ResourceItem).
func (p PodInfo) GetName() string { return p.Name }

// GetNamespace returns the pod namespace (implements ResourceItem).
func (p PodInfo) GetNamespace() string { return p.Namespace }

// GetStatus returns the pod status string (implements ResourceItem).
func (p PodInfo) GetStatus() string { return string(p.Status) }

// AllNamespaces is the sentinel value indicating all-namespaces mode.
// When Namespace is set to this value, pods from all namespaces are fetched.
const AllNamespaces = ""

// ClusterInfo holds the current cluster connection details.
type ClusterInfo struct {
	ContextName string
	Namespace   string
	Server      string
}

// ResourceFilter controls which pods are displayed.
type ResourceFilter struct {
	ShowDirtyOnly      bool
	ControllerKindFilter ControllerKind // empty means show all
}

// DeleteResult holds the outcome of a single pod deletion.
type DeleteResult struct {
	PodName   string
	Namespace string
	Success   bool
	Error     error
}

// PodEvent holds a single Kubernetes event for a pod.
type PodEvent struct {
	Type           string // "Normal" or "Warning"
	Reason         string
	Message        string
	Source         string // component that generated the event
	Count          int32
	FirstTimestamp time.Time
	LastTimestamp   time.Time
}
