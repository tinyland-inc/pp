package starship

import (
	"strings"
	"testing"
)

func TestGenerateStarshipConfig_AllModules(t *testing.T) {
	cfg := DefaultStarshipModuleConfig()
	out := GenerateStarshipConfig(cfg)

	for _, section := range []string{"[custom.pp_claude]", "[custom.pp_billing]", "[custom.pp_infra]"} {
		if !strings.Contains(out, section) {
			t.Errorf("expected output to contain %q", section)
		}
	}
}

func TestGenerateStarshipConfig_ClaudeOnly(t *testing.T) {
	cfg := DefaultStarshipModuleConfig()
	cfg.EnableBilling = false
	cfg.EnableInfra = false

	out := GenerateStarshipConfig(cfg)

	if !strings.Contains(out, "[custom.pp_claude]") {
		t.Error("expected output to contain [custom.pp_claude]")
	}
	if strings.Contains(out, "[custom.pp_billing]") {
		t.Error("expected output NOT to contain [custom.pp_billing]")
	}
	if strings.Contains(out, "[custom.pp_infra]") {
		t.Error("expected output NOT to contain [custom.pp_infra]")
	}
}

func TestGenerateStarshipConfig_NoModules(t *testing.T) {
	cfg := DefaultStarshipModuleConfig()
	cfg.EnableClaude = false
	cfg.EnableBilling = false
	cfg.EnableInfra = false

	out := GenerateStarshipConfig(cfg)

	if !strings.Contains(out, "# prompt-pulse Starship custom modules") {
		t.Error("expected header comment to be present")
	}
	if strings.Contains(out, "[custom.") {
		t.Error("expected no custom module sections when all disabled")
	}
}

func TestGenerateStarshipConfig_CustomBinary(t *testing.T) {
	cfg := DefaultStarshipModuleConfig()
	cfg.BinaryPath = "/usr/local/bin/pp"

	out := GenerateStarshipConfig(cfg)

	if !strings.Contains(out, `command = "/usr/local/bin/pp --starship claude"`) {
		t.Error("expected custom binary path in command")
	}
	if !strings.Contains(out, `when = "command -v /usr/local/bin/pp"`) {
		t.Error("expected custom binary path in when clause")
	}
}

func TestGenerateStarshipConfig_CustomSymbols(t *testing.T) {
	cfg := DefaultStarshipModuleConfig()
	cfg.ClaudeSymbol = "C:"
	cfg.BillingSymbol = "B:"
	cfg.InfraSymbol = "I:"

	out := GenerateStarshipConfig(cfg)

	// Each symbol should appear in its respective section. We verify by
	// checking that the symbol line follows the correct section header.
	sections := map[string]string{
		"[custom.pp_claude]":  `symbol = "C:"`,
		"[custom.pp_billing]": `symbol = "B:"`,
		"[custom.pp_infra]":   `symbol = "I:"`,
	}
	for section, expected := range sections {
		idx := strings.Index(out, section)
		if idx == -1 {
			t.Errorf("missing section %s", section)
			continue
		}
		// Look for the symbol line within the next 300 chars of the section start.
		chunk := out[idx:]
		if len(chunk) > 300 {
			chunk = chunk[:300]
		}
		if !strings.Contains(chunk, expected) {
			t.Errorf("in section %s, expected %q", section, expected)
		}
	}
}

func TestGenerateStarshipConfig_ContainsTOML(t *testing.T) {
	cfg := DefaultStarshipModuleConfig()
	out := GenerateStarshipConfig(cfg)

	// Verify basic TOML structure elements.
	checks := []string{
		"[custom.",          // section headers
		`command = "`,       // quoted string values
		`style = "`,         // quoted style
		`shell = ["bash"`,   // array syntax
		`"--noprofile"`,     // array element
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("expected TOML structure element %q", c)
		}
	}
}

func TestGenerateStarshipFormatString_AllModules(t *testing.T) {
	cfg := DefaultStarshipModuleConfig()
	out := GenerateStarshipFormatString(cfg)

	expected := "${custom.pp_claude}${custom.pp_billing}${custom.pp_infra}"
	if out != expected {
		t.Errorf("got %q, want %q", out, expected)
	}
}

func TestGenerateStarshipFormatString_Partial(t *testing.T) {
	cfg := DefaultStarshipModuleConfig()
	cfg.EnableBilling = false

	out := GenerateStarshipFormatString(cfg)

	if !strings.Contains(out, "${custom.pp_claude}") {
		t.Error("expected pp_claude in format string")
	}
	if strings.Contains(out, "${custom.pp_billing}") {
		t.Error("expected pp_billing NOT in format string")
	}
	if !strings.Contains(out, "${custom.pp_infra}") {
		t.Error("expected pp_infra in format string")
	}
}

func TestDefaultStarshipModuleConfig(t *testing.T) {
	cfg := DefaultStarshipModuleConfig()

	if cfg.BinaryPath != "prompt-pulse" {
		t.Errorf("BinaryPath = %q, want %q", cfg.BinaryPath, "prompt-pulse")
	}
	if !cfg.EnableClaude {
		t.Error("EnableClaude should be true by default")
	}
	if !cfg.EnableBilling {
		t.Error("EnableBilling should be true by default")
	}
	if !cfg.EnableInfra {
		t.Error("EnableInfra should be true by default")
	}
	if cfg.ClaudeSymbol != "" {
		t.Errorf("ClaudeSymbol = %q, want empty string", cfg.ClaudeSymbol)
	}
	if cfg.BillingSymbol != "$" {
		t.Errorf("BillingSymbol = %q, want %q", cfg.BillingSymbol, "$")
	}
	if cfg.InfraSymbol != "" {
		t.Errorf("InfraSymbol = %q, want empty string", cfg.InfraSymbol)
	}
}
