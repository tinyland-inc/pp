// Package manpage generates a roff-formatted man page for prompt-pulse.
//
// The man page is generated at runtime from the actual KeyRegistry and
// compiled-in version information, keeping documentation in sync with
// the code automatically.
//
// Usage:
//
//	prompt-pulse --man | man -l -
//	prompt-pulse --man > ~/.local/share/man/man1/prompt-pulse.1
package manpage

import (
	"fmt"
	"strings"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/display/tui"
)

// Generate produces a complete roff-formatted man(1) page for prompt-pulse.
// The version, commit, and date parameters are passed from the build-time
// linker variables so the man page always reflects the current build.
func Generate(version, commit, date string) string {
	var b strings.Builder

	writeHeader(&b, version)
	writeName(&b)
	writeSynopsis(&b)
	writeDescription(&b)
	writeOptions(&b)
	writeKeybindings(&b)
	writeConfiguration(&b)
	writeShellIntegration(&b)
	writeFiles(&b)
	writeExamples(&b)
	writeEnvironment(&b)
	writeExitStatus(&b)
	writeSeeAlso(&b)
	writeAuthors(&b)
	writeBugs(&b)
	writeFooter(&b, version, commit, date)

	return b.String()
}

// roffEscape escapes special roff characters in a string.
func roffEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `-`, `\-`)
	s = strings.ReplaceAll(s, `.`, `\&.`)
	return s
}

func writeHeader(b *strings.Builder, version string) {
	month := time.Now().Format("January 2006")
	fmt.Fprintf(b, ".TH PROMPT-PULSE 1 \"%s\" \"prompt-pulse %s\" \"User Commands\"\n", month, version)
}

func writeName(b *strings.Builder) {
	b.WriteString(`.SH NAME
prompt\-pulse \- terminal dashboard for cloud infrastructure monitoring
`)
}

func writeSynopsis(b *strings.Builder) {
	b.WriteString(`.SH SYNOPSIS
.B prompt\-pulse
[\fIOPTIONS\fR]
`)
}

func writeDescription(b *strings.Builder) {
	b.WriteString(`.SH DESCRIPTION
.B prompt\-pulse
is a multi-source infrastructure status aggregator. It collects status from
Claude Code sessions, cloud billing APIs, and infrastructure health checks,
then surfaces that information through a terminal banner, Starship prompt
segments, or an interactive TUI dashboard.
.PP
The tool operates in several modes:
.IP \(bu 2
.B Banner mode
(default with \fB\-\-banner\fR): Renders a rich terminal banner with
Claude API usage, billing summaries, infrastructure status, optional
waifu images, and fastfetch system information.
.IP \(bu 2
.B TUI mode
(\fB\-\-tui\fR): Launches an interactive Bubbletea terminal UI with
tabbed navigation, scrolling, and real-time data refresh.
.IP \(bu 2
.B Daemon mode
(\fB\-\-daemon\fR): Runs a background polling loop that periodically
collects data from all configured providers and writes results to a
shared cache.
.IP \(bu 2
.B Starship mode
(\fB\-\-starship\fR \fIMODULE\fR): Outputs a single-line segment suitable
for embedding in a Starship prompt configuration.
.IP \(bu 2
.B One-shot mode
(default, no flags): Runs a single collection pass and exits.
`)
}

func writeOptions(b *strings.Builder) {
	b.WriteString(`.SH OPTIONS
`)

	flags := []struct {
		flag string
		arg  string
		desc string
	}{
		{"banner", "", "Display the system status banner. This is the primary display mode, rendering a multi-column terminal banner with Claude API usage, billing summaries, and infrastructure status."},
		{"tui", "", "Launch the interactive Bubbletea TUI dashboard. Provides tabbed navigation across Claude, billing, infrastructure, and system panels with vim-style keybindings."},
		{"daemon", "", "Run the background polling daemon. Collects data at the configured poll interval and writes to the cache directory. Managed by systemd (Linux) or launchd (macOS)."},
		{"starship", "MODULE", "Output a one-line Starship prompt segment. MODULE must be one of: claude, billing, infra. The output is designed for use in starship.toml custom modules."},
		{"shell", "SHELL", "Output a shell integration script for the specified shell. SHELL must be one of: bash, zsh, fish, nushell. The script provides a Ctrl+P keybinding to launch the TUI and convenience functions."},
		{"health", "", "Check daemon health status. Reads the health file from the cache directory and verifies the daemon ran within the expected poll interval. Exit code 0 means healthy, 1 means stale or missing."},
		{"json", "", "Output health check results as JSON. Must be used with \\fB\\-\\-health\\fR."},
		{"keys", "", "Show all registered keybindings in a formatted table. Can be filtered by mode and formatted as JSON."},
		{"mode", "MODE", "Filter keybindings by mode when used with \\fB\\-\\-keys\\fR. MODE must be one of: tui, shell, starship."},
		{"format", "FORMAT", "Output format for \\fB\\-\\-keys\\fR. FORMAT must be one of: table (default), json."},
		{"theme", "THEME", "Theme preset for display. THEME must be one of: monitoring (default), minimal, full, auto. When set to auto, the config file value is used."},
		{"config", "PATH", "Path to the YAML configuration file. Default: ~/.config/prompt-pulse/config.yaml."},
		{"waifu", "", "Enable waifu image display in banner mode. Requires chafa(1) for terminal image rendering. Overrides the config file setting."},
		{"fastfetch\\-enabled", "", "Enable fastfetch system info in the banner center column. Requires fastfetch(1) to be installed. Overrides the config file setting."},
		{"session\\-id", "ID", "Session ID for waifu image caching. If empty, a new session ID is auto-generated. Useful for maintaining consistent images across banner refreshes."},
		{"term\\-width", "N", "Override terminal width detection. 0 (default) means auto-detect."},
		{"term\\-height", "N", "Override terminal height detection. 0 (default) means auto-detect."},
		{"diagnose", "", "Diagnose Claude credentials and API connectivity. Tests each configured Claude account and reports authentication status."},
		{"billing\\-check", "", "Check billing provider API key configuration. Validates that required environment variables are set for each cloud provider."},
		{"use\\-mocks", "", "Use mock data instead of real API calls. Useful for testing display layouts and development."},
		{"mock\\-accounts", "N", "Number of mock Claude accounts to generate when using \\fB\\-\\-use\\-mocks\\fR. Default: 3."},
		{"mock\\-seed", "N", "Random seed for deterministic mock data generation. 0 (default) means random."},
		{"verbose", "", "Enable verbose (debug-level) logging to stderr and the log file."},
		{"version", "", "Print the version, commit hash, and build date, then exit."},
		{"man", "", "Print this man page to stdout in roff format. Pipe to man(1) for formatted viewing: \\fBprompt\\-pulse \\-\\-man | man \\-l \\-\\fR."},
	}

	for _, f := range flags {
		b.WriteString(".TP\n")
		if f.arg != "" {
			fmt.Fprintf(b, ".BR \\-\\-%s \" \\fI%s\\fR\"\n", f.flag, f.arg)
		} else {
			fmt.Fprintf(b, ".B \\-\\-%s\n", f.flag)
		}
		b.WriteString(f.desc + "\n")
	}
}

func writeKeybindings(b *strings.Builder) {
	b.WriteString(`.SH KEYBINDINGS
The following keybindings are registered in the KeyRegistry and are the
single source of truth for all prompt\-pulse input handling.
`)

	registry := tui.DefaultRegistry()

	modes := []struct {
		mode tui.KeyMode
		name string
		desc string
	}{
		{tui.ModeTUI, "TUI Mode", "Active in the interactive TUI dashboard (\\fB\\-\\-tui\\fR)."},
		{tui.ModeShell, "Shell Mode", "Active in shell integration scripts (\\fB\\-\\-shell\\fR)."},
		{tui.ModeStarship, "Starship Mode", "Active in Starship prompt modules."},
	}

	for _, m := range modes {
		entries := registry.ByMode(m.mode)
		if len(entries) == 0 {
			continue
		}

		fmt.Fprintf(b, ".SS %s\n%s\n", m.name, m.desc)

		// Group by category within each mode.
		categories := []struct {
			cat  tui.KeyCategory
			name string
		}{
			{tui.CategoryNavigation, "Navigation"},
			{tui.CategoryScroll, "Scrolling"},
			{tui.CategoryData, "Data"},
			{tui.CategorySystem, "System"},
		}

		for _, cat := range categories {
			var catEntries []tui.KeyEntry
			for _, e := range entries {
				if e.Category == cat.cat {
					catEntries = append(catEntries, e)
				}
			}
			if len(catEntries) == 0 {
				continue
			}

			fmt.Fprintf(b, ".PP\n\\fI%s:\\fR\n", cat.name)
			for _, e := range catEntries {
				keysStr := strings.Join(e.Binding.Keys(), ", ")
				desc := e.Binding.Help().Desc
				fmt.Fprintf(b, ".TP\n.B %s\n%s (since %s)\n", roffEscape(keysStr), desc, e.Since)
			}
		}
	}
}

func writeConfiguration(b *strings.Builder) {
	b.WriteString(`.SH CONFIGURATION
Configuration is read from a YAML file at
.B ~/.config/prompt\-pulse/config.yaml
by default, or from the path specified with \fB\-\-config\fR.
.PP
The configuration file is organized into the following top-level sections:
.SS daemon
.TP
.B poll_interval
Duration between data collection cycles (e.g., "15m", "1h"). Default: "15m".
.TP
.B cache_dir
Directory for cached API responses. Default: ~/.cache/prompt\-pulse.
.TP
.B log_file
Path for daemon log output. Default: ~/.local/log/prompt\-pulse.log.
.TP
.B account_stagger_delay
Delay between account requests to prevent thundering herd (e.g., "5s"). Default: "5s".
.TP
.B max_parallel_accounts
Maximum concurrent account requests. Default: 3.
.SS accounts
.PP
Supports multiple Claude accounts (max 5), plus cloud billing providers:
Civo, DigitalOcean, AWS, and DreamHost. Each Claude account can be either
"subscription" (using a credentials JSON file) or "api" (using an
environment variable).
.PP
Budget tracking is available per-provider with configurable monthly limits
and alert thresholds.
.SS tailscale
.PP
Tailscale mesh monitoring configuration. Supports API-based monitoring with
CLI fallback, and optional SSH-based node metrics collection.
.SS kubernetes
.PP
Kubernetes cluster contexts to monitor. Each context specifies a kubectl
context name, optional kubeconfig path, namespace, and dashboard URL.
.SS display
.TP
.B theme
Display theme: "minimal", "full", or "monitoring" (default).
.TP
.B enable_hyperlinks
Enable OSC 8 terminal hyperlinks for clickable URLs. Default: true.
.TP
.B waifu.enabled
Enable waifu image display in banner mode.
.TP
.B waifu.category
Image category (e.g., "neko"). Default: "neko".
.TP
.B waifu.cache_ttl
Duration for cached image validity. Default: "24h".
.TP
.B waifu.max_cache_mb
Maximum image cache size in megabytes. Default: 50.
.TP
.B fastfetch.enabled
Enable fastfetch system info in banner center column. Default: false.
.SS starship
.PP
Module toggles for Starship prompt integration:
.TP
.B modules.claude
Enable Claude usage module. Default: true.
.TP
.B modules.billing
Enable billing summary module. Default: true.
.TP
.B modules.infra
Enable infrastructure status module. Default: true.
`)
}

func writeShellIntegration(b *strings.Builder) {
	b.WriteString(`.SH SHELL INTEGRATION
Shell integration scripts provide a keybinding (default Ctrl+P) to launch
the TUI and convenience functions for status checks and daemon management.
.PP
Generate and source the integration script for your shell:
.SS Bash
.nf
eval "$(prompt\-pulse \-\-shell bash)"
.fi
.PP
Or add to ~/.bashrc for persistent integration.
.SS Zsh
.nf
eval "$(prompt\-pulse \-\-shell zsh)"
.fi
.PP
Or add to ~/.zshrc for persistent integration.
.SS Fish
.nf
prompt\-pulse \-\-shell fish | source
.fi
.PP
Or add to ~/.config/fish/config.fish for persistent integration.
.SS Nushell
.nf
prompt\-pulse \-\-shell nushell | save \-f ~/.config/nushell/prompt\-pulse.nu
source ~/.config/nushell/prompt\-pulse.nu
.fi
.PP
Nushell does not support eval, so the script must be saved to a file first.
.SS Nix / Home Manager
.PP
When using the Nix home-manager module, shell integration is configured
automatically. See the
.B tinyland.promptPulse.shellIntegration
options in
.BR prompt\-pulse.nix .
`)
}

func writeFiles(b *strings.Builder) {
	b.WriteString(`.SH FILES
.TP
.I ~/.config/prompt\-pulse/config.yaml
Primary configuration file (YAML).
.TP
.I ~/.cache/prompt\-pulse/
Cache directory for API responses and collector data.
.TP
.I ~/.cache/prompt\-pulse/health.json
Daemon health status file, updated after each collection pass.
.TP
.I ~/.cache/prompt\-pulse/prompt\-pulse.pid
PID file for the background daemon.
.TP
.I ~/.cache/prompt\-pulse/waifu/
Cached waifu images, organized by session.
.TP
.I ~/.local/log/prompt\-pulse.log
Daemon log file.
.TP
.I ~/.claude/.credentials.json
Default Claude subscription credentials file.
`)
}

func writeExamples(b *strings.Builder) {
	b.WriteString(`.SH EXAMPLES
Display the system status banner:
.PP
.nf
prompt\-pulse \-\-banner
.fi
.PP
Display the banner with a waifu image and fastfetch:
.PP
.nf
prompt\-pulse \-\-banner \-\-waifu \-\-fastfetch\-enabled
.fi
.PP
Launch the interactive TUI:
.PP
.nf
prompt\-pulse \-\-tui
.fi
.PP
Start the background daemon:
.PP
.nf
prompt\-pulse \-\-daemon
.fi
.PP
Check daemon health:
.PP
.nf
prompt\-pulse \-\-health
prompt\-pulse \-\-health \-\-json
.fi
.PP
View keybindings:
.PP
.nf
prompt\-pulse \-\-keys
prompt\-pulse \-\-keys \-\-mode tui
prompt\-pulse \-\-keys \-\-format json
.fi
.PP
Starship prompt integration (add to starship.toml):
.PP
.nf
[custom.claude]
command = "prompt\-pulse \-\-starship claude"
when = "test \-f ~/.cache/prompt\-pulse/health.json"
.fi
.PP
View this man page:
.PP
.nf
prompt\-pulse \-\-man | man \-l \-
.fi
.PP
Install the man page permanently:
.PP
.nf
prompt\-pulse \-\-man > ~/.local/share/man/man1/prompt\-pulse.1
.fi
.PP
Test with mock data:
.PP
.nf
prompt\-pulse \-\-banner \-\-use\-mocks \-\-mock\-accounts 5
prompt\-pulse \-\-tui \-\-use\-mocks \-\-mock\-seed 42
.fi
`)
}

func writeEnvironment(b *strings.Builder) {
	b.WriteString(`.SH ENVIRONMENT
.TP
.B PROMPT_PULSE_CONFIG
Override path to the configuration file.
.TP
.B CIVO_API_KEY
Civo cloud API key for billing data.
.TP
.B DIGITALOCEAN_TOKEN
DigitalOcean API token for billing data.
.TP
.B DREAMHOST_API_KEY
DreamHost API key for billing data.
.TP
.B AWS_PROFILE
AWS CLI profile name for billing data.
.TP
.B TAILSCALE_API_KEY
Tailscale API key for mesh monitoring.
.TP
.B ANTHROPIC_API_KEY
Anthropic API key for Claude API accounts.
`)
}

func writeExitStatus(b *strings.Builder) {
	b.WriteString(".SH EXIT STATUS\n")
	b.WriteString(".TP\n.B 0\n")
	b.WriteString("Success. For \\fB\\-\\-health\\fR, indicates the daemon is healthy.\n")
	b.WriteString(".TP\n.B 1\n")
	b.WriteString("Failure. For \\fB\\-\\-health\\fR, indicates the daemon health is stale or missing.\n")
}

func writeSeeAlso(b *strings.Builder) {
	b.WriteString(`.SH SEE ALSO
.BR starship (1),
.BR chafa (1),
.BR fastfetch (1),
.BR systemctl (1),
.BR launchctl (1),
.BR home\-manager (1)
`)
}

func writeAuthors(b *strings.Builder) {
	b.WriteString(`.SH AUTHORS
Tinyland Lab <https://gitlab.com/tinyland/lab>
`)
}

func writeBugs(b *strings.Builder) {
	b.WriteString(`.SH BUGS
Report bugs at <https://gitlab.com/tinyland/lab/prompt\-pulse/\-/issues>.
`)
}

func writeFooter(b *strings.Builder, version, commit, date string) {
	fmt.Fprintf(b, ".SH VERSION\n%s (%s) built %s\n", version, commit, date)
}
