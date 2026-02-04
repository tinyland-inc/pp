// Package config provides configuration parsing for prompt-pulse.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the prompt-pulse daemon configuration.
type Config struct {
	// Daemon holds daemon-level settings.
	Daemon DaemonConfig `yaml:"daemon"`

	// Accounts holds provider account configurations.
	Accounts AccountsConfig `yaml:"accounts"`

	// Tailscale holds Tailscale mesh networking settings.
	Tailscale TailscaleConfig `yaml:"tailscale"`

	// Kubernetes holds cluster connection settings.
	Kubernetes KubernetesConfig `yaml:"kubernetes"`

	// Display holds TUI rendering settings.
	Display DisplayConfig `yaml:"display"`

	// Starship holds starship prompt module toggles.
	Starship StarshipConfig `yaml:"starship"`
}

// DaemonConfig holds daemon-level settings.
type DaemonConfig struct {
	// PollInterval is a duration string (e.g. "15m", "1h") between data collection cycles.
	PollInterval string `yaml:"poll_interval"`
	// CacheDir is the directory for cached API responses.
	CacheDir string `yaml:"cache_dir"`
	// LogFile is the path for daemon log output.
	LogFile string `yaml:"log_file"`
}

// AccountsConfig holds all provider account configurations.
type AccountsConfig struct {
	// Claude holds Claude AI account entries.
	Claude []ClaudeAccount `yaml:"claude"`
	// Civo holds Civo cloud account settings.
	Civo CivoAccount `yaml:"civo"`
	// DigitalOcean holds DigitalOcean account settings.
	DigitalOcean DigitalOceanAccount `yaml:"digitalocean"`
	// AWS holds AWS account settings.
	AWS AWSAccount `yaml:"aws"`
	// DreamHost holds DreamHost account settings.
	DreamHost DreamHostAccount `yaml:"dreamhost"`
}

// ClaudeAccount represents a single Claude account entry.
type ClaudeAccount struct {
	// Name is a human-readable label for this account.
	Name string `yaml:"name"`
	// Type is the account type: "subscription" or "api".
	Type string `yaml:"type"`
	// CredentialsPath is the filesystem path to a credentials JSON file (subscription accounts).
	CredentialsPath string `yaml:"credentials_path"`
	// APIKeyEnv is the environment variable holding an API key (api accounts).
	APIKeyEnv string `yaml:"api_key_env"`
	// Enabled controls whether this account is polled.
	Enabled bool `yaml:"enabled"`
}

// CivoAccount holds Civo cloud settings.
type CivoAccount struct {
	// APIKeyEnv is the environment variable holding the Civo API key.
	APIKeyEnv string `yaml:"api_key_env"`
	// Region is the default Civo region.
	Region string `yaml:"region"`
}

// DigitalOceanAccount holds DigitalOcean settings.
type DigitalOceanAccount struct {
	// APIKeyEnv is the environment variable holding the DigitalOcean token.
	APIKeyEnv string `yaml:"api_key_env"`
}

// AWSAccount holds AWS settings.
type AWSAccount struct {
	// Profile is the AWS CLI profile name.
	Profile string `yaml:"profile"`
	// Regions is the list of AWS regions to query.
	Regions []string `yaml:"regions"`
}

// DreamHostAccount holds DreamHost settings.
type DreamHostAccount struct {
	// APIKeyEnv is the environment variable holding the DreamHost API key.
	APIKeyEnv string `yaml:"api_key_env"`
}

// TailscaleConfig holds Tailscale mesh networking settings.
type TailscaleConfig struct {
	// Tailnet is the tailnet name.
	Tailnet string `yaml:"tailnet"`
	// APIKeyEnv is the environment variable holding the Tailscale API key.
	APIKeyEnv string `yaml:"api_key_env"`
	// UseCLIFallback enables falling back to the tailscale CLI when the API is unavailable.
	UseCLIFallback bool `yaml:"use_cli_fallback"`
	// CollectNodeMetrics enables SSH-based system metrics collection for online nodes.
	CollectNodeMetrics bool `yaml:"collect_node_metrics"`
	// NodeMetricsSSHUser is the username for SSH connections.
	NodeMetricsSSHUser string `yaml:"node_metrics_ssh_user"`
	// NodeMetricsTimeout is the SSH timeout duration string (e.g. "5s").
	NodeMetricsTimeout string `yaml:"node_metrics_timeout"`
}

// KubernetesConfig holds cluster connection settings.
type KubernetesConfig struct {
	// Contexts is the list of Kubernetes cluster contexts to monitor.
	Contexts []KubeContext `yaml:"contexts"`
}

// KubeContext represents a single Kubernetes cluster context.
type KubeContext struct {
	// Name is the kubectl context name.
	Name string `yaml:"name"`
	// Kubeconfig is the path to the kubeconfig file.
	Kubeconfig string `yaml:"kubeconfig"`
	// Namespace is the default namespace to query.
	Namespace string `yaml:"namespace"`
	// DashboardURL is the URL to the cluster dashboard.
	DashboardURL string `yaml:"dashboard_url"`
}

// DisplayConfig holds TUI rendering settings.
type DisplayConfig struct {
	// Theme selects the display theme: "minimal", "full", or "monitoring".
	Theme string `yaml:"theme"`
	// EnableHyperlinks enables OSC 8 terminal hyperlinks.
	EnableHyperlinks bool `yaml:"enable_hyperlinks"`
	// Waifu holds waifu image display settings.
	Waifu WaifuConfig `yaml:"waifu"`
}

// WaifuConfig holds waifu image display settings.
type WaifuConfig struct {
	// Enabled controls whether waifu images are displayed.
	Enabled bool `yaml:"enabled"`
	// Category is the waifu image category (e.g. "neko").
	Category string `yaml:"category"`
	// CacheTTL is a duration string for how long cached images remain valid.
	CacheTTL string `yaml:"cache_ttl"`
	// MaxCacheMB is the maximum cache size in megabytes.
	MaxCacheMB int `yaml:"max_cache_mb"`
}

// StarshipConfig holds starship prompt module toggles.
type StarshipConfig struct {
	// Modules controls which starship modules are enabled.
	Modules StarshipModules `yaml:"modules"`
}

// StarshipModules holds individual module toggles.
type StarshipModules struct {
	// Claude enables the Claude usage module.
	Claude bool `yaml:"claude"`
	// Billing enables the billing summary module.
	Billing bool `yaml:"billing"`
	// Infra enables the infrastructure status module.
	Infra bool `yaml:"infra"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()

	return &Config{
		Daemon: DaemonConfig{
			PollInterval: "15m",
			CacheDir:     filepath.Join(home, ".cache", "prompt-pulse"),
			LogFile:      filepath.Join(home, ".local", "log", "prompt-pulse.log"),
		},
		Accounts: AccountsConfig{
			Claude: []ClaudeAccount{
				{
					Name:            "primary",
					Type:            "subscription",
					CredentialsPath: filepath.Join(home, ".claude", ".credentials.json"),
					APIKeyEnv:       "",
					Enabled:         true,
				},
			},
			Civo: CivoAccount{
				APIKeyEnv: "CIVO_API_KEY",
				Region:    "NYC1",
			},
			DigitalOcean: DigitalOceanAccount{
				APIKeyEnv: "DIGITALOCEAN_TOKEN",
			},
			AWS: AWSAccount{
				Profile: "default",
				Regions: []string{"us-east-1"},
			},
			DreamHost: DreamHostAccount{
				APIKeyEnv: "DREAMHOST_API_KEY",
			},
		},
		Tailscale: TailscaleConfig{
			Tailnet:            "",
			APIKeyEnv:          "TAILSCALE_API_KEY",
			UseCLIFallback:     true,
			CollectNodeMetrics: false,
			NodeMetricsSSHUser: "",
			NodeMetricsTimeout: "5s",
		},
		Kubernetes: KubernetesConfig{
			Contexts: []KubeContext{},
		},
		Display: DisplayConfig{
			Theme:            "monitoring",
			EnableHyperlinks: true,
			Waifu: WaifuConfig{
				Enabled:    false,
				Category:   "neko",
				CacheTTL:   "24h",
				MaxCacheMB: 50,
			},
		},
		Starship: StarshipConfig{
			Modules: StarshipModules{
				Claude:  true,
				Billing: true,
				Infra:   true,
			},
		},
	}
}

// LoadConfig loads configuration from a YAML file, merging with defaults.
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	if path == "" {
		return config, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// Validate checks the configuration for required fields and logical consistency.
func (c *Config) Validate() error {
	// Daemon validation
	if c.Daemon.PollInterval == "" {
		return fmt.Errorf("daemon.poll_interval is required")
	}
	if c.Daemon.CacheDir == "" {
		return fmt.Errorf("daemon.cache_dir is required")
	}
	if c.Daemon.LogFile == "" {
		return fmt.Errorf("daemon.log_file is required")
	}

	// Claude accounts validation
	for i, acct := range c.Accounts.Claude {
		if acct.Name == "" {
			return fmt.Errorf("accounts.claude[%d].name is required", i)
		}
		if acct.Type != "subscription" && acct.Type != "api" {
			return fmt.Errorf("accounts.claude[%d].type must be 'subscription' or 'api', got %q", i, acct.Type)
		}
		if acct.Type == "subscription" && acct.CredentialsPath == "" {
			return fmt.Errorf("accounts.claude[%d].credentials_path is required for subscription accounts", i)
		}
		if acct.Type == "api" && acct.APIKeyEnv == "" {
			return fmt.Errorf("accounts.claude[%d].api_key_env is required for api accounts", i)
		}
	}
	if len(c.Accounts.Claude) > 5 {
		return fmt.Errorf("accounts.claude supports a maximum of 5 accounts, got %d", len(c.Accounts.Claude))
	}

	// Display validation
	validThemes := map[string]bool{"minimal": true, "full": true, "monitoring": true}
	if !validThemes[c.Display.Theme] {
		return fmt.Errorf("display.theme must be 'minimal', 'full', or 'monitoring', got %q", c.Display.Theme)
	}

	// Waifu cache validation
	if c.Display.Waifu.MaxCacheMB < 0 {
		return fmt.Errorf("display.waifu.max_cache_mb must be non-negative, got %d", c.Display.Waifu.MaxCacheMB)
	}

	return nil
}

// SaveConfig saves configuration to a YAML file.
func SaveConfig(config *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
