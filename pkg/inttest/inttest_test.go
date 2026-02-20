package inttest

import (
	"strings"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/banner"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/config"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/layout"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/preset"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/shell"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/theme"
)

// ---------------------------------------------------------------------------
// Suite framework tests
// ---------------------------------------------------------------------------

func TestSuiteCreate(t *testing.T) {
	s := NewSuite("test-suite")
	if s.Name != "test-suite" {
		t.Errorf("suite name: got %q, want %q", s.Name, "test-suite")
	}
	if len(s.Tests) != 0 {
		t.Errorf("new suite should have 0 tests, got %d", len(s.Tests))
	}
}

func TestSuiteAddTests(t *testing.T) {
	s := NewSuite("add-tests")
	s.Add("test1", func(t *testing.T) {}, "tag1")
	s.Add("test2", func(t *testing.T) {}, "tag1", "tag2")
	s.Add("test3", func(t *testing.T) {})

	if len(s.Tests) != 3 {
		t.Errorf("suite should have 3 tests, got %d", len(s.Tests))
	}
	if len(s.Tests[1].Tags) != 2 {
		t.Errorf("test2 should have 2 tags, got %d", len(s.Tests[1].Tags))
	}
}

func TestSuiteRunAll(t *testing.T) {
	ran := make(map[string]bool)
	s := NewSuite("run-all")
	s.Add("a", func(t *testing.T) { ran["a"] = true })
	s.Add("b", func(t *testing.T) { ran["b"] = true })
	s.Add("c", func(t *testing.T) { ran["c"] = true })

	s.Run(t)

	for _, name := range []string{"a", "b", "c"} {
		if !ran[name] {
			t.Errorf("test %q was not run", name)
		}
	}
}

func TestSuiteRunTagged(t *testing.T) {
	ran := make(map[string]bool)
	s := NewSuite("run-tagged")
	s.Add("tagged1", func(t *testing.T) { ran["tagged1"] = true }, "fast")
	s.Add("tagged2", func(t *testing.T) { ran["tagged2"] = true }, "slow")
	s.Add("tagged3", func(t *testing.T) { ran["tagged3"] = true }, "fast", "slow")
	s.Add("untagged", func(t *testing.T) { ran["untagged"] = true })

	s.RunTagged(t, "fast")

	if !ran["tagged1"] {
		t.Error("tagged1 should have run (has 'fast' tag)")
	}
	if ran["tagged2"] {
		t.Error("tagged2 should NOT have run (only has 'slow' tag)")
	}
	if !ran["tagged3"] {
		t.Error("tagged3 should have run (has 'fast' tag)")
	}
	if ran["untagged"] {
		t.Error("untagged should NOT have run (no tags)")
	}
}

func TestSuiteSetupTeardown(t *testing.T) {
	setupCalled := false
	teardownCalled := false

	// Register our verification cleanup BEFORE s.Run so it executes
	// AFTER the suite's t.Cleanup (LIFO order).
	t.Cleanup(func() {
		if !teardownCalled {
			t.Error("teardown was not called")
		}
	})

	s := NewSuite("setup-teardown")
	s.Setup = func() error {
		setupCalled = true
		return nil
	}
	s.Teardown = func() error {
		teardownCalled = true
		return nil
	}
	s.Add("dummy", func(t *testing.T) {})

	s.Run(t)

	if !setupCalled {
		t.Error("setup was not called")
	}
}

func TestSuiteRunTaggedEmpty(t *testing.T) {
	ran := false
	s := NewSuite("empty-tags")
	s.Add("test", func(t *testing.T) { ran = true }, "x")

	// No tags provided: nothing should run.
	s.RunTagged(t)
	if ran {
		t.Error("test should not have run with no tags specified")
	}
}

// ---------------------------------------------------------------------------
// Pipeline tests
// ---------------------------------------------------------------------------

func TestPipelineFullExecution(t *testing.T) {
	p := itBuildFullPipeline()
	results, err := p.Execute()
	if err != nil {
		t.Fatalf("full pipeline failed: %v", err)
	}

	for _, r := range results {
		if !r.Passed {
			t.Errorf("stage %q failed: %s", r.Name, r.Error)
		}
	}

	if len(results) != 8 {
		t.Errorf("expected 8 stages, got %d", len(results))
	}
}

func TestPipelineStageFailure(t *testing.T) {
	p := itNewPipeline()
	p.AddStage("pass", func() error { return nil }, nil)
	p.AddStage("fail", func() error {
		return &pipelineError{"intentional failure"}
	}, nil)
	p.AddStage("skip", func() error { return nil }, nil)

	results, err := p.Execute()
	if err == nil {
		t.Fatal("pipeline should have returned an error")
	}

	if len(results) != 2 {
		t.Errorf("should have 2 results (pass + fail), got %d", len(results))
	}
	if results[0].Passed != true {
		t.Error("first stage should have passed")
	}
	if results[1].Passed != false {
		t.Error("second stage should have failed")
	}
}

func TestPipelinePartialExecution(t *testing.T) {
	p := itNewPipeline()

	count := 0
	for i := 0; i < 5; i++ {
		p.AddStage("stage"+string(rune('A'+i)), func() error {
			count++
			return nil
		}, nil)
	}

	_, err := p.Execute()
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 stages to run, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Cross-package compatibility tests
// ---------------------------------------------------------------------------

func TestWidgetCollectorCompat(t *testing.T) {
	itCheckWidgetCollectorCompat(t)
}

func TestThemePresetCompat(t *testing.T) {
	itCheckThemePresetCompat(t)
}

func TestLayoutConstraints(t *testing.T) {
	itCheckLayoutConstraints(t)
}

func TestConfigRoundTrip(t *testing.T) {
	itCheckConfigRoundTrip(t)
}

func TestConfigRoundTripSections(t *testing.T) {
	tomlStr := itMockConfig()
	cfg, err := config.LoadFromReader(strings.NewReader(tomlStr))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Validate individual collector configs.
	if cfg.Collectors.SysMetrics.Interval.Duration != 1*time.Second {
		t.Errorf("sysmetrics interval: got %v, want 1s", cfg.Collectors.SysMetrics.Interval.Duration)
	}
	if cfg.Collectors.Tailscale.Interval.Duration != 30*time.Second {
		t.Errorf("tailscale interval: got %v, want 30s", cfg.Collectors.Tailscale.Interval.Duration)
	}
	if cfg.Collectors.Claude.Interval.Duration != 5*time.Minute {
		t.Errorf("claude interval: got %v, want 5m", cfg.Collectors.Claude.Interval.Duration)
	}

	// Validate shell config.
	if cfg.Shell.BannerTimeout.Duration != 2*time.Second {
		t.Errorf("banner_timeout: got %v, want 2s", cfg.Shell.BannerTimeout.Duration)
	}
	if !cfg.Shell.InstantBanner {
		t.Error("instant_banner should be true")
	}
}

// ---------------------------------------------------------------------------
// Rendering integration tests
// ---------------------------------------------------------------------------

func TestBannerAllWidgets(t *testing.T) {
	itTestBannerAllWidgets(t)
}

func TestBannerResize(t *testing.T) {
	itTestBannerResize(t)
}

func TestEmptyState(t *testing.T) {
	itTestEmptyState(t)
}

func TestBannerPresetSelection(t *testing.T) {
	// Verify that SelectPreset returns the correct preset for given sizes.
	tests := []struct {
		w, h int
		want string
	}{
		{80, 24, "compact"},
		{120, 35, "standard"},
		{160, 45, "wide"},
		{200, 50, "ultrawide"},
		{40, 10, "compact"},
		{300, 80, "ultrawide"},
		{119, 35, "compact"},  // width just under standard
		{120, 34, "compact"},  // height just under standard
	}

	for _, tt := range tests {
		p := banner.SelectPreset(tt.w, tt.h)
		if p.Name != tt.want {
			t.Errorf("SelectPreset(%d, %d): got %q, want %q",
				tt.w, tt.h, p.Name, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Shell integration tests
// ---------------------------------------------------------------------------

func TestAllShellsGenerate(t *testing.T) {
	itTestAllShellsGenerate(t)
}

func TestShellBannerInvoke(t *testing.T) {
	itTestShellBannerInvoke(t)
}

func TestStarshipModule(t *testing.T) {
	itTestStarshipModule(t)
}

func TestShellDetect(t *testing.T) {
	itTestShellDetect(t)
}

func TestShellDefaultOptions(t *testing.T) {
	itTestShellDefaultOptions(t)
}

func TestShellNoBanner(t *testing.T) {
	itTestShellNoBanner(t)
}

// ---------------------------------------------------------------------------
// Mock data validity tests
// ---------------------------------------------------------------------------

func TestMockClaudeData(t *testing.T) {
	data := itMockClaudeData()
	if data == nil {
		t.Fatal("itMockClaudeData returned nil")
	}
	if _, ok := data["total_cost_usd"]; !ok {
		t.Error("missing total_cost_usd key")
	}
	if _, ok := data["accounts"]; !ok {
		t.Error("missing accounts key")
	}
}

func TestMockBillingData(t *testing.T) {
	data := itMockBillingData()
	if data == nil {
		t.Fatal("itMockBillingData returned nil")
	}
	if _, ok := data["total_monthly_usd"]; !ok {
		t.Error("missing total_monthly_usd key")
	}
	if _, ok := data["providers"]; !ok {
		t.Error("missing providers key")
	}
}

func TestMockTailscaleData(t *testing.T) {
	data := itMockTailscaleData()
	if data == nil {
		t.Fatal("itMockTailscaleData returned nil")
	}
	if _, ok := data["total_peers"]; !ok {
		t.Error("missing total_peers key")
	}
	if _, ok := data["online_peers"]; !ok {
		t.Error("missing online_peers key")
	}
	peers, ok := data["peers"].([]map[string]any)
	if !ok {
		t.Fatal("peers is not []map[string]any")
	}
	if len(peers) != 5 {
		t.Errorf("expected 5 peers, got %d", len(peers))
	}
}

func TestMockK8sData(t *testing.T) {
	data := itMockK8sData()
	if data == nil {
		t.Fatal("itMockK8sData returned nil")
	}
	clusters, ok := data["clusters"].([]map[string]any)
	if !ok {
		t.Fatal("clusters is not []map[string]any")
	}
	if len(clusters) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(clusters))
	}
}

func TestMockSysMetrics(t *testing.T) {
	data := itMockSysMetrics()
	if data == nil {
		t.Fatal("itMockSysMetrics returned nil")
	}
	for _, key := range []string{"cpu", "memory", "disk", "load_avg"} {
		if _, ok := data[key]; !ok {
			t.Errorf("missing %q key", key)
		}
	}
}

func TestMockWaifuImage(t *testing.T) {
	img := itMockWaifuImage()
	if len(img) == 0 {
		t.Fatal("itMockWaifuImage returned empty bytes")
	}
	// Verify PNG signature.
	if img[0] != 0x89 || img[1] != 0x50 || img[2] != 0x4E || img[3] != 0x47 {
		t.Error("itMockWaifuImage does not start with PNG signature")
	}
}

func TestMockConfig(t *testing.T) {
	toml := itMockConfig()
	if toml == "" {
		t.Fatal("itMockConfig returned empty string")
	}

	// Should be parseable.
	cfg, err := config.LoadFromReader(strings.NewReader(toml))
	if err != nil {
		t.Fatalf("itMockConfig not parseable: %v", err)
	}
	if cfg.General.LogLevel != "info" {
		t.Errorf("parsed log_level: got %q, want %q", cfg.General.LogLevel, "info")
	}
}

// ---------------------------------------------------------------------------
// Error path tests
// ---------------------------------------------------------------------------

func TestMissingConfig(t *testing.T) {
	// Loading from a non-existent file should return default config (not error).
	cfg, err := config.LoadFromFile("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("loading non-existent config should not error: %v", err)
	}
	if cfg == nil {
		t.Fatal("config should not be nil")
	}
	if cfg.Layout.Preset != "dashboard" {
		t.Errorf("default preset: got %q, want %q", cfg.Layout.Preset, "dashboard")
	}
}

func TestInvalidTheme(t *testing.T) {
	// Getting an unknown theme should fall back to default.
	th := theme.Get("nonexistent-theme")
	if th.Name != "default" {
		t.Errorf("unknown theme should fall back to default, got %q", th.Name)
	}
}

func TestUnknownPreset(t *testing.T) {
	// Getting an unknown preset should fall back to dashboard.
	p := preset.Get("nonexistent-preset")
	if p.Name != "dashboard" {
		t.Errorf("unknown preset should fall back to dashboard, got %q", p.Name)
	}
}

func TestCorruptedCache(t *testing.T) {
	dir, cleanup, err := itTempDir("inttest-corrupt")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer cleanup()

	store, err := cache.NewStore(cache.StoreConfig{
		Dir:             dir,
		MaxSizeMB:       10,
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("create cache store: %v", err)
	}
	defer store.Close()

	// Write a value, then try to get a different key.
	if err := store.PutString("real-key", "real-value"); err != nil {
		t.Fatalf("cache put: %v", err)
	}

	// Reading a non-existent key returns empty/false.
	val, ok := store.GetString("wrong-key")
	if ok {
		t.Errorf("expected cache miss for wrong-key, got %q", val)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	dir, cleanup, err := itTempDir("inttest-ttl")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer cleanup()

	store, err := cache.NewStore(cache.StoreConfig{
		Dir:             dir,
		MaxSizeMB:       10,
		DefaultTTL:      50 * time.Millisecond,
		CleanupInterval: 1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("create cache store: %v", err)
	}
	defer store.Close()

	if err := store.PutString("ephemeral", "short-lived"); err != nil {
		t.Fatalf("cache put: %v", err)
	}

	// Immediately should be retrievable.
	val, ok := store.GetString("ephemeral")
	if !ok {
		t.Fatal("cache get immediately after put should succeed")
	}
	if val != "short-lived" {
		t.Errorf("cache value: got %q, want %q", val, "short-lived")
	}

	// Wait for TTL to expire.
	time.Sleep(100 * time.Millisecond)

	_, ok = store.GetString("ephemeral")
	if ok {
		t.Error("cache get after TTL should return false")
	}
}

// ---------------------------------------------------------------------------
// Layout solver integration tests
// ---------------------------------------------------------------------------

func TestLayoutHorizontalSplit(t *testing.T) {
	l := layout.NewLayout(layout.Horizontal,
		layout.Length{Value: 20},
		layout.Fill{Weight: 1},
		layout.Percentage{Value: 30},
	)

	area := layout.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	rects := l.Split(area)

	if len(rects) != 3 {
		t.Fatalf("expected 3 rects, got %d", len(rects))
	}

	// First: exactly 20.
	if rects[0].Width != 20 {
		t.Errorf("rect[0] width: got %d, want 20", rects[0].Width)
	}

	// Third: 30% of 100 = 30.
	if rects[2].Width != 30 {
		t.Errorf("rect[2] width: got %d, want 30", rects[2].Width)
	}

	// Second: fills remaining = 100 - 20 - 30 = 50.
	if rects[1].Width != 50 {
		t.Errorf("rect[1] width: got %d, want 50", rects[1].Width)
	}

	// All heights should match input.
	for i, r := range rects {
		if r.Height != 50 {
			t.Errorf("rect[%d] height: got %d, want 50", i, r.Height)
		}
	}
}

func TestLayoutVerticalSplit(t *testing.T) {
	l := layout.NewLayout(layout.Vertical,
		layout.Ratio{Num: 1, Den: 3},
		layout.Ratio{Num: 2, Den: 3},
	)

	area := layout.Rect{X: 0, Y: 0, Width: 80, Height: 30}
	rects := l.Split(area)

	if len(rects) != 2 {
		t.Fatalf("expected 2 rects, got %d", len(rects))
	}

	// First: 1/3 of 30 = 10.
	if rects[0].Height != 10 {
		t.Errorf("rect[0] height: got %d, want 10", rects[0].Height)
	}

	// Second: 2/3 of 30 = 20.
	if rects[1].Height != 20 {
		t.Errorf("rect[1] height: got %d, want 20", rects[1].Height)
	}
}

func TestLayoutWithMarginAndSpacing(t *testing.T) {
	l := layout.NewLayout(layout.Horizontal,
		layout.Fill{Weight: 1},
		layout.Fill{Weight: 1},
	).WithMargin(2).WithSpacing(1)

	area := layout.Rect{X: 0, Y: 0, Width: 100, Height: 50}
	rects := l.Split(area)

	if len(rects) != 2 {
		t.Fatalf("expected 2 rects, got %d", len(rects))
	}

	// With margin 2 on each side: inner width = 96.
	// With spacing 1 between: usable = 95.
	// Each fill gets 95/2 = 47 (one gets 48 for rounding).
	total := rects[0].Width + rects[1].Width
	if total != 95 {
		t.Errorf("total width: got %d, want 95", total)
	}

	// X positions should respect margin.
	if rects[0].X != 2 {
		t.Errorf("rect[0] X: got %d, want 2", rects[0].X)
	}
}

// ---------------------------------------------------------------------------
// Preset resolution tests
// ---------------------------------------------------------------------------

func TestPresetNames(t *testing.T) {
	names := preset.Names()
	expected := []string{"billing", "dashboard", "minimal", "ops"}

	if len(names) != len(expected) {
		t.Fatalf("expected %d presets, got %d: %v", len(expected), len(names), names)
	}

	for i, name := range expected {
		if names[i] != name {
			t.Errorf("preset[%d]: got %q, want %q", i, names[i], name)
		}
	}
}

func TestPresetSelectByConfig(t *testing.T) {
	cfg := config.Config{Layout: config.LayoutConfig{Preset: "ops"}}
	name := preset.SelectByConfig(cfg)
	if name != "ops" {
		t.Errorf("SelectByConfig with 'ops': got %q", name)
	}

	cfg.Layout.Preset = ""
	name = preset.SelectByConfig(cfg)
	if name != "dashboard" {
		t.Errorf("SelectByConfig with empty: got %q, want 'dashboard'", name)
	}

	cfg.Layout.Preset = "auto"
	name = preset.SelectByConfig(cfg)
	if name != "dashboard" {
		t.Errorf("SelectByConfig with 'auto': got %q, want 'dashboard'", name)
	}
}

func TestPresetSelectForSize(t *testing.T) {
	tests := []struct {
		w, h int
		want string
	}{
		{50, 20, "minimal"},
		{99, 40, "minimal"},
		{100, 40, "dashboard"},
		{200, 60, "dashboard"},
	}

	for _, tt := range tests {
		got := preset.SelectForSize(tt.w, tt.h)
		if got != tt.want {
			t.Errorf("SelectForSize(%d, %d): got %q, want %q",
				tt.w, tt.h, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Theme tests
// ---------------------------------------------------------------------------

func TestAllThemesHaveRequiredColors(t *testing.T) {
	for _, name := range theme.Names() {
		t.Run(name, func(t *testing.T) {
			th := theme.Get(name)

			required := map[string]string{
				"Background":  th.Background,
				"Foreground":  th.Foreground,
				"Accent":      th.Accent,
				"Border":      th.Border,
				"BorderFocus": th.BorderFocus,
				"StatusOK":    th.StatusOK,
				"StatusWarn":  th.StatusWarn,
				"StatusError": th.StatusError,
				"GaugeFilled": th.GaugeFilled,
				"GaugeEmpty":  th.GaugeEmpty,
				"ChartLine":   th.ChartLine,
			}

			for field, val := range required {
				if val == "" {
					t.Errorf("theme %q field %s is empty", name, field)
				}
				if !strings.HasPrefix(val, "#") {
					t.Errorf("theme %q field %s: %q is not a hex color", name, field, val)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Shell script content tests
// ---------------------------------------------------------------------------

func TestShellScriptDaemonFunctions(t *testing.T) {
	opts := shell.Options{
		BinaryPath:      "prompt-pulse",
		ShowBanner:       true,
		DaemonAutoStart: true,
	}

	for _, sh := range []shell.ShellType{shell.Bash, shell.Zsh, shell.Ksh} {
		t.Run(string(sh), func(t *testing.T) {
			script := shell.Generate(sh, opts)

			for _, fn := range []string{"pp-start", "pp-stop", "pp-status"} {
				if !strings.Contains(script, fn) {
					t.Errorf("shell %q script missing daemon function %q", sh, fn)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helper type for test errors
// ---------------------------------------------------------------------------

type pipelineError struct {
	msg string
}

func (e *pipelineError) Error() string {
	return e.msg
}
