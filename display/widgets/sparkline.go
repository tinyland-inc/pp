package widgets

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// sparkBlocks contains 8 unicode block characters for sparkline rendering,
// ordered from lowest to highest.
var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// SparklineConfig controls the appearance and behavior of a sparkline chart.
type SparklineConfig struct {
	// Data points to render (most recent last).
	Data []float64
	// Width is the number of characters to render. If 0, uses len(Data).
	Width int
	// Min is the minimum value for scaling. If Min == Max, auto-scale.
	Min float64
	// Max is the maximum value for scaling.
	Max float64
	// Label is optional text shown before the sparkline.
	Label string
	// Color is the lipgloss color for the sparkline characters.
	Color lipgloss.Color
}

// RenderSparkline renders a unicode sparkline chart from the given configuration.
func RenderSparkline(cfg SparklineConfig) string {
	if len(cfg.Data) == 0 {
		return ""
	}

	data := cfg.Data

	// Determine effective width.
	width := cfg.Width
	if width <= 0 {
		width = len(data)
	}

	// Truncate to last Width points if needed.
	if width < len(data) {
		data = data[len(data)-width:]
	}

	// Auto-scale if Min == Max.
	minVal := cfg.Min
	maxVal := cfg.Max
	if minVal == maxVal {
		minVal = data[0]
		maxVal = data[0]
		for _, v := range data {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
	}

	// Build sparkline characters.
	var runes []rune
	allEqual := minVal == maxVal

	for _, v := range data {
		if allEqual {
			// All values equal: use mid-level block.
			runes = append(runes, sparkBlocks[len(sparkBlocks)/2])
			continue
		}
		// Normalize to 0-1 range, clamped.
		normalized := (v - minVal) / (maxVal - minVal)
		normalized = math.Max(0, math.Min(1, normalized))
		// Map to block index (0 to 7).
		idx := int(normalized * float64(len(sparkBlocks)-1))
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		runes = append(runes, sparkBlocks[idx])
	}

	// Left-pad with spaces if Width > len(data).
	sparkStr := string(runes)
	if width > len(data) {
		padding := strings.Repeat(" ", width-len(data))
		sparkStr = padding + sparkStr
	}

	// Apply color if set.
	if cfg.Color != "" {
		style := lipgloss.NewStyle().Foreground(cfg.Color)
		sparkStr = style.Render(sparkStr)
	}

	// Prepend label if set.
	if cfg.Label != "" {
		sparkStr = cfg.Label + " " + sparkStr
	}

	return sparkStr
}

// RenderSparklineWithRange renders a sparkline with auto-scaling and min/max labels.
// Format: min▁▂▃▄▅▆▇█max
func RenderSparklineWithRange(data []float64, width int) string {
	if len(data) == 0 {
		return ""
	}

	// Find min/max.
	minVal := data[0]
	maxVal := data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	sparkline := RenderSparkline(SparklineConfig{
		Data:  data,
		Width: width,
	})

	return fmt.Sprintf("%.0f%s%.0f", minVal, sparkline, maxVal)
}
