// Package widgets provides Bubble Tea components for prompt-pulse display.
package widgets

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// Color palette for Claude panel (matches theme.go).
var (
	claudeColorSuccess = lipgloss.Color("#22C55E") // Green - healthy status
	claudeColorWarning = lipgloss.Color("#EAB308") // Yellow - warning status
	claudeColorDanger  = lipgloss.Color("#EF4444") // Red - critical status
	claudeColorMuted   = lipgloss.Color("#6B7280") // Gray - muted text
	claudeColorPrimary = lipgloss.Color("#7C3AED") // Purple - headers
	claudeColorAccent  = lipgloss.Color("#06B6D4") // Cyan - accents
)

// AccountDisplay holds the display state for a single Claude account.
type AccountDisplay struct {
	// Name is the account identifier.
	Name string
	// Type is "subscription" or "api".
	Type string
	// Tier is the subscription/API tier (e.g., "pro", "max_5x").
	Tier string
	// Status is the account status ("ok", "auth_failed", etc).
	Status string

	// SessionBar is the Bubble Tea progress model for 5h/session usage.
	SessionBar progress.Model
	// WeeklyBar is the Bubble Tea progress model for 7d/weekly usage.
	WeeklyBar progress.Model

	// SessionPercent is the session utilization (0-100).
	SessionPercent float64
	// WeeklyPercent is the weekly utilization (0-100).
	WeeklyPercent float64

	// SessionReset is the formatted countdown (e.g., "2h 15m").
	SessionReset string
	// WeeklyReset is the formatted countdown (e.g., "3d 12h").
	WeeklyReset string

	// StatusIcon is the color-coded status indicator.
	StatusIcon string

	// ExtraUsage holds overuse credit data (subscription only).
	ExtraUsage *collectors.ExtraUsage
}

// ClaudePanel is a Bubble Tea component for displaying Claude account usage.
type ClaudePanel struct {
	// Accounts is the list of account displays.
	Accounts []AccountDisplay
	// Width is the panel width in characters.
	Width int
	// ShowCompact controls whether to show a compact single-line view.
	ShowCompact bool
}

// NewClaudePanel creates a ClaudePanel from collector data.
func NewClaudePanel(data *collectors.ClaudeUsage, width int) *ClaudePanel {
	if data == nil {
		return &ClaudePanel{Width: width}
	}

	panel := &ClaudePanel{
		Accounts: make([]AccountDisplay, 0, len(data.Accounts)),
		Width:    width,
	}

	// Support up to 5 accounts as per spec
	maxAccounts := 5
	if len(data.Accounts) < maxAccounts {
		maxAccounts = len(data.Accounts)
	}

	for i := 0; i < maxAccounts; i++ {
		acct := data.Accounts[i]
		display := panel.createAccountDisplay(acct)
		panel.Accounts = append(panel.Accounts, display)
	}

	return panel
}

// createAccountDisplay converts a ClaudeAccountUsage to an AccountDisplay.
func (p *ClaudePanel) createAccountDisplay(acct collectors.ClaudeAccountUsage) AccountDisplay {
	display := AccountDisplay{
		Name:   acct.Name,
		Type:   acct.Type,
		Tier:   acct.Tier,
		Status: acct.Status,
	}

	// Get reset schedule for countdown calculations
	schedule := acct.GetResetSchedule()

	if acct.Type == "subscription" {
		// Session (5h) usage
		if acct.FiveHour != nil {
			display.SessionPercent = acct.FiveHour.Utilization
			display.SessionBar = p.createProgressBar(acct.FiveHour.Utilization)
			if !acct.FiveHour.ResetsAt.IsZero() {
				display.SessionReset = FormatCountdown(acct.FiveHour.ResetsAt)
			}
		}

		// Weekly (7d) usage
		if acct.SevenDay != nil {
			display.WeeklyPercent = acct.SevenDay.Utilization
			display.WeeklyBar = p.createProgressBar(acct.SevenDay.Utilization)
			if !acct.SevenDay.ResetsAt.IsZero() {
				display.WeeklyReset = FormatCountdown(acct.SevenDay.ResetsAt)
			}
		}

		// Extra usage (overuse credits)
		if acct.ExtraUsage != nil && acct.ExtraUsage.Enabled {
			display.ExtraUsage = acct.ExtraUsage
		}
	} else if acct.Type == "api" && acct.RateLimits != nil {
		// API accounts use requests as "session" and tokens as "weekly"
		rl := acct.RateLimits

		// Requests utilization
		if rl.RequestsLimit > 0 {
			used := rl.RequestsLimit - rl.RequestsRemaining
			display.SessionPercent = float64(used) / float64(rl.RequestsLimit) * 100
			display.SessionBar = p.createProgressBar(display.SessionPercent)
			if !rl.RequestsReset.IsZero() {
				display.SessionReset = FormatCountdown(rl.RequestsReset)
			}
		}

		// Tokens utilization
		if rl.TokensLimit > 0 {
			used := rl.TokensLimit - rl.TokensRemaining
			display.WeeklyPercent = float64(used) / float64(rl.TokensLimit) * 100
			display.WeeklyBar = p.createProgressBar(display.WeeklyPercent)
			if !rl.TokensReset.IsZero() {
				display.WeeklyReset = FormatCountdown(rl.TokensReset)
			}
		}
	}

	// Set status icon based on utilization
	display.StatusIcon = p.getStatusIcon(acct.Status, display.SessionPercent, display.WeeklyPercent)

	// Use reset schedule if individual resets weren't set
	if display.SessionReset == "" && !schedule.SessionResets.IsZero() {
		display.SessionReset = FormatCountdown(schedule.SessionResets)
	}
	if display.WeeklyReset == "" && !schedule.WeeklyResets.IsZero() {
		display.WeeklyReset = FormatCountdown(schedule.WeeklyResets)
	}

	return display
}

// createProgressBar creates a Bubble Tea progress model with color thresholds.
func (p *ClaudePanel) createProgressBar(percent float64) progress.Model {
	// Calculate bar width based on panel width
	barWidth := p.Width - 30 // Reserve space for labels
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 30 {
		barWidth = 30
	}

	// Set color based on utilization thresholds
	var color lipgloss.Color
	switch {
	case percent >= 90:
		color = claudeColorDanger
	case percent >= 70:
		color = claudeColorWarning
	default:
		color = claudeColorSuccess
	}

	bar := progress.New(
		progress.WithWidth(barWidth),
		progress.WithoutPercentage(),
		progress.WithSolidFill(string(color)),
	)

	return bar
}

// getStatusIcon returns a color-coded status emoji based on status and utilization.
func (p *ClaudePanel) getStatusIcon(status string, sessionPct, weeklyPct float64) string {
	// Check status first
	if status != "ok" && status != "active" {
		return lipgloss.NewStyle().Foreground(claudeColorDanger).Render("*")
	}

	// Check utilization thresholds
	maxUtil := sessionPct
	if weeklyPct > maxUtil {
		maxUtil = weeklyPct
	}

	switch {
	case maxUtil >= 90:
		return lipgloss.NewStyle().Foreground(claudeColorDanger).Render("*")
	case maxUtil >= 70:
		return lipgloss.NewStyle().Foreground(claudeColorWarning).Render("*")
	default:
		return lipgloss.NewStyle().Foreground(claudeColorSuccess).Render("*")
	}
}

// Render produces the full panel output as a string.
func (p *ClaudePanel) Render() string {
	if len(p.Accounts) == 0 {
		return lipgloss.NewStyle().Foreground(claudeColorMuted).Render("No Claude accounts configured")
	}

	var sections []string

	for i, acct := range p.Accounts {
		section := p.renderAccount(acct)
		sections = append(sections, section)

		// Add separator between accounts (except after last)
		if i < len(p.Accounts)-1 {
			sep := lipgloss.NewStyle().Foreground(claudeColorMuted).Render(strings.Repeat("-", p.Width-4))
			sections = append(sections, sep)
		}
	}

	return strings.Join(sections, "\n")
}

// renderAccount renders a single account's display.
func (p *ClaudePanel) renderAccount(acct AccountDisplay) string {
	var lines []string

	// Header line: status icon, name, type badge, tier
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	tierStyle := lipgloss.NewStyle().Foreground(claudeColorAccent)
	badgeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(claudeColorPrimary).
		Padding(0, 1)

	typeBadge := "API"
	if acct.Type == "subscription" {
		typeBadge = "SUB"
	}

	header := fmt.Sprintf("%s %s %s %s",
		acct.StatusIcon,
		headerStyle.Render(acct.Name),
		badgeStyle.Render(typeBadge),
		tierStyle.Render(acct.Tier))
	lines = append(lines, header)

	// Status line if not OK
	if acct.Status != "ok" && acct.Status != "active" {
		statusStyle := lipgloss.NewStyle().Foreground(claudeColorDanger)
		lines = append(lines, "  "+statusStyle.Render("Status: "+acct.Status))
		return strings.Join(lines, "\n")
	}

	labelStyle := lipgloss.NewStyle().Foreground(claudeColorMuted).Width(10)
	resetStyle := lipgloss.NewStyle().Foreground(claudeColorMuted).Italic(true)

	// Session/5h bar
	if acct.SessionPercent > 0 || acct.SessionBar.Width > 0 {
		label := "5h"
		if acct.Type == "api" {
			label = "Req"
		}

		barLine := fmt.Sprintf("  %s %s %3.0f%%",
			labelStyle.Render(label),
			acct.SessionBar.ViewAs(acct.SessionPercent/100),
			acct.SessionPercent)

		if acct.SessionReset != "" {
			barLine += " " + resetStyle.Render("("+acct.SessionReset+")")
		}
		lines = append(lines, barLine)
	}

	// Weekly/7d bar
	if acct.WeeklyPercent > 0 || acct.WeeklyBar.Width > 0 {
		label := "7d"
		if acct.Type == "api" {
			label = "Tok"
		}

		barLine := fmt.Sprintf("  %s %s %3.0f%%",
			labelStyle.Render(label),
			acct.WeeklyBar.ViewAs(acct.WeeklyPercent/100),
			acct.WeeklyPercent)

		if acct.WeeklyReset != "" {
			barLine += " " + resetStyle.Render("("+acct.WeeklyReset+")")
		}
		lines = append(lines, barLine)
	}

	// Extra usage (overuse credits) gauge
	if acct.ExtraUsage != nil {
		extraBar := p.createProgressBar(acct.ExtraUsage.Utilization)
		usedDollars := acct.ExtraUsage.UsedCredits / 100.0
		limitDollars := float64(acct.ExtraUsage.MonthlyLimit) / 100.0

		barLine := fmt.Sprintf("  %s %s %3.0f%%",
			labelStyle.Render("Extra"),
			extraBar.ViewAs(acct.ExtraUsage.Utilization/100),
			acct.ExtraUsage.Utilization)
		lines = append(lines, barLine)

		creditLine := fmt.Sprintf("  %s",
			resetStyle.Render(fmt.Sprintf("$%.2f / $%.2f credits", usedDollars, limitDollars)))
		lines = append(lines, creditLine)
	}

	return strings.Join(lines, "\n")
}

// RenderCompact produces a single-line summary for banner display.
func (p *ClaudePanel) RenderCompact() string {
	if len(p.Accounts) == 0 {
		return lipgloss.NewStyle().Foreground(claudeColorMuted).Render("(no data)")
	}

	var parts []string
	for _, acct := range p.Accounts {
		if acct.Status != "ok" && acct.Status != "active" {
			parts = append(parts, fmt.Sprintf("%s: ERR", acct.Name))
			continue
		}

		// Format: "name: XX% (5h, Xh Xm) | YY% (7d)"
		var periods []string

		if acct.SessionPercent > 0 || acct.SessionReset != "" {
			period := fmt.Sprintf("%.0f%% (5h", acct.SessionPercent)
			if acct.SessionReset != "" {
				period += ", " + acct.SessionReset
			}
			period += ")"
			periods = append(periods, period)
		}

		if acct.WeeklyPercent > 0 || acct.WeeklyReset != "" {
			period := fmt.Sprintf("%.0f%% (7d", acct.WeeklyPercent)
			if acct.WeeklyReset != "" {
				period += ", " + acct.WeeklyReset
			}
			period += ")"
			periods = append(periods, period)
		}

		// Add extra usage indicator if present.
		if acct.ExtraUsage != nil {
			usedDollars := acct.ExtraUsage.UsedCredits / 100.0
			limitDollars := float64(acct.ExtraUsage.MonthlyLimit) / 100.0
			periods = append(periods, fmt.Sprintf("$%.0f/$%.0f extra", usedDollars, limitDollars))
		}

		if len(periods) > 0 {
			parts = append(parts, fmt.Sprintf("%s: %s", acct.Name, strings.Join(periods, " | ")))
		} else {
			parts = append(parts, fmt.Sprintf("%s: 0%% (5h)", acct.Name))
		}
	}

	return strings.Join(parts, " | ")
}

// FormatCountdown formats a time as a human-readable countdown.
// Returns strings like "2h 15m", "3d 12h", "45m", or "now" if already past.
func FormatCountdown(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	d := time.Until(t)
	if d <= 0 {
		return "now"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// GetUtilizationColor returns the appropriate color for a utilization percentage.
// Green (<70%), Yellow (70-89%), Red (>=90%)
func GetUtilizationColor(percent float64) lipgloss.Color {
	switch {
	case percent >= 90:
		return claudeColorDanger
	case percent >= 70:
		return claudeColorWarning
	default:
		return claudeColorSuccess
	}
}

// RenderUtilizationBar renders a simple text-based utilization bar with color.
func RenderUtilizationBar(percent float64, width int) string {
	if width < 5 {
		width = 5
	}

	color := GetUtilizationColor(percent)
	style := lipgloss.NewStyle().Foreground(color)

	filled := int((percent / 100) * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	bar := strings.Repeat("=", filled) + strings.Repeat("-", empty)
	return style.Render("[" + bar + "]")
}
