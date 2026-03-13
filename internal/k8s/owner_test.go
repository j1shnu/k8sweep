package k8s

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResolveOwners_ReplicaSetToDeployment(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-app-abc123",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", Name: "my-app"},
				},
			},
		},
	)

	pods := []PodInfo{
		{
			Name:       "my-app-abc123-xyz",
			Namespace:  "default",
			Controller: ControllerRef{Kind: ControllerReplicaSet, Name: "my-app-abc123"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := ResolveOwners(ctx, cs, pods)

	if result[0].Controller.Kind != ControllerDeployment {
		t.Errorf("expected Deployment, got %s", result[0].Controller.Kind)
	}
	if result[0].Controller.Name != "my-app" {
		t.Errorf("expected my-app, got %s", result[0].Controller.Name)
	}
}

func TestResolveOwners_JobToCronJob(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cleanup-12345",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "CronJob", Name: "cleanup"},
				},
			},
		},
	)

	pods := []PodInfo{
		{
			Name:       "cleanup-12345-abc",
			Namespace:  "default",
			Controller: ControllerRef{Kind: ControllerJob, Name: "cleanup-12345"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := ResolveOwners(ctx, cs, pods)

	if result[0].Controller.Kind != ControllerCronJob {
		t.Errorf("expected CronJob, got %s", result[0].Controller.Kind)
	}
	if result[0].Controller.Name != "cleanup" {
		t.Errorf("expected cleanup, got %s", result[0].Controller.Name)
	}
}

func TestResolveOwners_OrphanedReplicaSet(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "orphan-rs",
				Namespace: "default",
				// No ownerReferences
			},
		},
	)

	pods := []PodInfo{
		{
			Name:       "orphan-rs-xyz",
			Namespace:  "default",
			Controller: ControllerRef{Kind: ControllerReplicaSet, Name: "orphan-rs"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := ResolveOwners(ctx, cs, pods)

	if result[0].Controller.Kind != ControllerReplicaSet {
		t.Errorf("expected ReplicaSet, got %s", result[0].Controller.Kind)
	}
}

func TestResolveOwners_DirectControllers(t *testing.T) {
	cs := fake.NewSimpleClientset()

	pods := []PodInfo{
		{Name: "sts-pod", Namespace: "default", Controller: ControllerRef{Kind: ControllerStatefulSet, Name: "my-sts"}},
		{Name: "ds-pod", Namespace: "default", Controller: ControllerRef{Kind: ControllerDaemonSet, Name: "my-ds"}},
		{Name: "standalone", Namespace: "default", Controller: ControllerRef{Kind: ControllerStandalone}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := ResolveOwners(ctx, cs, pods)

	if result[0].Controller.Kind != ControllerStatefulSet {
		t.Errorf("expected StatefulSet, got %s", result[0].Controller.Kind)
	}
	if result[1].Controller.Kind != ControllerDaemonSet {
		t.Errorf("expected DaemonSet, got %s", result[1].Controller.Kind)
	}
	if result[2].Controller.Kind != ControllerStandalone {
		t.Errorf("expected Standalone, got %s", result[2].Controller.Kind)
	}
}

func TestResolveOwners_APIError(t *testing.T) {
	// Empty clientset — GET ReplicaSet will fail with not found
	cs := fake.NewSimpleClientset()

	pods := []PodInfo{
		{
			Name:       "pod-1",
			Namespace:  "default",
			Controller: ControllerRef{Kind: ControllerReplicaSet, Name: "missing-rs"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := ResolveOwners(ctx, cs, pods)

	// Should gracefully keep ReplicaSet
	if result[0].Controller.Kind != ControllerReplicaSet {
		t.Errorf("expected ReplicaSet on error, got %s", result[0].Controller.Kind)
	}
	if result[0].Controller.Name != "missing-rs" {
		t.Errorf("expected missing-rs, got %s", result[0].Controller.Name)
	}
}

func TestResolveOwners_CacheReuse(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared-rs",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", Name: "my-deploy"},
				},
			},
		},
	)

	pods := []PodInfo{
		{Name: "pod-1", Namespace: "default", Controller: ControllerRef{Kind: ControllerReplicaSet, Name: "shared-rs"}},
		{Name: "pod-2", Namespace: "default", Controller: ControllerRef{Kind: ControllerReplicaSet, Name: "shared-rs"}},
		{Name: "pod-3", Namespace: "default", Controller: ControllerRef{Kind: ControllerReplicaSet, Name: "shared-rs"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := ResolveOwners(ctx, cs, pods)

	for i, p := range result {
		if p.Controller.Kind != ControllerDeployment {
			t.Errorf("pod %d: expected Deployment, got %s", i, p.Controller.Kind)
		}
		if p.Controller.Name != "my-deploy" {
			t.Errorf("pod %d: expected my-deploy, got %s", i, p.Controller.Name)
		}
	}
}

func TestResolveOwners_NoPods(t *testing.T) {
	cs := fake.NewSimpleClientset()
	result := ResolveOwners(context.Background(), cs, nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestResolveOwners_NothingToResolve(t *testing.T) {
	cs := fake.NewSimpleClientset()
	pods := []PodInfo{
		{Name: "standalone", Namespace: "default", Controller: ControllerRef{Kind: ControllerStandalone}},
	}
	result := ResolveOwners(context.Background(), cs, pods)
	if len(result) != 1 || result[0].Controller.Kind != ControllerStandalone {
		t.Errorf("unexpected result: %v", result)
	}
}
