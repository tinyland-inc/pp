# Prompt-Pulse TUI Architecture

This document describes the architecture of the prompt-pulse terminal dashboard,
covering the data pipeline from collectors through caching to rendering.

## Overview

Prompt-pulse operates in several modes, all sharing the same data pipeline:

```
Collectors --> Cache Store --> Banner / TUI / Starship
(daemon)      (JSON files)    (display layer)
```

The **daemon** runs in the background, polling collectors at configured intervals
and writing results to disk-based JSON cache files. The **display layer** reads
these cache files and renders the data in the requested format.

## Package Layout

```
cmd/prompt-pulse/
  main.go                       # CLI entry point, flag parsing, mode dispatch
  daemon.go                     # Background polling loop
  integration_test.go           # End-to-end integration tests

  cache/
    store.go                    # Cache store: read/write JSON files with TTL

  collectors/
    collector.go                # Collector interface, Registry
    models.go                   # Shared data types (ClaudeUsage, BillingData, etc.)
    osc8.go                     # OSC 8 hyperlink formatting
    claude/                     # Claude API usage collector
    billing/                    # Cloud billing collector (Civo, DO, AWS, DreamHost)
    infra/                      # Infrastructure collector (Tailscale, K8s)
    fastfetch/                  # Fastfetch system info collector
    sysmetrics/                 # Local CPU/RAM/Disk/Load collector
    retry/                      # Retry logic for API calls

  config/
    config.go                   # YAML configuration parsing and validation

  display/
    banner/                     # Banner-mode rendering pipeline
      banner.go                 # Banner orchestrator (Generate, buildSections)
      layout.go                 # Banner-specific layout helpers (LayoutMode, WaifuSize)
      terminal.go               # Terminal detection wrapper
      box.go                    # Box drawing utilities
    layout/
      responsive.go             # Responsive layout engine (columnsForMode, featuresForMode)
    color/
      color.go                  # NO_COLOR, pipe detection, StripANSI
    render/
      protocol.go               # Image protocol detection (Kitty, iTerm2, Sixel, Chafa)
      chafa.go                  # Chafa subprocess rendering
      fallback.go               # Unicode half-block fallback renderer
      iterm2.go                 # iTerm2 inline image protocol
    starship/
      config.go                 # Starship module configuration
      output.go                 # Starship one-line output
    tui/
      app.go                    # Bubbletea Model (Init/Update/View)
      keys.go                   # keyMap for bubbletea key.Matches
      keyregistry.go            # KeyRegistry SSOT for all keybindings
      theme.go                  # Color palette and lipgloss styles
      theme_presets.go          # Monitoring/Minimal/Full theme presets
      fetch.go                  # fetchDataCmd for async cache reads
      claude_tab.go             # renderClaudeContent
      billing_tab.go            # renderBillingContent
      infra_tab.go              # renderInfraContent
      system_tab.go             # renderSystemContent
    widgets/
      gauge.go                  # Gauge bar renderer
      sparkline.go              # Unicode block sparkline renderer
      table.go                  # Table widget
      status.go                 # Status indicator (healthy/warning/critical)
      billing_panel.go          # BillingPanel widget, FormatMonthOverMonth
      claude_panel.go           # ClaudePanel widget
      infra_panel.go            # InfraPanel widget with tree display

  docs/
    manpage/
      manpage.go                # Man page generation from KeyRegistry

  shell/
    common.go                   # Shell integration script generation

  status/
    evaluator.go                # System status evaluation (healthy/warning/critical)
    selector.go                 # Waifu category selection based on status

  waifu/
    api.go                      # Waifu.pics API client
    render.go                   # Image rendering (protocol dispatch)
    process.go                  # Image processing (resize, format conversion)
    cache.go                    # Image cache (disk-based, TTL, size limit)
    session.go                  # Session-based waifu caching (LRU eviction)
    prefetch.go                 # Background image prefetching

  tests/
    mocks/                      # Mock data generators for testing
    visual/                     # Visual regression tests with golden files
    integration/                # Shell integration tests
```

## Data Flow

### Collection Pipeline

```
Daemon start
  |
  +-- Load config.yaml
  |
  +-- Create collectors (claude, billing, infra, fastfetch, sysmetrics)
  |
  +-- Poll loop (every poll_interval):
        |
        +-- For each collector:
        |     |
        |     +-- collector.Collect(ctx) -> CollectResult
        |     |
        |     +-- cache.Store.Put("collector_name", result.Data)
        |     |     Writes to ~/.cache/prompt-pulse/<name>.json
        |     |
        |     +-- Stagger delay between accounts
        |
        +-- Write health.json (timestamp, status, warnings)
```

### Display Pipeline (Banner Mode)

```
banner.Generate(ctx)
  |
  +-- cache.NewStore(cacheDir)
  |
  +-- Load cached data:
  |     claude  <- cache.GetTyped[ClaudeUsage]("claude", ttl)
  |     billing <- cache.GetTyped[BillingData]("billing", ttl)
  |     infra   <- cache.GetTyped[InfraStatus]("infra", ttl)
  |     fastfetch <- cache.GetTyped[FastfetchData]("fastfetch", ttl)
  |     sysmetrics <- cache.GetTyped[SysMetricsData]("sysmetrics", ttl)
  |
  +-- status.Evaluate(claude, billing, infra) -> SystemStatus
  |
  +-- [Optional] Waifu image fetch/render
  |     status.SelectCategory(systemStatus) -> category
  |     waifu.GetOrFetch(sessionID, category) -> imageData
  |     waifu.RenderImage(imageData, protocol, cols, rows) -> string
  |
  +-- layout.NewResponsiveConfig(width, height) -> ResponsiveConfig
  |     Detects LayoutMode, computes ColumnConfig, LayoutFeatures
  |
  +-- buildSections(claude, billing, infra, fastfetch, sysmetrics, features)
  |     Returns []layout.Section with Title + Content lines
  |
  +-- layout.Render(imageContent, sections, billing) -> RenderResult
        Composes columns based on LayoutMode (1/2/3/4 columns)
```

### Display Pipeline (TUI Mode)

```
tea.NewProgram(model, WithAltScreen, WithMouseCellMotion)
  |
  +-- Init():
  |     Returns Batch(spinner.Tick, tickCmd(30s), fetchDataCmd)
  |
  +-- Update loop:
  |     |
  |     +-- tea.WindowSizeMsg -> set dimensions, create viewport
  |     |
  |     +-- tickMsg -> schedule next tick + fetchDataCmd
  |     |
  |     +-- dataRefreshMsg -> update model data, refreshViewport()
  |     |     {claude, billing, infra, fastfetch, sysmetrics}
  |     |
  |     +-- tea.KeyMsg -> dispatch via key.Matches(msg, keys.*)
  |     |     Tab/number: switch active tab, refreshViewport()
  |     |     j/k/pgup/pgdn/g/G: forward to viewport for scrolling
  |     |     r: trigger manual refresh
  |     |     ?: toggle help
  |     |     q/ctrl+c: tea.Quit
  |     |
  |     +-- tea.MouseMsg -> check bubblezone tab hits, forward to viewport
  |     |
  |     +-- spinner.TickMsg -> animate spinner when loading/refreshing
  |
  +-- View():
        header (tab bar with active highlight)
        viewport (scrollable tab content)
        footer (help keys + scroll position + last updated time)
```

## Bubbletea Patterns

### Message Types

| Message | Source | Effect |
|---------|--------|--------|
| `tickMsg` | `tea.Tick(30s)` | Schedules next tick and data fetch |
| `dataRefreshMsg` | `fetchDataCmd` | Updates model data, calls `refreshViewport()` |
| `tea.WindowSizeMsg` | Terminal resize | Recalculates viewport dimensions |
| `tea.KeyMsg` | Keyboard input | Tab switching, scrolling, help, quit |
| `tea.MouseMsg` | Mouse input | Tab clicks, scroll wheel |
| `spinner.TickMsg` | Spinner animation | Updates spinner frame during loading |

### Model State

```go
type Model struct {
    activeTab   Tab          // TabClaude | TabBilling | TabInfra | TabSystem
    width       int          // Terminal columns
    height      int          // Terminal rows
    ready       bool         // True after first WindowSizeMsg
    loading     bool         // True until first dataRefreshMsg
    spinning    bool         // True when spinner animation active

    // Data from collectors (via cache)
    claude      *collectors.ClaudeUsage
    billing     *collectors.BillingData
    infra       *collectors.InfraStatus
    fastfetch   *collectors.FastfetchData
    sysmetrics  *collectors.SysMetricsData

    // Bubbletea components
    viewport    viewport.Model   // Scrollable content area
    help        help.Model       // Keybinding help display
    spinner     spinner.Model    // Loading spinner
    zone        *zone.Manager    // Clickable region tracking
}
```

### Tab Content Rendering

Each tab has a dedicated render function that receives typed data and terminal
dimensions:

| Tab | Function | Data |
|-----|----------|------|
| Claude | `renderClaudeContent(claude, width, height)` | Per-account gauges, rate limits, extra usage |
| Billing | `renderBillingContent(billing, width, height)` | Summary, provider table with sparklines, MoM delta |
| Infra | `renderInfraContent(infra, width, height)` | Tailscale mesh, K8s clusters, node metrics |
| System | `renderSystemContent(fastfetch, sysmetrics, width, height)` | Fastfetch static info + live sparklines/gauges |

Content is set on the viewport via `m.viewport.SetContent(content)`, enabling
native scrolling via the viewport component.

## KeyRegistry

The `KeyRegistry` is the single source of truth for all keybindings. It is
used by:

1. **TUI key handling** - `keys.go` defines the bubbletea `keyMap` used in `Update()`
2. **Man page generation** - `docs/manpage/` reads the registry to emit roff keybinding tables
3. **`--keys` flag** - CLI output in table or JSON format
4. **Help display** - `keyMap.ShortHelp()` and `keyMap.FullHelp()` for the bubbles/help component

Each entry contains:

```go
type KeyEntry struct {
    Binding  key.Binding    // Charmbracelet key binding (keys + help text)
    Mode     KeyMode        // "tui" | "shell" | "starship"
    Category KeyCategory    // "navigation" | "scroll" | "system" | "data"
    Since    string         // Version when introduced (e.g., "0.2.0")
}
```

Modes:
- **ModeTUI**: Active in the interactive dashboard (`--tui`)
- **ModeShell**: Active in shell integration scripts (`--shell`)
- **ModeStarship**: Active in starship prompt modules

Duplicate detection is enforced via `HasDuplicateKeys()` which checks for
conflicting key assignments within the same mode.

## Color System

### NO_COLOR Compliance

The `display/color/` package implements the NO_COLOR specification:

1. `ShouldDisableColor()` returns true if:
   - `NO_COLOR` environment variable is set (any value)
   - stdout is not a TTY (piped or redirected)

2. `Apply()` calls `lipgloss.SetColorProfile(termenv.Ascii)` when color should
   be disabled, causing all `lipgloss.Style.Render()` calls to produce plain text.

3. `StripANSI()` is a safety net applied to banner output when `colorEnabled`
   is false, catching any ANSI sequences that bypassed lipgloss.

### Theme Presets

Three presets are available via `--theme`:

| Preset | Description | Borders | Compact |
|--------|-------------|---------|---------|
| `monitoring` | Dark theme for status monitoring (default) | Yes | No |
| `minimal` | Clean, low-distraction | No | Yes |
| `full` | Rich theme with all visual features | Yes | No |

Each preset defines a 7-color palette (Primary, Secondary, Success, Warning,
Danger, Muted, Background) plus layout flags (ShowBorders, CompactMode).

`ApplyTheme()` updates package-level lipgloss style variables at runtime,
enabling theme switching without application restart.

### Lipgloss Color Profile

Colors flow through lipgloss with automatic degradation:
- True color terminals: Full hex colors
- 256-color terminals: Nearest ANSI color
- 16-color terminals: Nearest basic ANSI
- Ascii profile (NO_COLOR): No color codes emitted

## Responsive Layout

### LayoutMode Detection

Terminal dimensions are mapped to layout modes in `responsive.go`:

| Mode | MinWidth | MinHeight | Columns |
|------|----------|-----------|---------|
| Compact | 80 | 24 | 1 (vertical stack) |
| Standard | 120 | 24 | 3 (image + main + sparklines) |
| Wide | 160 | 35 | 4 (image + main + info + sparklines) |
| UltraWide | 200 | 50 | 4 (image + main + info + sparklines) |

### LayoutFeatures Progressive Density

Each wider mode enables strictly more features:

```
Compact:    VerticalStack=true,  no images, no sparklines
Standard:   ShowImage, ShowSparklines, ShowFullMetrics, ShowBorders
Wide:       + ShowGauges, ShowSysMetrics, ShowNodeMetrics, ShowBillingDelta
UltraWide:  + ShowSysMetricsSparklines, ShowExtraUsage
```

### Column Allocation

Column widths are computed by `columnsForMode(mode, termWidth)`:

- **Compact**: MainCols = termWidth - 4 (box borders)
- **Standard**: ImageCols=28, SparklineCols=20, MainCols=remaining
- **Wide**: ImageCols=36, MainCols=50, SparklineCols=24, InfoCols=remaining
- **UltraWide**: ImageCols=48, MainCols=50, InfoCols=50, SparklineCols=remaining

### ANSI-Aware String Handling

All column composition uses ANSI-aware string functions:
- `visibleLen(s)` - counts visible characters, skipping escape sequences
- `padToWidth(s, width)` - pads to exact visible width
- `truncateToWidth(s, width)` - truncates preserving ANSI sequences
- `padOrTruncate(s, width)` - combines pad and truncate

## Collector Architecture

### Interface

All collectors implement:

```go
type Collector interface {
    Name() string                                    // Unique identifier
    Description() string                             // Human-readable description
    Interval() time.Duration                         // Recommended poll interval
    Collect(ctx context.Context) (*CollectResult, error)
}
```

### Registry

Collectors are registered in a `Registry` and accessed by name:

```go
registry := collectors.NewRegistry()
registry.Register(claudeCollector)
registry.Register(billingCollector)
// ...
```

### SysMetrics Ring Buffers

The sysmetrics collector maintains internal ring buffers that survive daemon
restarts via cache persistence:

1. On first `Collect()` call, loads previous history from `~/.cache/prompt-pulse/sysmetrics.json`
2. Each collection appends to internal `cpuHistory`, `ramHistory`, `diskHistory` slices
3. `appendAndTrim()` enforces a maximum of 60 samples (1 hour at 1-minute intervals)
4. The full history is copied into the `CollectResult.Data` for cache serialization
5. Sparkline widgets consume the history arrays directly

Data sources (Linux):
- CPU: `/proc/stat` - delta computation between consecutive readings (idle vs total jiffies)
- RAM: `/proc/meminfo` - `(MemTotal - MemAvailable) / MemTotal * 100`
- Disk: `syscall.Statfs("/")` - `used / (used + available) * 100`
- Load: `/proc/loadavg` - 1, 5, 15 minute load averages

All file openers are injectable via function fields for testability without
requiring actual `/proc` filesystem access.

## Widget Library

### Gauge (`widgets/gauge.go`)

Renders horizontal bar charts with threshold coloring:

```
5h usage  [========------]  65%
```

Configuration: Width, Percent, Label, ThresholdWarning (70), ThresholdDanger (90),
FilledChar, EmptyChar.

### Sparkline (`widgets/sparkline.go`)

Renders time-series data as Unicode block characters:

```
CPU: [_..=##^.._]
```

Uses 8 block levels for resolution. Configurable width, min/max range,
label, and color.

### Table (`widgets/table.go`)

Renders formatted tables with column alignment and max width constraints.

### StatusIndicator (`widgets/status.go`)

Renders colored status bullets: healthy (green), warning (yellow),
critical (red), unknown (gray).

### FormatMonthOverMonth (`widgets/billing_panel.go`)

Computes billing delta between current and previous month spend:
- Up arrow (red) for cost increase
- Down arrow (green) for cost decrease
- Right arrow (gray) for flat (< 1% change)
- Includes percentage delta

## Image Rendering

### Protocol Detection

`display/render/protocol.go` detects terminal image protocol support:

1. Kitty (via `TERM_PROGRAM=kitty` or escape sequence probe)
2. iTerm2 (via `TERM_PROGRAM=iTerm.app`)
3. Sixel (via escape sequence probe)
4. Chafa (external binary detection)
5. Unicode half-block (fallback)
6. None (no image support)

### Rendering Pipeline

```
waifu.RenderImage(data, config)
  |
  +-- Protocol dispatch:
  |     Kitty:    Base64 encode, send via APC escape
  |     iTerm2:   Base64 encode, send via OSC 1337
  |     Sixel:    Convert via sixel encoder
  |     Chafa:    Subprocess: chafa --size=WxH --format=symbols
  |     Unicode:  Half-block character rendering
  |
  +-- Size constraints: MaxCols x MaxRows (from layout)
```

## Configuration

Configuration is loaded from `~/.config/prompt-pulse/config.yaml` with
validation via `config.Validate()`. Key sections:

- `daemon`: poll_interval, cache_dir, log_file, stagger_delay, max_parallel
- `accounts`: Claude accounts (subscription/api), cloud billing providers
- `tailscale`: API key, CLI fallback, node metrics
- `kubernetes`: Cluster contexts, namespaces, dashboard URLs
- `display`: Theme, hyperlinks, waifu settings, fastfetch toggle
- `starship`: Module enable/disable toggles

Nix integration generates the config file from `nix/home-manager/prompt-pulse.nix`
options, with all fields mapped to the YAML structure.
