package render

import (
	"os"
	"testing"
)

// withProtocolEnv sets an environment variable for the duration of a test.
func withProtocolEnv(t *testing.T, key, value string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

// clearProtocolEnv unsets an environment variable for the duration of a test.
func clearProtocolEnv(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		}
	})
}

// clearAllProtocolTermVars clears all terminal-related environment variables.
func clearAllProtocolTermVars(t *testing.T) {
	t.Helper()
	vars := []string{
		"TERM_PROGRAM", "TERM", "KITTY_WINDOW_ID",
		"ITERM_SESSION_ID", "LC_TERMINAL", "MLTERM",
		"XTERM_VERSION", "WEZTERM_EXECUTABLE",
		"SSH_CLIENT", "SSH_CONNECTION", "SSH_TTY", "TMUX",
	}
	for _, v := range vars {
		clearProtocolEnv(t, v)
	}
}

func TestDetectProtocol_Ghostty(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "TERM_PROGRAM", "ghostty")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %v, want ProtocolKitty", got)
	}
}

func TestDetectProtocol_Kitty(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "TERM_PROGRAM", "kitty")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %v, want ProtocolKitty", got)
	}
}

func TestDetectProtocol_WezTerm(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "TERM_PROGRAM", "WezTerm")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %v, want ProtocolKitty", got)
	}
}

func TestDetectProtocol_XtermKitty(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "TERM", "xterm-kitty")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %v, want ProtocolKitty", got)
	}
}

func TestDetectProtocol_KittyWindowID(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "KITTY_WINDOW_ID", "42")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %v, want ProtocolKitty", got)
	}
}

func TestDetectProtocol_ITerm2TermProgram(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "TERM_PROGRAM", "iTerm.app")

	if got := DetectProtocol(); got != ProtocolITerm2 {
		t.Errorf("DetectProtocol() = %v, want ProtocolITerm2", got)
	}
}

func TestDetectProtocol_ITerm2SessionID(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "ITERM_SESSION_ID", "some-session-id")

	if got := DetectProtocol(); got != ProtocolITerm2 {
		t.Errorf("DetectProtocol() = %v, want ProtocolITerm2", got)
	}
}

func TestDetectProtocol_ITerm2LCTerminal(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "LC_TERMINAL", "iTerm2")

	if got := DetectProtocol(); got != ProtocolITerm2 {
		t.Errorf("DetectProtocol() = %v, want ProtocolITerm2", got)
	}
}

func TestDetectProtocol_MLTERM(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "MLTERM", "3.9.0")

	if got := DetectProtocol(); got != ProtocolSixel {
		t.Errorf("DetectProtocol() = %v, want ProtocolSixel", got)
	}
}

func TestDetectProtocol_WezTermExecutable(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "WEZTERM_EXECUTABLE", "/usr/local/bin/wezterm")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %v, want ProtocolKitty", got)
	}
}

func TestDetectProtocol_Unknown(t *testing.T) {
	clearAllProtocolTermVars(t)

	got := DetectProtocol()
	// Should be either Chafa (if installed) or Unicode
	if got != ProtocolChafa && got != ProtocolUnicode {
		t.Errorf("DetectProtocol() = %v, want ProtocolChafa or ProtocolUnicode", got)
	}
}

func TestImageProtocol_String(t *testing.T) {
	tests := []struct {
		protocol ImageProtocol
		want     string
	}{
		{ProtocolKitty, "kitty"},
		{ProtocolSixel, "sixel"},
		{ProtocolITerm2, "iterm2"},
		{ProtocolUnicode, "unicode"},
		{ProtocolChafa, "chafa"},
		{ProtocolNone, "none"},
		{ImageProtocol(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.protocol.String(); got != tt.want {
				t.Errorf("ImageProtocol.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsSSHSession(t *testing.T) {
	clearAllProtocolTermVars(t)

	if IsSSHSession() {
		t.Error("IsSSHSession() = true without SSH vars, want false")
	}

	withProtocolEnv(t, "SSH_CLIENT", "192.168.1.1 12345 22")
	if !IsSSHSession() {
		t.Error("IsSSHSession() = false with SSH_CLIENT, want true")
	}
}

func TestIsTmuxSession(t *testing.T) {
	clearAllProtocolTermVars(t)

	if IsTmuxSession() {
		t.Error("IsTmuxSession() = true without TMUX, want false")
	}

	withProtocolEnv(t, "TMUX", "/tmp/tmux-1000/default,12345,0")
	if !IsTmuxSession() {
		t.Error("IsTmuxSession() = false with TMUX, want true")
	}
}

func TestDetectProtocolWithContext_SSHDegrades(t *testing.T) {
	clearAllProtocolTermVars(t)
	withProtocolEnv(t, "TERM_PROGRAM", "ghostty")
	withProtocolEnv(t, "SSH_CLIENT", "192.168.1.1 12345 22")

	got := DetectProtocolWithContext()
	// In SSH, Kitty protocol should degrade to chafa or unicode
	if got == ProtocolKitty {
		t.Errorf("DetectProtocolWithContext() = ProtocolKitty in SSH session, should degrade")
	}
}

func TestGetCapabilities(t *testing.T) {
	tests := []struct {
		protocol             ImageProtocol
		wantTransparency     bool
		wantAnimation        bool
		wantRequiresTempFile bool
		wantNonZeroColors    bool
	}{
		{ProtocolKitty, true, true, false, true},
		{ProtocolITerm2, true, true, false, true},
		{ProtocolSixel, false, false, false, true},
		{ProtocolUnicode, false, false, false, true},
		{ProtocolChafa, true, false, true, true},
		{ProtocolNone, false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.protocol.String(), func(t *testing.T) {
			caps := GetCapabilities(tt.protocol)

			if caps.SupportsTransparency != tt.wantTransparency {
				t.Errorf("SupportsTransparency = %v, want %v", caps.SupportsTransparency, tt.wantTransparency)
			}
			if caps.SupportsAnimation != tt.wantAnimation {
				t.Errorf("SupportsAnimation = %v, want %v", caps.SupportsAnimation, tt.wantAnimation)
			}
			if caps.RequiresTempFile != tt.wantRequiresTempFile {
				t.Errorf("RequiresTempFile = %v, want %v", caps.RequiresTempFile, tt.wantRequiresTempFile)
			}
			if (caps.MaxColors > 0) != tt.wantNonZeroColors {
				t.Errorf("MaxColors = %d, wantNonZero = %v", caps.MaxColors, tt.wantNonZeroColors)
			}
			if caps.Name == "" && tt.protocol != ProtocolNone {
				t.Error("Name should not be empty for supported protocols")
			}
		})
	}
}

func TestFormatDiagnostics(t *testing.T) {
	result := FormatDiagnostics()

	if result == "" {
		t.Error("FormatDiagnostics() should return non-empty string")
	}

	// Check for expected sections
	expectedSubstrings := []string{
		"Render Diagnostics:",
		"Detected Protocol:",
		"SSH Session:",
		"Chafa Available:",
	}

	for _, expected := range expectedSubstrings {
		if !contains(result, expected) {
			t.Errorf("FormatDiagnostics() missing expected substring: %q", expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
