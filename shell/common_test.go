package shell

import (
	"strings"
	"testing"
)

func TestShellType_String(t *testing.T) {
	tests := []struct {
		shell ShellType
		want  string
	}{
		{Bash, "bash"},
		{Zsh, "zsh"},
		{Fish, "fish"},
		{Nushell, "nushell"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.shell.String()
			if got != tt.want {
				t.Errorf("ShellType(%d).String() = %q, want %q", tt.shell, got, tt.want)
			}
		})
	}
}

func TestDefaultIntegrationConfig(t *testing.T) {
	cfg := DefaultIntegrationConfig()

	if cfg.BinaryPath != "prompt-pulse" {
		t.Errorf("BinaryPath = %q, want %q", cfg.BinaryPath, "prompt-pulse")
	}
	if cfg.ConfigPath != "~/.config/prompt-pulse/config.yaml" {
		t.Errorf("ConfigPath = %q, want %q", cfg.ConfigPath, "~/.config/prompt-pulse/config.yaml")
	}
	if cfg.TUIKeybinding != `\C-p` {
		t.Errorf("TUIKeybinding = %q, want %q", cfg.TUIKeybinding, `\C-p`)
	}
}

func TestGenerateIntegration_Bash(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateIntegration(Bash, cfg)

	if !strings.Contains(output, "bind -x") {
		t.Error("Bash dispatch should contain bind -x keybinding")
	}
	if !strings.Contains(output, "pp-status") {
		t.Error("Bash dispatch should contain pp-status function")
	}
}

func TestGenerateIntegration_Zsh(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateIntegration(Zsh, cfg)

	if !strings.Contains(output, "bindkey") {
		t.Error("Zsh dispatch should contain bindkey")
	}
	if !strings.Contains(output, "compdef") {
		t.Error("Zsh dispatch should contain compdef completion")
	}
}

func TestGenerateIntegration_Fish(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateIntegration(Fish, cfg)

	if !strings.Contains(output, `bind \cp`) {
		t.Error("Fish dispatch should contain bind \\cp keybinding")
	}
	if !strings.Contains(output, "function pp-status") {
		t.Error("Fish dispatch should contain pp-status function")
	}
}

func TestGenerateIntegration_Nushell(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateIntegration(Nushell, cfg)

	if !strings.Contains(output, "def pp-status") {
		t.Error("Nushell dispatch should contain pp-status command")
	}
	if !strings.Contains(output, `extern "prompt-pulse"`) {
		t.Error("Nushell dispatch should contain extern completion definition")
	}
}

func TestGenerateIntegration_Unknown(t *testing.T) {
	cfg := DefaultIntegrationConfig()
	output := GenerateIntegration(ShellType(99), cfg)

	if !strings.Contains(output, "not yet implemented") {
		t.Errorf("unknown shell dispatch should return not-yet-implemented placeholder, got: %s", output)
	}
}
