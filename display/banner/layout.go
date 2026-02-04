package banner

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// Color palette matching the TUI theme (display/tui/theme.go).
var (
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple - headers
	colorSecondary = lipgloss.Color("#06B6D4") // Cyan - section titles
	colorSuccess   = lipgloss.Color("#22C55E") // Green - healthy status
	colorWarning   = lipgloss.Color("#EAB308") // Yellow - warning status
	colorDanger    = lipgloss.Color("#EF4444") // Red - critical status
	colorMuted     = lipgloss.Color("#6B7280") // Gray - separators
)

// LayoutConfig controls banner layout behavior.
type LayoutConfig struct {
	// TermWidth is the terminal width in columns (default: 80).
	TermWidth int
	// TermHeight is the terminal height in rows (default: 24).
	TermHeight int
	// ImageCols is the width of the image column (default: 22).
	ImageCols int
	// ShowImage enables the image column (when false, only shows info).
	ShowImage bool
	// Hostname to display in the header.
	Hostname string
	// ColorEnabled enables ANSI colors.
	ColorEnabled bool
}

// DefaultLayoutConfig returns sensible defaults targeting 80x24.
func DefaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		TermWidth:    80,
		TermHeight:   24,
		ImageCols:    22,
		ShowImage:    true,
		Hostname:     "",
		ColorEnabled: true,
	}
}

// InfoData holds pre-formatted data for the info panel.
type InfoData struct {
	Claude  *collectors.ClaudeUsage
	Billing *collectors.BillingData
	Infra   *collectors.InfraStatus
	// StatusLevel is the evaluated system status ("healthy", "warning", "critical", "unknown").
	StatusLevel string
	// Uptime is the system uptime string (optional).
	Uptime string
}

// Layout composes the banner from image content and system info.
type Layout struct {
	config LayoutConfig
}

// NewLayout creates a new Layout with the given configuration.
func NewLayout(cfg LayoutConfig) *Layout {
	return &Layout{config: cfg}
}

// Render composes the complete banner output. imageContent is the pre-rendered
// image string (may be empty if ShowImage is false or no image available).
// Returns the composed banner string ready for terminal output.
func (l *Layout) Render(imageContent string, data InfoData) string {
	infoPanel := l.renderInfoPanel(data)

	showImage := l.config.ShowImage && imageContent != ""
	if showImage {
		return l.composeSideBySide(imageContent, infoPanel)
	}
	// No image: truncate info panel to TermHeight rows.
	lines := strings.Split(infoPanel, "\n")
	if len(lines) > l.config.TermHeight {
		lines = lines[:l.config.TermHeight]
	}
	return strings.Join(lines, "\n")
}

// renderInfoPanel builds the right-side info panel.
func (l *Layout) renderInfoPanel(data InfoData) string {
	var lines []string

	hostname := l.config.Hostname
	if hostname == "" {
		hostname = "localhost"
	}
	lines = append(lines, l.renderHeader(hostname, data.StatusLevel))
	lines = append(lines, l.separator())
	lines = append(lines, "")

	// Claude section.
	lines = append(lines, l.sectionTitle("Claude"))
	lines = append(lines, l.renderClaudeSummary(data.Claude)...)
	lines = append(lines, "")

	// Billing section.
	lines = append(lines, l.sectionTitle("Billing"))
	lines = append(lines, l.renderBillingSummary(data.Billing)...)
	lines = append(lines, "")

	// Infrastructure section.
	lines = append(lines, l.sectionTitle("Infrastructure"))
	lines = append(lines, l.renderInfraSummary(data.Infra)...)

	if data.Uptime != "" {
		lines = append(lines, "")
		lines = append(lines, l.styledMuted(fmt.Sprintf("  uptime: %s", data.Uptime)))
	}

	return strings.Join(lines, "\n")
}

// renderHeader returns the top header line with hostname and status.
func (l *Layout) renderHeader(hostname, statusLevel string) string {
	statusText := statusLevel
	if statusText == "" {
		statusText = "unknown"
	}

	if !l.config.ColorEnabled {
		return fmt.Sprintf("%s :: %s", hostname, statusText)
	}

	hostnameStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		Render(hostname)

	statusStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(l.statusColor(statusLevel)).
		Render(statusText)

	return fmt.Sprintf("%s :: %s", hostnameStyled, statusStyled)
}

// renderClaudeSummary returns a compact Claude usage summary.
// Format per account: "  name: XX% (5h) | XX% (7d)" or "  name: ERR"
func (l *Layout) renderClaudeSummary(data *collectors.ClaudeUsage) []string {
	if data == nil || len(data.Accounts) == 0 {
		return []string{l.styledMuted("  (no data)")}
	}

	var lines []string
	for _, acct := range data.Accounts {
		if acct.Status != "ok" {
			lines = append(lines, fmt.Sprintf("  %s: ERR", acct.Name))
			continue
		}

		switch acct.Type {
		case "subscription":
			line := l.formatSubscriptionAccount(acct)
			lines = append(lines, line)
		case "api":
			line := l.formatAPIAccount(acct)
			lines = append(lines, line)
		default:
			lines = append(lines, fmt.Sprintf("  %s: ERR", acct.Name))
		}
	}

	return lines
}

// formatSubscriptionAccount formats a subscription Claude account with reset times.
func (l *Layout) formatSubscriptionAccount(acct collectors.ClaudeAccountUsage) string {
	var parts []string
	if acct.FiveHour != nil {
		part := fmt.Sprintf("%.0f%% (5h", acct.FiveHour.Utilization)
		if !acct.FiveHour.ResetsAt.IsZero() {
			resetStr := l.formatRelativeTime(acct.FiveHour.ResetsAt)
			if resetStr != "" {
				part += ", " + resetStr
			}
		}
		part += ")"

		// Add warning indicator for high utilization.
		if acct.FiveHour.Utilization >= 80 {
			if l.config.ColorEnabled {
				warningIcon := lipgloss.NewStyle().Foreground(colorWarning).Render("⚠️")
				part += " " + warningIcon
			} else {
				part += " ⚠️"
			}
		}
		parts = append(parts, part)
	}
	if acct.SevenDay != nil {
		part := fmt.Sprintf("%.0f%% (7d", acct.SevenDay.Utilization)
		if !acct.SevenDay.ResetsAt.IsZero() {
			resetStr := l.formatRelativeTime(acct.SevenDay.ResetsAt)
			if resetStr != "" {
				part += ", " + resetStr
			}
		}
		part += ")"

		// Add warning indicator for high utilization.
		if acct.SevenDay.Utilization >= 80 {
			if l.config.ColorEnabled {
				warningIcon := lipgloss.NewStyle().Foreground(colorWarning).Render("⚠️")
				part += " " + warningIcon
			} else {
				part += " ⚠️"
			}
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("  %s: 0%% (5h)", acct.Name)
	}
	return fmt.Sprintf("  %s: %s", acct.Name, strings.Join(parts, " | "))
}

// formatRelativeTime formats a time.Time as a human-readable relative duration.
// Returns strings like "2h 15m", "3d 12h", "45m", or "" if already past.
func (l *Layout) formatRelativeTime(t time.Time) string {
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

// formatAPIAccount formats an API Claude account.
func (l *Layout) formatAPIAccount(acct collectors.ClaudeAccountUsage) string {
	if acct.RateLimits == nil {
		return fmt.Sprintf("  %s: 0/0 req", acct.Name)
	}
	used := acct.RateLimits.RequestsLimit - acct.RateLimits.RequestsRemaining
	return fmt.Sprintf("  %s: %d/%d req", acct.Name, used, acct.RateLimits.RequestsLimit)
}

// renderBillingSummary returns a compact billing summary.
// Format: "  $XX.XX / $YY.YY budget (ZZ% forecast)"
func (l *Layout) renderBillingSummary(data *collectors.BillingData) []string {
	if data == nil {
		return []string{l.styledMuted("  (no data)")}
	}

	line := fmt.Sprintf("  $%.0f this month", data.Total.CurrentMonthUSD)

	if data.Total.ForecastUSD != nil {
		line += fmt.Sprintf(" ($%.0f forecast)", *data.Total.ForecastUSD)
	}

	if data.Total.BudgetUSD != nil && data.Total.CurrentMonthUSD > *data.Total.BudgetUSD {
		if l.config.ColorEnabled {
			overBudget := lipgloss.NewStyle().
				Bold(true).
				Foreground(colorDanger).
				Render("OVER BUDGET")
			line += " " + overBudget
		} else {
			line += " OVER BUDGET"
		}
	}

	return []string{line}
}

// renderInfraSummary returns compact infrastructure status.
// Format: "  ts: X/Y online" and "  k8s: cluster (status)"
// Also shows per-node metrics if available.
func (l *Layout) renderInfraSummary(data *collectors.InfraStatus) []string {
	if data == nil {
		return []string{l.styledMuted("  (no data)")}
	}

	var lines []string

	if data.Tailscale != nil {
		lines = append(lines, fmt.Sprintf("  ts: %d/%d online",
			data.Tailscale.OnlineCount, data.Tailscale.TotalCount))

		// Show per-node metrics for nodes that have them.
		for _, node := range data.Tailscale.Nodes {
			if !node.Online {
				continue
			}
			if node.CPUPercent == nil && node.RAMPercent == nil && node.DiskPercent == nil {
				continue
			}

			// Format node metrics.
			var metrics []string
			if node.CPUPercent != nil {
				cpuStr := fmt.Sprintf("CPU %.0f%%", *node.CPUPercent)
				if l.config.ColorEnabled && *node.CPUPercent >= 80 {
					cpuStr = lipgloss.NewStyle().Foreground(colorWarning).Render(cpuStr)
				}
				metrics = append(metrics, cpuStr)
			}
			if node.RAMPercent != nil {
				ramStr := fmt.Sprintf("RAM %.0f%%", *node.RAMPercent)
				if l.config.ColorEnabled && *node.RAMPercent >= 80 {
					ramStr = lipgloss.NewStyle().Foreground(colorWarning).Render(ramStr)
				}
				metrics = append(metrics, ramStr)
			}
			if node.DiskPercent != nil {
				diskStr := fmt.Sprintf("Disk %.0f%%", *node.DiskPercent)
				if l.config.ColorEnabled && *node.DiskPercent >= 80 {
					diskStr = lipgloss.NewStyle().Foreground(colorWarning).Render(diskStr)
				}
				metrics = append(metrics, diskStr)
			}

			// Determine online status indicator.
			statusIcon := "🟢"
			if node.HasHighUtilization() {
				statusIcon = "🟡"
			}

			nodeLine := fmt.Sprintf("    %s %s: %s", statusIcon, node.Hostname, strings.Join(metrics, " | "))
			lines = append(lines, nodeLine)
		}
	}

	for _, cluster := range data.Kubernetes {
		statusStr := cluster.Status
		if l.config.ColorEnabled {
			statusStr = lipgloss.NewStyle().
				Foreground(l.k8sStatusColor(cluster.Status)).
				Render(cluster.Status)
		}
		lines = append(lines, fmt.Sprintf("  k8s: %s (%s)", cluster.Name, statusStr))
	}

	if len(lines) == 0 {
		return []string{l.styledMuted("  (no data)")}
	}

	return lines
}

// composeSideBySide places imageContent on the left and infoPanel on the right,
// handling line-by-line alignment. If imageContent is empty, only the info panel
// is shown (full-width).
func (l *Layout) composeSideBySide(imageContent, infoPanel string) string {
	if imageContent == "" {
		lines := strings.Split(infoPanel, "\n")
		if len(lines) > l.config.TermHeight {
			lines = lines[:l.config.TermHeight]
		}
		return strings.Join(lines, "\n")
	}

	imageLines := strings.Split(imageContent, "\n")
	infoLines := strings.Split(infoPanel, "\n")

	maxRows := len(imageLines)
	if len(infoLines) > maxRows {
		maxRows = len(infoLines)
	}
	if maxRows > l.config.TermHeight {
		maxRows = l.config.TermHeight
	}

	separator := " | "
	var result []string

	for i := 0; i < maxRows; i++ {
		imgLine := ""
		if i < len(imageLines) {
			imgLine = imageLines[i]
		}
		infoLine := ""
		if i < len(infoLines) {
			infoLine = infoLines[i]
		}

		// Pad the image line to ImageCols width.
		imgLine = l.padToWidth(imgLine, l.config.ImageCols)

		result = append(result, imgLine+separator+infoLine)
	}

	return strings.Join(result, "\n")
}

// padToWidth pads or truncates a string to exactly the given width.
// It accounts for visible character width, not byte length.
func (l *Layout) padToWidth(s string, width int) string {
	visible := visibleLen(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// visibleLen returns the visible length of a string, stripping ANSI escape sequences.
func visibleLen(s string) int {
	length := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '~' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		length++
	}
	return length
}

// separator returns a styled horizontal separator line.
func (l *Layout) separator() string {
	sep := strings.Repeat("\u2500", 24)
	if l.config.ColorEnabled {
		return lipgloss.NewStyle().Foreground(colorMuted).Render(sep)
	}
	return sep
}

// sectionTitle returns a styled section title.
func (l *Layout) sectionTitle(title string) string {
	if l.config.ColorEnabled {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(colorSecondary).
			Render(title)
	}
	return title
}

// styledMuted returns a muted (gray) styled string.
func (l *Layout) styledMuted(s string) string {
	if l.config.ColorEnabled {
		return lipgloss.NewStyle().Foreground(colorMuted).Render(s)
	}
	return s
}

// statusColor returns the lipgloss color for a given status level.
func (l *Layout) statusColor(status string) lipgloss.Color {
	switch status {
	case "healthy":
		return colorSuccess
	case "warning":
		return colorWarning
	case "critical":
		return colorDanger
	default:
		return colorMuted
	}
}

// k8sStatusColor returns the color for a Kubernetes cluster status.
func (l *Layout) k8sStatusColor(status string) lipgloss.Color {
	switch status {
	case "healthy":
		return colorSuccess
	case "degraded":
		return colorWarning
	case "offline":
		return colorDanger
	default:
		return colorMuted
	}
}
