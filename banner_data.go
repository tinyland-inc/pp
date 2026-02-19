package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/banner"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/billing"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/claude"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/k8s"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/sysmetrics"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/tailscale"
)

// bnMaxCacheAge is the maximum age of a cache file before it is considered
// stale and skipped in banner rendering.
const bnMaxCacheAge = 5 * time.Minute

// buildBannerFromCache reads cached collector JSON files written by the daemon
// and assembles them into BannerData widgets for the banner renderer.
func buildBannerFromCache(cacheDir, ver, commit string) banner.BannerData {
	widgets := []banner.WidgetData{
		{
			ID:      "status",
			Title:   "System Status",
			Content: fmt.Sprintf("prompt-pulse v%s (%s)", ver, commit),
			MinW:    30,
			MinH:    3,
		},
	}

	if m, err := bnReadCache[sysmetrics.Metrics](cacheDir, "sysmetrics"); err == nil && m != nil {
		content := fmt.Sprintf("CPU: %.0f%%  RAM: %.0f%%\nLoad: %.1f / %.1f / %.1f\nUptime: %s",
			m.CPU.Total, m.Memory.UsedPercent,
			m.Load.Load1, m.Load.Load5, m.Load.Load15,
			bnFormatUptime(m.Uptime))
		widgets = append(widgets, banner.WidgetData{
			ID: "system", Title: "System", Content: content, MinW: 30, MinH: 5,
		})
	}

	if s, err := bnReadCache[tailscale.Status](cacheDir, "tailscale"); err == nil && s != nil {
		content := fmt.Sprintf("Peers: %d/%d online\nNet: %s",
			s.OnlinePeers, s.TotalPeers, s.TailnetName)
		widgets = append(widgets, banner.WidgetData{
			ID: "tailscale", Title: "Tailscale", Content: content, MinW: 25, MinH: 4,
		})
	}

	if cs, err := bnReadCache[k8s.ClusterStatus](cacheDir, "k8s"); err == nil && cs != nil {
		var total, running, failed int
		for _, c := range cs.Clusters {
			if c.Connected {
				total += c.TotalPods
				running += c.RunningPods
				failed += c.FailedPods
			}
		}
		if total > 0 {
			content := fmt.Sprintf("Pods: %d/%d running", running, total)
			if failed > 0 {
				content += fmt.Sprintf(" (%d failed)", failed)
			}
			widgets = append(widgets, banner.WidgetData{
				ID: "k8s", Title: "Kubernetes", Content: content, MinW: 25, MinH: 3,
			})
		}
	}

	if r, err := bnReadCache[claude.UsageReport](cacheDir, "claude"); err == nil && r != nil {
		content := fmt.Sprintf("Cost: $%.2f", r.TotalCostUSD)
		widgets = append(widgets, banner.WidgetData{
			ID: "claude", Title: "Claude", Content: content, MinW: 20, MinH: 3,
		})
	}

	if b, err := bnReadCache[billing.BillingReport](cacheDir, "billing"); err == nil && b != nil {
		content := fmt.Sprintf("Spend: $%.2f/mo", b.TotalMonthlyUSD)
		if b.BudgetUSD > 0 {
			content += fmt.Sprintf(" (%.0f%% of budget)", b.BudgetPercent)
		}
		widgets = append(widgets, banner.WidgetData{
			ID: "billing", Title: "Cloud Billing", Content: content, MinW: 25, MinH: 3,
		})
	}

	return banner.BannerData{Widgets: widgets}
}

// bnReadCache reads a JSON cache file for the given collector key.
// Returns nil if the file does not exist, cannot be parsed, or is stale.
func bnReadCache[T any](cacheDir, key string) (*T, error) {
	path := filepath.Join(cacheDir, key+".json")

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if time.Since(info.ModTime()) > bnMaxCacheAge {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// bnFormatUptime formats a duration as a human-readable uptime string.
func bnFormatUptime(d time.Duration) string {
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
