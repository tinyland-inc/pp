// Package shell generates shell integration scripts for prompt-pulse.
//
// Each supported shell gets a generator function that produces a script snippet
// users can source in their shell RC file (~/.bashrc, ~/.zshrc, etc.). The
// generated scripts provide:
//
//   - A keybinding (default Ctrl+P) to launch the TUI
//   - Convenience functions for status checks and daemon management
//   - Shell-specific completions where applicable
package shell

import "fmt"

// ShellType identifies a supported shell.
type ShellType int

const (
	// Bash is the Bourne Again Shell.
	Bash ShellType = iota
	// Zsh is the Z Shell.
	Zsh
	// Fish is the Friendly Interactive Shell.
	Fish
	// Nushell is the Nu shell.
	Nushell
)

// String returns the lowercase name of the shell type.
func (s ShellType) String() string {
	switch s {
	case Bash:
		return "bash"
	case Zsh:
		return "zsh"
	case Fish:
		return "fish"
	case Nushell:
		return "nushell"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// IntegrationConfig controls how the generated shell integration behaves.
type IntegrationConfig struct {
	// BinaryPath is the path to the prompt-pulse binary.
	BinaryPath string
	// ConfigPath is the path to the config file.
	ConfigPath string
	// TUIKeybinding is the key combo to launch TUI (default: "\\C-p" for ctrl+p).
	TUIKeybinding string
}

// DefaultIntegrationConfig returns an IntegrationConfig with sensible defaults.
// It assumes prompt-pulse is available on PATH and uses the standard XDG config
// location.
func DefaultIntegrationConfig() IntegrationConfig {
	return IntegrationConfig{
		BinaryPath:    "prompt-pulse",
		ConfigPath:    "~/.config/prompt-pulse/config.yaml",
		TUIKeybinding: `\C-p`,
	}
}

// GenerateIntegration dispatches to the appropriate shell-specific generator.
// For shells that do not yet have a full implementation, it returns a comment
// indicating the shell is not yet supported.
func GenerateIntegration(shell ShellType, cfg IntegrationConfig) string {
	switch shell {
	case Bash:
		return GenerateBashIntegration(cfg)
	case Zsh:
		return GenerateZshIntegration(cfg)
	case Fish:
		return GenerateFishIntegration(cfg)
	case Nushell:
		return GenerateNushellIntegration(cfg)
	default:
		return fmt.Sprintf("# prompt-pulse: %s integration is not yet implemented\n", shell)
	}
}
