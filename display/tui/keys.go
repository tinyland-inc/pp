package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap defines all key bindings for the TUI application.
// It implements the help.KeyMap interface for bubbles/help integration.
type keyMap struct {
	Quit     key.Binding
	NextTab  key.Binding
	PrevTab  key.Binding
	Tab1     key.Binding
	Tab2     key.Binding
	Tab3     key.Binding
	Tab4     key.Binding
	ScrollUp   key.Binding
	ScrollDown key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	GoTop    key.Binding
	GoBottom key.Binding
	Help     key.Binding
	Refresh  key.Binding
}

// ShortHelp returns the compact set of keybindings shown by default in the footer.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.NextTab, k.ScrollDown, k.Quit}
}

// FullHelp returns the expanded keybinding groups shown when help is toggled.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.NextTab, k.PrevTab, k.Tab1, k.Tab2, k.Tab3, k.Tab4},
		{k.ScrollUp, k.ScrollDown, k.PageUp, k.PageDown, k.GoTop, k.GoBottom},
		{k.Refresh, k.Help, k.Quit},
	}
}

// keys holds the default key bindings used by the application.
var keys = keyMap{
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	NextTab:  key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab", "next tab")),
	PrevTab:  key.NewBinding(key.WithKeys("shift+tab", "left"), key.WithHelp("shift+tab", "prev tab")),
	Tab1:     key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "claude")),
	Tab2:     key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "billing")),
	Tab3:     key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "infra")),
	Tab4:     key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "system")),
	ScrollUp:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/up", "scroll up")),
	ScrollDown: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/dn", "scroll down")),
	PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
	PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")),
	GoTop:    key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	GoBottom: key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Refresh:  key.NewBinding(key.WithKeys("r", "ctrl+r"), key.WithHelp("r", "refresh")),
}
