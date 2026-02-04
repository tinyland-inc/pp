package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestGetThemePreset_Monitoring(t *testing.T) {
	p := GetThemePreset("monitoring")
	if p.Name != "monitoring" {
		t.Errorf("Name = %q, want %q", p.Name, "monitoring")
	}
}

func TestGetThemePreset_Minimal(t *testing.T) {
	p := GetThemePreset("minimal")
	if p.Name != "minimal" {
		t.Errorf("Name = %q, want %q", p.Name, "minimal")
	}
}

func TestGetThemePreset_Full(t *testing.T) {
	p := GetThemePreset("full")
	if p.Name != "full" {
		t.Errorf("Name = %q, want %q", p.Name, "full")
	}
}

func TestGetThemePreset_Unknown(t *testing.T) {
	p := GetThemePreset("nonexistent")
	if p.Name != "monitoring" {
		t.Errorf("unknown name should return monitoring, got %q", p.Name)
	}
}

func TestAllThemePresets(t *testing.T) {
	presets := AllThemePresets()
	if len(presets) != 3 {
		t.Errorf("expected 3 presets, got %d", len(presets))
	}

	// Verify mutation safety: modifying the returned slice should not affect
	// the internal list.
	presets[0].Name = "mutated"
	original := AllThemePresets()
	if original[0].Name == "mutated" {
		t.Error("AllThemePresets should return a copy, not a reference")
	}
}

func TestApplyTheme(t *testing.T) {
	// Start with the default theme (set by init in theme.go).
	beforeTab := styleActiveTab

	// Apply a different theme.
	ApplyTheme(MinimalTheme)
	afterTab := styleActiveTab

	// The active tab background should differ because monitoring uses #7C3AED
	// and minimal uses #8B5CF6 for Primary.
	beforeBg := beforeTab.GetBackground()
	afterBg := afterTab.GetBackground()

	if beforeBg == afterBg {
		t.Error("expected styleActiveTab background to change after ApplyTheme")
	}

	// Restore the default theme for other tests.
	ApplyTheme(MonitoringTheme)
}

func TestApplyTheme_CompactMode(t *testing.T) {
	// Minimal has CompactMode: true, which uses Padding(0, 1).
	ApplyTheme(MinimalTheme)
	top, right, bottom, left := styleContent.GetPadding()
	if top != 0 || bottom != 0 {
		t.Errorf("compact mode: vertical padding should be 0, got top=%d bottom=%d", top, bottom)
	}
	if right != 1 || left != 1 {
		t.Errorf("compact mode: horizontal padding should be 1, got right=%d left=%d", right, left)
	}

	// Full has CompactMode: false, which uses Padding(1, 2).
	ApplyTheme(FullTheme)
	top, right, bottom, left = styleContent.GetPadding()
	if top != 1 || bottom != 1 {
		t.Errorf("full mode: vertical padding should be 1, got top=%d bottom=%d", top, bottom)
	}
	if right != 2 || left != 2 {
		t.Errorf("full mode: horizontal padding should be 2, got right=%d left=%d", right, left)
	}

	// Restore default.
	ApplyTheme(MonitoringTheme)
}

func TestThemePreset_Names(t *testing.T) {
	for _, p := range AllThemePresets() {
		if p.Name == "" {
			t.Error("preset has empty Name")
		}
		if p.Description == "" {
			t.Errorf("preset %q has empty Description", p.Name)
		}
	}
}

func TestThemePreset_Colors(t *testing.T) {
	for _, p := range AllThemePresets() {
		colors := map[string]lipgloss.Color{
			"Primary":    p.Primary,
			"Secondary":  p.Secondary,
			"Success":    p.Success,
			"Warning":    p.Warning,
			"Danger":     p.Danger,
			"Muted":      p.Muted,
			"Background": p.Background,
		}
		for name, c := range colors {
			if string(c) == "" {
				t.Errorf("preset %q has empty %s color", p.Name, name)
			}
		}
	}
}
