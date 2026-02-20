package widgets

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/app"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/claudepersonal"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/components"
)

// ClaudePersonalWidget displays Claude personal plan usage tracking with a
// gauge showing messages used in the current rolling window.
type ClaudePersonalWidget struct {
	report *claudepersonal.Report
}

// NewClaudePersonalWidget creates a new ClaudePersonalWidget.
func NewClaudePersonalWidget() *ClaudePersonalWidget {
	return &ClaudePersonalWidget{}
}

// ID returns the unique identifier for this widget.
func (w *ClaudePersonalWidget) ID() string { return "claudepersonal" }

// Title returns the display name for this widget.
func (w *ClaudePersonalWidget) Title() string {
	if w.report == nil {
		return "Claude Pro"
	}
	return fmt.Sprintf("Claude Pro [%d/%d]", w.report.MessagesInWindow, w.report.MessageLimit)
}

// MinSize returns the minimum dimensions.
func (w *ClaudePersonalWidget) MinSize() (int, int) { return 25, 3 }

// Update handles data update events from the collector.
func (w *ClaudePersonalWidget) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case app.DataUpdateEvent:
		if msg.Source != "claudepersonal" {
			return nil
		}
		if msg.Err != nil {
			return nil
		}
		report, ok := msg.Data.(*claudepersonal.Report)
		if !ok {
			return nil
		}
		w.report = report
	}
	return nil
}

// HandleKey processes key events when focused. Currently no widget-specific
// keybindings.
func (w *ClaudePersonalWidget) HandleKey(_ tea.KeyMsg) tea.Cmd {
	return nil
}

// View renders the widget content.
func (w *ClaudePersonalWidget) View(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	if w.report == nil {
		return claudePersonalNoData(width, height)
	}

	lines := make([]string, 0, height)

	// Gauge: [||||||----] 23/45
	gaugeWidth := width - 12
	if gaugeWidth < 5 {
		gaugeWidth = 5
	}
	gauge := components.NewGauge(components.GaugeStyle{
		Width:             gaugeWidth,
		ShowPercent:       false,
		ShowValue:         true,
		Label:             "Usage",
		LabelWidth:        6,
		FilledColor:       claudePersonalGaugeColor(w.report),
		EmptyColor:        "#333333",
		WarningThreshold:  0.70,
		CriticalThreshold: 0.90,
		WarningColor:      "#FF9800",
		CriticalColor:     "#F44336",
	})
	gaugeLine := gauge.Render(
		float64(w.report.MessagesInWindow),
		float64(w.report.MessageLimit),
		gaugeWidth,
	)
	lines = append(lines, gaugeLine)

	// Status line.
	remaining := w.report.MessageLimit - w.report.MessagesInWindow
	if remaining < 0 {
		remaining = 0
	}
	statusLine := fmt.Sprintf("%d remaining in %dh window", remaining, w.report.WindowHours)
	if w.report.NextSlot > 0 {
		mins := int(w.report.NextSlot.Minutes())
		hours := mins / 60
		mins = mins % 60
		if hours > 0 {
			statusLine += fmt.Sprintf("  Reset: %dh%02dm", hours, mins)
		} else {
			statusLine += fmt.Sprintf("  Reset: %dm", mins)
		}
	}
	if len(statusLine) > width {
		statusLine = statusLine[:width]
	}
	lines = append(lines, statusLine)

	// Fill remaining height.
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// claudePersonalNoData renders a placeholder when no data is available.
func claudePersonalNoData(width, height int) string {
	msg := "Scanning sessions..."
	lines := make([]string, 0, height)
	topPad := (height - 1) / 2
	for i := 0; i < topPad; i++ {
		lines = append(lines, "")
	}
	if len(msg) > width {
		msg = msg[:width]
	}
	lines = append(lines, msg)
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// claudePersonalGaugeColor returns the gauge fill color based on usage ratio.
func claudePersonalGaugeColor(report *claudepersonal.Report) string {
	if report.MessageLimit <= 0 {
		return "#4CAF50"
	}
	ratio := float64(report.MessagesInWindow) / float64(report.MessageLimit)
	switch {
	case ratio >= 0.90:
		return "#F44336" // red
	case ratio >= 0.70:
		return "#FF9800" // yellow
	default:
		return "#4CAF50" // green
	}
}
