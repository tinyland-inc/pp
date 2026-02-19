package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/billing"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/claude"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/k8s"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/sysmetrics"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/tailscale"
)

func bnWriteFixture(t *testing.T, dir, key string, v interface{}) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture %s: %v", key, err)
	}
	if err := os.WriteFile(filepath.Join(dir, key+".json"), data, 0644); err != nil {
		t.Fatalf("write fixture %s: %v", key, err)
	}
}

func TestBuildBannerFromCache_Empty(t *testing.T) {
	dir := t.TempDir()
	data := buildBannerFromCache(dir, "2.0.5", "abc123")

	if len(data.Widgets) != 1 {
		t.Fatalf("expected 1 widget (status only), got %d", len(data.Widgets))
	}
	if data.Widgets[0].ID != "status" {
		t.Errorf("expected status widget, got %s", data.Widgets[0].ID)
	}
	if !strings.Contains(data.Widgets[0].Content, "2.0.5") {
		t.Errorf("status widget should contain version, got %q", data.Widgets[0].Content)
	}
}

func TestBuildBannerFromCache_WithSysmetrics(t *testing.T) {
	dir := t.TempDir()
	bnWriteFixture(t, dir, "sysmetrics", sysmetrics.Metrics{
		CPU:    sysmetrics.CPUMetrics{Total: 42.5, Count: 8},
		Memory: sysmetrics.MemoryMetrics{UsedPercent: 67.3},
		Load:   sysmetrics.LoadMetrics{Load1: 1.5, Load5: 2.0, Load15: 1.8},
		Uptime: 3 * time.Hour,
	})

	data := buildBannerFromCache(dir, "2.0.5", "abc123")

	if len(data.Widgets) != 2 {
		t.Fatalf("expected 2 widgets (status + system), got %d", len(data.Widgets))
	}

	sysWidget := data.Widgets[1]
	if sysWidget.ID != "system" {
		t.Errorf("expected system widget, got %s", sysWidget.ID)
	}
	if !strings.Contains(sysWidget.Content, "CPU: 42%") {
		t.Errorf("system widget should contain CPU percentage, got %q", sysWidget.Content)
	}
	if !strings.Contains(sysWidget.Content, "RAM: 67%") {
		t.Errorf("system widget should contain RAM percentage, got %q", sysWidget.Content)
	}
}

func TestBuildBannerFromCache_WithAll(t *testing.T) {
	dir := t.TempDir()

	bnWriteFixture(t, dir, "sysmetrics", sysmetrics.Metrics{
		CPU:    sysmetrics.CPUMetrics{Total: 10},
		Memory: sysmetrics.MemoryMetrics{UsedPercent: 50},
		Load:   sysmetrics.LoadMetrics{Load1: 1.0},
		Uptime: 48 * time.Hour,
	})

	bnWriteFixture(t, dir, "tailscale", tailscale.Status{
		OnlinePeers: 3,
		TotalPeers:  5,
		TailnetName: "tinyland.ts.net",
	})

	bnWriteFixture(t, dir, "k8s", k8s.ClusterStatus{
		Clusters: []k8s.ClusterInfo{
			{Context: "civo", Connected: true, TotalPods: 15, RunningPods: 12, FailedPods: 1},
		},
	})

	bnWriteFixture(t, dir, "claude", claude.UsageReport{
		TotalCostUSD: 142.30,
	})

	bnWriteFixture(t, dir, "billing", billing.BillingReport{
		TotalMonthlyUSD: 23.45,
		BudgetUSD:       100.0,
		BudgetPercent:   23.45,
	})

	data := buildBannerFromCache(dir, "2.0.5", "abc123")

	// status + system + tailscale + k8s + claude + billing = 6
	if len(data.Widgets) != 6 {
		t.Fatalf("expected 6 widgets, got %d", len(data.Widgets))
	}

	ids := make(map[string]bool)
	for _, w := range data.Widgets {
		ids[w.ID] = true
	}
	for _, expected := range []string{"status", "system", "tailscale", "k8s", "claude", "billing"} {
		if !ids[expected] {
			t.Errorf("missing widget: %s", expected)
		}
	}
}

func TestBuildBannerFromCache_StaleCache(t *testing.T) {
	dir := t.TempDir()

	// Write a fixture then backdate it beyond the staleness threshold.
	bnWriteFixture(t, dir, "sysmetrics", sysmetrics.Metrics{
		CPU: sysmetrics.CPUMetrics{Total: 50},
	})
	staleTime := time.Now().Add(-10 * time.Minute)
	path := filepath.Join(dir, "sysmetrics.json")
	if err := os.Chtimes(path, staleTime, staleTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	data := buildBannerFromCache(dir, "2.0.5", "abc123")

	// Stale cache should be skipped — only status widget.
	if len(data.Widgets) != 1 {
		t.Fatalf("expected 1 widget (stale cache skipped), got %d", len(data.Widgets))
	}
}

func TestBnFormatUptime(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "30m"},
		{2*time.Hour + 15*time.Minute, "2h 15m"},
		{49*time.Hour + 30*time.Minute, "2d 1h"},
	}
	for _, tt := range tests {
		got := bnFormatUptime(tt.d)
		if got != tt.want {
			t.Errorf("bnFormatUptime(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
