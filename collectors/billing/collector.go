// Package billing provides the unified cloud billing collector for prompt-pulse.
// It coordinates data collection across Civo, DigitalOcean, AWS, and DreamHost,
// running each provider's fetch concurrently with per-provider error isolation.
package billing

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// collectorName is the unique identifier for this collector.
	collectorName = "billing"

	// collectorDescription describes what this collector gathers.
	collectorDescription = "Cloud provider billing across Civo, DigitalOcean, AWS, and DreamHost"

	// defaultInterval is the recommended polling interval. Billing data
	// changes slowly, so hourly polling is sufficient.
	defaultInterval = 1 * time.Hour
)

// ProviderFetcher abstracts billing data retrieval for a single provider.
type ProviderFetcher interface {
	FetchBilling(ctx context.Context) (*collectors.ProviderBilling, error)
}

// ProviderConfig holds the configuration for a single billing provider.
type ProviderConfig struct {
	// Name identifies the provider: "civo", "digitalocean", "aws", "dreamhost".
	Name string

	// Enabled controls whether this provider is polled during collection.
	Enabled bool

	// APIKeyEnv is the environment variable name holding the API key or token.
	APIKeyEnv string
}

// Package-level factory functions. These create the real client implementations
// by default, but can be overridden in tests to inject mocks.
var (
	// newCivoFetcher creates a ProviderFetcher for Civo accounts.
	newCivoFetcher = func(apiKey, region string, logger *slog.Logger) ProviderFetcher {
		return NewCivoClient(apiKey, region, logger)
	}

	// newDOFetcher creates a ProviderFetcher for DigitalOcean accounts.
	newDOFetcher = func(apiToken string, logger *slog.Logger) ProviderFetcher {
		return NewDOClient(apiToken, logger)
	}

	// newAWSFetcher creates a ProviderFetcher for AWS accounts.
	// The profile parameter is the AWS CLI profile name. The regions slice
	// determines the region used for Cost Explorer API calls.
	newAWSFetcher = func(profile string, regions []string, logger *slog.Logger) ProviderFetcher {
		return NewAWSClient(profile, regions, logger)
	}

	// newDreamHostFetcher creates a ProviderFetcher for DreamHost accounts.
	newDreamHostFetcher = func(apiKey string, logger *slog.Logger) ProviderFetcher {
		return NewDreamHostClient(apiKey, logger)
	}
)

// BillingCollector implements collectors.Collector for cloud billing data.
// It coordinates concurrent data collection across multiple cloud providers,
// isolating per-provider failures so one broken provider does not prevent
// collection from the others.
type BillingCollector struct {
	providers []ProviderConfig
	logger    *slog.Logger
}

// NewBillingCollector creates a BillingCollector for the given providers.
// If logger is nil, a no-op logger is used.
func NewBillingCollector(providers []ProviderConfig, logger *slog.Logger) *BillingCollector {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &BillingCollector{
		providers: providers,
		logger:    logger,
	}
}

// Name returns the collector's unique identifier.
func (b *BillingCollector) Name() string {
	return collectorName
}

// Description returns a human-readable description of what this collector gathers.
func (b *BillingCollector) Description() string {
	return collectorDescription
}

// Interval returns the recommended polling interval for this collector.
func (b *BillingCollector) Interval() time.Duration {
	return defaultInterval
}

// providerResult holds the outcome of collecting data from a single provider.
type providerResult struct {
	billing  collectors.ProviderBilling
	warnings []string
}

// Collect gathers billing data from all enabled cloud providers concurrently.
// Per-provider errors are isolated: a failing provider produces a result with
// Status="error" and a warning, but does not prevent other providers from
// being collected. Only a cancelled context returns an error at the top level.
func (b *BillingCollector) Collect(ctx context.Context) (*collectors.CollectResult, error) {
	// Check for context cancellation before starting.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Filter to enabled providers only.
	var enabled []ProviderConfig
	for _, p := range b.providers {
		if p.Enabled {
			enabled = append(enabled, p)
		} else {
			b.logger.Debug("skipping disabled provider", "provider", p.Name)
		}
	}

	// Collect from all enabled providers concurrently.
	results := make([]providerResult, len(enabled))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allWarnings []string

	for i, p := range enabled {
		wg.Add(1)
		go func(idx int, provider ProviderConfig) {
			defer wg.Done()

			result := b.collectProvider(ctx, provider)

			results[idx] = result

			if len(result.warnings) > 0 {
				mu.Lock()
				allWarnings = append(allWarnings, result.warnings...)
				mu.Unlock()
			}
		}(i, p)
	}

	wg.Wait()

	// Check for context cancellation after all goroutines complete.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Assemble provider billing entries.
	providers := make([]collectors.ProviderBilling, len(results))
	for i, r := range results {
		providers[i] = r.billing
	}

	// Calculate summary totals.
	summary := calculateSummary(providers)

	return &collectors.CollectResult{
		Collector: collectorName,
		Timestamp: time.Now(),
		Data: &collectors.BillingData{
			Providers: providers,
			Total:     summary,
		},
		Warnings: allWarnings,
	}, nil
}

// getAPIKeyFromEnvOrFile looks up an API key from environment variable
// or from a file path specified in ENVVAR_FILE. This supports SOPS-decrypted
// secrets which are mounted as files rather than environment variables.
func getAPIKeyFromEnvOrFile(envVar string) string {
	// 1. Direct env var takes precedence.
	if key := os.Getenv(envVar); key != "" {
		return key
	}
	// 2. File-based lookup (env var + "_FILE" suffix).
	fileEnv := envVar + "_FILE"
	if filePath := os.Getenv(fileEnv); filePath != "" {
		data, err := os.ReadFile(filePath)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// collectProvider fetches billing data for a single provider. It never returns
// an error; failures are captured in the providerResult with an appropriate
// status and warning.
func (b *BillingCollector) collectProvider(ctx context.Context, p ProviderConfig) providerResult {
	// Look up the API key from environment or file.
	apiKey := getAPIKeyFromEnvOrFile(p.APIKeyEnv)
	if apiKey == "" {
		b.logger.Warn("API key not found in environment", "provider", p.Name, "env_var", p.APIKeyEnv)
		return providerResult{
			billing: collectors.ProviderBilling{
				Provider:  p.Name,
				Status:    "error",
				FetchedAt: time.Now(),
			},
			warnings: []string{fmt.Sprintf("provider %q: environment variable %q is empty", p.Name, p.APIKeyEnv)},
		}
	}

	// Create the appropriate client via factory function.
	fetcher, err := b.createFetcher(p.Name, apiKey)
	if err != nil {
		b.logger.Warn("unsupported provider", "provider", p.Name, "error", err)
		return providerResult{
			billing: collectors.ProviderBilling{
				Provider:  p.Name,
				Status:    "error",
				FetchedAt: time.Now(),
			},
			warnings: []string{fmt.Sprintf("provider %q: %v", p.Name, err)},
		}
	}

	// Fetch billing data.
	billing, err := fetcher.FetchBilling(ctx)
	if err != nil {
		b.logger.Warn("failed to fetch billing", "provider", p.Name, "error", err)
		return providerResult{
			billing: collectors.ProviderBilling{
				Provider:  p.Name,
				Status:    "error",
				FetchedAt: time.Now(),
			},
			warnings: []string{fmt.Sprintf("provider %q: %v", p.Name, err)},
		}
	}

	// Ensure status is set.
	if billing.Status == "" {
		billing.Status = "ok"
	}

	return providerResult{
		billing: *billing,
	}
}

// createFetcher returns the appropriate ProviderFetcher for the named provider.
// For AWS, the apiKey parameter is interpreted as the AWS CLI profile name
// (since AWS uses profiles, not API keys). If the profile is empty, "default"
// is used.
func (b *BillingCollector) createFetcher(name, apiKey string) (ProviderFetcher, error) {
	switch name {
	case "civo":
		return newCivoFetcher(apiKey, "NYC1", b.logger), nil
	case "digitalocean":
		return newDOFetcher(apiKey, b.logger), nil
	case "aws":
		profile := apiKey
		if profile == "" {
			profile = "default"
		}
		return newAWSFetcher(profile, []string{"us-east-1"}, b.logger), nil
	case "dreamhost":
		return newDreamHostFetcher(apiKey, b.logger), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", name)
	}
}

// calculateSummary aggregates billing data across all providers into a
// BillingSummary. ForecastUSD is only non-nil if at least one provider
// has a forecast. BudgetUSD is only non-nil if at least one provider
// has a budget.
func calculateSummary(providers []collectors.ProviderBilling) collectors.BillingSummary {
	var summary collectors.BillingSummary
	var hasForecast, hasBudget bool
	var forecastTotal, budgetTotal float64

	for _, p := range providers {
		// Only include successful providers in spend totals.
		if p.Status == "error" {
			continue
		}

		summary.CurrentMonthUSD += p.CurrentMonth.SpendUSD

		if p.CurrentMonth.ForecastUSD != nil {
			hasForecast = true
			forecastTotal += *p.CurrentMonth.ForecastUSD
		}

		if p.CurrentMonth.BudgetUSD != nil {
			hasBudget = true
			budgetTotal += *p.CurrentMonth.BudgetUSD
		}
	}

	if hasForecast {
		summary.ForecastUSD = &forecastTotal
	}

	if hasBudget {
		summary.BudgetUSD = &budgetTotal
	}

	return summary
}

// Compile-time interface compliance check.
var _ collectors.Collector = (*BillingCollector)(nil)
