package shell

import (
	"strings"
	"testing"
)

func TestGenerateFishIntegration_ContainsKeybinding(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	out := GenerateFishIntegration(cfg)

	if !strings.Contains(out, `bind \cp`) {
		t.Error("expected Fish output to contain bind \\cp keybinding")
	}
	if !strings.Contains(out, "_prompt_pulse_tui") {
		t.Error("expected Fish output to contain _prompt_pulse_tui function")
	}
}

func TestGenerateFishIntegration_ContainsFunctions(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	out := GenerateFishIntegration(cfg)

	functions := []string{
		"function pp-status",
		"function pp-tui",
		"function pp-daemon-start",
		"function pp-daemon-stop",
	}
	for _, fn := range functions {
		if !strings.Contains(out, fn) {
			t.Errorf("expected Fish output to contain %q", fn)
		}
	}
}

func TestGenerateFishIntegration_ContainsCompletions(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	out := GenerateFishIntegration(cfg)

	if !strings.Contains(out, "complete -c prompt-pulse") {
		t.Error("expected Fish output to contain completions for prompt-pulse")
	}

	// Fish uses -l for long option names (e.g., -l tui corresponds to --tui)
	flags := []string{"-l tui", "-l daemon", "-l starship", "-l config", "-l version", "-l verbose"}
	for _, flag := range flags {
		if !strings.Contains(out, flag) {
			t.Errorf("expected Fish completions to reference flag %q", flag)
		}
	}
}

func TestGenerateFishIntegration_UsesBinaryPath(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	cfg.BinaryPath = "/opt/bin/my-prompt-pulse"
	out := GenerateFishIntegration(cfg)

	if !strings.Contains(out, "/opt/bin/my-prompt-pulse --tui") {
		t.Error("expected Fish output to use custom binary path for TUI invocation")
	}
	if !strings.Contains(out, "/opt/bin/my-prompt-pulse --starship claude") {
		t.Error("expected Fish output to use custom binary path for starship invocation")
	}
	if !strings.Contains(out, "complete -c /opt/bin/my-prompt-pulse") {
		t.Error("expected Fish completions to use custom binary path")
	}
}

func TestGenerateFishIntegration_Header(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	out := GenerateFishIntegration(cfg)

	if !strings.HasPrefix(out, "# prompt-pulse shell integration for Fish") {
		t.Error("expected Fish output to start with Fish-specific header comment")
	}
}

func TestGenerateFishIntegration_FunctionDescriptions(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	out := GenerateFishIntegration(cfg)

	descriptions := []string{
		`-d "Show prompt-pulse status"`,
		`-d "Launch prompt-pulse TUI"`,
		`-d "Start prompt-pulse daemon"`,
		`-d "Stop prompt-pulse daemon"`,
	}
	for _, desc := range descriptions {
		if !strings.Contains(out, desc) {
			t.Errorf("expected Fish output to contain function description %q", desc)
		}
	}
}
