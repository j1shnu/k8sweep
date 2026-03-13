package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/poddetail"
)

// containerRunning returns true if the container's state starts with "Running".
func containerRunning(c k8s.ContainerDetail) bool {
	return strings.HasPrefix(c.State, "Running")
}

// hasRunningContainer returns true if at least one container is in Running state.
func hasRunningContainer(containers []k8s.ContainerDetail) bool {
	for _, c := range containers {
		if containerRunning(c) {
			return true
		}
	}
	return false
}

func (m Model) handlePodDetailLoaded(msg PodDetailLoadedMsg) Model {
	if m.state != stateViewingDetail || msg.PodKey != m.detailPodKey {
		return m
	}
	newModel := m
	if msg.Err != nil {
		newModel.detailData = nil
		newModel.detailStatus = ""
		newModel.podDetail = m.podDetail.SetError(msg.Err.Error())
	} else {
		// Enrich detail with resolved controller from allPods cache
		detail := msg.Detail
		for _, p := range m.allPods {
			if p.Namespace == detail.Namespace && p.Name == detail.Name {
				ref := p.Controller.String()
				if ref != "" && ref != detail.Owner {
					detail.ResolvedController = ref
				}
				break
			}
		}
		newModel.detailData = detail
		newModel.detailStatus = ""
		newModel.podDetail = m.podDetail.SetDetail(detail)
	}
	return newModel
}

func (m Model) handlePodEventsLoaded(msg PodEventsLoadedMsg) Model {
	if m.state != stateViewingDetail || msg.PodKey != m.detailPodKey {
		return m
	}
	newModel := m
	if msg.Err != nil {
		newModel.podDetail = m.podDetail.SetEventsError(msg.Err.Error())
	} else {
		newModel.podDetail = m.podDetail.SetEvents(msg.Events)
	}
	return newModel
}

func (m Model) handlePodLogsLoaded(msg PodLogsLoadedMsg) Model {
	if m.state != stateViewingDetail || msg.PodKey != m.detailPodKey {
		return m
	}
	newModel := m
	if msg.Err != nil {
		newModel.podDetail = m.podDetail.SetLogsError(msg.Err.Error())
	} else {
		newModel.podDetail = m.podDetail.SetLogs(msg.Lines, msg.Container)
	}
	return newModel
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Info) || msg.String() == "esc":
		// If in a subview (events/logs), go back to detail instead of closing
		if m.podDetail.Subview() != poddetail.SubviewDetail {
			newModel := m
			newModel.podDetail = m.podDetail.ShowDetail()
			return newModel, nil
		}
		newModel := m
		newModel.state = stateBrowsing
		newModel.podDetail = m.podDetail.Hide()
		newModel.detailData = nil
		newModel.detailStatus = ""
		newModel.shellWarningAcked = false
		return newModel, nil
	case key.Matches(msg, m.keys.Events):
		if m.detailData == nil {
			newModel := m
			newModel.detailStatus = "Pod details are still loading"
			return newModel, nil
		}
		newModel := m
		newModel.podDetail = m.podDetail.SetEventsLoading()
		newModel.detailStatus = ""
		return newModel, newModel.fetchPodEventsCmd(m.detailData.Namespace, m.detailData.Name)
	case key.Matches(msg, m.keys.Logs):
		if m.detailData == nil {
			newModel := m
			newModel.detailStatus = "Pod details are still loading"
			return newModel, nil
		}
		if len(m.detailData.Containers) == 0 {
			newModel := m
			newModel.detailStatus = "No containers available in this pod"
			return newModel, nil
		}
		// Single container: fetch logs directly
		if len(m.detailData.Containers) == 1 {
			newModel := m
			newModel.podDetail = m.podDetail.SetLogsLoading()
			newModel.detailStatus = ""
			return newModel, newModel.fetchPodLogsCmd(
				m.detailData.Namespace,
				m.detailData.Name,
				m.detailData.Containers[0].Name,
			)
		}
		// Multiple containers: show picker
		newModel := m
		newModel.state = statePickingContainer
		newModel.containerPickFor = pickForLogs
		newModel.detailStatus = ""
		newModel.containerSel = m.containerSel.SetContainers(m.detailData.Containers)
		return newModel, nil
	case key.Matches(msg, m.keys.Shell):
		if m.detailData == nil {
			newModel := m
			newModel.detailStatus = "Pod details are still loading"
			return newModel, nil
		}
		if !isShellEligibleStatus(m.detailData.Status) {
			newModel := m
			newModel.detailStatus = "Shell unavailable for pod status: " + string(m.detailData.Status)
			return newModel, nil
		}
		if len(m.detailData.Containers) == 0 {
			newModel := m
			newModel.detailStatus = "No containers available in this pod"
			return newModel, nil
		}

		// Warn on risky pod states — but only if at least one container is running.
		// If no containers are running, skip the warning and let the per-container
		// check below show "Shell unavailable" directly.
		isRisky := m.detailData.Status == k8s.StatusCrashLoopBack || m.detailData.Status == k8s.StatusOOMKilled
		if isRisky && !m.shellWarningAcked && hasRunningContainer(m.detailData.Containers) {
			newModel := m
			newModel.shellWarningAcked = true
			newModel.detailStatus = "Warning: pod is in " + string(m.detailData.Status) + " — shell may not work properly. Press [" + m.keys.Shell.Keys()[0] + "] again to proceed."
			return newModel, nil
		}

		if len(m.detailData.Containers) == 1 {
			container := m.detailData.Containers[0]
			if !containerRunning(container) {
				newModel := m
				newModel.detailStatus = fmt.Sprintf("Shell unavailable: container %s is not running (%s)", container.Name, container.State)
				return newModel, nil
			}
			newModel := m
			newModel.shellWarningAcked = false
			newModel.detailStatus = ""
			return newModel, newModel.openShellCmd(
				m.detailData.Namespace,
				m.detailData.Name,
				container.Name,
			)
		}

		newModel := m
		newModel.state = statePickingContainer
		newModel.containerPickFor = pickForShell
		newModel.shellWarningAcked = false
		newModel.detailStatus = ""
		newModel.containerSel = m.containerSel.SetContainers(m.detailData.Containers)
		return newModel, nil
	case msg.String() == "j" || msg.String() == "down":
		newModel := m
		newModel.podDetail = m.podDetail.ScrollDown()
		newModel.pendingG = false
		return newModel, nil
	case msg.String() == "k" || msg.String() == "up":
		newModel := m
		newModel.podDetail = m.podDetail.ScrollUp()
		newModel.pendingG = false
		return newModel, nil
	case key.Matches(msg, m.keys.GoBottom):
		newModel := m
		newModel.podDetail = m.podDetail.ScrollToBottom()
		newModel.pendingG = false
		return newModel, nil
	case msg.String() == "g":
		if m.pendingG && time.Since(m.pendingGTime) < 500*time.Millisecond {
			newModel := m
			newModel.podDetail = m.podDetail.ScrollToTop()
			newModel.pendingG = false
			return newModel, nil
		}
		newModel := m
		newModel.pendingG = true
		newModel.pendingGTime = time.Now()
		return newModel, nil
	}
	return m, nil
}

func (m Model) handleContainerPickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		newModel := m
		newModel.state = stateViewingDetail
		newModel.detailStatus = ""
		return newModel, nil
	case "j", "down":
		newModel := m
		newModel.containerSel = m.containerSel.MoveDown()
		return newModel, nil
	case "k", "up":
		newModel := m
		newModel.containerSel = m.containerSel.MoveUp()
		return newModel, nil
	case "enter":
		selected := m.containerSel.Selected()
		if selected == nil || m.detailData == nil {
			newModel := m
			newModel.detailStatus = "No container selected"
			return newModel, nil
		}
		// Logs: any container state is valid (even terminated pods have logs)
		if m.containerPickFor == pickForLogs {
			newModel := m
			newModel.state = stateViewingDetail
			newModel.podDetail = m.podDetail.SetLogsLoading()
			newModel.detailStatus = ""
			return newModel, newModel.fetchPodLogsCmd(
				m.detailData.Namespace,
				m.detailData.Name,
				selected.Name,
			)
		}
		// Shell: require running container
		if !containerRunning(*selected) {
			newModel := m
			newModel.detailStatus = fmt.Sprintf("Shell unavailable: container %s is not running (%s)", selected.Name, selected.State)
			return newModel, nil
		}
		newModel := m
		newModel.state = stateViewingDetail
		newModel.detailStatus = ""
		return newModel, newModel.openShellCmd(
			m.detailData.Namespace,
			m.detailData.Name,
			selected.Name,
		)
	}
	return m, nil
}

func (m Model) handlePodShellExited(msg PodShellExitedMsg) Model {
	if msg.PodKey != m.detailPodKey {
		return m
	}

	newModel := m
	newModel.state = stateBrowsing
	newModel.podDetail = m.podDetail.Hide()
	newModel.detailData = nil
	newModel.detailStatus = ""

	if msg.Err != nil {
		newModel.err = msg.Err
		newModel.statusMsg = ""
		return newModel
	}

	newModel.err = nil
	newModel.statusMsg = fmt.Sprintf(
		"Shell closed: %s (%s via %s)",
		msg.Container,
		msg.ShellPath,
		msg.Backend,
	)
	return newModel
}
