package shell

import (
	"strings"
	"testing"
)

func TestGenerateBashIntegration_ContainsKeybinding(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateBashIntegration(cfg)

	if !strings.Contains(output, "bind -x") {
		t.Error("output should contain bind -x for bash keybinding")
	}
	if !strings.Contains(output, `\C-p`) {
		t.Error("output should contain \\C-p keybinding")
	}
}

func TestGenerateBashIntegration_ContainsFunctions(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateBashIntegration(cfg)

	functions := []string{"pp-status", "pp-tui", "pp-daemon-start", "pp-daemon-stop"}
	for _, fn := range functions {
		if !strings.Contains(output, fn+"()") {
			t.Errorf("output should contain function %s()", fn)
		}
	}
}

func TestGenerateBashIntegration_UsesBinaryPath(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	cfg.BinaryPath = "/usr/local/bin/prompt-pulse"
	output := GenerateBashIntegration(cfg)

	if !strings.Contains(output, "/usr/local/bin/prompt-pulse --tui") {
		t.Error("output should use custom binary path for --tui")
	}
	if !strings.Contains(output, "/usr/local/bin/prompt-pulse --starship") {
		t.Error("output should use custom binary path for --starship")
	}
	if !strings.Contains(output, "/usr/local/bin/prompt-pulse --daemon") {
		t.Error("output should use custom binary path for --daemon")
	}
}

func TestGenerateBashIntegration_Header(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateBashIntegration(cfg)

	if !strings.HasPrefix(output, "# prompt-pulse shell integration for Bash") {
		t.Error("output should start with Bash header comment")
	}
}

func TestGenerateBashIntegration_DefaultConfig(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateBashIntegration(cfg)

	if output == "" {
		t.Error("output should not be empty with default config")
	}
	if !strings.Contains(output, "prompt-pulse") {
		t.Error("output should reference prompt-pulse binary")
	}
}
