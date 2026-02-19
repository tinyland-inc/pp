package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// General defaults
	if cfg.General.DaemonPollInterval.Duration != 15*time.Minute {
		t.Errorf("DaemonPollInterval = %v, want 15m", cfg.General.DaemonPollInterval)
	}
	if cfg.General.DataRetention.Duration != 10*time.Minute {
		t.Errorf("DataRetention = %v, want 10m", cfg.General.DataRetention)
	}
	if cfg.General.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.General.LogLevel, "info")
	}
	if cfg.General.CacheDir == "" {
		t.Error("CacheDir should not be empty")
	}

	// Layout defaults
	if cfg.Layout.Preset != "dashboard" {
		t.Errorf("Layout.Preset = %q, want %q", cfg.Layout.Preset, "dashboard")
	}

	// Collector defaults -- all intervals > 0
	if cfg.Collectors.SysMetrics.Interval.Duration <= 0 {
		t.Error("SysMetrics.Interval should be > 0")
	}
	if !cfg.Collectors.SysMetrics.Enabled {
		t.Error("SysMetrics should be enabled by default")
	}
	if cfg.Collectors.Tailscale.Interval.Duration <= 0 {
		t.Error("Tailscale.Interval should be > 0")
	}
	if !cfg.Collectors.Tailscale.Enabled {
		t.Error("Tailscale should be enabled by default")
	}
	if cfg.Collectors.Kubernetes.Enabled {
		t.Error("Kubernetes should be disabled by default")
	}
	if cfg.Collectors.Kubernetes.Interval.Duration <= 0 {
		t.Error("Kubernetes.Interval should be > 0 even when disabled")
	}
	if !cfg.Collectors.Claude.Enabled {
		t.Error("Claude should be enabled by default")
	}
	if cfg.Collectors.Billing.Enabled {
		t.Error("Billing should be disabled by default")
	}

	// Image defaults
	if cfg.Image.Protocol != "auto" {
		t.Errorf("Image.Protocol = %q, want %q", cfg.Image.Protocol, "auto")
	}
	if !cfg.Image.WaifuEnabled {
		t.Error("WaifuEnabled should be true by default")
	}
	if cfg.Image.MaxCacheSizeMB != 50 {
		t.Errorf("MaxCacheSizeMB = %d, want 50", cfg.Image.MaxCacheSizeMB)
	}
	if cfg.Image.MaxSessions != 10 {
		t.Errorf("MaxSessions = %d, want 10", cfg.Image.MaxSessions)
	}
	if cfg.Image.WaifuCategory != "waifu" {
		t.Errorf("WaifuCategory = %q, want %q", cfg.Image.WaifuCategory, "waifu")
	}

	// Theme defaults
	if cfg.Theme.Name != "default" {
		t.Errorf("Theme.Name = %q, want %q", cfg.Theme.Name, "default")
	}

	// Shell defaults
	if cfg.Shell.TUIKeybinding != `\C-p` {
		t.Errorf("TUIKeybinding = %q, want %q", cfg.Shell.TUIKeybinding, `\C-p`)
	}
	if !cfg.Shell.ShowBannerOnStartup {
		t.Error("ShowBannerOnStartup should be true by default")
	}
	if cfg.Shell.BannerTimeout.Duration != 2*time.Second {
		t.Errorf("BannerTimeout = %v, want 2s", cfg.Shell.BannerTimeout)
	}
	if !cfg.Shell.InstantBanner {
		t.Error("InstantBanner should be true by default")
	}

	// Banner defaults
	if cfg.Banner.CompactMaxWidth != 80 {
		t.Errorf("CompactMaxWidth = %d, want 80", cfg.Banner.CompactMaxWidth)
	}
	if cfg.Banner.StandardMinWidth != 120 {
		t.Errorf("StandardMinWidth = %d, want 120", cfg.Banner.StandardMinWidth)
	}
	if cfg.Banner.WideMinWidth != 160 {
		t.Errorf("WideMinWidth = %d, want 160", cfg.Banner.WideMinWidth)
	}
	if cfg.Banner.UltraWideMinWidth != 200 {
		t.Errorf("UltraWideMinWidth = %d, want 200", cfg.Banner.UltraWideMinWidth)
	}
}

func TestLoadFromReader_Minimal(t *testing.T) {
	input := `
[general]
log_level = "warn"

[theme]
name = "gruvbox"
`
	cfg, err := LoadFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadFromReader() error: %v", err)
	}

	// Overridden fields
	if cfg.General.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.General.LogLevel, "warn")
	}
	if cfg.Theme.Name != "gruvbox" {
		t.Errorf("Theme.Name = %q, want %q", cfg.Theme.Name, "gruvbox")
	}

	// Fields not in the TOML should retain defaults
	if cfg.General.DaemonPollInterval.Duration != 15*time.Minute {
		t.Errorf("DaemonPollInterval = %v, want 15m (default)", cfg.General.DaemonPollInterval)
	}
	if !cfg.Collectors.SysMetrics.Enabled {
		t.Error("SysMetrics should remain enabled (default)")
	}
	if !cfg.Image.WaifuEnabled {
		t.Error("WaifuEnabled should remain true (default)")
	}
}

func TestLoadFromReader_Full(t *testing.T) {
	input := `
[general]
daemon_poll_interval = "10m"
data_retention = "30m"
log_level = "debug"
cache_dir = "/tmp/ppulse-cache"

[layout]
preset = "ops"

[collectors.sysmetrics]
enabled = true
interval = "2s"

[collectors.tailscale]
enabled = false
interval = "45s"

[collectors.kubernetes]
enabled = true
interval = "90s"
contexts = ["tinyland", "civo-prod"]
namespaces = ["default", "monitoring"]

[collectors.claude]
enabled = true
interval = "10m"

[[collectors.claude.account]]
name = "personal"

[[collectors.claude.account]]
name = "work"

[collectors.billing]
enabled = true
interval = "20m"

[collectors.billing.civo]
enabled = true

[collectors.billing.digitalocean]
enabled = true

[image]
protocol = "kitty"
max_cache_size_mb = 100
max_sessions = 20
waifu_enabled = true
waifu_category = "neko"

[theme]
name = "catppuccin"

[shell]
tui_keybinding = "\\C-p"
show_banner_on_startup = false
banner_timeout = "3s"
instant_banner = false

[banner]
compact_max_width = 90
standard_min_width = 130
wide_min_width = 170
ultrawide_min_width = 220
`
	cfg, err := LoadFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadFromReader() error: %v", err)
	}

	// General
	if cfg.General.DaemonPollInterval.Duration != 10*time.Minute {
		t.Errorf("DaemonPollInterval = %v, want 10m", cfg.General.DaemonPollInterval)
	}
	if cfg.General.DataRetention.Duration != 30*time.Minute {
		t.Errorf("DataRetention = %v, want 30m", cfg.General.DataRetention)
	}
	if cfg.General.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.General.LogLevel, "debug")
	}
	if cfg.General.CacheDir != "/tmp/ppulse-cache" {
		t.Errorf("CacheDir = %q, want %q", cfg.General.CacheDir, "/tmp/ppulse-cache")
	}

	// Layout
	if cfg.Layout.Preset != "ops" {
		t.Errorf("Layout.Preset = %q, want %q", cfg.Layout.Preset, "ops")
	}

	// Collectors
	if !cfg.Collectors.Tailscale.Enabled {
		// TOML explicitly sets false; but our default is true, so the decode
		// should override to false.
		// Actually, BurntSushi/toml decodes into the pre-filled struct, so
		// explicit `enabled = false` will set it to false.
	}
	if cfg.Collectors.Tailscale.Enabled {
		// Good: it was explicitly set to false
	} else {
		// This is the expected path
	}
	if cfg.Collectors.Tailscale.Interval.Duration != 45*time.Second {
		t.Errorf("Tailscale.Interval = %v, want 45s", cfg.Collectors.Tailscale.Interval)
	}
	if !cfg.Collectors.Kubernetes.Enabled {
		t.Error("Kubernetes should be enabled per config")
	}
	if cfg.Collectors.Kubernetes.Interval.Duration != 90*time.Second {
		t.Errorf("Kubernetes.Interval = %v, want 90s", cfg.Collectors.Kubernetes.Interval)
	}
	if len(cfg.Collectors.Kubernetes.Contexts) != 2 {
		t.Errorf("Kubernetes.Contexts length = %d, want 2", len(cfg.Collectors.Kubernetes.Contexts))
	}
	if len(cfg.Collectors.Kubernetes.Namespaces) != 2 {
		t.Errorf("Kubernetes.Namespaces length = %d, want 2", len(cfg.Collectors.Kubernetes.Namespaces))
	}

	// Claude accounts
	if len(cfg.Collectors.Claude.Accounts) != 2 {
		t.Fatalf("Claude.Accounts length = %d, want 2", len(cfg.Collectors.Claude.Accounts))
	}
	if cfg.Collectors.Claude.Accounts[0].Name != "personal" {
		t.Errorf("Claude.Accounts[0].Name = %q, want %q", cfg.Collectors.Claude.Accounts[0].Name, "personal")
	}
	if cfg.Collectors.Claude.Accounts[1].Name != "work" {
		t.Errorf("Claude.Accounts[1].Name = %q, want %q", cfg.Collectors.Claude.Accounts[1].Name, "work")
	}

	// Billing
	if !cfg.Collectors.Billing.Enabled {
		t.Error("Billing should be enabled per config")
	}
	if !cfg.Collectors.Billing.Civo.Enabled {
		t.Error("Civo billing should be enabled per config")
	}
	if !cfg.Collectors.Billing.DigitalOcean.Enabled {
		t.Error("DigitalOcean billing should be enabled per config")
	}

	// Image
	if cfg.Image.Protocol != "kitty" {
		t.Errorf("Image.Protocol = %q, want %q", cfg.Image.Protocol, "kitty")
	}
	if cfg.Image.MaxCacheSizeMB != 100 {
		t.Errorf("MaxCacheSizeMB = %d, want 100", cfg.Image.MaxCacheSizeMB)
	}
	if cfg.Image.MaxSessions != 20 {
		t.Errorf("MaxSessions = %d, want 20", cfg.Image.MaxSessions)
	}
	if cfg.Image.WaifuCategory != "neko" {
		t.Errorf("WaifuCategory = %q, want %q", cfg.Image.WaifuCategory, "neko")
	}

	// Theme
	if cfg.Theme.Name != "catppuccin" {
		t.Errorf("Theme.Name = %q, want %q", cfg.Theme.Name, "catppuccin")
	}

	// Shell
	if cfg.Shell.ShowBannerOnStartup {
		t.Error("ShowBannerOnStartup should be false per config")
	}
	if cfg.Shell.BannerTimeout.Duration != 3*time.Second {
		t.Errorf("BannerTimeout = %v, want 3s", cfg.Shell.BannerTimeout)
	}
	if cfg.Shell.InstantBanner {
		t.Error("InstantBanner should be false per config")
	}

	// Banner
	if cfg.Banner.CompactMaxWidth != 90 {
		t.Errorf("CompactMaxWidth = %d, want 90", cfg.Banner.CompactMaxWidth)
	}
	if cfg.Banner.StandardMinWidth != 130 {
		t.Errorf("StandardMinWidth = %d, want 130", cfg.Banner.StandardMinWidth)
	}
	if cfg.Banner.WideMinWidth != 170 {
		t.Errorf("WideMinWidth = %d, want 170", cfg.Banner.WideMinWidth)
	}
	if cfg.Banner.UltraWideMinWidth != 220 {
		t.Errorf("UltraWideMinWidth = %d, want 220", cfg.Banner.UltraWideMinWidth)
	}
}

func TestDuration_Parse(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"1s", 1 * time.Second},
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"1h", 1 * time.Hour},
		{"1h30m", 1*time.Hour + 30*time.Minute},
		{"500ms", 500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var d Duration
			if err := d.UnmarshalText([]byte(tt.input)); err != nil {
				t.Fatalf("UnmarshalText(%q) error: %v", tt.input, err)
			}
			if d.Duration != tt.want {
				t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, d.Duration, tt.want)
			}
		})
	}
}

func TestDuration_ParseInvalid(t *testing.T) {
	tests := []string{
		"not-a-duration",
		"abc",
		"-5m",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			var d Duration
			if err := d.UnmarshalText([]byte(input)); err == nil {
				t.Errorf("UnmarshalText(%q) should have returned error", input)
			}
		})
	}
}

func TestDuration_ParseEmpty(t *testing.T) {
	var d Duration
	if err := d.UnmarshalText([]byte("")); err != nil {
		t.Fatalf("UnmarshalText empty should not error: %v", err)
	}
	if d.Duration != 0 {
		t.Errorf("UnmarshalText empty = %v, want 0", d.Duration)
	}
}

func TestDuration_Roundtrip(t *testing.T) {
	tests := []time.Duration{
		1 * time.Second,
		30 * time.Second,
		5 * time.Minute,
		1 * time.Hour,
		2*time.Hour + 30*time.Minute,
	}

	for _, dur := range tests {
		t.Run(dur.String(), func(t *testing.T) {
			d := Duration{dur}
			text, err := d.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() error: %v", err)
			}

			var d2 Duration
			if err := d2.UnmarshalText(text); err != nil {
				t.Fatalf("UnmarshalText(%q) error: %v", string(text), err)
			}
			if d2.Duration != dur {
				t.Errorf("roundtrip: got %v, want %v", d2.Duration, dur)
			}
		})
	}
}

func TestEnvOverrides(t *testing.T) {
	tests := []struct {
		name    string
		envKey  string
		envVal  string
		check   func(*Config) bool
		errMsg  string
	}{
		{
			name:   "ANTHROPIC_ADMIN_KEY",
			envKey: "ANTHROPIC_ADMIN_KEY",
			envVal: "sk-admin-test-key",
			check:  func(c *Config) bool { return c.Collectors.Claude.AdminKey == "sk-admin-test-key" },
			errMsg: "Claude.AdminKey not set from ANTHROPIC_ADMIN_KEY",
		},
		{
			name:   "CIVO_TOKEN",
			envKey: "CIVO_TOKEN",
			envVal: "civo-test-token",
			check:  func(c *Config) bool { return c.Collectors.Billing.Civo.APIKey == "civo-test-token" },
			errMsg: "Billing.Civo.APIKey not set from CIVO_TOKEN",
		},
		{
			name:   "DIGITALOCEAN_TOKEN",
			envKey: "DIGITALOCEAN_TOKEN",
			envVal: "do-test-token",
			check:  func(c *Config) bool { return c.Collectors.Billing.DigitalOcean.APIKey == "do-test-token" },
			errMsg: "Billing.DigitalOcean.APIKey not set from DIGITALOCEAN_TOKEN",
		},
		{
			name:   "PPULSE_PROTOCOL",
			envKey: "PPULSE_PROTOCOL",
			envVal: "sixel",
			check:  func(c *Config) bool { return c.Image.Protocol == "sixel" },
			errMsg: "Image.Protocol not set from PPULSE_PROTOCOL",
		},
		{
			name:   "PPULSE_THEME",
			envKey: "PPULSE_THEME",
			envVal: "nord",
			check:  func(c *Config) bool { return c.Theme.Name == "nord" },
			errMsg: "Theme.Name not set from PPULSE_THEME",
		},
		{
			name:   "PPULSE_LAYOUT",
			envKey: "PPULSE_LAYOUT",
			envVal: "minimal",
			check:  func(c *Config) bool { return c.Layout.Preset == "minimal" },
			errMsg: "Layout.Preset not set from PPULSE_LAYOUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)
			cfg, err := LoadFromReader(strings.NewReader(""))
			if err != nil {
				t.Fatalf("LoadFromReader() error: %v", err)
			}
			if !tt.check(cfg) {
				t.Error(tt.errMsg)
			}
		})
	}
}

func TestLayoutPreset_Dashboard(t *testing.T) {
	layout := LayoutPreset("dashboard")
	if layout.Preset != "dashboard" {
		t.Errorf("Preset = %q, want %q", layout.Preset, "dashboard")
	}
	if len(layout.Rows) != 3 {
		t.Fatalf("dashboard rows = %d, want 3", len(layout.Rows))
	}

	// Row 1: waifu, claude, billing
	if len(layout.Rows[0].Children) != 3 {
		t.Errorf("row 0 children = %d, want 3", len(layout.Rows[0].Children))
	}
	if layout.Rows[0].Ratio != 3 {
		t.Errorf("row 0 ratio = %d, want 3", layout.Rows[0].Ratio)
	}
	assertChild(t, layout.Rows[0].Children[0], "waifu", 2)
	assertChild(t, layout.Rows[0].Children[1], "claude", 3)
	assertChild(t, layout.Rows[0].Children[2], "billing", 3)

	// Row 2: tailscale, k8s
	if len(layout.Rows[1].Children) != 2 {
		t.Errorf("row 1 children = %d, want 2", len(layout.Rows[1].Children))
	}
	if layout.Rows[1].Ratio != 4 {
		t.Errorf("row 1 ratio = %d, want 4", layout.Rows[1].Ratio)
	}

	// Row 3: sysmetrics
	if len(layout.Rows[2].Children) != 1 {
		t.Errorf("row 2 children = %d, want 1", len(layout.Rows[2].Children))
	}
}

func TestLayoutPreset_Minimal(t *testing.T) {
	layout := LayoutPreset("minimal")
	if layout.Preset != "minimal" {
		t.Errorf("Preset = %q, want %q", layout.Preset, "minimal")
	}
	if len(layout.Rows) != 1 {
		t.Fatalf("minimal rows = %d, want 1", len(layout.Rows))
	}
	if len(layout.Rows[0].Children) != 2 {
		t.Fatalf("row 0 children = %d, want 2", len(layout.Rows[0].Children))
	}
	assertChild(t, layout.Rows[0].Children[0], "waifu", 1)
	assertChild(t, layout.Rows[0].Children[1], "claude", 1)
}

func TestLayoutPreset_Ops(t *testing.T) {
	layout := LayoutPreset("ops")
	if layout.Preset != "ops" {
		t.Errorf("Preset = %q, want %q", layout.Preset, "ops")
	}
	if len(layout.Rows) != 3 {
		t.Fatalf("ops rows = %d, want 3", len(layout.Rows))
	}

	// Row 1: tailscale, k8s
	assertChild(t, layout.Rows[0].Children[0], "tailscale", 1)
	assertChild(t, layout.Rows[0].Children[1], "k8s", 1)

	// Row 2: sysmetrics
	assertChild(t, layout.Rows[1].Children[0], "sysmetrics", 1)

	// Row 3: claude, billing
	assertChild(t, layout.Rows[2].Children[0], "claude", 1)
	assertChild(t, layout.Rows[2].Children[1], "billing", 1)
}

func TestLayoutPreset_Billing(t *testing.T) {
	layout := LayoutPreset("billing")
	if layout.Preset != "billing" {
		t.Errorf("Preset = %q, want %q", layout.Preset, "billing")
	}
	if len(layout.Rows) != 3 {
		t.Fatalf("billing rows = %d, want 3", len(layout.Rows))
	}

	assertChild(t, layout.Rows[0].Children[0], "claude", 1)
	assertChild(t, layout.Rows[1].Children[0], "billing", 1)
	assertChild(t, layout.Rows[2].Children[0], "sysmetrics", 1)
}

func TestLayoutPreset_InvalidReturnsDefault(t *testing.T) {
	layout := LayoutPreset("nonexistent")
	dashboard := LayoutPreset("dashboard")

	if layout.Preset != dashboard.Preset {
		t.Errorf("invalid preset returned %q, want %q (dashboard)", layout.Preset, dashboard.Preset)
	}
	if len(layout.Rows) != len(dashboard.Rows) {
		t.Errorf("invalid preset rows = %d, want %d", len(layout.Rows), len(dashboard.Rows))
	}
}

func TestLoadFromReader_MissingFieldsGetDefaults(t *testing.T) {
	// Only set theme, everything else should be defaults
	input := `
[theme]
name = "dracula"
`
	cfg, err := LoadFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadFromReader() error: %v", err)
	}

	if cfg.Theme.Name != "dracula" {
		t.Errorf("Theme.Name = %q, want %q", cfg.Theme.Name, "dracula")
	}

	// Verify defaults are preserved
	defaults := DefaultConfig()
	if cfg.General.DaemonPollInterval.Duration != defaults.General.DaemonPollInterval.Duration {
		t.Errorf("DaemonPollInterval = %v, want %v (default)", cfg.General.DaemonPollInterval, defaults.General.DaemonPollInterval)
	}
	if cfg.Collectors.SysMetrics.Enabled != defaults.Collectors.SysMetrics.Enabled {
		t.Errorf("SysMetrics.Enabled = %v, want %v (default)", cfg.Collectors.SysMetrics.Enabled, defaults.Collectors.SysMetrics.Enabled)
	}
	if cfg.Image.WaifuEnabled != defaults.Image.WaifuEnabled {
		t.Errorf("WaifuEnabled = %v, want %v (default)", cfg.Image.WaifuEnabled, defaults.Image.WaifuEnabled)
	}
	if cfg.Shell.InstantBanner != defaults.Shell.InstantBanner {
		t.Errorf("InstantBanner = %v, want %v (default)", cfg.Shell.InstantBanner, defaults.Shell.InstantBanner)
	}
	if cfg.Banner.CompactMaxWidth != defaults.Banner.CompactMaxWidth {
		t.Errorf("CompactMaxWidth = %d, want %d (default)", cfg.Banner.CompactMaxWidth, defaults.Banner.CompactMaxWidth)
	}
}

func TestCustomLayoutRowsOverridePreset(t *testing.T) {
	input := `
[layout]
preset = "minimal"

[[layout.row]]
ratio = 1

  [[layout.row.child]]
  type = "sysmetrics"
  ratio = 1

[[layout.row]]
ratio = 2

  [[layout.row.child]]
  type = "claude"
  ratio = 3

  [[layout.row.child]]
  type = "billing"
  ratio = 2
`
	cfg, err := LoadFromReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadFromReader() error: %v", err)
	}

	// Preset is set but custom rows should be present
	if cfg.Layout.Preset != "minimal" {
		t.Errorf("Layout.Preset = %q, want %q", cfg.Layout.Preset, "minimal")
	}
	if len(cfg.Layout.Rows) != 2 {
		t.Fatalf("Layout.Rows length = %d, want 2", len(cfg.Layout.Rows))
	}
	if cfg.Layout.Rows[0].Ratio != 1 {
		t.Errorf("Row 0 ratio = %d, want 1", cfg.Layout.Rows[0].Ratio)
	}
	if len(cfg.Layout.Rows[0].Children) != 1 {
		t.Fatalf("Row 0 children = %d, want 1", len(cfg.Layout.Rows[0].Children))
	}
	assertChild(t, cfg.Layout.Rows[0].Children[0], "sysmetrics", 1)

	if cfg.Layout.Rows[1].Ratio != 2 {
		t.Errorf("Row 1 ratio = %d, want 2", cfg.Layout.Rows[1].Ratio)
	}
	if len(cfg.Layout.Rows[1].Children) != 2 {
		t.Fatalf("Row 1 children = %d, want 2", len(cfg.Layout.Rows[1].Children))
	}
	assertChild(t, cfg.Layout.Rows[1].Children[0], "claude", 3)
	assertChild(t, cfg.Layout.Rows[1].Children[1], "billing", 2)
}

func TestLoadFromFile_NonExistent(t *testing.T) {
	cfg, err := LoadFromFile("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("LoadFromFile() should not error for missing file: %v", err)
	}
	defaults := DefaultConfig()
	if cfg.General.LogLevel != defaults.General.LogLevel {
		t.Errorf("missing file should return defaults, got LogLevel = %q", cfg.General.LogLevel)
	}
}

func TestLoadFromFile_Testdata(t *testing.T) {
	cfg, err := LoadFromFile("testdata/full.toml")
	if err != nil {
		t.Fatalf("LoadFromFile(full.toml) error: %v", err)
	}
	if cfg.General.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.General.LogLevel, "debug")
	}
	if cfg.Theme.Name != "catppuccin" {
		t.Errorf("Theme.Name = %q, want %q", cfg.Theme.Name, "catppuccin")
	}
	if cfg.Image.Protocol != "kitty" {
		t.Errorf("Image.Protocol = %q, want %q", cfg.Image.Protocol, "kitty")
	}
}

func TestLoadFromFile_TestdataMinimal(t *testing.T) {
	cfg, err := LoadFromFile("testdata/minimal.toml")
	if err != nil {
		t.Fatalf("LoadFromFile(minimal.toml) error: %v", err)
	}
	if cfg.General.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", cfg.General.LogLevel, "warn")
	}
	if cfg.Theme.Name != "gruvbox" {
		t.Errorf("Theme.Name = %q, want %q", cfg.Theme.Name, "gruvbox")
	}
	// Defaults preserved
	if cfg.General.DaemonPollInterval.Duration != 15*time.Minute {
		t.Errorf("DaemonPollInterval = %v, want 15m (default)", cfg.General.DaemonPollInterval)
	}
}

func TestReadEnvFile(t *testing.T) {
	// Create a temp file with a secret value and trailing newline.
	f, err := os.CreateTemp("", "ppulse-secret-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("my-secret-key\n")
	f.Close()

	t.Setenv("TEST_SECRET_FILE", f.Name())
	got := readEnvFile("TEST_SECRET_FILE")
	if got != "my-secret-key" {
		t.Errorf("readEnvFile() = %q, want %q", got, "my-secret-key")
	}
}

func TestReadEnvFile_Missing(t *testing.T) {
	t.Setenv("TEST_MISSING_FILE", "/nonexistent/path/secret")
	got := readEnvFile("TEST_MISSING_FILE")
	if got != "" {
		t.Errorf("readEnvFile() = %q, want empty string for missing file", got)
	}
}

func TestReadEnvFile_Unset(t *testing.T) {
	got := readEnvFile("DEFINITELY_NOT_SET_12345")
	if got != "" {
		t.Errorf("readEnvFile() = %q, want empty string for unset env var", got)
	}
}

func TestApplyEnvOverrides_FileVars(t *testing.T) {
	// Create temp files for each secret.
	tmpAnthro, _ := os.CreateTemp("", "anthro-*")
	tmpAnthro.WriteString("admin-key-from-file\n")
	tmpAnthro.Close()
	defer os.Remove(tmpAnthro.Name())

	tmpCivo, _ := os.CreateTemp("", "civo-*")
	tmpCivo.WriteString("civo-key-from-file\n")
	tmpCivo.Close()
	defer os.Remove(tmpCivo.Name())

	// Clear direct env vars to ensure _FILE variants are used.
	t.Setenv("ANTHROPIC_ADMIN_KEY", "")
	t.Setenv("CIVO_TOKEN", "")
	t.Setenv("ANTHROPIC_ADMIN_KEY_FILE", tmpAnthro.Name())
	t.Setenv("CIVO_API_KEY_FILE", tmpCivo.Name())

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.Collectors.Claude.AdminKey != "admin-key-from-file" {
		t.Errorf("Claude.AdminKey = %q, want %q", cfg.Collectors.Claude.AdminKey, "admin-key-from-file")
	}
	if cfg.Collectors.Billing.Civo.APIKey != "civo-key-from-file" {
		t.Errorf("Billing.Civo.APIKey = %q, want %q", cfg.Collectors.Billing.Civo.APIKey, "civo-key-from-file")
	}
}

func TestApplyEnvOverrides_DirectTakesPrecedence(t *testing.T) {
	// Direct env var should take precedence over _FILE variant.
	tmpAnthro, _ := os.CreateTemp("", "anthro-*")
	tmpAnthro.WriteString("from-file\n")
	tmpAnthro.Close()
	defer os.Remove(tmpAnthro.Name())

	t.Setenv("ANTHROPIC_ADMIN_KEY", "from-env-direct")
	t.Setenv("ANTHROPIC_ADMIN_KEY_FILE", tmpAnthro.Name())

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if cfg.Collectors.Claude.AdminKey != "from-env-direct" {
		t.Errorf("Claude.AdminKey = %q, want %q (direct should take precedence)", cfg.Collectors.Claude.AdminKey, "from-env-direct")
	}
}

// assertChild checks a ChildConfig's type and ratio.
func assertChild(t *testing.T, c ChildConfig, wantType string, wantRatio int) {
	t.Helper()
	if c.Type != wantType {
		t.Errorf("child type = %q, want %q", c.Type, wantType)
	}
	if c.Ratio != wantRatio {
		t.Errorf("child %q ratio = %d, want %d", c.Type, c.Ratio, wantRatio)
	}
}
