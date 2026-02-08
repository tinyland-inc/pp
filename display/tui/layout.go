package tui

import (
	"strings"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/internal/format"
)

// LayoutSize represents a responsive breakpoint for terminal width.
type LayoutSize int

const (
	// LayoutCompact is used for terminals narrower than 60 characters.
	LayoutCompact LayoutSize = iota
	// LayoutNormal is used for terminals between 60 and 120 characters wide.
	LayoutNormal
	// LayoutWide is used for terminals wider than 120 characters.
	LayoutWide
)

// DetectLayout returns the appropriate LayoutSize for the given terminal width.
func DetectLayout(width int) LayoutSize {
	switch {
	case width < 60:
		return LayoutCompact
	case width <= 120:
		return LayoutNormal
	default:
		return LayoutWide
	}
}

// LayoutConfig holds responsive layout values that adapt to terminal width.
type LayoutConfig struct {
	// GaugeWidth is the character width for gauge bars.
	GaugeWidth int
	// TableMaxWidth is the maximum width for tables.
	TableMaxWidth int
	// ShowSparklines controls whether sparkline charts are rendered.
	ShowSparklines bool
	// ShowMiniGauges controls whether inline mini gauges appear in table cells.
	ShowMiniGauges bool
	// ContentPadding is the horizontal padding for content sections.
	ContentPadding int
}

// LayoutForSize returns a LayoutConfig appropriate for the given size and width.
func LayoutForSize(size LayoutSize, width int) LayoutConfig {
	switch size {
	case LayoutCompact:
		return LayoutConfig{
			GaugeWidth:     10,
			TableMaxWidth:  width - 4,
			ShowSparklines: false,
			ShowMiniGauges: false,
			ContentPadding: 1,
		}
	case LayoutWide:
		return LayoutConfig{
			GaugeWidth:     30,
			TableMaxWidth:  width - 12,
			ShowSparklines: true,
			ShowMiniGauges: true,
			ContentPadding: 3,
		}
	default: // LayoutNormal
		return LayoutConfig{
			GaugeWidth:     20,
			TableMaxWidth:  width - 8,
			ShowSparklines: true,
			ShowMiniGauges: false,
			ContentPadding: 2,
		}
	}
}

// truncateText is a convenience wrapper for format.TruncateWithEllipsis.
func truncateText(s string, maxWidth int) string {
	return format.TruncateWithEllipsis(s, maxWidth)
}

// formatRelativeTime is a convenience wrapper for format.FormatTimeSince.
func formatRelativeTime(t time.Time) string {
	return format.FormatTimeSince(t)
}

// formatDuration is a convenience wrapper for format.FormatDuration.
func formatDuration(d time.Duration) string {
	return format.FormatDuration(d)
}

// horizontalRule returns a horizontal line of the given width using box-drawing
// characters.
func horizontalRule(width int) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat("\u2500", width)
}

// sectionTitle renders a centered title with horizontal rules on either side.
// Format: "---- Title ----"
func sectionTitle(title string, width int) string {
	if width <= 0 {
		return title
	}

	titleLen := len([]rune(title))
	// 2 spaces around the title text.
	decorLen := titleLen + 2
	if decorLen >= width {
		return title
	}

	remaining := width - decorLen
	leftLen := remaining / 2
	rightLen := remaining - leftLen

	left := strings.Repeat("\u2500", leftLen)
	right := strings.Repeat("\u2500", rightLen)

	return left + " " + title + " " + right
}
