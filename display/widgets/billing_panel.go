// Package widgets provides reusable UI components for prompt-pulse displays.
package widgets

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// BillingPanelConfig controls the appearance and behavior of the billing dashboard panel.
type BillingPanelConfig struct {
	// Width is the total character width of the panel.
	Width int
	// ColorEnabled enables ANSI color output.
	ColorEnabled bool
	// SparklineWidth is the number of characters for sparkline charts.
	SparklineWidth int
	// GaugeWidth is the character width for budget gauges.
	GaugeWidth int
	// ShowProviderDetails enables per-provider detail rows.
	ShowProviderDetails bool
}

// DefaultBillingPanelConfig returns sensible defaults.
func DefaultBillingPanelConfig() BillingPanelConfig {
	return BillingPanelConfig{
		Width:               60,
		ColorEnabled:        true,
		SparklineWidth:      20,
		GaugeWidth:          15,
		ShowProviderDetails: true,
	}
}

// ProviderSpend holds billing data for a single provider with history for sparklines.
type ProviderSpend struct {
	// Name identifies the provider: "civo", "digitalocean", "aws", "dreamhost".
	Name string
	// Current is the current month spend in USD.
	Current float64
	// Forecast is the projected end-of-month spend in USD.
	Forecast *float64
	// Budget is the monthly budget limit in USD.
	Budget *float64
	// PreviousMonth is last month's total spend in USD, if available.
	PreviousMonth *float64
	// History contains up to 30 days of daily spend for sparkline rendering.
	// Most recent day is last.
	History []float64
	// Status indicates data freshness: "ok", "error", "stale".
	Status string
}

// BillingPanelData holds all data for rendering the billing dashboard.
type BillingPanelData struct {
	// Providers contains per-provider billing data with history.
	Providers []ProviderSpend
	// TotalCurrent is the aggregate current month spend.
	TotalCurrent float64
	// TotalForecast is the aggregate forecasted spend.
	TotalForecast *float64
	// TotalBudget is the aggregate monthly budget.
	TotalBudget *float64
	// FetchedAt is when this data was last collected.
	FetchedAt time.Time
}

// RenderBillingPanel renders the complete billing dashboard with sparklines and gauges.
func RenderBillingPanel(data BillingPanelData, cfg BillingPanelConfig) string {
	var lines []string

	// Color palette.
	colorSuccess := lipgloss.Color("#22C55E")
	colorWarning := lipgloss.Color("#EAB308")
	colorDanger := lipgloss.Color("#EF4444")
	colorMuted := lipgloss.Color("#6B7280")
	colorPrimary := lipgloss.Color("#06B6D4")

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(colorMuted)
	spendStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))

	// Title.
	if cfg.ColorEnabled {
		lines = append(lines, titleStyle.Render("Billing Dashboard"))
	} else {
		lines = append(lines, "Billing Dashboard")
	}
	lines = append(lines, "")

	// Total summary row with gauge.
	totalLine := renderTotalSummary(data, cfg, spendStyle, labelStyle, colorSuccess, colorWarning, colorDanger)
	lines = append(lines, totalLine)
	lines = append(lines, "")

	// Per-provider rows with sparklines and gauges.
	if cfg.ShowProviderDetails && len(data.Providers) > 0 {
		for _, p := range data.Providers {
			providerLines := renderProviderRow(p, cfg, labelStyle, colorSuccess, colorWarning, colorDanger, colorMuted)
			lines = append(lines, providerLines...)
		}
	}

	// Fetch timestamp.
	if !data.FetchedAt.IsZero() {
		timestamp := data.FetchedAt.Format("15:04:05")
		if cfg.ColorEnabled {
			lines = append(lines, labelStyle.Render(fmt.Sprintf("Updated: %s", timestamp)))
		} else {
			lines = append(lines, fmt.Sprintf("Updated: %s", timestamp))
		}
	}

	return strings.Join(lines, "\n")
}

// renderTotalSummary renders the aggregate billing summary with budget gauge.
func renderTotalSummary(
	data BillingPanelData,
	cfg BillingPanelConfig,
	spendStyle, labelStyle lipgloss.Style,
	colorSuccess, colorWarning, colorDanger lipgloss.Color,
) string {
	var parts []string

	// Current spend.
	spendText := fmt.Sprintf("$%.2f", data.TotalCurrent)
	if cfg.ColorEnabled {
		parts = append(parts, labelStyle.Render("Total: ")+spendStyle.Render(spendText))
	} else {
		parts = append(parts, fmt.Sprintf("Total: %s", spendText))
	}

	// Forecast.
	if data.TotalForecast != nil {
		forecastText := fmt.Sprintf("$%.2f forecast", *data.TotalForecast)
		if cfg.ColorEnabled {
			parts = append(parts, labelStyle.Render(forecastText))
		} else {
			parts = append(parts, forecastText)
		}
	}

	// Budget gauge.
	if data.TotalBudget != nil && *data.TotalBudget > 0 {
		percent := (data.TotalCurrent / *data.TotalBudget) * 100
		gaugeStr := RenderGauge(GaugeConfig{
			Width:            cfg.GaugeWidth,
			Percent:          percent,
			ShowPercent:      true,
			ThresholdWarning: 70,
			ThresholdDanger:  90,
		})
		parts = append(parts, gaugeStr)
	}

	return strings.Join(parts, " | ")
}

// renderProviderRow renders a single provider with sparkline and gauge.
func renderProviderRow(
	p ProviderSpend,
	cfg BillingPanelConfig,
	labelStyle lipgloss.Style,
	colorSuccess, colorWarning, colorDanger, colorMuted lipgloss.Color,
) []string {
	var lines []string

	// Provider name with status indicator.
	statusIcon := "+"
	statusColor := colorSuccess
	if p.Status == "error" {
		statusIcon = "x"
		statusColor = colorDanger
	} else if p.Status == "stale" {
		statusIcon = "!"
		statusColor = colorWarning
	}

	var nameStr string
	if cfg.ColorEnabled {
		iconStyle := lipgloss.NewStyle().Foreground(statusColor)
		nameStr = iconStyle.Render(statusIcon) + " " + strings.Title(p.Name)
	} else {
		nameStr = statusIcon + " " + strings.Title(p.Name)
	}

	// Current spend.
	spendStr := fmt.Sprintf("$%.2f", p.Current)

	// Sparkline for history.
	var sparklineStr string
	if len(p.History) > 0 {
		sparklineStr = RenderSparkline(SparklineConfig{
			Data:  p.History,
			Width: cfg.SparklineWidth,
			Color: colorMuted,
		})
	} else {
		// No history: show placeholder.
		sparklineStr = strings.Repeat("-", cfg.SparklineWidth)
		if cfg.ColorEnabled {
			sparklineStr = lipgloss.NewStyle().Foreground(colorMuted).Render(sparklineStr)
		}
	}

	// Budget gauge (if budget is set).
	var gaugeStr string
	if p.Budget != nil && *p.Budget > 0 {
		percent := (p.Current / *p.Budget) * 100
		gaugeStr = RenderMiniGauge(percent, cfg.GaugeWidth)
	}

	// Assemble the line.
	line := fmt.Sprintf("  %-14s %8s  %s", nameStr, spendStr, sparklineStr)
	if gaugeStr != "" {
		line += "  " + gaugeStr
	}
	lines = append(lines, line)

	// Optional forecast line.
	if p.Forecast != nil {
		forecastLine := fmt.Sprintf("    forecast: $%.2f", *p.Forecast)
		if cfg.ColorEnabled {
			forecastLine = labelStyle.Render(forecastLine)
		}
		lines = append(lines, forecastLine)
	}

	// Month-over-month comparison.
	if mom := FormatMonthOverMonth(p.Current, p.PreviousMonth, cfg.ColorEnabled); mom != "" {
		momLine := "    " + mom
		lines = append(lines, momLine)
	}

	return lines
}

// FormatMonthOverMonth computes the month-over-month delta between current
// and previous spend and returns a formatted string with arrow and percentage.
// Returns empty string if previousMonth is nil or zero.
// Uses unicode arrows: up for increase, down for decrease, right for unchanged (<1% change).
// The colorEnabled flag controls whether ANSI colors are applied:
// red for increase (costs going up), green for decrease (costs going down).
func FormatMonthOverMonth(current float64, previousMonth *float64, colorEnabled bool) string {
	if previousMonth == nil || *previousMonth == 0 {
		return ""
	}

	prev := *previousMonth
	delta := current - prev
	pctChange := (delta / prev) * 100

	colorIncrease := lipgloss.Color("#EF4444") // red - costs going up is bad
	colorDecrease := lipgloss.Color("#22C55E") // green - costs going down is good
	colorFlat := lipgloss.Color("#6B7280")     // gray - unchanged

	var arrow string
	var color lipgloss.Color

	absPct := math.Abs(pctChange)
	if absPct < 1.0 {
		arrow = "\u2192" // right arrow for flat
		color = colorFlat
	} else if delta > 0 {
		arrow = "\u2191" // up arrow for increase
		color = colorIncrease
	} else {
		arrow = "\u2193" // down arrow for decrease
		color = colorDecrease
	}

	text := fmt.Sprintf("%s%.0f%% vs last mo", arrow, absPct)

	if colorEnabled {
		return lipgloss.NewStyle().Foreground(color).Render(text)
	}
	return text
}

// RenderCompactBillingPanel renders a compact single-line billing summary
// suitable for banner integration.
func RenderCompactBillingPanel(data BillingPanelData, colorEnabled bool) string {
	colorSuccess := lipgloss.Color("#22C55E")
	colorWarning := lipgloss.Color("#EAB308")
	colorDanger := lipgloss.Color("#EF4444")
	colorMuted := lipgloss.Color("#6B7280")

	var parts []string

	// Total spend.
	spendStr := fmt.Sprintf("$%.0f", data.TotalCurrent)
	parts = append(parts, spendStr)

	// Forecast if available.
	if data.TotalForecast != nil {
		parts = append(parts, fmt.Sprintf("($%.0f forecast)", *data.TotalForecast))
	}

	// Budget status.
	if data.TotalBudget != nil && *data.TotalBudget > 0 {
		percent := (data.TotalCurrent / *data.TotalBudget) * 100
		if percent >= 100 {
			if colorEnabled {
				overStyle := lipgloss.NewStyle().Bold(true).Foreground(colorDanger)
				parts = append(parts, overStyle.Render("OVER BUDGET"))
			} else {
				parts = append(parts, "OVER BUDGET")
			}
		} else if percent >= 90 {
			if colorEnabled {
				warnStyle := lipgloss.NewStyle().Foreground(colorWarning)
				parts = append(parts, warnStyle.Render(fmt.Sprintf("%.0f%% of budget", percent)))
			} else {
				parts = append(parts, fmt.Sprintf("%.0f%% of budget", percent))
			}
		} else {
			remaining := *data.TotalBudget - data.TotalCurrent
			if colorEnabled {
				remainStyle := lipgloss.NewStyle().Foreground(colorSuccess)
				parts = append(parts, remainStyle.Render(fmt.Sprintf("$%.0f remaining", remaining)))
			} else {
				parts = append(parts, fmt.Sprintf("$%.0f remaining", remaining))
			}
		}
	}

	// Mini sparkline for total trend.
	var allHistory []float64
	for _, p := range data.Providers {
		if len(p.History) > len(allHistory) {
			// Aggregate daily totals across providers.
			// For simplicity, just use the longest provider history as trend indicator.
			allHistory = p.History
		}
	}
	if len(allHistory) > 0 {
		miniSparkline := RenderSparkline(SparklineConfig{
			Data:  allHistory,
			Width: 10,
			Color: colorMuted,
		})
		parts = append(parts, miniSparkline)
	}

	return strings.Join(parts, " ")
}

// CalculateForecast computes a linear forecast based on current spend and days elapsed.
// Returns the projected end-of-month spend.
func CalculateForecast(currentSpend float64, daysElapsed, daysInMonth int) float64 {
	if daysElapsed <= 0 {
		return currentSpend
	}
	dailyRate := currentSpend / float64(daysElapsed)
	return dailyRate * float64(daysInMonth)
}
