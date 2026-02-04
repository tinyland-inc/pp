// Package claude provides the multi-account Claude usage collector for prompt-pulse.
// It coordinates data collection across subscription (OAuth) and API-key accounts,
// running each account's fetch concurrently with per-account error isolation.
package claude

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// collectorName is the unique identifier for this collector.
	collectorName = "claude"

	// collectorDescription describes what this collector gathers.
	collectorDescription = "Claude AI usage across subscription and API accounts"

	// defaultInterval is the recommended polling interval.
	defaultInterval = 15 * time.Minute
)

// UsageFetcher fetches subscription usage data. The OAuthClient in oauth.go
// satisfies this interface, returning the raw OAuthUsageResponse which the
// collector converts to the canonical ClaudeAccountUsage model.
type UsageFetcher interface {
	FetchUsage(ctx context.Context) (*OAuthUsageResponse, error)
}

// RateLimitFetcher fetches API rate limit data for API-key accounts.
// The APIClient in api.go implements this interface.
type RateLimitFetcher interface {
	FetchRateLimits(ctx context.Context) (*collectors.ClaudeAccountUsage, error)
}

// CredentialLoader loads OAuth credentials from a file path.
// The fileCredentialLoader in credentials.go provides the production implementation.
type CredentialLoader interface {
	Load(path string) (*OAuthCredential, error)
}

// normalizeTierString converts a raw tier string to its canonical short form
// using the tierMapping defined in credentials.go. Returns the input unchanged
// if no mapping exists, or "pro" for an empty string.
func normalizeTierString(raw string) string {
	if raw == "" {
		return "pro"
	}
	if normalized, ok := tierMapping[raw]; ok {
		return normalized
	}
	return raw
}

// Package-level factory functions. These create the real client implementations
// by default, but can be overridden in tests to inject mocks.
var (
	// newUsageFetcher creates a UsageFetcher for subscription accounts.
	// Default: wraps NewOAuthClient from oauth.go.
	newUsageFetcher = func(accessToken string, logger *slog.Logger) UsageFetcher {
		return NewOAuthClient(accessToken, logger)
	}

	// newRateLimitFetcher creates a RateLimitFetcher for API-key accounts.
	// Default: wraps NewAPIClient from api.go.
	newRateLimitFetcher = func(apiKey string, logger *slog.Logger) RateLimitFetcher {
		return NewAPIClient(apiKey, logger)
	}

	// newCredentialLoader creates a CredentialLoader for reading credential files.
	// Default: uses fileCredentialLoader from credentials.go.
	newCredentialLoader = func() CredentialLoader {
		return &fileCredentialLoader{}
	}
)

// AccountConfig wraps configuration for a single Claude account with runtime state.
type AccountConfig struct {
	// Name is the user-defined label for this account.
	Name string

	// Type is "subscription" or "api".
	Type string

	// CredentialsPath is the filesystem path to the OAuth credentials JSON
	// (subscription accounts only).
	CredentialsPath string

	// APIKeyEnv is the environment variable name that holds the API key
	// (API accounts only).
	APIKeyEnv string

	// Enabled controls whether this account is polled during collection.
	Enabled bool
}

// ClaudeCollector implements collectors.Collector for Claude usage data.
// It coordinates concurrent data collection across multiple subscription
// and API accounts, isolating per-account failures so one broken account
// does not prevent collection from the others.
type ClaudeCollector struct {
	accounts   []AccountConfig
	logger     *slog.Logger
	credLoader CredentialLoader
}

// NewClaudeCollector creates a ClaudeCollector for the given accounts.
// If logger is nil, a no-op logger is used.
func NewClaudeCollector(accounts []AccountConfig, logger *slog.Logger) *ClaudeCollector {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &ClaudeCollector{
		accounts:   accounts,
		logger:     logger,
		credLoader: newCredentialLoader(),
	}
}

// Name returns the collector's unique identifier.
func (c *ClaudeCollector) Name() string {
	return collectorName
}

// Description returns a human-readable description of what this collector gathers.
func (c *ClaudeCollector) Description() string {
	return collectorDescription
}

// Interval returns the recommended polling interval for this collector.
func (c *ClaudeCollector) Interval() time.Duration {
	return defaultInterval
}

// accountResult holds the outcome of collecting data from a single account.
type accountResult struct {
	usage    collectors.ClaudeAccountUsage
	warnings []string
}

// Collect gathers usage data from all enabled Claude accounts concurrently.
// Per-account errors are isolated: a failing account produces a result with
// status "auth_failed" or "error" and a warning, but does not prevent other
// accounts from being collected. Only a cancelled context returns an error
// at the top level.
func (c *ClaudeCollector) Collect(ctx context.Context) (*collectors.CollectResult, error) {
	// Check for context cancellation before starting.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Filter to enabled accounts only.
	var enabled []AccountConfig
	for _, acct := range c.accounts {
		if acct.Enabled {
			enabled = append(enabled, acct)
		} else {
			c.logger.Debug("skipping disabled account", "account", acct.Name)
		}
	}

	// Collect from all enabled accounts concurrently.
	results := make([]accountResult, len(enabled))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allWarnings []string

	for i, acct := range enabled {
		wg.Add(1)
		go func(idx int, account AccountConfig) {
			defer wg.Done()

			result := c.collectAccount(ctx, account)

			results[idx] = result

			if len(result.warnings) > 0 {
				mu.Lock()
				allWarnings = append(allWarnings, result.warnings...)
				mu.Unlock()
			}
		}(i, acct)
	}

	wg.Wait()

	// Check for context cancellation after all goroutines complete.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Assemble the final result.
	accounts := make([]collectors.ClaudeAccountUsage, len(results))
	for i, r := range results {
		accounts[i] = r.usage
	}

	return &collectors.CollectResult{
		Collector: collectorName,
		Timestamp: time.Now(),
		Data: &collectors.ClaudeUsage{
			Accounts: accounts,
		},
		Warnings: allWarnings,
	}, nil
}

// collectAccount fetches data for a single account. It never returns an error;
// failures are captured in the accountResult with an appropriate status and warning.
func (c *ClaudeCollector) collectAccount(ctx context.Context, acct AccountConfig) accountResult {
	switch acct.Type {
	case "subscription":
		return c.collectSubscription(ctx, acct)
	case "api":
		return c.collectAPI(ctx, acct)
	default:
		return accountResult{
			usage: collectors.ClaudeAccountUsage{
				Name:   acct.Name,
				Type:   acct.Type,
				Status: "error",
			},
			warnings: []string{fmt.Sprintf("account %q: unknown type %q", acct.Name, acct.Type)},
		}
	}
}

// collectSubscription handles data collection for a subscription (OAuth) account.
func (c *ClaudeCollector) collectSubscription(ctx context.Context, acct AccountConfig) accountResult {
	c.logger.Debug("collecting subscription account", "account", acct.Name, "credentials_path", acct.CredentialsPath)

	// Load credentials from file.
	creds, err := c.credLoader.Load(acct.CredentialsPath)
	if err != nil {
		c.logger.Warn("failed to load credentials", "account", acct.Name, "error", err)
		return accountResult{
			usage: collectors.ClaudeAccountUsage{
				Name:   acct.Name,
				Type:   "subscription",
				Status: "auth_failed",
			},
			warnings: []string{fmt.Sprintf("account %q: failed to load credentials: %v", acct.Name, err)},
		}
	}

	// Check if credentials have expired.
	if creds.IsExpired() {
		expiresAt := time.UnixMilli(creds.ExpiresAt)
		c.logger.Warn("credentials expired", "account", acct.Name, "expired_at", expiresAt)
		return accountResult{
			usage: collectors.ClaudeAccountUsage{
				Name:   acct.Name,
				Type:   "subscription",
				Status: "auth_failed",
			},
			warnings: []string{fmt.Sprintf("account %q: OAuth credentials expired at %s", acct.Name, expiresAt.Format(time.RFC3339))},
		}
	}

	// Create the OAuth client and fetch usage.
	fetcher := newUsageFetcher(creds.AccessToken, c.logger)
	rawUsage, err := fetcher.FetchUsage(ctx)
	if err != nil {
		status := StatusFromError(err)
		c.logger.Warn("failed to fetch usage", "account", acct.Name, "status", status, "error", err)
		return accountResult{
			usage: collectors.ClaudeAccountUsage{
				Name:   acct.Name,
				Type:   "subscription",
				Status: status,
			},
			warnings: []string{fmt.Sprintf("account %q: %v", acct.Name, err)},
		}
	}

	// Convert the raw response to the canonical model.
	usage := rawUsage.ToAccountUsage(acct.Name)

	// Use the tier from credentials if available, otherwise normalize from the response.
	if creds.RateLimitTier != "" {
		usage.Tier = creds.NormalizeTier()
	} else {
		usage.Tier = normalizeTierString(usage.Tier)
	}

	c.logger.Debug("subscription account collected successfully", "account", acct.Name, "tier", usage.Tier)

	return accountResult{
		usage: usage,
	}
}

// collectAPI handles data collection for an API-key account.
func (c *ClaudeCollector) collectAPI(ctx context.Context, acct AccountConfig) accountResult {
	c.logger.Debug("collecting API account", "account", acct.Name, "api_key_env", acct.APIKeyEnv)

	// Look up the API key from the environment.
	apiKey := os.Getenv(acct.APIKeyEnv)
	if apiKey == "" {
		c.logger.Warn("API key not found in environment", "account", acct.Name, "env_var", acct.APIKeyEnv)
		return accountResult{
			usage: collectors.ClaudeAccountUsage{
				Name:   acct.Name,
				Type:   "api",
				Status: "auth_failed",
			},
			warnings: []string{fmt.Sprintf("account %q: environment variable %q is empty", acct.Name, acct.APIKeyEnv)},
		}
	}

	// Create the API client and fetch rate limits.
	fetcher := newRateLimitFetcher(apiKey, c.logger)
	usage, err := fetcher.FetchRateLimits(ctx)
	if err != nil {
		status := StatusFromError(err)
		c.logger.Warn("failed to fetch rate limits", "account", acct.Name, "status", status, "error", err)
		return accountResult{
			usage: collectors.ClaudeAccountUsage{
				Name:   acct.Name,
				Type:   "api",
				Status: status,
			},
			warnings: []string{fmt.Sprintf("account %q: %v", acct.Name, err)},
		}
	}

	// Fill in account metadata.
	usage.Name = acct.Name
	usage.Type = "api"
	if usage.Status == "" {
		usage.Status = "ok"
	}
	if usage.Tier == "" {
		usage.Tier = "unknown"
	}

	c.logger.Debug("API account collected successfully", "account", acct.Name, "tier", usage.Tier)

	return accountResult{
		usage: *usage,
	}
}

// Compile-time interface compliance check.
var _ collectors.Collector = (*ClaudeCollector)(nil)
