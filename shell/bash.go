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

# Display system status banner
pp-banner() {
    %[1]s --banner
}
`, cfg.BinaryPath, cfg.TUIKeybinding)
}
