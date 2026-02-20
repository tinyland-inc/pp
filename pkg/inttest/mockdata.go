package inttest

import (
	"os"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/app"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/banner"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/cache"
)

// itMockClaudeData returns mock Claude API response data structured as
// a generic map. This mirrors the shape of claude.UsageReport without
// importing the collector package.
func itMockClaudeData() map[string]any {
	return map[string]any{
		"total_cost_usd": 142.30,
		"accounts": []map[string]any{
			{
				"name": "personal",
				"models": []map[string]any{
					{"model": "claude-opus-4-20250514", "cost_usd": 98.50, "input_tokens": 1500000, "output_tokens": 800000},
					{"model": "claude-3-5-sonnet-20241022", "cost_usd": 43.80, "input_tokens": 3200000, "output_tokens": 1200000},
				},
			},
		},
		"period_start": "2026-02-01T00:00:00Z",
		"period_end":   "2026-02-28T00:00:00Z",
	}
}

// itMockBillingData returns mock billing data for cloud providers.
func itMockBillingData() map[string]any {
	return map[string]any{
		"total_monthly_usd": 57.50,
		"budget_usd":        100.0,
		"providers": []map[string]any{
			{
				"name":        "civo",
				"monthly_usd": 12.50,
				"services": []map[string]any{
					{"name": "k3s-cluster", "monthly_usd": 10.00},
					{"name": "network", "monthly_usd": 2.50},
				},
			},
			{
				"name":        "digitalocean",
				"monthly_usd": 45.00,
				"services": []map[string]any{
					{"name": "doks-cluster", "monthly_usd": 36.00},
					{"name": "load-balancer", "monthly_usd": 9.00},
				},
			},
		},
	}
}

// itMockTailscaleData returns mock Tailscale peer list data.
func itMockTailscaleData() map[string]any {
	return map[string]any{
		"total_peers":  5,
		"online_peers": 3,
		"peers": []map[string]any{
			{"hostname": "honey", "online": true, "os": "linux", "ip": "100.64.0.1"},
			{"hostname": "petting-zoo-mini", "online": true, "os": "darwin", "ip": "100.64.0.2"},
			{"hostname": "localhost", "online": true, "os": "darwin", "ip": "100.64.0.3"},
			{"hostname": "yoga", "online": false, "os": "linux", "ip": "100.64.0.4"},
			{"hostname": "xoxd-bates", "online": false, "os": "darwin", "ip": "100.64.0.5"},
		},
	}
}

// itMockK8sData returns mock Kubernetes cluster data.
func itMockK8sData() map[string]any {
	return map[string]any{
		"clusters": []map[string]any{
			{
				"name":         "civo-tinyland",
				"connected":    true,
				"total_pods":   10,
				"running_pods": 9,
				"failed_pods":  1,
				"nodes":        3,
				"deployments": []map[string]any{
					{"name": "prompt-pulse-daemon", "replicas": 2, "ready": 2},
					{"name": "sunshine-proxy", "replicas": 1, "ready": 1},
				},
			},
			{
				"name":         "doks-prod",
				"connected":    true,
				"total_pods":   5,
				"running_pods": 5,
				"failed_pods":  0,
				"nodes":        2,
				"deployments": []map[string]any{
					{"name": "api-gateway", "replicas": 2, "ready": 2},
				},
			},
		},
	}
}

// itMockSysMetrics returns mock system metrics data.
func itMockSysMetrics() map[string]any {
	return map[string]any{
		"cpu": map[string]any{
			"total":    45.2,
			"per_core": []float64{52.1, 38.4, 45.0, 45.3},
			"cores":    4,
		},
		"memory": map[string]any{
			"total_bytes":  17179869184, // 16 GB
			"used_bytes":   10737418240, // 10 GB
			"used_percent": 62.5,
		},
		"disk": map[string]any{
			"total_bytes":  512110190592, // 512 GB
			"used_bytes":   363646451712, // ~339 GB
			"used_percent": 71.0,
			"mount_point":  "/System/Volumes/Data",
		},
		"load_avg": map[string]any{
			"load1":  2.5,
			"load5":  2.1,
			"load15": 1.8,
		},
	}
}

// itMockWaifuImage returns a minimal valid PNG file (1x1 pixel, RGBA).
// This is the smallest possible valid PNG.
func itMockWaifuImage() []byte {
	// Minimal 1x1 white pixel PNG.
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // 8-bit RGB
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00, // compressed
		0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC, // data
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, // IEND chunk
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}

// itMockConfig returns a complete v2 TOML config string suitable for
// parsing with config.LoadFromReader.
func itMockConfig() string {
	return `[general]
daemon_poll_interval = "15m"
data_retention = "10m"
log_level = "info"
cache_dir = "/tmp/prompt-pulse-test"

[layout]
preset = "dashboard"

[collectors.sysmetrics]
enabled = true
interval = "1s"

[collectors.tailscale]
enabled = true
interval = "30s"

[collectors.kubernetes]
enabled = false
interval = "60s"
contexts = ["civo-tinyland", "doks-prod"]
namespaces = ["default", "monitoring"]

[collectors.claude]
enabled = true
interval = "5m"

[collectors.billing]
enabled = false
interval = "15m"

[collectors.billing.civo]
enabled = false

[collectors.billing.digitalocean]
enabled = false

[image]
protocol = "auto"
max_cache_size_mb = 50
max_sessions = 10
waifu_enabled = true
waifu_category = "waifu"

[theme]
name = "catppuccin"

[shell]
tui_keybinding = "\\C-p"
show_banner_on_startup = true
banner_timeout = "2s"
instant_banner = true

[banner]
compact_max_width = 80
standard_min_width = 120
wide_min_width = 160
ultrawide_min_width = 200
`
}

// itMockBannerWidgets returns a slice of WidgetData for all six widget
// types, suitable for banner rendering tests.
func itMockBannerWidgets() []banner.WidgetData {
	return []banner.WidgetData{
		{
			ID:      "waifu",
			Title:   "Waifu",
			Content: "[image placeholder]",
			MinW:    30,
			MinH:    15,
		},
		{
			ID:      "claude",
			Title:   "Claude Usage",
			Content: itMockClaudeWidget(),
			MinW:    25,
			MinH:    5,
		},
		{
			ID:      "billing",
			Title:   "Cloud Billing",
			Content: itMockBillingWidget(),
			MinW:    25,
			MinH:    5,
		},
		{
			ID:      "tailscale",
			Title:   "Tailscale",
			Content: itMockTailscaleWidget(),
			MinW:    25,
			MinH:    6,
		},
		{
			ID:      "k8s",
			Title:   "Kubernetes",
			Content: itMockK8sWidget(),
			MinW:    25,
			MinH:    5,
		},
		{
			ID:      "sysmetrics",
			Title:   "System Metrics",
			Content: itMockSysMetricsWidget(),
			MinW:    25,
			MinH:    5,
		},
	}
}

// itMockAppWidgets returns a slice of app.Widget implementations
// using PlaceholderWidget for TUI and AppModel testing.
func itMockAppWidgets() []app.Widget {
	return []app.Widget{
		app.NewPlaceholder("claude", "Claude Usage"),
		app.NewPlaceholder("billing", "Cloud Billing"),
		app.NewPlaceholder("tailscale", "Tailscale"),
		app.NewPlaceholder("k8s", "Kubernetes"),
		app.NewPlaceholder("sysmetrics", "System Metrics"),
		app.NewPlaceholder("waifu", "Waifu"),
	}
}

// itTempDir creates a temporary directory for test use and returns the
// path, a cleanup function, and any error.
func itTempDir(prefix string) (string, func(), error) {
	dir, err := os.MkdirTemp("", prefix+"-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() {
		os.RemoveAll(dir)
	}
	return dir, cleanup, nil
}

// itNewCacheStore creates a cache.Store in the given directory with
// test-friendly settings (small size, short TTL, fast cleanup).
func itNewCacheStore(dir string) (*cache.Store, error) {
	return cache.NewStore(cache.StoreConfig{
		Dir:             dir,
		MaxSizeMB:       10,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Second,
	})
}
