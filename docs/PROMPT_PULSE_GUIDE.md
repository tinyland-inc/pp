# Prompt-Pulse User Guide

## Overview

prompt-pulse is a shell-integrated infrastructure monitoring dashboard written in
Go. It consolidates status information from multiple sources into a single
glanceable view, available as Starship prompt segments, a system login banner, or
a full interactive TUI.

```
+------------------------------------------------------------------+
| prompt-pulse                                                      |
|                                                                   |
|  +-----------+-----------+-----------+                            |
|  | [Claude]  |  Billing  |  Infra    |  <-- tab bar               |
|  +-----------+-----------+-----------+                            |
|                                                                   |
|  Claude AI Usage                                                  |
|                                                                   |
|  personal  [SUB] pro  OK                                          |
|  5h usage  [========------] 62%     Resets in 43m                 |
|  7d usage  [====----------] 31%     Resets at 18:00               |
|  ------                                                           |
|  work      [API] tier_2  OK                                       |
|  Requests  [====---------] 34%     800 / 2000 used               |
|  Tokens    [==-----------] 18%     9k / 50k used                  |
|                                                                   |
|  q: quit | tab: switch | 1-3: jump    Updated: 14:32:07          |
+------------------------------------------------------------------+
```

### What It Collects

- **Claude AI** -- Usage across up to 5 accounts (subscription and API key),
  including 5-hour and 7-day rolling windows, extra usage credits, and API rate
  limits.
- **Cloud Billing** -- Current-month spend, forecasts, and budget status from
  Civo, DigitalOcean, AWS Cost Explorer, and DreamHost.
- **Infrastructure** -- Tailscale mesh node connectivity and Kubernetes cluster
  health (node readiness, CPU, memory, pod counts).
- **System Status** -- Automatic health evaluation with configurable thresholds
  that drive waifu banner mood selection.

### Display Modes

| Mode | Flag | Description |
|------|------|-------------|
| Single pass | *(none)* | Collect once, write to cache, exit |
| Daemon | `--daemon` | Background polling loop with PID file |
| TUI | `--tui` | Interactive Bubbletea dashboard |
| Banner | `--banner` | Terminal login banner with optional waifu image |
| Starship | `--starship <module>` | One-line output for Starship prompt segment |

---

## Quick Start

### 1. Install

```bash
# Using Nix (recommended)
nix profile install .#prompt-pulse

# Or build from source
go build -o prompt-pulse .
```

### 2. Create a minimal configuration

```bash
mkdir -p ~/.config/prompt-pulse

cat > ~/.config/prompt-pulse/config.yaml << 'EOF'
daemon:
  poll_interval: "15m"

accounts:
  claude:
    - name: personal
      type: subscription
      credentials_path: ~/.claude/.credentials.json
      enabled: true
EOF
```

### 3. Run a single collection pass

```bash
prompt-pulse
```

This fetches data from all configured providers, writes results to the cache at
`~/.cache/prompt-pulse/`, and exits.

### 4. Check status from the command line

```bash
prompt-pulse --starship claude
# Output: personal:pro 62%

prompt-pulse --starship billing
# Output: $142 ($180 forecast)

prompt-pulse --starship infra
# Output: ts:5/7 k8s:civo:healthy
```

### 5. Launch the TUI

```bash
prompt-pulse --tui
```

### 6. Add to your shell

Source the shell integration in your RC file to get the `pp-status`, `pp-tui`,
`pp-banner`, `pp-daemon-start`, and `pp-daemon-stop` convenience functions. See
the [Shell Integration](#shell-integration) section for details.

---

## Installation

### Nix (Recommended)

prompt-pulse is packaged as a Nix flake output. If your system uses the
crush-dots flake:

```bash
# Install to user profile
nix profile install .#prompt-pulse

# Or add to your home-manager packages
# (see nix/home-manager/prompt-pulse.nix)
```

### Go Install

Requires Go 1.23 or later:

```bash
go install gitlab.com/tinyland/lab/prompt-pulse@latest
```

### From Source

```bash
git clone https://gitlab.com/tinyland/lab/prompt-pulse.git
cd prompt-pulse
go build -o prompt-pulse .
```

Build with version metadata:

```bash
go build -ldflags "\
  -X main.version=0.1.0 \
  -X main.commit=$(git rev-parse --short HEAD) \
  -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o prompt-pulse .
```

---

## Configuration

### Config File Location

The default configuration file is:

```
~/.config/prompt-pulse/config.yaml
```

Override with the `--config` flag:

```bash
prompt-pulse --config /path/to/custom-config.yaml
```

If the config file does not exist, prompt-pulse uses built-in defaults and
continues without error.

### Full Configuration Reference

```yaml
# ~/.config/prompt-pulse/config.yaml
# All fields are optional -- defaults are shown in comments.

daemon:
  # How often the daemon polls all collectors.
  # Go duration format: "15m", "1h", "30s", "2h30m".
  poll_interval: "15m"          # default: "15m"

  # Directory for cached API responses (JSON files).
  cache_dir: "~/.cache/prompt-pulse"  # default: ~/.cache/prompt-pulse

  # Log file path. Directory is created automatically.
  log_file: "~/.local/log/prompt-pulse.log"  # default: ~/.local/log/prompt-pulse.log

accounts:
  # Claude AI accounts. Maximum 5 accounts.
  claude:
    - name: "personal"                            # required: human-readable label
      type: "subscription"                        # required: "subscription" or "api"
      credentials_path: "~/.claude/.credentials.json"  # required for subscription
      api_key_env: ""                             # required for api (env var name)
      enabled: true                               # default: true

    - name: "work-api"
      type: "api"
      api_key_env: "ANTHROPIC_API_KEY"
      enabled: true

  # Civo cloud
  civo:
    api_key_env: "CIVO_API_KEY"    # default: "CIVO_API_KEY"
    region: "NYC1"                 # default: "NYC1"

  # DigitalOcean
  digitalocean:
    api_key_env: "DIGITALOCEAN_TOKEN"  # default: "DIGITALOCEAN_TOKEN"

  # AWS Cost Explorer
  aws:
    profile: "default"             # default: "default"
    regions:                       # default: ["us-east-1"]
      - "us-east-1"
      - "us-west-2"

  # DreamHost
  dreamhost:
    api_key_env: "DREAMHOST_API_KEY"  # default: "DREAMHOST_API_KEY"

# Tailscale mesh networking
tailscale:
  tailnet: "tinyland.ts.net"       # your tailnet name (empty = disabled)
  api_key_env: "TAILSCALE_API_KEY" # default: "TAILSCALE_API_KEY"
  use_cli_fallback: true           # default: true; falls back to `tailscale` CLI

# Kubernetes cluster monitoring
kubernetes:
  contexts:
    - name: "civo-production"       # kubectl context name
      kubeconfig: "~/.kube/config"  # kubeconfig file path
      namespace: "default"          # namespace to query
      dashboard_url: "https://dashboard.example.com"

    - name: "homelab-k3s"
      kubeconfig: "~/.kube/homelab.yaml"
      namespace: "fuzzy-dev"

# Display settings
display:
  # Theme: "minimal", "full", or "monitoring"
  theme: "monitoring"              # default: "monitoring"

  # Enable OSC 8 terminal hyperlinks for dashboard URLs
  enable_hyperlinks: true          # default: true

  # Waifu banner image settings (waifu.pics API)
  waifu:
    enabled: false                 # default: false
    category: "neko"               # default: "neko"; override for status-based selection
    cache_ttl: "24h"               # default: "24h"
    max_cache_mb: 50               # default: 50

# Starship prompt module toggles
starship:
  modules:
    claude: true                   # default: true
    billing: true                  # default: true
    infra: true                    # default: true
```

### Example Configurations

#### Minimal (Claude only)

```yaml
accounts:
  claude:
    - name: personal
      type: subscription
      credentials_path: ~/.claude/.credentials.json
      enabled: true
```

#### Full (all providers)

```yaml
daemon:
  poll_interval: "10m"

accounts:
  claude:
    - name: personal
      type: subscription
      credentials_path: ~/.claude/.credentials.json
      enabled: true
    - name: work
      type: api
      api_key_env: ANTHROPIC_API_KEY_WORK
      enabled: true
  civo:
    api_key_env: CIVO_API_KEY
    region: NYC1
  digitalocean:
    api_key_env: DIGITALOCEAN_TOKEN
  aws:
    profile: production
    regions: [us-east-1, eu-west-1]
  dreamhost:
    api_key_env: DREAMHOST_API_KEY

tailscale:
  tailnet: tinyland.ts.net
  api_key_env: TAILSCALE_API_KEY
  use_cli_fallback: true

kubernetes:
  contexts:
    - name: civo-cluster
      kubeconfig: ~/.kube/config
      namespace: fuzzy-dev
      dashboard_url: https://dashboard.civo.com/clusters/abc123
    - name: homelab-rke2
      kubeconfig: ~/.kube/homelab.yaml
      namespace: default

display:
  theme: monitoring
  enable_hyperlinks: true
  waifu:
    enabled: true
    category: ""  # empty = auto-select based on system status
    cache_ttl: "24h"
    max_cache_mb: 50

starship:
  modules:
    claude: true
    billing: true
    infra: true
```

#### Custom Thresholds (status evaluation only)

Status evaluation thresholds are currently set at compile time via
`status.DefaultEvaluatorConfig()`. The defaults are:

| Threshold | Default | Description |
|-----------|---------|-------------|
| Claude warning | 80% | 5-hour utilization triggers warning |
| Claude critical | 95% | 5-hour utilization triggers critical |
| Billing budget warning | 80% | % of budget triggers warning |
| Billing budget critical | 100% | % of budget triggers critical |
| Tailscale warning | 50% | % of nodes online triggers warning |
| K8s node ready minimum | 80% | % of nodes ready triggers warning |

---

## CLI Usage

### Flags

```
prompt-pulse [flags]

Flags:
  -banner           Display system status banner
  -daemon           Run background polling daemon
  -tui              Launch interactive Bubbletea TUI
  -starship string  Output one-line Starship format (claude|billing|infra)
  -config string    Path to configuration file
                    (default: ~/.config/prompt-pulse/config.yaml)
  -verbose          Enable verbose (debug-level) logging
  -version          Print version and exit
```

### Default Mode (no flags)

When invoked with no flags, prompt-pulse runs a **single collection pass**:
collects data from all configured providers, writes results to the cache
directory, and exits. This is useful for cron jobs or manual cache refreshes.

```bash
prompt-pulse                         # collect once with default config
prompt-pulse --config /etc/pp.yaml   # collect once with custom config
prompt-pulse --verbose               # collect with debug logging
```

### Version

```bash
prompt-pulse --version
# Output: prompt-pulse 0.1.0 (abc1234) built 2026-01-15T12:00:00Z
```

### Shell Functions (installed via shell integration)

When the shell integration is sourced, the following functions become available:

| Function | Description |
|----------|-------------|
| `pp-status` | Print all three Starship modules (claude, billing, infra) |
| `pp-tui` | Launch the interactive TUI |
| `pp-banner` | Display the system status banner |
| `pp-daemon-start` | Start the daemon in the background, print its PID |
| `pp-daemon-stop` | Stop the running daemon via `pkill` |

Additionally, **Ctrl+P** launches the TUI as a keybinding (configurable).

---

## Daemon Mode

The daemon runs a continuous background polling loop that keeps the cache
fresh for Starship segments and banner generation.

### Starting the Daemon

```bash
# Directly
prompt-pulse --daemon

# Via shell function (backgrounds automatically)
pp-daemon-start
# Output: prompt-pulse daemon started (PID: 12345)
```

### PID File Management

The daemon writes a PID file to `{cache_dir}/prompt-pulse.pid` (default:
`~/.cache/prompt-pulse/prompt-pulse.pid`).

- On startup, it checks if another instance is already running by reading the
  PID file and sending signal 0 to the recorded process.
- If the PID file contains a stale PID (process no longer exists), the file is
  cleaned up automatically.
- On shutdown, the PID file is removed.
- If another daemon is already running, startup fails with an error message
  including the existing PID.

### Polling Intervals

The daemon uses two levels of interval control:

1. **Global poll interval** (`daemon.poll_interval`): The ticker that drives
   collection passes. Default: 15 minutes.

2. **Per-collector intervals**: Each collector declares its own recommended
   interval via the `Interval()` method. If a collector ran more recently than
   its declared interval, it is skipped during that pass. This prevents
   rate-limited APIs from being hit too frequently even with a short global
   interval.

### Concurrent Collection

During each pass, all eligible collectors run concurrently in separate
goroutines. Results are written to the cache independently, so a slow provider
does not block others.

### Graceful Shutdown

The daemon handles `SIGINT` and `SIGTERM` signals:

1. Receives signal
2. Logs "received shutdown signal"
3. Cancels the context (stops in-flight collectors)
4. Logs final cache state (key names and ages)
5. Removes the PID file
6. Exits cleanly

```bash
# Stop via signal
kill $(cat ~/.cache/prompt-pulse/prompt-pulse.pid)

# Or via shell function
pp-daemon-stop
```

---

## TUI Dashboard

The TUI is an interactive Bubbletea application that presents cached data across
three tabs. It runs in the alternate terminal screen.

```bash
prompt-pulse --tui
# or
pp-tui
```

### Tab Navigation

| Key | Action |
|-----|--------|
| `Tab` or `Right` | Next tab |
| `Shift+Tab` or `Left` | Previous tab |
| `1` | Jump to Claude tab |
| `2` | Jump to Billing tab |
| `3` | Jump to Infra tab |
| `q` or `Ctrl+C` | Quit |

### Responsive Layout

The TUI detects terminal width and adjusts automatically:

| Terminal Width | Layout | Gauge Width | Features |
|----------------|--------|-------------|----------|
| < 60 cols | Compact | 10 chars | Tables only, no sparklines |
| 60-120 cols | Normal | 20 chars | Tables + sparklines |
| > 120 cols | Wide | 30 chars | Tables + sparklines + mini gauges |

### Claude Tab

Displays per-account usage information:

- **Subscription accounts**: 5-hour and 7-day usage gauge bars with reset
  countdown timers, plus extra usage credits (if enabled).
- **API accounts**: Request and token usage gauge bars with limits and reset
  times.
- **Status badges**: SUB/API type badge, tier name, and OK/ERR indicator per
  account.

Gauge bars change color at configurable thresholds:
- Below 70%: normal (green)
- 70-90%: warning (yellow)
- Above 90%: danger (red)

### Billing Tab

Displays aggregate cloud spending:

- **Summary**: Current month total, forecast (if available), and budget status
  with OVER BUDGET / under budget indicators.
- **Provider table**: Per-provider breakdown with columns for status, spend,
  forecast, previous month, and billing period.
- **Dashboard links**: OSC 8 hyperlinks to provider billing dashboards (when
  `display.enable_hyperlinks` is true).

### Infra Tab

Displays infrastructure health:

- **Tailscale Mesh**: Online/total node count, connectivity gauge, and node
  table with name, hostname, IP, OS, online status, and last seen time.
- **Kubernetes Clusters**: Per-cluster health gauge (ready/total nodes), status
  indicator (healthy/degraded/offline), and node table with CPU, memory, and
  pod counts.

### Themes

Three built-in themes are available via `display.theme`:

| Theme | Description |
|-------|-------------|
| `monitoring` | Dark purple theme optimized for status monitoring (default) |
| `minimal` | Clean, low-distraction theme with no borders and compact padding |
| `full` | Rich theme with all visual features and lighter colors |

---

## Starship Integration

prompt-pulse integrates with [Starship](https://starship.rs/) via custom
modules that call the binary and display one-line status in your prompt.

### Setup

**Step 1**: Generate the Starship configuration snippet:

The generated TOML defines three custom modules. Add them to
`~/.config/starship.toml`:

```toml
# prompt-pulse Starship custom modules

[custom.pp_claude]
command = "prompt-pulse --starship claude"
when = "command -v prompt-pulse"
format = "[$symbol($output)]($style) "
symbol = ""
style = "purple"
shell = ["bash", "--noprofile", "--norc"]

[custom.pp_billing]
command = "prompt-pulse --starship billing"
when = "command -v prompt-pulse"
format = "[$symbol($output)]($style) "
symbol = "$"
style = "green"
shell = ["bash", "--noprofile", "--norc"]

[custom.pp_infra]
command = "prompt-pulse --starship infra"
when = "command -v prompt-pulse"
format = "[$symbol($output)]($style) "
symbol = ""
style = "cyan"
shell = ["bash", "--noprofile", "--norc"]
```

**Step 2**: Add the module references to your Starship `format` string:

```toml
format = """
$directory\
$git_branch\
$git_status\
${custom.pp_claude}\
${custom.pp_billing}\
${custom.pp_infra}\
$character"""
```

### Available Modules

#### pp_claude

Displays per-account usage in the format:

```
account_name:tier NN% | account_name:tier NN%
```

Examples:
```
personal:pro 62% | work:tier_2 34%
personal:pro 62%
work-api:ERR
```

For subscription accounts, the percentage is the 5-hour utilization. For API
accounts, it is the requests used percentage. Accounts with errors show `:ERR`.

#### pp_billing

Displays aggregate spending:

```
$142 ($180 forecast)
$85
$200 ($250 forecast) OVER BUDGET
```

#### pp_infra

Displays infrastructure summary:

```
ts:5/7 k8s:civo:healthy
ts:3/3
k8s:homelab:degraded
```

### Staleness Indicator

When cached data is older than the configured `poll_interval` (TTL), Starship
output is suffixed with ` ?` to indicate staleness:

```
personal:pro 62% ?
```

This tells you the daemon may not be running or the last collection failed.

---

## Shell Integration

prompt-pulse generates shell integration scripts that provide keybindings and
convenience functions. Four shells are supported.

### Bash

Add to `~/.bashrc`:

```bash
# prompt-pulse integration
eval "$(prompt-pulse shell bash)"
# Or source a pre-generated file:
# source ~/.config/prompt-pulse/bash-integration.sh
```

The integration provides:
- `Ctrl+P` keybinding to launch the TUI (via `bind -x`)
- `pp-status`, `pp-tui`, `pp-banner`, `pp-daemon-start`, `pp-daemon-stop`

### Zsh

Add to `~/.zshrc`:

```zsh
# prompt-pulse integration
eval "$(prompt-pulse shell zsh)"
```

The integration provides:
- `Ctrl+P` keybinding to launch the TUI (via `zle` widget + `bindkey`)
- `pp-status`, `pp-tui`, `pp-banner`, `pp-daemon-start`, `pp-daemon-stop`
- Tab completion for `prompt-pulse` flags

### Fish

Add to `~/.config/fish/config.fish`:

```fish
# prompt-pulse integration
prompt-pulse shell fish | source
```

The integration provides:
- `Ctrl+P` keybinding to launch the TUI (via `bind`)
- `pp-status`, `pp-tui`, `pp-banner`, `pp-daemon-start`, `pp-daemon-stop`
- Tab completions for all flags including `--starship` subcommands

### Nushell

Add to `~/.config/nushell/config.nu`:

```nu
# prompt-pulse integration
source ~/.config/prompt-pulse/nushell-integration.nu
```

The Nushell integration provides:
- `pp-status`, `pp-tui`, `pp-daemon-start`, `pp-daemon-stop`
- Tab completions via `extern` definitions
- Keybinding instructions (must be added manually to `$env.config.keybindings`
  because Nushell keybindings cannot be set dynamically):

```nu
# Add to $env.config.keybindings in config.nu:
{
    name: prompt_pulse_tui
    modifier: control
    keycode: char_p
    mode: [emacs vi_normal vi_insert]
    event: {
        send: executehostcommand
        cmd: "prompt-pulse --tui"
    }
}
```

---

## Waifu Banner

The banner mode displays a system status summary suitable for terminal login
(e.g., in `.bashrc` or `.zshrc`). Optionally, it includes an anime-style image
from the [waifu.pics](https://waifu.pics/) API, rendered inline using terminal
image protocols.

```bash
prompt-pulse --banner
```

### Enabling

Set `display.waifu.enabled: true` in your config:

```yaml
display:
  waifu:
    enabled: true
    category: ""        # empty = auto-select based on status
    cache_ttl: "24h"
    max_cache_mb: 50
```

### Banner Layout

When a waifu image is available, the banner renders side-by-side:

```
[image]  | hostname :: healthy
[image]  | ────────────────────────
[image]  |
[image]  | Claude
[image]  |   personal: 62% (5h) | 31% (7d)
[image]  |   work: 800/2000 req
[image]  |
[image]  | Billing
[image]  |   $142 this month ($180 forecast)
[image]  |
[image]  | Infrastructure
[image]  |   ts: 5/7 online
[image]  |   k8s: civo (healthy)
[image]  |
[image]  |   uptime: 14d 6h 32m
```

Without an image, the info panel renders full-width.

### Status-Based Categories

When `display.waifu.category` is empty, the banner automatically selects a
waifu.pics category based on the evaluated system health:

| Status Level | Categories |
|-------------|-----------|
| Healthy | happy, smile, wave, dance, wink, highfive |
| Warning | smug, blush, poke, nom, pat |
| Critical | cry, bonk, slap, bite, kill |
| Unknown | waifu, neko, shinobu, megumin |

A random category from the appropriate list is chosen each time.

### Custom Categories

Set `display.waifu.category` to force a specific category regardless of status:

```yaml
display:
  waifu:
    enabled: true
    category: "neko"  # always use neko
```

Valid SFW categories: `waifu`, `neko`, `shinobu`, `megumin`, `bully`, `cuddle`,
`cry`, `hug`, `awoo`, `kiss`, `lick`, `pat`, `smug`, `bonk`, `yeet`, `blush`,
`smile`, `wave`, `highfive`, `handhold`, `nom`, `bite`, `glomp`, `slap`,
`kill`, `kick`, `happy`, `wink`, `poke`, `dance`, `cringe`.

### Cache Management

Images are cached in `{cache_dir}/waifu/` (default:
`~/.cache/prompt-pulse/waifu/`). Cache behavior:

- **TTL**: Images expire after `display.waifu.cache_ttl` (default: 24 hours).
  Expired images are automatically removed on access.
- **Size limit**: Total cache size is capped at `display.waifu.max_cache_mb`
  (default: 50 MB). When exceeded, the oldest images are evicted (LRU-like).
- **Prefetching**: The daemon prefetches images in the background so the banner
  can render instantly (target: under 100ms with cached data). The banner itself
  only reads from cache and never makes network requests.
- **Atomic writes**: Images are written via temp file + rename to prevent
  corruption from concurrent access.

---

## Status Evaluation

prompt-pulse evaluates system health by analyzing cached collector data against
configurable thresholds. The result drives waifu category selection and banner
status display.

### Health Levels

| Level | Severity | Description |
|-------|----------|-------------|
| `healthy` | 0 (best) | All systems normal |
| `unknown` | 1 | Insufficient data to evaluate |
| `warning` | 2 | Something needs attention |
| `critical` | 3 (worst) | Immediate attention required |

The **overall** status is the worst (highest severity) across all components.

### Thresholds

Default thresholds used for evaluation:

| Component | Condition | Level |
|-----------|-----------|-------|
| Claude | 5h utilization > 80% | Warning |
| Claude | 5h utilization > 95% | Critical |
| Claude | Account status not "ok" | Warning |
| Billing | Spend > 80% of budget | Warning |
| Billing | Spend > 100% of budget | Critical |
| Billing | Forecast exceeds budget | Warning |
| Billing | Provider status "error" | Warning |
| Tailscale | < 50% nodes online | Warning |
| Tailscale | 0 nodes online | Critical |
| Kubernetes | Cluster status "degraded" | Warning |
| Kubernetes | Cluster status "offline" | Critical |
| Kubernetes | < 80% nodes ready | Warning |

### Component Rules

Each component (claude, billing, infra) is evaluated independently. The worst
result from any sub-check becomes that component's level. The overall system
level is the worst across all three components.

---

## Circuit Breaker

Collectors are optionally wrapped with a circuit breaker that handles persistent
API failures gracefully.

### How It Works

The circuit breaker has three states:

```
            success
  +------+  ------>  +--------+
  | Open |           | Closed |  <-- normal operation
  +------+  <------  +--------+
      |     failure      |
      |   (>= max)       | failure
      |                   | (< max)
      v                   v
  +----------+       +--------+
  | wait...  |       | Closed |
  +----------+       +--------+
      |
      | timeout elapsed
      v
  +-----------+   success    +--------+
  | Half-Open | ----------> | Closed |
  +-----------+              +--------+
      |
      | failure (re-open with backoff)
      v
  +------+
  | Open |
  +------+
```

1. **Closed** (normal): Requests pass through. Failures are counted.
2. **Open** (tripped): After `max_failures` consecutive failures, the circuit
   opens. Requests are blocked and return a synthetic warning. The collector is
   skipped entirely, saving API calls and reducing log noise.
3. **Half-Open** (probe): After `reset_timeout` elapses, one request is allowed
   through as a probe. If it succeeds, the circuit closes. If it fails, the
   circuit re-opens with an exponentially increased timeout.

### Configuration

Default circuit breaker settings:

| Setting | Default | Description |
|---------|---------|-------------|
| `max_failures` | 3 | Consecutive failures before opening |
| `reset_timeout` | 1 minute | Initial wait before half-open probe |
| `max_reset_timeout` | 30 minutes | Cap on exponential backoff |
| `backoff_multiplier` | 2.0 | Timeout multiplier on each re-open |

### Monitoring

Circuit breaker statistics are available programmatically via `Stats()`:

- `State`: Current state (closed/open/half_open)
- `ConsecutiveFails`: Current failure streak
- `TotalFailures`: Lifetime failure count
- `TotalSuccesses`: Lifetime success count
- `LastFailure` / `LastSuccess`: Timestamps
- `CurrentTimeout`: Current reset timeout (includes backoff)
- `ConsecutiveSkips`: Number of collection passes skipped while open

---

## Provider Setup

### Claude Accounts

prompt-pulse supports up to 5 Claude accounts, mixing subscription and API
types.

#### Subscription (OAuth)

Subscription accounts read credentials from the Claude Code credentials file:

```yaml
accounts:
  claude:
    - name: personal
      type: subscription
      credentials_path: ~/.claude/.credentials.json
      enabled: true
```

The credentials file is the standard file written by `claude login` at
`~/.claude/.credentials.json`. prompt-pulse reads the OAuth tokens from this
file to query subscription usage endpoints.

Subscription accounts track:
- 5-hour rolling usage window (utilization % and reset time)
- 7-day rolling usage window (utilization % and reset time)
- Extra usage credits (if enabled: monthly limit, used amount, utilization %)
- Subscription tier: `pro`, `max_5x`, `max_20x`

#### API Key

API accounts read the key from an environment variable:

```yaml
accounts:
  claude:
    - name: work-api
      type: api
      api_key_env: ANTHROPIC_API_KEY
      enabled: true
```

Ensure the environment variable is set before running prompt-pulse:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

API accounts track rate limits from Anthropic response headers:
- Requests: limit, remaining, reset time
- Tokens: limit, remaining, reset time
- API tier: `tier_1`, `tier_2`, `tier_3`, `tier_4`

### Civo

```yaml
accounts:
  civo:
    api_key_env: CIVO_API_KEY
    region: NYC1
```

Set the environment variable:

```bash
export CIVO_API_KEY="your-civo-api-key"
```

Get your API key from the [Civo Dashboard](https://dashboard.civo.com/security).

### DigitalOcean

```yaml
accounts:
  digitalocean:
    api_key_env: DIGITALOCEAN_TOKEN
```

```bash
export DIGITALOCEAN_TOKEN="your-do-token"
```

Create a token at [DigitalOcean API Settings](https://cloud.digitalocean.com/account/api/tokens).

### AWS Cost Explorer

```yaml
accounts:
  aws:
    profile: default
    regions:
      - us-east-1
```

AWS uses the standard credential chain (environment variables, `~/.aws/credentials`,
IAM role). The `profile` field selects the AWS CLI profile. Ensure the profile
has `ce:GetCostAndUsage` permissions.

### DreamHost

```yaml
accounts:
  dreamhost:
    api_key_env: DREAMHOST_API_KEY
```

```bash
export DREAMHOST_API_KEY="your-dreamhost-key"
```

### Tailscale

```yaml
tailscale:
  tailnet: tinyland.ts.net
  api_key_env: TAILSCALE_API_KEY
  use_cli_fallback: true
```

```bash
export TAILSCALE_API_KEY="tskey-api-..."
```

Create an API key at [Tailscale Admin Console](https://login.tailscale.com/admin/settings/keys).

When `use_cli_fallback` is true and the API key is missing or the API call
fails, prompt-pulse falls back to running `tailscale status --json` locally.

### Kubernetes

```yaml
kubernetes:
  contexts:
    - name: civo-cluster
      kubeconfig: ~/.kube/config
      namespace: fuzzy-dev
      dashboard_url: https://dashboard.civo.com/clusters/abc123
```

Each context uses the specified kubeconfig file and kubectl context name.
prompt-pulse queries the Kubernetes API for node status, resource usage, and
pod counts. The `dashboard_url` is used for OSC 8 hyperlinks in the TUI.

---

## Troubleshooting

### Common Issues

#### "failed to load config" on startup

The config file has a YAML syntax error. Validate with:

```bash
python3 -c "import yaml; yaml.safe_load(open('$HOME/.config/prompt-pulse/config.yaml'))"
```

#### Starship modules show nothing

1. Verify the daemon is running or run a manual collection:
   ```bash
   prompt-pulse --verbose
   ```
2. Check that cache files exist:
   ```bash
   ls -la ~/.cache/prompt-pulse/
   ```
3. Test individual modules:
   ```bash
   prompt-pulse --starship claude
   prompt-pulse --starship billing
   prompt-pulse --starship infra
   ```

Empty output means no cached data. Check the log file for errors.

#### "daemon already running (PID XXXX)"

Another daemon instance is active. Either stop it first or check if the PID
file is stale:

```bash
# Check if the process actually exists
kill -0 $(cat ~/.cache/prompt-pulse/prompt-pulse.pid) 2>/dev/null
echo $?  # 0 = running, 1 = stale

# Remove stale PID file
rm ~/.cache/prompt-pulse/prompt-pulse.pid
```

#### Banner shows "(no data)" for all sections

The banner only reads from cache, never from the network. Run a collection
first:

```bash
prompt-pulse                 # single collection pass
prompt-pulse --banner        # now shows data
```

Or start the daemon for continuous updates.

#### Waifu image not showing in banner

1. Verify waifu is enabled: `display.waifu.enabled: true`
2. The daemon must prefetch images. Run the daemon for at least one cycle.
3. Check the image cache:
   ```bash
   ls -la ~/.cache/prompt-pulse/waifu/
   ```
4. Verify your terminal supports image protocols (kitty, iTerm2, sixel).

### Debug Logging

Enable verbose logging to see detailed collection information:

```bash
prompt-pulse --verbose --daemon
```

Or for a single pass:

```bash
prompt-pulse --verbose
```

Logs are written to both stderr and the log file
(`~/.local/log/prompt-pulse.log` by default).

### Cache Inspection

Cache files are human-readable JSON stored at `~/.cache/prompt-pulse/`:

```bash
# List cached entries
ls ~/.cache/prompt-pulse/*.json

# View Claude data
cat ~/.cache/prompt-pulse/claude.json | python3 -m json.tool

# Check cache freshness (file modification time)
stat ~/.cache/prompt-pulse/claude.json

# View waifu image cache stats
ls -lhS ~/.cache/prompt-pulse/waifu/

# Clear all cached data
rm ~/.cache/prompt-pulse/*.json
```

---

## Architecture

### Package Structure

```
cmd/prompt-pulse/
+-- main.go                        Entry point, flag parsing, mode dispatch
+-- daemon.go                      Daemon loop, PID management, collector wiring
|
+-- config/
|   +-- config.go                  YAML config parsing, defaults, validation
|
+-- collectors/
|   +-- collector.go               Collector interface and Registry
|   +-- models.go                  Shared data models (ClaudeUsage, BillingData, InfraStatus)
|   +-- osc8.go                    OSC 8 hyperlink helpers
|   +-- claude/
|   |   +-- collector.go           Claude usage collector
|   |   +-- api.go                 Anthropic API client
|   |   +-- oauth.go               OAuth token refresh
|   |   +-- credentials.go         Credential file parsing
|   |   +-- ratelimit.go           Rate limit header parsing
|   +-- billing/
|   |   +-- collector.go           Aggregate billing collector
|   |   +-- civo.go                Civo billing API
|   |   +-- digitalocean.go        DigitalOcean billing API
|   |   +-- aws.go                 AWS Cost Explorer
|   |   +-- dreamhost.go           DreamHost billing API
|   |   +-- currency.go            Currency conversion helpers
|   +-- infra/
|   |   +-- collector.go           Infrastructure collector
|   |   +-- tailscale.go           Tailscale API + CLI fallback
|   |   +-- kubernetes.go          Kubernetes API queries
|   |   +-- dashboard.go           Dashboard URL generation
|   +-- retry/
|       +-- retry.go               Circuit breaker implementation
|
+-- cache/
|   +-- store.go                   JSON file-based cache with TTL
|
+-- status/
|   +-- evaluator.go               Health evaluation rules and thresholds
|   +-- selector.go                Status-to-waifu-category mapping
|
+-- display/
|   +-- banner/
|   |   +-- banner.go              Banner generation pipeline
|   |   +-- layout.go              Side-by-side layout composition
|   +-- starship/
|   |   +-- config.go              Starship TOML generation
|   |   +-- output.go              Cache-reading Starship module output
|   +-- tui/
|   |   +-- app.go                 Bubbletea model, Update, View
|   |   +-- keys.go                Keybindings
|   |   +-- theme.go               Color palette
|   |   +-- theme_presets.go       Monitoring/Minimal/Full theme presets
|   |   +-- layout.go              Responsive layout breakpoints
|   |   +-- claude_tab.go          Claude tab renderer
|   |   +-- billing_tab.go         Billing tab renderer
|   |   +-- infra_tab.go           Infra tab renderer
|   +-- widgets/
|       +-- gauge.go               Gauge bar rendering
|       +-- sparkline.go           Sparkline chart rendering
|       +-- table.go               Table rendering
|       +-- status.go              Status indicator rendering
|
+-- shell/
|   +-- common.go                  Shell type enum, IntegrationConfig, dispatch
|   +-- bash.go                    Bash integration generator
|   +-- zsh.go                     Zsh integration generator
|   +-- fish.go                    Fish integration generator
|   +-- nushell.go                 Nushell integration generator
|
+-- waifu/
    +-- api.go                     waifu.pics API client
    +-- cache.go                   Image cache with TTL and LRU eviction
    +-- process.go                 Image resizing and processing
    +-- render.go                  Terminal image protocol rendering
    +-- prefetch.go                Background image prefetching
```

### Data Flow

```
                                  +------------------+
                                  |  waifu.pics API  |
                                  +--------+---------+
                                           |
                                  prefetch |
                                           v
+------------------+              +--------+---------+
| Claude API       |              | Image Cache      |
| Civo API         | --collect--> | ~/.cache/prompt- |
| DigitalOcean API |              | pulse/waifu/     |
| AWS Cost Exp.    |              +------------------+
| DreamHost API    |
| Tailscale API    |              +------------------+
| Kubernetes API   | --collect--> | JSON Cache       |
+------------------+              | ~/.cache/prompt- |
                                  | pulse/*.json     |
        daemon loop               +--------+---------+
        (every 15m)                        |
                                  read     |  read
                               +-----------+-----------+
                               |           |           |
                               v           v           v
                          +--------+  +---------+  +--------+
                          |Starship|  | Banner  |  |  TUI   |
                          |Modules |  |Generator|  |        |
                          +--------+  +---------+  +--------+
                               |           |           |
                               v           v           v
                          [prompt]   [terminal]   [alt screen]
```

The key design principle is **cache-mediated decoupling**: the daemon writes to
cache files, and all display modes (Starship, banner, TUI) read from cache. This
means display operations are fast (no network) and the daemon can run
independently.
