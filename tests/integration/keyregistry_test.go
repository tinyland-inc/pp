package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/display/tui"
)

// ---------- Key Registry Tests ----------

// TestNoDuplicateKeys verifies that no two keybindings share the same key
// within the same mode (TUI, Shell, Starship).
func TestNoDuplicateKeys(t *testing.T) {
	reg := tui.DefaultRegistry()
	conflicts := reg.HasDuplicateKeys()

	if len(conflicts) > 0 {
		for _, c := range conflicts {
			t.Errorf("key conflict: %s", c)
		}
	}
}

// TestAllKeysDocumented verifies every key binding has a non-empty help description.
func TestAllKeysDocumented(t *testing.T) {
	reg := tui.DefaultRegistry()

	for i, entry := range reg.Entries {
		help := entry.Binding.Help()

		if help.Key == "" {
			t.Errorf("entry %d: help key is empty (mode=%s, category=%s)",
				i, entry.Mode, entry.Category)
		}
		if help.Desc == "" {
			t.Errorf("entry %d (key=%q): help description is empty (mode=%s, category=%s)",
				i, help.Key, entry.Mode, entry.Category)
		}
	}
}

// TestAllKeysHaveMode verifies every key binding has a valid mode.
func TestAllKeysHaveMode(t *testing.T) {
	validModes := map[tui.KeyMode]bool{
		tui.ModeTUI:      true,
		tui.ModeShell:    true,
		tui.ModeStarship: true,
	}

	reg := tui.DefaultRegistry()

	for i, entry := range reg.Entries {
		if !validModes[entry.Mode] {
			t.Errorf("entry %d: invalid mode %q", i, entry.Mode)
		}
	}
}

// TestAllKeysHaveCategory verifies every key binding has a valid category.
func TestAllKeysHaveCategory(t *testing.T) {
	validCategories := map[tui.KeyCategory]bool{
		tui.CategoryNavigation: true,
		tui.CategoryScroll:     true,
		tui.CategorySystem:     true,
		tui.CategoryData:       true,
	}

	reg := tui.DefaultRegistry()

	for i, entry := range reg.Entries {
		if !validCategories[entry.Category] {
			t.Errorf("entry %d: invalid category %q", i, entry.Category)
		}
	}
}

// TestAllKeysHaveSince verifies every key binding has a non-empty Since version.
func TestAllKeysHaveSince(t *testing.T) {
	reg := tui.DefaultRegistry()

	for i, entry := range reg.Entries {
		if entry.Since == "" {
			t.Errorf("entry %d (mode=%s, desc=%q): Since version is empty",
				i, entry.Mode, entry.Binding.Help().Desc)
		}
	}
}

// TestFormatTableOutput verifies --keys table output is non-empty and
// properly formatted with expected sections.
func TestFormatTableOutput(t *testing.T) {
	reg := tui.DefaultRegistry()
	table := reg.FormatTable()

	if table == "" {
		t.Fatal("FormatTable() returned empty string")
	}

	// Should contain TUI mode section.
	if !strings.Contains(table, "TUI Mode") {
		t.Error("table should contain 'TUI Mode' section")
	}

	// Should contain SHELL mode section.
	if !strings.Contains(table, "SHELL Mode") {
		t.Error("table should contain 'SHELL Mode' section")
	}

	// Should contain separator lines.
	if !strings.Contains(table, "---") {
		t.Error("table should contain separator lines")
	}

	// Should contain at least one key description.
	if !strings.Contains(table, "quit") {
		t.Error("table should contain 'quit' key description")
	}
	if !strings.Contains(table, "next tab") {
		t.Error("table should contain 'next tab' key description")
	}
	if !strings.Contains(table, "refresh") {
		t.Error("table should contain 'refresh' key description")
	}
}

// TestFormatJSONOutput verifies --keys JSON output parses correctly and
// contains the expected fields.
func TestFormatJSONOutput(t *testing.T) {
	reg := tui.DefaultRegistry()
	entries := reg.FormatJSON()

	if len(entries) == 0 {
		t.Fatal("FormatJSON() returned empty slice")
	}

	// Verify the output is valid JSON by marshaling and unmarshaling.
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("json.Marshal(FormatJSON()) error: %v", err)
	}

	var parsed []map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if len(parsed) != len(entries) {
		t.Errorf("parsed %d entries, want %d", len(parsed), len(entries))
	}

	// Verify required fields exist in every entry.
	requiredFields := []string{"keys", "desc", "mode", "category", "since"}
	for i, entry := range parsed {
		for _, field := range requiredFields {
			val, ok := entry[field]
			if !ok {
				t.Errorf("entry %d: missing field %q", i, field)
				continue
			}
			if val == "" {
				t.Errorf("entry %d: field %q is empty", i, field)
			}
		}
	}
}

// TestFormatJSONContainsAllModes verifies JSON output includes entries from all modes.
func TestFormatJSONContainsAllModes(t *testing.T) {
	reg := tui.DefaultRegistry()
	entries := reg.FormatJSON()

	modes := make(map[string]bool)
	for _, entry := range entries {
		modes[entry["mode"]] = true
	}

	if !modes["tui"] {
		t.Error("JSON output should contain TUI mode entries")
	}
	if !modes["shell"] {
		t.Error("JSON output should contain Shell mode entries")
	}
}

// TestByModeReturnsCorrectEntries validates that ByMode filtering works.
func TestByModeReturnsCorrectEntries(t *testing.T) {
	reg := tui.DefaultRegistry()

	tuiEntries := reg.ByMode(tui.ModeTUI)
	if len(tuiEntries) == 0 {
		t.Error("ByMode(TUI) should return entries")
	}
	for _, entry := range tuiEntries {
		if entry.Mode != tui.ModeTUI {
			t.Errorf("ByMode(TUI) returned entry with mode %q", entry.Mode)
		}
	}

	shellEntries := reg.ByMode(tui.ModeShell)
	if len(shellEntries) == 0 {
		t.Error("ByMode(Shell) should return entries")
	}
	for _, entry := range shellEntries {
		if entry.Mode != tui.ModeShell {
			t.Errorf("ByMode(Shell) returned entry with mode %q", entry.Mode)
		}
	}
}

// TestByCategoryReturnsCorrectEntries validates that ByCategory filtering works.
func TestByCategoryReturnsCorrectEntries(t *testing.T) {
	reg := tui.DefaultRegistry()

	categories := []tui.KeyCategory{
		tui.CategoryNavigation,
		tui.CategoryScroll,
		tui.CategorySystem,
		tui.CategoryData,
	}

	for _, cat := range categories {
		entries := reg.ByCategory(cat)
		if len(entries) == 0 {
			t.Errorf("ByCategory(%s) should return entries", cat)
			continue
		}
		for _, entry := range entries {
			if entry.Category != cat {
				t.Errorf("ByCategory(%s) returned entry with category %q", cat, entry.Category)
			}
		}
	}
}

// TestMinimumKeyCount validates the registry has at least the expected
// number of keybindings across all modes.
func TestMinimumKeyCount(t *testing.T) {
	reg := tui.DefaultRegistry()

	// TUI should have at least 14 bindings (6 nav + 6 scroll + 1 help + 1 quit + 1 refresh - some share).
	tuiEntries := reg.ByMode(tui.ModeTUI)
	if len(tuiEntries) < 14 {
		t.Errorf("TUI mode has %d entries, expected at least 14", len(tuiEntries))
	}

	// Shell should have at least 1 binding (Ctrl+P).
	shellEntries := reg.ByMode(tui.ModeShell)
	if len(shellEntries) < 1 {
		t.Errorf("Shell mode has %d entries, expected at least 1", len(shellEntries))
	}

	// Total should be at least 15.
	if len(reg.Entries) < 15 {
		t.Errorf("total registry has %d entries, expected at least 15", len(reg.Entries))
	}
}
