// Package render provides terminal image rendering protocol detection and rendering.
// It supports multiple protocols: Kitty Graphics Protocol (Ghostty, Kitty, WezTerm),
// iTerm2 inline images, Sixel graphics, chafa CLI auto-detection, and Unicode
// half-block fallback for universal terminal compatibility.
package render

import (
	"os"
	"os/exec"
	"strings"
)

// ImageProtocol identifies which image rendering protocol to use.
type ImageProtocol int

const (
	// ProtocolKitty uses the Kitty Graphics Protocol (supported by Ghostty, Kitty, WezTerm).
	ProtocolKitty ImageProtocol = iota
	// ProtocolSixel uses the Sixel graphics protocol (xterm, mlterm, some terminals).
	ProtocolSixel
	// ProtocolITerm2 uses iTerm2 native inline images protocol.
	ProtocolITerm2
	// ProtocolUnicode uses half-block unicode characters with ANSI 24-bit color.
	ProtocolUnicode
	// ProtocolChafa uses chafa CLI for auto-detected terminal graphics.
	ProtocolChafa
	// ProtocolNone indicates no image rendering support.
	ProtocolNone
)

// String returns the human-readable name of the protocol.
func (p ImageProtocol) String() string {
	switch p {
	case ProtocolKitty:
		return "kitty"
	case ProtocolSixel:
		return "sixel"
	case ProtocolITerm2:
		return "iterm2"
	case ProtocolUnicode:
		return "unicode"
	case ProtocolChafa:
		return "chafa"
	case ProtocolNone:
		return "none"
	default:
		return "unknown"
	}
}

// DetectProtocol inspects environment variables to determine which image
// rendering protocol the current terminal supports. It returns the best
// available protocol based on terminal capabilities.
//
// Detection priority:
//  1. TERM_PROGRAM for known terminal emulators
//  2. TERM=xterm-kitty for Kitty terminal
//  3. KITTY_WINDOW_ID environment variable
//  4. iTerm2 specific environment variables
//  5. MLTERM/XTERM_VERSION for potential Sixel support
//  6. chafa CLI availability as middle-ground option
//  7. Unicode half-blocks as universal fallback
func DetectProtocol() ImageProtocol {
	// Priority 1: Check TERM_PROGRAM for known terminal emulators
	termProgram := os.Getenv("TERM_PROGRAM")
	switch strings.ToLower(termProgram) {
	case "ghostty", "kitty", "wezterm":
		return ProtocolKitty
	case "iterm.app":
		return ProtocolITerm2
	case "apple_terminal":
		// macOS Terminal.app has limited graphics support
		return checkChafaOrUnicode()
	}

	// Priority 2: Check TERM variable for kitty-specific term
	if os.Getenv("TERM") == "xterm-kitty" {
		return ProtocolKitty
	}

	// Priority 3: Check KITTY_WINDOW_ID (set even if TERM_PROGRAM is overridden)
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return ProtocolKitty
	}

	// Priority 4: iTerm2 specific environment variables
	if os.Getenv("ITERM_SESSION_ID") != "" || os.Getenv("LC_TERMINAL") == "iTerm2" {
		return ProtocolITerm2
	}

	// Priority 5: Check for terminals that support Sixel
	if isSixelCapable() {
		return ProtocolSixel
	}

	// Priority 6: Check WEZTERM_EXECUTABLE (WezTerm without TERM_PROGRAM set)
	if os.Getenv("WEZTERM_EXECUTABLE") != "" {
		return ProtocolKitty
	}

	// Priority 7: chafa CLI as middle-ground (handles many protocols automatically)
	return checkChafaOrUnicode()
}

// isSixelCapable checks if the terminal likely supports Sixel graphics.
func isSixelCapable() bool {
	// MLTERM is a known Sixel-capable terminal
	if os.Getenv("MLTERM") != "" {
		return true
	}

	// xterm with specific configuration may support Sixel
	// We check XTERM_VERSION but this doesn't guarantee Sixel support
	// as it requires -ti 340 flag at startup
	xtermVersion := os.Getenv("XTERM_VERSION")
	if xtermVersion != "" && strings.HasPrefix(xtermVersion, "XTerm(") {
		// xterm may support sixel, but we can't be sure without querying
		// Fall through to chafa which will auto-detect
		return false
	}

	// VTE-based terminals (GNOME Terminal, Tilix, etc.) with sixel patches
	// VTE_VERSION is set but sixel support varies
	// Safe to fall through to chafa auto-detection

	return false
}

// checkChafaOrUnicode returns ProtocolChafa if chafa is available,
// otherwise falls back to ProtocolUnicode.
func checkChafaOrUnicode() ImageProtocol {
	if IsChafaAvailable() {
		return ProtocolChafa
	}
	return ProtocolUnicode
}

// IsChafaAvailable returns true if the chafa CLI tool is in PATH.
func IsChafaAvailable() bool {
	_, err := exec.LookPath("chafa")
	return err == nil
}

// IsSSHSession returns true if we're running inside an SSH session.
// SSH sessions often have limited terminal graphics support.
func IsSSHSession() bool {
	// SSH_CLIENT or SSH_CONNECTION are set in SSH sessions
	return os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_CONNECTION") != "" ||
		os.Getenv("SSH_TTY") != ""
}

// IsTmuxSession returns true if we're running inside tmux.
// tmux may passthrough some protocols but not all.
func IsTmuxSession() bool {
	return os.Getenv("TMUX") != ""
}

// DetectProtocolWithContext performs protocol detection with additional
// context awareness for SSH and tmux sessions. In degraded environments,
// it may prefer simpler protocols for reliability.
func DetectProtocolWithContext() ImageProtocol {
	protocol := DetectProtocol()

	// SSH sessions: prefer simpler protocols
	if IsSSHSession() {
		switch protocol {
		case ProtocolKitty, ProtocolITerm2, ProtocolSixel:
			// These may work over SSH but are unreliable
			// Prefer chafa (handles degraded modes) or unicode
			return checkChafaOrUnicode()
		}
	}

	// tmux: Kitty protocol passthrough requires tmux 3.4+ with allow-passthrough
	// For reliability, we still allow Kitty but could be made configurable
	if IsTmuxSession() && protocol == ProtocolKitty {
		// Check if chafa is available as it handles tmux better
		if IsChafaAvailable() {
			return ProtocolChafa
		}
	}

	return protocol
}

// ProtocolCapabilities describes what a protocol can do.
type ProtocolCapabilities struct {
	// SupportsTransparency indicates if the protocol can render transparent images.
	SupportsTransparency bool
	// SupportsAnimation indicates if the protocol can render animated images.
	SupportsAnimation bool
	// MaxColors is the maximum color depth (24-bit = 16777216, 256, etc.).
	MaxColors int
	// RequiresTempFile indicates if the protocol needs to write temp files.
	RequiresTempFile bool
	// Name is the human-readable protocol name.
	Name string
}

// GetCapabilities returns the capabilities of a given protocol.
func GetCapabilities(p ImageProtocol) ProtocolCapabilities {
	switch p {
	case ProtocolKitty:
		return ProtocolCapabilities{
			SupportsTransparency: true,
			SupportsAnimation:    true,
			MaxColors:            16777216, // 24-bit
			RequiresTempFile:     false,
			Name:                 "Kitty Graphics Protocol",
		}
	case ProtocolSixel:
		return ProtocolCapabilities{
			SupportsTransparency: false,
			SupportsAnimation:    false,
			MaxColors:            256, // Typically 256 colors
			RequiresTempFile:     false,
			Name:                 "Sixel Graphics",
		}
	case ProtocolITerm2:
		return ProtocolCapabilities{
			SupportsTransparency: true,
			SupportsAnimation:    true,
			MaxColors:            16777216, // 24-bit
			RequiresTempFile:     false,
			Name:                 "iTerm2 Inline Images",
		}
	case ProtocolUnicode:
		return ProtocolCapabilities{
			SupportsTransparency: false,
			SupportsAnimation:    false,
			MaxColors:            16777216, // ANSI 24-bit color
			RequiresTempFile:     false,
			Name:                 "Unicode Half-Blocks",
		}
	case ProtocolChafa:
		return ProtocolCapabilities{
			SupportsTransparency: true,  // Depends on output protocol
			SupportsAnimation:    false, // Static frames only
			MaxColors:            16777216,
			RequiresTempFile:     true, // Needs to write temp file for chafa
			Name:                 "Chafa CLI",
		}
	default:
		return ProtocolCapabilities{
			Name: "None",
		}
	}
}
