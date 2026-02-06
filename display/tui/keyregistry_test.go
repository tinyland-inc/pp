package tui

import "testing"

func TestDefaultRegistry_NoDuplicateKeys(t *testing.T) {
	reg := DefaultRegistry()
	conflicts := reg.HasDuplicateKeys()
	for _, c := range conflicts {
		t.Errorf("key conflict: %s", c)
	}
}

func TestDefaultRegistry_HasTUIBindings(t *testing.T) {
	reg := DefaultRegistry()
	tuiEntries := reg.ByMode(ModeTUI)
	if len(tuiEntries) == 0 {
		t.Error("expected TUI mode bindings")
	}
}

func TestDefaultRegistry_HasShellBindings(t *testing.T) {
	reg := DefaultRegistry()
	shellEntries := reg.ByMode(ModeShell)
	if len(shellEntries) == 0 {
		t.Error("expected Shell mode bindings")
	}
}

func TestDefaultRegistry_ByCategory(t *testing.T) {
	reg := DefaultRegistry()

	nav := reg.ByCategory(CategoryNavigation)
	if len(nav) == 0 {
		t.Error("expected navigation category bindings")
	}

	scroll := reg.ByCategory(CategoryScroll)
	if len(scroll) == 0 {
		t.Error("expected scroll category bindings")
	}

	system := reg.ByCategory(CategorySystem)
	if len(system) == 0 {
		t.Error("expected system category bindings")
	}

	data := reg.ByCategory(CategoryData)
	if len(data) == 0 {
		t.Error("expected data category bindings")
	}
}

func TestDefaultRegistry_FormatTable(t *testing.T) {
	reg := DefaultRegistry()
	table := reg.FormatTable()
	if table == "" {
		t.Error("expected non-empty table output")
	}
	if !containsString(table, "TUI Mode") {
		t.Error("expected table to contain 'TUI Mode' section")
	}
	if !containsString(table, "SHELL Mode") {
		t.Error("expected table to contain 'SHELL Mode' section")
	}
}

func TestDefaultRegistry_FormatJSON(t *testing.T) {
	reg := DefaultRegistry()
	entries := reg.FormatJSON()
	if len(entries) == 0 {
		t.Error("expected non-empty JSON entries")
	}

	// Check required fields.
	for i, e := range entries {
		if e["keys"] == "" {
			t.Errorf("entry %d: missing keys", i)
		}
		if e["desc"] == "" {
			t.Errorf("entry %d: missing desc", i)
		}
		if e["mode"] == "" {
			t.Errorf("entry %d: missing mode", i)
		}
	}
}
