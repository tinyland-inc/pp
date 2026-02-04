package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors/billing"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors/claude"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors/infra"
	"gitlab.com/tinyland/lab/prompt-pulse/config"
)

// daemon manages the background polling loop that periodically runs all
// registered collectors and writes results to the shared cache.
type daemon struct {
	config   *config.Config
	logger   *slog.Logger
	store    *cache.Store
	registry *collectors.Registry
	pidFile  string
	lastRun  map[string]time.Time
	mu       sync.Mutex // protects lastRun
}

// newDaemon creates a daemon with real collectors wired from the configuration.
// It initialises the cache store and registers all collectors in the registry.
func newDaemon(cfg *config.Config, logger *slog.Logger) (*daemon, error) {
	store, err := cache.NewStore(cfg.Daemon.CacheDir, logger)
	if err != nil {
		return nil, fmt.Errorf("daemon: create cache store: %w", err)
	}

	registry := collectors.NewRegistry()

	// Register Claude collector.
	claudeAccounts := configToClaudeAccounts(cfg.Accounts.Claude)
	claudeCollector := claude.NewClaudeCollector(claudeAccounts, logger)
	registry.Register(claudeCollector)

	// Register billing collector.
	billingProviders := configToBillingProviders(cfg.Accounts)
	billingCollector := billing.NewBillingCollector(billingProviders, logger)
	registry.Register(billingCollector)

	// Register infrastructure collector.
	infraCfg := configToInfraConfig(cfg)
	infraCollector := infra.NewInfraCollector(infraCfg, logger)
	registry.Register(infraCollector)

	pidFile := filepath.Join(cfg.Daemon.CacheDir, "prompt-pulse.pid")

	return &daemon{
		config:   cfg,
		logger:   logger,
		store:    store,
		registry: registry,
		pidFile:  pidFile,
		lastRun:  make(map[string]time.Time),
	}, nil
}

// writePIDFile writes the current process PID to the PID file.
// The PID file path is {CacheDir}/prompt-pulse.pid.
func (d *daemon) writePIDFile() error {
	// Ensure the directory exists.
	dir := filepath.Dir(d.pidFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create PID file directory: %w", err)
	}
	pid := os.Getpid()
	data := []byte(strconv.Itoa(pid))
	if err := os.WriteFile(d.pidFile, data, 0o644); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}
	d.logger.Info("wrote PID file", "path", d.pidFile, "pid", pid)
	return nil
}

// removePIDFile removes the PID file on shutdown.
func (d *daemon) removePIDFile() {
	if err := os.Remove(d.pidFile); err != nil && !os.IsNotExist(err) {
		d.logger.Error("failed to remove PID file", "path", d.pidFile, "error", err)
		return
	}
	d.logger.Info("removed PID file", "path", d.pidFile)
}

// isRunning checks if another daemon instance is already running by reading
// the PID file and checking if the process exists. If the PID file contains
// a stale PID (process no longer exists), the file is cleaned up.
func (d *daemon) isRunning() (bool, int) {
	data, err := os.ReadFile(d.pidFile)
	if err != nil {
		return false, 0
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		// Corrupt PID file -- remove it.
		d.logger.Warn("corrupt PID file, removing", "path", d.pidFile, "content", string(data))
		os.Remove(d.pidFile)
		return false, 0
	}

	// Check if the process exists by sending signal 0.
	process, err := os.FindProcess(pid)
	if err != nil {
		// On Unix, FindProcess never returns an error, but handle it anyway.
		os.Remove(d.pidFile)
		return false, 0
	}

	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process does not exist -- stale PID file.
		d.logger.Warn("stale PID file, removing", "path", d.pidFile, "pid", pid)
		os.Remove(d.pidFile)
		return false, 0
	}

	return true, pid
}

// run starts the daemon polling loop. It checks for an existing instance,
// writes a PID file, runs an immediate collection pass, then ticks at the
// configured poll interval until the context is cancelled.
func (d *daemon) run(ctx context.Context) error {
	// Check if another instance is running.
	if running, pid := d.isRunning(); running {
		return fmt.Errorf("daemon already running (PID %d)", pid)
	}

	// Write PID file.
	if err := d.writePIDFile(); err != nil {
		return fmt.Errorf("write PID file: %w", err)
	}
	defer d.removePIDFile()

	interval := parseDuration(d.config.Daemon.PollInterval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start.
	if err := d.runOnce(ctx); err != nil {
		d.logger.Error("initial collection failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("daemon shutting down gracefully")
			d.shutdown()
			return ctx.Err()
		case <-ticker.C:
			if err := d.runOnce(ctx); err != nil {
				d.logger.Error("collection cycle failed", "error", err)
			}
		}
	}
}

// shutdown performs cleanup on daemon exit, logging final cache state.
func (d *daemon) shutdown() {
	d.logger.Info("performing shutdown cleanup")
	if meta, err := d.store.Meta(); err == nil {
		for key, ts := range meta.LastUpdate {
			d.logger.Info("cache entry at shutdown",
				"key", key,
				"age", time.Since(ts).String(),
			)
		}
	}
}

// runOnce performs a single collection pass across all registered collectors.
// Collectors run concurrently via goroutines. Each collector is subject to
// per-collector interval tracking: if a collector ran too recently (based on
// its Interval()), it is skipped for this pass.
func (d *daemon) runOnce(ctx context.Context) error {
	start := time.Now()
	d.logger.Debug("starting collection pass")

	allCollectors := d.registry.All()

	var wg sync.WaitGroup
	for _, c := range allCollectors {
		// Check per-collector interval: skip if last run was too recent.
		d.mu.Lock()
		if lastRun, ok := d.lastRun[c.Name()]; ok {
			if time.Since(lastRun) < c.Interval() {
				d.logger.Debug("skipping collector, interval not elapsed",
					"name", c.Name(),
					"interval", c.Interval(),
					"since_last", time.Since(lastRun),
				)
				d.mu.Unlock()
				continue
			}
		}
		d.mu.Unlock()

		wg.Add(1)
		go func(col collectors.Collector) {
			defer wg.Done()
			d.collectOne(ctx, col)
		}(c)
	}

	wg.Wait()

	elapsed := time.Since(start)
	d.logger.Info("collection pass complete",
		"duration", fmt.Sprintf("%dms", elapsed.Milliseconds()),
	)

	return nil
}

// collectOne runs a single collector, logs warnings, writes results to the
// cache, and updates the lastRun timestamp for per-collector interval tracking.
func (d *daemon) collectOne(ctx context.Context, c collectors.Collector) {
	d.logger.Debug("running collector", "name", c.Name())

	result, err := c.Collect(ctx)
	if err != nil {
		d.logger.Error("collector failed",
			"name", c.Name(),
			"error", err,
		)
		return
	}

	// Log any non-fatal warnings from the collector.
	for _, w := range result.Warnings {
		d.logger.Warn("collector warning",
			"name", c.Name(),
			"warning", w,
		)
	}

	// Write the collected data to the cache.
	if err := d.store.Set(c.Name(), result.Data); err != nil {
		d.logger.Error("cache write failed",
			"name", c.Name(),
			"error", err,
		)
		return
	}

	// Update the last-run timestamp for this collector.
	d.mu.Lock()
	d.lastRun[c.Name()] = time.Now()
	d.mu.Unlock()
}

// configToClaudeAccounts converts config.ClaudeAccount entries to the
// claude.AccountConfig type expected by the Claude collector.
func configToClaudeAccounts(accounts []config.ClaudeAccount) []claude.AccountConfig {
	result := make([]claude.AccountConfig, len(accounts))
	for i, a := range accounts {
		result[i] = claude.AccountConfig{
			Name:            a.Name,
			Type:            a.Type,
			CredentialsPath: a.CredentialsPath,
			APIKeyEnv:       a.APIKeyEnv,
			Enabled:         a.Enabled,
		}
	}
	return result
}

// configToBillingProviders creates billing.ProviderConfig entries from the
// accounts configuration. Each cloud provider (Civo, DigitalOcean, AWS,
// DreamHost) is mapped to a billing provider entry. A provider is considered
// enabled if its API key environment variable is non-empty.
func configToBillingProviders(accounts config.AccountsConfig) []billing.ProviderConfig {
	var providers []billing.ProviderConfig

	if accounts.Civo.APIKeyEnv != "" {
		providers = append(providers, billing.ProviderConfig{
			Name:      "civo",
			Enabled:   true,
			APIKeyEnv: accounts.Civo.APIKeyEnv,
		})
	}

	if accounts.DigitalOcean.APIKeyEnv != "" {
		providers = append(providers, billing.ProviderConfig{
			Name:      "digitalocean",
			Enabled:   true,
			APIKeyEnv: accounts.DigitalOcean.APIKeyEnv,
		})
	}

	if accounts.AWS.Profile != "" {
		// AWS uses profiles, not API keys. The profile name is passed via
		// the APIKeyEnv field so the billing collector can use it as an
		// identifier in environment variable lookup.
		providers = append(providers, billing.ProviderConfig{
			Name:      "aws",
			Enabled:   true,
			APIKeyEnv: "AWS_PROFILE",
		})
	}

	if accounts.DreamHost.APIKeyEnv != "" {
		providers = append(providers, billing.ProviderConfig{
			Name:      "dreamhost",
			Enabled:   true,
			APIKeyEnv: accounts.DreamHost.APIKeyEnv,
		})
	}

	return providers
}

// configToInfraConfig creates an infra.InfraCollectorConfig from the
// application configuration, reading the Tailscale API key from the
// environment variable specified in the config.
func configToInfraConfig(cfg *config.Config) infra.InfraCollectorConfig {
	// Read the Tailscale API key from the configured environment variable.
	var tsAPIKey string
	if cfg.Tailscale.APIKeyEnv != "" {
		tsAPIKey = os.Getenv(cfg.Tailscale.APIKeyEnv)
	}

	// Convert Kubernetes contexts from config to infra types.
	kubeContexts := make([]infra.KubeContextConfig, len(cfg.Kubernetes.Contexts))
	for i, kc := range cfg.Kubernetes.Contexts {
		kubeContexts[i] = infra.KubeContextConfig{
			Name:         kc.Name,
			Kubeconfig:   kc.Kubeconfig,
			Namespace:    kc.Namespace,
			DashboardURL: kc.DashboardURL,
		}
	}

	return infra.InfraCollectorConfig{
		Tailnet:         cfg.Tailscale.Tailnet,
		TailscaleAPIKey: tsAPIKey,
		UseCLIFallback:  cfg.Tailscale.UseCLIFallback,
		KubeContexts:    kubeContexts,
	}
}
