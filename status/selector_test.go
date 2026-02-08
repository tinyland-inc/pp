package status

import (
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/waifu"
)

// deterministicRand always returns 0, selecting the first element.
func deterministicRand(n int) int { return 0 }

// lastRand returns n-1, selecting the last element.
func lastRand(n int) int { return n - 1 }

func TestDefaultCategoryMapping_NonEmpty(t *testing.T) {
	m := DefaultCategoryMapping()

	tests := []struct {
		name       string
		categories []string
	}{
		{"Healthy", m.Healthy},
		{"Warning", m.Warning},
		{"Critical", m.Critical},
		{"Unknown", m.Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.categories) == 0 {
				t.Errorf("DefaultCategoryMapping().%s is empty", tt.name)
			}
		})
	}
}

func TestDefaultCategoryMapping_AllCategoriesValid(t *testing.T) {
	m := DefaultCategoryMapping()

	allCategories := make([]string, 0)
	allCategories = append(allCategories, m.Healthy...)
	allCategories = append(allCategories, m.Warning...)
	allCategories = append(allCategories, m.Critical...)
	allCategories = append(allCategories, m.Unknown...)

	for _, c := range allCategories {
		if !waifu.IsValidCategory(c) {
			t.Errorf("category %q in DefaultCategoryMapping is not a valid waifu category", c)
		}
	}
}

func TestDefaultSelectorConfig_ReturnsWorkingConfig(t *testing.T) {
	cfg := DefaultSelectorConfig()

	if cfg.RandFunc == nil {
		t.Fatal("DefaultSelectorConfig().RandFunc is nil")
	}
	if len(cfg.Mapping.Healthy) == 0 {
		t.Error("DefaultSelectorConfig().Mapping.Healthy is empty")
	}
	if cfg.OverrideCategory != "" {
		t.Errorf("DefaultSelectorConfig().OverrideCategory = %q, want empty", cfg.OverrideCategory)
	}
}

func TestSelectCategory_ByLevel(t *testing.T) {
	cfg := DefaultSelectorConfig()
	cfg.RandFunc = deterministicRand
	sel := NewSelector(cfg)

	tests := []struct {
		name     string
		level    Level
		expected []string
	}{
		{"Healthy", LevelHealthy, cfg.Mapping.Healthy},
		{"Warning", LevelWarning, cfg.Mapping.Warning},
		{"Critical", LevelCritical, cfg.Mapping.Critical},
		{"Unknown", LevelUnknown, cfg.Mapping.Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sel.SelectCategory(tt.level)
			if got != tt.expected[0] {
				t.Errorf("SelectCategory(%v) = %q, want %q", tt.level, got, tt.expected[0])
			}
		})
	}
}

func TestSelectCategory_OverrideCategory_Valid(t *testing.T) {
	cfg := DefaultSelectorConfig()
	cfg.OverrideCategory = "neko"
	cfg.RandFunc = deterministicRand
	sel := NewSelector(cfg)

	// Override should be returned regardless of level.
	levels := []Level{LevelHealthy, LevelWarning, LevelCritical, LevelUnknown}
	for _, level := range levels {
		got := sel.SelectCategory(level)
		if got != "neko" {
			t.Errorf("SelectCategory(%v) with override = %q, want %q", level, got, "neko")
		}
	}
}

func TestSelectCategory_OverrideCategory_Invalid(t *testing.T) {
	cfg := DefaultSelectorConfig()
	cfg.OverrideCategory = "not_a_real_category"
	cfg.RandFunc = deterministicRand
	sel := NewSelector(cfg)

	// Invalid override should fall back to level-based selection.
	got := sel.SelectCategory(LevelHealthy)
	if got != cfg.Mapping.Healthy[0] {
		t.Errorf("SelectCategory with invalid override = %q, want %q (first Healthy)", got, cfg.Mapping.Healthy[0])
	}
}

func TestSelectCategory_NeverReturnsEmptyString(t *testing.T) {
	cfg := SelectorConfig{
		Mapping: CategoryMapping{
			Healthy:  []string{},
			Warning:  []string{},
			Critical: []string{},
			Unknown:  []string{},
		},
		RandFunc: deterministicRand,
	}
	sel := NewSelector(cfg)

	levels := []Level{LevelHealthy, LevelWarning, LevelCritical, LevelUnknown}
	for _, level := range levels {
		got := sel.SelectCategory(level)
		if got == "" {
			t.Errorf("SelectCategory(%v) returned empty string", level)
		}
		if got != "waifu" {
			t.Errorf("SelectCategory(%v) with empty mapping = %q, want %q", level, got, "waifu")
		}
	}
}

func TestSelectCategory_EmptyMappingFallsBackToWaifu(t *testing.T) {
	cfg := SelectorConfig{
		Mapping:  CategoryMapping{},
		RandFunc: deterministicRand,
	}
	sel := NewSelector(cfg)

	got := sel.SelectCategory(LevelCritical)
	if got != "waifu" {
		t.Errorf("SelectCategory with nil mapping list = %q, want %q", got, "waifu")
	}
}

func TestSelectCategory_InvalidCategoriesInMappingAreSkipped(t *testing.T) {
	cfg := SelectorConfig{
		Mapping: CategoryMapping{
			Healthy: []string{"bogus_invalid", "happy"},
		},
		RandFunc: deterministicRand,
	}
	sel := NewSelector(cfg)

	got := sel.SelectCategory(LevelHealthy)
	if got != "happy" {
		t.Errorf("SelectCategory should skip invalid categories; got %q, want %q", got, "happy")
	}
}

func TestSelectCategory_AllInvalidCategoriesFallsBack(t *testing.T) {
	cfg := SelectorConfig{
		Mapping: CategoryMapping{
			Healthy: []string{"fake1", "fake2", "fake3"},
		},
		RandFunc: deterministicRand,
	}
	sel := NewSelector(cfg)

	got := sel.SelectCategory(LevelHealthy)
	if got != "waifu" {
		t.Errorf("SelectCategory with all-invalid mapping = %q, want %q", got, "waifu")
	}
}

func TestCategoriesForLevel_ReturnsCorrectList(t *testing.T) {
	m := CategoryMapping{
		Healthy:  []string{"happy", "smile"},
		Warning:  []string{"smug"},
		Critical: []string{"cry", "bonk", "slap"},
		Unknown:  []string{"waifu"},
	}
	cfg := SelectorConfig{Mapping: m, RandFunc: deterministicRand}
	sel := NewSelector(cfg)

	tests := []struct {
		name     string
		level    Level
		expected []string
	}{
		{"Healthy", LevelHealthy, m.Healthy},
		{"Warning", LevelWarning, m.Warning},
		{"Critical", LevelCritical, m.Critical},
		{"Unknown", LevelUnknown, m.Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sel.CategoriesForLevel(tt.level)
			if len(got) != len(tt.expected) {
				t.Fatalf("CategoriesForLevel(%v) returned %d items, want %d", tt.level, len(got), len(tt.expected))
			}
			for i, c := range got {
				if c != tt.expected[i] {
					t.Errorf("CategoriesForLevel(%v)[%d] = %q, want %q", tt.level, i, c, tt.expected[i])
				}
			}
		})
	}
}

func TestCategoriesForLevel_UnknownLevelDefaultsToUnknown(t *testing.T) {
	m := DefaultCategoryMapping()
	cfg := SelectorConfig{Mapping: m, RandFunc: deterministicRand}
	sel := NewSelector(cfg)

	// A level value outside defined constants should map to Unknown.
	got := sel.CategoriesForLevel(Level(99))
	if len(got) != len(m.Unknown) {
		t.Errorf("CategoriesForLevel(99) returned %d items, want %d (Unknown list)", len(got), len(m.Unknown))
	}
}

func TestSelectCategory_CustomMapping(t *testing.T) {
	custom := CategoryMapping{
		Healthy:  []string{"dance"},
		Warning:  []string{"poke"},
		Critical: []string{"kill"},
		Unknown:  []string{"shinobu"},
	}
	cfg := SelectorConfig{
		Mapping:  custom,
		RandFunc: deterministicRand,
	}
	sel := NewSelector(cfg)

	tests := []struct {
		level    Level
		expected string
	}{
		{LevelHealthy, "dance"},
		{LevelWarning, "poke"},
		{LevelCritical, "kill"},
		{LevelUnknown, "shinobu"},
	}

	for _, tt := range tests {
		got := sel.SelectCategory(tt.level)
		if got != tt.expected {
			t.Errorf("SelectCategory(%v) with custom mapping = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

func TestSelectCategory_DeterministicWithLastRand(t *testing.T) {
	cfg := DefaultSelectorConfig()
	cfg.RandFunc = lastRand
	sel := NewSelector(cfg)

	// Should select last element from each list.
	healthyList := cfg.Mapping.Healthy
	expected := healthyList[len(healthyList)-1]

	got := sel.SelectCategory(LevelHealthy)
	if got != expected {
		t.Errorf("SelectCategory(LevelHealthy) with lastRand = %q, want %q", got, expected)
	}
}

func TestSelectCategory_AllReturnedCategoriesAreValid(t *testing.T) {
	cfg := DefaultSelectorConfig()
	sel := NewSelector(cfg)

	levels := []Level{LevelHealthy, LevelWarning, LevelCritical, LevelUnknown}
	for _, level := range levels {
		// Run multiple times with default (real) randomness.
		for i := 0; i < 50; i++ {
			got := sel.SelectCategory(level)
			if got == "" {
				t.Fatalf("SelectCategory(%v) returned empty string on iteration %d", level, i)
			}
			if !waifu.IsValidCategory(got) {
				t.Errorf("SelectCategory(%v) returned invalid category %q on iteration %d", level, got, i)
			}
		}
	}
}

func TestNewSelector_NilRandFunc(t *testing.T) {
	cfg := SelectorConfig{
		Mapping: DefaultCategoryMapping(),
		// RandFunc intentionally nil.
	}
	sel := NewSelector(cfg)

	// Should not panic; the constructor provides a default RandFunc.
	got := sel.SelectCategory(LevelHealthy)
	if got == "" {
		t.Error("SelectCategory returned empty string with nil RandFunc")
	}
	if !waifu.IsValidCategory(got) {
		t.Errorf("SelectCategory returned invalid category %q with nil RandFunc", got)
	}
}

func TestSelectCategory_FallbackCategoryIsValid(t *testing.T) {
	if !waifu.IsValidCategory(fallbackCategory) {
		t.Errorf("fallbackCategory %q is not a valid waifu category", fallbackCategory)
	}
}

func TestDefaultCategoryMapping_NoDuplicatesPerLevel(t *testing.T) {
	m := DefaultCategoryMapping()

	checkDuplicates := func(name string, cats []string) {
		seen := make(map[string]bool)
		for _, c := range cats {
			if seen[c] {
				t.Errorf("DefaultCategoryMapping().%s has duplicate category %q", name, c)
			}
			seen[c] = true
		}
	}

	checkDuplicates("Healthy", m.Healthy)
	checkDuplicates("Warning", m.Warning)
	checkDuplicates("Critical", m.Critical)
	checkDuplicates("Unknown", m.Unknown)
}

func TestSelectCategory_OverrideEmptyString_UsesLevelBased(t *testing.T) {
	cfg := DefaultSelectorConfig()
	cfg.OverrideCategory = ""
	cfg.RandFunc = deterministicRand
	sel := NewSelector(cfg)

	got := sel.SelectCategory(LevelHealthy)
	if got != cfg.Mapping.Healthy[0] {
		t.Errorf("SelectCategory with empty override = %q, want %q", got, cfg.Mapping.Healthy[0])
	}
}
