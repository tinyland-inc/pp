package shell

import "fmt"

// GenerateNushellIntegration returns a Nushell script snippet that provides
// prompt-pulse commands and completions. Keybinding configuration is emitted
// as comments because Nushell keybindings must be defined statically in the
// user's config.nu and cannot be added dynamically via source.
func GenerateNushellIntegration(cfg IntegrationConfig) string {
	return fmt.Sprintf(`# prompt-pulse shell integration for Nushell

# Keybinding: Add the following block to $env.config.keybindings in your config.nu:
# {
#     name: prompt_pulse_tui
#     modifier: control
#     keycode: char_p
#     mode: [emacs vi_normal vi_insert]
#     event: {
#         send: executehostcommand
#         cmd: "%[1]s --tui"
#     }
# }

# Show prompt-pulse status
def pp-status [] {
    %[1]s --starship claude
    %[1]s --starship billing
    %[1]s --starship infra
}

# Launch prompt-pulse TUI
def pp-tui [] {
    %[1]s --tui
}

# Start prompt-pulse daemon
def pp-daemon-start [] {
    %[1]s --daemon &
    print "prompt-pulse daemon started"
}

# Stop prompt-pulse daemon
def pp-daemon-stop [] {
    ps | where name =~ "%[1]s" | each { |it| kill $it.pid }
}

# Check daemon health
def pp-health [] {
    %[1]s --health
}

# Show all keybindings
def pp-keys [...args] {
    %[1]s --keys ...$args
}

# Force immediate data refresh
def pp-refresh [] {
    %[1]s
}

# Completions
def "nu-complete prompt-pulse starship" [] {
    ["claude" "billing" "infra"]
}

def "nu-complete prompt-pulse mode" [] {
    ["tui" "shell" "starship"]
}

def "nu-complete prompt-pulse format" [] {
    ["table" "json"]
}

extern "%[1]s" [
    --tui                                    # Launch interactive TUI
    --daemon                                 # Run background daemon
    --banner                                 # Display system status banner
    --starship: string@"nu-complete prompt-pulse starship"  # Output Starship format
    --health                                 # Check daemon health status
    --keys                                   # Show all keybindings
    --mode: string@"nu-complete prompt-pulse mode"  # Filter keybindings by mode
    --format: string@"nu-complete prompt-pulse format"  # Output format for --keys
    --config: path                           # Config file path
    --version                                # Show version
    --verbose                                # Verbose logging
]
`, cfg.BinaryPath)
}
