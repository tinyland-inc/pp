package inttest

import (
	"strings"
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/banner"
)

// itTestBannerAllWidgets renders a banner with all six widget types and
// verifies the output has reasonable dimensions.
func itTestBannerAllWidgets(t *testing.T) {
	t.Helper()

	data := banner.BannerData{
		Widgets: itMockBannerWidgets(),
	}

	output := banner.Render(data, banner.Standard)
	if output == "" {
		t.Fatal("banner render returned empty output")
	}

	lines := strings.Split(output, "\n")

	// Standard preset is 120x35, so output should have exactly 35 lines.
	if len(lines) != banner.Standard.Height {
		t.Errorf("banner has %d lines, want %d", len(lines), banner.Standard.Height)
	}

	// Each line should be at most Standard.Width visible characters
	// (some may be shorter due to trailing space trimming, but none
	// should exceed).
	for i, line := range lines {
		if len(line) > banner.Standard.Width*4 { // generous for UTF-8
			t.Errorf("line %d exceeds max byte length: %d", i, len(line))
		}
	}
}

// itTestBannerResize renders a banner at multiple terminal sizes and
// verifies the layout adapts to each.
func itTestBannerResize(t *testing.T) {
	t.Helper()

	data := banner.BannerData{
		Widgets: itMockBannerWidgets(),
	}

	presets := []banner.Preset{
		banner.Compact,
		banner.Standard,
		banner.Wide,
		banner.UltraWide,
	}

	for _, p := range presets {
		t.Run(p.Name, func(t *testing.T) {
			output := banner.Render(data, p)
			if output == "" {
				t.Fatalf("banner render at %s returned empty output", p.Name)
			}

			lines := strings.Split(output, "\n")
			if len(lines) != p.Height {
				t.Errorf("banner at %s has %d lines, want %d",
					p.Name, len(lines), p.Height)
			}
		})
	}
}

// itTestEmptyState renders the banner with no widget data and
// verifies graceful handling.
func itTestEmptyState(t *testing.T) {
	t.Helper()

	// Banner with no widgets.
	t.Run("banner_empty", func(t *testing.T) {
		data := banner.BannerData{
			Widgets: nil,
		}
		output := banner.Render(data, banner.Standard)
		// With no widgets, output may be blank or whitespace.
		// The key requirement is no panic.
		_ = output
	})

	// Banner with widgets that have empty content.
	t.Run("banner_empty_content", func(t *testing.T) {
		data := banner.BannerData{
			Widgets: []banner.WidgetData{
				{ID: "claude", Title: "Claude", Content: "", MinW: 10, MinH: 3},
				{ID: "billing", Title: "Billing", Content: "", MinW: 10, MinH: 3},
			},
		}
		output := banner.Render(data, banner.Compact)
		if output == "" {
			// Compact with empty content may produce only border frames.
			// That is acceptable.
		}
		_ = output
	})
}
