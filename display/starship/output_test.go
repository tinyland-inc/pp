package starship

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// newTestLogger returns a no-op logger suitable for tests.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestStore creates a cache store in a temporary directory.
func newTestStore(t *testing.T) (*cache.Store, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := cache.NewStore(dir, newTestLogger())
	if err != nil {
		t.Fatalf("NewStore(%q): %v", dir, err)
	}
	return store, dir
}

// newTestOutput creates an Output backed by the given cache directory.
func newTestOutput(t *testing.T, dir string, ttl time.Duration) *Output {
	t.Helper()
	out, err := NewOutput(OutputConfig{
		CacheDir: dir,
		CacheTTL: ttl,
		Logger:   newTestLogger(),
	})
	if err != nil {
		t.Fatalf("NewOutput: %v", err)
	}
	return out
}

// ========== Claude Module Tests ==========

func TestOutput_Claude_Fresh(t *testing.T) {
	store, dir := newTestStore(t)

	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "test",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 45,
					ResetsAt:    time.Date(2025, 1, 15, 5, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	if err := cache.SetTyped(store, CacheKeyClaude, data); err != nil {
		t.Fatalf("SetTyped: %v", err)
	}

	out := newTestOutput(t, dir, 30*time.Minute)
	result, err := out.Module("claude")
	if err != nil {
		t.Fatalf("Module(claude): %v", err)
	}

	want := data.StarshipOutput()
	if result != want {
		t.Errorf("Module(claude) = %q, want %q", result, want)
	}
}

func TestOutput_Claude_Stale(t *testing.T) {
	store, dir := newTestStore(t)

	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "stale-acct",
				Type:   "subscription",
				Tier:   "max_5x",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 80,
					ResetsAt:    time.Date(2025, 1, 15, 5, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	if err := cache.SetTyped(store, CacheKeyClaude, data); err != nil {
		t.Fatalf("SetTyped: %v", err)
	}

	// Set the file modification time to 1 hour ago so it is past the 30-minute TTL.
	cachePath := filepath.Join(dir, CacheKeyClaude+".json")
	past := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(cachePath, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	out := newTestOutput(t, dir, 30*time.Minute)
	result, err := out.Module("claude")
	if err != nil {
		t.Fatalf("Module(claude): %v", err)
	}

	want := data.StarshipOutput() + " ?"
	if result != want {
		t.Errorf("Module(claude) stale = %q, want %q", result, want)
	}
}

func TestOutput_Claude_Missing(t *testing.T) {
	_, dir := newTestStore(t)

	out := newTestOutput(t, dir, 30*time.Minute)
	result, err := out.Module("claude")
	if err != nil {
		t.Fatalf("Module(claude): %v", err)
	}
	if result != "" {
		t.Errorf("Module(claude) on empty cache = %q, want empty string", result)
	}
}

// ========== Billing Module Tests ==========

func TestOutput_Billing_Fresh(t *testing.T) {
	store, dir := newTestStore(t)

	data := &collectors.BillingData{
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 42,
		},
	}
	if err := cache.SetTyped(store, CacheKeyBilling, data); err != nil {
		t.Fatalf("SetTyped: %v", err)
	}

	out := newTestOutput(t, dir, 30*time.Minute)
	result, err := out.Module("billing")
	if err != nil {
		t.Fatalf("Module(billing): %v", err)
	}

	want := data.StarshipOutput()
	if result != want {
		t.Errorf("Module(billing) = %q, want %q", result, want)
	}
}

func TestOutput_Billing_WithForecast(t *testing.T) {
	store, dir := newTestStore(t)

	forecast := 200.0
	data := &collectors.BillingData{
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 95,
			ForecastUSD:     &forecast,
		},
	}
	if err := cache.SetTyped(store, CacheKeyBilling, data); err != nil {
		t.Fatalf("SetTyped: %v", err)
	}

	out := newTestOutput(t, dir, 30*time.Minute)
	result, err := out.Module("billing")
	if err != nil {
		t.Fatalf("Module(billing): %v", err)
	}

	want := "$95 ($200 forecast)"
	if result != want {
		t.Errorf("Module(billing) = %q, want %q", result, want)
	}
}

// ========== Infra Module Tests ==========

func TestOutput_Infra_Fresh(t *testing.T) {
	store, dir := newTestStore(t)

	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			OnlineCount: 5,
			TotalCount:  8,
		},
		Kubernetes: []collectors.KubernetesCluster{
			{Name: "civo", Status: "healthy"},
		},
	}
	if err := cache.SetTyped(store, CacheKeyInfra, data); err != nil {
		t.Fatalf("SetTyped: %v", err)
	}

	out := newTestOutput(t, dir, 30*time.Minute)
	result, err := out.Module("infra")
	if err != nil {
		t.Fatalf("Module(infra): %v", err)
	}

	want := data.StarshipOutput()
	if result != want {
		t.Errorf("Module(infra) = %q, want %q", result, want)
	}
}

// ========== Module Dispatch Tests ==========

func TestOutput_Module_Unknown(t *testing.T) {
	_, dir := newTestStore(t)
	out := newTestOutput(t, dir, 30*time.Minute)

	_, err := out.Module("nonexistent")
	if err == nil {
		t.Error("Module(nonexistent) expected error, got nil")
	}
}

func TestOutput_Module_AllModules(t *testing.T) {
	store, dir := newTestStore(t)

	claude := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "dev",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 20,
					ResetsAt:    time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	billing := &collectors.BillingData{
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 10,
		},
	}
	infra := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			OnlineCount: 3,
			TotalCount:  3,
		},
	}

	if err := cache.SetTyped(store, CacheKeyClaude, claude); err != nil {
		t.Fatalf("SetTyped(claude): %v", err)
	}
	if err := cache.SetTyped(store, CacheKeyBilling, billing); err != nil {
		t.Fatalf("SetTyped(billing): %v", err)
	}
	if err := cache.SetTyped(store, CacheKeyInfra, infra); err != nil {
		t.Fatalf("SetTyped(infra): %v", err)
	}

	out := newTestOutput(t, dir, 30*time.Minute)

	modules := []string{"claude", "billing", "infra"}
	for _, m := range modules {
		result, err := out.Module(m)
		if err != nil {
			t.Errorf("Module(%q): unexpected error: %v", m, err)
		}
		if result == "" {
			t.Errorf("Module(%q) = empty string, expected output", m)
		}
	}
}

// ========== Edge Case Tests ==========

func TestOutput_EmptyData(t *testing.T) {
	store, dir := newTestStore(t)

	// ClaudeUsage with empty Accounts slice should produce "".
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{},
	}
	if err := cache.SetTyped(store, CacheKeyClaude, data); err != nil {
		t.Fatalf("SetTyped: %v", err)
	}

	out := newTestOutput(t, dir, 30*time.Minute)
	result := out.Claude()
	if result != "" {
		t.Errorf("Claude() with empty accounts = %q, want empty string", result)
	}
}

func TestDefaultOutputConfig(t *testing.T) {
	cfg := DefaultOutputConfig()

	if cfg.CacheTTL != 30*time.Minute {
		t.Errorf("DefaultOutputConfig().CacheTTL = %v, want 30m", cfg.CacheTTL)
	}
	if cfg.Logger == nil {
		t.Error("DefaultOutputConfig().Logger is nil, want non-nil")
	}
	if cfg.CacheDir == "" {
		t.Error("DefaultOutputConfig().CacheDir is empty")
	}

	home, err := os.UserHomeDir()
	if err == nil {
		want := filepath.Join(home, ".cache", "prompt-pulse")
		if cfg.CacheDir != want {
			t.Errorf("DefaultOutputConfig().CacheDir = %q, want %q", cfg.CacheDir, want)
		}
	}
}

func TestNewOutput_InvalidDir(t *testing.T) {
	// Use /dev/null as the cache directory -- this should fail because it is
	// not a directory and MkdirAll will error.
	_, err := NewOutput(OutputConfig{
		CacheDir: "/dev/null/impossible",
		CacheTTL: 5 * time.Minute,
		Logger:   newTestLogger(),
	})
	if err == nil {
		t.Error("NewOutput with invalid dir expected error, got nil")
	}
}
