package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// Tab identifies which tab is currently active.
type Tab int

const (
	TabClaude Tab = iota
	TabBilling
	TabInfra
	TabSystem
	tabCount // sentinel for wrapping
)

// tabNames maps each Tab value to its display label.
var tabNames = map[Tab]string{
	TabClaude:  "Claude",
	TabBilling: "Billing",
	TabInfra:   "Infra",
	TabSystem:  "System",
}

// ModelConfig holds configuration passed to the TUI model.
type ModelConfig struct {
	// CacheDir is the path to the prompt-pulse cache directory.
	CacheDir string
	// CacheTTL is the maximum age of cached data before it is considered stale.
	CacheTTL time.Duration
	// RefreshInterval is the duration between automatic data refreshes.
	// Defaults to 30 seconds if zero.
	RefreshInterval time.Duration
}

// Model is the top-level Bubbletea model for the prompt-pulse TUI.
type Model struct {
	activeTab   Tab
	width       int
	height      int
	claude      *collectors.ClaudeUsage
	billing     *collectors.BillingData
	infra       *collectors.InfraStatus
	fastfetch   *collectors.FastfetchData
	sysmetrics  *collectors.SysMetricsData
	lastUpdated time.Time
	ready       bool

	// Viewport for scrollable tab content.
	viewport viewport.Model

	// Help model for keybinding display.
	help     help.Model
	showHelp bool

	// Configuration for cache-based refresh.
	config ModelConfig

	// Zone manager for clickable regions (tab headers, etc.).
	zone *zone.Manager

	// Spinner shown during initial data load.
	spinner  spinner.Model
	loading  bool // true until first dataRefreshMsg
	spinning bool // true when spinner animation is active
}

// newSpinner returns a configured spinner for data loading indication.
func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPrimary)
	return s
}

// NewModel returns an initialized Model with TabClaude active.
func NewModel() Model {
	h := help.New()
	h.ShowAll = false
	return Model{
		activeTab: TabClaude,
		help:      h,
		config: ModelConfig{
			RefreshInterval: 30 * time.Second,
		},
		zone:    zone.New(),
		spinner: newSpinner(),
	}
}

// NewModelWithConfig returns an initialized Model with the given configuration.
func NewModelWithConfig(cfg ModelConfig) Model {
	h := help.New()
	h.ShowAll = false
	if cfg.RefreshInterval == 0 {
		cfg.RefreshInterval = 30 * time.Second
	}
	loading := cfg.CacheDir != ""
	return Model{
		activeTab: TabClaude,
		help:      h,
		config:    cfg,
		zone:      zone.New(),
		spinner:   newSpinner(),
		loading:   loading,
		spinning:  loading,
	}
}

// SetClaudeData updates the model with new Claude usage data.
func (m *Model) SetClaudeData(data *collectors.ClaudeUsage) {
	m.claude = data
	m.lastUpdated = time.Now()
}

// SetBillingData updates the model with new billing data.
func (m *Model) SetBillingData(data *collectors.BillingData) {
	m.billing = data
	m.lastUpdated = time.Now()
}

// SetInfraData updates the model with new infrastructure status data.
func (m *Model) SetInfraData(data *collectors.InfraStatus) {
	m.infra = data
	m.lastUpdated = time.Now()
}

// SetFastfetchData updates the model with new fastfetch system data.
func (m *Model) SetFastfetchData(data *collectors.FastfetchData) {
	m.fastfetch = data
	m.lastUpdated = time.Now()
}

// SetSysMetricsData updates the model with new system metrics data.
func (m *Model) SetSysMetricsData(data *collectors.SysMetricsData) {
	m.sysmetrics = data
	m.lastUpdated = time.Now()
}

// tickMsg signals a periodic refresh timer has elapsed.
type tickMsg time.Time

// dataRefreshMsg carries refreshed data from the cache.
type dataRefreshMsg struct {
	claude     *collectors.ClaudeUsage
	billing    *collectors.BillingData
	infra      *collectors.InfraStatus
	fastfetch  *collectors.FastfetchData
	sysmetrics *collectors.SysMetricsData
	err        error
}

// tickCmd returns a command that fires a tickMsg after the given interval.
func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Init implements tea.Model. Returns initial commands for tick refresh.
func (m Model) Init() tea.Cmd {
	if m.config.CacheDir != "" {
		return tea.Batch(
			m.spinner.Tick,
			tickCmd(m.config.RefreshInterval),
			fetchDataCmd(m.config.CacheDir, m.config.CacheTTL),
		)
	}
	return nil
}

// Update implements tea.Model. It handles key presses, window resize, tick,
// and data refresh events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
			m.help.ShowAll = m.showHelp
			return m, nil
		case key.Matches(msg, keys.Refresh):
			if m.config.CacheDir != "" {
				m.spinning = true
				return m, tea.Batch(m.spinner.Tick, fetchDataCmd(m.config.CacheDir, m.config.CacheTTL))
			}
			return m, nil
		case key.Matches(msg, keys.NextTab):
			m.activeTab = (m.activeTab + 1) % tabCount
			m.refreshViewport()
		case key.Matches(msg, keys.PrevTab):
			m.activeTab = (m.activeTab - 1 + tabCount) % tabCount
			m.refreshViewport()
		case key.Matches(msg, keys.Tab1):
			m.activeTab = TabClaude
			m.refreshViewport()
		case key.Matches(msg, keys.Tab2):
			m.activeTab = TabBilling
			m.refreshViewport()
		case key.Matches(msg, keys.Tab3):
			m.activeTab = TabInfra
			m.refreshViewport()
		case key.Matches(msg, keys.Tab4):
			m.activeTab = TabSystem
			m.refreshViewport()
		case key.Matches(msg, keys.GoTop):
			m.viewport.GotoTop()
			return m, nil
		case key.Matches(msg, keys.GoBottom):
			m.viewport.GotoBottom()
			return m, nil
		default:
			// Forward remaining keys to viewport for scroll handling.
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.help.Width = msg.Width
		contentHeight := m.contentHeight()
		m.viewport = viewport.New(msg.Width, contentHeight)
		m.viewport.MouseWheelEnabled = true
		m.refreshViewport()

	case tea.MouseMsg:
		// Check for tab header clicks.
		for i := Tab(0); i < tabCount; i++ {
			if m.zone.Get(tabZoneID(i)).InBounds(msg) {
				if m.activeTab != i {
					m.activeTab = i
					m.refreshViewport()
				}
				return m, nil
			}
		}
		// Forward to viewport for scroll handling.
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tickMsg:
		cmds = append(cmds, tickCmd(m.config.RefreshInterval))
		if m.config.CacheDir != "" {
			cmds = append(cmds, fetchDataCmd(m.config.CacheDir, m.config.CacheTTL))
		}

	case dataRefreshMsg:
		m.loading = false
		m.spinning = false
		if msg.err == nil {
			if msg.claude != nil {
				m.claude = msg.claude
			}
			if msg.billing != nil {
				m.billing = msg.billing
			}
			if msg.infra != nil {
				m.infra = msg.infra
			}
			if msg.fastfetch != nil {
				m.fastfetch = msg.fastfetch
			}
			if msg.sysmetrics != nil {
				m.sysmetrics = msg.sysmetrics
			}
			m.lastUpdated = time.Now()
			m.refreshViewport()
		}

	case spinner.TickMsg:
		if m.spinning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model. It renders the header, active tab content, and footer.
func (m Model) View() string {
	if !m.ready {
		if m.spinning {
			return m.spinner.View() + " Initializing..."
		}
		return "Initializing..."
	}

	header := m.renderHeader()
	content := m.viewport.View()
	footer := m.renderFooter()

	output := lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
	return m.zone.Scan(output)
}

// contentHeight returns the available height for viewport content after
// reserving space for header and footer.
func (m Model) contentHeight() int {
	// Header: ~3 lines (tab bar + border + margin)
	// Footer: ~2 lines (help text + margin)
	reserved := 5
	if m.showHelp {
		reserved += 3 // expanded help takes more space
	}
	h := m.height - reserved
	if h < 1 {
		h = 1
	}
	return h
}

// refreshViewport updates the viewport content based on the active tab.
func (m *Model) refreshViewport() {
	if !m.ready {
		return
	}
	contentHeight := m.contentHeight()
	m.viewport.Width = m.width
	m.viewport.Height = contentHeight

	var content string
	switch m.activeTab {
	case TabClaude:
		content = renderClaudeContent(m.claude, m.width, contentHeight)
	case TabBilling:
		content = renderBillingContent(m.billing, m.width, contentHeight)
	case TabInfra:
		content = renderInfraContent(m.infra, m.width, contentHeight)
	case TabSystem:
		content = renderSystemContent(m.fastfetch, m.sysmetrics, m.width, contentHeight)
	}

	m.viewport.SetContent(styleContent.Width(m.width).Render(content))
}

// tabZoneID returns the bubblezone ID for a given tab.
func tabZoneID(t Tab) string {
	return fmt.Sprintf("tab-%d", t)
}

// renderHeader renders the tab bar with the active tab highlighted.
// Tab labels are wrapped in bubblezone marks for click detection.
func (m Model) renderHeader() string {
	var tabs []string
	for i := Tab(0); i < tabCount; i++ {
		name := tabNames[i]
		var rendered string
		if i == m.activeTab {
			rendered = styleActiveTab.Render(name)
		} else {
			rendered = styleInactiveTab.Render(name)
		}
		tabs = append(tabs, m.zone.Mark(tabZoneID(i), rendered))
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	return styleHeader.Width(m.width).Render(tabBar)
}

// renderFooter renders help keybindings and the last-updated timestamp.
func (m Model) renderFooter() string {
	helpView := m.help.View(keys)

	var right string
	// Scroll position indicator.
	pct := m.viewport.ScrollPercent()
	switch {
	case pct <= 0:
		right = "[top]"
	case pct >= 1:
		right = "[end]"
	default:
		right = fmt.Sprintf("[%d%%]", int(pct*100))
	}

	if m.spinning && !m.loading {
		right += "  " + m.spinner.View() + " refreshing"
	} else if !m.lastUpdated.IsZero() {
		right += fmt.Sprintf("  Updated: %s", m.lastUpdated.Format("15:04:05"))
	}

	// Right-align the timestamp/scroll indicator.
	leftWidth := lipgloss.Width(helpView)
	rightWidth := lipgloss.Width(right)
	gap := m.width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}
	padding := lipgloss.NewStyle().Width(gap).Render("")

	footerLine := helpView + padding + lipgloss.NewStyle().Foreground(colorMuted).Render(right)
	return styleFooter.Width(m.width).Render(footerLine)
}
