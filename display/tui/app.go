package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// Tab identifies which tab is currently active.
type Tab int

const (
	TabClaude Tab = iota
	TabBilling
	TabInfra
	tabCount // sentinel for wrapping
)

// tabNames maps each Tab value to its display label.
var tabNames = map[Tab]string{
	TabClaude:  "Claude",
	TabBilling: "Billing",
	TabInfra:   "Infra",
}

// Model is the top-level Bubbletea model for the prompt-pulse TUI.
type Model struct {
	activeTab   Tab
	width       int
	height      int
	claude      *collectors.ClaudeUsage
	billing     *collectors.BillingData
	infra       *collectors.InfraStatus
	lastUpdated time.Time
	ready       bool
}

// NewModel returns an initialized Model with TabClaude active.
func NewModel() Model {
	return Model{
		activeTab: TabClaude,
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

// Init implements tea.Model. No initial commands are needed.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. It handles key presses and window resize events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.NextTab):
			m.activeTab = (m.activeTab + 1) % tabCount
		case key.Matches(msg, keys.PrevTab):
			m.activeTab = (m.activeTab - 1 + tabCount) % tabCount
		case key.Matches(msg, keys.Tab1):
			m.activeTab = TabClaude
		case key.Matches(msg, keys.Tab2):
			m.activeTab = TabBilling
		case key.Matches(msg, keys.Tab3):
			m.activeTab = TabInfra
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	}

	return m, nil
}

// View implements tea.Model. It renders the header, active tab content, and footer.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	header := m.renderHeader()
	content := m.renderTabContent()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

// renderHeader renders the tab bar with the active tab highlighted.
func (m Model) renderHeader() string {
	var tabs []string
	for i := Tab(0); i < tabCount; i++ {
		name := tabNames[i]
		if i == m.activeTab {
			tabs = append(tabs, styleActiveTab.Render(name))
		} else {
			tabs = append(tabs, styleInactiveTab.Render(name))
		}
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	return styleHeader.Width(m.width).Render(tabBar)
}

// renderTabContent delegates to the appropriate tab renderer based on the active tab.
func (m Model) renderTabContent() string {
	// Reserve space for header and footer (approximate).
	contentHeight := m.height - 6
	if contentHeight < 1 {
		contentHeight = 1
	}

	var content string
	switch m.activeTab {
	case TabClaude:
		content = renderClaudeContent(m.claude, m.width, contentHeight)
	case TabBilling:
		content = renderBillingContent(m.billing, m.width, contentHeight)
	case TabInfra:
		content = renderInfraContent(m.infra, m.width, contentHeight)
	default:
		content = ""
	}

	return styleContent.Width(m.width).Render(content)
}

// renderFooter renders the help text and last updated timestamp.
func (m Model) renderFooter() string {
	help := "q: quit | tab: switch | 1-3: jump"

	var timestamp string
	if !m.lastUpdated.IsZero() {
		timestamp = fmt.Sprintf("  Updated: %s", m.lastUpdated.Format("15:04:05"))
	}

	return styleFooter.Width(m.width).Render(help + timestamp)
}
