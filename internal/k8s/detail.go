package k8s

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodDetail holds comprehensive information for a single pod.
type PodDetail struct {
	Name               string
	Namespace          string
	Status             PodStatus
	Node               string
	Age                time.Duration
	PodIP              string
	HostIP             string
	QoSClass           string
	Owner              string // raw ownerRef e.g. "ReplicaSet/my-app-abc123"
	ResolvedController string // resolved top-level controller e.g. "Deployment/my-app"
	Labels             map[string]string
	Annotations        map[string]string
	Containers         []ContainerDetail
	Conditions         []PodCondition
}

// ContainerDetail holds per-container information.
type ContainerDetail struct {
	Name         string
	Image        string
	Ports        []ContainerPort
	State        string
	Ready        bool
	RestartCount int32
	Requests     ResourceList
	Limits       ResourceList
}

// ContainerPort describes a network port on a container.
type ContainerPort struct {
	ContainerPort int32
	Protocol      string
}

// ResourceList holds CPU and memory resource strings.
type ResourceList struct {
	CPU    string
	Memory string
}

// PodCondition describes the state of a pod condition.
type PodCondition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

// GetPodDetail fetches full details for a single pod via the Kubernetes API.
func GetPodDetail(ctx context.Context, client *Client, namespace, name string) (*PodDetail, error) {
	pod, err := client.Clientset().CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, name, err)
	}

	owner := ""
	if len(pod.OwnerReferences) > 0 {
		ref := pod.OwnerReferences[0]
		owner = ref.Kind + "/" + ref.Name
	}

	containers := make([]ContainerDetail, 0, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		cd := ContainerDetail{
			Name:  c.Name,
			Image: c.Image,
		}

		for _, p := range c.Ports {
			cd.Ports = append(cd.Ports, ContainerPort{
				ContainerPort: p.ContainerPort,
				Protocol:      string(p.Protocol),
			})
		}

		// Resource requests and limits
		if req := c.Resources.Requests; req != nil {
			cd.Requests = ResourceList{
				CPU:    req.Cpu().String(),
				Memory: req.Memory().String(),
			}
		}
		if lim := c.Resources.Limits; lim != nil {
			cd.Limits = ResourceList{
				CPU:    lim.Cpu().String(),
				Memory: lim.Memory().String(),
			}
		}

		// Find container status
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == c.Name {
				cd.Ready = cs.Ready
				cd.RestartCount = cs.RestartCount
				if cs.State.Running != nil {
					cd.State = "Running"
				} else if cs.State.Waiting != nil {
					cd.State = "Waiting: " + cs.State.Waiting.Reason
				} else if cs.State.Terminated != nil {
					cd.State = "Terminated: " + cs.State.Terminated.Reason
				}
				break
			}
		}

		containers = append(containers, cd)
	}

	conditions := make([]PodCondition, 0, len(pod.Status.Conditions))
	for _, c := range pod.Status.Conditions {
		conditions = append(conditions, PodCondition{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		})
	}

	return &PodDetail{
		Name:        pod.Name,
		Namespace:   pod.Namespace,
		Status:      derivePodStatus(*pod),
		Node:        pod.Spec.NodeName,
		Age:         time.Since(pod.CreationTimestamp.Time),
		PodIP:       pod.Status.PodIP,
		HostIP:      pod.Status.HostIP,
		QoSClass:    string(pod.Status.QOSClass),
		Owner:       owner,
		Labels:      pod.Labels,
		Annotations: pod.Annotations,
		Containers:  containers,
		Conditions:  conditions,
	}, nil
}
