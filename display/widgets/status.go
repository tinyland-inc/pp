package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusLevel represents the severity or state of a status indicator.
type StatusLevel int

const (
	// StatusOK indicates a healthy or successful state.
	StatusOK StatusLevel = iota
	// StatusWarning indicates a degraded or warning state.
	StatusWarning
	// StatusCritical indicates an error or critical failure.
	StatusCritical
	// StatusUnknown indicates an indeterminate state.
	StatusUnknown
	// StatusPending indicates a pending or in-progress state.
	StatusPending
)

// StatusConfig holds the configuration for rendering a status indicator.
type StatusConfig struct {
	// Level determines the color and icon.
	Level StatusLevel
	// Text is the label shown next to the indicator.
	Text string
	// ShowIcon controls whether the colored dot is shown.
	ShowIcon bool
}

// statusIcons maps each status level to its display icon.
var statusIcons = map[StatusLevel]string{
	StatusOK:       "\u25CF", // ● green dot
	StatusWarning:  "\u25CF", // ● yellow dot
	StatusCritical: "\u25CF", // ● red dot
	StatusUnknown:  "\u25CB", // ○ gray outline
	StatusPending:  "\u25CC", // ◌ dotted circle
}

// statusColors maps each status level to its display color.
var statusColors = map[StatusLevel]lipgloss.Color{
	StatusOK:       lipgloss.Color("#22C55E"),
	StatusWarning:  lipgloss.Color("#EAB308"),
	StatusCritical: lipgloss.Color("#EF4444"),
	StatusUnknown:  lipgloss.Color("#6B7280"),
	StatusPending:  lipgloss.Color("#3B82F6"),
}

// RenderStatus renders a status indicator with an optional colored icon and text.
func RenderStatus(cfg StatusConfig) string {
	color := statusColors[cfg.Level]
	style := lipgloss.NewStyle().Foreground(color)

	if cfg.ShowIcon {
		icon := statusIcons[cfg.Level]
		coloredIcon := style.Render(icon)
		if cfg.Text == "" {
			return coloredIcon
		}
		return coloredIcon + " " + cfg.Text
	}

	return style.Render(cfg.Text)
}

// RenderStatusFromString renders a status indicator from a plain status string.
// Common status strings are mapped to appropriate levels with ShowIcon enabled.
func RenderStatusFromString(status string) string {
	level := StatusLevelFromString(status)
	return RenderStatus(StatusConfig{
		Level:    level,
		Text:     status,
		ShowIcon: true,
	})
}

// StatusLevelFromString maps a status string to a StatusLevel.
// Matching is case-insensitive for lowercase variants.
func StatusLevelFromString(status string) StatusLevel {
	lower := strings.ToLower(status)

	switch {
	case lower == "ok" || lower == "healthy" || status == "Ready":
		return StatusOK
	case lower == "warning" || lower == "degraded":
		return StatusWarning
	case lower == "error" || lower == "critical" || lower == "auth_failed" || status == "NotReady":
		return StatusCritical
	case lower == "pending" || lower == "limited":
		return StatusPending
	default:
		return StatusUnknown
	}
}
