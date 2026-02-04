package status

import (
	"math/rand/v2"

	"gitlab.com/tinyland/lab/prompt-pulse/waifu"
)

// CategoryMapping maps a status Level to a list of waifu.pics SFW categories.
// When selecting an image, a random category from the list is chosen.
type CategoryMapping struct {
	Healthy  []string
	Warning  []string
	Critical []string
	Unknown  []string
}

// DefaultCategoryMapping returns the default mood-to-category mapping.
func DefaultCategoryMapping() CategoryMapping {
	return CategoryMapping{
		Healthy:  []string{"happy", "smile", "wave", "dance", "wink", "highfive"},
		Warning:  []string{"smug", "blush", "poke", "nom", "pat"},
		Critical: []string{"cry", "bonk", "slap", "bite", "kill"},
		Unknown:  []string{"waifu", "neko", "shinobu", "megumin"},
	}
}

// SelectorConfig configures waifu category selection behavior.
type SelectorConfig struct {
	// Mapping overrides the default category-to-mood mapping.
	Mapping CategoryMapping
	// OverrideCategory forces a specific category regardless of status.
	// If empty, selection is based on status level.
	OverrideCategory string
	// RandFunc provides the random index for category selection.
	// If nil, a default implementation is used. This allows deterministic testing.
	RandFunc func(n int) int
}

// DefaultSelectorConfig returns a SelectorConfig with default mappings.
func DefaultSelectorConfig() SelectorConfig {
	return SelectorConfig{
		Mapping:  DefaultCategoryMapping(),
		RandFunc: rand.IntN,
	}
}

// Selector chooses waifu.pics categories based on system status level.
type Selector struct {
	config SelectorConfig
}

// NewSelector creates a Selector with the given configuration.
// If RandFunc is nil, a default random implementation is used.
func NewSelector(cfg SelectorConfig) *Selector {
	if cfg.RandFunc == nil {
		cfg.RandFunc = rand.IntN
	}
	return &Selector{config: cfg}
}

// fallbackCategory is returned when no valid category can be determined.
const fallbackCategory = "waifu"

// SelectCategory picks a waifu.pics category appropriate for the given status Level.
// If OverrideCategory is set and valid, it is always returned.
// Otherwise, picks a random category from the appropriate Level's mapping list.
// Never returns an empty string; falls back to "waifu" if all else fails.
func (s *Selector) SelectCategory(level Level) string {
	// If an override is set and valid, use it directly.
	if s.config.OverrideCategory != "" {
		if waifu.IsValidCategory(s.config.OverrideCategory) {
			return s.config.OverrideCategory
		}
		// Invalid override: fall through to level-based selection.
	}

	candidates := s.validCategoriesForLevel(level)
	if len(candidates) == 0 {
		return fallbackCategory
	}

	idx := s.config.RandFunc(len(candidates))
	return candidates[idx]
}

// CategoriesForLevel returns all candidate categories for the given level.
func (s *Selector) CategoriesForLevel(level Level) []string {
	return s.rawCategoriesForLevel(level)
}

// rawCategoriesForLevel returns the raw (unfiltered) category list for a level.
func (s *Selector) rawCategoriesForLevel(level Level) []string {
	switch level {
	case LevelHealthy:
		return s.config.Mapping.Healthy
	case LevelWarning:
		return s.config.Mapping.Warning
	case LevelCritical:
		return s.config.Mapping.Critical
	case LevelUnknown:
		return s.config.Mapping.Unknown
	default:
		return s.config.Mapping.Unknown
	}
}

// validCategoriesForLevel returns only categories that pass waifu.IsValidCategory.
func (s *Selector) validCategoriesForLevel(level Level) []string {
	raw := s.rawCategoriesForLevel(level)
	valid := make([]string, 0, len(raw))
	for _, c := range raw {
		if waifu.IsValidCategory(c) {
			valid = append(valid, c)
		}
	}
	return valid
}
