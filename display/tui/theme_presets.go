package tui

import "github.com/charmbracelet/lipgloss"

// ThemePreset defines a complete color scheme and layout configuration
// that can be applied at runtime to change the TUI appearance.
type ThemePreset struct {
	Name        string
	Description string
	// Colors
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Danger     lipgloss.Color
	Muted      lipgloss.Color
	Background lipgloss.Color
	// Layout
	ShowBorders bool
	CompactMode bool
}

// Predefined theme presets.
var (
	// MonitoringTheme is the default dark theme optimized for status monitoring.
	MonitoringTheme = ThemePreset{
		Name:        "monitoring",
		Description: "Dark theme for status monitoring",
		Primary:     lipgloss.Color("#7C3AED"),
		Secondary:   lipgloss.Color("#06B6D4"),
		Success:     lipgloss.Color("#22C55E"),
		Warning:     lipgloss.Color("#EAB308"),
		Danger:      lipgloss.Color("#EF4444"),
		Muted:       lipgloss.Color("#6B7280"),
		Background:  lipgloss.Color("#1E1B2E"),
		ShowBorders: true,
		CompactMode: false,
	}

	// MinimalTheme is a clean, low-distraction theme.
	MinimalTheme = ThemePreset{
		Name:        "minimal",
		Description: "Clean minimal theme",
		Primary:     lipgloss.Color("#8B5CF6"),
		Secondary:   lipgloss.Color("#67E8F9"),
		Success:     lipgloss.Color("#4ADE80"),
		Warning:     lipgloss.Color("#FCD34D"),
		Danger:      lipgloss.Color("#F87171"),
		Muted:       lipgloss.Color("#9CA3AF"),
		Background:  lipgloss.Color("#0F172A"),
		ShowBorders: false,
		CompactMode: true,
	}

	// FullTheme is a rich theme with all visual features enabled.
	FullTheme = ThemePreset{
		Name:        "full",
		Description: "Rich theme with all features",
		Primary:     lipgloss.Color("#A78BFA"),
		Secondary:   lipgloss.Color("#22D3EE"),
		Success:     lipgloss.Color("#34D399"),
		Warning:     lipgloss.Color("#FBBF24"),
		Danger:      lipgloss.Color("#FB7185"),
		Muted:       lipgloss.Color("#D1D5DB"),
		Background:  lipgloss.Color("#1E293B"),
		ShowBorders: true,
		CompactMode: false,
	}
)

// allPresets is the canonical list of available theme presets.
var allPresets = []ThemePreset{MonitoringTheme, MinimalTheme, FullTheme}

// GetThemePreset returns the theme preset matching the given name.
// Unknown names return MonitoringTheme as the default.
func GetThemePreset(name string) ThemePreset {
	for _, p := range allPresets {
		if p.Name == name {
			return p
		}
	}
	return MonitoringTheme
}

// AllThemePresets returns all available theme presets.
func AllThemePresets() []ThemePreset {
	out := make([]ThemePreset, len(allPresets))
	copy(out, allPresets)
	return out
}

// ApplyTheme updates the package-level style variables to use the given
// preset's colors. This allows runtime theme switching without restarting
// the application.
func ApplyTheme(preset ThemePreset) {
	styleActiveTab = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(preset.Primary).
		Padding(0, 2)

	styleInactiveTab = lipgloss.NewStyle().
		Foreground(preset.Muted).
		Padding(0, 2)

	if preset.ShowBorders {
		styleHeader = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(preset.Muted).
			MarginBottom(1)
	} else {
		styleHeader = lipgloss.NewStyle().
			MarginBottom(1)
	}

	styleFooter = lipgloss.NewStyle().
		Foreground(preset.Muted).
		MarginTop(1)

	if preset.CompactMode {
		styleContent = lipgloss.NewStyle().
			Padding(0, 1)
	} else {
		styleContent = lipgloss.NewStyle().
			Padding(1, 2)
	}

	styleTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(preset.Secondary)
}
