package shell

import "fmt"

// GenerateFishIntegration returns a Fish shell script snippet that provides
// prompt-pulse keybindings, helper functions, and tab completions.
func GenerateFishIntegration(cfg IntegrationConfig) string {
	return fmt.Sprintf(`# prompt-pulse shell integration for Fish

# Launch prompt-pulse TUI with %s
function _prompt_pulse_tui
    commandline -f repaint
    %s --tui
    commandline -f repaint
end
bind \cp _prompt_pulse_tui

# Show prompt-pulse status
function pp-status -d "Show prompt-pulse status"
    %s --starship claude
    %s --starship billing
    %s --starship infra
end

# Launch prompt-pulse TUI
function pp-tui -d "Launch prompt-pulse TUI"
    %s --tui
end

# Start prompt-pulse daemon
function pp-daemon-start -d "Start prompt-pulse daemon"
    %s --daemon &
    echo "prompt-pulse daemon started (PID: $last_pid)"
end

# Stop prompt-pulse daemon
function pp-daemon-stop -d "Stop prompt-pulse daemon"
    pkill -f "%s --daemon"
end

# Completions
complete -c %s -l tui -d "Launch interactive TUI"
complete -c %s -l daemon -d "Run background daemon"
complete -c %s -l starship -d "Output Starship format" -xa "claude billing infra"
complete -c %s -l config -d "Config file path" -rF
complete -c %s -l version -d "Show version"
complete -c %s -l verbose -d "Verbose logging"
`,
		cfg.TUIKeybinding,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
		cfg.BinaryPath,
	)
}
