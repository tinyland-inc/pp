package shell

import (
	"strings"
	"testing"
)

func TestGenerateZshIntegration_ContainsKeybinding(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateZshIntegration(cfg)

	if !strings.Contains(output, "bindkey '^P'") {
		t.Error("output should contain bindkey '^P' for zsh keybinding")
	}
	if !strings.Contains(output, "zle -N") {
		t.Error("output should contain zle -N widget registration")
	}
}

func TestGenerateZshIntegration_ContainsFunctions(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateZshIntegration(cfg)

	functions := []string{"pp-status", "pp-tui", "pp-daemon-start", "pp-daemon-stop", "pp-banner"}
	for _, fn := range functions {
		if !strings.Contains(output, fn+"()") {
			t.Errorf("output should contain function %s()", fn)
		}
	}
}

func TestGenerateZshIntegration_ContainsBannerWithSessionID(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateZshIntegration(cfg)

	if !strings.Contains(output, "PPULSE_SESSION_ID") {
		t.Error("output should contain PPULSE_SESSION_ID for session-aware waifu")
	}
	if !strings.Contains(output, "--banner --session-id") {
		t.Error("output should pass session-id to banner command")
	}
}

func TestGenerateZshIntegration_ContainsCompletion(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateZshIntegration(cfg)

	if !strings.Contains(output, "compdef") {
		t.Error("output should contain compdef for zsh completion")
	}
	if !strings.Contains(output, "_describe") {
		t.Error("output should contain _describe for completion descriptions")
	}
}

func TestGenerateZshIntegration_UsesBinaryPath(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	cfg.BinaryPath = "/opt/bin/prompt-pulse"
	output := GenerateZshIntegration(cfg)

	if !strings.Contains(output, "/opt/bin/prompt-pulse --tui") {
		t.Error("output should use custom binary path for --tui")
	}
	if !strings.Contains(output, "/opt/bin/prompt-pulse --starship") {
		t.Error("output should use custom binary path for --starship")
	}
	if !strings.Contains(output, "/opt/bin/prompt-pulse --daemon") {
		t.Error("output should use custom binary path for --daemon")
	}
}

func TestGenerateZshIntegration_Header(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateZshIntegration(cfg)

	if !strings.HasPrefix(output, "# prompt-pulse shell integration for Zsh") {
		t.Error("output should start with Zsh header comment")
	}
}
