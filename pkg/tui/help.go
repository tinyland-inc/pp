package tui

import (
	"strings"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/components"
)

// tuiHelpWidth is the fixed width of the help panel.
const tuiHelpWidth = 60

// tuiRenderHelp renders a centered help panel listing all keybindings.
// The panel is 60 characters wide and approximately 20 lines tall,
// centered within the given width and height.
func tuiRenderHelp(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	helpLines := []string{
		"",
		components.Bold("  Keybindings"),
		"",
		"  Tab / Shift+Tab     Cycle widget focus",
		"  h / l               Navigate left / right",
		"  j / k               Navigate down / up",
		"  Enter               Expand / collapse widget",
		"  Escape              Close overlay / collapse",
		"  ?                   Toggle this help",
		"  /                   Enter search mode",
		"  q                   Quit",
		"  Ctrl+C              Force quit",
		"",
		components.Bold("  Waifu (when focused)"),
		"",
		"  n / Right           Next image",
		"  p / Left            Previous image",
		"  r                   Random image",
		"  i                   Toggle info overlay",
		"",
		components.Bold("  Search Mode"),
		"",
		"  Type to filter      Matches widget ID and title",
		"  Enter               Confirm search filter",
		"  Escape              Cancel search",
		"",
	}

	helpContent := strings.Join(helpLines, "\n")

	panelW := tuiHelpWidth
	if panelW > width {
		panelW = width
	}

	panelH := len(helpLines) + 2 // +2 for top/bottom border
	if panelH > height {
		panelH = height
	}

	style := components.BoxStyle{
		Border:     components.BorderRounded,
		Title:      "Help",
		TitleAlign: components.AlignCenter,
		FG:         "#7C3AED",
	}

	panel := components.RenderBox(helpContent, panelW, panelH, style)

	// Center the panel within the available area.
	panelLines := strings.Split(panel, "\n")

	// Vertical centering: add empty lines above.
	topPad := (height - len(panelLines)) / 2
	if topPad < 0 {
		topPad = 0
	}

	// Horizontal centering: add spaces to the left of each line.
	leftPad := (width - panelW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	leftPadStr := strings.Repeat(" ", leftPad)

	var result strings.Builder

	// Top padding lines.
	emptyLine := strings.Repeat(" ", width)
	for i := 0; i < topPad; i++ {
		result.WriteString(emptyLine)
		result.WriteByte('\n')
	}

	// Panel lines with horizontal padding.
	for i, line := range panelLines {
		result.WriteString(leftPadStr)
		result.WriteString(line)
		if i < len(panelLines)-1 {
			result.WriteByte('\n')
		}
	}

	// Bottom padding to fill height.
	linesUsed := topPad + len(panelLines)
	for i := linesUsed; i < height; i++ {
		result.WriteByte('\n')
		result.WriteString(emptyLine)
	}

	return result.String()
}
