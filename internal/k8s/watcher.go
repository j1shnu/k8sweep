package k8s

import (
	"context"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const watchDebounceInterval = 200 * time.Millisecond

// PodWatcher watches pods in a namespace and sends debounced pod list
// updates through a channel. Uses a list+watch loop instead of informers
// for compatibility with fake clientsets in tests.
type PodWatcher struct {
	clientset kubernetes.Interface
	namespace string
	eventCh   chan []PodInfo
	triggerCh chan struct{}
	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once

	mu       sync.RWMutex
	pods     map[string]*corev1.Pod // keyed by namespace/name
	fatalErr error                  // set when a non-retriable error occurs (e.g. auth failure)
}

// NewPodWatcher creates a watcher for the given namespace.
// Use namespace "" for all namespaces.
func NewPodWatcher(clientset kubernetes.Interface, namespace string) *PodWatcher {
	return &PodWatcher{
		clientset: clientset,
		namespace: namespace,
		eventCh:   make(chan []PodInfo, 1),
		triggerCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		pods:      make(map[string]*corev1.Pod),
	}
}

// Start begins the list-watch loop and debounce goroutine.
// Safe to call multiple times — only the first call starts goroutines.
func (w *PodWatcher) Start() {
	w.startOnce.Do(func() {
		go w.listWatchLoop()
		go w.debounceLoop()
	})
}

// Stop shuts down the watcher. The Events channel is closed after
// the debounce loop exits.
func (w *PodWatcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

// Events returns a read-only channel that receives debounced pod list updates.
// Closed when the watcher stops.
func (w *PodWatcher) Events() <-chan []PodInfo {
	return w.eventCh
}

// ListPods returns the current pods from the local cache.
// Safe to call from any goroutine.
func (w *PodWatcher) ListPods() []PodInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()
	pods := make([]PodInfo, 0, len(w.pods))
	for _, pod := range w.pods {
		pods = append(pods, mapPodToInfo(*pod))
	}
	return pods
}

func (w *PodWatcher) notify() {
	select {
	case w.triggerCh <- struct{}{}:
	default:
	}
}

func podCacheKey(pod *corev1.Pod) string {
	return pod.Namespace + "/" + pod.Name
}

// FatalErr returns the non-retriable error that caused the watcher to stop,
// or nil if the watcher stopped normally.
func (w *PodWatcher) FatalErr() error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.fatalErr
}

// isNonRetriableErr returns true for errors that will never succeed on retry,
// such as authentication (401) or authorization (403) failures.
func isNonRetriableErr(err error) bool {
	return k8serrors.IsUnauthorized(err) || k8serrors.IsForbidden(err)
}

// listWatchLoop performs an initial list, then watches for changes.
// On watch errors, it re-lists and re-watches.
// On non-retriable errors (auth failures), it stops immediately.
func (w *PodWatcher) listWatchLoop() {
	for {
		select {
		case <-w.stopCh:
			return
		default:
		}

		resourceVersion, err := w.listPods()
		if err != nil {
			if isNonRetriableErr(err) {
				w.mu.Lock()
				w.fatalErr = err
				w.mu.Unlock()
				w.Stop()
				return
			}
			select {
			case <-w.stopCh:
				return
			case <-time.After(2 * time.Second):
				continue // retry list
			}
		}

		w.notify()
		w.watchPods(resourceVersion)
		// Watch ended (error or expired) — loop back to re-list
	}
}

func (w *PodWatcher) listPods() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := w.clientset.CoreV1().Pods(w.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	w.mu.Lock()
	// Clear and rebuild cache from list
	newPods := make(map[string]*corev1.Pod, len(list.Items))
	for i := range list.Items {
		pod := &list.Items[i]
		newPods[podCacheKey(pod)] = pod
	}
	w.pods = newPods
	w.mu.Unlock()

	return list.ResourceVersion, nil
}

func (w *PodWatcher) watchPods(resourceVersion string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel watch when stopCh closes
	go func() {
		select {
		case <-w.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	watcher, err := w.clientset.CoreV1().Pods(w.namespace).Watch(ctx, metav1.ListOptions{
		ResourceVersion: resourceVersion,
	})
	if err != nil {
		return
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		select {
		case <-w.stopCh:
			return
		default:
		}

		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}

		w.mu.Lock()
		switch event.Type {
		case watch.Added, watch.Modified:
			w.pods[podCacheKey(pod)] = pod
		case watch.Deleted:
			delete(w.pods, podCacheKey(pod))
		}
		w.mu.Unlock()

		w.notify()
	}
}

func (w *PodWatcher) debounceLoop() {
	defer close(w.eventCh)
	for {
		select {
		case <-w.stopCh:
			return
		case <-w.triggerCh:
			timer := time.NewTimer(watchDebounceInterval)
		drain:
			for {
				select {
				case <-w.triggerCh:
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(watchDebounceInterval)
				case <-timer.C:
					break drain
				case <-w.stopCh:
					timer.Stop()
					return
				}
			}
			pods := w.ListPods()
			select {
			case w.eventCh <- pods:
			case <-w.stopCh:
				return
			}
		}
	}
}
