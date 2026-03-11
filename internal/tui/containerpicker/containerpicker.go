package containerpicker

import (
	"fmt"
	"strings"

	"github.com/jprasad/k8sweep/internal/k8s"
	"github.com/jprasad/k8sweep/internal/tui/styles"
)

const maxVisibleItems = 8

// Model is the container picker overlay shown before opening a shell.
type Model struct {
	containers []k8s.ContainerDetail
	cursor     int
}

// New creates an empty container picker model.
func New() Model {
	return Model{}
}

// SetContainers loads containers and resets cursor to first item.
func (m Model) SetContainers(containers []k8s.ContainerDetail) Model {
	return Model{
		containers: containers,
		cursor:     0,
	}
}

// MoveUp moves the selection cursor up by one item.
func (m Model) MoveUp() Model {
	if m.cursor <= 0 {
		return m
	}
	m.cursor--
	return m
}

// MoveDown moves the selection cursor down by one item.
func (m Model) MoveDown() Model {
	if m.cursor >= len(m.containers)-1 {
		return m
	}
	m.cursor++
	return m
}

// Selected returns the currently selected container.
func (m Model) Selected() *k8s.ContainerDetail {
	if len(m.containers) == 0 || m.cursor < 0 || m.cursor >= len(m.containers) {
		return nil
	}
	c := m.containers[m.cursor]
	return &c
}

// View renders the overlay.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Select Container Shell Target"))
	b.WriteString("\n\n")

	if len(m.containers) == 0 {
		b.WriteString(styles.FooterHelp.Render("  No containers available"))
		return styles.OverlayBox.Render(b.String())
	}

	start := 0
	if m.cursor >= maxVisibleItems {
		start = m.cursor - maxVisibleItems + 1
	}
	end := start + maxVisibleItems
	if end > len(m.containers) {
		end = len(m.containers)
	}

	for i := start; i < end; i++ {
		c := m.containers[i]
		prefix := "  "
		line := fmt.Sprintf("%s | image: %s", c.Name, c.Image)
		if i == m.cursor {
			prefix = styles.Pointer.Render("> ")
			line = styles.SelectedRow.Render(line)
		}
		b.WriteString(prefix + line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.FooterHelp.Render("[j/k or ↑/↓ move, enter select, esc cancel]"))
	return styles.OverlayBox.Render(b.String())
}
