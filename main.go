// prompt-pulse is a multi-source infrastructure status aggregator.
//
// It collects status from Claude Code sessions, cloud billing APIs, and
// infrastructure health checks, then surfaces that information through
// Starship prompt segments, an inline banner, or an interactive TUI.
//
// Usage:
//
//	prompt-pulse [flags]
//
// Flags:
//
//	-banner           Display system status banner
//	-daemon           Run background daemon
//	-tui              Launch interactive Bubbletea TUI
//	-starship string  Output one-line Starship segment (claude|billing|infra|all)
//	-shell string     Output shell integration script (bash|zsh|fish|ksh)
//	-config string    Path to configuration file (default: ~/.config/prompt-pulse/config.toml)
//	-theme string     Theme override (default|gruvbox|nord|catppuccin|dracula|tokyo-night)
//	-health           Check daemon health status
//	-diagnose         Claude diagnostics
//	-migrate          Run v1-to-v2 config migration
//	-man              Print man page to stdout in roff format
//	-verbose          Enable verbose logging
//	-version          Print version and exit
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/app"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/banner"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/config"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/daemon"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/docs"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/migrate"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/shell"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/starship"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/theme"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/tui"
)

func main() {
	var (
		configPath     = flag.String("config", "", "Path to configuration file (default: ~/.config/prompt-pulse/config.toml)")
		runDaemon      = flag.Bool("daemon", false, "Run background daemon")
		runTUI         = flag.Bool("tui", false, "Launch interactive Bubbletea TUI")
		runBanner      = flag.Bool("banner", false, "Display system status banner")
		starshipMod    = flag.String("starship", "", "Output one-line Starship segment (claude|billing|infra|all)")
		shellType      = flag.String("shell", "", "Output shell integration script (bash|zsh|fish|ksh)")
		themeFlag      = flag.String("theme", "", "Theme override")
		runHealth      = flag.Bool("health", false, "Check daemon health status")
		healthJSON     = flag.Bool("json", false, "Output health check as JSON (with -health)")
		runDiagnose    = flag.Bool("diagnose", false, "Claude diagnostics")
		runMigrate     = flag.Bool("migrate", false, "Run v1-to-v2 config migration")
		showMan        = flag.Bool("man", false, "Print man page to stdout in roff format")
		manDir         = flag.String("man-dir", "", "Write all man pages to directory (e.g., /usr/share/man)")
		verbose        = flag.Bool("verbose", false, "Enable verbose logging")
		showVersion    = flag.Bool("version", false, "Print version and exit")
		termWidth      = flag.Int("term-width", 0, "Terminal width override (0 = auto-detect)")
		termHeight     = flag.Int("term-height", 0, "Terminal height override (0 = auto-detect)")
		waifuMode      = flag.Bool("waifu", false, "Enable waifu image in banner")
		sessionID      = flag.String("session-id", "", "Session ID for per-session waifu caching")
		showBanner     = flag.Bool("show-banner", false, "Show banner in shell integration")
		daemonAutoStart = flag.Bool("daemon-autostart", false, "Auto-start daemon in shell integration")
	)
	flag.Parse()

	// ---------------------------------------------------------------
	// Commands that don't require config
	// ---------------------------------------------------------------

	if *showVersion {
		fmt.Printf("prompt-pulse %s (%s) built %s\n", version, commit, date)
		os.Exit(0)
	}

	if *showMan {
		mp := docs.New(os.TempDir())
		// Generate the main prompt-pulse man page in roff format.
		mp.Format = "roff"
		mp.Add("prompt-pulse", "prompt-pulse",
			"Terminal dashboard with live data, waifu rendering, and TUI mode.",
			1,
		)
		output, err := mp.GenerateSingle()
		if err != nil {
			fmt.Fprintf(os.Stderr, "man page generation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(output)
		os.Exit(0)
	}

	if *manDir != "" {
		n, err := docs.WriteManPages(*manDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "man page generation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote %d man pages to %s\n", n, *manDir)
		os.Exit(0)
	}

	if *runDiagnose {
		fmt.Println("prompt-pulse v2 diagnostics")
		fmt.Println("===========================")
		fmt.Println()
		fmt.Println("Theme registry:")
		for _, name := range theme.Names() {
			marker := "  "
			if name == theme.Current.Name {
				marker = "* "
			}
			fmt.Printf("  %s%s\n", marker, name)
		}
		fmt.Println()
		fmt.Println("Config search paths:")
		home, _ := os.UserHomeDir()
		fmt.Printf("  %s\n", filepath.Join(home, ".config", "prompt-pulse", "config.toml"))
		fmt.Println()
		fmt.Println("Daemon status:")
		dcfg := daemon.DefaultConfig()
		d, err := daemon.New(dcfg)
		if err != nil {
			fmt.Printf("  daemon init error: %v\n", err)
		} else if d.IsRunning() {
			fmt.Println("  running")
			if health, err := d.Health(); err == nil {
				data, _ := json.MarshalIndent(health, "  ", "  ")
				fmt.Println("  " + string(data))
			}
		} else {
			fmt.Println("  not running")
		}
		os.Exit(0)
	}

	if *shellType != "" {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "prompt-pulse: shell integration panic: %v\n", r)
				os.Exit(1)
			}
		}()
		var st shell.ShellType
		switch *shellType {
		case "bash":
			st = shell.Bash
		case "zsh":
			st = shell.Zsh
		case "fish":
			st = shell.Fish
		case "ksh":
			st = shell.Ksh
		default:
			fmt.Fprintf(os.Stderr, "unknown shell: %s (supported: bash, zsh, fish, ksh)\n", *shellType)
			os.Exit(1)
		}
		opts := shell.Options{
			ShowBanner:      *showBanner,
			DaemonAutoStart: *daemonAutoStart,
		}
		fmt.Print(shell.Generate(st, opts))
		os.Exit(0)
	}

	if *runMigrate {
		home, _ := os.UserHomeDir()
		v1Path := filepath.Join(home, ".config", "prompt-pulse", "config.yaml")
		v2Path := filepath.Join(home, ".config", "prompt-pulse", "config.toml")

		// Allow overriding the source config via -config flag.
		if *configPath != "" {
			v1Path = *configPath
		}

		needs, err := migrate.NeedsMigration(v1Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migration check failed: %v\n", err)
			os.Exit(1)
		}
		if !needs {
			fmt.Println("No migration needed (config is already v2 format or does not exist).")
			os.Exit(0)
		}

		result, err := migrate.Migrate(v1Path, v2Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Migration complete.")
		if result.BackupPath != "" {
			fmt.Printf("  Backup: %s\n", result.BackupPath)
		}
		fmt.Printf("  Changes: %d\n", len(result.Changes))
		for _, c := range result.Changes {
			fmt.Printf("    [%s] %s: %s -> %s\n", c.Action, c.Field, c.OldValue, c.NewValue)
		}
		for _, w := range result.Warnings {
			fmt.Printf("  Warning: %s\n", w)
		}
		os.Exit(0)
	}

	// ---------------------------------------------------------------
	// Load configuration (required for remaining modes)
	// ---------------------------------------------------------------

	var cfg *config.Config
	var cfgErr error

	if *configPath != "" {
		cfg, cfgErr = config.LoadFromFile(*configPath)
	} else {
		cfg, cfgErr = config.Load()
	}
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", cfgErr)
		os.Exit(1)
	}

	// Apply theme override from CLI flag.
	if *themeFlag != "" {
		theme.SetCurrent(*themeFlag)
	} else if cfg.Theme.Name != "" {
		theme.SetCurrent(cfg.Theme.Name)
	}

	_ = *verbose   // reserved for future structured logging
	_ = *sessionID // reserved for per-session waifu caching

	// Apply CLI waifu override to config.
	if *waifuMode {
		cfg.Image.WaifuEnabled = true
	}

	// ---------------------------------------------------------------
	// Health check
	// ---------------------------------------------------------------

	if *runHealth {
		dcfg := daemon.DefaultConfig()
		d, err := daemon.New(dcfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "daemon init error: %v\n", err)
			os.Exit(1)
		}

		if !d.IsRunning() {
			if *healthJSON {
				fmt.Println(`{"status":"not_running"}`)
			} else {
				fmt.Fprintln(os.Stderr, "daemon not running")
			}
			os.Exit(1)
		}

		health, err := d.Health()
		if err != nil {
			if *healthJSON {
				fmt.Printf(`{"status":"error","error":"%s"}`, err.Error())
				fmt.Println()
			} else {
				fmt.Fprintf(os.Stderr, "health check failed: %v\n", err)
			}
			os.Exit(1)
		}

		if *healthJSON {
			data, _ := json.MarshalIndent(health, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("daemon healthy (PID %d, uptime %s)\n", health.PID, health.Uptime)
			for name, c := range health.Collectors {
				status := "ok"
				if !c.Healthy {
					status = "unhealthy"
				}
				fmt.Printf("  %s: %s (errors: %d)\n", name, status, c.ErrorCount)
			}
		}
		os.Exit(0)
	}

	// ---------------------------------------------------------------
	// Context with signal handling
	// ---------------------------------------------------------------

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// ---------------------------------------------------------------
	// Starship mode
	// ---------------------------------------------------------------

	if *starshipMod != "" {
		scfg := starship.Config{
			CacheDir: cfg.General.CacheDir,
		}
		switch *starshipMod {
		case "claude":
			scfg.ShowClaude = true
		case "billing":
			scfg.ShowBilling = true
		case "infra", "tailscale":
			scfg.ShowTailscale = true
		case "k8s", "kubernetes":
			scfg.ShowK8s = true
		case "system", "sys":
			scfg.ShowSystem = true
		case "all":
			scfg.ShowClaude = true
			scfg.ShowBilling = true
			scfg.ShowTailscale = true
			scfg.ShowK8s = true
			scfg.ShowSystem = true
		default:
			fmt.Fprintf(os.Stderr, "unknown starship segment: %s (supported: claude, billing, infra, k8s, system, all)\n", *starshipMod)
			os.Exit(1)
		}

		result := starship.Render(scfg)
		if result != "" {
			fmt.Print(result)
		}
		os.Exit(0)
	}

	// ---------------------------------------------------------------
	// Banner mode
	// ---------------------------------------------------------------

	if *runBanner {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "prompt-pulse: banner panic: %v\n", r)
				os.Exit(1)
			}
		}()

		// Determine terminal dimensions.
		width := *termWidth
		height := *termHeight
		if width <= 0 {
			width = 120 // sensible default
		}
		if height <= 0 {
			height = 35
		}

		preset := banner.SelectPreset(width, height)

		// Build widget data from cached collector data.
		data := buildBannerFromCache(cfg.General.CacheDir, version, commit)

		result, err := banner.RenderCached(cfg.General.CacheDir, data, preset)
		if err != nil {
			fmt.Fprintf(os.Stderr, "banner render failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(result)
		os.Exit(0)
	}

	// ---------------------------------------------------------------
	// TUI mode
	// ---------------------------------------------------------------

	if *runTUI {
		defer func() {
			if r := recover(); r != nil {
				// Attempt to restore terminal from alt-screen before printing error.
				fmt.Print("\x1b[?1049l\x1b[?25h")
				fmt.Fprintf(os.Stderr, "prompt-pulse: TUI panic: %v\n", r)
				os.Exit(1)
			}
		}()

		// Build widgets and collectors from config.
		tuiWidgets, registry := buildTUIWidgetsAndCollectors(cfg)

		model := tui.New(tuiWidgets)

		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

		// Start collector runner that sends data updates to the TUI.
		updatesCh := make(chan collectors.Update, collectors.DefaultUpdateBufferSize)
		runner := collectors.NewRunner(registry, updatesCh)

		if err := runner.Start(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "collector runner error: %v\n", err)
			os.Exit(1)
		}

		// Bridge goroutine: convert collector updates into Bubbletea messages.
		go func() {
			for update := range updatesCh {
				p.Send(app.DataUpdateEvent{
					Source:    update.Source,
					Data:     update.Data,
					Err:      update.Error,
					Timestamp: update.Timestamp,
				})
			}
		}()

		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			runner.Stop()
			os.Exit(1)
		}

		runner.Stop()
		os.Exit(0)
	}

	// ---------------------------------------------------------------
	// Daemon mode
	// ---------------------------------------------------------------

	if *runDaemon {
		dcfg := daemon.DefaultConfig()
		if cfg.General.CacheDir != "" {
			dcfg.DataDir = cfg.General.CacheDir
		}

		d, err := daemon.New(dcfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "daemon init failed: %v\n", err)
			os.Exit(1)
		}
		d.SetAppConfig(cfg)

		fmt.Fprintf(os.Stderr, "starting prompt-pulse daemon v%s\n", version)
		if err := d.Start(ctx); err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// ---------------------------------------------------------------
	// Default: print usage
	// ---------------------------------------------------------------

	fmt.Printf("prompt-pulse v%s (%s) built %s\n", version, commit, date)
	fmt.Println()
	fmt.Println("Usage: prompt-pulse [flags]")
	fmt.Println()
	flag.PrintDefaults()
}
