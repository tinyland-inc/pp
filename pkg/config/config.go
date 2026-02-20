package config

// Config is the root configuration for prompt-pulse v2.
type Config struct {
	// General settings
	General GeneralConfig `toml:"general"`

	// Dashboard layout
	Layout LayoutConfig `toml:"layout"`

	// Data collectors
	Collectors CollectorsConfig `toml:"collectors"`

	// Image/waifu settings
	Image ImageConfig `toml:"image"`

	// Theme
	Theme ThemeConfig `toml:"theme"`

	// Shell integration
	Shell ShellConfig `toml:"shell"`

	// Banner mode settings
	Banner BannerConfig `toml:"banner"`
}

// GeneralConfig holds daemon-level general settings.
type GeneralConfig struct {
	// DaemonPollInterval is the base polling interval for the daemon.
	DaemonPollInterval Duration `toml:"daemon_poll_interval"`

	// DataRetention is how long time-series data is kept.
	DataRetention Duration `toml:"data_retention"`

	// LogLevel for daemon logging.
	LogLevel string `toml:"log_level"`

	// CacheDir overrides the default cache directory.
	CacheDir string `toml:"cache_dir"`
}

// LayoutConfig defines the dashboard layout via presets or custom rows.
type LayoutConfig struct {
	// Preset selects a built-in layout preset.
	// Options: "dashboard" (default), "minimal", "ops", "billing"
	Preset string `toml:"preset"`

	// Rows defines custom layout rows. If non-empty, overrides the preset.
	Rows []RowConfig `toml:"row"`
}

// RowConfig defines a single row in a custom layout.
type RowConfig struct {
	// Ratio is the proportional height weight for this row (default: 1).
	Ratio int `toml:"ratio"`

	// Children are the widgets or sub-containers in this row.
	Children []ChildConfig `toml:"child"`
}

// ChildConfig defines a widget or sub-container in a layout row.
type ChildConfig struct {
	// Type is the widget type: "waifu", "claude", "billing", "tailscale", "k8s", "sysmetrics"
	Type string `toml:"type"`

	// Ratio is the proportional width weight for this child (default: 1).
	Ratio int `toml:"ratio"`

	// Children are nested children for sub-rows within a column.
	Children []ChildConfig `toml:"child"`
}

// CollectorsConfig holds settings for all data collectors.
type CollectorsConfig struct {
	SysMetrics SysMetricsCollectorConfig `toml:"sysmetrics"`
	Tailscale  TailscaleCollectorConfig  `toml:"tailscale"`
	Kubernetes K8sCollectorConfig        `toml:"kubernetes"`
	Claude     ClaudeCollectorConfig     `toml:"claude"`
	Billing    BillingCollectorConfig    `toml:"billing"`
}

// SysMetricsCollectorConfig controls system metrics collection.
type SysMetricsCollectorConfig struct {
	Enabled  bool     `toml:"enabled"`
	Interval Duration `toml:"interval"`
}

// TailscaleCollectorConfig controls Tailscale status collection.
type TailscaleCollectorConfig struct {
	Enabled  bool     `toml:"enabled"`
	Interval Duration `toml:"interval"`
}

// K8sCollectorConfig controls Kubernetes status collection.
type K8sCollectorConfig struct {
	Enabled    bool     `toml:"enabled"`
	Interval   Duration `toml:"interval"`
	Contexts   []string `toml:"contexts"`
	Namespaces []string `toml:"namespaces"`
}

// ClaudeCollectorConfig controls Claude usage collection.
type ClaudeCollectorConfig struct {
	Enabled  bool     `toml:"enabled"`
	Interval Duration `toml:"interval"`

	// AdminKey is the Anthropic Admin API key.
	// Prefer setting via ANTHROPIC_ADMIN_KEY environment variable instead
	// of storing in the config file.
	AdminKey string `toml:"admin_key"`

	// Accounts holds per-account configurations.
	Accounts []ClaudeAccountConfig `toml:"account"`
}

// ClaudeAccountConfig represents a single Claude account entry.
type ClaudeAccountConfig struct {
	// Name is the display name for this account.
	Name string `toml:"name"`

	// AdminKey is the per-account admin key.
	// Prefer setting via environment variable instead of config file.
	AdminKey string `toml:"admin_key"`

	// OrganizationID is the Anthropic organization identifier.
	// If empty, auto-discovered via GET /v1/organizations.
	OrganizationID string `toml:"organization_id"`
}

// BillingCollectorConfig controls billing data collection.
type BillingCollectorConfig struct {
	Enabled      bool     `toml:"enabled"`
	Interval     Duration `toml:"interval"`
	Civo         CivoConfig `toml:"civo"`
	DigitalOcean DOConfig   `toml:"digitalocean"`
}

// CivoConfig holds Civo cloud billing settings.
type CivoConfig struct {
	Enabled bool `toml:"enabled"`

	// APIKey for Civo API access.
	// Prefer setting via CIVO_TOKEN environment variable.
	APIKey string `toml:"api_key"`
}

// DOConfig holds DigitalOcean billing settings.
type DOConfig struct {
	Enabled bool `toml:"enabled"`

	// APIKey for DigitalOcean API access.
	// Prefer setting via DIGITALOCEAN_TOKEN environment variable.
	APIKey string `toml:"api_key"`
}

// ImageConfig holds image and waifu display settings.
type ImageConfig struct {
	// Protocol override: "auto", "kitty", "iterm2", "sixel", "halfblocks", "none"
	Protocol string `toml:"protocol"`

	// MaxCacheSizeMB is the maximum disk cache size for images in MB.
	MaxCacheSizeMB int `toml:"max_cache_size_mb"`

	// MaxSessions is the maximum number of per-session cached images.
	MaxSessions int `toml:"max_sessions"`

	// WaifuEnabled toggles waifu image display.
	WaifuEnabled bool `toml:"waifu_enabled"`

	// WaifuCategory for API fetching.
	WaifuCategory string `toml:"waifu_category"`
}

// ThemeConfig selects the visual theme.
type ThemeConfig struct {
	// Name of the built-in theme.
	// Options: "default", "gruvbox", "nord", "catppuccin", "dracula", "tokyo-night"
	Name string `toml:"name"`
}

// ShellConfig holds shell integration settings.
type ShellConfig struct {
	// TUIKeybinding for TUI toggle.
	TUIKeybinding string `toml:"tui_keybinding"`

	// ShowBannerOnStartup shows a banner when a new shell starts.
	ShowBannerOnStartup bool `toml:"show_banner_on_startup"`

	// BannerTimeout is the max time to wait for banner data.
	BannerTimeout Duration `toml:"banner_timeout"`

	// InstantBanner uses pre-rendered cache for <1ms display.
	InstantBanner bool `toml:"instant_banner"`
}

// BannerConfig holds terminal width threshold overrides for banner modes.
type BannerConfig struct {
	// CompactMaxWidth is the max terminal width for compact mode.
	CompactMaxWidth int `toml:"compact_max_width"`

	// StandardMinWidth is the min terminal width for standard mode.
	StandardMinWidth int `toml:"standard_min_width"`

	// WideMinWidth is the min terminal width for wide mode.
	WideMinWidth int `toml:"wide_min_width"`

	// UltraWideMinWidth is the min terminal width for ultra-wide mode.
	UltraWideMinWidth int `toml:"ultrawide_min_width"`
}
