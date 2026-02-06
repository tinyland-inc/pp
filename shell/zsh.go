package shell

import "fmt"

// GenerateZshIntegration returns a Zsh script snippet that provides
// prompt-pulse shell integration. Source the output in ~/.zshrc.
func GenerateZshIntegration(cfg IntegrationConfig) string {
	return fmt.Sprintf(`# prompt-pulse shell integration for Zsh
# Source this in your ~/.zshrc

# Launch prompt-pulse TUI with Ctrl+P
_prompt_pulse_tui() {
    BUFFER=""
    zle reset-prompt
    %[1]s --tui
    zle reset-prompt
}
zle -N _prompt_pulse_tui
bindkey '^P' _prompt_pulse_tui

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

# Zsh completion for prompt-pulse
_prompt_pulse_completion() {
    local -a commands
    commands=(
        '--banner:Display system status banner'
        '--tui:Launch interactive TUI'
        '--daemon:Run background daemon'
        '--starship:Output Starship format'
        '--health:Check daemon health status'
        '--keys:Show all keybindings'
        '--config:Config file path'
        '--version:Show version'
        '--verbose:Verbose logging'
    )
    _describe 'prompt-pulse' commands
}
compdef _prompt_pulse_completion prompt-pulse
`, cfg.BinaryPath)
}
