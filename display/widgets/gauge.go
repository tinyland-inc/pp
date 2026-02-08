package widgets

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GaugeConfig controls the appearance and behavior of a horizontal bar gauge.
type GaugeConfig struct {
	// Width is the total character width of the gauge bar.
	Width int
	// Percent is the value from 0 to 100.
	Percent float64
	// Label is optional text shown to the left of the bar.
	Label string
	// ShowPercent controls whether "XX%" is shown to the right.
	ShowPercent bool
	// ThresholdWarning is the % at which color changes to yellow (default: 70).
	ThresholdWarning float64
	// ThresholdDanger is the % at which color changes to red (default: 90).
	ThresholdDanger float64
	// FilledChar is the character for filled portion (default: "█").
	FilledChar string
	// EmptyChar is the character for empty portion (default: "░").
	EmptyChar string
}

// DefaultGaugeConfig returns a GaugeConfig with sensible defaults.
func DefaultGaugeConfig() GaugeConfig {
	return GaugeConfig{
		Width:            20,
		ShowPercent:       true,
		ThresholdWarning: 70,
		ThresholdDanger:  90,
		FilledChar:       "█",
		EmptyChar:        "░",
	}
}

// gaugeColor returns the lipgloss color for the given percentage based on thresholds.
func gaugeColor(percent, warning, danger float64) lipgloss.Color {
	switch {
	case percent >= danger:
		return lipgloss.Color("#EF4444")
	case percent >= warning:
		return lipgloss.Color("#EAB308")
	default:
		return lipgloss.Color("#22C55E")
	}
}

// RenderGauge renders a horizontal bar gauge with optional label and percentage.
// Format: [Label] [████████░░░░] [XX%]
func RenderGauge(cfg GaugeConfig) string {
	// Clamp percent to 0-100.
	percent := math.Max(0, math.Min(100, cfg.Percent))

	// Resolve defaults for characters if empty.
	filledChar := cfg.FilledChar
	if filledChar == "" {
		filledChar = "█"
	}
	emptyChar := cfg.EmptyChar
	if emptyChar == "" {
		emptyChar = "░"
	}

	width := cfg.Width
	if width <= 0 {
		width = 20
	}

	// Calculate filled and empty counts.
	filledCount := int(math.Round(percent / 100.0 * float64(width)))
	emptyCount := width - filledCount

	// Build the bar segments.
	filledStr := strings.Repeat(filledChar, filledCount)
	emptyStr := strings.Repeat(emptyChar, emptyCount)

	// Apply color to filled portion.
	color := gaugeColor(percent, cfg.ThresholdWarning, cfg.ThresholdDanger)
	style := lipgloss.NewStyle().Foreground(color)
	coloredFilled := style.Render(filledStr)

	// Assemble the bar.
	bar := coloredFilled + emptyStr

	// Build final output.
	var sb strings.Builder

	if cfg.Label != "" {
		sb.WriteString(cfg.Label)
		sb.WriteString(" ")
	}

	sb.WriteString(bar)

	if cfg.ShowPercent {
		sb.WriteString(" ")
		sb.WriteString(fmt.Sprintf("%3.0f%%", percent))
	}

	return sb.String()
}

// RenderMiniGauge renders a compact gauge bar with no label or percentage text.
// Uses the same color thresholds as RenderGauge.
func RenderMiniGauge(percent float64, width int) string {
	return RenderGauge(GaugeConfig{
		Width:            width,
		Percent:          percent,
		ShowPercent:      false,
		ThresholdWarning: 70,
		ThresholdDanger:  90,
		FilledChar:       "█",
		EmptyChar:        "░",
	})
}
