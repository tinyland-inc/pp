package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// tuiHandleKey processes all keyboard input for the TUI model.
// It handles global keys (quit, help, search, navigation) and delegates
// arrow keys to the focused widget's HandleKey method.
func tuiHandleKey(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C always quits, regardless of mode.
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	// When in search mode, most keys are captured as search input.
	if m.searchMode {
		return tuiHandleSearchKey(m, msg)
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "/":
		m.searchMode = true
		m.searchQuery = ""
		return m, nil

	case "tab":
		m = tuiCycleFocus(m, 1)
		return m, nil

	case "shift+tab":
		m = tuiCycleFocus(m, -1)
		return m, nil

	case "enter":
		if m.expanded >= 0 {
			// Collapse if already expanded.
			m.expanded = -1
		} else if len(m.widgets) > 0 {
			m.expanded = m.focused
		}
		return m, nil

	case "esc":
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		if m.expanded >= 0 {
			m.expanded = -1
			return m, nil
		}
		return m, nil

	// Vim-style navigation between widgets (only when not expanded).
	case "h", "l":
		if m.expanded < 0 {
			if msg.String() == "l" {
				m = tuiCycleFocus(m, 1)
			} else {
				m = tuiCycleFocus(m, -1)
			}
		}
		return m, nil

	case "j":
		if m.expanded < 0 {
			m = tuiCycleFocus(m, 1)
		}
		return m, nil

	case "k":
		if m.expanded < 0 {
			m = tuiCycleFocus(m, -1)
		}
		return m, nil
	}

	// Forward remaining keys (arrows, unhandled runes) to the focused widget.
	if m.focused >= 0 && m.focused < len(m.widgets) {
		cmd := m.widgets[m.focused].HandleKey(msg)
		return m, cmd
	}

	return m, nil
}

// tuiHandleSearchKey processes key events while in search mode.
// Escape exits search mode, Enter confirms search, Backspace deletes
// the last character, and all other runes are appended to the query.
func tuiHandleSearchKey(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.searchMode = false
		m.searchQuery = ""
		return m, nil

	case tea.KeyEnter:
		// Confirm search: exit search mode but keep the filter active.
		m.searchMode = false
		return m, nil

	case tea.KeyBackspace:
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
		}
		return m, nil

	case tea.KeyRunes:
		m.searchQuery += string(msg.Runes)
		return m, nil
	}

	return m, nil
}

// tuiCycleFocus moves the focus by delta positions, wrapping around the
// widget list. delta=1 moves forward, delta=-1 moves backward.
func tuiCycleFocus(m Model, delta int) Model {
	n := len(m.widgets)
	if n == 0 {
		return m
	}
	m.focused = ((m.focused + delta) % n + n) % n
	return m
}
