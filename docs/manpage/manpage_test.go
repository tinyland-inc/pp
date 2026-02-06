package manpage

import (
	"strings"
	"testing"
)

func TestGenerate_ValidRoff(t *testing.T) {
	page := Generate("0.1.0", "abc1234", "2026-02-06")

	// Must start with .TH header.
	if !strings.HasPrefix(page, ".TH PROMPT-PULSE 1") {
		t.Errorf("man page should start with .TH header, got: %s", page[:80])
	}

	// Must contain all required sections.
	requiredSections := []string{
		".SH NAME",
		".SH SYNOPSIS",
		".SH DESCRIPTION",
		".SH OPTIONS",
		".SH KEYBINDINGS",
		".SH CONFIGURATION",
		".SH SHELL INTEGRATION",
		".SH FILES",
		".SH EXAMPLES",
		".SH ENVIRONMENT",
		".SH EXIT STATUS",
		".SH SEE ALSO",
		".SH AUTHORS",
		".SH BUGS",
		".SH VERSION",
	}

	for _, section := range requiredSections {
		if !strings.Contains(page, section) {
			t.Errorf("man page missing required section: %s", section)
		}
	}
}

func TestGenerate_ContainsVersion(t *testing.T) {
	page := Generate("1.2.3", "deadbeef", "2026-02-06")

	if !strings.Contains(page, "1.2.3") {
		t.Error("man page should contain the version string")
	}
	if !strings.Contains(page, "deadbeef") {
		t.Error("man page should contain the commit hash")
	}
}

func TestGenerate_ContainsAllFlags(t *testing.T) {
	page := Generate("0.1.0", "dev", "unknown")

	expectedFlags := []string{
		"banner",
		"tui",
		"daemon",
		"starship",
		"shell",
		"health",
		"json",
		"keys",
		"mode",
		"format",
		"theme",
		"config",
		"waifu",
		"fastfetch",
		"session\\-id",
		"term\\-width",
		"term\\-height",
		"diagnose",
		"billing\\-check",
		"use\\-mocks",
		"mock\\-accounts",
		"mock\\-seed",
		"verbose",
		"version",
		"man",
	}

	for _, flag := range expectedFlags {
		if !strings.Contains(page, flag) {
			t.Errorf("man page missing flag: --%s", flag)
		}
	}
}

func TestGenerate_ContainsKeybindings(t *testing.T) {
	page := Generate("0.1.0", "dev", "unknown")

	// TUI keybindings from the KeyRegistry.
	expectedKeys := []string{
		"next tab",
		"prev tab",
		"scroll up",
		"scroll down",
		"quit",
		"help",
		"refresh",
	}

	for _, key := range expectedKeys {
		if !strings.Contains(page, key) {
			t.Errorf("man page missing keybinding description: %q", key)
		}
	}

	// Shell keybindings.
	if !strings.Contains(page, "launch TUI") {
		t.Error("man page missing shell Ctrl+P keybinding")
	}
}

func TestGenerate_ContainsModeGroups(t *testing.T) {
	page := Generate("0.1.0", "dev", "unknown")

	if !strings.Contains(page, "TUI Mode") {
		t.Error("man page missing TUI Mode section")
	}
	if !strings.Contains(page, "Shell Mode") {
		t.Error("man page missing Shell Mode section")
	}
}

func TestGenerate_ContainsShellTypes(t *testing.T) {
	page := Generate("0.1.0", "dev", "unknown")

	for _, shell := range []string{"Bash", "Zsh", "Fish", "Nushell"} {
		if !strings.Contains(page, shell) {
			t.Errorf("man page missing shell integration for: %s", shell)
		}
	}
}

func TestGenerate_ContainsFilePaths(t *testing.T) {
	page := Generate("0.1.0", "dev", "unknown")

	expectedPaths := []string{
		"config.yaml",
		"health.json",
		"prompt\\-pulse.pid",
		"prompt\\-pulse.log",
	}

	for _, path := range expectedPaths {
		if !strings.Contains(page, path) {
			t.Errorf("man page missing file path: %s", path)
		}
	}
}

func TestGenerate_ContainsEnvironmentVars(t *testing.T) {
	page := Generate("0.1.0", "dev", "unknown")

	expectedVars := []string{
		"PROMPT_PULSE_CONFIG",
		"CIVO_API_KEY",
		"DIGITALOCEAN_TOKEN",
		"DREAMHOST_API_KEY",
		"TAILSCALE_API_KEY",
	}

	for _, envVar := range expectedVars {
		if !strings.Contains(page, envVar) {
			t.Errorf("man page missing environment variable: %s", envVar)
		}
	}
}

func TestGenerate_NoEmptyOutput(t *testing.T) {
	page := Generate("0.1.0", "dev", "unknown")

	if len(page) < 1000 {
		t.Errorf("man page seems too short: %d bytes", len(page))
	}
}

func TestRoffEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"ctrl-p", `ctrl\-p`},
		{"e.g.", `e\&.g\&.`},
		{`foo\bar`, `foo\\bar`},
	}

	for _, tt := range tests {
		got := roffEscape(tt.input)
		if got != tt.expected {
			t.Errorf("roffEscape(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
