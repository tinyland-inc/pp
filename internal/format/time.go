// Package format provides shared string and time formatting utilities.
package format

import (
	"fmt"
	"time"
)

// FormatTimeUntil formats a time.Time as a human-readable duration until that time.
// Returns strings like "2h 15m", "3d 12h", "45m", or "now" if the time has passed.
func FormatTimeUntil(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	d := time.Until(t)
	if d <= 0 {
		return "now"
	}

	return FormatDuration(d)
}

// FormatTimeSince formats a time.Time as a human-readable duration since that time.
// Returns strings like "2h 15m ago", "3d 12h ago", "45m ago", or "just now".
func FormatTimeSince(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	d := time.Since(t)
	if d < 0 {
		d = -d
	}

	if d < 10*time.Second {
		return "just now"
	}

	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}

	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}

	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}

	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd ago", days)
}

// FormatDuration renders a time.Duration as a concise human-readable string.
// Returns strings like "1s", "5m 30s", "2h 15m", "3d 4h".
func FormatDuration(d time.Duration) string {
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

// FormatCompactReset returns ultra-compact time formatting (2h, 45m, 3d).
// Used for starship output where space is at a premium.
func FormatCompactReset(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	d := time.Until(t)
	if d <= 0 {
		return "now"
	}

	hours := int(d.Hours())
	if hours >= 24 {
		return fmt.Sprintf("%dd", hours/24)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// FormatRelativeTime formats a time.Time as a human-readable relative duration.
// Returns strings like "2h 15m", "3d 12h", "45m", or "now" if already past.
func FormatRelativeTime(t time.Time) string {
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
