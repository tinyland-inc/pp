// Package banner orchestrates the full banner generation pipeline.
// This file provides terminal size detection utilities.

package banner

import (
	"os"
	"strconv"

	"github.com/charmbracelet/x/term"
)

// DetectTerminalSize returns the current terminal dimensions.
// It attempts TTY detection first via the term package, then falls back
// to COLUMNS/LINES environment variables, and finally to 80x24 defaults.
func DetectTerminalSize() (width, height int) {
	// Try TTY detection first using stdout file descriptor
	w, h, err := term.GetSize(os.Stdout.Fd())
	if err == nil && w > 0 && h > 0 {
		return w, h
	}

	// Try environment variables
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if w, err := strconv.Atoi(cols); err == nil && w > 0 {
			width = w
		}
	}
	if lines := os.Getenv("LINES"); lines != "" {
		if h, err := strconv.Atoi(lines); err == nil && h > 0 {
			height = h
		}
	}

	// Defaults
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}
	return width, height
}
