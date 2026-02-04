package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap defines all key bindings for the TUI application.
type keyMap struct {
	Quit    key.Binding
	NextTab key.Binding
	PrevTab key.Binding
	Tab1    key.Binding
	Tab2    key.Binding
	Tab3    key.Binding
}

// keys holds the default key bindings used by the application.
var keys = keyMap{
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	NextTab: key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab", "next tab")),
	PrevTab: key.NewBinding(key.WithKeys("shift+tab", "left"), key.WithHelp("shift+tab", "prev tab")),
	Tab1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "claude")),
	Tab2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "billing")),
	Tab3:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "infra")),
}
