package color_test

import (
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/display/tui"
)

// TestThemeResolution verifies the --theme flag resolution logic.
// This mirrors the resolution in main.go: "auto" uses the config value,
// other values override it.
func TestThemeResolution(t *testing.T) {
	tests := []struct {
		name       string
		flagValue  string
		configVal  string
		wantTheme  string
	}{
		{
			name:      "auto uses config monitoring",
			flagValue: "auto",
			configVal: "monitoring",
			wantTheme: "monitoring",
		},
		{
			name:      "auto uses config minimal",
			flagValue: "auto",
			configVal: "minimal",
			wantTheme: "minimal",
		},
		{
			name:      "auto uses config full",
			flagValue: "auto",
			configVal: "full",
			wantTheme: "full",
		},
		{
			name:      "explicit monitoring overrides config",
			flagValue: "monitoring",
			configVal: "full",
			wantTheme: "monitoring",
		},
		{
			name:      "explicit minimal overrides config",
			flagValue: "minimal",
			configVal: "monitoring",
			wantTheme: "minimal",
		},
		{
			name:      "explicit full overrides config",
			flagValue: "full",
			configVal: "minimal",
			wantTheme: "full",
		},
		{
			name:      "unknown flag falls back to monitoring",
			flagValue: "nonexistent",
			configVal: "minimal",
			wantTheme: "monitoring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the resolution logic from main.go.
			themeName := tt.configVal
			if tt.flagValue != "auto" {
				themeName = tt.flagValue
			}

			preset := tui.GetThemePreset(themeName)
			if preset.Name != tt.wantTheme {
				t.Errorf("resolved theme = %q, want %q", preset.Name, tt.wantTheme)
			}
		})
	}
}

// TestApplyThemePresets verifies that all three theme presets can be applied
// without panicking.
func TestApplyThemePresets(t *testing.T) {
	for _, preset := range tui.AllThemePresets() {
		t.Run(preset.Name, func(t *testing.T) {
			// Should not panic.
			tui.ApplyTheme(preset)
		})
	}
	// Restore default.
	tui.ApplyTheme(tui.MonitoringTheme)
}
