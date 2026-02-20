package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Load reads configuration from the standard config path.
// Search order:
//  1. $XDG_CONFIG_HOME/prompt-pulse/config.toml
//  2. ~/.config/prompt-pulse/config.toml
//
// If no file exists, returns DefaultConfig().
func Load() (*Config, error) {
	paths := configSearchPaths()
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return LoadFromFile(p)
		}
	}
	return DefaultConfig(), nil
}

// LoadFromFile reads configuration from a specific file path.
func LoadFromFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	defer f.Close()
	return LoadFromReader(f)
}

// LoadFromReader reads configuration from an io.Reader.
func LoadFromReader(r io.Reader) (*Config, error) {
	cfg := DefaultConfig()
	if _, err := toml.NewDecoder(r).Decode(cfg); err != nil {
		return nil, err
	}
	applyEnvOverrides(cfg)
	return cfg, nil
}

// DefaultConfig returns the default configuration with sensible defaults.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(xdgCacheHome(home), "prompt-pulse")

	return &Config{
		General: GeneralConfig{
			DaemonPollInterval: Duration{15 * time.Minute},
			DataRetention:      Duration{10 * time.Minute},
			LogLevel:           "info",
			CacheDir:           cacheDir,
		},
		Layout: LayoutConfig{
			Preset: "dashboard",
		},
		Collectors: CollectorsConfig{
			SysMetrics: SysMetricsCollectorConfig{
				Enabled:  true,
				Interval: Duration{1 * time.Second},
			},
			Tailscale: TailscaleCollectorConfig{
				Enabled:  true,
				Interval: Duration{30 * time.Second},
			},
			Kubernetes: K8sCollectorConfig{
				Enabled:  false,
				Interval: Duration{60 * time.Second},
			},
			Claude: ClaudeCollectorConfig{
				Enabled:  true,
				Interval: Duration{5 * time.Minute},
			},
			Billing: BillingCollectorConfig{
				Enabled:  false,
				Interval: Duration{15 * time.Minute},
			},
		},
		Image: ImageConfig{
			Protocol:       "auto",
			MaxCacheSizeMB: 50,
			MaxSessions:    10,
			WaifuEnabled:   true,
			WaifuCategory:  "waifu",
		},
		Theme: ThemeConfig{
			Name: "default",
		},
		Shell: ShellConfig{
			TUIKeybinding:       `\C-p`,
			ShowBannerOnStartup: true,
			BannerTimeout:       Duration{2 * time.Second},
			InstantBanner:       true,
		},
		Banner: BannerConfig{
			CompactMaxWidth:   80,
			StandardMinWidth:  120,
			WideMinWidth:      160,
			UltraWideMinWidth: 200,
		},
	}
}

// applyEnvOverrides checks environment variables and overrides config values.
// Direct env vars take precedence over _FILE variants (sops-nix pattern).
func applyEnvOverrides(cfg *Config) {
	// Multi-account: ANTHROPIC_ADMIN_KEYS_FILE contains "name:key" lines.
	if v := readEnvFile("ANTHROPIC_ADMIN_KEYS_FILE"); v != "" {
		for _, line := range strings.Split(v, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			name, key, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			cfg.Collectors.Claude.Accounts = append(cfg.Collectors.Claude.Accounts, ClaudeAccountConfig{
				Name:     strings.TrimSpace(name),
				AdminKey: strings.TrimSpace(key),
			})
		}
	}
	// Single-account fallback: ANTHROPIC_ADMIN_KEY or _FILE.
	if len(cfg.Collectors.Claude.Accounts) == 0 {
		if v := os.Getenv("ANTHROPIC_ADMIN_KEY"); v != "" {
			cfg.Collectors.Claude.AdminKey = v
		} else if v := readEnvFile("ANTHROPIC_ADMIN_KEY_FILE"); v != "" {
			cfg.Collectors.Claude.AdminKey = v
		}
	}
	if v := os.Getenv("CIVO_TOKEN"); v != "" {
		cfg.Collectors.Billing.Civo.APIKey = v
	} else if v := readEnvFile("CIVO_API_KEY_FILE"); v != "" {
		cfg.Collectors.Billing.Civo.APIKey = v
	}
	if v := os.Getenv("CIVO_REGION"); v != "" {
		cfg.Collectors.Billing.Civo.Region = v
	}
	if v := os.Getenv("DIGITALOCEAN_TOKEN"); v != "" {
		cfg.Collectors.Billing.DigitalOcean.APIKey = v
	} else if v := readEnvFile("DIGITALOCEAN_TOKEN_FILE"); v != "" {
		cfg.Collectors.Billing.DigitalOcean.APIKey = v
	}
	if v := os.Getenv("PPULSE_PROTOCOL"); v != "" {
		cfg.Image.Protocol = v
	}
	if v := os.Getenv("PPULSE_THEME"); v != "" {
		cfg.Theme.Name = v
	}
	if v := os.Getenv("PPULSE_LAYOUT"); v != "" {
		cfg.Layout.Preset = v
	}
}

// readEnvFile reads the content of a file whose path is given by an
// environment variable. This supports the sops-nix pattern where secrets
// are decrypted to files and their paths exported as *_FILE env vars.
// Returns empty string if the env var is unset or the file can't be read.
func readEnvFile(envVar string) string {
	path := os.Getenv(envVar)
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// Trim trailing newline (common in secret files).
	s := string(data)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// configSearchPaths returns the ordered list of config file paths to try.
func configSearchPaths() []string {
	home, _ := os.UserHomeDir()
	var paths []string

	xdg := xdgConfigHome(home)
	paths = append(paths, filepath.Join(xdg, "prompt-pulse", "config.toml"))

	// If XDG_CONFIG_HOME was explicitly set, also try the fallback default.
	defaultXDG := filepath.Join(home, ".config")
	if xdg != defaultXDG {
		paths = append(paths, filepath.Join(defaultXDG, "prompt-pulse", "config.toml"))
	}

	return paths
}

// xdgConfigHome returns XDG_CONFIG_HOME or ~/.config as fallback.
func xdgConfigHome(home string) string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	return filepath.Join(home, ".config")
}

// xdgCacheHome returns XDG_CACHE_HOME or ~/.cache as fallback.
func xdgCacheHome(home string) string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return v
	}
	return filepath.Join(home, ".cache")
}
