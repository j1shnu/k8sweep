package app

import (
	"context"
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
)

// appState represents the current UI state.
type appState int

const (
	stateBrowsing appState = iota
	stateConfirming
	stateSwitchingNamespace
)

const pollInterval = 10 * time.Second

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

	statusMsg string
	err       error
	width     int
	height    int
}

// NewModel creates the initial application model.
func NewModel(client *k8s.Client) Model {
	info := client.GetClusterInfo()
	keys := DefaultKeyMap()

	// Pre-assign the initial fetchID so Init can create a matching fetch command.
	initialID := fetchSeq.Add(1)

	return Model{
		client:    client,
		keys:      keys,
		state:     stateBrowsing,
		namespace: info.Namespace,
		fetchID:   initialID,
		header:    header.New(info.ContextName, info.Namespace),
		podList:   podlist.New(),
		footer:    footer.New(keys.ShortHelp()),
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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		pods, err := k8s.ListPods(ctx, client, ns)
		return PodsLoadedMsg{Pods: pods, Err: err, FetchID: id}
	}
	return tea.Batch(fetchCmd, m.tickCmd())
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
		if m.statusMsg != "" {
			status = "\n" + m.statusMsg
		}
		if m.err != nil {
			status = "\n Error: " + m.err.Error()
		}
		filterLabel := ""
		if m.filter.ShowDirtyOnly {
			filterLabel = "  [filter: dirty only]"
		}
		return m.header.View() + filterLabel + "\n" +
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
	if msg.Err != nil {
		newModel := m
		newModel.err = msg.Err
		return newModel
	}
	pods := msg.Pods
	if m.filter.ShowDirtyOnly {
		pods = k8s.FilterDirtyPods(pods)
	}
	newModel := m
	newModel.podList = m.podList.SetItems(pods)
	newModel.err = nil
	newModel.statusMsg = ""
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
		return newModel
	}
	newModel := m
	newModel.nsSwitcher = m.nsSwitcher.SetNamespaces(msg.Namespaces).Activate()
	newModel.state = stateSwitchingNamespace
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
		return m, m.fetchNamespacesCmd()
	case key.Matches(msg, m.keys.Refresh):
		newModel := m
		newModel.statusMsg = "Refreshing..."
		return newModel, newModel.fetchPodsCmd()
	case key.Matches(msg, m.keys.Filter):
		newModel := m
		newModel.filter = k8s.ResourceFilter{ShowDirtyOnly: !m.filter.ShowDirtyOnly}
		return newModel, newModel.fetchPodsCmd()
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
	newModel := Model{
		client:     m.client,
		keys:       m.keys,
		state:      stateBrowsing,
		filter:     m.filter,
		namespace:  ns,
		header:     m.header.SetNamespace(ns),
		podList:    m.podList.SetItems(nil),
		footer:     m.footer,
		confirm:    m.confirm,
		nsSwitcher: m.nsSwitcher.Deactivate(),
		width:      m.width,
		height:     m.height,
	}
	return newModel, newModel.fetchPodsCmd()
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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
	return tea.Tick(pollInterval, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}
