package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
)

const loadingTickInterval = 80 * time.Millisecond

// nsSpinnerFrames are the animation frames for the namespace loading spinner.
var nsSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// startAndWatchCmd starts the watcher and waits for the first event.
func (m Model) startAndWatchCmd() tea.Cmd {
	if m.watcher == nil {
		return nil
	}
	watcher := m.watcher
	id := m.watchID
	return func() tea.Msg {
		watcher.Start()
		pods, ok := <-watcher.Events()
		if !ok {
			return WatchStoppedMsg{WatchID: id}
		}
		return WatchPodsMsg{Pods: pods, WatchID: id}
	}
}

// watchPodsCmd waits for the next event from the watcher.
func (m Model) watchPodsCmd() tea.Cmd {
	if m.watcher == nil {
		return nil
	}
	ch := m.watcher.Events()
	id := m.watchID
	return func() tea.Msg {
		pods, ok := <-ch
		if !ok {
			return WatchStoppedMsg{WatchID: id}
		}
		return WatchPodsMsg{Pods: pods, WatchID: id}
	}
}

// fetchPodsCmd creates a command that fetches pods (used for manual refresh fallback).
func (m *Model) fetchPodsCmd() tea.Cmd {
	id := fetchSeq.Add(1)
	m.fetchID = id
	ns := m.namespace
	client := m.client
	return func() tea.Msg {
		if ns == k8s.AllNamespaces {
			ctx, cancel := context.WithTimeout(context.Background(), fetchTimeoutAllNS)
			defer cancel()
			pods, err := k8s.ListPodsAllNamespaces(ctx, client)
			return PodsLoadedMsg{Pods: pods, Err: err, FetchID: id}
		}
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		pods, err := k8s.ListPods(ctx, client, ns)
		return PodsLoadedMsg{Pods: pods, Err: err, FetchID: id}
	}
}

func (m Model) fetchMetricsCmd() tea.Cmd {
	if !m.metricsAvailable {
		return nil
	}
	ns := m.namespace
	mc := m.client.GetMetricsClient()
	timeout := fetchTimeout
	if ns == k8s.AllNamespaces {
		timeout = fetchTimeoutAllNS
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		metrics := k8s.FetchPodMetrics(ctx, mc, ns)
		return MetricsLoadedMsg{Metrics: metrics, Namespace: ns}
	}
}

func (m Model) deletePodsCmd(pods []k8s.PodInfo) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), deleteTimeout)
		defer cancel()
		results := k8s.DeletePods(ctx, client, pods)
		return PodsDeletedMsg{Results: results, ForceDelete: false}
	}
}

func (m Model) forceDeletePodsCmd(pods []k8s.PodInfo) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), deleteTimeout)
		defer cancel()
		results := k8s.ForceDeletePods(ctx, client, pods)
		return PodsDeletedMsg{Results: results, ForceDelete: true}
	}
}

func (m Model) fetchNamespacesCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ns, err := client.ListNamespaces(ctx)
		return NamespacesLoadedMsg{Namespaces: ns, Err: err}
	}
}

func (m Model) fetchPodDetailCmd(namespace, name string) tea.Cmd {
	client := m.client
	podKey := namespace + "/" + name
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		detail, err := k8s.GetPodDetail(ctx, client, namespace, name)
		return PodDetailLoadedMsg{Detail: detail, Err: err, PodKey: podKey}
	}
}

const metricsProbeTimeout = 3 * time.Second

func (m Model) probeMetricsCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), metricsProbeTimeout)
		defer cancel()
		available := k8s.CheckMetricsAvailable(ctx, client)
		return MetricsProbedMsg{Available: available}
	}
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(metricsInterval, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

// loadingTickCmd sends a LoadingTickMsg after a short interval for spinner animation.
func loadingTickCmd() tea.Cmd {
	return tea.Tick(loadingTickInterval, func(time.Time) tea.Msg {
		return LoadingTickMsg{}
	})
}

func (m Model) fetchPodEventsCmd(namespace, name string) tea.Cmd {
	client := m.client
	podKey := namespace + "/" + name
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		events, err := k8s.GetPodEvents(ctx, client, namespace, name)
		return PodEventsLoadedMsg{Events: events, Err: err, PodKey: podKey}
	}
}

func (m Model) fetchPodLogsCmd(namespace, name, container string) tea.Cmd {
	client := m.client
	podKey := namespace + "/" + name
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		lines, err := k8s.GetPodLogs(ctx, client, namespace, name, container, k8s.DefaultTailLines)
		return PodLogsLoadedMsg{Lines: lines, Container: container, Err: err, PodKey: podKey}
	}
}

func searchDebounceCmd(seq uint64, query string) tea.Cmd {
	return tea.Tick(searchDebounce, func(time.Time) tea.Msg {
		return SearchDebouncedMsg{Seq: seq, Query: query}
	})
}
