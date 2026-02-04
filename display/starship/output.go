package starship

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// Cache key constants used to read collector data from the file-based cache.
const (
	CacheKeyClaude  = "claude"
	CacheKeyBilling = "billing"
	CacheKeyInfra   = "infra"
)

// OutputConfig holds the configuration for the Starship output module.
type OutputConfig struct {
	// CacheDir is the directory where prompt-pulse stores cached collector data.
	CacheDir string

	// CacheTTL is the maximum age of cached data before it is considered stale.
	// Stale data is still displayed but marked with a "?" suffix.
	CacheTTL time.Duration

	// Logger is used for diagnostic messages. A no-op logger is used if nil.
	Logger *slog.Logger
}

// DefaultOutputConfig returns an OutputConfig with sensible defaults:
// cache directory at ~/.cache/prompt-pulse, 30-minute TTL, and a no-op logger.
func DefaultOutputConfig() OutputConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return OutputConfig{
		CacheDir: filepath.Join(home, ".cache", "prompt-pulse"),
		CacheTTL: 30 * time.Minute,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// Output reads cached collector data and formats it as one-line strings
// suitable for Starship custom modules.
type Output struct {
	store  *cache.Store
	config OutputConfig
}

// NewOutput creates a new Output with the given configuration.
// It initialises the underlying cache store and returns an error if the
// cache directory cannot be created.
func NewOutput(cfg OutputConfig) (*Output, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	store, err := cache.NewStore(cfg.CacheDir, logger)
	if err != nil {
		return nil, fmt.Errorf("starship: create cache store: %w", err)
	}

	return &Output{
		store:  store,
		config: cfg,
	}, nil
}

// Module is the main entry point for Starship integration. It reads cached
// data for the named module and returns a formatted one-line string.
//
// Supported module names: "claude", "billing", "infra".
//
// If no cached data exists (cache miss), an empty string is returned so that
// Starship hides the module. If the data is stale (older than CacheTTL),
// the output is suffixed with " ?" to signal staleness.
//
// An error is returned only for unknown module names.
func (o *Output) Module(module string) (string, error) {
	switch module {
	case CacheKeyClaude:
		return o.Claude(), nil
	case CacheKeyBilling:
		return o.Billing(), nil
	case CacheKeyInfra:
		return o.Infra(), nil
	default:
		return "", fmt.Errorf("starship: unknown module %q", module)
	}
}

// Claude reads cached ClaudeUsage data and returns a formatted string.
// Returns an empty string on cache miss or when no accounts are present.
func (o *Output) Claude() string {
	data, fresh, err := cache.GetTyped[collectors.ClaudeUsage](o.store, CacheKeyClaude, o.config.CacheTTL)
	if err != nil || data == nil {
		return ""
	}

	output := data.StarshipOutput()
	if output == "" {
		return ""
	}

	if !fresh {
		output += " ?"
	}
	return output
}

// Billing reads cached BillingData and returns a formatted string.
// Returns an empty string on cache miss.
func (o *Output) Billing() string {
	data, fresh, err := cache.GetTyped[collectors.BillingData](o.store, CacheKeyBilling, o.config.CacheTTL)
	if err != nil || data == nil {
		return ""
	}

	output := data.StarshipOutput()
	if output == "" {
		return ""
	}

	if !fresh {
		output += " ?"
	}
	return output
}

// Infra reads cached InfraStatus data and returns a formatted string.
// Returns an empty string on cache miss.
func (o *Output) Infra() string {
	data, fresh, err := cache.GetTyped[collectors.InfraStatus](o.store, CacheKeyInfra, o.config.CacheTTL)
	if err != nil || data == nil {
		return ""
	}

	output := data.StarshipOutput()
	if output == "" {
		return ""
	}

	if !fresh {
		output += " ?"
	}
	return output
}
