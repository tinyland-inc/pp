// Package infra provides infrastructure status collectors for prompt-pulse.
// It gathers mesh network and cluster state from Tailscale and Kubernetes
// APIs, returning canonical data structures for dashboard rendering.
package infra

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// collectorName is the unique identifier for this collector.
	collectorName = "infra"

	// collectorDescription describes what this collector gathers.
	collectorDescription = "Infrastructure status across Tailscale mesh and Kubernetes clusters"

	// defaultInterval is the recommended polling interval.
	defaultInterval = 5 * time.Minute
)

// Package-level factory functions. These create the real client implementations
// by default, but can be overridden in tests to inject mocks.
var (
	// newTailscaleAPIFetcher creates a TailscaleFetcher using the HTTP API.
	newTailscaleAPIFetcher = func(tailnet, apiKey string, logger *slog.Logger) TailscaleFetcher {
		return NewTailscaleAPIClient(tailnet, apiKey, logger)
	}

	// newTailscaleCLIFetcher creates a TailscaleFetcher using the CLI binary.
	newTailscaleCLIFetcher = func(logger *slog.Logger) TailscaleFetcher {
		return NewTailscaleCLIClient(logger)
	}

	// newKubernetesFetcher creates a KubernetesFetcher using kubectl.
	newKubernetesFetcher = func(logger *slog.Logger) KubernetesFetcher {
		return NewKubectlClient(logger)
	}
)

// InfraCollectorConfig holds the configuration for the infrastructure collector.
type InfraCollectorConfig struct {
	// Tailnet is the Tailscale tailnet name (e.g., "tinyland.ts.net").
	Tailnet string

	// TailscaleAPIKey is the Tailscale API key for HTTP API access.
	// If empty, only CLI fallback is available.
	TailscaleAPIKey string

	// UseCLIFallback enables falling back to the tailscale CLI binary
	// when the API client fails or no API key is configured.
	UseCLIFallback bool

	// KubeContexts lists the Kubernetes clusters to monitor.
	KubeContexts []KubeContextConfig
}

// InfraCollector implements collectors.Collector for infrastructure status.
// It coordinates data collection across Tailscale mesh and Kubernetes clusters,
// isolating per-component failures so one failing cluster does not prevent
// collection from the others.
type InfraCollector struct {
	config InfraCollectorConfig
	logger *slog.Logger
}

// NewInfraCollector creates an InfraCollector with the given configuration.
// If logger is nil, a no-op logger is used.
func NewInfraCollector(config InfraCollectorConfig, logger *slog.Logger) *InfraCollector {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &InfraCollector{
		config: config,
		logger: logger,
	}
}

// Name returns the collector's unique identifier.
func (c *InfraCollector) Name() string {
	return collectorName
}

// Description returns a human-readable description of what this collector gathers.
func (c *InfraCollector) Description() string {
	return collectorDescription
}

// Interval returns the recommended polling interval for this collector.
func (c *InfraCollector) Interval() time.Duration {
	return defaultInterval
}

// Collect gathers infrastructure status from Tailscale and Kubernetes.
// Tailscale is fetched first (API with optional CLI fallback), then all
// Kubernetes clusters are fetched concurrently. Per-component failures
// produce warnings but do not prevent other components from being collected.
// Only a cancelled context returns an error at the top level.
func (c *InfraCollector) Collect(ctx context.Context) (*collectors.CollectResult, error) {
	// Check for context cancellation before starting.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var warnings []string

	// Fetch Tailscale status.
	tsStatus, tsWarnings := c.collectTailscale(ctx)
	warnings = append(warnings, tsWarnings...)

	// Fetch Kubernetes clusters concurrently.
	clusters, k8sWarnings := c.collectKubernetes(ctx)
	warnings = append(warnings, k8sWarnings...)

	// Check for context cancellation after collection completes.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return &collectors.CollectResult{
		Collector: collectorName,
		Timestamp: time.Now(),
		Data: &collectors.InfraStatus{
			Tailscale:  tsStatus,
			Kubernetes: clusters,
		},
		Warnings: warnings,
	}, nil
}

// collectTailscale fetches Tailscale mesh status using the API client first,
// with optional CLI fallback. Returns nil status (not an error) if both fail.
func (c *InfraCollector) collectTailscale(ctx context.Context) (*collectors.TailscaleStatus, []string) {
	var warnings []string

	// If no API key and no CLI fallback, skip Tailscale entirely.
	if c.config.TailscaleAPIKey == "" && !c.config.UseCLIFallback {
		c.logger.Debug("tailscale not configured (no API key, CLI fallback disabled)")
		return nil, nil
	}

	// Try API client first if an API key is available.
	if c.config.TailscaleAPIKey != "" {
		c.logger.Debug("fetching tailscale status via API", "tailnet", c.config.Tailnet)

		fetcher := newTailscaleAPIFetcher(c.config.Tailnet, c.config.TailscaleAPIKey, c.logger)
		status, err := fetcher.FetchStatus(ctx)
		if err == nil {
			return status, nil
		}

		c.logger.Warn("tailscale API failed", "error", err)
		warnings = append(warnings, fmt.Sprintf("tailscale API: %v", err))

		// Fall through to CLI fallback if enabled.
		if !c.config.UseCLIFallback {
			return nil, warnings
		}
	}

	// Try CLI fallback.
	c.logger.Debug("fetching tailscale status via CLI fallback")

	fetcher := newTailscaleCLIFetcher(c.logger)
	status, err := fetcher.FetchStatus(ctx)
	if err == nil {
		return status, warnings
	}

	c.logger.Warn("tailscale CLI fallback failed", "error", err)
	warnings = append(warnings, fmt.Sprintf("tailscale CLI: %v", err))

	return nil, warnings
}

// clusterResult holds the outcome of fetching a single Kubernetes cluster.
type clusterResult struct {
	cluster  collectors.KubernetesCluster
	warnings []string
}

// collectKubernetes fetches status from all configured Kubernetes clusters
// concurrently. Failed clusters are included with status="offline" and a
// warning, preserving the input order.
func (c *InfraCollector) collectKubernetes(ctx context.Context) ([]collectors.KubernetesCluster, []string) {
	if len(c.config.KubeContexts) == 0 {
		return nil, nil
	}

	results := make([]clusterResult, len(c.config.KubeContexts))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allWarnings []string

	fetcher := newKubernetesFetcher(c.logger)

	for i, kubeCtx := range c.config.KubeContexts {
		wg.Add(1)
		go func(idx int, kc KubeContextConfig) {
			defer wg.Done()

			c.logger.Debug("fetching kubernetes cluster", "context", kc.Name, "platform", kc.Platform)

			cluster, err := fetcher.FetchCluster(ctx, kc)
			if err != nil {
				c.logger.Warn("kubernetes cluster fetch failed",
					"context", kc.Name,
					"error", err,
				)

				result := clusterResult{
					cluster: collectors.KubernetesCluster{
						Name:         kc.Name,
						Platform:     kc.Platform,
						Status:       "offline",
						DashboardURL: kc.DashboardURL,
					},
					warnings: []string{fmt.Sprintf("kubernetes %q: %v", kc.Name, err)},
				}
				results[idx] = result

				mu.Lock()
				allWarnings = append(allWarnings, result.warnings...)
				mu.Unlock()
				return
			}

			results[idx] = clusterResult{
				cluster: *cluster,
			}
		}(i, kubeCtx)
	}

	wg.Wait()

	clusters := make([]collectors.KubernetesCluster, len(results))
	for i, r := range results {
		clusters[i] = r.cluster
	}

	return clusters, allWarnings
}

// Compile-time interface compliance check.
var _ collectors.Collector = (*InfraCollector)(nil)
