package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/widgets"
)

// renderSystemContent renders the System tab content.
// It displays system information collected via fastfetch and live system
// metrics (CPU, RAM, Disk) with sparkline history and gauges.
func renderSystemContent(fastfetch *collectors.FastfetchData, metrics *collectors.SysMetricsData, width, height int) string {
	hasFastfetch := fastfetch != nil && !fastfetch.IsEmpty()
	hasMetrics := metrics != nil

	if !hasFastfetch && !hasMetrics {
		return "No system data available\n\nEnsure the daemon is running and fastfetch is installed."
	}

	var sections []string

	// Fastfetch section (static system info).
	if hasFastfetch {
		sections = append(sections, renderFastfetchSection(fastfetch, width)...)
	}

	// Sysmetrics section (live metrics with sparklines).
	if hasMetrics {
		if hasFastfetch {
			// Separator between fastfetch and sysmetrics.
			separatorStyle := lipgloss.NewStyle().Foreground(colorMuted)
			sepWidth := width - 4
			if sepWidth < 10 {
				sepWidth = 10
			}
			sections = append(sections, "")
			sections = append(sections, separatorStyle.Render(strings.Repeat("\u2500", sepWidth)))
			sections = append(sections, "")
		}
		sections = append(sections, renderSysMetricsSection(metrics, width)...)
	}

	return strings.Join(sections, "\n")
}

// renderFastfetchSection renders the static system information from fastfetch.
func renderFastfetchSection(data *collectors.FastfetchData, width int) []string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorSecondary)
	labelStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	var sections []string

	sections = append(sections, titleStyle.Render("System Information"))
	sections = append(sections, "")

	// Render each fastfetch module as a labeled row.
	modules := []struct {
		label string
		mod   collectors.FastfetchModule
	}{
		{"OS", data.OS},
		{"Host", data.Host},
		{"Kernel", data.Kernel},
		{"Uptime", data.Uptime},
		{"CPU", data.CPU},
		{"GPU", data.GPU},
		{"Memory", data.Memory},
		{"Disk", data.Disk},
		{"Packages", data.Packages},
		{"Shell", data.Shell},
		{"Terminal", data.Terminal},
		{"Local IP", data.LocalIP},
	}

	for _, m := range modules {
		if m.mod.Type == "" || m.mod.Result == "" {
			continue
		}
		line := labelStyle.Render(m.label+":") + " " + valueStyle.Render(m.mod.Result)
		sections = append(sections, line)
	}

	// Optional modules section.
	optionalModules := []struct {
		label string
		mod   collectors.FastfetchModule
	}{
		{"Battery", data.Battery},
		{"WM", data.WM},
		{"Processes", data.Processes},
		{"Swap", data.Swap},
		{"Public IP", data.PublicIP},
	}

	var optLines []string
	for _, m := range optionalModules {
		if m.mod.Type == "" || m.mod.Result == "" {
			continue
		}
		optLines = append(optLines, labelStyle.Render(m.label+":") + " " + valueStyle.Render(m.mod.Result))
	}

	if len(optLines) > 0 {
		sections = append(sections, "")
		separatorStyle := lipgloss.NewStyle().Foreground(colorMuted)
		sepWidth := width - 4
		if sepWidth < 10 {
			sepWidth = 10
		}
		sections = append(sections, separatorStyle.Render(strings.Repeat("\u2500", sepWidth)))
		sections = append(sections, optLines...)
	}

	return sections
}

// metricColorForValue returns a color based on utilization thresholds:
// green < 70%, yellow < 90%, red >= 90%.
func metricColorForValue(value float64) lipgloss.Color {
	switch {
	case value >= 90:
		return lipgloss.Color("#EF4444") // red
	case value >= 70:
		return lipgloss.Color("#EAB308") // yellow
	default:
		return lipgloss.Color("#22C55E") // green
	}
}

// renderSysMetricsSection renders the live system metrics with sparklines and gauges.
func renderSysMetricsSection(data *collectors.SysMetricsData, width int) []string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorSecondary)
	labelStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Width(6)
	valueStyle := lipgloss.NewStyle().Bold(true).Width(7).Align(lipgloss.Right)
	mutedStyle := lipgloss.NewStyle().Foreground(colorMuted)

	var lines []string

	lines = append(lines, titleStyle.Render("System Metrics"))
	lines = append(lines, "")

	// Determine sparkline width: use available space minus label, value, gauge, and padding.
	// Layout: "  CPU   45.2%  [sparkline]  [gauge]"
	// Reserve: 2 indent + 6 label + 7 value + 2 spacing + 2 spacing + gauge(20) = ~39
	sparkWidth := width - 39
	if sparkWidth < 10 {
		sparkWidth = 10
	}
	if sparkWidth > 40 {
		sparkWidth = 40
	}

	gaugeWidth := 20
	if width < 80 {
		gaugeWidth = 12
	}

	// Each metric: CPU, RAM, Disk.
	metrics := []struct {
		label   string
		value   float64
		history []float64
	}{
		{"CPU", data.CPU, data.CPUHistory},
		{"RAM", data.RAM, data.RAMHistory},
		{"Disk", data.Disk, data.DiskHistory},
	}

	for _, m := range metrics {
		color := metricColorForValue(m.value)

		// Value text with color.
		valText := valueStyle.Foreground(color).Render(fmt.Sprintf("%.1f%%", m.value))

		// Sparkline from history.
		var sparkStr string
		if len(m.history) > 0 {
			sparkStr = widgets.RenderSparkline(widgets.SparklineConfig{
				Data:  m.history,
				Width: sparkWidth,
				Min:   0,
				Max:   100,
				Color: color,
			})
		} else {
			sparkStr = mutedStyle.Render(strings.Repeat("\u2500", sparkWidth))
		}

		// Mini gauge bar.
		gauge := widgets.RenderGauge(widgets.GaugeConfig{
			Width:            gaugeWidth,
			Percent:          m.value,
			ShowPercent:      false,
			ThresholdWarning: 70,
			ThresholdDanger:  90,
		})

		line := "  " + labelStyle.Render(m.label) + valText + "  " + sparkStr + "  " + gauge
		lines = append(lines, line)
	}

	// Load average section.
	lines = append(lines, "")
	loadLabel := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("Load:")
	loadValues := mutedStyle.Render(fmt.Sprintf("%.2f / %.2f / %.2f",
		data.LoadAvg1, data.LoadAvg5, data.LoadAvg15))
	lines = append(lines, "  "+loadLabel+" "+loadValues)

	return lines
}
