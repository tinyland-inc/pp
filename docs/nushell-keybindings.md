# Nushell Keybindings for prompt-pulse

Nushell keybindings cannot be dynamically added via sourced scripts. They must be
defined statically in your `config.nu` file. This document explains how to set up
prompt-pulse keybindings manually.

## Quick Setup

Add the following to your `~/.config/nushell/config.nu`:

```nushell
# Add this to the $env.config.keybindings list
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

## Complete Integration

### Step 1: Generate Shell Functions

Run this command to get the prompt-pulse shell integration script:

```bash
prompt-pulse --shell nushell >> ~/.config/nushell/prompt-pulse.nu
```

### Step 2: Source in config.nu

Add to your `~/.config/nushell/config.nu`:

```nushell
source ~/.config/nushell/prompt-pulse.nu
```

### Step 3: Add Keybinding

In the same `config.nu`, find or create the `$env.config.keybindings` section and add:

```nushell
$env.config = ($env.config | upsert keybindings [
    # ... your existing keybindings ...
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
    {
        name: prompt_pulse_banner
        modifier: control_shift
        keycode: char_b
        mode: [emacs vi_normal vi_insert]
        event: {
            send: executehostcommand
            cmd: "prompt-pulse --banner"
        }
    }
])
```

## Available Commands

After sourcing the integration script, these commands are available:

| Command | Description |
|---------|-------------|
| `pp-status` | Show status for all collectors (claude, billing, infra) |
| `pp-tui` | Launch the interactive TUI |
| `pp-daemon-start` | Start the background daemon |
| `pp-daemon-stop` | Stop the background daemon |

## Banner with Session-Aware Waifu

Unlike bash/zsh/fish, Nushell does not have a `$PPULSE_SESSION_ID` environment
variable set automatically. To use session-aware waifu images, you can manually
pass a session ID:

```nushell
def pp-banner [] {
    let session_id = $"($nu.pid)-(date now | format date '%s')"
    prompt-pulse --banner --session-id $session_id
}
```

Or add this to your prompt-pulse.nu for automatic session management.

## Keybinding Reference

| Key Combo | Action |
|-----------|--------|
| Ctrl+P | Launch TUI (recommended) |
| Ctrl+Shift+B | Show banner (optional) |

## Troubleshooting

### Keybinding Not Working

1. Ensure the keybinding is in the correct format in your config.nu
2. Restart nushell (`exec nu`) to reload configuration
3. Check for keybinding conflicts with `keybindings list`

### Commands Not Found

1. Verify the integration script is sourced: `source ~/.config/nushell/prompt-pulse.nu`
2. Check that prompt-pulse is in your PATH: `which prompt-pulse`
3. Ensure the integration file exists: `ls ~/.config/nushell/prompt-pulse.nu`

## Why Manual Setup?

Nushell's security model prevents dynamically adding keybindings via sourced
scripts. This is a deliberate design decision that ensures:

1. Users explicitly opt-in to keybindings
2. No script can unexpectedly modify input handling
3. Configuration is auditable and version-controllable

This differs from bash/zsh/fish where `bind`/`bindkey` can be called at runtime.
