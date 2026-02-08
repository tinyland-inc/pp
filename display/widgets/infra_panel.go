// Package widgets provides reusable display components for prompt-pulse.
// infra_panel.go implements tree-structured views for Kubernetes clusters
// and Tailscale mesh networks with node health indicators.
package widgets

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// Box drawing characters for tree structure.
const (
	treeVertical   = "│"
	treeBranch     = "├─"
	treeLastBranch = "└─"
	treeSpace      = "  "
)

// Status indicators.
const (
	statusOnline  = "🟢"
	statusWarning = "🟡"
	statusOffline = "🔴"
)

// InfraPanelConfig controls the appearance of the infrastructure panel.
type InfraPanelConfig struct {
	// Width is the total panel width in characters.
	Width int
	// ShowMiniGauges enables mini progress bars for node metrics.
	ShowMiniGauges bool
	// GaugeWidth is the width of mini gauges (default: 8).
	GaugeWidth int
	// ColorEnabled enables ANSI color output.
	ColorEnabled bool
	// MaxNodes limits displayed nodes per section (0 = unlimited).
	MaxNodes int
}

// DefaultInfraPanelConfig returns sensible defaults for the panel.
func DefaultInfraPanelConfig() InfraPanelConfig {
	return InfraPanelConfig{
		Width:          60,
		ShowMiniGauges: true,
		GaugeWidth:     8,
		ColorEnabled:   true,
		MaxNodes:       0,
	}
}

// InfraPanel renders tree-structured infrastructure views.
type InfraPanel struct {
	config InfraPanelConfig
}

// NewInfraPanel creates an InfraPanel with the given configuration.
func NewInfraPanel(cfg InfraPanelConfig) *InfraPanel {
	if cfg.GaugeWidth <= 0 {
		cfg.GaugeWidth = 8
	}
	if cfg.Width <= 0 {
		cfg.Width = 60
	}
	return &InfraPanel{config: cfg}
}

// Render generates the complete infrastructure panel output.
func (p *InfraPanel) Render(data *collectors.InfraStatus) string {
	if data == nil {
		return p.renderEmptyState()
	}

	var sections []string

	// Render Tailscale mesh section.
	if data.Tailscale != nil {
		sections = append(sections, p.renderTailscaleSection(data.Tailscale))
	}

	// Render Kubernetes clusters section.
	if len(data.Kubernetes) > 0 {
		sections = append(sections, p.renderKubernetesSection(data.Kubernetes))
	}

	if len(sections) == 0 {
		return p.renderEmptyState()
	}

	return strings.Join(sections, "\n")
}

// renderEmptyState returns a placeholder when no data is available.
func (p *InfraPanel) renderEmptyState() string {
	return p.styledMuted("(no infrastructure data)")
}

// renderTailscaleSection generates the Tailscale mesh tree view.
func (p *InfraPanel) renderTailscaleSection(ts *collectors.TailscaleStatus) string {
	var lines []string

	// Section header with summary.
	headerStyle := lipgloss.NewStyle()
	if p.config.ColorEnabled {
		headerStyle = headerStyle.Bold(true).Foreground(lipgloss.Color("#06B6D4"))
	}

	// Determine overall mesh status.
	meshStatus := statusOnline
	if ts.OnlineCount == 0 {
		meshStatus = statusOffline
	} else if ts.OnlineCount < ts.TotalCount {
		meshStatus = statusWarning
	}

	header := fmt.Sprintf("%s Tailscale Mesh (%s)", meshStatus, p.truncateTailnet(ts.Tailnet))
	if p.config.ColorEnabled {
		header = headerStyle.Render(header)
	}
	lines = append(lines, header)

	// Online/total summary.
	summary := fmt.Sprintf("  %d/%d nodes online", ts.OnlineCount, ts.TotalCount)
	lines = append(lines, p.styledMuted(summary))

	// Render individual nodes.
	nodes := ts.Nodes
	if p.config.MaxNodes > 0 && len(nodes) > p.config.MaxNodes {
		nodes = nodes[:p.config.MaxNodes]
	}

	for i, node := range nodes {
		isLast := i == len(nodes)-1
		nodeLine := p.renderTailscaleNode(node, isLast)
		lines = append(lines, nodeLine)
	}

	// Show truncation indicator if needed.
	if p.config.MaxNodes > 0 && len(ts.Nodes) > p.config.MaxNodes {
		remaining := len(ts.Nodes) - p.config.MaxNodes
		lines = append(lines, p.styledMuted(fmt.Sprintf("  ... +%d more nodes", remaining)))
	}

	return strings.Join(lines, "\n")
}

// renderTailscaleNode renders a single Tailscale node with tree structure.
func (p *InfraPanel) renderTailscaleNode(node collectors.TailscaleNode, isLast bool) string {
	// Select tree branch character.
	branch := treeBranch
	if isLast {
		branch = treeLastBranch
	}

	// Determine status indicator.
	status := statusOnline
	if !node.Online {
		status = statusOffline
	} else if node.HasHighUtilization() {
		status = statusWarning
	}

	// Build node line with hostname and IP.
	hostname := p.truncateString(node.Hostname, 15)
	ip := node.IP
	if ip == "" {
		ip = "no IP"
	}

	var metricsStr string
	if node.Online && p.config.ShowMiniGauges {
		metricsStr = p.renderNodeMetrics(node)
	} else if !node.Online {
		metricsStr = p.styledDanger("OFFLINE")
	}

	// Compose the node line.
	if metricsStr != "" {
		return fmt.Sprintf("  %s %s %-15s %-17s %s", branch, status, hostname, ip, metricsStr)
	}
	return fmt.Sprintf("  %s %s %-15s %s", branch, status, hostname, ip)
}

// renderNodeMetrics renders mini gauges for CPU, RAM, and disk.
func (p *InfraPanel) renderNodeMetrics(node collectors.TailscaleNode) string {
	var parts []string

	if node.CPUPercent != nil {
		cpuGauge := p.renderCompactGauge("CPU", *node.CPUPercent)
		parts = append(parts, cpuGauge)
	}
	if node.RAMPercent != nil {
		ramGauge := p.renderCompactGauge("RAM", *node.RAMPercent)
		parts = append(parts, ramGauge)
	}
	if node.DiskPercent != nil {
		diskGauge := p.renderCompactGauge("Disk", *node.DiskPercent)
		parts = append(parts, diskGauge)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// renderCompactGauge renders a compact labeled gauge.
func (p *InfraPanel) renderCompactGauge(label string, percent float64) string {
	// Use mini gauge from gauge.go.
	bar := RenderMiniGauge(percent, p.config.GaugeWidth)
	return fmt.Sprintf("%s %s", label, bar)
}

// renderKubernetesSection generates the Kubernetes clusters tree view.
func (p *InfraPanel) renderKubernetesSection(clusters []collectors.KubernetesCluster) string {
	var lines []string

	// Section header.
	headerStyle := lipgloss.NewStyle()
	if p.config.ColorEnabled {
		headerStyle = headerStyle.Bold(true).Foreground(lipgloss.Color("#06B6D4"))
	}

	header := "Kubernetes Clusters"
	if p.config.ColorEnabled {
		header = headerStyle.Render(header)
	}
	lines = append(lines, header)

	// Render each cluster.
	for i, cluster := range clusters {
		isLastCluster := i == len(clusters)-1
		clusterLines := p.renderKubernetesCluster(cluster, isLastCluster)
		lines = append(lines, clusterLines...)
	}

	return strings.Join(lines, "\n")
}

// renderKubernetesCluster renders a single Kubernetes cluster with its nodes.
func (p *InfraPanel) renderKubernetesCluster(cluster collectors.KubernetesCluster, isLastCluster bool) []string {
	var lines []string

	// Cluster header with status.
	clusterStatus := p.k8sStatusIndicator(cluster.Status)
	readyStr := fmt.Sprintf("%d/%d nodes ready", cluster.ReadyNodes, cluster.TotalNodes)

	// Platform suffix if available.
	platformStr := ""
	if cluster.Platform != "" {
		platformStr = fmt.Sprintf(" (%s)", cluster.Platform)
	}

	clusterLine := fmt.Sprintf("  %s %s%s - %s", clusterStatus, cluster.Name, platformStr, readyStr)
	lines = append(lines, clusterLine)

	// Render nodes.
	nodes := cluster.Nodes
	if p.config.MaxNodes > 0 && len(nodes) > p.config.MaxNodes {
		nodes = nodes[:p.config.MaxNodes]
	}

	for i, node := range nodes {
		isLastNode := i == len(nodes)-1
		nodeLine := p.renderKubernetesNode(node, isLastNode, isLastCluster)
		lines = append(lines, nodeLine)
	}

	// Truncation indicator.
	if p.config.MaxNodes > 0 && len(cluster.Nodes) > p.config.MaxNodes {
		remaining := len(cluster.Nodes) - p.config.MaxNodes
		lines = append(lines, p.styledMuted(fmt.Sprintf("    ... +%d more nodes", remaining)))
	}

	return lines
}

// renderKubernetesNode renders a single Kubernetes node with metrics.
func (p *InfraPanel) renderKubernetesNode(node collectors.KubernetesNode, isLastNode, isLastCluster bool) string {
	// Select tree branch character.
	branch := treeBranch
	if isLastNode {
		branch = treeLastBranch
	}

	// Determine status indicator.
	status := p.k8sNodeStatusIndicator(node.Status, node.CPUPercent, node.MemPercent)

	// Truncate node name if too long.
	nodeName := p.truncateString(node.Name, 20)

	// Build metrics string.
	var metricsStr string
	if node.Status == "Ready" && p.config.ShowMiniGauges {
		cpuGauge := p.renderCompactGauge("CPU", node.CPUPercent)
		memGauge := p.renderCompactGauge("RAM", node.MemPercent)
		metricsStr = fmt.Sprintf("%s %s", cpuGauge, memGauge)
	} else if node.Status != "Ready" {
		metricsStr = p.styledDanger(node.Status)
	}

	// Compose the node line with indent.
	if metricsStr != "" {
		return fmt.Sprintf("    %s %s %-20s %s", branch, status, nodeName, metricsStr)
	}
	return fmt.Sprintf("    %s %s %s", branch, status, nodeName)
}

// k8sStatusIndicator returns a status emoji for cluster health.
func (p *InfraPanel) k8sStatusIndicator(status string) string {
	switch status {
	case "healthy":
		return statusOnline
	case "degraded":
		return statusWarning
	case "offline":
		return statusOffline
	default:
		return statusWarning
	}
}

// k8sNodeStatusIndicator returns a status emoji based on node state and metrics.
func (p *InfraPanel) k8sNodeStatusIndicator(status string, cpuPercent, memPercent float64) string {
	if status != "Ready" {
		return statusOffline
	}
	// Check for high utilization.
	if cpuPercent >= 80 || memPercent >= 80 {
		return statusWarning
	}
	return statusOnline
}

// truncateTailnet shortens a tailnet name for display.
func (p *InfraPanel) truncateTailnet(tailnet string) string {
	// Remove common suffixes.
	tailnet = strings.TrimSuffix(tailnet, ".ts.net")
	return p.truncateString(tailnet, 20)
}

// truncateString truncates a string to maxLen, adding "..." if needed.
func (p *InfraPanel) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// styledMuted returns a gray-styled string.
func (p *InfraPanel) styledMuted(s string) string {
	if p.config.ColorEnabled {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render(s)
	}
	return s
}

// styledDanger returns a red-styled string.
func (p *InfraPanel) styledDanger(s string) string {
	if p.config.ColorEnabled {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render(s)
	}
	return s
}

// RenderInfraBox renders the infrastructure panel wrapped in a box.
func RenderInfraBox(data *collectors.InfraStatus, width int) string {
	panel := NewInfraPanel(InfraPanelConfig{
		Width:          width - 4, // Account for box borders.
		ShowMiniGauges: true,
		GaugeWidth:     6,
		ColorEnabled:   true,
		MaxNodes:       6,
	})

	content := panel.Render(data)

	// Build box using lipgloss.
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(0, 1).
		Width(width)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	title := titleStyle.Render(" Infrastructure ")

	// Compose box with title.
	box := boxStyle.Render(content)

	// Insert title into top border.
	lines := strings.Split(box, "\n")
	if len(lines) > 0 {
		// Find position to insert title (after first border character).
		firstLine := lines[0]
		if len(firstLine) > 4 {
			// Replace part of top border with title.
			titlePos := 2
			titleLen := lipgloss.Width(title)
			if titlePos+titleLen < len(firstLine) {
				// Carefully insert title into border.
				runes := []rune(firstLine)
				titleRunes := []rune(title)
				for i, r := range titleRunes {
					if titlePos+i < len(runes) {
						runes[titlePos+i] = r
					}
				}
				lines[0] = string(runes)
			}
		}
	}

	return strings.Join(lines, "\n")
}
