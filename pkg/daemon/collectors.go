package daemon

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/billing"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/claude"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/k8s"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/sysmetrics"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/tailscale"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors/waifu"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/config"
)

// BuildRegistry creates a collector registry from the application config,
// registering only the collectors that are enabled.
func BuildRegistry(cfg *config.Config) *collectors.Registry {
	reg := collectors.NewRegistry()

	if cfg.Collectors.SysMetrics.Enabled {
		c := sysmetrics.New(sysmetrics.Config{
			FastInterval: cfg.Collectors.SysMetrics.Interval.Duration,
			SlowInterval: 60 * time.Second,
		})
		if err := reg.Register(c); err != nil {
			log.Printf("daemon: register sysmetrics: %v", err)
		}
	}

	if cfg.Collectors.Tailscale.Enabled {
		c := tailscale.New(
			tailscale.Config{Interval: cfg.Collectors.Tailscale.Interval.Duration},
			tailscale.NewLocalClient(""),
		)
		if err := reg.Register(c); err != nil {
			log.Printf("daemon: register tailscale: %v", err)
		}
	}

	if cfg.Collectors.Kubernetes.Enabled {
		c := k8s.New(k8s.Config{
			Interval:   cfg.Collectors.Kubernetes.Interval.Duration,
			Contexts:   cfg.Collectors.Kubernetes.Contexts,
			Namespaces: cfg.Collectors.Kubernetes.Namespaces,
		})
		if err := reg.Register(c); err != nil {
			log.Printf("daemon: register k8s: %v", err)
		}
	}

	if cfg.Collectors.Claude.Enabled {
		var accounts []claude.AccountConfig
		if cfg.Collectors.Claude.AdminKey != "" {
			accounts = append(accounts, claude.AccountConfig{
				Name:        "default",
				AdminAPIKey: cfg.Collectors.Claude.AdminKey,
			})
		}
		for _, a := range cfg.Collectors.Claude.Accounts {
			accounts = append(accounts, claude.AccountConfig{
				Name:           a.Name,
				AdminAPIKey:    a.AdminKey,
				OrganizationID: a.OrganizationID,
			})
		}
		c := claude.New(
			claude.Config{
				Interval: cfg.Collectors.Claude.Interval.Duration,
				Accounts: accounts,
			},
			nil, // use default HTTP client
		)
		if err := reg.Register(c); err != nil {
			log.Printf("daemon: register claude: %v", err)
		}
	}

	if cfg.Collectors.Waifu.Enabled {
		wcfg := waifu.Config{
			Interval:  cfg.Collectors.Waifu.Interval.Duration,
			Endpoint:  cfg.Collectors.Waifu.Endpoint,
			Category:  cfg.Collectors.Waifu.Category,
			MaxImages: cfg.Collectors.Waifu.MaxImages,
		}
		if wcfg.CacheDir = cfg.Collectors.Waifu.CacheDir; wcfg.CacheDir == "" {
			wcfg.CacheDir = filepath.Join(cfg.General.CacheDir, "waifu")
		}
		c := waifu.New(wcfg, nil)
		if err := reg.Register(c); err != nil {
			log.Printf("daemon: register waifu: %v", err)
		}
	}

	if cfg.Collectors.Billing.Enabled {
		bcfg := billing.Config{
			Interval: cfg.Collectors.Billing.Interval.Duration,
		}
		if cfg.Collectors.Billing.Civo.APIKey != "" {
			bcfg.Civo = &billing.CivoConfig{
				APIKey: cfg.Collectors.Billing.Civo.APIKey,
				Region: cfg.Collectors.Billing.Civo.Region,
			}
		}
		if cfg.Collectors.Billing.DigitalOcean.APIKey != "" {
			bcfg.DigitalOcean = &billing.DOConfig{
				APIToken: cfg.Collectors.Billing.DigitalOcean.APIKey,
			}
		}
		c := billing.New(bcfg)
		if err := reg.Register(c); err != nil {
			log.Printf("daemon: register billing: %v", err)
		}
	}

	return reg
}

// ConsumeUpdates reads from the updates channel and writes each collector's
// data to a JSON cache file. It blocks until the context is cancelled.
func ConsumeUpdates(ctx context.Context, updates <-chan collectors.Update, cacheDir string, d *Daemon) {
	for {
		select {
		case <-ctx.Done():
			return
		case u := <-updates:
			if u.Error != nil {
				continue
			}
			// Write data to cache file via atomic rename.
			data, err := json.Marshal(u.Data)
			if err != nil {
				log.Printf("daemon: marshal %s data: %v", u.Source, err)
				continue
			}

			dest := filepath.Join(cacheDir, u.Source+".json")
			tmp := dest + ".tmp"
			if err := os.WriteFile(tmp, data, 0o644); err != nil {
				log.Printf("daemon: write %s cache: %v", u.Source, err)
				continue
			}
			if err := os.Rename(tmp, dest); err != nil {
				log.Printf("daemon: rename %s cache: %v", u.Source, err)
				continue
			}

			// Update daemon health from collector status.
			d.UpdateCollector(u.Source, true, 0)
		}
	}
}
