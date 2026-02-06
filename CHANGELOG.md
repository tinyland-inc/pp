# Changelog

All notable changes to prompt-pulse are documented in this file.

## [0.2.0] - Phase 2: High-Density Dynamic TUI Banner

**Branch**: `dev/banner-upgrades`
**Date**: 2026-02-06

### New Features

#### TUI Dashboard (Pre-Phase 2, committed on this branch)
- Interactive Bubbletea TUI with 4 tabbed panels: Claude, Billing, Infra, System
- `tea.Tick` 30-second cache polling with `dataRefreshMsg` pattern for live data refresh
- `bubbles/viewport` scrollable content within each tab with j/k, pgup/pgdn, g/G navigation
- `bubbles/help` integration with `?` toggle showing ShortHelp (compact) and FullHelp (grouped)
- `bubbles/spinner` for initial data load and manual refresh indication
- `bubblezone` clickable tab headers with mouse support (`tea.WithMouseCellMotion()`)
- KeyRegistry single source of truth for all keybindings across TUI, shell, and starship modes
- Health check system: daemon writes `health.json`, `--health` flag reads it with configurable staleness threshold

#### Sprint 0: MCP Pipeline Fix
- Fixed Claude Code MCP pipeline: corrected target path from `~/.config/claude-code/settings.json` to `~/.claude.json`
- Enabled API-key MCP servers (perplexity-pro, perplexity-reasoning, perplexity-files, shodan) with `${VAR}` environment variable expansion
- Fixed `.cache/registries.json` gitignore exclusion to restore Nix flake visibility

#### Sprint 1: Starship Dedup and System Metrics
- **S1-A**: Deduplicated `starship.nix` into single canonical module at `nix/modules/tools/starship.nix`, eliminating duplicate configuration paths
- **S1-B**: New `sysmetrics` collector package (`collectors/sysmetrics/`) reading CPU from `/proc/stat` (delta-based), RAM from `/proc/meminfo` (MemTotal - MemAvailable), Disk from `syscall.Statfs`, and Load Average from `/proc/loadavg`. Maintains 60-sample ring buffers with cache persistence across daemon restarts
- **S1-C**: System tab sparklines in TUI with Unicode block characters (`RenderSparkline`), threshold-colored gauges (green < 70%, yellow 70-90%, red >= 90%), and 1/5/15 minute load average display

#### Sprint 2: Progressive Density and Hidden Data
- **S2-A**: Banner progressive density via 5 new `LayoutFeatures` flags: `ShowGauges`, `ShowSysMetrics`, `ShowSysMetricsSparklines`, `ShowExtraUsage`, `ShowBillingDelta`. Each wider LayoutMode enables strictly more features: Compact (none) < Standard (sparklines, full metrics) < Wide (+ gauges, sysmetrics, billing delta) < UltraWide (+ sysmetrics sparklines, extra usage)
- **S2-B**: ExtraUsage rendering in Claude tab, banner, and compact modes. Shows overuse credit utilization with gauge bar and dollar amounts. Displayed when `features.ShowExtraUsage` is true (UltraWide) or always in TUI Claude tab
- **S2-C**: PreviousMonth billing delta with `FormatMonthOverMonth()` in `display/widgets/billing_panel.go`. Uses Unicode arrows (up/down/right), percentage deltas, and red/green/gray coloring. Integrated into both TUI billing tab and banner billing section

#### Sprint 3: NO_COLOR and Man Page
- **S3-A**: NO_COLOR spec compliance (https://no-color.org/) via `display/color/` package. `ShouldDisableColor()` checks `NO_COLOR` env var existence and pipe/redirect detection via `go-isatty`. `Apply()` sets lipgloss to `termenv.Ascii` profile. `StripANSI()` safety net for residual escape sequences. 3 theme presets (monitoring/minimal/full) with `--theme` flag and `ApplyTheme()` runtime switching
- **S3-B**: Man page generation from KeyRegistry via `docs/manpage/` package. `--man` flag outputs complete roff-formatted man(1) page with all CLI flags, keybindings grouped by mode and category, configuration reference, shell integration instructions, and build-time version info. Usage: `prompt-pulse --man | man -l -`

#### Sprint 4: Integration Tests and Nix Config
- **S4-A**: 43 integration tests added: 27 TUI lifecycle tests (tab switching, viewport scrolling, data refresh, spinner, help toggle, mouse clicks, window resize), 5 daemon tests (collection, health file, stagger delay, context cancellation, error handling), 11 KeyRegistry tests (duplicate detection, mode filtering, category filtering, format table, format JSON, completeness)
- **S4-B**: 7 missing Nix config fields added to `nix/home-manager/prompt-pulse.nix` and `nix/hosts/base.nix`: account `priority`, `pollInterval`, Kubernetes `platform`, `clusterType`, `timeout`, `priority`, and waifu `maxSessions`

### Enhancements

- Responsive layout engine (`display/layout/responsive.go`) supports 4 layout modes: Compact (80x24), Standard (120x40), Wide (160x60), UltraWide (200x80) with progressive column allocation
- Column composition methods: `composeSideBySide`, `composeTwoColumns`, `composeThreeColumns`, `composeFourColumns` with ANSI-aware padding via `visibleLen()`/`padToWidth()`
- Banner `buildSections()` now accepts fastfetch, sysmetrics data and LayoutFeatures for conditional rendering
- Billing section in banner mode respects `ShowFullMetrics` for per-provider detail and `ShowBillingDelta` for month-over-month comparison
- InfraPanel widget (`display/widgets/infra_panel.go`) with tree-structured display, mini gauges, and configurable max node count
- System tab renders both fastfetch static info (OS, Host, Kernel, CPU, GPU, Memory, Disk, Packages, Shell, Terminal, LocalIP) and live sysmetrics with sparklines + gauges
- Mock data system (`tests/mocks/`) with `MockClaudeUsage()`, `MockBillingData()`, `MockInfraStatus()`, `MockFastfetchData()`, `SeedRandom()` for deterministic testing
- `--use-mocks`, `--mock-accounts`, `--mock-seed` CLI flags for display layout testing without live API calls

### Bug Fixes

- Fixed waifu image sizing to match responsive layout column allocations (Standard: 28x14, Wide: 36x18, UltraWide: 48x24)
- Corrected column separator spacing in 3-column and 4-column layouts
- Fixed billing section showing misleading `$0` when all providers errored (now shows `N/A (N providers unreachable)`)

### Testing

- 43 new integration tests across TUI lifecycle, daemon, and KeyRegistry
- Visual regression golden files updated for all 4 layout modes (standard, wide, ultrawide) x 4 scenarios (full-data, no-data, error-state, with-waifu)
- SysMetrics collector tests with mock `/proc/stat`, `/proc/meminfo`, `/proc/loadavg` file openers and mock `statfsFunc`
- Theme preset tests verifying MonitoringTheme, MinimalTheme, FullTheme apply correctly
- Man page generation test verifying roff output structure

### Dependencies

- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - TUI components (viewport, help, spinner)
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/charmbracelet/x/term` - Terminal size detection
- `github.com/lrstanley/bubblezone` - Clickable zone management
- `github.com/mattn/go-isatty` - TTY detection for NO_COLOR
- `github.com/muesli/termenv` - Color profile management

### Configuration

New config fields in `config.yaml`:
```yaml
accounts:
  claude:
    - priority: 1              # Account display priority
      poll_interval: "15m"     # Per-account poll interval

kubernetes:
  - platform: "k3s"           # Platform type (k3s, kind, minikube, eks, gke)
    cluster_type: "dev"        # Cluster classification
    timeout: "30s"             # kubectl timeout
    priority: 1                # Display priority

display:
  waifu:
    max_sessions: 10           # Max session images before LRU eviction
```
