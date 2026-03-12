package app

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/common"
	"github.com/jprasad/k8sweep/internal/tui/confirm"
	"github.com/jprasad/k8sweep/internal/tui/containerpicker"
	"github.com/jprasad/k8sweep/internal/tui/footer"
	"github.com/jprasad/k8sweep/internal/tui/header"
	"github.com/jprasad/k8sweep/internal/tui/help"
	"github.com/jprasad/k8sweep/internal/tui/namespace"
	"github.com/jprasad/k8sweep/internal/tui/poddetail"
	"github.com/jprasad/k8sweep/internal/tui/podlist"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

// appState represents the current UI state.
type appState int

const (
	stateBrowsing appState = iota
	stateConfirming
	stateSwitchingNamespace
	stateHelp
	stateViewingDetail
	statePickingContainer
	stateSearching
)

const (
	fetchTimeout      = 15 * time.Second
	fetchTimeoutAllNS = 120 * time.Second
	metricsInterval   = 30 * time.Second
	searchDebounce    = 120 * time.Millisecond
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

	// watcher provides real-time pod updates via the Kubernetes watch API.
	watcher *k8s.PodWatcher
	watchID uint64 // incremented on each new watcher to discard stale events

	header       header.Model
	podList      podlist.Model
	footer       footer.Model
	confirm      confirm.Model
	nsSwitcher   namespace.Model
	help         help.Model
	podDetail    poddetail.Model
	containerSel containerpicker.Model
	detailPodKey     string // tracks which pod's detail was requested
	detailData       *k8s.PodDetail
	detailStatus     string
	shellWarningAcked bool // true after user acknowledges shell warning for risky pod states

	// Search
	searchInput textinput.Model
	searchQuery string // active name filter (empty = no filter)
	searchSeq   uint64 // increments on each search input to discard stale debounce ticks

	allPods        []k8s.PodInfo // cached full pod list for client-side filtering
	totalPodCount  int
	statusMsg      string
	err            error
	width          int
	height         int
	nsLoading      bool // true while fetching namespaces
	nsSpinnerFrame int  // spinner animation frame for namespace loading

	// Vim gg support: tracks pending 'g' keypress for go-to-top
	pendingG     bool
	pendingGTime time.Time

	metricsAvailable bool                      // true if Metrics API is available
	pendingMetrics   map[string]k8s.PodMetrics // buffered metrics awaiting pod data merge
	lastMetrics      map[string]k8s.PodMetrics // last known metrics, reused until fresh metrics arrive
}

// NewModel creates the initial application model.
func NewModel(client *k8s.Client) Model {
	info := client.GetClusterInfo()
	keys := DefaultKeyMap()

	// Pre-assign the initial fetchID so Init can create a matching fetch command.
	initialID := fetchSeq.Add(1)

	metricsAvail := client.MetricsAvailable()

	pl := podlist.New()
	if info.Namespace == k8s.AllNamespaces {
		pl = pl.SetShowNamespace(true)
	}
	if metricsAvail {
		pl = pl.SetMetricsAvailable(true)
	}

	watcher := k8s.NewPodWatcher(client.Clientset(), info.Namespace)

	return Model{
		client:           client,
		keys:             keys,
		state:            stateBrowsing,
		namespace:        info.Namespace,
		fetchID:          initialID,
		watcher:          watcher,
		watchID:          1,
		header:           header.New(info.ContextName, info.Namespace),
		podList:          pl,
		footer:           footer.New(keys.ShortHelp()),
		nsSwitcher:       namespace.New(),
		help:             help.New(keys.FullHelp()),
		containerSel:     containerpicker.New(),
		metricsAvailable: metricsAvail,
	}
}

// Init starts the watcher, metrics polling, and loading animation.
func (m Model) Init() tea.Cmd {
	if m.watcher != nil {
		return tea.Batch(m.startAndWatchCmd(), m.fetchMetricsCmd(), m.tickCmd(), loadingTickCmd())
	}
	// Fallback for environments without a watcher (e.g., tests)
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
	return tea.Batch(fetchCmd, m.fetchMetricsCmd(), m.tickCmd(), loadingTickCmd())
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg), nil

	case WatchPodsMsg:
		return m.handleWatchPods(msg)

	case WatchStoppedMsg:
		// Old watcher stopped (namespace switch); ignore
		return m, nil

	case PodsLoadedMsg:
		return m.handlePodsLoaded(msg), nil

	case MetricsLoadedMsg:
		return m.handleMetricsLoaded(msg), nil

	case PodDetailLoadedMsg:
		return m.handlePodDetailLoaded(msg), nil

	case PodShellExitedMsg:
		return m.handlePodShellExited(msg), nil

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

	case SearchDebouncedMsg:
		return m.handleSearchDebounced(msg), nil

	case TickMsg:
		// Metrics-only tick (pod updates come from watcher)
		if m.state == stateBrowsing {
			return m, tea.Batch(m.fetchMetricsCmd(), m.tickCmd())
		}
		return m, m.tickCmd()

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	// Pass non-key messages to active sub-component
	switch m.state {
	case stateSwitchingNamespace:
		return m.updateNSSwitcher(msg)
	case stateSearching:
		return m.handleSearchMsg(msg)
	}

	return m, nil
}

// View renders the full UI.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.state {
	case stateHelp:
		return m.header.View() + "\n" +
			m.help.View() + "\n" +
			m.footer.View() + "\n"

	case stateViewingDetail:
		status := ""
		if m.detailStatus != "" {
			status = "\n" + styles.StatusMessage.Render(" "+m.detailStatus)
		}
		return m.header.View() + "\n" +
			m.podDetail.View() + status + "\n"

	case statePickingContainer:
		status := ""
		if m.detailStatus != "" {
			status = "\n" + styles.StatusMessage.Render(" "+m.detailStatus)
		}
		return m.header.View() + "\n" +
			m.podDetail.View() + "\n" +
			m.containerSel.View() + status + "\n"

	case stateConfirming:
		return m.header.View() + "\n" +
			m.podList.View() + "\n\n" +
			m.confirm.View() + "\n"

	case stateSwitchingNamespace:
		return m.header.View() + "\n" +
			m.nsSwitcher.View() + "\n"

	case stateSearching:
		status := ""
		if m.err != nil {
			status = "\n Error: " + m.err.Error()
		}
		return m.header.View() + "\n" +
			m.podList.View() + status + "\n" +
			m.searchInput.View() + "\n"

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
		searchHint := ""
		if m.searchQuery != "" {
			searchHint = "\n " + styles.FilterBadge.Render(" SEARCH ") + " " + m.searchQuery
		}
		return m.header.View() + "\n" +
			m.podList.View() + searchHint + status + "\n" +
			m.footer.View() + "\n"
	}
}

// --- Message handlers ---

func (m Model) handleResize(msg tea.WindowSizeMsg) Model {
	// -1 extra for the column header row rendered inside podlist.View()
	listHeight := msg.Height - common.HeaderHeight - common.FooterHeight - 3
	if listHeight < 3 {
		listHeight = 3
	}
	newModel := m
	newModel.header = m.header.Update(msg)
	newModel.podList = m.podList.SetSize(msg.Width, listHeight)
	newModel.footer = m.footer.SetWidth(msg.Width)
	newModel.help = m.help.SetSize(msg.Width, msg.Height-common.HeaderHeight-common.FooterHeight-1)
	newModel.podDetail = m.podDetail.SetSize(msg.Width, msg.Height-common.HeaderHeight-1)
	newModel.width = msg.Width
	newModel.height = msg.Height
	return newModel
}

func (m Model) handleWatchPods(msg WatchPodsMsg) (Model, tea.Cmd) {
	if msg.WatchID != m.watchID {
		return m, nil // stale event from old watcher
	}

	allPods := msg.Pods
	if m.lastMetrics != nil {
		allPods = k8s.MergePodMetrics(allPods, m.lastMetrics)
	}

	displayPods := applyFilters(allPods, m.filter, m.activeSearchQuery())
	totalCount := len(allPods)

	newModel := m
	newModel.allPods = allPods
	newModel.podList = m.podList.SetItemsSorted(displayPods)
	newModel.totalPodCount = totalCount
	newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), totalCount)).
		SetStatusSummary(buildStatusSummary(allPods))
	newModel.err = nil
	newModel.statusMsg = ""

	return newModel, m.watchPodsCmd()
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
	allPods := msg.Pods

	if m.pendingMetrics != nil {
		allPods = k8s.MergePodMetrics(allPods, m.pendingMetrics)
	} else if m.lastMetrics != nil {
		allPods = k8s.MergePodMetrics(allPods, m.lastMetrics)
	}

	displayPods := applyFilters(allPods, m.filter, m.activeSearchQuery())
	totalCount := len(allPods)

	newModel := m
	newModel.allPods = allPods
	newModel.pendingMetrics = nil
	newModel.podList = m.podList.SetItemsSorted(displayPods)
	newModel.totalPodCount = totalCount
	newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), totalCount)).
		SetStatusSummary(buildStatusSummary(allPods))
	newModel.err = nil
	if msg.Err != nil {
		newModel.statusMsg = "Warning: some namespaces failed to load"
	} else {
		newModel.statusMsg = ""
	}
	return newModel
}

func (m Model) handleMetricsLoaded(msg MetricsLoadedMsg) Model {
	if msg.Namespace != m.namespace {
		return m // stale metrics from a previous namespace
	}
	if msg.Metrics == nil {
		return m
	}

	newModel := m
	newModel.lastMetrics = msg.Metrics

	if m.allPods != nil {
		allPods := k8s.MergePodMetrics(m.allPods, msg.Metrics)
		displayPods := applyFilters(allPods, m.filter, m.activeSearchQuery())
		newModel.allPods = allPods
		newModel.pendingMetrics = nil
		newModel.podList = m.podList.SetItemsSorted(displayPods)
		newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), len(allPods))).
			SetStatusSummary(buildStatusSummary(allPods))
		return newModel
	}

	newModel.pendingMetrics = msg.Metrics
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
	// Watcher will detect the deletions automatically.
	// Fallback fetch for environments without a watcher.
	if m.watcher == nil {
		return newModel, newModel.fetchPodsCmd()
	}
	return newModel, nil
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
	if key.Matches(msg, m.keys.Quit) && (m.state == stateBrowsing || m.state == stateHelp) {
		if m.watcher != nil {
			m.watcher.Stop()
		}
		return m, tea.Quit
	}

	switch m.state {
	case stateBrowsing:
		return m.handleBrowsingKey(msg)
	case stateConfirming:
		return m.handleConfirmingKey(msg)
	case stateSwitchingNamespace:
		return m.updateNSSwitcher(msg)
	case stateHelp:
		return m.handleHelpKey(msg)
	case stateViewingDetail:
		return m.handleDetailKey(msg)
	case statePickingContainer:
		return m.handleContainerPickerKey(msg)
	case stateSearching:
		return m.handleSearchKey(msg)
	}
	return m, nil
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		newModel := m
		newModel.help = m.help.ScrollDown()
		return newModel, nil
	case "k", "up":
		newModel := m
		newModel.help = m.help.ScrollUp()
		return newModel, nil
	}
	if key.Matches(msg, m.keys.Help) || msg.String() == "esc" {
		newModel := m
		newModel.state = stateBrowsing
		return newModel, nil
	}
	return m, nil
}

func (m Model) handleBrowsingKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Help):
		newModel := m
		newModel.state = stateHelp
		newModel.help = m.help.SetSize(m.width, m.height-common.HeaderHeight-common.FooterHeight-1)
		return newModel, nil
	case key.Matches(msg, m.keys.Up):
		newModel := m
		newModel.podList = m.podList.MoveUp()
		newModel.pendingG = false
		return newModel, nil
	case key.Matches(msg, m.keys.Down):
		newModel := m
		newModel.podList = m.podList.MoveDown()
		newModel.pendingG = false
		return newModel, nil
	case key.Matches(msg, m.keys.PageUp):
		newModel := m
		newModel.podList = m.podList.PageUp()
		newModel.pendingG = false
		return newModel, nil
	case key.Matches(msg, m.keys.PageDown):
		newModel := m
		newModel.podList = m.podList.PageDown()
		newModel.pendingG = false
		return newModel, nil
	case key.Matches(msg, m.keys.GoBottom):
		newModel := m
		newModel.podList = m.podList.GoBottom()
		newModel.pendingG = false
		return newModel, nil
	case msg.String() == "g":
		if m.pendingG && time.Since(m.pendingGTime) < 500*time.Millisecond {
			newModel := m
			newModel.podList = m.podList.GoTop()
			newModel.pendingG = false
			return newModel, nil
		}
		newModel := m
		newModel.pendingG = true
		newModel.pendingGTime = time.Now()
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
		return m.enterConfirmDelete(confirm.ActionDelete)
	case key.Matches(msg, m.keys.ForceDelete):
		return m.enterConfirmDelete(confirm.ActionForceDelete)
	case key.Matches(msg, m.keys.Namespace):
		newModel := m
		newModel.nsLoading = true
		newModel.nsSpinnerFrame = 0
		newModel.statusMsg = ""
		return newModel, tea.Batch(newModel.fetchNamespacesCmd(), loadingTickCmd())
	case key.Matches(msg, m.keys.Refresh):
		newModel := m
		if m.watcher != nil {
			// Read from watcher cache for instant refresh
			pods := m.watcher.ListPods()
			allPods := pods
			if m.lastMetrics != nil {
				allPods = k8s.MergePodMetrics(allPods, m.lastMetrics)
			}
			displayPods := applyFilters(allPods, m.filter, m.activeSearchQuery())
			newModel.allPods = allPods
			newModel.totalPodCount = len(allPods)
			newModel.podList = m.podList.SetItemsSorted(displayPods)
			newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), len(allPods))).
				SetStatusSummary(buildStatusSummary(allPods))
			newModel.statusMsg = "Refreshed"
			return newModel, m.fetchMetricsCmd()
		}
		newModel.podList = m.podList.SetLoading()
		newModel.statusMsg = "Refreshing..."
		return newModel, tea.Batch(newModel.fetchPodsCmd(), newModel.fetchMetricsCmd(), loadingTickCmd())
	case key.Matches(msg, m.keys.Info):
		pod := m.podList.CursorItem()
		if pod == nil {
			return m, nil
		}
		newModel := m
		newModel.state = stateViewingDetail
		newModel.detailPodKey = pod.Namespace + "/" + pod.Name
		newModel.detailData = nil
		newModel.detailStatus = ""
		newModel.podDetail = m.podDetail.SetSize(m.width, m.height-common.HeaderHeight-1).SetLoading()
		return newModel, newModel.fetchPodDetailCmd(pod.Namespace, pod.Name)
	case key.Matches(msg, m.keys.Sort):
		col := m.podList.SortColumn()
		order := m.podList.SortOrder()
		if order == podlist.SortAsc {
			// First press on this column was asc → flip to desc
			order = podlist.SortDesc
		} else {
			// Already desc → cycle to next column, reset to asc
			col = podlist.NextSortColumn(col, m.podList.MetricsAvailable())
			order = podlist.SortAsc
		}
		newModel := m
		newModel.podList = m.podList.SetSort(col, order)
		newModel.statusMsg = "Sort: " + podlist.SortColumnLabel(col) + " " + podlist.SortIndicator(order)
		return newModel, nil
	case key.Matches(msg, m.keys.Search):
		return m.enterSearchMode()
	case key.Matches(msg, m.keys.Filter):
		newFilter := !m.filter.ShowDirtyOnly
		newModel := m
		newModel.filter = k8s.ResourceFilter{ShowDirtyOnly: newFilter}
		if newFilter {
			newModel.statusMsg = "Filter: showing dirty pods only"
		} else {
			newModel.statusMsg = "Filter: showing all pods"
		}
		if m.allPods != nil {
			displayPods := applyFilters(m.allPods, newModel.filter, m.activeSearchQuery())
			newModel.podList = m.podList.SetItemsSorted(displayPods)
			// Turning filter OFF should reset pagination/cursor to page 1.
			if !newFilter {
				newModel.podList = newModel.podList.GoTop()
			}
			newModel.header = m.header.SetFilter(newFilter, buildPodCountLabel(newFilter, len(displayPods), len(m.allPods))).
				SetStatusSummary(buildStatusSummary(m.allPods))
			return newModel, nil
		}
		newModel.podList = m.podList.SetLoading()
		newModel.header = m.header.SetFilter(newFilter, "").
			SetStatusSummary(header.StatusSummary{})
		return newModel, tea.Batch(newModel.fetchPodsCmd(), newModel.fetchMetricsCmd(), loadingTickCmd())
	}
	return m, nil
}

// enterConfirmDelete enters the confirm dialog for delete or force delete.
func (m Model) enterConfirmDelete(action confirm.ActionType) (Model, tea.Cmd) {
	selected := m.podList.GetSelected()
	if len(selected) == 0 {
		newModel := m
		newModel.statusMsg = "No pods selected"
		return newModel, nil
	}

	names := make([]string, len(selected))
	var warnings []string
	for i, p := range selected {
		names[i] = p.Namespace + "/" + p.Name
		if p.OwnerRef == "" {
			warnings = append(warnings, p.Name+" is standalone (no controller — delete is permanent)")
		}
	}

	newModel := m
	newModel.confirm = confirm.NewWithAction(names, action, warnings)
	newModel.state = stateConfirming
	return newModel, nil
}

func (m Model) handleConfirmingKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	newConfirm, cmd := m.confirm.Update(msg)
	newModel := m
	newModel.confirm = newConfirm
	if newConfirm.IsConfirmed() {
		pods := m.podList.GetSelected()
		newModel.state = stateBrowsing
		if newConfirm.Action() == confirm.ActionForceDelete {
			newModel.statusMsg = "Force deleting..."
			return newModel, newModel.forceDeletePodsCmd(pods)
		}
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

// --- Namespace switcher ---

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
	if ns == namespace.AllNamespacesLabel {
		ns = k8s.AllNamespaces
	}
	isAllNS := ns == k8s.AllNamespaces

	// Stop old watcher
	if m.watcher != nil {
		m.watcher.Stop()
	}

	// Create new watcher for the new namespace
	newWatcher := k8s.NewPodWatcher(m.client.Clientset(), ns)
	newWatchID := m.watchID + 1

	newModel := Model{
		client:           m.client,
		keys:             m.keys,
		state:            stateBrowsing,
		filter:           m.filter,
		namespace:        ns,
		watcher:          newWatcher,
		watchID:          newWatchID,
		header:           m.header.SetNamespace(ns).SetFilter(m.filter.ShowDirtyOnly, "").SetStatusSummary(header.StatusSummary{}),
		podList:          m.podList.SetShowNamespace(isAllNS).SetItems(nil).SetLoading(),
		footer:           m.footer,
		confirm:          m.confirm,
		nsSwitcher:       m.nsSwitcher.Deactivate(),
		help:             m.help,
		podDetail:        m.podDetail.Hide(),
		containerSel:     containerpicker.New(),
		detailData:       nil,
		detailStatus:     "",
		width:            m.width,
		height:           m.height,
		metricsAvailable: m.metricsAvailable,
	}
	return newModel, tea.Batch(newModel.startAndWatchCmd(), newModel.fetchMetricsCmd(), loadingTickCmd())
}


