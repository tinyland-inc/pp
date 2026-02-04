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
#         cmd: "%s --tui"
#     }
# }

# Show prompt-pulse status
def pp-status [] {
    %s --starship claude
    %s --starship billing
    %s --starship infra
}

# Launch prompt-pulse TUI
def pp-tui [] {
    %s --tui
}

# Start prompt-pulse daemon
def pp-daemon-start [] {
    %s --daemon &
    print "prompt-pulse daemon started"
}

# Stop prompt-pulse daemon
def pp-daemon-stop [] {
    ps | where name =~ "%s" | each { |it| kill $it.pid }
}

# Completions
def "nu-complete prompt-pulse starship" [] {
    ["claude" "billing" "infra"]
}

extern "%s" [
    --tui                                    # Launch interactive TUI
    --daemon                                 # Run background daemon
    --starship: string@"nu-complete prompt-pulse starship"  # Output Starship format
    --config: path                           # Config file path
    --version                                # Show version
    --verbose                                # Verbose logging
]
`,
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
