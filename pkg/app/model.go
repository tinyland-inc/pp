package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/layout"
)

// Config holds application-level configuration for the dashboard.
type Config struct {
	// RefreshInterval controls how often the tick fires for data refresh.
	RefreshInterval time.Duration

	// DefaultLayout is the layout preset to use on startup.
	DefaultLayout string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		RefreshInterval: 2 * time.Second,
		DefaultLayout:   "default",
	}
}

// Widget is the interface that all dashboard widgets must implement.
// It is intentionally minimal so that widget authors can focus on rendering.
type Widget interface {
	// ID returns a unique identifier for this widget.
	ID() string

	// Title returns the human-readable display name.
	Title() string

	// Update handles messages directed at this widget. It returns an optional
	// command, following the Elm architecture pattern.
	Update(msg tea.Msg) tea.Cmd

	// View renders the widget content into the given area dimensions.
	// The returned string must fit within width x height cells.
	View(width, height int) string

	// MinSize returns the minimum width and height this widget requires.
	MinSize() (minW, minH int)

	// HandleKey processes a key event when this widget has focus.
	// Return nil if the key was not consumed.
	HandleKey(key tea.KeyMsg) tea.Cmd
}

// AppModel is the root bubbletea Model for the prompt-pulse v2 dashboard.
// It owns the widget registry, layout state, data store, and input routing.
type AppModel struct {
	// Terminal dimensions from the most recent WindowSizeMsg.
	width  int
	height int

	// Widget registry and ordering.
	widgets     map[string]Widget
	widgetOrder []string // Determines tab-cycle and render order.

	// Focus and expansion state.
	focusedWidget  string // ID of the widget that receives key events.
	expandedWidget string // Non-empty means one widget is fullscreen.

	// Layout rects computed from the current terminal size.
	layoutRects map[string]layout.Rect
	layoutDirty bool // True when a resize invalidates the cached layout.

	// Data store: latest payload from each collector, keyed by source name.
	dataStore map[string]interface{}

	// Modal states.
	helpVisible bool
	quitting    bool

	// Application configuration.
	config *Config
}

// NewAppModel creates a new AppModel with the given widgets registered in order.
// The first widget receives initial focus.
func NewAppModel(cfg *Config, widgets ...Widget) AppModel {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	wMap := make(map[string]Widget, len(widgets))
	order := make([]string, 0, len(widgets))
	for _, w := range widgets {
		wMap[w.ID()] = w
		order = append(order, w.ID())
	}

	focused := ""
	if len(order) > 0 {
		focused = order[0]
	}

	return AppModel{
		widgets:     wMap,
		widgetOrder: order,
		focusedWidget:  focused,
		layoutRects: make(map[string]layout.Rect),
		layoutDirty: true,
		dataStore:   make(map[string]interface{}),
		config:      cfg,
	}
}

// Init implements tea.Model. It returns a tick command to start the refresh
// cycle.
func (m AppModel) Init() tea.Cmd {
	return TickCmd(m.config.RefreshInterval)
}

// Update implements tea.Model. It routes messages to the appropriate handler.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layoutDirty = true
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case DataUpdateEvent:
		if msg.Err == nil && msg.Data != nil {
			m.dataStore[msg.Source] = msg.Data
		}
		// Forward to all widgets so they can react to new data.
		var cmds []tea.Cmd
		for _, w := range m.widgets {
			if cmd := w.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case TickEvent:
		return m, TickCmd(m.config.RefreshInterval)

	case WidgetFocusEvent:
		m.FocusWidget(msg.WidgetID)
		return m, nil

	case WidgetExpandEvent:
		if m.expandedWidget == msg.WidgetID {
			m.expandedWidget = ""
		} else {
			m.expandedWidget = msg.WidgetID
		}
		m.layoutDirty = true
		return m, nil

	default:
		// Forward unknown messages to all widgets.
		var cmds []tea.Cmd
		for _, w := range m.widgets {
			if cmd := w.Update(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	}
}

// handleKey processes keyboard input: global keys first, then delegates to
// the focused widget.
func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "?":
		m.helpVisible = !m.helpVisible
		return m, nil

	case "tab":
		m.CycleFocusForward()
		return m, nil

	case "shift+tab":
		m.CycleFocusBackward()
		return m, nil

	case "enter":
		m.ToggleExpand()
		m.layoutDirty = true
		return m, nil

	case "esc":
		if m.expandedWidget != "" {
			m.expandedWidget = ""
			m.layoutDirty = true
		}
		return m, nil
	}

	// Delegate to the focused widget.
	if w, ok := m.widgets[m.focusedWidget]; ok {
		if cmd := w.HandleKey(msg); cmd != nil {
			return m, cmd
		}
	}

	return m, nil
}

// View implements tea.Model. It computes the layout if dirty, renders each
// widget, and composes the final output.
func (m AppModel) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Recompute layout if the terminal was resized or a widget was expanded.
	if m.layoutDirty {
		m.computeLayout()
	}

	// If a widget is expanded, render only that widget.
	if m.expandedWidget != "" {
		if w, ok := m.widgets[m.expandedWidget]; ok {
			content := w.View(m.width, m.height-1) // Reserve 1 line for status.
			status := m.renderStatusBar()
			return lipgloss.JoinVertical(lipgloss.Left, content, status)
		}
	}

	// Render all widgets into their layout rects.
	var rows []string
	currentRowWidgets := make([]string, 0)

	for _, id := range m.widgetOrder {
		w, ok := m.widgets[id]
		if !ok {
			continue
		}
		rect, ok := m.layoutRects[id]
		if !ok {
			rect = layout.Rect{Width: m.width, Height: 5}
		}

		rendered := m.renderWidget(w, rect, id == m.focusedWidget)

		currentRowWidgets = append(currentRowWidgets, rendered)
	}

	// For now, stack widgets vertically. The layout engine can evolve to
	// support grid placement in later weeks.
	if len(currentRowWidgets) > 0 {
		rows = append(rows, currentRowWidgets...)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	// Overlay help if visible.
	if m.helpVisible {
		helpOverlay := m.renderHelp()
		content = lipgloss.JoinVertical(lipgloss.Left, content, helpOverlay)
	}

	return content
}

// renderWidget renders a single widget with an optional focus border.
func (m AppModel) renderWidget(w Widget, rect layout.Rect, focused bool) string {
	width := rect.Width
	height := rect.Height

	if width <= 0 {
		width = m.width
	}
	if height <= 0 {
		height = 5
	}

	// Reserve space for the border (2 chars width, 2 chars height).
	innerW := width - 2
	innerH := height - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	content := w.View(innerW, innerH)

	borderColor := lipgloss.Color("#6B7280") // Muted gray
	if focused {
		borderColor = lipgloss.Color("#7C3AED") // Purple for focus
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(innerW).
		Height(innerH)

	title := w.Title()
	if focused {
		title = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true).
			Render(title)
	}

	// Set the border title via lipgloss (v0.9+ supports BorderTop title).
	style = style.BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true)

	// Render the content with border, then prepend the title line.
	box := style.Render(content)

	// Replace the first border line with one that includes the title.
	lines := strings.SplitN(box, "\n", 2)
	if len(lines) >= 1 && title != "" {
		topBorder := lines[0]
		topRunes := []rune(topBorder)
		titleStr := " " + title + " "
		titleRunes := []rune(titleStr)

		if len(topRunes) >= 2+len(titleRunes) {
			// Title fits: inject it after the corner character.
			lines[0] = string(topRunes[:2]) + titleStr + string(topRunes[2+len(titleRunes):])
		} else if len(topRunes) > 4 {
			// Title too wide: truncate it to fit within the border.
			maxTitle := len(topRunes) - 4
			lines[0] = string(topRunes[:2]) + " " + string(titleRunes[1:maxTitle+1]) + " " + string(topRunes[len(topRunes)-1:])
		}
		// else: border too narrow for any title, leave as-is.
	}

	return strings.Join(lines, "\n")
}

// computeLayout distributes available terminal space among registered widgets.
// For now this uses a simple vertical stack; the constraint solver from
// pkg/layout will be integrated in a later week.
func (m *AppModel) computeLayout() {
	if len(m.widgetOrder) == 0 {
		return
	}

	availableHeight := m.height
	if m.helpVisible {
		availableHeight -= 4 // Reserve space for help overlay.
	}

	perWidget := availableHeight / len(m.widgetOrder)
	if perWidget < 3 {
		perWidget = 3
	}

	y := 0
	for _, id := range m.widgetOrder {
		m.layoutRects[id] = layout.Rect{
			X:      0,
			Y:      y,
			Width:  m.width,
			Height: perWidget,
		}
		y += perWidget
	}

	m.layoutDirty = false
}

// renderStatusBar renders a one-line status bar at the bottom.
func (m AppModel) renderStatusBar() string {
	status := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Render("Press ? for help | Tab to cycle widgets | Enter to expand | Esc to collapse | q to quit")
	return status
}

// renderHelp renders the help overlay content.
func (m AppModel) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(0, 1)

	lines := []string{
		lipgloss.NewStyle().Bold(true).Render("Keybindings"),
		"",
		"  Tab / Shift+Tab   Cycle widget focus",
		"  Enter             Expand focused widget",
		"  Esc               Collapse expanded widget",
		"  ?                 Toggle this help",
		"  q / Ctrl+C        Quit",
	}

	return helpStyle.Render(strings.Join(lines, "\n"))
}

// DataStore returns the current data store for testing and inspection.
func (m AppModel) DataStore() map[string]interface{} {
	return m.dataStore
}

// Width returns the current terminal width.
func (m AppModel) Width() int {
	return m.width
}

// Height returns the current terminal height.
func (m AppModel) Height() int {
	return m.height
}

// LayoutDirty returns whether the layout needs recomputation.
func (m AppModel) LayoutDirty() bool {
	return m.layoutDirty
}

// FocusedWidget returns the ID of the currently focused widget.
func (m AppModel) FocusedWidgetID() string {
	return m.focusedWidget
}

// ExpandedWidget returns the ID of the expanded widget, or empty string.
func (m AppModel) ExpandedWidgetID() string {
	return m.expandedWidget
}

// HelpVisible returns whether the help overlay is shown.
func (m AppModel) HelpVisible() bool {
	return m.helpVisible
}

// Quitting returns whether the application is in the process of quitting.
func (m AppModel) Quitting() bool {
	return m.quitting
}
