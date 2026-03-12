package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
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
		newModel.detailData = msg.Detail
		newModel.detailStatus = ""
		newModel.podDetail = m.podDetail.SetDetail(msg.Detail)
	}
	return newModel
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Info) || msg.String() == "esc":
		newModel := m
		newModel.state = stateBrowsing
		newModel.podDetail = m.podDetail.Hide()
		newModel.detailData = nil
		newModel.detailStatus = ""
		newModel.shellWarningAcked = false
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
			newModel.detailStatus = "Warning: pod is in " + string(m.detailData.Status) + " — shell may not work properly. Press [e] again to proceed."
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
		newModel.shellWarningAcked = false
		newModel.detailStatus = ""
		newModel.containerSel = m.containerSel.SetContainers(m.detailData.Containers)
		return newModel, nil
	case msg.String() == "j" || msg.String() == "down":
		newModel := m
		newModel.podDetail = m.podDetail.ScrollDown()
		return newModel, nil
	case msg.String() == "k" || msg.String() == "up":
		newModel := m
		newModel.podDetail = m.podDetail.ScrollUp()
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
