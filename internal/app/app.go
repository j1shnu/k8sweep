package app

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/config"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/common"
	"github.com/jprasad/k8sweep/internal/tui/containerpicker"
	"github.com/jprasad/k8sweep/internal/tui/deletepreview"
	"github.com/jprasad/k8sweep/internal/tui/deletesummary"
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
	stateDeletePreview
	stateDeleteSummary
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

// pickPurpose tracks why the container picker is being shown.
type pickPurpose int

const (
	pickForShell pickPurpose = iota
	pickForLogs
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

	header        header.Model
	podList       podlist.Model
	footer        footer.Model
	deletePreview deletepreview.Model
	deleteSummary deletesummary.Model
	nsSwitcher    namespace.Model
	help          help.Model
	podDetail     poddetail.Model
	containerSel  containerpicker.Model
	detailPodKey      string // tracks which pod's detail was requested
	detailData        *k8s.PodDetail
	detailStatus      string
	shellWarningAcked bool       // true after user acknowledges shell warning for risky pod states
	containerPickFor  pickPurpose // what the container picker is being used for

	controllerDrillDown string // group key for drill-down filter (empty = full view)

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

	// qq support: tracks pending 'q' keypress for quit-from-anywhere
	pendingQ     bool
	pendingQTime time.Time

	metricsAvailable bool                      // true if Metrics API is available
	pendingMetrics   map[string]k8s.PodMetrics // buffered metrics awaiting pod data merge
	lastMetrics      map[string]k8s.PodMetrics // last known metrics, reused until fresh metrics arrive

	prefsPath              string // path to preferences file (empty = no persistence)
	initialCollapseApplied bool   // true after first pod load applies saved collapse state
	savedAllCollapsed      bool   // saved preference: start with groups collapsed
}

// NewModel creates the initial application model.
func NewModel(client *k8s.Client, opts ...ModelOption) Model {
	cfg := modelConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	info := client.GetClusterInfo()
	keys := DefaultKeyMap()

	// Pre-assign the initial fetchID so Init can create a matching fetch command.
	initialID := fetchSeq.Add(1)

	pl := podlist.New()
	if info.Namespace == k8s.AllNamespaces {
		pl = pl.SetShowNamespace(true)
	}

	// Apply saved sort preference
	if cfg.prefs.SortColumn != "" {
		col := podlist.ParseSortColumn(cfg.prefs.SortColumn)
		order := podlist.ParseSortOrder(cfg.prefs.SortOrder)
		pl = pl.SetSort(col, order)
	}

	watcher := k8s.NewPodWatcher(client.Clientset(), info.Namespace)

	filter := k8s.ResourceFilter{ShowDirtyOnly: cfg.prefs.DirtyFilter}

	hdr := header.New(info.ContextName, info.Namespace)
	if cfg.prefs.DirtyFilter {
		hdr = hdr.SetFilter(true, "")
	}

	return Model{
		client:            client,
		keys:              keys,
		state:             stateBrowsing,
		filter:            filter,
		namespace:         info.Namespace,
		fetchID:           initialID,
		watcher:           watcher,
		watchID:           1,
		header:            hdr,
		podList:           pl,
		footer:            footer.New(keys.ShortHelp()),
		nsSwitcher:        namespace.New(),
		help:              help.New(keys.FullHelp()),
		containerSel:      containerpicker.New(),
		searchQuery:         cfg.prefs.SearchQuery,
		controllerDrillDown: cfg.prefs.ControllerDrillDown,
		prefsPath:           cfg.prefsPath,
		savedAllCollapsed:   cfg.prefs.AllCollapsed,
	}
}

// modelConfig holds optional configuration for NewModel.
type modelConfig struct {
	prefs     config.Preferences
	prefsPath string
}

// ModelOption configures NewModel.
type ModelOption func(*modelConfig)

// WithPreferences sets the initial preferences to restore.
func WithPreferences(prefs config.Preferences, path string) ModelOption {
	return func(c *modelConfig) {
		c.prefs = prefs
		c.prefsPath = path
	}
}

// Init starts the watcher, metrics polling, and loading animation.
func (m Model) Init() tea.Cmd {
	if m.watcher != nil {
		return tea.Batch(m.startAndWatchCmd(), m.probeMetricsCmd(), m.tickCmd(), loadingTickCmd())
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
	return tea.Batch(fetchCmd, m.probeMetricsCmd(), m.tickCmd(), loadingTickCmd())
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg), nil

	case WatchPodsMsg:
		return m.handleWatchPods(msg)

	case WatchStoppedMsg:
		if msg.Err != nil {
			newModel := m
			newModel.err = msg.Err
			newModel.podList = m.podList.SetItems(nil) // clears loading state
			return newModel, nil
		}
		// Old watcher stopped (namespace switch); ignore
		return m, nil

	case PodsLoadedMsg:
		return m.handlePodsLoaded(msg)

	case OwnerResolvedMsg:
		return m.handleOwnerResolved(msg), nil

	case MetricsProbedMsg:
		return m.handleMetricsProbed(msg)

	case MetricsLoadedMsg:
		return m.handleMetricsLoaded(msg), nil

	case PodDetailLoadedMsg:
		return m.handlePodDetailLoaded(msg), nil

	case PodEventsLoadedMsg:
		return m.handlePodEventsLoaded(msg), nil

	case PodLogsLoadedMsg:
		return m.handlePodLogsLoaded(msg), nil

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

	case PrefsSavedMsg:
		return m, nil // silently ignore save results

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

	case stateDeletePreview:
		return m.header.View() + "\n" +
			m.deletePreview.View() + "\n"

	case stateDeleteSummary:
		return m.header.View() + "\n" +
			m.deleteSummary.View() + "\n"

	case stateSwitchingNamespace:
		return m.header.View() + "\n" +
			m.nsSwitcher.View() + "\n"

	case stateSearching:
		status := ""
		if m.err != nil {
			status = "\n " + styles.ErrorMessage.Render("Error: "+m.err.Error())
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
			status = "\n " + styles.ErrorMessage.Render("Error: "+m.err.Error())
		}
		filterHints := ""
		if m.controllerDrillDown != "" {
			filterHints += "\n " + styles.FilterBadge.Render(" CONTROLLER ") + " " + m.controllerDrillDown + "  " + styles.FooterHelp.Render("[esc] exit")
		}
		if m.filter.ControllerKindFilter != "" {
			filterHints += "\n " + styles.FilterBadge.Render(" CTRL ") + " " + string(m.filter.ControllerKindFilter)
		}
		if m.searchQuery != "" {
			filterHints += "\n " + styles.FilterBadge.Render(" SEARCH ") + " " + m.searchQuery
		}
		return m.header.View() + "\n" +
			m.podList.View() + filterHints + status + "\n" +
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

	displayPods := applyFilters(allPods, m.filter, m.activeSearchQuery(), m.controllerDrillDown)
	totalCount := len(allPods)

	newModel := m
	newModel.allPods = allPods
	newModel.podList = m.podList.SetItemsSorted(displayPods)
	newModel.totalPodCount = totalCount
	newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), totalCount)).
		SetStatusSummary(buildStatusSummary(allPods))
	newModel.err = nil
	newModel.statusMsg = ""

	// Apply saved collapse preference on first data load
	if !newModel.initialCollapseApplied && newModel.savedAllCollapsed {
		newModel.podList = newModel.podList.CollapseAll()
		newModel.initialCollapseApplied = true
	} else if !newModel.initialCollapseApplied {
		newModel.initialCollapseApplied = true
	}

	// Use fetchID for owner resolution tracking (watchID is for watch events only)
	resolveID := fetchSeq.Add(1)
	newModel.fetchID = resolveID

	return newModel, tea.Batch(m.watchPodsCmd(), newModel.resolveOwnersCmd(allPods, resolveID))
}

func (m Model) handlePodsLoaded(msg PodsLoadedMsg) (Model, tea.Cmd) {
	// Discard stale responses from a previous fetch
	if msg.FetchID != m.fetchID {
		return m, nil
	}
	if msg.Err != nil && len(msg.Pods) == 0 {
		newModel := m
		newModel.err = msg.Err
		return newModel, nil
	}
	allPods := msg.Pods

	if m.pendingMetrics != nil {
		allPods = k8s.MergePodMetrics(allPods, m.pendingMetrics)
	} else if m.lastMetrics != nil {
		allPods = k8s.MergePodMetrics(allPods, m.lastMetrics)
	}

	displayPods := applyFilters(allPods, m.filter, m.activeSearchQuery(), m.controllerDrillDown)
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

	// Apply saved collapse preference on first data load
	if !newModel.initialCollapseApplied && newModel.savedAllCollapsed {
		newModel.podList = newModel.podList.CollapseAll()
		newModel.initialCollapseApplied = true
	} else if !newModel.initialCollapseApplied {
		newModel.initialCollapseApplied = true
	}

	return newModel, newModel.resolveOwnersCmd(allPods, msg.FetchID)
}

func (m Model) handleOwnerResolved(msg OwnerResolvedMsg) Model {
	// Discard stale resolution results
	if msg.FetchID != m.fetchID {
		return m
	}

	allPods := msg.Pods
	if m.lastMetrics != nil {
		allPods = k8s.MergePodMetrics(allPods, m.lastMetrics)
	}

	displayPods := applyFilters(allPods, m.filter, m.activeSearchQuery(), m.controllerDrillDown)

	newModel := m
	newModel.allPods = allPods
	newModel.podList = m.podList.SetItemsSorted(displayPods)
	newModel.totalPodCount = len(allPods)
	newModel.header = m.header.SetFilter(m.filter.ShowDirtyOnly, buildPodCountLabel(m.filter.ShowDirtyOnly, len(displayPods), len(allPods))).
		SetStatusSummary(buildStatusSummary(allPods))

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
		displayPods := applyFilters(allPods, m.filter, m.activeSearchQuery(), m.controllerDrillDown)
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
	action := common.DeleteNormal
	if msg.ForceDelete {
		action = common.DeleteForce
	}

	newModel := m
	newModel.state = stateDeleteSummary
	newModel.deleteSummary = deletesummary.New(msg.Results, action).
		SetSize(m.width, m.height-common.HeaderHeight-1)
	newModel.err = nil
	newModel.statusMsg = ""

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
	// ctrl+c always quits
	if msg.String() == "ctrl+c" {
		if m.watcher != nil {
			m.watcher.Stop()
		}
		return m, tea.Quit
	}

	// Single q quits from browsing/help (existing behavior)
	if msg.String() == "q" && (m.state == stateBrowsing || m.state == stateHelp) {
		if m.watcher != nil {
			m.watcher.Stop()
		}
		return m, tea.Quit
	}

	// qq quits from screens that don't accept text input.
	// Excluded: stateSearching and stateSwitchingNamespace (q is a valid character).
	hasTextInput := m.state == stateSearching || m.state == stateSwitchingNamespace
	if msg.String() == "q" && !hasTextInput {
		if m.pendingQ && time.Since(m.pendingQTime) < 500*time.Millisecond {
			if m.watcher != nil {
				m.watcher.Stop()
			}
			return m, tea.Quit
		}
		newModel := m
		newModel.pendingQ = true
		newModel.pendingQTime = time.Now()
		return newModel, nil
	}
	// Any non-q key resets pending q
	if m.pendingQ {
		newModel := m
		newModel.pendingQ = false
		m = newModel
	}

	switch m.state {
	case stateBrowsing:
		return m.handleBrowsingKey(msg)
	case stateDeletePreview:
		return m.handleDeletePreviewKey(msg)
	case stateDeleteSummary:
		return m.handleDeleteSummaryKey(msg)
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

func (m Model) handleDeletePreviewKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	newPreview, cmd := m.deletePreview.Update(msg)
	newModel := m
	newModel.deletePreview = newPreview
	if newPreview.IsConfirmed() {
		pods := newPreview.Pods()
		newModel.state = stateBrowsing
		if newPreview.Action() == common.DeleteForce {
			newModel.statusMsg = "Force deleting..."
			return newModel, newModel.forceDeletePodsCmd(pods)
		}
		newModel.statusMsg = "Deleting..."
		return newModel, newModel.deletePodsCmd(pods)
	}
	if newPreview.IsCancelled() {
		newModel.state = stateBrowsing
		newModel.statusMsg = "Cancelled"
		return newModel, nil
	}
	return newModel, cmd
}

func (m Model) handleDeleteSummaryKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	newSummary, cmd := m.deleteSummary.Update(msg)
	newModel := m
	newModel.deleteSummary = newSummary
	if newSummary.IsDismissed() {
		newModel.state = stateBrowsing
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

	// Reset controller filter and drill-down on namespace switch.
	// controllerDrillDown is intentionally omitted (zero-valued) to clear it,
	// since controller group keys are namespace-scoped and invalid after switch.
	newFilter := k8s.ResourceFilter{ShowDirtyOnly: m.filter.ShowDirtyOnly}
	newModel := Model{
		client:           m.client,
		keys:             m.keys,
		state:            stateBrowsing,
		filter:           newFilter,
		namespace:        ns,
		watcher:          newWatcher,
		watchID:          newWatchID,
		header:           m.header.SetNamespace(ns).SetFilter(m.filter.ShowDirtyOnly, "").SetStatusSummary(header.StatusSummary{}),
		podList:          m.podList.SetShowNamespace(isAllNS).SetItems(nil).SetLoading(),
		footer:           m.footer,
		nsSwitcher:       m.nsSwitcher.Deactivate(),
		help:             m.help,
		podDetail:        m.podDetail.Hide(),
		containerSel:      containerpicker.New(),
		detailData:        nil,
		detailStatus:      "",
		shellWarningAcked: false,
		containerPickFor:  pickForShell,
		width:             m.width,
		height:           m.height,
		searchQuery:            m.searchQuery,
		metricsAvailable:       m.metricsAvailable,
		prefsPath:              m.prefsPath,
		initialCollapseApplied: true, // don't re-apply collapse on namespace switch
	}
	return newModel, tea.Batch(newModel.startAndWatchCmd(), newModel.fetchMetricsCmd(), loadingTickCmd(), newModel.savePrefsCmd())
}
