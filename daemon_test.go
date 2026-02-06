package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors/billing"
	"gitlab.com/tinyland/lab/prompt-pulse/config"
)

func TestConfigToClaudeAccounts(t *testing.T) {
	input := []config.ClaudeAccount{
		{
			Name:            "personal",
			Type:            "subscription",
			CredentialsPath: "/home/user/.claude/.credentials.json",
			APIKeyEnv:       "",
			Enabled:         true,
		},
		{
			Name:            "work",
			Type:            "api",
			CredentialsPath: "",
			APIKeyEnv:       "ANTHROPIC_API_KEY",
			Enabled:         false,
		},
	}

	result := configToClaudeAccounts(input)

	if len(result) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(result))
	}

	// Verify first account.
	if result[0].Name != "personal" {
		t.Errorf("result[0].Name = %q, want %q", result[0].Name, "personal")
	}
	if result[0].Type != "subscription" {
		t.Errorf("result[0].Type = %q, want %q", result[0].Type, "subscription")
	}
	if result[0].CredentialsPath != "/home/user/.claude/.credentials.json" {
		t.Errorf("result[0].CredentialsPath = %q, want %q", result[0].CredentialsPath, "/home/user/.claude/.credentials.json")
	}
	if !result[0].Enabled {
		t.Error("result[0].Enabled = false, want true")
	}

	// Verify second account.
	if result[1].Name != "work" {
		t.Errorf("result[1].Name = %q, want %q", result[1].Name, "work")
	}
	if result[1].Type != "api" {
		t.Errorf("result[1].Type = %q, want %q", result[1].Type, "api")
	}
	if result[1].APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("result[1].APIKeyEnv = %q, want %q", result[1].APIKeyEnv, "ANTHROPIC_API_KEY")
	}
	if result[1].Enabled {
		t.Error("result[1].Enabled = true, want false")
	}
}

func TestConfigToClaudeAccounts_Empty(t *testing.T) {
	result := configToClaudeAccounts(nil)
	if len(result) != 0 {
		t.Fatalf("expected 0 accounts, got %d", len(result))
	}
}

func TestConfigToBillingProviders(t *testing.T) {
	accounts := config.AccountsConfig{
		Civo: config.CivoAccount{
			APIKeyEnv: "CIVO_API_KEY",
			Region:    "NYC1",
		},
		DigitalOcean: config.DigitalOceanAccount{
			APIKeyEnv: "DIGITALOCEAN_TOKEN",
		},
		AWS: config.AWSAccount{
			Profile: "default",
			Regions: []string{"us-east-1"},
		},
		DreamHost: config.DreamHostAccount{
			APIKeyEnv: "DREAMHOST_API_KEY",
		},
	}

	result := configToBillingProviders(accounts)

	if len(result) != 4 {
		t.Fatalf("expected 4 providers, got %d", len(result))
	}

	// Build a map for easier lookup.
	byName := make(map[string]billing.ProviderConfig)
	for _, p := range result {
		byName[p.Name] = p
	}

	// Verify each provider is present and enabled.
	for _, name := range []string{"civo", "digitalocean", "aws", "dreamhost"} {
		p, ok := byName[name]
		if !ok {
			t.Errorf("provider %q not found in result", name)
			continue
		}
		if !p.Enabled {
			t.Errorf("provider %q.Enabled = false, want true", name)
		}
	}

	// Verify specific API key env mappings.
	if byName["civo"].APIKeyEnv != "CIVO_API_KEY" {
		t.Errorf("civo APIKeyEnv = %q, want %q", byName["civo"].APIKeyEnv, "CIVO_API_KEY")
	}
	if byName["digitalocean"].APIKeyEnv != "DIGITALOCEAN_TOKEN" {
		t.Errorf("digitalocean APIKeyEnv = %q, want %q", byName["digitalocean"].APIKeyEnv, "DIGITALOCEAN_TOKEN")
	}
	if byName["dreamhost"].APIKeyEnv != "DREAMHOST_API_KEY" {
		t.Errorf("dreamhost APIKeyEnv = %q, want %q", byName["dreamhost"].APIKeyEnv, "DREAMHOST_API_KEY")
	}
}

func TestConfigToBillingProviders_EmptyFields(t *testing.T) {
	// When no API key envs are set, no providers should be returned.
	accounts := config.AccountsConfig{}

	result := configToBillingProviders(accounts)
	if len(result) != 0 {
		t.Fatalf("expected 0 providers for empty config, got %d", len(result))
	}
}

func TestConfigToInfraConfig(t *testing.T) {
	// Set a test environment variable for Tailscale.
	const testEnvKey = "TEST_PP_TAILSCALE_KEY"
	t.Setenv(testEnvKey, "tskey-test-123")

	cfg := &config.Config{
		Tailscale: config.TailscaleConfig{
			Tailnet:        "example.ts.net",
			APIKeyEnv:      testEnvKey,
			UseCLIFallback: true,
		},
		Kubernetes: config.KubernetesConfig{
			Contexts: []config.KubeContext{
				{
					Name:         "prod-civo",
					Kubeconfig:   "/home/user/.kube/civo.yaml",
					Namespace:    "default",
					DashboardURL: "https://dashboard.civo.com",
				},
				{
					Name:       "dev-local",
					Kubeconfig: "/home/user/.kube/config",
					Namespace:  "dev",
				},
			},
		},
	}

	result := configToInfraConfig(cfg)

	if result.Tailnet != "example.ts.net" {
		t.Errorf("Tailnet = %q, want %q", result.Tailnet, "example.ts.net")
	}
	if result.TailscaleAPIKey != "tskey-test-123" {
		t.Errorf("TailscaleAPIKey = %q, want %q", result.TailscaleAPIKey, "tskey-test-123")
	}
	if !result.UseCLIFallback {
		t.Error("UseCLIFallback = false, want true")
	}
	if len(result.KubeContexts) != 2 {
		t.Fatalf("expected 2 kube contexts, got %d", len(result.KubeContexts))
	}
	if result.KubeContexts[0].Name != "prod-civo" {
		t.Errorf("KubeContexts[0].Name = %q, want %q", result.KubeContexts[0].Name, "prod-civo")
	}
	if result.KubeContexts[0].Kubeconfig != "/home/user/.kube/civo.yaml" {
		t.Errorf("KubeContexts[0].Kubeconfig = %q, want %q", result.KubeContexts[0].Kubeconfig, "/home/user/.kube/civo.yaml")
	}
	if result.KubeContexts[0].DashboardURL != "https://dashboard.civo.com" {
		t.Errorf("KubeContexts[0].DashboardURL = %q, want %q", result.KubeContexts[0].DashboardURL, "https://dashboard.civo.com")
	}
	if result.KubeContexts[1].Namespace != "dev" {
		t.Errorf("KubeContexts[1].Namespace = %q, want %q", result.KubeContexts[1].Namespace, "dev")
	}
}

func TestConfigToInfraConfig_NoAPIKey(t *testing.T) {
	cfg := &config.Config{
		Tailscale: config.TailscaleConfig{
			Tailnet:        "example.ts.net",
			APIKeyEnv:      "NONEXISTENT_ENV_VAR_PROMPT_PULSE_TEST",
			UseCLIFallback: false,
		},
	}

	// Ensure the env var does not exist.
	os.Unsetenv("NONEXISTENT_ENV_VAR_PROMPT_PULSE_TEST")

	result := configToInfraConfig(cfg)

	if result.TailscaleAPIKey != "" {
		t.Errorf("TailscaleAPIKey = %q, want empty string", result.TailscaleAPIKey)
	}
}

func TestNewDaemon(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Daemon: config.DaemonConfig{
			PollInterval: "5m",
			CacheDir:     filepath.Join(tmpDir, "cache"),
			LogFile:      filepath.Join(tmpDir, "test.log"),
		},
		Accounts: config.AccountsConfig{
			Claude: []config.ClaudeAccount{
				{
					Name:            "test-account",
					Type:            "subscription",
					CredentialsPath: "/tmp/fake-creds.json",
					Enabled:         true,
				},
			},
			Civo: config.CivoAccount{
				APIKeyEnv: "CIVO_API_KEY",
			},
		},
		Tailscale: config.TailscaleConfig{
			UseCLIFallback: true,
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	d, err := newDaemon(cfg, logger)
	if err != nil {
		t.Fatalf("newDaemon() error: %v", err)
	}

	if d.config != cfg {
		t.Error("daemon.config does not match input")
	}
	if d.store == nil {
		t.Error("daemon.store is nil")
	}
	if d.registry == nil {
		t.Error("daemon.registry is nil")
	}

	// Verify all four collectors are registered.
	all := d.registry.All()
	if len(all) != 4 {
		t.Fatalf("expected 4 collectors registered, got %d", len(all))
	}

	names := make(map[string]bool)
	for _, c := range all {
		names[c.Name()] = true
	}

	for _, expected := range []string{"claude", "billing", "infra", "fastfetch"} {
		if !names[expected] {
			t.Errorf("collector %q not registered", expected)
		}
	}
}

// mockCollector is a test collector that returns static data.
type mockCollector struct {
	name string
	data interface{}
	err  error
}

func (m *mockCollector) Name() string                  { return m.name }
func (m *mockCollector) Description() string            { return "mock " + m.name }
func (m *mockCollector) Interval() time.Duration        { return time.Minute }
func (m *mockCollector) Collect(ctx context.Context) (*collectors.CollectResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &collectors.CollectResult{
		Collector: m.name,
		Timestamp: time.Now(),
		Data:      m.data,
	}, nil
}

func TestDaemon_RunOnce(t *testing.T) {
	tmpDir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	store, err := cache.NewStore(filepath.Join(tmpDir, "cache"), logger)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	registry := collectors.NewRegistry()
	registry.Register(&mockCollector{
		name: "test-collector",
		data: map[string]string{"status": "ok"},
	})

	d := &daemon{
		config: &config.Config{
			Daemon: config.DaemonConfig{
				PollInterval: "5m",
				CacheDir:     filepath.Join(tmpDir, "cache"),
			},
		},
		logger:   logger,
		store:    store,
		registry: registry,
		lastRun:  make(map[string]time.Time),
	}

	ctx := context.Background()
	if err := d.runOnce(ctx); err != nil {
		t.Fatalf("runOnce() error: %v", err)
	}

	// Verify the data was written to cache.
	keys := store.Keys()
	found := false
	for _, k := range keys {
		if k == "test-collector" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("cache key %q not found after runOnce; keys = %v", "test-collector", keys)
	}
}

func TestDaemon_RunOnce_CollectorError(t *testing.T) {
	tmpDir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	store, err := cache.NewStore(filepath.Join(tmpDir, "cache"), logger)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	registry := collectors.NewRegistry()
	registry.Register(&mockCollector{
		name: "failing-collector",
		err:  context.DeadlineExceeded,
	})
	registry.Register(&mockCollector{
		name: "succeeding-collector",
		data: map[string]string{"status": "ok"},
	})

	d := &daemon{
		config: &config.Config{
			Daemon: config.DaemonConfig{
				PollInterval: "5m",
				CacheDir:     filepath.Join(tmpDir, "cache"),
			},
		},
		logger:   logger,
		store:    store,
		registry: registry,
		lastRun:  make(map[string]time.Time),
	}

	ctx := context.Background()
	// runOnce should not return an error even if one collector fails.
	if err := d.runOnce(ctx); err != nil {
		t.Fatalf("runOnce() error: %v", err)
	}

	// The succeeding collector should have written its data.
	keys := store.Keys()
	found := false
	for _, k := range keys {
		if k == "succeeding-collector" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("cache key %q not found after runOnce; keys = %v", "succeeding-collector", keys)
	}

	// The failing collector should not have written anything.
	for _, k := range keys {
		if k == "failing-collector" {
			t.Error("failing collector should not have written to cache")
		}
	}
}

// mockSlowCollector is a test collector that sleeps for a specified duration
// before returning data. Used to verify concurrent execution.
type mockSlowCollector struct {
	name     string
	data     interface{}
	duration time.Duration
	interval time.Duration
	err      error
}

func (m *mockSlowCollector) Name() string           { return m.name }
func (m *mockSlowCollector) Description() string     { return "slow mock " + m.name }
func (m *mockSlowCollector) Interval() time.Duration {
	if m.interval > 0 {
		return m.interval
	}
	return time.Minute
}
func (m *mockSlowCollector) Collect(ctx context.Context) (*collectors.CollectResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	select {
	case <-time.After(m.duration):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &collectors.CollectResult{
		Collector: m.name,
		Timestamp: time.Now(),
		Data:      m.data,
	}, nil
}

// newTestDaemon creates a daemon for testing with a temporary directory,
// a store, and empty registry. The caller can register collectors as needed.
func newTestDaemon(t *testing.T) (*daemon, string) {
	t.Helper()
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	d := &daemon{
		config: &config.Config{
			Daemon: config.DaemonConfig{
				PollInterval: "5m",
				CacheDir:     cacheDir,
			},
		},
		logger:   logger,
		store:    store,
		registry: collectors.NewRegistry(),
		pidFile:  filepath.Join(cacheDir, "prompt-pulse.pid"),
		lastRun:  make(map[string]time.Time),
	}

	return d, tmpDir
}

func TestDaemon_WritePIDFile(t *testing.T) {
	d, _ := newTestDaemon(t)

	if err := d.writePIDFile(); err != nil {
		t.Fatalf("writePIDFile() error: %v", err)
	}

	data, err := os.ReadFile(d.pidFile)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("PID file contains non-integer: %q", string(data))
	}

	if pid != os.Getpid() {
		t.Errorf("PID file contains %d, want %d", pid, os.Getpid())
	}
}

func TestDaemon_RemovePIDFile(t *testing.T) {
	d, _ := newTestDaemon(t)

	if err := d.writePIDFile(); err != nil {
		t.Fatalf("writePIDFile() error: %v", err)
	}

	// Verify the file exists.
	if _, err := os.Stat(d.pidFile); err != nil {
		t.Fatalf("PID file does not exist after write: %v", err)
	}

	d.removePIDFile()

	// Verify the file is gone.
	if _, err := os.Stat(d.pidFile); !os.IsNotExist(err) {
		t.Errorf("PID file still exists after removePIDFile(); err = %v", err)
	}
}

func TestDaemon_IsRunning_NoFile(t *testing.T) {
	d, _ := newTestDaemon(t)

	running, pid := d.isRunning()
	if running {
		t.Errorf("isRunning() = true, want false (no PID file)")
	}
	if pid != 0 {
		t.Errorf("isRunning() pid = %d, want 0", pid)
	}
}

func TestDaemon_IsRunning_CurrentProcess(t *testing.T) {
	d, _ := newTestDaemon(t)

	// Write current process PID.
	if err := d.writePIDFile(); err != nil {
		t.Fatalf("writePIDFile() error: %v", err)
	}

	running, pid := d.isRunning()
	if !running {
		t.Error("isRunning() = false, want true (current process is running)")
	}
	if pid != os.Getpid() {
		t.Errorf("isRunning() pid = %d, want %d", pid, os.Getpid())
	}
}

func TestDaemon_IsRunning_StaleProcess(t *testing.T) {
	d, _ := newTestDaemon(t)

	// Write a PID that almost certainly does not exist.
	// Use a very high PID that is unlikely to be a real process.
	stalePID := 4999999
	if err := os.MkdirAll(filepath.Dir(d.pidFile), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(d.pidFile, []byte(strconv.Itoa(stalePID)), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	running, pid := d.isRunning()
	if running {
		t.Errorf("isRunning() = true, want false (stale PID %d)", stalePID)
	}
	if pid != 0 {
		t.Errorf("isRunning() pid = %d, want 0 for stale process", pid)
	}

	// Verify the stale PID file was cleaned up.
	if _, err := os.Stat(d.pidFile); !os.IsNotExist(err) {
		t.Error("stale PID file was not cleaned up")
	}
}

func TestDaemon_RunOnce_Concurrent(t *testing.T) {
	d, _ := newTestDaemon(t)

	sleepDuration := 50 * time.Millisecond

	// Register 3 slow collectors that each sleep for sleepDuration.
	for i := 0; i < 3; i++ {
		d.registry.Register(&mockSlowCollector{
			name:     fmt.Sprintf("slow-%d", i),
			data:     map[string]string{"status": "ok"},
			duration: sleepDuration,
		})
	}

	start := time.Now()
	ctx := context.Background()
	if err := d.runOnce(ctx); err != nil {
		t.Fatalf("runOnce() error: %v", err)
	}
	elapsed := time.Since(start)

	// If executed concurrently, total time should be around sleepDuration
	// (not 3x sleepDuration). Allow generous margin for CI/slow machines.
	maxExpected := sleepDuration * 2
	if elapsed > maxExpected {
		t.Errorf("runOnce() took %v, want < %v (concurrent execution expected)", elapsed, maxExpected)
	}
}

func TestDaemon_RunOnce_PerCollectorInterval(t *testing.T) {
	d, _ := newTestDaemon(t)

	// Register two collectors with a 1-minute interval.
	d.registry.Register(&mockCollector{
		name: "fresh-collector",
		data: map[string]string{"status": "fresh"},
	})
	d.registry.Register(&mockCollector{
		name: "stale-collector",
		data: map[string]string{"status": "stale"},
	})

	// Mark "fresh-collector" as having just run (should be skipped).
	d.lastRun["fresh-collector"] = time.Now()

	// Mark "stale-collector" as not having run recently (should execute).
	// (Not setting lastRun means it will run.)

	ctx := context.Background()
	if err := d.runOnce(ctx); err != nil {
		t.Fatalf("runOnce() error: %v", err)
	}

	// Verify stale-collector ran (written to cache).
	keys := d.store.Keys()
	staleFound := false
	freshFound := false
	for _, k := range keys {
		if k == "stale-collector" {
			staleFound = true
		}
		if k == "fresh-collector" {
			freshFound = true
		}
	}

	if !staleFound {
		t.Error("stale-collector should have been collected but was not found in cache")
	}
	if freshFound {
		t.Error("fresh-collector should have been skipped but was found in cache")
	}
}

func TestDaemon_Run_AlreadyRunning(t *testing.T) {
	d, _ := newTestDaemon(t)

	// Write PID file with current process PID to simulate already running.
	if err := d.writePIDFile(); err != nil {
		t.Fatalf("writePIDFile() error: %v", err)
	}
	defer d.removePIDFile()

	ctx := context.Background()
	err := d.run(ctx)
	if err == nil {
		t.Fatal("run() should return an error when daemon is already running")
	}

	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("run() error = %q, want error containing 'already running'", err.Error())
	}
}

func TestDaemon_Shutdown(t *testing.T) {
	d, _ := newTestDaemon(t)

	// shutdown should not panic even with an empty cache.
	d.shutdown()
}

func TestDaemon_CollectOne_Success(t *testing.T) {
	d, _ := newTestDaemon(t)

	col := &mockCollector{
		name: "success-collector",
		data: map[string]string{"status": "ok"},
	}

	ctx := context.Background()
	d.collectOne(ctx, col)

	// Verify the data was written to cache.
	keys := d.store.Keys()
	found := false
	for _, k := range keys {
		if k == "success-collector" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("cache key %q not found after collectOne; keys = %v", "success-collector", keys)
	}

	// Verify lastRun was updated.
	d.mu.Lock()
	lastRun, ok := d.lastRun["success-collector"]
	d.mu.Unlock()
	if !ok {
		t.Error("lastRun not set for success-collector")
	}
	if time.Since(lastRun) > time.Second {
		t.Errorf("lastRun is too old: %v ago", time.Since(lastRun))
	}
}

func TestDaemon_CollectOne_Error(t *testing.T) {
	d, _ := newTestDaemon(t)

	col := &mockCollector{
		name: "error-collector",
		err:  fmt.Errorf("simulated failure"),
	}

	ctx := context.Background()
	// collectOne should not panic on error.
	d.collectOne(ctx, col)

	// Verify no data was written to cache.
	keys := d.store.Keys()
	for _, k := range keys {
		if k == "error-collector" {
			t.Error("error-collector should not have written to cache")
		}
	}

	// Verify lastRun was NOT updated.
	d.mu.Lock()
	_, ok := d.lastRun["error-collector"]
	d.mu.Unlock()
	if ok {
		t.Error("lastRun should not be set for error-collector")
	}
}

