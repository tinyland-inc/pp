package shell

import (
	"strings"
	"testing"
)

func TestGenerateNushellIntegration_ContainsKeybinding(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	out := GenerateNushellIntegration(cfg)

	if !strings.Contains(out, "prompt_pulse_tui") {
		t.Error("expected Nushell output to contain keybinding name prompt_pulse_tui")
	}
	if !strings.Contains(out, "modifier: control") {
		t.Error("expected Nushell output to contain modifier: control")
	}
	if !strings.Contains(out, "keycode: char_p") {
		t.Error("expected Nushell output to contain keycode: char_p")
	}
	if !strings.Contains(out, "executehostcommand") {
		t.Error("expected Nushell output to contain executehostcommand event")
	}
}

func TestGenerateNushellIntegration_ContainsCommands(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	out := GenerateNushellIntegration(cfg)

	commands := []string{
		"def pp-status",
		"def pp-tui",
		"def pp-daemon-start",
		"def pp-daemon-stop",
	}
	for _, cmd := range commands {
		if !strings.Contains(out, cmd) {
			t.Errorf("expected Nushell output to contain %q", cmd)
		}
	}
}

func TestGenerateNushellIntegration_ContainsCompletions(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	out := GenerateNushellIntegration(cfg)

	if !strings.Contains(out, `extern "prompt-pulse"`) {
		t.Error("expected Nushell output to contain extern definition for prompt-pulse")
	}
	if !strings.Contains(out, "nu-complete prompt-pulse starship") {
		t.Error("expected Nushell output to contain nu-complete function for starship")
	}

	completionValues := []string{"claude", "billing", "infra"}
	for _, val := range completionValues {
		if !strings.Contains(out, `"`+val+`"`) {
			t.Errorf("expected Nushell completions to include %q", val)
		}
	}
}

func TestGenerateNushellIntegration_UsesBinaryPath(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	cfg.BinaryPath = "/opt/bin/my-prompt-pulse"
	out := GenerateNushellIntegration(cfg)

	if !strings.Contains(out, "/opt/bin/my-prompt-pulse --tui") {
		t.Error("expected Nushell output to use custom binary path for TUI invocation")
	}
	if !strings.Contains(out, "/opt/bin/my-prompt-pulse --starship claude") {
		t.Error("expected Nushell output to use custom binary path for starship invocation")
	}
	if !strings.Contains(out, `extern "/opt/bin/my-prompt-pulse"`) {
		t.Error("expected Nushell extern to use custom binary path")
	}
}

func TestGenerateNushellIntegration_Header(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	out := GenerateNushellIntegration(cfg)

	if !strings.HasPrefix(out, "# prompt-pulse shell integration for Nushell") {
		t.Error("expected Nushell output to start with Nushell-specific header comment")
	}
}

func TestGenerateNushellIntegration_KeybindingIsComment(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	out := GenerateNushellIntegration(cfg)

	// Every line in the keybinding block should be a comment (starts with #)
	// Find the keybinding section and verify all its lines are comments
	lines := strings.Split(out, "\n")
	inKeybindingBlock := false
	foundKeybindingBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inKeybindingBlock {
				// Empty line ends the keybinding block
				break
			}
			continue
		}

		if strings.Contains(line, "Add the following block") {
			inKeybindingBlock = true
			foundKeybindingBlock = true
		}

		if inKeybindingBlock && trimmed != "" {
			if !strings.HasPrefix(trimmed, "#") {
				t.Errorf("expected keybinding line to be a comment, got: %q", line)
			}
		}
	}

	if !foundKeybindingBlock {
		t.Error("expected Nushell output to contain keybinding instruction block")
	}
}
