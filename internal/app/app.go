package app

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/common"
	"github.com/jprasad/k8sweep/internal/tui/confirm"
	"github.com/jprasad/k8sweep/internal/tui/footer"
	"github.com/jprasad/k8sweep/internal/tui/header"
	"github.com/jprasad/k8sweep/internal/tui/namespace"
	"github.com/jprasad/k8sweep/internal/tui/podlist"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// appState represents the current UI state.
type appState int

const (
	stateBrowsing appState = iota
	stateConfirming
	stateSwitchingNamespace
)

const (
	pollInterval      = 10 * time.Second
	pollIntervalAllNS = 60 * time.Second
	fetchTimeout      = 15 * time.Second
	fetchTimeoutAllNS = 120 * time.Second
)

// fetchSeq is a global atomic counter used to tag pod fetch requests.
// Stale responses (from a previous fetch) are discarded by comparing sequence numbers.
var fetchSeq atomic.Uint64

// Model is the top-level Bubble Tea model.
type Model struct {
	client    *k8s.Client
	keys      KeyMap
	state     appState
	filter    k8s.ResourceFilter
	namespace string

	// fetchID tracks the most recent pod fetch request sequence number.
	// Responses with a different ID are discarded as stale.
	fetchID uint64

	header     header.Model
	podList    podlist.Model
	footer     footer.Model
	confirm    confirm.Model
	nsSwitcher namespace.Model

	allPods        []k8s.PodInfo // cached full pod list for client-side filtering
	totalPodCount  int
	statusMsg      string
	err            error
	width          int
	height         int
	nsLoading      bool // true while fetching namespaces
	nsSpinnerFrame int  // spinner animation frame for namespace loading
}

// NewModel creates the initial application model.
func NewModel(client *k8s.Client) Model {
	info := client.GetClusterInfo()
	keys := DefaultKeyMap()

	// Pre-assign the initial fetchID so Init can create a matching fetch command.
	initialID := fetchSeq.Add(1)

	pl := podlist.New()
	if info.Namespace == k8s.AllNamespaces {
		pl = pl.SetShowNamespace(true)
	}

	return Model{
		client:     client,
		keys:       keys,
		state:      stateBrowsing,
		namespace:  info.Namespace,
		fetchID:    initialID,
		header:     header.New(info.ContextName, info.Namespace),
		podList:    pl,
		footer:     footer.New(keys.ShortHelp()),
		nsSwitcher: namespace.New(),
	}
}

// Init starts the initial commands.
// Uses the pre-assigned fetchID from NewModel to avoid the value-receiver copy problem.
func (m Model) Init() tea.Cmd {
	id := m.fetchID
	ns := m.namespace
	client := m.client
	fetchCmd := func() tea.Msg {
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
	return tea.Batch(fetchCmd, m.tickCmd(), loadingTickCmd())
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg), nil

	case PodsLoadedMsg:
		return m.handlePodsLoaded(msg), nil

	case PodsDeletedMsg:
		return m.handlePodsDeleted(msg)

	case NamespacesLoadedMsg:
		return m.handleNamespacesLoaded(msg), nil

	case LoadingTickMsg:
		podLoading := m.podList.IsLoading()
		if podLoading || m.nsLoading {
			newModel := m
			if podLoading {
				newModel.podList = m.podList.TickLoading()
			}
			if m.nsLoading {
				newModel.nsSpinnerFrame = (m.nsSpinnerFrame + 1) % len(nsSpinnerFrames)
			}
			return newModel, loadingTickCmd()
		}
		return m, nil

	case TickMsg:
		if m.state == stateBrowsing {
			return m, tea.Batch(m.fetchPodsCmd(), m.tickCmd())
		}
		return m, m.tickCmd()

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	// Pass non-key messages to active sub-component
	if m.state == stateSwitchingNamespace {
		return m.updateNSSwitcher(msg)
	}

	return m, nil
}

// View renders the full UI.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.state {
	case stateConfirming:
		return m.header.View() + "\n" +
			m.podList.View() + "\n\n" +
			m.confirm.View() + "\n"

	case stateSwitchingNamespace:
		return m.header.View() + "\n" +
			m.nsSwitcher.View() + "\n"

	default:
		status := ""
		if m.nsLoading {
			spinner := styles.LoadingSpinner.Render(nsSpinnerFrames[m.nsSpinnerFrame])
			label := styles.LoadingPrefix.Render(" Loading namespaces...")
			status = "\n  " + spinner + label
		} else if m.statusMsg != "" {
			status = "\n" + m.statusMsg
		}
		if m.err != nil {
			status = "\n Error: " + m.err.Error()
		}
		return m.header.View() + "\n" +
			m.podList.View() + status + "\n" +
			m.footer.View() + "\n"
	}
}

// --- Message handlers ---

func (m Model) handleResize(msg tea.WindowSizeMsg) Model {
	listHeight := msg.Height - common.HeaderHeight - common.FooterHeight - 2
	if listHeight < 3 {
		listHeight = 3
	}
	newModel := m
	newModel.header = m.header.Update(msg)
	newModel.podList = m.podList.SetSize(msg.Width, listHeight)
	newModel.footer = m.footer.SetWidth(msg.Width)
	newModel.width = msg.Width
	newModel.height = msg.Height
	return newModel
}

func (m Model) handlePodsLoaded(msg PodsLoadedMsg) Model {
	// Discard stale responses from a previous fetch
	if msg.FetchID != m.fetchID {
		return m
	}
	if msg.Err != nil && len(msg.Pods) == 0 {
		newModel := m
		newModel.err = msg.Err
		return newModel
	}
	// Always cache the full unfiltered list for client-side filter toggling
	allPods := msg.Pods
	totalCount := len(allPods)
	displayPods := allPods
	if m.filter.ShowDirtyOnly {
		displayPods = k8s.FilterDirtyPods(allPods)
	}
	newModel := m
	newModel.allPods = allPods
	newModel.podList = m.podList.SetItems(displayPods)
	newModel.totalPodCount = totalCount
	newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), totalCount))
	newModel.err = nil
	if msg.Err != nil {
		newModel.statusMsg = "Warning: some namespaces failed to load"
	} else {
		newModel.statusMsg = ""
	}
	return newModel
}

func (m Model) handlePodsDeleted(msg PodsDeletedMsg) (Model, tea.Cmd) {
	successCount := 0
	for _, r := range msg.Results {
		if r.Success {
			successCount++
		}
	}
	newModel := m
	newModel.state = stateBrowsing
	newModel.err = nil
	if successCount > 0 {
		newModel.statusMsg = " Deleted " + strconv.Itoa(successCount) + " pod(s)"
	} else {
		newModel.statusMsg = ""
	}
	return newModel, newModel.fetchPodsCmd()
}

func (m Model) handleNamespacesLoaded(msg NamespacesLoadedMsg) Model {
	if msg.Err != nil {
		newModel := m
		newModel.err = msg.Err
		newModel.state = stateBrowsing
		newModel.nsLoading = false
		return newModel
	}
	newModel := m
	newModel.nsSwitcher = m.nsSwitcher.SetNamespaces(msg.Namespaces).Activate()
	newModel.state = stateSwitchingNamespace
	newModel.statusMsg = ""
	newModel.nsLoading = false
	return newModel
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Global quit
	if key.Matches(msg, m.keys.Quit) && m.state == stateBrowsing {
		return m, tea.Quit
	}

	switch m.state {
	case stateBrowsing:
		return m.handleBrowsingKey(msg)
	case stateConfirming:
		return m.handleConfirmingKey(msg)
	case stateSwitchingNamespace:
		return m.updateNSSwitcher(msg)
	}
	return m, nil
}

func (m Model) handleBrowsingKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		newModel := m
		newModel.podList = m.podList.MoveUp()
		return newModel, nil
	case key.Matches(msg, m.keys.Down):
		newModel := m
		newModel.podList = m.podList.MoveDown()
		return newModel, nil
	case key.Matches(msg, m.keys.Select):
		newModel := m
		newModel.podList = m.podList.ToggleSelect()
		return newModel, nil
	case key.Matches(msg, m.keys.SelectAll):
		newModel := m
		if m.podList.SelectedCount() == m.podList.Len() {
			newModel.podList = m.podList.DeselectAll()
		} else {
			newModel.podList = m.podList.SelectAll()
		}
		return newModel, nil
	case key.Matches(msg, m.keys.Delete):
		selected := m.podList.GetSelected()
		if len(selected) == 0 {
			newModel := m
			newModel.statusMsg = "No pods selected"
			return newModel, nil
		}
		names := make([]string, len(selected))
		for i, p := range selected {
			names[i] = p.Namespace + "/" + p.Name
		}
		newModel := m
		newModel.confirm = confirm.New(names)
		newModel.state = stateConfirming
		return newModel, nil
	case key.Matches(msg, m.keys.Namespace):
		newModel := m
		newModel.nsLoading = true
		newModel.nsSpinnerFrame = 0
		newModel.statusMsg = ""
		return newModel, tea.Batch(newModel.fetchNamespacesCmd(), loadingTickCmd())
	case key.Matches(msg, m.keys.Refresh):
		newModel := m
		newModel.podList = m.podList.SetLoading()
		newModel.statusMsg = "Refreshing..."
		return newModel, tea.Batch(newModel.fetchPodsCmd(), loadingTickCmd())
	case key.Matches(msg, m.keys.Filter):
		newFilter := !m.filter.ShowDirtyOnly
		newModel := m
		newModel.filter = k8s.ResourceFilter{ShowDirtyOnly: newFilter}
		if newFilter {
			newModel.statusMsg = "Filter: showing dirty pods only"
		} else {
			newModel.statusMsg = "Filter: showing all pods"
		}
		// Use cached pods if available — no API call needed
		if m.allPods != nil {
			displayPods := m.allPods
			if newFilter {
				displayPods = k8s.FilterDirtyPods(m.allPods)
			}
			newModel.podList = m.podList.SetItems(displayPods)
			newModel.header = m.header.SetFilter(newFilter, buildPodCountLabel(newFilter, len(displayPods), len(m.allPods)))
			return newModel, nil
		}
		// No cached data — must fetch
		newModel.podList = m.podList.SetLoading()
		newModel.header = m.header.SetFilter(newFilter, "")
		return newModel, tea.Batch(newModel.fetchPodsCmd(), loadingTickCmd())
	}
	return m, nil
}

func (m Model) handleConfirmingKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	newConfirm, cmd := m.confirm.Update(msg)
	newModel := m
	newModel.confirm = newConfirm
	if newConfirm.IsConfirmed() {
		pods := m.podList.GetSelected()
		newModel.state = stateBrowsing
		newModel.statusMsg = "Deleting..."
		return newModel, newModel.deletePodsCmd(pods)
	}
	if newConfirm.IsCancelled() {
		newModel.state = stateBrowsing
		newModel.statusMsg = "Cancelled"
		return newModel, nil
	}
	return newModel, cmd
}

// updateNSSwitcher delegates to the namespace switcher and handles its outcomes.
func (m Model) updateNSSwitcher(msg tea.Msg) (Model, tea.Cmd) {
	newNS, cmd := m.nsSwitcher.Update(msg)
	newModel := m
	newModel.nsSwitcher = newNS
	if ns, ok := newNS.GetSelected(); ok {
		return newModel.switchNamespace(ns)
	}
	if !newNS.IsActive() {
		newModel.state = stateBrowsing
		return newModel, cmd
	}
	return newModel, cmd
}

func (m Model) switchNamespace(ns string) (Model, tea.Cmd) {
	// Map the "All Namespaces" label back to the sentinel value
	if ns == namespace.AllNamespacesLabel {
		ns = k8s.AllNamespaces
	}
	isAllNS := ns == k8s.AllNamespaces
	newModel := Model{
		client:     m.client,
		keys:       m.keys,
		state:      stateBrowsing,
		filter:     m.filter,
		namespace:  ns,
		header:     m.header.SetNamespace(ns).SetFilter(m.filter.ShowDirtyOnly, ""),
		podList:    m.podList.SetShowNamespace(isAllNS).SetItems(nil).SetLoading(),
		footer:     m.footer,
		confirm:    m.confirm,
		nsSwitcher: m.nsSwitcher.Deactivate(),
		width:      m.width,
		height:     m.height,
	}
	return newModel, tea.Batch(newModel.fetchPodsCmd(), loadingTickCmd())
}

// --- Command factories ---

// fetchPodsCmd creates a command that fetches pods. Uses a sequence number
// to allow the receiver to discard stale responses from previous fetches.
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

func (m Model) deletePodsCmd(pods []k8s.PodInfo) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		results := k8s.DeletePods(ctx, client, pods)
		return PodsDeletedMsg{Results: results}
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

func (m Model) tickCmd() tea.Cmd {
	interval := pollInterval
	if m.namespace == k8s.AllNamespaces {
		interval = pollIntervalAllNS
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

const loadingTickInterval = 80 * time.Millisecond

// nsSpinnerFrames are the animation frames for the namespace loading spinner.
var nsSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// loadingTickCmd sends a LoadingTickMsg after a short interval for spinner animation.
func loadingTickCmd() tea.Cmd {
	return tea.Tick(loadingTickInterval, func(time.Time) tea.Msg {
		return LoadingTickMsg{}
	})
}

// buildPodCountLabel returns a label like "3/10 dirty pods" when filter is active.
func buildPodCountLabel(filterActive bool, shown, total int) string {
	if !filterActive {
		return ""
	}
	return fmt.Sprintf("%d/%d dirty pods", shown, total)
}
