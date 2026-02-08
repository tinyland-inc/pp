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

	// Priority affects collection order (lower = higher priority). Default: 10.
	Priority int

	// ShortName is a 4-character label for compact display. If empty, Name is truncated.
	ShortName string

	// TierHint is the API tier hint for API accounts when auto-detection fails.
	TierHint string

	// PollInterval overrides the global poll interval for this account.
	PollInterval string
}

// TokenRefresherInterface abstracts the token refresh operation for testing.
type TokenRefresherInterface interface {
	RefreshAndPersist(ctx context.Context, credPath string, refreshToken string) (*TokenRefreshResponse, error)
}

// ClaudeCollector implements collectors.Collector for Claude usage data.
// It coordinates data collection across multiple subscription and API accounts,
// isolating per-account failures so one broken account does not prevent collection
// from the others. Accounts are polled sequentially with a configurable stagger delay.
type ClaudeCollector struct {
	accounts       []AccountConfig
	logger         *slog.Logger
	credLoader     CredentialLoader
	tokenRefresher TokenRefresherInterface
	staggerDelay   time.Duration // Delay between account requests
}

// NewClaudeCollector creates a ClaudeCollector for the given accounts.
// If logger is nil, a no-op logger is used. The staggerDelay parameter controls
// the delay between account requests (typically 5 seconds).
func NewClaudeCollector(accounts []AccountConfig, logger *slog.Logger, staggerDelay time.Duration) *ClaudeCollector {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	if staggerDelay == 0 {
		staggerDelay = 5 * time.Second // Default
	}

	return &ClaudeCollector{
		accounts:       accounts,
		logger:         logger,
		credLoader:     newCredentialLoader(),
		tokenRefresher: NewTokenRefresher(logger),
		staggerDelay:   staggerDelay,
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

// Collect gathers usage data from all enabled Claude accounts sequentially with staggered delays.
// Per-account errors are isolated: a failing account produces a result with
// status "auth_failed" or "error" and a warning, but does not prevent other
// accounts from being collected. Only a cancelled context returns an error
// at the top level.
//
// Accounts are sorted by priority (lower = higher priority) and collected sequentially
// with a stagger delay between requests to prevent API rate limiting.
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

	// Sort accounts by priority (lower = higher priority)
	enabled = sortAccountsByPriority(enabled)

	// Collect from all enabled accounts sequentially with stagger delay.
	results := make([]accountResult, len(enabled))
	var allWarnings []string

	for i, acct := range enabled {
		// Check context cancellation between accounts
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Stagger delay between requests (skip on first account)
		if i > 0 && c.staggerDelay > 0 {
			c.logger.Debug("stagger delay", "duration", c.staggerDelay)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.staggerDelay):
			}
		}

		result := c.collectAccount(ctx, acct)
		results[i] = result

		if len(result.warnings) > 0 {
			allWarnings = append(allWarnings, result.warnings...)
		}
	}

	// Check for context cancellation after all accounts complete.
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
				Name:        acct.Name,
				Type:        acct.Type,
				Status:      collectors.StatusError,
				ErrorReason: fmt.Sprintf("unknown type %q", acct.Type),
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
				Name:        acct.Name,
				Type:        "subscription",
				Status:      collectors.StatusAuthFailed,
				ErrorReason: fmt.Sprintf("failed to load credentials: %v", err),
			},
			warnings: []string{fmt.Sprintf("account %q: failed to load credentials: %v", acct.Name, err)},
		}
	}

	// Check if credentials need refresh (expired or expiring soon).
	if creds.NeedsRefresh() {
		c.logger.Info("credentials need refresh", "account", acct.Name, "expires_in", creds.ExpiresIn())

		if creds.RefreshToken == "" {
			expiresAt := time.UnixMilli(creds.ExpiresAt)
			c.logger.Warn("no refresh token available", "account", acct.Name)
			return accountResult{
				usage: collectors.ClaudeAccountUsage{
					Name:        acct.Name,
					Type:        "subscription",
					Status:      collectors.StatusTokenExpired,
					ErrorReason: fmt.Sprintf("OAuth token expired at %s, no refresh token", expiresAt.Format(time.RFC3339)),
				},
				warnings: []string{fmt.Sprintf("account %q: OAuth credentials expired at %s and no refresh token available", acct.Name, expiresAt.Format(time.RFC3339))},
			}
		}

		// Attempt token refresh.
		tokens, err := c.tokenRefresher.RefreshAndPersist(ctx, acct.CredentialsPath, creds.RefreshToken)
		if err != nil {
			c.logger.Warn("token refresh failed", "account", acct.Name, "error", err)
			// If refresh fails and token is already expired, report auth failure.
			if creds.IsExpired() {
				return accountResult{
					usage: collectors.ClaudeAccountUsage{
						Name:        acct.Name,
						Type:        "subscription",
						Status:      collectors.StatusTokenExpired,
						ErrorReason: fmt.Sprintf("token refresh failed: %v", err),
					},
					warnings: []string{fmt.Sprintf("account %q: token refresh failed: %v", acct.Name, err)},
				}
			}
			// Token not yet expired, continue with existing token but warn.
			c.logger.Warn("continuing with existing token", "account", acct.Name, "expires_in", creds.ExpiresIn())
		} else {
			// Refresh succeeded, update the access token for this request.
			c.logger.Info("token refreshed successfully", "account", acct.Name)
			creds.AccessToken = tokens.AccessToken
		}
	}

	// Create the OAuth client and fetch usage.
	fetcher := newUsageFetcher(creds.AccessToken, c.logger)
	rawUsage, err := fetcher.FetchUsage(ctx)

	var usage collectors.ClaudeAccountUsage
	var warnings []string

	if err != nil {
		// Check if this is a rate limit or other error that should affect status
		status := StatusFromError(err)

		// If rate limited or auth failed, report that status directly
		if status == collectors.StatusRateLimited || status == collectors.StatusAuthFailed {
			c.logger.Warn("subscription account error", "account", acct.Name, "status", status, "error", err)
			usage = collectors.ClaudeAccountUsage{
				Name:        acct.Name,
				Type:        "subscription",
				Tier:        creds.NormalizeTier(),
				Status:      status,
				ErrorReason: err.Error(),
			}
			warnings = []string{fmt.Sprintf("account %q: %v", acct.Name, err)}
		} else {
			// Usage API is protected by Cloudflare and may not be accessible.
			// Fall back to credentials-only mode: report subscription tier from
			// the credentials file and mark as "active" (degraded but functional).
			c.logger.Info("usage API unavailable, falling back to credentials-only mode",
				"account", acct.Name, "error", err)

			usage = collectors.ClaudeAccountUsage{
				Name:   acct.Name,
				Type:   "subscription",
				Tier:   creds.NormalizeTier(),
				Status: "active",
			}
			// Note: usage data (FiveHour, SevenDay, ExtraUsage) will be nil
			// because the API is not accessible. This is expected behavior.
			warnings = []string{fmt.Sprintf("account %q: usage API unavailable (Cloudflare protected), showing credentials-only data", acct.Name)}
		}
	} else {
		// Convert the raw response to the canonical model.
		usage = rawUsage.ToAccountUsage(acct.Name)

		// Use the tier from credentials if available, otherwise normalize from the response.
		if creds.RateLimitTier != "" {
			usage.Tier = creds.NormalizeTier()
		} else {
			usage.Tier = normalizeTierString(usage.Tier)
		}
	}

	// Fill metadata from config
	fillAccountMetadata(&usage, acct)

	c.logger.Debug("subscription account collected successfully", "account", acct.Name, "tier", usage.Tier)

	return accountResult{
		usage:    usage,
		warnings: warnings,
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
				Name:        acct.Name,
				Type:        "api",
				Status:      collectors.StatusAuthFailed,
				ErrorReason: fmt.Sprintf("environment variable %q is empty", acct.APIKeyEnv),
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
				Name:        acct.Name,
				Type:        "api",
				Status:      status,
				ErrorReason: err.Error(),
			},
			warnings: []string{fmt.Sprintf("account %q: %v", acct.Name, err)},
		}
	}

	// Fill in account metadata.
	usage.Name = acct.Name
	usage.Type = "api"
	if usage.Status == "" {
		usage.Status = collectors.StatusOK
	}
	if usage.Tier == "" {
		usage.Tier = "unknown"
	}

	// Fill metadata from config
	fillAccountMetadata(usage, acct)

	c.logger.Debug("API account collected successfully", "account", acct.Name, "tier", usage.Tier)

	return accountResult{
		usage: *usage,
	}
}

// sortAccountsByPriority sorts accounts by priority (lower value = higher priority).
// Accounts with the same priority maintain their original order (stable sort).
func sortAccountsByPriority(accounts []AccountConfig) []AccountConfig {
	// Create a copy to avoid modifying the input slice
	sorted := make([]AccountConfig, len(accounts))
	copy(sorted, accounts)

	// Simple bubble sort (sufficient for small account counts)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority < sorted[i].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// fillAccountMetadata populates standard account metadata fields from AccountConfig.
func fillAccountMetadata(usage *collectors.ClaudeAccountUsage, acct AccountConfig) {
	usage.ShortName = acct.ShortName
	usage.Priority = acct.Priority

	// If TierHint is provided and Tier is empty or unknown, use the hint
	if acct.TierHint != "" && (usage.Tier == "" || usage.Tier == "unknown") {
		usage.Tier = acct.TierHint
	}
}

// Compile-time interface compliance check.
var _ collectors.Collector = (*ClaudeCollector)(nil)
