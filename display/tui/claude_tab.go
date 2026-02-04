package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/widgets"
)

// renderClaudeContent renders the Claude usage tab content.
// It displays per-account usage gauges, rate limits, and status indicators.
func renderClaudeContent(data *collectors.ClaudeUsage, width, height int) string {
	if data == nil || len(data.Accounts) == 0 {
		return "No Claude usage data available"
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorSecondary)
	labelStyle := lipgloss.NewStyle().Foreground(colorMuted)
	accountNameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	badgeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(colorPrimary).
		Padding(0, 1)
	tierStyle := lipgloss.NewStyle().Foreground(colorSecondary)
	separatorStyle := lipgloss.NewStyle().Foreground(colorMuted)

	var sections []string

	// Section title.
	sections = append(sections, titleStyle.Render("Claude AI Usage"))
	sections = append(sections, "")

	for i, acct := range data.Accounts {
		if i > 0 {
			// Horizontal separator between accounts.
			sepWidth := width - 4
			if sepWidth < 10 {
				sepWidth = 10
			}
			sections = append(sections, separatorStyle.Render(strings.Repeat("\u2500", sepWidth)))
			sections = append(sections, "")
		}

		// Header line: name, type badge, tier, status.
		typeBadge := "API"
		if acct.Type == "subscription" {
			typeBadge = "SUB"
		}

		header := accountNameStyle.Render(acct.Name) +
			" " + badgeStyle.Render(typeBadge) +
			" " + tierStyle.Render(acct.Tier) +
			" " + widgets.RenderStatusFromString(acct.Status)
		sections = append(sections, header)
		sections = append(sections, "")

		// Determine gauge width based on available space.
		gaugeWidth := claudeGaugeWidth(width)

		if acct.Type == "subscription" {
			sections = append(sections, renderSubscriptionUsage(acct, gaugeWidth, labelStyle)...)
		} else if acct.Type == "api" {
			sections = append(sections, renderAPIUsage(acct, gaugeWidth, labelStyle)...)
		}
	}

	return strings.Join(sections, "\n")
}

// claudeGaugeWidth calculates gauge bar width from available display width.
func claudeGaugeWidth(width int) int {
	// Reserve space for label (~14 chars) + padding (~10 chars).
	labelWidth := 14
	gaugeWidth := width - labelWidth - 10
	if gaugeWidth > 30 {
		gaugeWidth = 30
	}
	if gaugeWidth < 8 {
		gaugeWidth = 8
	}
	return gaugeWidth
}

// renderSubscriptionUsage renders the gauge bars and reset times for a subscription account.
func renderSubscriptionUsage(acct collectors.ClaudeAccountUsage, gaugeWidth int, labelStyle lipgloss.Style) []string {
	var lines []string

	if acct.FiveHour != nil {
		gauge := widgets.RenderGauge(widgets.GaugeConfig{
			Width:            gaugeWidth,
			Percent:          acct.FiveHour.Utilization,
			Label:            labelStyle.Render("5h usage"),
			ShowPercent:      true,
			ThresholdWarning: 70,
			ThresholdDanger:  90,
			FilledChar:       "\u2588",
			EmptyChar:        "\u2591",
		})
		lines = append(lines, gauge)
		lines = append(lines, labelStyle.Render("  "+formatResetTime(acct.FiveHour.ResetsAt)))
	}

	if acct.SevenDay != nil {
		gauge := widgets.RenderGauge(widgets.GaugeConfig{
			Width:            gaugeWidth,
			Percent:          acct.SevenDay.Utilization,
			Label:            labelStyle.Render("7d usage"),
			ShowPercent:      true,
			ThresholdWarning: 70,
			ThresholdDanger:  90,
			FilledChar:       "\u2588",
			EmptyChar:        "\u2591",
		})
		lines = append(lines, gauge)
		lines = append(lines, labelStyle.Render("  "+formatResetTime(acct.SevenDay.ResetsAt)))
	}

	if acct.FiveHour == nil && acct.SevenDay == nil {
		lines = append(lines, labelStyle.Render("  No usage period data"))
	}

	// Extra usage section.
	if acct.ExtraUsage != nil && acct.ExtraUsage.Enabled {
		lines = append(lines, "")
		lines = append(lines, renderExtraUsage(acct.ExtraUsage, gaugeWidth, labelStyle)...)
	}

	return lines
}

// renderExtraUsage renders the extra usage (overuse credits) section.
func renderExtraUsage(extra *collectors.ExtraUsage, gaugeWidth int, labelStyle lipgloss.Style) []string {
	var lines []string

	usedDollars := extra.UsedCredits / 100.0
	limitDollars := float64(extra.MonthlyLimit) / 100.0

	gauge := widgets.RenderGauge(widgets.GaugeConfig{
		Width:            gaugeWidth,
		Percent:          extra.Utilization,
		Label:            labelStyle.Render("Extra"),
		ShowPercent:      true,
		ThresholdWarning: 70,
		ThresholdDanger:  90,
		FilledChar:       "\u2588",
		EmptyChar:        "\u2591",
	})
	lines = append(lines, gauge)
	lines = append(lines, labelStyle.Render(fmt.Sprintf("  $%.2f / $%.2f", usedDollars, limitDollars)))

	return lines
}

// renderAPIUsage renders the gauge bars and reset times for an API account.
func renderAPIUsage(acct collectors.ClaudeAccountUsage, gaugeWidth int, labelStyle lipgloss.Style) []string {
	var lines []string

	if acct.RateLimits == nil {
		lines = append(lines, labelStyle.Render("  No rate limit data"))
		return lines
	}

	rl := acct.RateLimits

	// Requests gauge.
	requestsUsed := rl.RequestsLimit - rl.RequestsRemaining
	var requestsPct float64
	if rl.RequestsLimit > 0 {
		requestsPct = float64(requestsUsed) / float64(rl.RequestsLimit) * 100
	}
	reqGauge := widgets.RenderGauge(widgets.GaugeConfig{
		Width:            gaugeWidth,
		Percent:          requestsPct,
		Label:            labelStyle.Render("Requests"),
		ShowPercent:      true,
		ThresholdWarning: 70,
		ThresholdDanger:  90,
		FilledChar:       "\u2588",
		EmptyChar:        "\u2591",
	})
	lines = append(lines, reqGauge)
	lines = append(lines, labelStyle.Render(fmt.Sprintf("  %d / %d used", requestsUsed, rl.RequestsLimit)))
	lines = append(lines, labelStyle.Render("  "+formatResetTime(rl.RequestsReset)))

	// Tokens gauge.
	tokensUsed := rl.TokensLimit - rl.TokensRemaining
	var tokensPct float64
	if rl.TokensLimit > 0 {
		tokensPct = float64(tokensUsed) / float64(rl.TokensLimit) * 100
	}
	tokGauge := widgets.RenderGauge(widgets.GaugeConfig{
		Width:            gaugeWidth,
		Percent:          tokensPct,
		Label:            labelStyle.Render("Tokens"),
		ShowPercent:      true,
		ThresholdWarning: 70,
		ThresholdDanger:  90,
		FilledChar:       "\u2588",
		EmptyChar:        "\u2591",
	})
	lines = append(lines, tokGauge)
	lines = append(lines, labelStyle.Render(fmt.Sprintf("  %d / %d used", tokensUsed, rl.TokensLimit)))
	lines = append(lines, labelStyle.Render("  "+formatResetTime(rl.TokensReset)))

	return lines
}

// formatResetTime formats a reset time as a human-readable string.
// If the reset is less than 60 minutes away, shows "Resets in Xm".
// Otherwise, shows "Resets at HH:MM".
func formatResetTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	remaining := time.Until(t)
	if remaining <= 0 {
		return "Reset pending"
	}

	if remaining < 60*time.Minute {
		return fmt.Sprintf("Resets in %dm", int(remaining.Minutes()))
	}

	return fmt.Sprintf("Resets at %s", t.Format("15:04"))
}
