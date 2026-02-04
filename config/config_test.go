package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Daemon defaults
	if cfg.Daemon.PollInterval != "15m" {
		t.Errorf("expected PollInterval=15m, got %s", cfg.Daemon.PollInterval)
	}
	if cfg.Daemon.CacheDir == "" {
		t.Error("expected CacheDir to be set")
	}
	if cfg.Daemon.LogFile == "" {
		t.Error("expected LogFile to be set")
	}

	// Claude account defaults
	if len(cfg.Accounts.Claude) != 1 {
		t.Fatalf("expected 1 default Claude account, got %d", len(cfg.Accounts.Claude))
	}
	if cfg.Accounts.Claude[0].Name != "primary" {
		t.Errorf("expected Claude account name=primary, got %s", cfg.Accounts.Claude[0].Name)
	}
	if cfg.Accounts.Claude[0].Type != "subscription" {
		t.Errorf("expected Claude account type=subscription, got %s", cfg.Accounts.Claude[0].Type)
	}
	if !cfg.Accounts.Claude[0].Enabled {
		t.Error("expected default Claude account to be enabled")
	}

	// Provider defaults
	if cfg.Accounts.Civo.APIKeyEnv != "CIVO_API_KEY" {
		t.Errorf("expected Civo APIKeyEnv=CIVO_API_KEY, got %s", cfg.Accounts.Civo.APIKeyEnv)
	}
	if cfg.Accounts.Civo.Region != "NYC1" {
		t.Errorf("expected Civo Region=NYC1, got %s", cfg.Accounts.Civo.Region)
	}
	if cfg.Accounts.DigitalOcean.APIKeyEnv != "DIGITALOCEAN_TOKEN" {
		t.Errorf("expected DO APIKeyEnv=DIGITALOCEAN_TOKEN, got %s", cfg.Accounts.DigitalOcean.APIKeyEnv)
	}
	if cfg.Accounts.AWS.Profile != "default" {
		t.Errorf("expected AWS Profile=default, got %s", cfg.Accounts.AWS.Profile)
	}
	if len(cfg.Accounts.AWS.Regions) != 1 || cfg.Accounts.AWS.Regions[0] != "us-east-1" {
		t.Errorf("expected AWS Regions=[us-east-1], got %v", cfg.Accounts.AWS.Regions)
	}

	// Tailscale defaults
	if cfg.Tailscale.APIKeyEnv != "TAILSCALE_API_KEY" {
		t.Errorf("expected Tailscale APIKeyEnv=TAILSCALE_API_KEY, got %s", cfg.Tailscale.APIKeyEnv)
	}
	if !cfg.Tailscale.UseCLIFallback {
		t.Error("expected Tailscale UseCLIFallback to be true")
	}

	// Display defaults
	if cfg.Display.Theme != "monitoring" {
		t.Errorf("expected Theme=monitoring, got %s", cfg.Display.Theme)
	}
	if !cfg.Display.EnableHyperlinks {
		t.Error("expected EnableHyperlinks to be true")
	}
	if cfg.Display.Waifu.Enabled {
		t.Error("expected Waifu to be disabled by default")
	}
	if cfg.Display.Waifu.MaxCacheMB != 50 {
		t.Errorf("expected Waifu MaxCacheMB=50, got %d", cfg.Display.Waifu.MaxCacheMB)
	}

	// Starship defaults
	if !cfg.Starship.Modules.Claude {
		t.Error("expected Starship Claude module enabled")
	}
	if !cfg.Starship.Modules.Billing {
		t.Error("expected Starship Billing module enabled")
	}
	if !cfg.Starship.Modules.Infra {
		t.Error("expected Starship Infra module enabled")
	}
}

func TestDefaultConfigValidates(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid, got error: %v", err)
	}
}

func TestLoadConfigNonExistent(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error for non-existent file: %v", err)
	}
	// Should return defaults
	if cfg.Daemon.PollInterval != "15m" {
		t.Errorf("expected default PollInterval=15m, got %s", cfg.Daemon.PollInterval)
	}
}

func TestLoadConfigEmptyPath(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("unexpected error for empty path: %v", err)
	}
	if cfg.Daemon.PollInterval != "15m" {
		t.Errorf("expected default PollInterval=15m, got %s", cfg.Daemon.PollInterval)
	}
}

func TestLoadConfigEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty file should use defaults
	if cfg.Daemon.PollInterval != "15m" {
		t.Errorf("expected default PollInterval=15m, got %s", cfg.Daemon.PollInterval)
	}
}

func TestLoadConfigValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
daemon:
  poll_interval: 30m
  cache_dir: /tmp/test-cache
  log_file: /tmp/test.log

accounts:
  claude:
    - name: work
      type: api
      api_key_env: CLAUDE_API_KEY
      enabled: true

  civo:
    api_key_env: MY_CIVO_KEY
    region: LON1

display:
  theme: minimal
  enable_hyperlinks: false
  waifu:
    enabled: true
    category: waifu
    cache_ttl: 12h
    max_cache_mb: 100

starship:
  modules:
    claude: false
    billing: true
    infra: false
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Overridden values
	if cfg.Daemon.PollInterval != "30m" {
		t.Errorf("expected PollInterval=30m, got %s", cfg.Daemon.PollInterval)
	}
	if cfg.Daemon.CacheDir != "/tmp/test-cache" {
		t.Errorf("expected CacheDir=/tmp/test-cache, got %s", cfg.Daemon.CacheDir)
	}
	if len(cfg.Accounts.Claude) != 1 {
		t.Fatalf("expected 1 Claude account, got %d", len(cfg.Accounts.Claude))
	}
	if cfg.Accounts.Claude[0].Name != "work" {
		t.Errorf("expected Claude account name=work, got %s", cfg.Accounts.Claude[0].Name)
	}
	if cfg.Accounts.Claude[0].Type != "api" {
		t.Errorf("expected Claude account type=api, got %s", cfg.Accounts.Claude[0].Type)
	}
	if cfg.Accounts.Civo.Region != "LON1" {
		t.Errorf("expected Civo Region=LON1, got %s", cfg.Accounts.Civo.Region)
	}
	if cfg.Display.Theme != "minimal" {
		t.Errorf("expected Theme=minimal, got %s", cfg.Display.Theme)
	}
	if cfg.Display.EnableHyperlinks {
		t.Error("expected EnableHyperlinks=false")
	}
	if !cfg.Display.Waifu.Enabled {
		t.Error("expected Waifu enabled")
	}
	if cfg.Display.Waifu.MaxCacheMB != 100 {
		t.Errorf("expected Waifu MaxCacheMB=100, got %d", cfg.Display.Waifu.MaxCacheMB)
	}
	if cfg.Starship.Modules.Claude {
		t.Error("expected Starship Claude module disabled")
	}
	if !cfg.Starship.Modules.Billing {
		t.Error("expected Starship Billing module enabled")
	}

	// Defaults preserved for unspecified fields
	if cfg.Accounts.DigitalOcean.APIKeyEnv != "DIGITALOCEAN_TOKEN" {
		t.Errorf("expected default DO APIKeyEnv=DIGITALOCEAN_TOKEN, got %s", cfg.Accounts.DigitalOcean.APIKeyEnv)
	}
	if cfg.Tailscale.APIKeyEnv != "TAILSCALE_API_KEY" {
		t.Errorf("expected default Tailscale APIKeyEnv, got %s", cfg.Tailscale.APIKeyEnv)
	}
}

func TestLoadConfigPartial(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
daemon:
  poll_interval: 5m
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Overridden value
	if cfg.Daemon.PollInterval != "5m" {
		t.Errorf("expected PollInterval=5m, got %s", cfg.Daemon.PollInterval)
	}

	// Defaults preserved
	if cfg.Display.Theme != "monitoring" {
		t.Errorf("expected default Theme=monitoring, got %s", cfg.Display.Theme)
	}
	if !cfg.Starship.Modules.Claude {
		t.Error("expected default Starship Claude module enabled")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
daemon:
  poll_interval: [invalid
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestValidateMissingPollInterval(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Daemon.PollInterval = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for empty poll_interval")
	}
}

func TestValidateMissingCacheDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Daemon.CacheDir = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for empty cache_dir")
	}
}

func TestValidateMissingLogFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Daemon.LogFile = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for empty log_file")
	}
}

func TestValidateInvalidTheme(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Display.Theme = "neon"
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for invalid theme")
	}
}

func TestValidateClaudeAccountMissingName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Accounts.Claude = []ClaudeAccount{
		{Name: "", Type: "subscription", CredentialsPath: "/tmp/creds.json", Enabled: true},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for missing Claude account name")
	}
}

func TestValidateClaudeAccountInvalidType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Accounts.Claude = []ClaudeAccount{
		{Name: "test", Type: "free", Enabled: true},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for invalid Claude account type")
	}
}

func TestValidateClaudeSubscriptionMissingCredentials(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Accounts.Claude = []ClaudeAccount{
		{Name: "test", Type: "subscription", CredentialsPath: "", Enabled: true},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for subscription account missing credentials_path")
	}
}

func TestValidateClaudeAPIMissingKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Accounts.Claude = []ClaudeAccount{
		{Name: "test", Type: "api", APIKeyEnv: "", Enabled: true},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for api account missing api_key_env")
	}
}

func TestValidateClaudeTooManyAccounts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Accounts.Claude = make([]ClaudeAccount, 6)
	for i := range cfg.Accounts.Claude {
		cfg.Accounts.Claude[i] = ClaudeAccount{
			Name:            "acct",
			Type:            "api",
			APIKeyEnv:       "KEY",
			Enabled:         true,
		}
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for more than 5 Claude accounts")
	}
}

func TestValidateNegativeWaifuCache(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Display.Waifu.MaxCacheMB = -1
	if err := cfg.Validate(); err == nil {
		t.Error("expected validation error for negative waifu cache size")
	}
}

func TestSaveAndReloadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yaml")

	cfg := DefaultConfig()
	cfg.Daemon.PollInterval = "1h"
	cfg.Display.Theme = "full"

	if err := SaveConfig(cfg, configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if loaded.Daemon.PollInterval != "1h" {
		t.Errorf("expected PollInterval=1h, got %s", loaded.Daemon.PollInterval)
	}
	if loaded.Display.Theme != "full" {
		t.Errorf("expected Theme=full, got %s", loaded.Display.Theme)
	}
}

func TestXDGPaths(t *testing.T) {
	cfg := DefaultConfig()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	expectedCache := filepath.Join(home, ".cache", "prompt-pulse")
	if cfg.Daemon.CacheDir != expectedCache {
		t.Errorf("expected CacheDir=%s, got %s", expectedCache, cfg.Daemon.CacheDir)
	}

	expectedLog := filepath.Join(home, ".local", "log", "prompt-pulse.log")
	if cfg.Daemon.LogFile != expectedLog {
		t.Errorf("expected LogFile=%s, got %s", expectedLog, cfg.Daemon.LogFile)
	}

	expectedCreds := filepath.Join(home, ".claude", ".credentials.json")
	if len(cfg.Accounts.Claude) > 0 && cfg.Accounts.Claude[0].CredentialsPath != expectedCreds {
		t.Errorf("expected CredentialsPath=%s, got %s", expectedCreds, cfg.Accounts.Claude[0].CredentialsPath)
	}
}
