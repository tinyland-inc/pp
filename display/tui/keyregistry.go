package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

// KeyMode identifies the context where a keybinding is active.
type KeyMode string

const (
	// ModeTUI is the interactive TUI dashboard.
	ModeTUI KeyMode = "tui"
	// ModeShell is shell integration (Ctrl+P, aliases).
	ModeShell KeyMode = "shell"
	// ModeStarship is starship prompt modules.
	ModeStarship KeyMode = "starship"
)

// KeyCategory groups keybindings by function.
type KeyCategory string

const (
	CategoryNavigation KeyCategory = "navigation"
	CategoryScroll     KeyCategory = "scroll"
	CategorySystem     KeyCategory = "system"
	CategoryData       KeyCategory = "data"
)

// KeyEntry represents a single registered keybinding with metadata.
type KeyEntry struct {
	// Binding is the charmbracelet key binding.
	Binding key.Binding
	// Mode is the context where this binding is active.
	Mode KeyMode
	// Category groups this binding by function.
	Category KeyCategory
	// Since is the version where this binding was introduced.
	Since string
}

// KeyRegistry is the single source of truth for all prompt-pulse keybindings.
type KeyRegistry struct {
	Entries []KeyEntry
}

// DefaultRegistry returns the canonical key registry with all bindings.
func DefaultRegistry() *KeyRegistry {
	return &KeyRegistry{
		Entries: []KeyEntry{
			// Navigation
			{Binding: key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab", "next tab")), Mode: ModeTUI, Category: CategoryNavigation, Since: "0.1.0"},
			{Binding: key.NewBinding(key.WithKeys("shift+tab", "left"), key.WithHelp("shift+tab", "prev tab")), Mode: ModeTUI, Category: CategoryNavigation, Since: "0.1.0"},
			{Binding: key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "claude")), Mode: ModeTUI, Category: CategoryNavigation, Since: "0.1.0"},
			{Binding: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "billing")), Mode: ModeTUI, Category: CategoryNavigation, Since: "0.1.0"},
			{Binding: key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "infra")), Mode: ModeTUI, Category: CategoryNavigation, Since: "0.1.0"},
			{Binding: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "system")), Mode: ModeTUI, Category: CategoryNavigation, Since: "0.2.0"},

			// Scroll
			{Binding: key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/up", "scroll up")), Mode: ModeTUI, Category: CategoryScroll, Since: "0.2.0"},
			{Binding: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/dn", "scroll down")), Mode: ModeTUI, Category: CategoryScroll, Since: "0.2.0"},
			{Binding: key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")), Mode: ModeTUI, Category: CategoryScroll, Since: "0.2.0"},
			{Binding: key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")), Mode: ModeTUI, Category: CategoryScroll, Since: "0.2.0"},
			{Binding: key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")), Mode: ModeTUI, Category: CategoryScroll, Since: "0.2.0"},
			{Binding: key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")), Mode: ModeTUI, Category: CategoryScroll, Since: "0.2.0"},

			// System
			{Binding: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")), Mode: ModeTUI, Category: CategorySystem, Since: "0.2.0"},
			{Binding: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")), Mode: ModeTUI, Category: CategorySystem, Since: "0.1.0"},

			// Data
			{Binding: key.NewBinding(key.WithKeys("r", "ctrl+r"), key.WithHelp("r", "refresh")), Mode: ModeTUI, Category: CategoryData, Since: "0.2.0"},

			// Shell bindings
			{Binding: key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("Ctrl+P", "launch TUI")), Mode: ModeShell, Category: CategoryNavigation, Since: "0.1.0"},
		},
	}
}

// ByMode returns all entries matching the given mode.
func (r *KeyRegistry) ByMode(mode KeyMode) []KeyEntry {
	var result []KeyEntry
	for _, e := range r.Entries {
		if e.Mode == mode {
			result = append(result, e)
		}
	}
	return result
}

// ByCategory returns all entries matching the given category.
func (r *KeyRegistry) ByCategory(cat KeyCategory) []KeyEntry {
	var result []KeyEntry
	for _, e := range r.Entries {
		if e.Category == cat {
			result = append(result, e)
		}
	}
	return result
}

// HasDuplicateKeys checks for duplicate key assignments within a mode.
// Returns a list of conflicts (empty if none).
func (r *KeyRegistry) HasDuplicateKeys() []string {
	type modeKey struct {
		mode KeyMode
		key  string
	}
	seen := make(map[modeKey]string)
	var conflicts []string

	for _, e := range r.Entries {
		for _, k := range e.Binding.Keys() {
			mk := modeKey{mode: e.Mode, key: k}
			if existing, ok := seen[mk]; ok {
				conflicts = append(conflicts, fmt.Sprintf(
					"duplicate key %q in mode %s: %s vs %s",
					k, e.Mode, existing, e.Binding.Help().Desc,
				))
			} else {
				seen[mk] = e.Binding.Help().Desc
			}
		}
	}

	return conflicts
}

// FormatTable returns a formatted table of all keybindings.
func (r *KeyRegistry) FormatTable() string {
	var sb strings.Builder

	modes := []KeyMode{ModeTUI, ModeShell, ModeStarship}
	for _, mode := range modes {
		entries := r.ByMode(mode)
		if len(entries) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("\n%s Mode:\n", strings.ToUpper(string(mode))))
		sb.WriteString(strings.Repeat("-", 50) + "\n")

		for _, e := range entries {
			keysStr := strings.Join(e.Binding.Keys(), ", ")
			sb.WriteString(fmt.Sprintf("  %-20s  %s\n", keysStr, e.Binding.Help().Desc))
		}
	}

	return sb.String()
}

// FormatJSON returns a JSON-compatible slice of binding descriptions.
func (r *KeyRegistry) FormatJSON() []map[string]string {
	var result []map[string]string
	for _, e := range r.Entries {
		result = append(result, map[string]string{
			"keys":     strings.Join(e.Binding.Keys(), ", "),
			"desc":     e.Binding.Help().Desc,
			"mode":     string(e.Mode),
			"category": string(e.Category),
			"since":    e.Since,
		})
	}
	return result
}
