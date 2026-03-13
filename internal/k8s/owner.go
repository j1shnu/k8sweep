package k8s

import (
	"context"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const ownerResolveWorkers = 4

// ResolveOwners returns a new pod slice with resolved top-level controllers.
// ReplicaSet → Deployment, Job → CronJob chains are resolved via API lookups.
// On lookup errors, the pod keeps its immediate owner (graceful degradation).
func ResolveOwners(ctx context.Context, clientset kubernetes.Interface, pods []PodInfo) []PodInfo {
	// Build set of unique owners that need resolution
	type resolveKey struct {
		kind      ControllerKind
		namespace string
		name      string
	}
	toResolve := make(map[resolveKey]struct{})
	for _, p := range pods {
		switch p.Controller.Kind {
		case ControllerReplicaSet, ControllerJob:
			toResolve[resolveKey{p.Controller.Kind, p.Namespace, p.Controller.Name}] = struct{}{}
		}
	}

	if len(toResolve) == 0 {
		return pods
	}

	// Resolve owners in parallel with a worker pool
	type resolveResult struct {
		key    resolveKey
		result ControllerRef
	}

	keys := make([]resolveKey, 0, len(toResolve))
	for k := range toResolve {
		keys = append(keys, k)
	}

	resultsCh := make(chan resolveResult, len(keys))
	keyCh := make(chan resolveKey, len(keys))

	var wg sync.WaitGroup
	workers := ownerResolveWorkers
	if len(keys) < workers {
		workers = len(keys)
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := range keyCh {
				ref := resolveOne(ctx, clientset, k.kind, k.namespace, k.name)
				resultsCh <- resolveResult{key: k, result: ref}
			}
		}()
	}

	for _, k := range keys {
		keyCh <- k
	}
	close(keyCh)

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	resolved := make(map[resolveKey]ControllerRef, len(keys))
	for r := range resultsCh {
		resolved[r.key] = r.result
	}

	// Build new pod slice with resolved controllers
	result := make([]PodInfo, len(pods))
	for i, p := range pods {
		result[i] = p
		switch p.Controller.Kind {
		case ControllerReplicaSet, ControllerJob:
			k := resolveKey{p.Controller.Kind, p.Namespace, p.Controller.Name}
			if ref, ok := resolved[k]; ok {
				result[i].Controller = ref
			}
		}
	}
	return result
}

// resolveOne resolves a single owner reference to its top-level controller.
func resolveOne(ctx context.Context, clientset kubernetes.Interface, kind ControllerKind, namespace, name string) ControllerRef {
	switch kind {
	case ControllerReplicaSet:
		return resolveReplicaSet(ctx, clientset, namespace, name)
	case ControllerJob:
		return resolveJob(ctx, clientset, namespace, name)
	default:
		return ControllerRef{Kind: kind, Name: name}
	}
}

// resolveReplicaSet checks if a ReplicaSet is owned by a Deployment.
func resolveReplicaSet(ctx context.Context, clientset kubernetes.Interface, namespace, name string) ControllerRef {
	rs, err := clientset.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ControllerRef{Kind: ControllerReplicaSet, Name: name}
	}
	for _, ref := range rs.OwnerReferences {
		if ref.Kind == "Deployment" {
			return ControllerRef{Kind: ControllerDeployment, Name: ref.Name}
		}
	}
	return ControllerRef{Kind: ControllerReplicaSet, Name: name}
}

// resolveJob checks if a Job is owned by a CronJob.
func resolveJob(ctx context.Context, clientset kubernetes.Interface, namespace, name string) ControllerRef {
	job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ControllerRef{Kind: ControllerJob, Name: name}
	}
	for _, ref := range job.OwnerReferences {
		if ref.Kind == "CronJob" {
			return ControllerRef{Kind: ControllerCronJob, Name: ref.Name}
		}
	}
	return ControllerRef{Kind: ControllerJob, Name: name}
}
