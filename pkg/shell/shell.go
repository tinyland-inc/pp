// Package shell generates shell integration scripts for prompt-pulse v2.
//
// Each supported shell gets a generator that produces a script snippet users
// eval in their shell RC file. The generated scripts install:
//
//   - A banner display on shell startup (calls prompt-pulse banner)
//   - A keybinding to launch the TUI dashboard (default: Ctrl+P)
//   - Lazy completion loading
//   - Daemon management functions (pp-start, pp-stop, pp-status)
//
// All private helpers are prefixed with "sh" to avoid naming conflicts with
// other packages in the prompt-pulse module.
package shell

import "fmt"

// ShellType identifies a supported shell for integration script generation.
type ShellType string

const (
	// Bash is the Bourne Again Shell.
	Bash ShellType = "bash"
	// Zsh is the Z Shell.
	Zsh ShellType = "zsh"
	// Fish is the Friendly Interactive Shell.
	Fish ShellType = "fish"
	// Ksh is the KornShell 93.
	Ksh ShellType = "ksh"
)

// Options controls how the generated shell integration behaves.
type Options struct {
	// BinaryPath is the path to the prompt-pulse binary.
	// If empty, defaults to "prompt-pulse" (assumes PATH lookup).
	BinaryPath string

	// Keybinding is the key combo to launch TUI.
	// Defaults vary by shell: "\C-p" for bash/zsh/ksh, "ctrl-p" for fish.
	Keybinding string

	// WaifuKeybinding is the key combo to launch TUI in expanded waifu mode.
	// Defaults vary by shell: "\C-w" for bash/zsh/ksh, "ctrl-w" for fish.
	// Set to empty string to disable.
	WaifuKeybinding string

	// ShowBanner displays the system status banner on shell start.
	ShowBanner bool

	// DaemonAutoStart auto-starts the daemon if not running on shell init.
	DaemonAutoStart bool

	// EnableCompletions installs tab completions for the prompt-pulse binary.
	EnableCompletions bool
}

// shDefaultOptions returns Options with sensible defaults filled in for the
// given shell type. Only zero-valued fields in the caller's Options are
// overwritten.
func shDefaultOptions(shell ShellType, opts Options) Options {
	if opts.BinaryPath == "" {
		opts.BinaryPath = "prompt-pulse"
	}
	if opts.Keybinding == "" {
		switch shell {
		case Fish:
			opts.Keybinding = "ctrl-p"
		default:
			opts.Keybinding = `\C-p`
		}
	}
	if opts.WaifuKeybinding == "" {
		switch shell {
		case Fish:
			opts.WaifuKeybinding = "ctrl-w"
		default:
			opts.WaifuKeybinding = `\C-w`
		}
	}
	return opts
}

// Generate returns the shell integration script for the given shell type.
// The returned string is meant to be eval'd or sourced in the user's shell
// RC file.
func Generate(shell ShellType, opts Options) string {
	opts = shDefaultOptions(shell, opts)

	switch shell {
	case Bash:
		return shGenerateBash(opts)
	case Zsh:
		return shGenerateZsh(opts)
	case Fish:
		return shGenerateFish(opts)
	case Ksh:
		return shGenerateKsh(opts)
	default:
		return fmt.Sprintf("# prompt-pulse: %s integration is not supported\n", shell)
	}
}

// shQuote wraps a string in single quotes for POSIX shells, escaping any
// embedded single quotes via the '\'' idiom.
func shQuote(s string) string {
	out := "'"
	for _, c := range s {
		if c == '\'' {
			out += `'\''`
		} else {
			out += string(c)
		}
	}
	out += "'"
	return out
}
