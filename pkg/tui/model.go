// Package tui implements the fullscreen interactive TUI dashboard using
// Bubbletea's Elm architecture. It manages a widget grid, keyboard
// navigation, widget expansion, search filtering, and a help overlay.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/app"
)

// Model is the root Bubbletea model for the fullscreen TUI dashboard.
type Model struct {
	widgets     []app.Widget // all registered widgets
	focused     int          // index of focused widget
	expanded    int          // index of expanded widget (-1 = none)
	showHelp    bool         // help overlay visible
	searchMode  bool         // search mode active
	searchQuery string       // current search query
	width       int          // terminal width
	height      int          // terminal height
	statusMsg   string       // bottom status bar message
	ready       bool         // initial size received
}

// New creates a new TUI Model with the given widgets. The first widget
// receives initial focus, no widget is expanded, and help is hidden.
func New(widgets []app.Widget) Model {
	return Model{
		widgets:  widgets,
		focused:  0,
		expanded: -1,
	}
}

// Init implements tea.Model. No initial commands are needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It routes messages to the appropriate handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		// Forward resize to all widgets so they can invalidate caches.
		for _, w := range m.widgets {
			w.Update(msg)
		}
		return m, nil

	case tea.KeyMsg:
		// Clamp focused index before handling keys.
		if len(m.widgets) > 0 && m.focused >= len(m.widgets) {
			m.focused = len(m.widgets) - 1
		}
		return tuiHandleKey(m, msg)

	case app.DataUpdateEvent:
		// Forward data updates to all widgets so they can react to new data.
		var cmds []tea.Cmd
		for _, w := range m.widgets {
			if cmd := w.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil
	}

	// Forward any other messages to all widgets.
	var cmds []tea.Cmd
	for _, w := range m.widgets {
		if cmd := w.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// View implements tea.Model. It renders the grid, expanded widget, help
// overlay, or search bar depending on the current state.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Guard: clamp focused index to widget bounds.
	if len(m.widgets) > 0 && m.focused >= len(m.widgets) {
		m.focused = len(m.widgets) - 1
	}

	// Determine which widgets are visible (search filtering).
	visibleIndices := tuiVisibleIndices(m)

	var content string

	if m.expanded >= 0 && m.expanded < len(m.widgets) {
		// Render the expanded widget fullscreen (minus status bar row).
		content = tuiRenderExpanded(m.widgets[m.expanded], m.width, m.height-1)
	} else if len(m.widgets) > 0 {
		// Compute grid layout for visible widgets.
		cells := tuiComputeGrid(m.widgets, m.width, m.height, visibleIndices, m.focused)
		content = tuiRenderGrid(cells, m.width, m.height-1)
	}

	// Render the bottom bar: search bar or status bar.
	var bottomBar string
	if m.searchMode {
		bottomBar = tuiRenderSearchBar(m.searchQuery, m.width)
	} else {
		bottomBar = tuiRenderStatusBar(m.statusMsg, m.width)
	}

	// Help overlay replaces content if visible.
	if m.showHelp {
		content = tuiRenderHelp(m.width, m.height-1)
	}

	return content + "\n" + bottomBar
}

// tuiVisibleIndices returns the indices of widgets that should be displayed,
// taking into account search filtering.
func tuiVisibleIndices(m Model) []int {
	if m.searchMode && m.searchQuery != "" {
		return tuiFilterWidgets(m.widgets, m.searchQuery)
	}
	indices := make([]int, len(m.widgets))
	for i := range m.widgets {
		indices[i] = i
	}
	return indices
}

// Focused returns the index of the currently focused widget.
func (m Model) Focused() int {
	return m.focused
}

// Expanded returns the index of the expanded widget (-1 if none).
func (m Model) Expanded() int {
	return m.expanded
}

// ShowHelp returns whether the help overlay is visible.
func (m Model) ShowHelp() bool {
	return m.showHelp
}

// SearchMode returns whether search mode is active.
func (m Model) SearchMode() bool {
	return m.searchMode
}

// SearchQuery returns the current search query string.
func (m Model) SearchQuery() string {
	return m.searchQuery
}

// Width returns the current terminal width.
func (m Model) Width() int {
	return m.width
}

// Height returns the current terminal height.
func (m Model) Height() int {
	return m.height
}

// Ready returns whether the initial window size has been received.
func (m Model) Ready() bool {
	return m.ready
}
