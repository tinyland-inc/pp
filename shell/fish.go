package shell

import "fmt"

// GenerateFishIntegration returns a Fish shell script snippet that provides
// prompt-pulse keybindings, helper functions, and tab completions.
func GenerateFishIntegration(cfg IntegrationConfig) string {
	return fmt.Sprintf(`# prompt-pulse shell integration for Fish

# Launch prompt-pulse TUI with %[2]s
function _prompt_pulse_tui
    commandline -f repaint
    %[1]s --tui
    commandline -f repaint
end
bind \cp _prompt_pulse_tui

# Show prompt-pulse status
function pp-status -d "Show prompt-pulse status"
    %[1]s --starship claude
    %[1]s --starship billing
    %[1]s --starship infra
end

# Launch prompt-pulse TUI
function pp-tui -d "Launch prompt-pulse TUI"
    %[1]s --tui
end

# Start prompt-pulse daemon
function pp-daemon-start -d "Start prompt-pulse daemon"
    %[1]s --daemon &
    echo "prompt-pulse daemon started (PID: $last_pid)"
end

# Stop prompt-pulse daemon
function pp-daemon-stop -d "Stop prompt-pulse daemon"
    pkill -f "%[1]s --daemon"
end

# Display system status banner with session-aware waifu
# Each shell session gets its own unique waifu image via PPULSE_SESSION_ID
function pp-banner -d "Display system status banner"
    if not set -q PPULSE_SESSION_ID
        set -gx PPULSE_SESSION_ID "$fish_pid-(date +%%s)"
    end
    %[1]s --banner --session-id "$PPULSE_SESSION_ID"
end

# Check daemon health
function pp-health -d "Check daemon health"
    %[1]s --health
end

# Show all keybindings
function pp-keys -d "Show all keybindings"
    %[1]s --keys $argv
end

# Force immediate data refresh
function pp-refresh -d "Force immediate data refresh"
    %[1]s
end

# Completions
complete -c %[1]s -l tui -d "Launch interactive TUI"
complete -c %[1]s -l daemon -d "Run background daemon"
complete -c %[1]s -l banner -d "Display system status banner"
complete -c %[1]s -l starship -d "Output Starship format" -xa "claude billing infra"
complete -c %[1]s -l health -d "Check daemon health status"
complete -c %[1]s -l keys -d "Show all keybindings"
complete -c %[1]s -l mode -d "Filter keybindings by mode" -xa "tui shell starship"
complete -c %[1]s -l format -d "Output format for --keys" -xa "table json"
complete -c %[1]s -l config -d "Config file path" -rF
complete -c %[1]s -l version -d "Show version"
complete -c %[1]s -l verbose -d "Verbose logging"
`, cfg.BinaryPath, cfg.TUIKeybinding)
}
