package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/config"
	"gitlab.com/tinyland/lab/prompt-pulse/display/banner"
	"gitlab.com/tinyland/lab/prompt-pulse/display/starship"
	"gitlab.com/tinyland/lab/prompt-pulse/status"
)

// integrationCollector returns configurable data for integration tests.
// It is named differently from mockCollector (defined in daemon_test.go)
// to avoid redefinition.
type integrationCollector struct {
	collectorName string
	collectorData interface{}
	collectorErr  error
}

func (ic *integrationCollector) Name() string        { return ic.collectorName }
func (ic *integrationCollector) Description() string  { return "integration test " + ic.collectorName }
func (ic *integrationCollector) Interval() time.Duration { return time.Minute }
func (ic *integrationCollector) Collect(ctx context.Context) (*collectors.CollectResult, error) {
	if ic.collectorErr != nil {
		return nil, ic.collectorErr
	}
	return &collectors.CollectResult{
		Collector: ic.collectorName,
		Timestamp: time.Now(),
		Data:      ic.collectorData,
	}, nil
}

// ---------------------------------------------------------------------------
// Realistic mock data helpers
// ---------------------------------------------------------------------------

// testClaudeUsage returns a ClaudeUsage with the given 5-hour utilization
// for a subscription account and an API account.
func testClaudeUsage(utilization float64) *collectors.ClaudeUsage {
	return &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "personal",
				Type:   "subscription",
				Tier:   "max_5x",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: utilization,
					ResetsAt:    time.Now().Add(2 * time.Hour),
				},
				SevenDay: &collectors.UsagePeriod{
					Utilization: utilization * 0.3,
					ResetsAt:    time.Now().Add(5 * 24 * time.Hour),
				},
			},
			{
				Name:   "work-api",
				Type:   "api",
				Tier:   "tier_4",
				Status: "ok",
				RateLimits: &collectors.APIRateLimits{
					RequestsLimit:     4000,
					RequestsRemaining: 3500,
					RequestsReset:     time.Now().Add(1 * time.Minute),
					TokensLimit:       400000,
					TokensRemaining:   350000,
					TokensReset:       time.Now().Add(1 * time.Minute),
				},
			},
		},
	}
}

// testBillingData returns a BillingData with the given total spend.
func testBillingData(spend float64) *collectors.BillingData {
	forecast := spend * 1.2
	budget := 200.0
	return &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider:     "civo",
				AccountName:  "Civo (production)",
				Status:       "ok",
				DashboardURL: "https://dashboard.civo.com",
				CurrentMonth: collectors.MonthCost{
					SpendUSD:    spend * 0.6,
					ForecastUSD: &forecast,
					BudgetUSD:   &budget,
					StartDate:   "2026-02-01",
					EndDate:     "2026-02-28",
				},
				FetchedAt: time.Now(),
			},
			{
				Provider:    "digitalocean",
				AccountName: "DigitalOcean",
				Status:      "ok",
				CurrentMonth: collectors.MonthCost{
					SpendUSD:  spend * 0.4,
					StartDate: "2026-02-01",
					EndDate:   "2026-02-28",
				},
				FetchedAt: time.Now(),
			},
		},
		Total: collectors.BillingSummary{
			CurrentMonthUSD: spend,
			ForecastUSD:     &forecast,
			BudgetUSD:       &budget,
		},
	}
}

// testInfraStatus returns an InfraStatus with the given online and total node counts.
func testInfraStatus(online, total int) *collectors.InfraStatus {
	nodes := make([]collectors.TailscaleNode, total)
	for i := 0; i < total; i++ {
		nodes[i] = collectors.TailscaleNode{
			Name:     fmt.Sprintf("node-%d", i),
			Hostname: fmt.Sprintf("node-%d.ts.net", i),
			IP:       fmt.Sprintf("100.64.0.%d", i+1),
			OS:       "linux",
			Online:   i < online,
			LastSeen: time.Now(),
		}
	}
	return &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "tinyland.ts.net",
			OnlineCount: online,
			TotalCount:  total,
			Nodes:       nodes,
		},
		Kubernetes: []collectors.KubernetesCluster{
			{
				Name:       "bitter-darkness",
				Platform:   "civo",
				Status:     "healthy",
				TotalNodes: 3,
				ReadyNodes: 3,
			},
		},
	}
}

// testLogger returns a quiet logger for test output.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// writeMinimalConfig writes a minimal valid config.yaml to dir and returns its path.
func writeMinimalConfig(t *testing.T, dir string) string {
	t.Helper()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := fmt.Sprintf(`daemon:
  poll_interval: "5m"
  cache_dir: %q
  log_file: %q
accounts:
  claude:
    - name: test
      type: subscription
      credentials_path: /tmp/fake-creds.json
      enabled: true
display:
  theme: monitoring
  enable_hyperlinks: false
`, filepath.Join(dir, "cache"), filepath.Join(dir, "prompt-pulse.log"))
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return cfgPath
}

// ---------------------------------------------------------------------------
// Integration tests
// ---------------------------------------------------------------------------

// TestIntegration_FullPipeline tests the complete pipeline:
// config -> cache populate -> banner render -> starship output.
func TestIntegration_FullPipeline(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()

	// Step 1-3: Write a minimal config and load it.
	cfgPath := writeMinimalConfig(t, tmpDir)
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Step 4: Create a cache store.
	cacheDir := cfg.Daemon.CacheDir
	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Step 5: Populate cache with realistic data.
	claudeData := testClaudeUsage(45.0)
	billingData := testBillingData(120.0)
	infraData := testInfraStatus(4, 5)

	if err := cache.SetTyped(store, "claude", claudeData); err != nil {
		t.Fatalf("SetTyped(claude): %v", err)
	}
	if err := cache.SetTyped(store, "billing", billingData); err != nil {
		t.Fatalf("SetTyped(billing): %v", err)
	}
	if err := cache.SetTyped(store, "infra", infraData); err != nil {
		t.Fatalf("SetTyped(infra): %v", err)
	}

	// Step 6: Verify starship output reads from cache correctly.
	ttl := parseDuration(cfg.Daemon.PollInterval)
	out, err := starship.NewOutput(starship.OutputConfig{
		CacheDir: cacheDir,
		CacheTTL: ttl,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("NewOutput: %v", err)
	}

	for _, mod := range []string{"claude", "billing", "infra"} {
		result, err := out.Module(mod)
		if err != nil {
			t.Errorf("Module(%q) error: %v", mod, err)
		}
		if result == "" {
			t.Errorf("Module(%q) returned empty string, expected content", mod)
		}
	}

	// Step 7: Verify banner generation produces non-empty output.
	bannerCfg := banner.BannerConfig{
		CacheDir:     cacheDir,
		CacheTTL:     ttl,
		WaifuEnabled: false,
		Hostname:     "integration-test",
		TermWidth:    80,
		TermHeight:   24,
		Logger:       logger,
	}
	b := banner.NewBanner(bannerCfg)
	ctx := context.Background()
	output, err := b.Generate(ctx)
	if err != nil {
		t.Fatalf("Banner.Generate: %v", err)
	}
	if output == "" {
		t.Error("Banner.Generate returned empty string")
	}

	// Step 8: Verify status evaluation.
	evaluator := status.NewEvaluator(status.DefaultEvaluatorConfig())
	sysStatus := evaluator.Evaluate(claudeData, billingData, infraData)
	if sysStatus.Overall != status.LevelHealthy {
		t.Errorf("status.Overall = %v, want Healthy (utilization 45%% is under threshold)", sysStatus.Overall)
	}
}

// TestIntegration_ConfigToCollectorsRoundTrip tests that config values
// correctly map to collector parameter types.
func TestIntegration_ConfigToCollectorsRoundTrip(t *testing.T) {
	cfg := &config.Config{
		Accounts: config.AccountsConfig{
			Claude: []config.ClaudeAccount{
				{
					Name:            "my-account",
					Type:            "subscription",
					CredentialsPath: "/home/user/.claude/.credentials.json",
					Enabled:         true,
				},
				{
					Name:      "api-account",
					Type:      "api",
					APIKeyEnv: "ANTHROPIC_API_KEY",
					Enabled:   false,
				},
			},
			Civo: config.CivoAccount{
				APIKeyEnv: "CIVO_API_KEY",
				Region:    "NYC1",
			},
			DigitalOcean: config.DigitalOceanAccount{
				APIKeyEnv: "DIGITALOCEAN_TOKEN",
			},
			DreamHost: config.DreamHostAccount{
				APIKeyEnv: "DREAMHOST_API_KEY",
			},
		},
		Tailscale: config.TailscaleConfig{
			Tailnet:        "example.ts.net",
			APIKeyEnv:      "TS_API_KEY_FOR_TEST",
			UseCLIFallback: true,
		},
		Kubernetes: config.KubernetesConfig{
			Contexts: []config.KubeContext{
				{
					Name:         "prod",
					Kubeconfig:   "/home/user/.kube/config",
					Namespace:    "default",
					DashboardURL: "https://k8s.example.com",
				},
			},
		},
	}

	// Claude accounts mapping.
	claudeAccounts := configToClaudeAccounts(cfg.Accounts.Claude)
	if len(claudeAccounts) != 2 {
		t.Fatalf("expected 2 claude accounts, got %d", len(claudeAccounts))
	}
	if claudeAccounts[0].Name != "my-account" {
		t.Errorf("claude[0].Name = %q, want %q", claudeAccounts[0].Name, "my-account")
	}
	if claudeAccounts[1].APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("claude[1].APIKeyEnv = %q, want %q", claudeAccounts[1].APIKeyEnv, "ANTHROPIC_API_KEY")
	}
	if claudeAccounts[1].Enabled {
		t.Error("claude[1].Enabled = true, want false")
	}

	// Billing providers mapping.
	providers := configToBillingProviders(cfg.Accounts)
	if len(providers) != 3 {
		t.Fatalf("expected 3 billing providers, got %d", len(providers))
	}
	byName := make(map[string]bool)
	for _, p := range providers {
		byName[p.Name] = p.Enabled
	}
	for _, expected := range []string{"civo", "digitalocean", "dreamhost"} {
		if !byName[expected] {
			t.Errorf("billing provider %q not found or not enabled", expected)
		}
	}

	// Infra config mapping.
	infraCfg := configToInfraConfig(cfg)
	if infraCfg.Tailnet != "example.ts.net" {
		t.Errorf("infraCfg.Tailnet = %q, want %q", infraCfg.Tailnet, "example.ts.net")
	}
	if !infraCfg.UseCLIFallback {
		t.Error("infraCfg.UseCLIFallback = false, want true")
	}
	if len(infraCfg.KubeContexts) != 1 {
		t.Fatalf("expected 1 kube context, got %d", len(infraCfg.KubeContexts))
	}
	if infraCfg.KubeContexts[0].Name != "prod" {
		t.Errorf("kube context name = %q, want %q", infraCfg.KubeContexts[0].Name, "prod")
	}
	if infraCfg.KubeContexts[0].DashboardURL != "https://k8s.example.com" {
		t.Errorf("kube context dashboard = %q, want %q", infraCfg.KubeContexts[0].DashboardURL, "https://k8s.example.com")
	}
}

// TestIntegration_CacheDataIntegrity tests that data written by collectors
// can be read back by display components with all fields preserved.
func TestIntegration_CacheDataIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()

	store, err := cache.NewStore(filepath.Join(tmpDir, "cache"), logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ttl := 5 * time.Minute

	// Write and read back ClaudeUsage.
	claudeIn := testClaudeUsage(72.5)
	if err := cache.SetTyped(store, "claude", claudeIn); err != nil {
		t.Fatalf("SetTyped(claude): %v", err)
	}
	claudeOut, fresh, err := cache.GetTyped[collectors.ClaudeUsage](store, "claude", ttl)
	if err != nil {
		t.Fatalf("GetTyped(claude): %v", err)
	}
	if !fresh {
		t.Error("claude data should be fresh immediately after writing")
	}
	if claudeOut == nil {
		t.Fatal("GetTyped(claude) returned nil")
	}
	if len(claudeOut.Accounts) != 2 {
		t.Fatalf("claude accounts = %d, want 2", len(claudeOut.Accounts))
	}
	if claudeOut.Accounts[0].FiveHour.Utilization != 72.5 {
		t.Errorf("claude 5h utilization = %f, want 72.5", claudeOut.Accounts[0].FiveHour.Utilization)
	}
	if claudeOut.Accounts[1].RateLimits.RequestsLimit != 4000 {
		t.Errorf("claude API requests limit = %d, want 4000", claudeOut.Accounts[1].RateLimits.RequestsLimit)
	}

	// Verify StarshipOutput on deserialized data.
	starshipOut := claudeOut.StarshipOutput()
	if starshipOut == "" {
		t.Error("ClaudeUsage.StarshipOutput() returned empty string")
	}
	if !strings.Contains(starshipOut, "personal") {
		t.Errorf("StarshipOutput missing account name 'personal': %q", starshipOut)
	}

	// Write and read back BillingData.
	billingIn := testBillingData(150.0)
	if err := cache.SetTyped(store, "billing", billingIn); err != nil {
		t.Fatalf("SetTyped(billing): %v", err)
	}
	billingOut, fresh, err := cache.GetTyped[collectors.BillingData](store, "billing", ttl)
	if err != nil {
		t.Fatalf("GetTyped(billing): %v", err)
	}
	if !fresh {
		t.Error("billing data should be fresh")
	}
	if billingOut == nil {
		t.Fatal("GetTyped(billing) returned nil")
	}
	if billingOut.Total.CurrentMonthUSD != 150.0 {
		t.Errorf("billing total = %f, want 150.0", billingOut.Total.CurrentMonthUSD)
	}
	billingStarship := billingOut.StarshipOutput()
	if billingStarship == "" {
		t.Error("BillingData.StarshipOutput() returned empty string")
	}
	if !strings.Contains(billingStarship, "$150") {
		t.Errorf("billing starship missing '$150': %q", billingStarship)
	}

	// Write and read back InfraStatus.
	infraIn := testInfraStatus(3, 5)
	if err := cache.SetTyped(store, "infra", infraIn); err != nil {
		t.Fatalf("SetTyped(infra): %v", err)
	}
	infraOut, fresh, err := cache.GetTyped[collectors.InfraStatus](store, "infra", ttl)
	if err != nil {
		t.Fatalf("GetTyped(infra): %v", err)
	}
	if !fresh {
		t.Error("infra data should be fresh")
	}
	if infraOut == nil {
		t.Fatal("GetTyped(infra) returned nil")
	}
	if infraOut.Tailscale.OnlineCount != 3 {
		t.Errorf("infra online count = %d, want 3", infraOut.Tailscale.OnlineCount)
	}
	infraStarship := infraOut.StarshipOutput()
	if infraStarship == "" {
		t.Error("InfraStatus.StarshipOutput() returned empty string")
	}
	if !strings.Contains(infraStarship, "ts:3/5") {
		t.Errorf("infra starship missing 'ts:3/5': %q", infraStarship)
	}
}

// TestIntegration_StatusToBanner tests that a warning status evaluation
// correctly feeds into banner generation with warning indicators.
func TestIntegration_StatusToBanner(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	cacheDir := filepath.Join(tmpDir, "cache")

	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// 85% utilization is above the default warning threshold of 80%.
	claudeData := testClaudeUsage(85.0)
	billingData := testBillingData(100.0)
	infraData := testInfraStatus(4, 5)

	if err := cache.SetTyped(store, "claude", claudeData); err != nil {
		t.Fatalf("SetTyped(claude): %v", err)
	}
	if err := cache.SetTyped(store, "billing", billingData); err != nil {
		t.Fatalf("SetTyped(billing): %v", err)
	}
	if err := cache.SetTyped(store, "infra", infraData); err != nil {
		t.Fatalf("SetTyped(infra): %v", err)
	}

	// Evaluate status -- should be Warning.
	evaluator := status.NewEvaluator(status.DefaultEvaluatorConfig())
	sysStatus := evaluator.Evaluate(claudeData, billingData, infraData)
	if sysStatus.Overall != status.LevelWarning {
		t.Errorf("status.Overall = %v, want Warning", sysStatus.Overall)
	}

	// Generate banner and verify it contains warning-related content.
	bannerCfg := banner.BannerConfig{
		CacheDir:     cacheDir,
		CacheTTL:     5 * time.Minute,
		WaifuEnabled: false,
		Hostname:     "test-host",
		TermWidth:    80,
		TermHeight:   24,
		Logger:       logger,
	}
	b := banner.NewBanner(bannerCfg)
	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Banner.Generate: %v", err)
	}
	if output == "" {
		t.Error("Banner.Generate returned empty string")
	}
	// The banner should contain the host name and data content.
	if !strings.Contains(output, "test-host") {
		t.Errorf("banner output missing hostname 'test-host'")
	}
}

// TestIntegration_StatusToBanner_Critical tests that critical data
// (high usage + offline K8s) produces a critical status.
func TestIntegration_StatusToBanner_Critical(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	cacheDir := filepath.Join(tmpDir, "cache")

	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// 98% utilization is above critical threshold (95%).
	claudeData := testClaudeUsage(98.0)
	billingData := testBillingData(250.0) // Over budget (200.0)
	infraData := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "tinyland.ts.net",
			OnlineCount: 0,
			TotalCount:  5,
			Nodes:       make([]collectors.TailscaleNode, 5),
		},
		Kubernetes: []collectors.KubernetesCluster{
			{
				Name:       "bitter-darkness",
				Platform:   "civo",
				Status:     "offline",
				TotalNodes: 3,
				ReadyNodes: 0,
			},
		},
	}

	if err := cache.SetTyped(store, "claude", claudeData); err != nil {
		t.Fatalf("SetTyped(claude): %v", err)
	}
	if err := cache.SetTyped(store, "billing", billingData); err != nil {
		t.Fatalf("SetTyped(billing): %v", err)
	}
	if err := cache.SetTyped(store, "infra", infraData); err != nil {
		t.Fatalf("SetTyped(infra): %v", err)
	}

	evaluator := status.NewEvaluator(status.DefaultEvaluatorConfig())
	sysStatus := evaluator.Evaluate(claudeData, billingData, infraData)
	if sysStatus.Overall != status.LevelCritical {
		t.Errorf("status.Overall = %v, want Critical", sysStatus.Overall)
	}

	// Banner should still generate without error.
	bannerCfg := banner.BannerConfig{
		CacheDir:     cacheDir,
		CacheTTL:     5 * time.Minute,
		WaifuEnabled: false,
		Hostname:     "critical-host",
		TermWidth:    80,
		TermHeight:   24,
		Logger:       logger,
	}
	b := banner.NewBanner(bannerCfg)
	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Banner.Generate: %v", err)
	}
	if output == "" {
		t.Error("Banner.Generate returned empty string for critical state")
	}
}

// TestIntegration_StatusToBanner_Healthy tests that all-healthy data
// produces a healthy status level.
func TestIntegration_StatusToBanner_Healthy(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	cacheDir := filepath.Join(tmpDir, "cache")

	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	claudeData := testClaudeUsage(25.0)
	billingData := testBillingData(50.0)
	infraData := testInfraStatus(5, 5)

	if err := cache.SetTyped(store, "claude", claudeData); err != nil {
		t.Fatalf("SetTyped(claude): %v", err)
	}
	if err := cache.SetTyped(store, "billing", billingData); err != nil {
		t.Fatalf("SetTyped(billing): %v", err)
	}
	if err := cache.SetTyped(store, "infra", infraData); err != nil {
		t.Fatalf("SetTyped(infra): %v", err)
	}

	evaluator := status.NewEvaluator(status.DefaultEvaluatorConfig())
	sysStatus := evaluator.Evaluate(claudeData, billingData, infraData)
	if sysStatus.Overall != status.LevelHealthy {
		t.Errorf("status.Overall = %v, want Healthy", sysStatus.Overall)
	}

	bannerCfg := banner.BannerConfig{
		CacheDir:     cacheDir,
		CacheTTL:     5 * time.Minute,
		WaifuEnabled: false,
		Hostname:     "healthy-host",
		TermWidth:    80,
		TermHeight:   24,
		Logger:       logger,
	}
	b := banner.NewBanner(bannerCfg)
	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Banner.Generate: %v", err)
	}
	if output == "" {
		t.Error("Banner.Generate returned empty string for healthy state")
	}
	if !strings.Contains(output, "healthy-host") {
		t.Errorf("banner output missing hostname 'healthy-host'")
	}
}

// TestIntegration_DaemonCycleWithMocks tests that a daemon runOnce pass
// with mock collectors writes the correct data to the cache.
func TestIntegration_DaemonCycleWithMocks(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	cacheDir := filepath.Join(tmpDir, "cache")

	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	claudeData := testClaudeUsage(60.0)
	billingData := testBillingData(80.0)
	infraData := testInfraStatus(3, 4)

	registry := collectors.NewRegistry()
	registry.Register(&integrationCollector{
		collectorName: "claude",
		collectorData: claudeData,
	})
	registry.Register(&integrationCollector{
		collectorName: "billing",
		collectorData: billingData,
	})
	registry.Register(&integrationCollector{
		collectorName: "infra",
		collectorData: infraData,
	})

	d := &daemon{
		config: &config.Config{
			Daemon: config.DaemonConfig{
				PollInterval: "5m",
				CacheDir:     cacheDir,
			},
		},
		logger:   logger,
		store:    store,
		registry: registry,
		pidFile:  filepath.Join(cacheDir, "test.pid"),
		lastRun:  make(map[string]time.Time),
	}

	ctx := context.Background()
	if err := d.runOnce(ctx); err != nil {
		t.Fatalf("runOnce: %v", err)
	}

	// Verify all three keys exist in cache.
	keys := store.Keys()
	expectedKeys := map[string]bool{"claude": false, "billing": false, "infra": false}
	for _, k := range keys {
		if _, ok := expectedKeys[k]; ok {
			expectedKeys[k] = true
		}
	}
	for key, found := range expectedKeys {
		if !found {
			t.Errorf("cache key %q not found after runOnce; keys = %v", key, keys)
		}
	}

	// Read back and verify the data.
	ttl := 5 * time.Minute
	claudeOut, _, err := cache.GetTyped[collectors.ClaudeUsage](store, "claude", ttl)
	if err != nil {
		t.Fatalf("GetTyped(claude): %v", err)
	}
	if claudeOut == nil {
		t.Fatal("claude data is nil after runOnce")
	}
	if len(claudeOut.Accounts) != 2 {
		t.Errorf("claude accounts = %d, want 2", len(claudeOut.Accounts))
	}

	billingOut, _, err := cache.GetTyped[collectors.BillingData](store, "billing", ttl)
	if err != nil {
		t.Fatalf("GetTyped(billing): %v", err)
	}
	if billingOut == nil {
		t.Fatal("billing data is nil after runOnce")
	}
	if billingOut.Total.CurrentMonthUSD != 80.0 {
		t.Errorf("billing total = %f, want 80.0", billingOut.Total.CurrentMonthUSD)
	}
}

// TestIntegration_StarshipModules tests that all three starship modules
// produce output from cached data.
func TestIntegration_StarshipModules(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	cacheDir := filepath.Join(tmpDir, "cache")

	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Populate cache.
	if err := cache.SetTyped(store, "claude", testClaudeUsage(50.0)); err != nil {
		t.Fatalf("SetTyped(claude): %v", err)
	}
	if err := cache.SetTyped(store, "billing", testBillingData(100.0)); err != nil {
		t.Fatalf("SetTyped(billing): %v", err)
	}
	if err := cache.SetTyped(store, "infra", testInfraStatus(4, 5)); err != nil {
		t.Fatalf("SetTyped(infra): %v", err)
	}

	out, err := starship.NewOutput(starship.OutputConfig{
		CacheDir: cacheDir,
		CacheTTL: 5 * time.Minute,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("NewOutput: %v", err)
	}

	tests := []struct {
		module   string
		contains string
	}{
		{"claude", "personal"},
		{"billing", "$100"},
		{"infra", "ts:4/5"},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			result, err := out.Module(tt.module)
			if err != nil {
				t.Fatalf("Module(%q): %v", tt.module, err)
			}
			if result == "" {
				t.Errorf("Module(%q) returned empty string", tt.module)
			}
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Module(%q) = %q, want it to contain %q", tt.module, result, tt.contains)
			}
		})
	}

	// Verify unknown module returns error.
	_, err = out.Module("nonexistent")
	if err == nil {
		t.Error("Module(nonexistent) should return error for unknown module")
	}
}

// TestIntegration_EmptyCache tests graceful behavior when the cache is empty.
func TestIntegration_EmptyCache(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	cacheDir := filepath.Join(tmpDir, "cache")

	// Create store but do not populate it.
	_, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Starship modules should return empty strings (cache miss).
	out, err := starship.NewOutput(starship.OutputConfig{
		CacheDir: cacheDir,
		CacheTTL: 5 * time.Minute,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("NewOutput: %v", err)
	}

	for _, mod := range []string{"claude", "billing", "infra"} {
		result, err := out.Module(mod)
		if err != nil {
			t.Errorf("Module(%q) error: %v", mod, err)
		}
		if result != "" {
			t.Errorf("Module(%q) = %q, want empty string for empty cache", mod, result)
		}
	}

	// Banner should still render without crashing.
	bannerCfg := banner.BannerConfig{
		CacheDir:     cacheDir,
		CacheTTL:     5 * time.Minute,
		WaifuEnabled: false,
		Hostname:     "empty-cache-host",
		TermWidth:    80,
		TermHeight:   24,
		Logger:       logger,
	}
	b := banner.NewBanner(bannerCfg)
	output, err := b.Generate(context.Background())
	if err != nil {
		t.Fatalf("Banner.Generate: %v", err)
	}
	if output == "" {
		t.Error("Banner.Generate returned empty string for empty cache")
	}

	// Status should evaluate to Unknown with nil data.
	evaluator := status.NewEvaluator(status.DefaultEvaluatorConfig())
	sysStatus := evaluator.Evaluate(nil, nil, nil)
	if sysStatus.Overall != status.LevelUnknown {
		t.Errorf("status.Overall = %v, want Unknown for nil data", sysStatus.Overall)
	}
}

// TestIntegration_StaleCache tests that stale cache data is handled correctly,
// including the staleness indicator in starship output.
func TestIntegration_StaleCache(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	cacheDir := filepath.Join(tmpDir, "cache")

	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Write data to cache.
	if err := cache.SetTyped(store, "claude", testClaudeUsage(40.0)); err != nil {
		t.Fatalf("SetTyped(claude): %v", err)
	}

	// Set the file modification time to the past (beyond TTL).
	cachePath := filepath.Join(cacheDir, "claude.json")
	pastTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(cachePath, pastTime, pastTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// Use a short TTL so the data is stale.
	shortTTL := 5 * time.Minute

	// GetTyped should return the data but mark it as not fresh.
	claudeOut, fresh, err := cache.GetTyped[collectors.ClaudeUsage](store, "claude", shortTTL)
	if err != nil {
		t.Fatalf("GetTyped(claude): %v", err)
	}
	if claudeOut == nil {
		t.Fatal("stale data should still be returned, got nil")
	}
	if fresh {
		t.Error("data should be stale (fresh=false) after Chtimes to the past")
	}

	// Starship output should include the stale indicator " ?".
	out, err := starship.NewOutput(starship.OutputConfig{
		CacheDir: cacheDir,
		CacheTTL: shortTTL,
		Logger:   logger,
	})
	if err != nil {
		t.Fatalf("NewOutput: %v", err)
	}
	result, err := out.Module("claude")
	if err != nil {
		t.Fatalf("Module(claude): %v", err)
	}
	if result == "" {
		t.Error("stale data should still produce output, got empty string")
	}
	if !strings.HasSuffix(result, " ?") {
		t.Errorf("stale starship output should end with ' ?', got %q", result)
	}
}

// TestIntegration_ConfigDefaultsWork tests that DefaultConfig produces
// a valid, usable configuration.
func TestIntegration_ConfigDefaultsWork(t *testing.T) {
	cfg := config.DefaultConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() error: %v", err)
	}

	// Config converter functions should not panic with default values.
	claudeAccounts := configToClaudeAccounts(cfg.Accounts.Claude)
	if len(claudeAccounts) == 0 {
		t.Error("DefaultConfig should have at least one Claude account")
	}

	providers := configToBillingProviders(cfg.Accounts)
	// Default config has API key envs set, so all 4 providers should be present.
	if len(providers) < 3 {
		t.Errorf("expected at least 3 billing providers from defaults, got %d", len(providers))
	}

	infraCfg := configToInfraConfig(cfg)
	if !infraCfg.UseCLIFallback {
		t.Error("DefaultConfig should have UseCLIFallback = true")
	}
}

// TestIntegration_MultipleDaemonCycles tests that multiple runOnce cycles
// produce correct cache state with the most recent data.
func TestIntegration_MultipleDaemonCycles(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testLogger()
	cacheDir := filepath.Join(tmpDir, "cache")

	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Use a collector that tracks call count to return different data each time.
	callCount := 0
	claudeCollector := &integrationCollector{
		collectorName: "claude",
	}

	registry := collectors.NewRegistry()
	registry.Register(claudeCollector)

	d := &daemon{
		config: &config.Config{
			Daemon: config.DaemonConfig{
				PollInterval: "5m",
				CacheDir:     cacheDir,
			},
		},
		logger:   logger,
		store:    store,
		registry: registry,
		pidFile:  filepath.Join(cacheDir, "test.pid"),
		lastRun:  make(map[string]time.Time),
	}

	ctx := context.Background()

	// Run 3 cycles, updating data each time.
	for i := 0; i < 3; i++ {
		callCount++
		utilization := float64(callCount * 20)
		claudeCollector.collectorData = testClaudeUsage(utilization)

		// Clear lastRun so the collector runs every cycle regardless of interval.
		d.mu.Lock()
		d.lastRun = make(map[string]time.Time)
		d.mu.Unlock()

		if err := d.runOnce(ctx); err != nil {
			t.Fatalf("runOnce (cycle %d): %v", i+1, err)
		}
	}

	// Verify the cache has the data from the last cycle (60% utilization).
	ttl := 5 * time.Minute
	claudeOut, _, err := cache.GetTyped[collectors.ClaudeUsage](store, "claude", ttl)
	if err != nil {
		t.Fatalf("GetTyped(claude): %v", err)
	}
	if claudeOut == nil {
		t.Fatal("claude data is nil after 3 runOnce cycles")
	}
	if len(claudeOut.Accounts) == 0 {
		t.Fatal("no claude accounts after 3 cycles")
	}
	expected := 60.0
	actual := claudeOut.Accounts[0].FiveHour.Utilization
	if actual != expected {
		t.Errorf("claude 5h utilization = %f after 3 cycles, want %f (most recent)", actual, expected)
	}
}
