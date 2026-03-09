package app

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings for the application.
type KeyMap struct {
	Up          key.Binding
	Down        key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	GoBottom    key.Binding
	Select      key.Binding
	Delete      key.Binding
	ForceDelete key.Binding
	Namespace   key.Binding
	Refresh     key.Binding
	Filter      key.Binding
	Search      key.Binding
	Sort        key.Binding
	Info        key.Binding
	Help        key.Binding
	Quit        key.Binding
	Confirm     key.Binding
	Cancel      key.Binding
	SelectAll   key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "move cursor up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "move cursor down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l/→", "page down"),
		),
		GoBottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go to last pod"),
		),
		Select: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle pod selection"),
		),
		Delete: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "delete selected pods"),
		),
		ForceDelete: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "force delete selected pods"),
		),
		Namespace: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "switch namespace"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh pod list"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "toggle dirty-only filter"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search pods by name"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "cycle sort column"),
		),
		Info: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "pod details"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
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
			key.WithHelp("a", "select/deselect all pods"),
		),
	}
}

// ShortHelp returns the short help keybindings for the footer bar.
// Only essential action keys are shown; navigation/selection keys are in FullHelp (? view).
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Delete, k.ForceDelete, k.Search, k.Sort, k.Info, k.Help, k.Quit}
}

// FullHelp returns grouped keybindings for the full help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.GoBottom, k.Search, k.Select, k.SelectAll},
		{k.Delete, k.ForceDelete, k.Refresh, k.Filter},
		{k.Sort, k.Info, k.Namespace, k.Help, k.Quit},
	}
}
