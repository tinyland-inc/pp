package main

import (
	"os"
	"path/filepath"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/app"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/billing"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/claude"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/claudepersonal"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/k8s"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/sysmetrics"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/tailscale"
	waifuCollector "gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/waifu"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/config"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/image"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/terminal"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/waifu"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/widgets"
)

// buildTUIWidgetsAndCollectors creates widget instances and registers the
// matching collectors based on the loaded configuration. Widgets are always
// created for enabled collectors (they show "No data" gracefully when the
// collector has not yet delivered results). The returned slice preserves a
// sensible display order: sysmetrics, tailscale, k8s, claude, billing.
func buildTUIWidgetsAndCollectors(cfg *config.Config) ([]app.Widget, *collectors.Registry) {
	registry := collectors.NewRegistry()
	var ws []app.Widget

	// --- SysMetrics ---
	if cfg.Collectors.SysMetrics.Enabled {
		interval := cfg.Collectors.SysMetrics.Interval.Duration
		if interval <= 0 {
			interval = 2 * time.Second
		}
		c := sysmetrics.New(sysmetrics.Config{
			FastInterval: interval,
		})
		_ = registry.Register(c)
		ws = append(ws, widgets.NewSysMetricsWidget())
	}

	// --- Tailscale ---
	if cfg.Collectors.Tailscale.Enabled {
		interval := cfg.Collectors.Tailscale.Interval.Duration
		if interval <= 0 {
			interval = 10 * time.Second
		}
		client := tailscale.NewLocalClient("")
		c := tailscale.New(tailscale.Config{
			Interval: interval,
		}, client)
		_ = registry.Register(c)
		ws = append(ws, widgets.NewTailscaleWidget())
	}

	// --- Kubernetes ---
	if cfg.Collectors.Kubernetes.Enabled {
		interval := cfg.Collectors.Kubernetes.Interval.Duration
		if interval <= 0 {
			interval = 15 * time.Second
		}
		c := k8s.New(k8s.Config{
			Interval:   interval,
			Contexts:   cfg.Collectors.Kubernetes.Contexts,
			Namespaces: cfg.Collectors.Kubernetes.Namespaces,
		})
		_ = registry.Register(c)
		ws = append(ws, widgets.NewK8sWidget())
	}

	// --- Claude ---
	if cfg.Collectors.Claude.Enabled {
		interval := cfg.Collectors.Claude.Interval.Duration
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		accounts := buildClaudeAccounts(cfg)
		c := claude.New(claude.Config{
			Interval: interval,
			Accounts: accounts,
		}, nil) // nil client uses default HTTP client
		_ = registry.Register(c)
		ws = append(ws, widgets.NewClaudeWidget())
	}

	// --- Waifu ---
	if cfg.Image.WaifuEnabled {
		waifuCacheDir := cfg.Collectors.Waifu.CacheDir
		if waifuCacheDir == "" {
			waifuCacheDir = filepath.Join(cfg.General.CacheDir, "waifu")
		}

		sessionMgr := waifu.NewSessionManager(waifu.SessionConfig{
			ImageDir: waifuCacheDir,
			CacheDir: cfg.General.CacheDir,
		})

		caps := terminal.DetectCapabilities()
		renderer := image.NewRenderer(*caps, cfg.Image)

		ws = append(ws, widgets.NewWaifuWidget(sessionMgr, renderer, waifuCacheDir))

		// Register the waifu collector if an endpoint is configured.
		if cfg.Collectors.Waifu.Endpoint != "" {
			interval := cfg.Collectors.Waifu.Interval.Duration
			if interval <= 0 {
				interval = 1 * time.Hour
			}
			category := cfg.Collectors.Waifu.Category
			if category == "" {
				category = cfg.Image.WaifuCategory
			}
			wc := waifuCollector.New(waifuCollector.Config{
				Interval:  interval,
				Endpoint:  cfg.Collectors.Waifu.Endpoint,
				Category:  category,
				CacheDir:  waifuCacheDir,
				MaxImages: cfg.Collectors.Waifu.MaxImages,
			}, nil)
			_ = registry.Register(wc)
		}
	}

	// --- Billing ---
	if cfg.Collectors.Billing.Enabled {
		interval := cfg.Collectors.Billing.Interval.Duration
		if interval <= 0 {
			interval = 15 * time.Minute
		}
		bcfg := billing.Config{
			Interval: interval,
		}
		if cfg.Collectors.Billing.Civo.Enabled {
			apiKey := cfg.Collectors.Billing.Civo.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("CIVO_TOKEN")
			}
			if apiKey != "" {
				bcfg.Civo = &billing.CivoConfig{
					APIKey: apiKey,
				}
			}
		}
		if cfg.Collectors.Billing.DigitalOcean.Enabled {
			apiKey := cfg.Collectors.Billing.DigitalOcean.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("DIGITALOCEAN_TOKEN")
			}
			if apiKey != "" {
				bcfg.DigitalOcean = &billing.DOConfig{
					APIToken: apiKey,
				}
			}
		}
		c := billing.New(bcfg)
		_ = registry.Register(c)
		ws = append(ws, widgets.NewBillingWidget())
	}

	// --- Claude Personal ---
	// Always enabled: scans local JSONL files, no API key needed.
	{
		c := claudepersonal.New(claudepersonal.Config{
			StateDir:  cfg.General.CacheDir,
			Interval:  claudepersonal.DefaultScanInterval,
		})
		_ = registry.Register(c)
		ws = append(ws, widgets.NewClaudePersonalWidget())
	}

	return ws, registry
}

// buildClaudeAccounts converts the config's Claude account entries into
// the collector's AccountConfig slice. It resolves admin keys from the
// config, per-account overrides, and the ANTHROPIC_ADMIN_KEY env var.
func buildClaudeAccounts(cfg *config.Config) []claude.AccountConfig {
	// If explicit accounts are configured, use them.
	if len(cfg.Collectors.Claude.Accounts) > 0 {
		accounts := make([]claude.AccountConfig, 0, len(cfg.Collectors.Claude.Accounts))
		for _, a := range cfg.Collectors.Claude.Accounts {
			key := a.AdminKey
			if key == "" {
				key = cfg.Collectors.Claude.AdminKey
			}
			if key == "" {
				key = os.Getenv("ANTHROPIC_ADMIN_KEY")
			}
			accounts = append(accounts, claude.AccountConfig{
				Name:        a.Name,
				AdminAPIKey: key,
			})
		}
		return accounts
	}

	// Fallback: single default account from the global admin key.
	key := cfg.Collectors.Claude.AdminKey
	if key == "" {
		key = os.Getenv("ANTHROPIC_ADMIN_KEY")
	}
	if key == "" {
		// No key available; return empty so the collector shows disconnected.
		return nil
	}

	return []claude.AccountConfig{
		{
			Name:        "default",
			AdminAPIKey: key,
		},
	}
}
