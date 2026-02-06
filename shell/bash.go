package shell

import "fmt"

// GenerateBashIntegration returns a Bash script snippet that provides
// prompt-pulse shell integration. Source the output in ~/.bashrc.
func GenerateBashIntegration(cfg IntegrationConfig) string {
	return fmt.Sprintf(`# prompt-pulse shell integration for Bash
# Source this in your ~/.bashrc or ~/.bash_profile

# Launch prompt-pulse TUI with Ctrl+P
_prompt_pulse_tui() {
    %[1]s --tui
}
bind -x '"%[2]s": _prompt_pulse_tui'

# Quick status check
pp-status() {
    %[1]s --starship claude
    %[1]s --starship billing
    %[1]s --starship infra
}

# Launch TUI
pp-tui() {
    %[1]s --tui
}

# Start daemon in background
pp-daemon-start() {
    %[1]s --daemon &
    echo "prompt-pulse daemon started (PID: $!)"
}

# Stop daemon
pp-daemon-stop() {
    pkill -f "%[1]s --daemon"
}

# Display system status banner with session-aware waifu
# Each shell session gets its own unique waifu image via PPULSE_SESSION_ID
pp-banner() {
    export PPULSE_SESSION_ID="${PPULSE_SESSION_ID:-$$-$(date +%%s)}"
    %[1]s --banner --session-id "$PPULSE_SESSION_ID"
}

# Check daemon health
pp-health() {
    %[1]s --health
}

# Show all keybindings
pp-keys() {
    %[1]s --keys "$@"
}

# Force immediate data refresh
pp-refresh() {
    %[1]s
}
`, cfg.BinaryPath, cfg.TUIKeybinding)
}
