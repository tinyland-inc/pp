package integration

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/config"
)

// mockCollector is a test collector that returns static data.
type mockCollector struct {
	name        string
	description string
	interval    time.Duration
	data        interface{}
	err         error
}

func (m *mockCollector) Name() string                  { return m.name }
func (m *mockCollector) Description() string            { return m.description }
func (m *mockCollector) Interval() time.Duration        { return m.interval }
func (m *mockCollector) Collect(_ context.Context) (*collectors.CollectResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &collectors.CollectResult{
		Collector: m.name,
		Timestamp: time.Now(),
		Data:      m.data,
	}, nil
}

// TestAllCollectorsRegistered verifies that the expected 5 collector names are
// consistent with the Registry API. This test does not instantiate the actual
// daemon (which requires config file I/O) but validates the Registry correctly
// holds and returns collectors by name.
func TestAllCollectorsRegistered(t *testing.T) {
	registry := collectors.NewRegistry()

	expectedCollectors := []struct {
		name        string
		description string
	}{
		{"claude", "Claude AI usage metrics"},
		{"billing", "Cloud provider billing data"},
		{"infra", "Infrastructure status (Tailscale + K8s)"},
		{"fastfetch", "System info via fastfetch"},
		{"sysmetrics", "Live system metrics (CPU, RAM, Disk)"},
	}

	for _, ec := range expectedCollectors {
		registry.Register(&mockCollector{
			name:        ec.name,
			description: ec.description,
			interval:    time.Minute,
			data:        map[string]string{"status": "ok"},
		})
	}

	all := registry.All()
	if len(all) != 5 {
		t.Fatalf("expected 5 collectors registered, got %d", len(all))
	}

	// Verify each collector can be retrieved by name.
	for _, ec := range expectedCollectors {
		c, found := registry.Get(ec.name)
		if !found {
			t.Errorf("collector %q not found in registry", ec.name)
			continue
		}
		if c.Name() != ec.name {
			t.Errorf("collector name = %q, want %q", c.Name(), ec.name)
		}
	}
}

// TestRegistryReplacesDuplicates verifies that registering a collector with the
// same name replaces the existing one.
func TestRegistryReplacesDuplicates(t *testing.T) {
	registry := collectors.NewRegistry()

	// Register first version.
	registry.Register(&mockCollector{
		name:        "test",
		description: "version 1",
		interval:    time.Minute,
	})

	// Register replacement.
	registry.Register(&mockCollector{
		name:        "test",
		description: "version 2",
		interval:    2 * time.Minute,
	})

	all := registry.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 collector after replacement, got %d", len(all))
	}
	if all[0].Description() != "version 2" {
		t.Errorf("expected replaced collector, got description %q", all[0].Description())
	}
}

// TestHealthFileWrittenAfterCollection validates that the health file
// is properly written and contains the expected structure.
func TestHealthFileWrittenAfterCollection(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	registry := collectors.NewRegistry()
	registry.Register(&mockCollector{
		name:     "claude",
		interval: time.Minute,
		data:     map[string]string{"status": "ok"},
	})
	registry.Register(&mockCollector{
		name:     "billing",
		interval: time.Minute,
		data:     map[string]string{"status": "ok"},
	})

	// Simulate a poll cycle: collect from all collectors and write to cache.
	ctx := context.Background()
	for _, c := range registry.All() {
		result, err := c.Collect(ctx)
		if err != nil {
			t.Fatalf("Collect(%s) error: %v", c.Name(), err)
		}
		if err := store.Set(result.Collector, result.Data); err != nil {
			t.Fatalf("store.Set(%s) error: %v", result.Collector, err)
		}
	}

	// Simulate health file write (the daemon does this after each poll).
	collectorNames := make([]string, 0)
	for _, c := range registry.All() {
		collectorNames = append(collectorNames, c.Name())
	}
	writeTestHealthFile(t, cacheDir, collectorNames)

	// Verify health file exists and is valid JSON.
	healthPath := filepath.Join(cacheDir, "health.json")
	data, err := os.ReadFile(healthPath)
	if err != nil {
		t.Fatalf("ReadFile(health.json) error: %v", err)
	}

	var health struct {
		Status     string            `json:"status"`
		LastPoll   time.Time         `json:"last_poll"`
		Collectors map[string]string `json:"collectors"`
	}
	if err := json.Unmarshal(data, &health); err != nil {
		t.Fatalf("Unmarshal(health.json) error: %v", err)
	}

	if health.Status != "ok" {
		t.Errorf("health.status = %q, want %q", health.Status, "ok")
	}
	if len(health.Collectors) != 2 {
		t.Errorf("health.collectors count = %d, want 2", len(health.Collectors))
	}
	for _, name := range collectorNames {
		if health.Collectors[name] != "ok" {
			t.Errorf("health.collectors[%q] = %q, want %q", name, health.Collectors[name], "ok")
		}
	}
	if time.Since(health.LastPoll) > time.Minute {
		t.Error("health.last_poll should be recent")
	}
}

// writeTestHealthFile writes a health file to the given cache directory.
// This mirrors the daemon's writeHealthFile function.
func writeTestHealthFile(t *testing.T, cacheDir string, collectorNames []string) {
	t.Helper()

	type healthStatus struct {
		Status     string            `json:"status"`
		LastPoll   time.Time         `json:"last_poll"`
		Collectors map[string]string `json:"collectors"`
	}

	status := healthStatus{
		Status:     "ok",
		LastPoll:   time.Now(),
		Collectors: make(map[string]string, len(collectorNames)),
	}
	for _, name := range collectorNames {
		status.Collectors[name] = "ok"
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal health status: %v", err)
	}

	path := filepath.Join(cacheDir, "health.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// TestCollectorResultsCacheable validates that collector results can be stored
// and retrieved from the cache with correct types.
func TestCollectorResultsCacheable(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	store, err := cache.NewStore(cacheDir, logger)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	// Write mock claude data.
	claudeData := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "test",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
			},
		},
	}
	if err := store.Set("claude", claudeData); err != nil {
		t.Fatalf("store.Set(claude) error: %v", err)
	}

	// Retrieve and verify.
	result, fresh, err := cache.GetTyped[collectors.ClaudeUsage](store, "claude", time.Hour)
	if err != nil {
		t.Fatalf("GetTyped(claude) error: %v", err)
	}
	if !fresh {
		t.Error("claude data should be fresh immediately after write")
	}
	if result == nil {
		t.Fatal("expected non-nil claude result")
	}
	if len(result.Accounts) != 1 {
		t.Errorf("expected 1 account, got %d", len(result.Accounts))
	}
	if result.Accounts[0].Name != "test" {
		t.Errorf("account name = %q, want %q", result.Accounts[0].Name, "test")
	}
}

// TestDefaultDaemonConfig validates default daemon config values.
func TestDefaultDaemonConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.Daemon.PollInterval == "" {
		t.Error("default poll interval should not be empty")
	}
	if cfg.Daemon.CacheDir == "" {
		t.Error("default cache dir should not be empty")
	}
}
