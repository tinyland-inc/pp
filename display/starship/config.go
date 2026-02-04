package starship

import (
	"fmt"
	"strings"
)

// StarshipModuleConfig holds configuration for generating Starship TOML
// custom module definitions.
type StarshipModuleConfig struct {
	// BinaryPath is the path to the prompt-pulse binary.
	BinaryPath string
	// EnableClaude enables the Claude usage module.
	EnableClaude bool
	// EnableBilling enables the billing module.
	EnableBilling bool
	// EnableInfra enables the infrastructure module.
	EnableInfra bool
	// ClaudeSymbol is the icon for Claude module.
	ClaudeSymbol string
	// BillingSymbol is the icon for billing module.
	BillingSymbol string
	// InfraSymbol is the icon for infra module.
	InfraSymbol string
}

// DefaultStarshipModuleConfig returns a StarshipModuleConfig with sensible
// defaults: all modules enabled, binary as "prompt-pulse", and simple symbols.
func DefaultStarshipModuleConfig() StarshipModuleConfig {
	return StarshipModuleConfig{
		BinaryPath:    "prompt-pulse",
		EnableClaude:  true,
		EnableBilling: true,
		EnableInfra:   true,
		ClaudeSymbol:  "",
		BillingSymbol: "$",
		InfraSymbol:   "",
	}
}

// moduleSpec describes a single Starship custom module section to generate.
type moduleSpec struct {
	name   string
	style  string
	symbol string
}

// GenerateStarshipConfig generates Starship TOML configuration content for
// all enabled prompt-pulse custom modules. The output is suitable for appending
// to ~/.config/starship.toml.
func GenerateStarshipConfig(cfg StarshipModuleConfig) string {
	var b strings.Builder

	b.WriteString("# prompt-pulse Starship custom modules\n")
	b.WriteString("# Add these sections to your ~/.config/starship.toml\n")

	modules := enabledModules(cfg)
	if len(modules) > 0 {
		b.WriteString("#\n")
		b.WriteString("# Add ")
		refs := make([]string, len(modules))
		for i, m := range modules {
			refs[i] = fmt.Sprintf("\"custom.%s\"", m.name)
		}
		b.WriteString(strings.Join(refs, ", "))
		b.WriteString(" to your format string\n")
	}

	for _, m := range modules {
		b.WriteString("\n")
		writeModule(&b, cfg.BinaryPath, m)
	}

	return b.String()
}

// GenerateStarshipFormatString returns the format string snippet containing
// Starship variable references for all enabled modules. This can be inserted
// into the user's existing format string.
func GenerateStarshipFormatString(cfg StarshipModuleConfig) string {
	var b strings.Builder
	for _, m := range enabledModules(cfg) {
		fmt.Fprintf(&b, "${custom.%s}", m.name)
	}
	return b.String()
}

// enabledModules returns the list of module specs that are enabled in cfg.
func enabledModules(cfg StarshipModuleConfig) []moduleSpec {
	var modules []moduleSpec
	if cfg.EnableClaude {
		modules = append(modules, moduleSpec{
			name:   "pp_claude",
			style:  "purple",
			symbol: cfg.ClaudeSymbol,
		})
	}
	if cfg.EnableBilling {
		modules = append(modules, moduleSpec{
			name:   "pp_billing",
			style:  "green",
			symbol: cfg.BillingSymbol,
		})
	}
	if cfg.EnableInfra {
		modules = append(modules, moduleSpec{
			name:   "pp_infra",
			style:  "cyan",
			symbol: cfg.InfraSymbol,
		})
	}
	return modules
}

// writeModule writes a single [custom.<name>] TOML section to the builder.
func writeModule(b *strings.Builder, binaryPath string, m moduleSpec) {
	// Derive the collector name from the module name by stripping the "pp_" prefix.
	collectorName := strings.TrimPrefix(m.name, "pp_")

	fmt.Fprintf(b, "[custom.%s]\n", m.name)
	fmt.Fprintf(b, "command = \"%s --starship %s\"\n", binaryPath, collectorName)
	fmt.Fprintf(b, "when = \"command -v %s\"\n", binaryPath)
	fmt.Fprintf(b, "format = \"[$symbol($output)]($style) \"\n")
	fmt.Fprintf(b, "symbol = \"%s\"\n", m.symbol)
	fmt.Fprintf(b, "style = \"%s\"\n", m.style)
	fmt.Fprintf(b, "shell = [\"bash\", \"--noprofile\", \"--norc\"]\n")
}
