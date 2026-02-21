package cockpit

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keyboard bindings for the dashboard.
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Enter        key.Binding
	New          key.Binding
	Kill         key.Binding
	ToggleClaude key.Binding
	ToggleExpand key.Binding
	Help         key.Binding
	Quit         key.Binding
	Confirm      key.Binding
	Cancel       key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("k/up", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("j/down", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "attach"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new session"),
		),
		Kill: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "kill"),
		),
		ToggleClaude: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "claude only"),
		),
		ToggleExpand: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "expand/collapse"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("n", "esc"),
			key.WithHelp("n/esc", "cancel"),
		),
	}
}

// ShortHelp returns the short help bindings (shown in footer).
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.New, k.Kill, k.ToggleClaude, k.Help, k.Quit}
}

// FullHelp returns the full help bindings.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter},
		{k.New, k.Kill, k.ToggleClaude},
		{k.ToggleExpand, k.Help, k.Quit},
	}
}
