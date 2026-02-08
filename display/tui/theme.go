package tui

import "github.com/charmbracelet/lipgloss"

// Color palette for the monitoring dashboard theme.
const (
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple
	colorSecondary = lipgloss.Color("#06B6D4") // Cyan
	colorSuccess   = lipgloss.Color("#22C55E") // Green
	colorWarning   = lipgloss.Color("#EAB308") // Yellow
	colorDanger    = lipgloss.Color("#EF4444") // Red
	colorMuted     = lipgloss.Color("#6B7280") // Gray
	colorBg        = lipgloss.Color("#1E1B2E") // Dark purple bg
)

// Styles used throughout the TUI.
var (
	styleActiveTab   lipgloss.Style
	styleInactiveTab lipgloss.Style
	styleHeader      lipgloss.Style
	styleFooter      lipgloss.Style
	styleContent     lipgloss.Style
	styleTitle       lipgloss.Style
)

func init() {
	styleActiveTab = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(colorPrimary).
		Padding(0, 2)

	styleInactiveTab = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 2)

	styleHeader = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(colorMuted).
		MarginBottom(1)

	styleFooter = lipgloss.NewStyle().
		Foreground(colorMuted).
		MarginTop(1)

	styleContent = lipgloss.NewStyle().
		Padding(1, 2)

	styleTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(colorSecondary)
}
