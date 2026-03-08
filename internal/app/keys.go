package app

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings for the application.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Select    key.Binding
	Delete    key.Binding
	Namespace key.Binding
	Refresh   key.Binding
	Filter    key.Binding
	Quit      key.Binding
	Confirm   key.Binding
	Cancel    key.Binding
	SelectAll key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "select"),
		),
		Delete: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "delete"),
		),
		Namespace: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "namespace"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "yes"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		SelectAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "select all"),
		),
	}
}

// ShortHelp returns the short help keybindings for the browsing state.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Delete, k.Namespace, k.Refresh, k.Filter, k.Quit}
}

// FullHelp returns grouped keybindings for the full help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Select, k.SelectAll, k.Delete},
		{k.Namespace, k.Refresh, k.Filter},
		{k.Quit},
	}
}
