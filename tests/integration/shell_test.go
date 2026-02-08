// Package integration contains cross-shell integration tests for prompt-pulse.
// These tests validate that shell integration scripts contain all required
// components and work correctly across bash, zsh, and fish shells.
package integration

import (
	"strings"
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/shell"
)

// TestShellTemplates validates that all shell integration templates
// contain the required components for proper prompt-pulse integration.
func TestShellTemplates(t *testing.T) {
	shells := []struct {
		name      string
		shellType shell.ShellType
		generator func(shell.IntegrationConfig) string
	}{
		{"bash", shell.Bash, shell.GenerateBashIntegration},
		{"zsh", shell.Zsh, shell.GenerateZshIntegration},
		{"fish", shell.Fish, shell.GenerateFishIntegration},
	}

	cfg := shell.DefaultIntegrationConfig()

	for _, sh := range shells {
		t.Run(sh.name, func(t *testing.T) {
			template := sh.generator(cfg)

			// Verify pp-banner function exists
			if !strings.Contains(template, "pp-banner") {
				t.Error("Missing pp-banner function")
			}

			// Verify prompt-pulse binary call
			if !strings.Contains(template, "prompt-pulse") {
				t.Error("Missing prompt-pulse binary call")
			}

			// Verify session ID support
			if !strings.Contains(template, "PPULSE_SESSION_ID") {
				t.Error("Missing session ID support (PPULSE_SESSION_ID)")
			}

			// Verify core functions exist
			coreFunctions := []string{"pp-status", "pp-tui", "pp-daemon-start", "pp-daemon-stop"}
			for _, fn := range coreFunctions {
				if !strings.Contains(template, fn) {
					t.Errorf("Missing core function: %s", fn)
				}
			}
		})
	}
}

// TestBashIntegration validates Bash-specific integration features.
func TestBashIntegration(t *testing.T) {
	cfg := shell.DefaultIntegrationConfig()
	template := shell.GenerateBashIntegration(cfg)

	t.Run("keybinding", func(t *testing.T) {
		if !strings.Contains(template, "bind -x") {
			t.Error("Bash template should use bind -x for keybinding")
		}
	})

	t.Run("function_syntax", func(t *testing.T) {
		// Bash uses funcname() { ... } syntax
		if !strings.Contains(template, "pp-status()") {
			t.Error("Bash should use function() syntax for pp-status")
		}
	})

	t.Run("session_id_export", func(t *testing.T) {
		// Bash uses export for environment variables
		if !strings.Contains(template, "export PPULSE_SESSION_ID") {
			t.Error("Bash should export PPULSE_SESSION_ID")
		}
	})

	t.Run("banner_command", func(t *testing.T) {
		if !strings.Contains(template, "--banner --session-id") {
			t.Error("Bash pp-banner should pass session ID to banner command")
		}
	})
}

// TestZshIntegration validates Zsh-specific integration features.
func TestZshIntegration(t *testing.T) {
	cfg := shell.DefaultIntegrationConfig()
	template := shell.GenerateZshIntegration(cfg)

	t.Run("keybinding", func(t *testing.T) {
		if !strings.Contains(template, "bindkey") {
			t.Error("Zsh template should use bindkey for keybinding")
		}
		if !strings.Contains(template, "zle -N") {
			t.Error("Zsh template should register widget with zle -N")
		}
	})

	t.Run("completion", func(t *testing.T) {
		if !strings.Contains(template, "compdef") {
			t.Error("Zsh template should include compdef for completion")
		}
		if !strings.Contains(template, "_describe") {
			t.Error("Zsh template should use _describe for completion descriptions")
		}
	})

	t.Run("function_syntax", func(t *testing.T) {
		// Zsh also uses funcname() { ... } syntax
		if !strings.Contains(template, "pp-status()") {
			t.Error("Zsh should use function() syntax for pp-status")
		}
	})

	t.Run("session_id_export", func(t *testing.T) {
		if !strings.Contains(template, "export PPULSE_SESSION_ID") {
			t.Error("Zsh should export PPULSE_SESSION_ID")
		}
	})

	t.Run("banner_command", func(t *testing.T) {
		if !strings.Contains(template, "--banner --session-id") {
			t.Error("Zsh pp-banner should pass session ID to banner command")
		}
	})
}

// TestFishIntegration validates Fish-specific integration features.
func TestFishIntegration(t *testing.T) {
	cfg := shell.DefaultIntegrationConfig()
	template := shell.GenerateFishIntegration(cfg)

	t.Run("keybinding", func(t *testing.T) {
		if !strings.Contains(template, `bind \cp`) {
			t.Error("Fish template should use bind \\cp for keybinding")
		}
	})

	t.Run("function_syntax", func(t *testing.T) {
		// Fish uses "function name" syntax
		if !strings.Contains(template, "function pp-status") {
			t.Error("Fish should use 'function name' syntax for pp-status")
		}
	})

	t.Run("function_descriptions", func(t *testing.T) {
		// Fish functions should have descriptions with -d flag
		if !strings.Contains(template, `-d "`) {
			t.Error("Fish functions should have descriptions")
		}
	})

	t.Run("completions", func(t *testing.T) {
		if !strings.Contains(template, "complete -c") {
			t.Error("Fish template should include completions with complete -c")
		}
	})

	t.Run("session_id_global_export", func(t *testing.T) {
		// Fish uses "set -gx" for global exports
		if !strings.Contains(template, "set -gx PPULSE_SESSION_ID") {
			t.Error("Fish should use 'set -gx' for global export of PPULSE_SESSION_ID")
		}
	})

	t.Run("session_id_check", func(t *testing.T) {
		// Fish should check if variable is set with "set -q"
		if !strings.Contains(template, "set -q PPULSE_SESSION_ID") {
			t.Error("Fish should check if PPULSE_SESSION_ID is set with 'set -q'")
		}
	})

	t.Run("banner_command", func(t *testing.T) {
		if !strings.Contains(template, "--banner --session-id") {
			t.Error("Fish pp-banner should pass session ID to banner command")
		}
	})
}

// TestNushellIntegration validates Nushell-specific integration features.
// Note: Nushell keybindings cannot be dynamically added and require manual setup.
func TestNushellIntegration(t *testing.T) {
	cfg := shell.DefaultIntegrationConfig()
	template := shell.GenerateNushellIntegration(cfg)

	t.Run("keybinding_documentation", func(t *testing.T) {
		// Nushell keybindings are documented as comments
		if !strings.Contains(template, "modifier: control") {
			t.Error("Nushell should document keybinding configuration")
		}
		if !strings.Contains(template, "keycode: char_p") {
			t.Error("Nushell should document Ctrl+P keybinding")
		}
	})

	t.Run("command_syntax", func(t *testing.T) {
		// Nushell uses "def" for command definitions
		if !strings.Contains(template, "def pp-status") {
			t.Error("Nushell should use 'def' syntax for pp-status")
		}
	})

	t.Run("completions", func(t *testing.T) {
		if !strings.Contains(template, `extern "prompt-pulse"`) {
			t.Error("Nushell should define extern completions")
		}
	})
}

// TestSessionAwareWaifuRefresh validates that session-aware waifu
// refresh works correctly in all shell templates.
func TestSessionAwareWaifuRefresh(t *testing.T) {
	cfg := shell.DefaultIntegrationConfig()

	shells := []struct {
		name      string
		generator func(shell.IntegrationConfig) string
	}{
		{"bash", shell.GenerateBashIntegration},
		{"zsh", shell.GenerateZshIntegration},
		{"fish", shell.GenerateFishIntegration},
	}

	for _, sh := range shells {
		t.Run(sh.name, func(t *testing.T) {
			template := sh.generator(cfg)

			// Session ID should be used for banner
			if !strings.Contains(template, "PPULSE_SESSION_ID") {
				t.Error("Missing PPULSE_SESSION_ID support")
			}

			// Banner should receive session ID
			if !strings.Contains(template, "--session-id") {
				t.Error("Banner should receive --session-id flag")
			}

			// Session ID should include process ID for uniqueness
			// Bash/Zsh use $$, Fish uses $fish_pid
			if sh.name == "fish" {
				if !strings.Contains(template, "$fish_pid") {
					t.Error("Fish should use $fish_pid for session uniqueness")
				}
			} else {
				if !strings.Contains(template, "$$") {
					t.Errorf("%s should use $$ for session uniqueness", sh.name)
				}
			}
		})
	}
}

// TestCustomBinaryPath validates that all shells correctly use
// a custom binary path when configured.
func TestCustomBinaryPath(t *testing.T) {
	cfg := shell.DefaultIntegrationConfig()
	cfg.BinaryPath = "/opt/bin/prompt-pulse"

	shells := []struct {
		name      string
		generator func(shell.IntegrationConfig) string
	}{
		{"bash", shell.GenerateBashIntegration},
		{"zsh", shell.GenerateZshIntegration},
		{"fish", shell.GenerateFishIntegration},
	}

	for _, sh := range shells {
		t.Run(sh.name, func(t *testing.T) {
			template := sh.generator(cfg)

			// Custom path should be used throughout
			if !strings.Contains(template, "/opt/bin/prompt-pulse --tui") {
				t.Error("Custom binary path should be used for TUI")
			}
			if !strings.Contains(template, "/opt/bin/prompt-pulse --banner") {
				t.Error("Custom binary path should be used for banner")
			}
			if !strings.Contains(template, "/opt/bin/prompt-pulse --daemon") {
				t.Error("Custom binary path should be used for daemon")
			}
		})
	}
}

// TestGenerateIntegration validates the dispatch function works correctly.
func TestGenerateIntegration(t *testing.T) {
	cfg := shell.DefaultIntegrationConfig()

	testCases := []struct {
		shellType shell.ShellType
		contains  string
	}{
		{shell.Bash, "bind -x"},
		{shell.Zsh, "bindkey"},
		{shell.Fish, `bind \cp`},
		{shell.Nushell, "def pp-status"},
	}

	for _, tc := range testCases {
		t.Run(tc.shellType.String(), func(t *testing.T) {
			output := shell.GenerateIntegration(tc.shellType, cfg)
			if !strings.Contains(output, tc.contains) {
				t.Errorf("GenerateIntegration(%s) should contain %q", tc.shellType, tc.contains)
			}
		})
	}
}

// TestAllShellsHaveHeader validates that all shell templates
// start with an appropriate header comment.
func TestAllShellsHaveHeader(t *testing.T) {
	cfg := shell.DefaultIntegrationConfig()

	shells := []struct {
		name      string
		generator func(shell.IntegrationConfig) string
	}{
		{"bash", shell.GenerateBashIntegration},
		{"zsh", shell.GenerateZshIntegration},
		{"fish", shell.GenerateFishIntegration},
		{"nushell", shell.GenerateNushellIntegration},
	}

	for _, sh := range shells {
		t.Run(sh.name, func(t *testing.T) {
			template := sh.generator(cfg)

			expectedHeader := "# prompt-pulse shell integration"
			if !strings.HasPrefix(template, expectedHeader) {
				t.Errorf("Template should start with %q", expectedHeader)
			}
		})
	}
}
