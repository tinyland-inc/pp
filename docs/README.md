# prompt-pulse

A shell monitoring dashboard that aggregates Claude API usage, cloud billing, and infrastructure status into Starship prompt segments, an interactive TUI, and status banners.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
  - [Via Nix Flake](#via-nix-flake)
  - [Via Home Manager](#via-home-manager)
  - [Direct Build](#direct-build)
- [Configuration](#configuration)
  - [Daemon Settings](#daemon-settings)
  - [Claude Accounts](#claude-accounts)
  - [Cloud Billing](#cloud-billing)
  - [Tailscale](#tailscale)
  - [Kubernetes](#kubernetes)
  - [Display Settings](#display-settings)
  - [Starship Modules](#starship-modules)
- [Usage](#usage)
  - [Commands](#commands)
  - [Shell Aliases](#shell-aliases)
  - [Starship Integration](#starship-integration)
  - [Daemon Mode](#daemon-mode)
- [Architecture](#architecture)
- [Terminal Requirements](#terminal-requirements)
- [Troubleshooting](#troubleshooting)

## Overview

prompt-pulse is a multi-source infrastructure status aggregator designed for developers managing multiple Claude accounts, cloud providers, and infrastructure. It provides:

- **Claude Usage Tracking**: Monitor up to 5 Claude accounts (subscription or API-based)
- **Cloud Billing**: Real-time cost tracking for Civo, DigitalOcean, AWS, and DreamHost
- **Infrastructure Monitoring**: Kubernetes cluster health and Tailscale mesh status
- **Status Banners**: System status displays with optional waifu images
- **Starship Integration**: Compact prompt segments for at-a-glance monitoring
- **Interactive TUI**: Full-featured dashboard with keyboard navigation

## Features

| Feature | Description |
|---------|-------------|
| Multi-account Claude | Track usage across 5 accounts (subscription or API) |
| Cloud Billing | Civo, DigitalOcean, AWS, DreamHost cost monitoring |
| Infrastructure | Kubernetes cluster health, Tailscale mesh status |
| Waifu Banners | Status-based anime images via Kitty graphics protocol |
| Starship Modules | Custom prompt segments for claude/billing/infra |
| OSC 8 Hyperlinks | Clickable terminal links to dashboards |
| Background Daemon | Periodic data collection with file-based cache |
| Interactive TUI | Tab-based dashboard with Bubbletea |

## Installation

### Via Nix Flake

Add to your flake inputs and install the package:

```nix
{
  inputs.crush-dots.url = "gitlab:tinyland/lab/crush-dots";

  outputs = { self, nixpkgs, crush-dots, ... }: {
    # Use in a NixOS or home-manager configuration
  };
}
```

Install directly:

```bash
nix profile install gitlab:tinyland/lab/crush-dots#prompt-pulse
```

Or build locally:

```bash
cd /path/to/crush-dots
nix build .#prompt-pulse
./result/bin/prompt-pulse --version
```

### Via Home Manager

Enable the module in your home-manager configuration:

```nix
{ config, pkgs, ... }:

{
  imports = [
    # Import the prompt-pulse module
    ./nix/home-manager/prompt-pulse.nix
  ];

  tinyland.promptPulse = {
    enable = true;

    # Enable background daemon
    daemon.enable = true;
    daemon.pollInterval = "15m";

    # Configure Claude accounts
    accounts.claude = [
      {
        name = "personal";
        type = "subscription";
        credentialsPath = "${config.home.homeDirectory}/.claude/.credentials.json";
        enabled = true;
      }
      {
        name = "work";
        type = "api";
        apiKeyEnv = "ANTHROPIC_API_KEY";
        enabled = true;
      }
    ];

    # Cloud billing (environment variables hold API keys)
    accounts.civo.apiKeyEnv = "CIVO_API_KEY";
    accounts.digitalocean.apiKeyEnv = "DIGITALOCEAN_TOKEN";
    accounts.aws.profile = "default";
    accounts.aws.regions = [ "us-east-1" "us-west-2" ];

    # Tailscale monitoring
    tailscale.tailnet = "your-tailnet.ts.net";
    tailscale.useCLIFallback = true;

    # Kubernetes clusters
    kubernetes.contexts = [
      {
        name = "prod";
        namespace = "default";
        dashboardURL = "https://k8s.example.com";
      }
    ];

    # Display settings
    display.theme = "monitoring";
    display.enableHyperlinks = true;
    display.waifu.enable = true;
    display.waifu.category = "neko";

    # Starship integration
    starship.claude = true;
    starship.billing = true;
    starship.infra = true;

    # Shell integration
    shellIntegration.bash = true;
    shellIntegration.zsh = true;
    shellIntegration.enableAliases = true;
  };
}
```

The home-manager module automatically:
- Installs the prompt-pulse binary
- Generates `~/.config/prompt-pulse/config.yaml`
- Creates systemd user service (Linux) or launchd agent (macOS)
- Configures shell aliases and integration

### Direct Build

Build from source using Go:

```bash
cd cmd/prompt-pulse
go build -o prompt-pulse .
./prompt-pulse --version
```

With vendored dependencies (no network required):

```bash
go build -mod=vendor -o prompt-pulse .
```

## Configuration

prompt-pulse reads configuration from `~/.config/prompt-pulse/config.yaml`. A default configuration is used if the file does not exist.

### Full Configuration Example

```yaml
# Daemon settings
daemon:
  poll_interval: "15m"      # Duration between collection cycles
  cache_dir: ~/.cache/prompt-pulse
  log_file: ~/.local/log/prompt-pulse.log

# Claude AI accounts (max 5)
accounts:
  claude:
    - name: personal
      type: subscription           # Uses credentials file
      credentials_path: ~/.claude/.credentials.json
      enabled: true
    - name: work-api
      type: api                    # Uses environment variable
      api_key_env: ANTHROPIC_API_KEY
      enabled: true

  # Cloud billing providers
  civo:
    api_key_env: CIVO_API_KEY
    region: NYC1

  digitalocean:
    api_key_env: DIGITALOCEAN_TOKEN

  aws:
    profile: default
    regions:
      - us-east-1
      - us-west-2

  dreamhost:
    api_key_env: DREAMHOST_API_KEY

# Tailscale mesh monitoring
tailscale:
  tailnet: example.ts.net
  api_key_env: TAILSCALE_API_KEY
  use_cli_fallback: true

# Kubernetes cluster monitoring
kubernetes:
  contexts:
    - name: prod-cluster
      kubeconfig: ~/.kube/prod-config
      namespace: default
      dashboard_url: https://k8s.example.com
    - name: dev-cluster
      namespace: development

# Display settings
display:
  theme: monitoring              # minimal, full, or monitoring
  enable_hyperlinks: true        # OSC 8 clickable links
  waifu:
    enabled: true
    category: neko               # Image category
    cache_ttl: "24h"
    max_cache_mb: 50

# Starship prompt modules
starship:
  modules:
    claude: true
    billing: true
    infra: true
```

### Daemon Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `poll_interval` | `15m` | Duration between data collection cycles (e.g., `15m`, `1h`, `30s`) |
| `cache_dir` | `~/.cache/prompt-pulse` | Directory for cached API responses |
| `log_file` | `~/.local/log/prompt-pulse.log` | Log file path |

### Claude Accounts

prompt-pulse supports up to 5 Claude accounts. Each account can be either:

**Subscription Account** (uses Claude Code credentials):
```yaml
- name: personal
  type: subscription
  credentials_path: ~/.claude/.credentials.json
  enabled: true
```

**API Account** (uses API key from environment variable):
```yaml
- name: work
  type: api
  api_key_env: ANTHROPIC_API_KEY
  enabled: true
```

### Cloud Billing

Each cloud provider requires an API key stored in an environment variable:

| Provider | Environment Variable | Notes |
|----------|---------------------|-------|
| Civo | `CIVO_API_KEY` | Also requires region setting |
| DigitalOcean | `DIGITALOCEAN_TOKEN` | |
| AWS | Uses `AWS_PROFILE` | Requires AWS CLI configured |
| DreamHost | `DREAMHOST_API_KEY` | |

### Tailscale

```yaml
tailscale:
  tailnet: your-tailnet.ts.net
  api_key_env: TAILSCALE_API_KEY
  use_cli_fallback: true    # Fall back to `tailscale status` CLI
```

### Kubernetes

Monitor multiple Kubernetes clusters:

```yaml
kubernetes:
  contexts:
    - name: prod
      kubeconfig: ~/.kube/config    # Optional, uses default if empty
      namespace: production
      dashboard_url: https://dashboard.example.com
```

### Display Settings

| Setting | Values | Description |
|---------|--------|-------------|
| `theme` | `minimal`, `full`, `monitoring` | Display theme for TUI and banners |
| `enable_hyperlinks` | `true`/`false` | Enable OSC 8 clickable terminal links |
| `waifu.enabled` | `true`/`false` | Show waifu images in banners |
| `waifu.category` | string | Image category (e.g., `neko`, `waifu`) |
| `waifu.cache_ttl` | duration | How long cached images remain valid |
| `waifu.max_cache_mb` | integer | Maximum image cache size in MB |

### Starship Modules

Enable or disable individual Starship prompt modules:

```yaml
starship:
  modules:
    claude: true      # Claude usage percentage
    billing: true     # Cloud spend summary
    infra: true       # Infrastructure health
```

## Usage

### Commands

```bash
# Display version information
prompt-pulse --version

# Run a single data collection pass (default behavior)
prompt-pulse

# Launch interactive TUI dashboard
prompt-pulse --tui

# Display system status banner
prompt-pulse --banner

# Output Starship module format
prompt-pulse --starship claude
prompt-pulse --starship billing
prompt-pulse --starship infra

# Run as background daemon
prompt-pulse --daemon

# Specify custom config file
prompt-pulse --config /path/to/config.yaml

# Enable verbose logging
prompt-pulse --verbose
```

### Shell Aliases

When shell integration is enabled, these aliases are available:

| Alias | Command | Description |
|-------|---------|-------------|
| `pp` | `prompt-pulse` | Run single collection |
| `pp-tui` | `prompt-pulse --tui` | Launch TUI dashboard |
| `pp-status` | Combined starship output | Quick status check |
| `pp-banner` | `prompt-pulse --banner` | Display status banner |
| `pp-daemon-start` | Start daemon | Background polling |
| `pp-daemon-stop` | Stop daemon | Kill background process |

### Shell Integration

Generate shell integration scripts:

```bash
# Bash integration (add to ~/.bashrc)
eval "$(prompt-pulse shell bash)"

# Zsh integration (add to ~/.zshrc)
eval "$(prompt-pulse shell zsh)"

# Fish integration (add to ~/.config/fish/config.fish)
prompt-pulse shell fish | source
```

Shell integration provides:
- Ctrl+P keybinding to launch TUI
- Convenience functions for daemon management
- Shell-specific completions (where applicable)

### Starship Integration

Add custom modules to your `~/.config/starship.toml`:

```toml
# Add to your format string
format = """
$username$hostname$directory${custom.pp_claude}${custom.pp_billing}${custom.pp_infra}$git_branch$character
"""

# Claude usage module
[custom.pp_claude]
command = "prompt-pulse --starship claude"
when = "command -v prompt-pulse"
format = "[$symbol($output)]($style) "
symbol = ""
style = "purple"
shell = ["bash", "--noprofile", "--norc"]

# Billing module
[custom.pp_billing]
command = "prompt-pulse --starship billing"
when = "command -v prompt-pulse"
format = "[$symbol($output)]($style) "
symbol = "$"
style = "green"
shell = ["bash", "--noprofile", "--norc"]

# Infrastructure module
[custom.pp_infra]
command = "prompt-pulse --starship infra"
when = "command -v prompt-pulse"
format = "[$symbol($output)]($style) "
symbol = ""
style = "cyan"
shell = ["bash", "--noprofile", "--norc"]
```

### Daemon Mode

The background daemon periodically collects data from all configured sources:

```bash
# Start daemon in background
prompt-pulse --daemon &

# Or use the shell alias
pp-daemon-start

# Stop the daemon
pp-daemon-stop
```

**Linux (systemd)**: If using home-manager with `daemon.enable = true`, the daemon runs as a systemd user service:

```bash
systemctl --user status prompt-pulse
systemctl --user restart prompt-pulse
journalctl --user -u prompt-pulse -f
```

**macOS (launchd)**: The daemon runs as a launchd user agent:

```bash
launchctl list | grep prompt-pulse
launchctl stop dev.tinyland.prompt-pulse
launchctl start dev.tinyland.prompt-pulse
```

## Architecture

prompt-pulse follows a collector/cache/display architecture:

```
+-------------------+     +-------------------+     +-------------------+
|    Collectors     |     |      Cache        |     |     Display       |
+-------------------+     +-------------------+     +-------------------+
| Claude Collector  |---->|                   |---->| Starship Output   |
| Billing Collector |---->|   File-based      |---->| Interactive TUI   |
| Infra Collector   |---->|   JSON Cache      |---->| Status Banner     |
+-------------------+     +-------------------+     +-------------------+
        ^                         |
        |                         v
+-------------------+     +-------------------+
|      Daemon       |     |   Status Eval     |
+-------------------+     +-------------------+
| Periodic polling  |     | Health scoring    |
| PID file mgmt     |     | Waifu selection   |
+-------------------+     +-------------------+
```

### Collectors

Each collector implements a common interface and runs concurrently:

| Collector | Sources | Data Collected |
|-----------|---------|----------------|
| Claude | OAuth credentials, API keys | Usage %, reset time, rate limits |
| Billing | Cloud provider APIs | Current spend, projections |
| Infra | Kubernetes API, Tailscale | Pod counts, node health, mesh status |

### Cache Layer

- File-based JSON cache in `~/.cache/prompt-pulse/`
- TTL-based freshness checking
- Stale data marked with `?` suffix in Starship output
- Cache populated by daemon; read by display commands

### Display Modes

| Mode | Command | Description |
|------|---------|-------------|
| Starship | `--starship <module>` | One-line output for prompt integration |
| TUI | `--tui` | Interactive dashboard with tabs |
| Banner | `--banner` | Full-width status display with optional images |

## Terminal Requirements

### Waifu Image Support

For waifu banner images, prompt-pulse uses the Kitty Graphics Protocol:

| Terminal | Support | Notes |
|----------|---------|-------|
| Ghostty | Full | Native Kitty protocol support |
| Kitty | Full | Original protocol implementation |
| WezTerm | Full | Kitty protocol compatible |
| iTerm2 | Partial | Limited image support |
| Others | Fallback | Unicode half-block art rendering |

Terminal detection is automatic based on `TERM_PROGRAM` and `TERM` environment variables.

### OSC 8 Hyperlinks

For clickable dashboard URLs, your terminal must support OSC 8 escape sequences:

| Terminal | OSC 8 Support |
|----------|---------------|
| Ghostty | Yes |
| Kitty | Yes |
| WezTerm | Yes |
| iTerm2 | Yes |
| GNOME Terminal | Yes (3.26+) |
| Windows Terminal | Yes |
| tmux | Partial (passthrough) |

### Unicode Requirements

The TUI and status displays use Unicode box-drawing and block characters. Ensure your terminal uses a font with good Unicode coverage (e.g., JetBrains Mono, Fira Code, Nerd Fonts).

## Troubleshooting

### Common Issues

**Cache directory not found**:
```bash
mkdir -p ~/.cache/prompt-pulse
mkdir -p ~/.local/log
```

**Daemon already running**:
```bash
# Check for existing process
cat ~/.cache/prompt-pulse/prompt-pulse.pid
kill $(cat ~/.cache/prompt-pulse/prompt-pulse.pid)
```

**Stale data (? suffix)**:
The daemon may not be running or the poll interval has not elapsed. Start or restart the daemon:
```bash
pp-daemon-start
```

**API key not found**:
Ensure environment variables are set in your shell RC file:
```bash
export CIVO_API_KEY="your-key"
export DIGITALOCEAN_TOKEN="your-token"
```

**Claude credentials error**:
For subscription accounts, verify the credentials file exists and is valid JSON:
```bash
cat ~/.claude/.credentials.json | jq .
```

### Debug Logging

Enable verbose logging to see detailed collector activity:
```bash
prompt-pulse --verbose --daemon
tail -f ~/.local/log/prompt-pulse.log
```

### Cache Inspection

View cached data directly:
```bash
cat ~/.cache/prompt-pulse/claude.json | jq .
cat ~/.cache/prompt-pulse/billing.json | jq .
cat ~/.cache/prompt-pulse/infra.json | jq .
```

### Service Management

**Linux (systemd)**:
```bash
# Check service status
systemctl --user status prompt-pulse

# View logs
journalctl --user -u prompt-pulse -f

# Restart service
systemctl --user restart prompt-pulse
```

**macOS (launchd)**:
```bash
# Check if loaded
launchctl list | grep prompt-pulse

# View logs
tail -f ~/.local/state/log/prompt-pulse.log

# Restart
launchctl stop dev.tinyland.prompt-pulse
launchctl start dev.tinyland.prompt-pulse
```

## License

MIT License. See LICENSE file for details.

## Related Projects

- [Starship](https://starship.rs/) - Cross-shell prompt
- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [waifu.pics](https://waifu.pics/) - Waifu image API
