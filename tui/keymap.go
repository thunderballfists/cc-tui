package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Open    key.Binding
	Window  key.Binding
	New     key.Binding
	Kill    key.Binding
	Filter  key.Binding
	Search  key.Binding
	Archive key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
	Top     key.Binding
	Bottom  key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/up", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/dn", "down")),
		Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("h", "collapse")),
		Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("l", "expand")),
		Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("ret", "open")),
		Window:  key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "window")),
		New:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		Kill:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "kill")),
		Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Search:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "search")),
		Archive: key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "archive")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:    key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q", "quit")),
		Top:     key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "top")),
		Bottom:  key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
	}
}

// ShortHelp returns keybindings to show in the short help view.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Open, k.New, k.Quit}
}

// FullHelp returns keybindings to show in the full help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Open, k.Window, k.New, k.Kill},
		{k.Filter, k.Search, k.Archive, k.Refresh, k.Help, k.Quit},
		{k.Top, k.Bottom},
	}
}
