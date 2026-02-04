package banner

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// BoxStyle defines Unicode box-drawing characters.
type BoxStyle struct {
	TopLeft, TopRight, BottomLeft, BottomRight rune
	Horizontal, Vertical                       rune
}

// RoundedBox uses rounded corner box-drawing characters.
var RoundedBox = BoxStyle{
	TopLeft: '╭', TopRight: '╮', BottomLeft: '╰', BottomRight: '╯',
	Horizontal: '─', Vertical: '│',
}

// SharpBox uses sharp corner box-drawing characters.
var SharpBox = BoxStyle{
	TopLeft: '┌', TopRight: '┐', BottomLeft: '└', BottomRight: '┘',
	Horizontal: '─', Vertical: '│',
}

// RenderBox wraps content lines in a Unicode box.
func RenderBox(lines []string, width int, title string, style BoxStyle, titleColor lipgloss.Color) string {
	if width < 4 {
		width = 80
	}
	innerWidth := width - 2

	var result strings.Builder

	// Top border with optional title
	result.WriteRune(style.TopLeft)
	if title != "" {
		titleStyled := lipgloss.NewStyle().Foreground(titleColor).Bold(true).Render(title)
		titleLen := len(title) + 2 // space padding
		result.WriteString(strings.Repeat(string(style.Horizontal), 1))
		result.WriteString(" ")
		result.WriteString(titleStyled)
		result.WriteString(" ")
		remaining := innerWidth - titleLen - 2
		if remaining > 0 {
			result.WriteString(strings.Repeat(string(style.Horizontal), remaining))
		}
	} else {
		result.WriteString(strings.Repeat(string(style.Horizontal), innerWidth))
	}
	result.WriteRune(style.TopRight)
	result.WriteString("\n")

	// Content lines
	for _, line := range lines {
		result.WriteRune(style.Vertical)
		result.WriteString(" ")
		paddedLine := padOrTruncate(line, innerWidth-2)
		result.WriteString(paddedLine)
		result.WriteString(" ")
		result.WriteRune(style.Vertical)
		result.WriteString("\n")
	}

	// Bottom border
	result.WriteRune(style.BottomLeft)
	result.WriteString(strings.Repeat(string(style.Horizontal), innerWidth))
	result.WriteRune(style.BottomRight)

	return result.String()
}

// padOrTruncate pads or truncates a string to exactly the given width.
// It accounts for visible character width, not byte length, by using visibleLen.
func padOrTruncate(s string, width int) string {
	visible := visibleLen(s)
	if visible >= width {
		// Truncate to width (simple byte-based truncation for now;
		// for ANSI-aware truncation, additional logic would be needed)
		return truncateToWidth(s, width)
	}
	return s + strings.Repeat(" ", width-visible)
}

// truncateToWidth truncates a string to at most width visible characters.
// It preserves ANSI escape sequences but counts only visible characters.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}

	var result strings.Builder
	visibleCount := 0
	inEscape := false

	for _, r := range s {
		if inEscape {
			result.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '~' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}
		if visibleCount >= width {
			break
		}
		result.WriteRune(r)
		visibleCount++
	}

	return result.String()
}
