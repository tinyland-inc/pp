// prompt-pulse is a multi-source infrastructure status aggregator.
//
// It collects status from Claude Code sessions, cloud billing APIs, and
// infrastructure health checks, then surfaces that information through
// Starship prompt segments or an interactive TUI.
//
// Usage:
//
//	prompt-pulse [flags]
//
// Flags:
//
//	-banner           Display system status banner
//	-daemon           Run background polling daemon
//	-tui              Launch interactive Bubbletea TUI
//	-starship string  Output one-line Starship format (claude|billing|infra)
//	-config string    Path to configuration file (default: ~/.config/prompt-pulse/config.yaml)
//	-verbose          Enable verbose logging
//	-version          Print version and exit
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/tinyland/lab/prompt-pulse/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/config"
	"gitlab.com/tinyland/lab/prompt-pulse/display/banner"
	"gitlab.com/tinyland/lab/prompt-pulse/display/starship"
	"gitlab.com/tinyland/lab/prompt-pulse/display/tui"
)

var (
	version = "0.1.0"
	commit  = "dev"
	date    = "unknown"
)

// defaultPollInterval is the fallback duration when the configured poll
// interval cannot be parsed.
const defaultPollInterval = 15 * time.Minute

func main() {
	// Parse command line flags
	var (
		configPath  = flag.String("config", "", "Path to configuration file")
		runDaemon   = flag.Bool("daemon", false, "Run background polling daemon")
		runTUI      = flag.Bool("tui", false, "Launch interactive Bubbletea TUI")
		runBanner   = flag.Bool("banner", false, "Display system status banner")
		showWaifu   = flag.Bool("waifu", false, "Show waifu image in banner (requires -banner)")
		starshipMod = flag.String("starship", "", "Output one-line Starship format (claude|billing|infra)")
		verbose     = flag.Bool("verbose", false, "Enable verbose logging")
		showVersion = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("prompt-pulse %s (%s) built %s\n", version, commit, date)
		os.Exit(0)
	}

	// Resolve configuration path
	if *configPath == "" {
		home, _ := os.UserHomeDir()
		*configPath = filepath.Join(home, ".config", "prompt-pulse", "config.yaml")
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}

	// Setup log file directory
	if err := ensureLogDir(cfg.Daemon.LogFile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	// Setup logging - write to both stderr and log file
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}

	// Open log file for writing
	logFile, err := os.OpenFile(cfg.Daemon.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Create multi-writer for both stderr and log file
	multiWriter := io.MultiWriter(os.Stderr, logFile)
	logger := slog.New(slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("received shutdown signal")
		cancel()
	}()

	// Determine operation mode
	switch {
	case *runTUI:
		model := tui.NewModel()

		// Load cached data to populate the model before launch.
		store, storeErr := cache.NewStore(cfg.Daemon.CacheDir, logger)
		if storeErr == nil {
			ttl := parseDuration(cfg.Daemon.PollInterval)
			if claude, _, _ := cache.GetTyped[collectors.ClaudeUsage](store, "claude", ttl); claude != nil {
				model.SetClaudeData(claude)
			}
			if billing, _, _ := cache.GetTyped[collectors.BillingData](store, "billing", ttl); billing != nil {
				model.SetBillingData(billing)
			}
			if infra, _, _ := cache.GetTyped[collectors.InfraStatus](store, "infra", ttl); infra != nil {
				model.SetInfraData(infra)
			}
		} else {
			logger.Warn("failed to open cache for TUI", "error", storeErr)
		}

		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			logger.Error("TUI error", "error", err)
			os.Exit(1)
		}

	case *starshipMod != "":
		out, err := starship.NewOutput(starship.OutputConfig{
			CacheDir: cfg.Daemon.CacheDir,
			CacheTTL: parseDuration(cfg.Daemon.PollInterval),
			Logger:   logger,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "starship init failed: %v\n", err)
			os.Exit(1)
		}
		result, err := out.Module(*starshipMod)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		if result != "" {
			fmt.Print(result)
		}

	case *runBanner:
		// --waifu flag overrides config file setting
		waifuEnabled := cfg.Display.Waifu.Enabled || *showWaifu
		termWidth, termHeight := banner.DetectTerminalSize()
		bannerCfg := banner.BannerConfig{
			CacheDir:        cfg.Daemon.CacheDir,
			CacheTTL:        parseDuration(cfg.Daemon.PollInterval),
			WaifuEnabled:    waifuEnabled,
			WaifuCategory:   cfg.Display.Waifu.Category,
			WaifuCacheDir:   filepath.Join(cfg.Daemon.CacheDir, "waifu"),
			WaifuCacheTTL:   parseDuration(cfg.Display.Waifu.CacheTTL),
			WaifuMaxCacheMB: cfg.Display.Waifu.MaxCacheMB,
			TermWidth:       termWidth,
			TermHeight:      termHeight,
			Logger:          logger,
		}
		b := banner.NewBanner(bannerCfg)
		output, err := b.Generate(ctx)
		if err != nil {
			logger.Error("banner generation failed", "error", err)
			os.Exit(1)
		}
		fmt.Print(output)

	case *runDaemon:
		d, err := newDaemon(cfg, logger)
		if err != nil {
			logger.Error("daemon init failed", "error", err)
			os.Exit(1)
		}
		logger.Info("starting prompt-pulse daemon",
			"poll_interval", cfg.Daemon.PollInterval,
			"config", *configPath,
		)
		if err := d.run(ctx); err != nil && err != context.Canceled {
			logger.Error("daemon error", "error", err)
			os.Exit(1)
		}

	default:
		// Default: run a single collection pass
		d, err := newDaemon(cfg, logger)
		if err != nil {
			logger.Error("daemon init failed", "error", err)
			os.Exit(1)
		}
		if err := d.runOnce(ctx); err != nil {
			logger.Error("collection failed", "error", err)
			os.Exit(1)
		}
	}
}

// parseDuration parses a Go duration string (e.g. "15m", "1h", "30s").
// Returns defaultPollInterval if the string is empty or unparseable.
func parseDuration(s string) time.Duration {
	if s == "" {
		return defaultPollInterval
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultPollInterval
	}
	return d
}

func ensureLogDir(logFile string) error {
	dir := filepath.Dir(logFile)
	return os.MkdirAll(dir, 0755)
}
