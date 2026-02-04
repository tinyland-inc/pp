package tui

import (
	"fmt"
	"strings"
	"time"
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

// truncateText truncates a string to maxWidth characters, appending "..."
// if the string exceeds the limit. If maxWidth is less than 4, the string
// is hard-truncated without an ellipsis suffix.
func truncateText(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}

	if maxWidth < 4 {
		return string(runes[:maxWidth])
	}

	return string(runes[:maxWidth-3]) + "..."
}

// formatRelativeTime formats a time.Time as a human-readable relative string.
// Examples: "just now", "30s ago", "5m ago", "2h ago", "3d ago".
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	d := time.Since(t)
	if d < 0 {
		d = -d
	}

	switch {
	case d < 10*time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

// formatDuration renders a time.Duration as a concise human-readable string.
// Examples: "1s", "5m 30s", "2h 15m", "3d 4h".
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	if d < time.Second {
		return "0s"
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
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
