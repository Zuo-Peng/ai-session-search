package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Quit       key.Binding
	PreviewUp  key.Binding
	PreviewDn  key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "ctrl+k"),
		key.WithHelp("up/C-k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "ctrl+j"),
		key.WithHelp("dn/C-j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open"),
	),
	Quit: key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc", "quit"),
	),
	PreviewUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("C-u", "preview up"),
	),
	PreviewDn: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("C-d", "preview down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "preview pgup"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdn", "preview pgdn"),
	),
}
