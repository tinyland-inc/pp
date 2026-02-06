// Package color provides centralized color profile detection for prompt-pulse.
//
// It implements the NO_COLOR specification (https://no-color.org/) and
// automatic pipe/redirect detection. When color is disabled, lipgloss is
// set to the Ascii profile so all styled renders produce plain text.
package color

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

// ShouldDisableColor returns true if color output should be suppressed.
// This happens when:
//   - The NO_COLOR environment variable is set (any value, per https://no-color.org/)
//   - stdout is not a terminal (pipe or redirect)
func ShouldDisableColor() bool {
	// NO_COLOR spec: if the variable exists, disable color.
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return true
	}

	// Pipe/redirect detection: if stdout is not a TTY, disable color.
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return true
	}

	return false
}

// Apply configures the global lipgloss renderer based on ShouldDisableColor.
// When color is disabled, lipgloss.SetColorProfile(termenv.Ascii) is called
// so that all lipgloss.Style.Render() calls produce plain text without ANSI
// escape sequences.
// Returns true if color is enabled, false if disabled.
func Apply() bool {
	if ShouldDisableColor() {
		lipgloss.SetColorProfile(termenv.Ascii)
		return false
	}
	return true
}

// ForceDisable sets the lipgloss color profile to Ascii, unconditionally
// disabling all color output. This is useful for tests.
func ForceDisable() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

// StripANSI removes all ANSI escape sequences from a string.
// This is a safety net for any output that bypasses lipgloss styling.
func StripANSI(s string) string {
	var result []byte
	inEscape := false
	for i := 0; i < len(s); i++ {
		if inEscape {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') || s[i] == '~' {
				inEscape = false
			}
			continue
		}
		if s[i] == '\x1b' {
			inEscape = true
			continue
		}
		result = append(result, s[i])
	}
	return string(result)
}
