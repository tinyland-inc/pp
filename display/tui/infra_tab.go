package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/widgets"
)

// renderInfraContent renders the Infrastructure tab content.
// It displays Tailscale mesh status and Kubernetes cluster health using
// responsive layout based on the available width.
func renderInfraContent(data *collectors.InfraStatus, width, height int) string {
	if data == nil {
		return "No infrastructure data available"
	}

	layout := LayoutForSize(DetectLayout(width), width)
	var sections []string

	// Section title.
	title := sectionTitle("Infrastructure Status", layout.TableMaxWidth)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorSecondary)
	sections = append(sections, titleStyle.Render(title))
	sections = append(sections, "")

	// Tailscale section.
	if data.Tailscale != nil {
		ts := renderTailscaleSection(data.Tailscale, layout)
		sections = append(sections, ts)
	}

	// Separator between Tailscale and Kubernetes.
	if data.Tailscale != nil && len(data.Kubernetes) > 0 {
		sep := lipgloss.NewStyle().Foreground(colorMuted).Render(
			horizontalRule(layout.TableMaxWidth),
		)
		sections = append(sections, "", sep, "")
	}

	// Kubernetes sections.
	for i, cluster := range data.Kubernetes {
		k8s := renderK8sSection(&cluster, layout)
		sections = append(sections, k8s)
		// Separator between clusters.
		if i < len(data.Kubernetes)-1 {
			sep := lipgloss.NewStyle().Foreground(colorMuted).Render(
				horizontalRule(layout.TableMaxWidth),
			)
			sections = append(sections, "", sep, "")
		}
	}

	return strings.Join(sections, "\n")
}

// renderTailscaleSection renders the Tailscale mesh network status.
func renderTailscaleSection(ts *collectors.TailscaleStatus, layout LayoutConfig) string {
	var lines []string

	// Header: "Tailscale Mesh - tailnet (online/total)"
	headerText := fmt.Sprintf("Tailscale Mesh - %s", ts.Tailnet)
	countText := fmt.Sprintf("(%d/%d online)", ts.OnlineCount, ts.TotalCount)
	header := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render(headerText) +
		" " + lipgloss.NewStyle().Foreground(colorMuted).Render(countText)
	lines = append(lines, header)

	// Online percentage gauge.
	var onlinePercent float64
	if ts.TotalCount > 0 {
		onlinePercent = float64(ts.OnlineCount) / float64(ts.TotalCount) * 100
	}
	gauge := widgets.RenderGauge(widgets.GaugeConfig{
		Width:            layout.GaugeWidth,
		Percent:          onlinePercent,
		Label:            "Online",
		ShowPercent:      true,
		ThresholdWarning: 50,
		ThresholdDanger:  25,
	})
	lines = append(lines, gauge)
	lines = append(lines, "")

	// Node table.
	if len(ts.Nodes) > 0 {
		table := renderTailscaleTable(ts.Nodes, layout)
		lines = append(lines, table)
	}

	return strings.Join(lines, "\n")
}

// renderTailscaleTable renders the Tailscale node table.
func renderTailscaleTable(nodes []collectors.TailscaleNode, layout LayoutConfig) string {
	columns := []widgets.Column{
		{Title: "Name", Width: 0, Align: widgets.AlignLeft},
		{Title: "Hostname", Width: 0, Align: widgets.AlignLeft},
		{Title: "IP", Width: 15, Align: widgets.AlignLeft},
		{Title: "OS", Width: 8, Align: widgets.AlignLeft},
		{Title: "Status", Width: 6, Align: widgets.AlignCenter},
		{Title: "Last Seen", Width: 10, Align: widgets.AlignRight},
		{Title: "CPU", Width: 6, Align: widgets.AlignRight},
		{Title: "RAM", Width: 6, Align: widgets.AlignRight},
		{Title: "Disk", Width: 6, Align: widgets.AlignRight},
	}

	rows := make([][]string, 0, len(nodes))
	for _, node := range nodes {
		name := node.Name
		if node.DashboardURL != "" {
			name = collectors.Link(node.DashboardURL, node.Name)
		}

		var status string
		if node.Online {
			status = widgets.RenderStatus(widgets.StatusConfig{
				Level:    widgets.StatusOK,
				Text:     "",
				ShowIcon: true,
			})
		} else {
			status = widgets.RenderStatus(widgets.StatusConfig{
				Level:    widgets.StatusCritical,
				Text:     "",
				ShowIcon: true,
			})
		}

		lastSeen := formatRelativeTime(node.LastSeen)

		cpuStr := "-"
		if node.CPUPercent != nil {
			cpuStr = fmt.Sprintf("%.0f%%", *node.CPUPercent)
		}
		ramStr := "-"
		if node.RAMPercent != nil {
			ramStr = fmt.Sprintf("%.0f%%", *node.RAMPercent)
		}
		diskStr := "-"
		if node.DiskPercent != nil {
			diskStr = fmt.Sprintf("%.0f%%", *node.DiskPercent)
		}

		rows = append(rows, []string{
			name,
			node.Hostname,
			node.IP,
			node.OS,
			status,
			lastSeen,
			cpuStr,
			ramStr,
			diskStr,
		})
	}

	cfg := widgets.DefaultTableConfig()
	cfg.Columns = columns
	cfg.Rows = rows
	cfg.MaxWidth = layout.TableMaxWidth
	cfg.ShowHeader = true

	return widgets.RenderTable(cfg)
}

// renderK8sSection renders a single Kubernetes cluster section.
func renderK8sSection(cluster *collectors.KubernetesCluster, layout LayoutConfig) string {
	var lines []string

	// Header: "K8s: name (platform)" with status indicator.
	clusterName := cluster.Name
	if cluster.DashboardURL != "" {
		clusterName = collectors.Link(cluster.DashboardURL, cluster.Name)
	}

	headerText := fmt.Sprintf("K8s: %s (%s)", clusterName, cluster.Platform)
	statusIndicator := widgets.RenderStatusFromString(cluster.Status)
	header := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render(headerText) +
		" " + statusIndicator
	lines = append(lines, header)

	// Cluster health gauge: ReadyNodes/TotalNodes.
	var healthPercent float64
	if cluster.TotalNodes > 0 {
		healthPercent = float64(cluster.ReadyNodes) / float64(cluster.TotalNodes) * 100
	}
	gaugeLabel := fmt.Sprintf("Nodes %d/%d", cluster.ReadyNodes, cluster.TotalNodes)
	gauge := widgets.RenderGauge(widgets.GaugeConfig{
		Width:            layout.GaugeWidth,
		Percent:          healthPercent,
		Label:            gaugeLabel,
		ShowPercent:      true,
		ThresholdWarning: 70,
		ThresholdDanger:  50,
	})
	lines = append(lines, gauge)
	lines = append(lines, "")

	// Node table.
	if len(cluster.Nodes) > 0 {
		table := renderK8sNodeTable(cluster.Nodes, layout)
		lines = append(lines, table)
	}

	return strings.Join(lines, "\n")
}

// renderK8sNodeTable renders the Kubernetes node table.
func renderK8sNodeTable(nodes []collectors.KubernetesNode, layout LayoutConfig) string {
	columns := []widgets.Column{
		{Title: "Node", Width: 0, Align: widgets.AlignLeft},
		{Title: "Status", Width: 10, Align: widgets.AlignCenter},
		{Title: "CPU", Width: 0, Align: widgets.AlignRight},
		{Title: "Mem", Width: 0, Align: widgets.AlignRight},
		{Title: "Pods", Width: 8, Align: widgets.AlignRight},
	}

	// Adjust column widths for mini gauges.
	if layout.ShowMiniGauges {
		columns[2].Width = 12
		columns[3].Width = 12
	}

	rows := make([][]string, 0, len(nodes))
	for _, node := range nodes {
		status := widgets.RenderStatusFromString(node.Status)

		var cpuStr, memStr string
		if layout.ShowMiniGauges {
			cpuStr = widgets.RenderMiniGauge(node.CPUPercent, 8)
			memStr = widgets.RenderMiniGauge(node.MemPercent, 8)
		} else {
			cpuStr = fmt.Sprintf("%.0f%%", node.CPUPercent)
			memStr = fmt.Sprintf("%.0f%%", node.MemPercent)
		}

		pods := fmt.Sprintf("%d/%d", node.PodCount, node.MaxPods)

		rows = append(rows, []string{
			node.Name,
			status,
			cpuStr,
			memStr,
			pods,
		})
	}

	cfg := widgets.DefaultTableConfig()
	cfg.Columns = columns
	cfg.Rows = rows
	cfg.MaxWidth = layout.TableMaxWidth
	cfg.ShowHeader = true

	return widgets.RenderTable(cfg)
}
