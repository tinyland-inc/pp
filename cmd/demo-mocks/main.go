package main

import (
	"flag"
	"fmt"
	"os"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/banner"
	"gitlab.com/tinyland/lab/prompt-pulse/display/layout"
)

func main() {
	termWidth := flag.Int("width", 160, "Terminal width")
	termHeight := flag.Int("height", 60, "Terminal height")
	withMetrics := flag.Bool("with-metrics", false, "Use mock data with node metrics")
	withHistory := flag.Bool("with-history", false, "Use mock data with billing history")
	flag.Parse()

	fmt.Println("=== Prompt-Pulse Mock Data Demo ===")
	fmt.Printf("Terminal size: %dx%d\n", *termWidth, *termHeight)
	fmt.Println()

	// Create mock data
	claudeMock := collectors.MockClaudeUsage()
	var billingMock *collectors.BillingData
	if *withHistory {
		billingMock = collectors.MockBillingDataWithHistory()
	} else {
		billingMock = collectors.MockBillingData()
	}

	var infraMock *collectors.InfraStatus
	if *withMetrics {
		infraMock = collectors.MockInfraStatusWithMetrics()
	} else {
		infraMock = collectors.MockInfraStatus()
	}

	fastfetchMock := collectors.MockFastfetchData()

	// Create layout
	layoutMode := banner.DetermineLayoutMode(*termWidth)
	fmt.Printf("Layout mode: %v\n", layoutMode)
	fmt.Println()

	responsiveConfig := layout.NewResponsiveConfig(*termWidth, *termHeight)
	responsiveConfig.ColorEnabled = false // ASCII-only for demo
	resp := layout.NewResponsiveLayout(responsiveConfig)

	// Build sections
	sections := buildMockSections(claudeMock, billingMock, infraMock, fastfetchMock, layoutMode)

	// Render
	result := resp.Render("", sections, billingMock)

	fmt.Println(result.Output)
	fmt.Println()
	fmt.Printf("Lines: %d, Truncated: %v\n", result.Lines, result.Truncated)
}

func buildMockSections(
	claude *collectors.ClaudeUsage,
	billing *collectors.BillingData,
	infra *collectors.InfraStatus,
	fastfetch *collectors.FastfetchData,
	mode banner.LayoutMode,
) []layout.Section {
	var sections []layout.Section

	// Status section
	sections = append(sections, layout.Section{
		Title:   "Status",
		Content: []string{"  yoga :: healthy", "  uptime: 1d 2h"},
	})

	// Claude section
	sections = append(sections, layout.Section{
		Title:   "Claude",
		Content: formatClaude(claude),
	})

	// Billing section
	sections = append(sections, layout.Section{
		Title:   "Billing",
		Content: formatBilling(billing),
	})

	// Infrastructure section
	showNodeMetrics := mode >= banner.LayoutWide // Wide and UltraWide show metrics
	sections = append(sections, layout.Section{
		Title:   "Infrastructure",
		Content: formatInfra(infra, showNodeMetrics),
	})

	// System section (fastfetch)
	if fastfetch != nil && !fastfetch.IsEmpty() {
		sections = append(sections, layout.Section{
			Title:   "System",
			Content: fastfetch.FormatCompact(),
		})
	}

	return sections
}

func formatClaude(c *collectors.ClaudeUsage) []string {
	if c == nil || len(c.Accounts) == 0 {
		return []string{"  (no data)"}
	}

	var lines []string
	for _, acct := range c.Accounts {
		if acct.Status != collectors.StatusOK {
			lines = append(lines, fmt.Sprintf("  %s: ERR", acct.Name))
			continue
		}

		util := acct.GetPrimaryUtilization()
		utilSec := acct.GetSecondaryUtilization()

		line := fmt.Sprintf("  %s: %.0f%% (5h) | %.0f%% (7d)", acct.Name, util, utilSec)
		lines = append(lines, line)
	}

	return lines
}

func formatBilling(b *collectors.BillingData) []string {
	if b == nil {
		return []string{"  (no data)"}
	}

	var lines []string

	for _, provider := range b.Providers {
		if provider.Status != collectors.StatusOK {
			lines = append(lines, fmt.Sprintf("  %s: ERR", provider.Provider))
			continue
		}

		lines = append(lines, fmt.Sprintf("  %s: $%.2f", provider.Provider, provider.CurrentMonth.SpendUSD))
	}

	if b.Total.CurrentMonthUSD > 0 {
		lines = append(lines, fmt.Sprintf("  Total: $%.2f this month", b.Total.CurrentMonthUSD))
	} else {
		lines = append(lines, "  $0 this month")
	}

	return lines
}

func formatInfra(i *collectors.InfraStatus, showNodeMetrics bool) []string {
	if i == nil {
		return []string{"  (no data)"}
	}

	var lines []string

	if i.Tailscale != nil {
		lines = append(lines, fmt.Sprintf("  ts: %d/%d online",
			i.Tailscale.OnlineCount, i.Tailscale.TotalCount))

		// Show per-node metrics if enabled
		if showNodeMetrics {
			for _, node := range i.Tailscale.Nodes {
				if !node.Online {
					continue
				}
				if node.CPUPercent == nil && node.RAMPercent == nil && node.DiskPercent == nil {
					continue
				}

				var metrics []string
				if node.CPUPercent != nil {
					metrics = append(metrics, fmt.Sprintf("CPU %.0f%%", *node.CPUPercent))
				}
				if node.RAMPercent != nil {
					metrics = append(metrics, fmt.Sprintf("RAM %.0f%%", *node.RAMPercent))
				}
				if node.DiskPercent != nil {
					metrics = append(metrics, fmt.Sprintf("Disk %.0f%%", *node.DiskPercent))
				}

				if len(metrics) > 0 {
					lines = append(lines, fmt.Sprintf("    %s: %s", node.Hostname, join(metrics, " | ")))
				}
			}
		}
	}

	for _, cluster := range i.Kubernetes {
		lines = append(lines, fmt.Sprintf("  k8s: %s (%s)", cluster.Name, cluster.Status))
	}

	if len(lines) == 0 {
		return []string{"  (no data)"}
	}

	return lines
}

func join(parts []string, sep string) string {
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += sep
		}
		result += part
	}
	return result
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Demo tool for prompt-pulse mock data visualization.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Compact layout (80x24)\n")
		fmt.Fprintf(os.Stderr, "  %s -width 80 -height 24\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Wide layout with node metrics (160x60)\n")
		fmt.Fprintf(os.Stderr, "  %s -width 160 -height 60 -with-metrics\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # UltraWide with billing history sparklines (200x80)\n")
		fmt.Fprintf(os.Stderr, "  %s -width 200 -height 80 -with-history\n", os.Args[0])
	}
}
