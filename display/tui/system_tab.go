package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// renderSystemContent renders the System tab content.
// It displays system information collected via fastfetch.
func renderSystemContent(data *collectors.FastfetchData, width, height int) string {
	if data == nil || data.IsEmpty() {
		return "No system data available\n\nEnsure the daemon is running and fastfetch is installed."
	}

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

	return strings.Join(sections, "\n")
}
