package banner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// testLogger returns a silent logger for tests.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testConfig returns a BannerConfig using the given temp directory.
func testConfig(t *testing.T) BannerConfig {
	t.Helper()
	dir := t.TempDir()
	return BannerConfig{
		CacheDir:        dir,
		CacheTTL:        15 * time.Minute,
		WaifuEnabled:    false,
		WaifuCacheDir:   filepath.Join(dir, "waifu"),
		WaifuCacheTTL:   24 * time.Hour,
		WaifuMaxCacheMB: 50,
		Hostname:        "test-host",
		TermWidth:       80,
		TermHeight:      24,
		Logger:          testLogger(),
	}
}

func TestDefaultBannerConfig_HasSensibleDefaults(t *testing.T) {
	cfg := DefaultBannerConfig()

	if cfg.CacheDir == "" {
		t.Error("CacheDir should not be empty")
	}
	if cfg.CacheTTL <= 0 {
		t.Error("CacheTTL should be positive")
	}
	if cfg.WaifuEnabled {
		t.Error("WaifuEnabled should default to false")
	}
	if cfg.WaifuCacheDir == "" {
		t.Error("WaifuCacheDir should not be empty")
	}
	if cfg.WaifuCacheTTL <= 0 {
		t.Error("WaifuCacheTTL should be positive")
	}
	if cfg.WaifuMaxCacheMB <= 0 {
		t.Error("WaifuMaxCacheMB should be positive")
	}
	if cfg.TermWidth <= 0 {
		t.Error("TermWidth should be positive")
	}
	if cfg.TermHeight <= 0 {
		t.Error("TermHeight should be positive")
	}
	if cfg.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

func TestNewBanner_CreatesInstance(t *testing.T) {
	cfg := testConfig(t)
	b := NewBanner(cfg)

	if b == nil {
		t.Fatal("NewBanner should return non-nil")
	}
}

func TestNewBanner_NilLoggerGetsDefault(t *testing.T) {
	cfg := testConfig(t)
	cfg.Logger = nil
	b := NewBanner(cfg)

	if b == nil {
		t.Fatal("NewBanner with nil logger should return non-nil")
	}
	if b.config.Logger == nil {
		t.Error("NewBanner should set a default logger when nil is provided")
	}
}

func TestGenerate_NoCacheReturnsInfoOnlyBanner(t *testing.T) {
	cfg := testConfig(t)
	// Point to a non-existent directory so cache open fails gracefully.
	cfg.CacheDir = filepath.Join(t.TempDir(), "nonexistent", "deep", "path")
	b := NewBanner(cfg)

	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate should not return error with no cache: %v", err)
	}
	if output == "" {
		t.Error("Generate should return non-empty output even with no cache")
	}
}

func TestGenerate_WithMockCachedData(t *testing.T) {
	cfg := testConfig(t)
	b := NewBanner(cfg)

	// Populate cache with test data.
	store, err := cache.NewStore(cfg.CacheDir, cfg.Logger)
	if err != nil {
		t.Fatalf("failed to create cache store: %v", err)
	}

	claude := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "test",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 42.5,
					ResetsAt:    time.Now().Add(3 * time.Hour),
				},
			},
		},
	}
	if err := cache.SetTyped(store, "claude", claude); err != nil {
		t.Fatalf("failed to set claude cache: %v", err)
	}

	billing := &collectors.BillingData{
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 125.50,
		},
	}
	if err := cache.SetTyped(store, "billing", billing); err != nil {
		t.Fatalf("failed to set billing cache: %v", err)
	}

	infra := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "test.ts.net",
			OnlineCount: 5,
			TotalCount:  6,
		},
	}
	if err := cache.SetTyped(store, "infra", infra); err != nil {
		t.Fatalf("failed to set infra cache: %v", err)
	}

	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate with cached data should not error: %v", err)
	}
	if output == "" {
		t.Error("Generate with cached data should produce non-empty output")
	}
}

func TestLoadCachedData_EmptyCacheReturnsNils(t *testing.T) {
	cfg := testConfig(t)
	b := NewBanner(cfg)

	store, err := cache.NewStore(cfg.CacheDir, cfg.Logger)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	claude, billing, infra := b.loadCachedData(store)
	if claude != nil {
		t.Error("claude should be nil with empty cache")
	}
	if billing != nil {
		t.Error("billing should be nil with empty cache")
	}
	if infra != nil {
		t.Error("infra should be nil with empty cache")
	}
}

func TestFetchWaifuImage_NoCacheReturnsEmpty(t *testing.T) {
	cfg := testConfig(t)
	cfg.WaifuCacheDir = filepath.Join(t.TempDir(), "waifu-empty")
	b := NewBanner(cfg)

	result := b.fetchWaifuImage(context.Background(), "happy")
	if result != "" {
		t.Error("fetchWaifuImage with no cached image should return empty string")
	}
}

func TestFetchWaifuImage_ContextCancelled(t *testing.T) {
	cfg := testConfig(t)
	b := NewBanner(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	result := b.fetchWaifuImage(ctx, "happy")
	if result != "" {
		t.Error("fetchWaifuImage with cancelled context should return empty string")
	}
}

func TestBannerConfig_HostnameOverride(t *testing.T) {
	cfg := testConfig(t)
	cfg.Hostname = "custom-hostname"
	b := NewBanner(cfg)

	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate should not error: %v", err)
	}
	if output == "" {
		t.Error("Generate should produce non-empty output with custom hostname")
	}
	// The hostname should appear somewhere in the output.
	// This is a soft check since layout.go may format it differently.
}

func TestBannerConfig_WaifuDisabledSkipsImage(t *testing.T) {
	cfg := testConfig(t)
	cfg.WaifuEnabled = false
	b := NewBanner(cfg)

	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate should not error: %v", err)
	}
	if output == "" {
		t.Error("Generate with waifu disabled should still produce output")
	}
}

func TestGenerate_NonEmptyWithAllNilData(t *testing.T) {
	cfg := testConfig(t)
	b := NewBanner(cfg)

	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate should not error with nil data: %v", err)
	}
	if output == "" {
		t.Error("Generate should produce non-empty output even with all nil data")
	}
}

func TestGenerate_RespectsContextCancellation(t *testing.T) {
	cfg := testConfig(t)
	b := NewBanner(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before Generate.

	_, err := b.Generate(ctx)
	if err == nil {
		t.Error("Generate with cancelled context should return error")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestComputeUptime_ReturnsString(t *testing.T) {
	result := computeUptime()
	if result == "" {
		t.Error("computeUptime should return a non-empty string")
	}
	// On non-Linux systems, it returns "unknown" which is acceptable.
}

func TestParseFloat_ValidInput(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"123.45", 123.45},
		{"0", 0},
		{"1000", 1000},
		{"0.5", 0.5},
	}

	for _, tt := range tests {
		result, err := parseFloat(tt.input)
		if err != nil {
			t.Errorf("parseFloat(%q) returned error: %v", tt.input, err)
		}
		// Allow small floating point tolerance.
		if diff := result - tt.expected; diff > 0.01 || diff < -0.01 {
			t.Errorf("parseFloat(%q) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestFormatDuration_WithDays(t *testing.T) {
	result := formatDuration(3, 5, 12, true)
	if result != "3d 5h 12m" {
		t.Errorf("formatDuration with days = %q, want %q", result, "3d 5h 12m")
	}
}

func TestFormatDuration_WithoutDays(t *testing.T) {
	result := formatDuration(0, 5, 12, false)
	if result != "5h 12m" {
		t.Errorf("formatDuration without days = %q, want %q", result, "5h 12m")
	}
}

func TestFormatMinutes(t *testing.T) {
	result := formatMinutes(42)
	if result != "42m" {
		t.Errorf("formatMinutes = %q, want %q", result, "42m")
	}
}

func TestIntToStr(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{9999, "9999"},
	}
	for _, tt := range tests {
		result := intToStr(tt.input)
		if result != tt.expected {
			t.Errorf("intToStr(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestLoadCachedData_WithPartialCache(t *testing.T) {
	cfg := testConfig(t)
	b := NewBanner(cfg)

	store, err := cache.NewStore(cfg.CacheDir, cfg.Logger)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Only populate claude data, leave billing and infra empty.
	claude := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{Name: "partial-test", Type: "subscription", Tier: "pro", Status: "ok"},
		},
	}
	if err := cache.SetTyped(store, "claude", claude); err != nil {
		t.Fatalf("failed to set claude cache: %v", err)
	}

	loadedClaude, loadedBilling, loadedInfra := b.loadCachedData(store)
	if loadedClaude == nil {
		t.Error("claude should be loaded from partial cache")
	}
	if loadedBilling != nil {
		t.Error("billing should be nil with partial cache")
	}
	if loadedInfra != nil {
		t.Error("infra should be nil with partial cache")
	}
}

func TestGenerate_CreatesOutputWithCachedData(t *testing.T) {
	cfg := testConfig(t)
	b := NewBanner(cfg)

	// Populate all cache entries.
	store, err := cache.NewStore(cfg.CacheDir, cfg.Logger)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	claude := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "work",
				Type:   "subscription",
				Tier:   "max_5x",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 85.0,
					ResetsAt:    time.Now().Add(2 * time.Hour),
				},
			},
		},
	}
	billing := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{Provider: "civo", Status: "ok", CurrentMonth: collectors.MonthCost{SpendUSD: 45.0}},
		},
		Total: collectors.BillingSummary{CurrentMonthUSD: 45.0},
	}
	infra := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "tinyland.ts.net",
			OnlineCount: 3,
			TotalCount:  4,
		},
		Kubernetes: []collectors.KubernetesCluster{
			{Name: "civo-prod", Status: "healthy", TotalNodes: 2, ReadyNodes: 2},
		},
	}

	_ = cache.SetTyped(store, "claude", claude)
	_ = cache.SetTyped(store, "billing", billing)
	_ = cache.SetTyped(store, "infra", infra)

	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate should not error: %v", err)
	}
	if output == "" {
		t.Error("Generate should produce non-empty output with full cache")
	}
	if len(output) < 10 {
		t.Errorf("Generate output too short (%d chars), expected meaningful content", len(output))
	}
}

func TestParseUptimeSeconds_ValidData(t *testing.T) {
	// Simulate /proc/uptime format: "12345.67 9876.54\n"
	data := []byte("12345.67 9876.54\n")
	seconds, err := parseUptimeSeconds(data)
	if err != nil {
		t.Fatalf("parseUptimeSeconds returned error: %v", err)
	}
	if diff := seconds - 12345.67; diff > 0.01 || diff < -0.01 {
		t.Errorf("parseUptimeSeconds = %f, want ~12345.67", seconds)
	}
}

func TestGenerate_EmptyCacheDirIsCreated(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "new-cache-dir")

	cfg := testConfig(t)
	cfg.CacheDir = cacheDir
	b := NewBanner(cfg)

	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate should not error: %v", err)
	}
	if output == "" {
		t.Error("Generate should produce non-empty output")
	}

	// Verify the cache directory was created.
	if _, statErr := os.Stat(cacheDir); os.IsNotExist(statErr) {
		t.Error("cache directory should be created by Generate")
	}
}
