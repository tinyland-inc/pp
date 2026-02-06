package layout

import (
	"strconv"
	"strings"
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// TestLayoutModeString verifies layout mode string representations.
func TestLayoutModeString(t *testing.T) {
	tests := []struct {
		mode LayoutMode
		want string
	}{
		{LayoutCompact, "compact"},
		{LayoutStandard, "standard"},
		{LayoutWide, "wide"},
		{LayoutUltraWide, "ultra-wide"},
		{LayoutMode(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.mode.String()
			if got != tt.want {
				t.Errorf("LayoutMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

// TestDetectLayoutMode verifies correct layout mode detection for various dimensions.
func TestDetectLayoutMode(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		want   LayoutMode
	}{
		// Ultra-wide threshold tests (MinWidth: 200, MinHeight: 50).
		{"ultra-wide exact", 200, 50, LayoutUltraWide},
		{"ultra-wide larger", 250, 100, LayoutUltraWide},
		{"ultra-wide height too small", 200, 49, LayoutWide},

		// Wide threshold tests (MinWidth: 160, MinHeight: 35).
		{"wide exact", 160, 35, LayoutWide},
		{"wide larger width", 199, 49, LayoutWide},
		{"wide height too small", 160, 34, LayoutStandard},

		// Standard threshold tests (MinWidth: 120, MinHeight: 24).
		{"standard exact", 120, 24, LayoutStandard},
		{"standard larger width", 159, 34, LayoutStandard},
		{"standard height too small", 120, 23, LayoutCompact},

		// Compact threshold tests.
		{"compact exact", 80, 24, LayoutCompact},
		{"compact smaller", 60, 20, LayoutCompact},
		{"compact tiny", 40, 10, LayoutCompact},

		// Edge cases.
		{"zero dimensions default to compact", 0, 0, LayoutCompact},
		{"negative dimensions default to compact", -1, -1, LayoutCompact},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectLayoutMode(tt.width, tt.height)
			if got != tt.want {
				t.Errorf("DetectLayoutMode(%d, %d) = %v, want %v",
					tt.width, tt.height, got, tt.want)
			}
		})
	}
}

// TestNewResponsiveConfig verifies configuration generation for each mode.
func TestNewResponsiveConfig(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		wantMode   LayoutMode
		wantImage  bool
		wantSpark  bool
		wantVStack bool
	}{
		{
			name:       "compact mode",
			width:      80,
			height:     24,
			wantMode:   LayoutCompact,
			wantImage:  false,
			wantSpark:  false,
			wantVStack: true,
		},
		{
			name:       "standard mode",
			width:      120,
			height:     24,
			wantMode:   LayoutStandard,
			wantImage:  true,
			wantSpark:  true,
			wantVStack: false,
		},
		{
			name:       "wide mode",
			width:      160,
			height:     35,
			wantMode:   LayoutWide,
			wantImage:  true,
			wantSpark:  true,
			wantVStack: false,
		},
		{
			name:       "ultra-wide mode",
			width:      200,
			height:     50,
			wantMode:   LayoutUltraWide,
			wantImage:  true,
			wantSpark:  true,
			wantVStack: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewResponsiveConfig(tt.width, tt.height)

			if cfg.Mode != tt.wantMode {
				t.Errorf("Mode = %v, want %v", cfg.Mode, tt.wantMode)
			}
			if cfg.Features.ShowImage != tt.wantImage {
				t.Errorf("ShowImage = %v, want %v", cfg.Features.ShowImage, tt.wantImage)
			}
			if cfg.Features.ShowSparklines != tt.wantSpark {
				t.Errorf("ShowSparklines = %v, want %v", cfg.Features.ShowSparklines, tt.wantSpark)
			}
			if cfg.Features.VerticalStack != tt.wantVStack {
				t.Errorf("VerticalStack = %v, want %v", cfg.Features.VerticalStack, tt.wantVStack)
			}
			if cfg.TermWidth != tt.width {
				t.Errorf("TermWidth = %d, want %d", cfg.TermWidth, tt.width)
			}
			if cfg.TermHeight != tt.height {
				t.Errorf("TermHeight = %d, want %d", cfg.TermHeight, tt.height)
			}
		})
	}
}

// TestColumnsForMode verifies column width allocation for each mode.
func TestColumnsForMode(t *testing.T) {
	tests := []struct {
		name          string
		mode          LayoutMode
		termWidth     int
		wantImageCols int
		wantMainCols  int
	}{
		{
			name:          "compact no image",
			mode:          LayoutCompact,
			termWidth:     80,
			wantImageCols: 0,
			wantMainCols:  76, // 80 - 4 for borders
		},
		{
			name:          "standard with image",
			mode:          LayoutStandard,
			termWidth:     120,
			wantImageCols: 28, // Matches GetWaifuSize() for Standard (28x14)
			wantMainCols:  66, // 120 - 28 - 20 - 6
		},
		{
			name:          "wide 3-column",
			mode:          LayoutWide,
			termWidth:     160,
			wantImageCols: 36, // Matches GetWaifuSize() for Wide (36x18)
			wantMainCols:  50,
		},
		{
			name:          "ultra-wide 4-column",
			mode:          LayoutUltraWide,
			termWidth:     200,
			wantImageCols: 48, // Matches GetWaifuSize() for UltraWide (48x24)
			wantMainCols:  50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols := columnsForMode(tt.mode, tt.termWidth)

			if cols.ImageCols != tt.wantImageCols {
				t.Errorf("ImageCols = %d, want %d", cols.ImageCols, tt.wantImageCols)
			}
			if cols.MainCols != tt.wantMainCols {
				t.Errorf("MainCols = %d, want %d", cols.MainCols, tt.wantMainCols)
			}
		})
	}
}

// TestFeaturesForMode verifies feature flags for each mode.
func TestFeaturesForMode(t *testing.T) {
	tests := []struct {
		mode         LayoutMode
		wantImage    bool
		wantSpark    bool
		wantMetrics  bool
		wantNode     bool
		wantVStack   bool
		wantBorders  bool
	}{
		{LayoutCompact, false, false, false, false, true, false},
		{LayoutStandard, true, true, true, true, false, true},
		{LayoutWide, true, true, true, true, false, true},
		{LayoutUltraWide, true, true, true, true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			f := featuresForMode(tt.mode)

			if f.ShowImage != tt.wantImage {
				t.Errorf("ShowImage = %v, want %v", f.ShowImage, tt.wantImage)
			}
			if f.ShowSparklines != tt.wantSpark {
				t.Errorf("ShowSparklines = %v, want %v", f.ShowSparklines, tt.wantSpark)
			}
			if f.ShowFullMetrics != tt.wantMetrics {
				t.Errorf("ShowFullMetrics = %v, want %v", f.ShowFullMetrics, tt.wantMetrics)
			}
			if f.ShowNodeMetrics != tt.wantNode {
				t.Errorf("ShowNodeMetrics = %v, want %v", f.ShowNodeMetrics, tt.wantNode)
			}
			if f.VerticalStack != tt.wantVStack {
				t.Errorf("VerticalStack = %v, want %v", f.VerticalStack, tt.wantVStack)
			}
			if f.ShowBorders != tt.wantBorders {
				t.Errorf("ShowBorders = %v, want %v", f.ShowBorders, tt.wantBorders)
			}
		})
	}
}

// TestRenderCompact verifies compact mode rendering.
func TestRenderCompact(t *testing.T) {
	cfg := NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false // Disable colors for test comparison.
	layout := NewResponsiveLayout(cfg)

	sections := []Section{
		{Title: "Claude", Content: []string{"personal: 45% (5h)", "work: 80% (5h)"}},
		{Title: "Billing", Content: []string{"$142 this month"}},
	}

	result := layout.Render("", sections, nil)

	// Should contain section titles.
	if !strings.Contains(result.Output, "Claude") {
		t.Error("compact output missing Claude section")
	}
	if !strings.Contains(result.Output, "Billing") {
		t.Error("compact output missing Billing section")
	}

	// Should not contain image separator.
	if strings.Contains(result.Output, " "+string(boxVertical)+" ") {
		t.Error("compact output should not have column separator")
	}

	// Should fit within terminal height.
	lines := strings.Split(result.Output, "\n")
	if len(lines) > 24 {
		t.Errorf("compact output has %d lines, expected <= 24", len(lines))
	}
}

// TestRenderStandard verifies standard mode rendering with image.
func TestRenderStandard(t *testing.T) {
	cfg := NewResponsiveConfig(120, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	imageContent := fakeImage(20, 10)
	sections := []Section{
		{Title: "Claude", Content: []string{"personal: 45% (5h)"}},
		{Title: "Billing", Content: []string{"$142 this month"}},
	}

	result := layout.Render(imageContent, sections, nil)

	// Should contain section titles.
	if !strings.Contains(result.Output, "Claude") {
		t.Error("standard output missing Claude section")
	}
	if !strings.Contains(result.Output, "Billing") {
		t.Error("standard output missing Billing section")
	}

	// Should contain column separator.
	if !strings.Contains(result.Output, " "+string(boxVertical)+" ") {
		t.Error("standard output missing column separator")
	}

	// Should contain image content.
	if !strings.Contains(result.Output, "XXXX") {
		t.Error("standard output missing image content")
	}
}

// TestRenderStandardNoImage verifies standard mode without image.
func TestRenderStandardNoImage(t *testing.T) {
	cfg := NewResponsiveConfig(120, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	sections := []Section{
		{Title: "Claude", Content: []string{"personal: 45% (5h)"}},
	}

	result := layout.Render("", sections, nil)

	// Should contain section content.
	if !strings.Contains(result.Output, "Claude") {
		t.Error("standard output missing Claude section")
	}

	// Standard mode now uses 3-column layout with sparklines,
	// so without an image, info is shown in multi-column format.
	// Should contain sparkline Trends section (with no data).
	if !strings.Contains(result.Output, "Trends") {
		t.Error("standard output should contain Trends sparkline section")
	}
}

// TestRenderWide verifies wide mode 3-column rendering.
func TestRenderWide(t *testing.T) {
	cfg := NewResponsiveConfig(160, 35)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	imageContent := fakeImage(22, 15)
	sections := []Section{
		{Title: "Claude", Content: []string{"personal: 45% (5h)"}},
		{Title: "Billing", Content: []string{"$142 this month"}},
		{Title: "Infrastructure", Content: []string{"ts: 4/5 online"}},
	}

	result := layout.Render(imageContent, sections, nil)

	// Should contain all section titles.
	for _, title := range []string{"Claude", "Billing", "Infrastructure"} {
		if !strings.Contains(result.Output, title) {
			t.Errorf("wide output missing %s section", title)
		}
	}

	// Should contain column separators (at least 2 for 3-column).
	sepCount := strings.Count(result.Output, " "+string(boxVertical)+" ")
	if sepCount < 2 {
		t.Errorf("wide output has %d separators, expected >= 2", sepCount)
	}
}

// TestRenderUltraWide verifies ultra-wide mode 4-column rendering.
func TestRenderUltraWide(t *testing.T) {
	cfg := NewResponsiveConfig(200, 50)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	imageContent := fakeImage(22, 15)
	sections := []Section{
		{Title: "Claude", Content: []string{"personal: 45% (5h)"}},
		{Title: "Billing", Content: []string{"$142 this month"}},
		{Title: "Infrastructure", Content: []string{"ts: 4/5 online"}},
	}

	result := layout.Render(imageContent, sections, nil)

	// Should contain sparkline section (with nil billing, shows placeholder).
	if !strings.Contains(result.Output, "Trends") {
		t.Error("ultra-wide output missing sparkline section")
	}

	// With nil billing, should show "(no data)".
	if !strings.Contains(result.Output, "(no data)") {
		t.Error("ultra-wide output with nil billing should show (no data)")
	}

	// Should have 4-column layout (3 separators per line).
	lines := strings.Split(result.Output, "\n")
	if len(lines) > 0 {
		firstLine := lines[0]
		sepCount := strings.Count(firstLine, " "+string(boxVertical)+" ")
		if sepCount < 3 {
			t.Errorf("ultra-wide first line has %d separators, expected 3", sepCount)
		}
	}
}

// TestRenderTruncation verifies content truncation to terminal height.
func TestRenderTruncation(t *testing.T) {
	cfg := NewResponsiveConfig(80, 10) // Small height.
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	// Create many sections that would exceed height.
	sections := []Section{
		{Title: "Section 1", Content: []string{"line1", "line2", "line3"}},
		{Title: "Section 2", Content: []string{"line1", "line2", "line3"}},
		{Title: "Section 3", Content: []string{"line1", "line2", "line3"}},
		{Title: "Section 4", Content: []string{"line1", "line2", "line3"}},
	}

	result := layout.Render("", sections, nil)

	lines := strings.Split(result.Output, "\n")
	if len(lines) > 10 {
		t.Errorf("output has %d lines, expected <= 10", len(lines))
	}
	if !result.Truncated {
		t.Error("Truncated should be true when content exceeds height")
	}
}

// TestStatusIndicator verifies color-coded status indicator rendering.
func TestStatusIndicator(t *testing.T) {
	tests := []struct {
		status     string
		wantIcon   string
		wantStatus string
	}{
		{"healthy", "●", "healthy"},
		{"warning", "●", "warning"},
		{"critical", "●", "critical"},
		{"unknown", "○", "unknown"},
		{"invalid", "○", "invalid"},
	}

	cfg := NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := layout.StatusIndicator(tt.status)
			if !strings.Contains(result, tt.wantIcon) {
				t.Errorf("StatusIndicator(%q) missing icon %q", tt.status, tt.wantIcon)
			}
			if !strings.Contains(result, tt.wantStatus) {
				t.Errorf("StatusIndicator(%q) missing status %q", tt.status, tt.wantStatus)
			}
		})
	}
}

// TestRenderBox verifies Unicode box rendering.
func TestRenderBox(t *testing.T) {
	cfg := NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	lines := []string{"Line 1", "Line 2", "Line 3"}
	result := layout.RenderBox(lines, 40, "Title")

	// Should contain box corners.
	if !strings.Contains(result, string(boxTopLeft)) {
		t.Error("box missing top-left corner")
	}
	if !strings.Contains(result, string(boxTopRight)) {
		t.Error("box missing top-right corner")
	}
	if !strings.Contains(result, string(boxBottomLeft)) {
		t.Error("box missing bottom-left corner")
	}
	if !strings.Contains(result, string(boxBottomRight)) {
		t.Error("box missing bottom-right corner")
	}

	// Should contain title.
	if !strings.Contains(result, "Title") {
		t.Error("box missing title")
	}

	// Should contain content.
	if !strings.Contains(result, "Line 1") {
		t.Error("box missing content")
	}
}

// TestRenderBoxNoTitle verifies box rendering without title.
func TestRenderBoxNoTitle(t *testing.T) {
	cfg := NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	lines := []string{"Content"}
	result := layout.RenderBox(lines, 40, "")

	// Should have continuous top border (no title gap).
	firstLine := strings.Split(result, "\n")[0]
	horizCount := strings.Count(firstLine, string(boxHorizontal))
	if horizCount < 38 { // 40 - 2 for corners
		t.Errorf("top border has %d horizontal chars, expected >= 38", horizCount)
	}
}

// TestVisibleLen verifies ANSI-aware string length calculation.
func TestVisibleLen(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"plain text", "hello", 5},
		{"empty", "", 0},
		{"ansi color", "\x1b[31mred\x1b[0m", 3},
		{"mixed ansi", "pre\x1b[32mgreen\x1b[0mpost", 12},
		{"bold", "\x1b[1mbold\x1b[0m", 4},
		{"multiple escapes", "\x1b[1;31;40mtext\x1b[0m", 4},
		{"tilde terminator", "\x1b[?25h", 0}, // Cursor show.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleLen(tt.s)
			if got != tt.want {
				t.Errorf("visibleLen(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

// TestTruncateToWidth verifies ANSI-aware string truncation.
func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{"plain shorter", "hello", 10, "hello"},
		{"plain exact", "hello", 5, "hello"},
		{"plain longer", "hello world", 5, "hello"},
		{"empty", "", 5, ""},
		{"zero width", "hello", 0, ""},
		{"ansi preserved", "\x1b[31mred\x1b[0m", 2, "\x1b[31mre\x1b[0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateToWidth(tt.s, tt.width)
			// For ANSI strings, just verify visible length.
			gotVisible := visibleLen(got)
			wantVisible := visibleLen(tt.want)
			if gotVisible != wantVisible {
				t.Errorf("truncateToWidth(%q, %d) visible len = %d, want %d",
					tt.s, tt.width, gotVisible, wantVisible)
			}
		})
	}
}

// TestPadToWidth verifies string padding.
func TestPadToWidth(t *testing.T) {
	tests := []struct {
		name  string
		s     string
		width int
		want  int // Expected visible length.
	}{
		{"shorter", "hello", 10, 10},
		{"exact", "hello", 5, 5},
		{"longer no change", "hello world", 5, 11},
		{"empty", "", 5, 5},
		{"with ansi", "\x1b[31mred\x1b[0m", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := padToWidth(tt.s, tt.width)
			gotVisible := visibleLen(got)
			if gotVisible != tt.want {
				t.Errorf("padToWidth(%q, %d) visible len = %d, want %d",
					tt.s, tt.width, gotVisible, tt.want)
			}
		})
	}
}

// TestComposeSideBySideAlignment verifies column alignment.
func TestComposeSideBySideAlignment(t *testing.T) {
	cfg := NewResponsiveConfig(120, 24)
	cfg.ColorEnabled = false
	cfg.Columns.ImageCols = 10
	layout := NewResponsiveLayout(cfg)

	imageContent := "AAAA\nBBBB\nCCCC"
	sections := []Section{
		{Title: "Test", Content: []string{"line1", "line2", "line3"}},
	}

	result := layout.Render(imageContent, sections, nil)
	lines := strings.Split(result.Output, "\n")

	// Each line should have image padded to ImageCols + separator.
	for i, line := range lines {
		if !strings.Contains(line, " "+string(boxVertical)+" ") {
			t.Errorf("line %d missing separator: %q", i, line)
		}
	}
}

// TestComposeSideBySideImageTaller verifies handling when image is taller.
func TestComposeSideBySideImageTaller(t *testing.T) {
	cfg := NewResponsiveConfig(120, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	imageContent := fakeImage(10, 20) // 20 lines tall.
	sections := []Section{
		{Title: "Test", Content: []string{"line1", "line2"}},
	}

	result := layout.Render(imageContent, sections, nil)
	lines := strings.Split(result.Output, "\n")

	// Should have at least as many lines as the image.
	if len(lines) < 20 {
		t.Errorf("output has %d lines, expected >= 20 (image height)", len(lines))
	}
}

// TestComposeSideBySideInfoTaller verifies handling when info is taller.
func TestComposeSideBySideInfoTaller(t *testing.T) {
	cfg := NewResponsiveConfig(120, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	imageContent := fakeImage(10, 5) // 5 lines tall.
	sections := []Section{
		{Title: "Test", Content: make([]string, 15)}, // Many lines.
	}

	result := layout.Render(imageContent, sections, nil)
	lines := strings.Split(result.Output, "\n")

	// Should have at least as many lines as the info panel.
	if len(lines) < 15 {
		t.Errorf("output has %d lines, expected >= 15 (info height)", len(lines))
	}
}

// TestGracefulDegradation verifies layout gracefully handles tiny terminals.
func TestGracefulDegradation(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"very small", 40, 10},
		{"minimal", 20, 5},
		{"zero", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic.
			cfg := NewResponsiveConfig(tt.width, tt.height)
			layout := NewResponsiveLayout(cfg)

			sections := []Section{
				{Title: "Test", Content: []string{"content"}},
			}

			result := layout.Render("", sections, nil)

			// Should produce some output.
			if result.Output == "" {
				t.Error("graceful degradation should produce output")
			}
		})
	}
}

// TestColorEnabled verifies color can be disabled.
func TestColorEnabled(t *testing.T) {
	cfg := NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	sections := []Section{
		{Title: "Test", Content: []string{"content"}},
	}

	result := layout.Render("", sections, nil)

	// Should not contain ANSI escape codes.
	if strings.Contains(result.Output, "\x1b[") {
		t.Error("color disabled output should not contain ANSI escapes")
	}
}

// TestColorEnabledWithColors verifies color flag is set when enabled.
// Note: Actual ANSI output depends on terminal detection by lipgloss/termenv,
// so in test environments without a TTY, colors may not be emitted.
func TestColorEnabledWithColors(t *testing.T) {
	cfg := NewResponsiveConfig(120, 24)
	cfg.ColorEnabled = true
	layout := NewResponsiveLayout(cfg)

	// Verify the color flag is set correctly in the config.
	if !layout.Config().ColorEnabled {
		t.Error("ColorEnabled should be true in config")
	}

	sections := []Section{
		{Title: "Test", Content: []string{"content"}},
	}

	result := layout.Render("", sections, nil)

	// Output should be non-empty regardless of color support.
	if result.Output == "" {
		t.Error("output should not be empty")
	}

	// Should contain the content.
	if !strings.Contains(result.Output, "Test") {
		t.Error("output should contain section title")
	}
}

// TestConfigRetrieval verifies config can be retrieved from layout.
func TestConfigRetrieval(t *testing.T) {
	cfg := NewResponsiveConfig(120, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	retrieved := layout.Config()
	if retrieved.Mode != cfg.Mode {
		t.Errorf("Config().Mode = %v, want %v", retrieved.Mode, cfg.Mode)
	}
	if retrieved.TermWidth != cfg.TermWidth {
		t.Errorf("Config().TermWidth = %d, want %d", retrieved.TermWidth, cfg.TermWidth)
	}
}

// TestOutputFitsWithin80Columns verifies compact mode fits in 80 columns.
func TestOutputFitsWithin80Columns(t *testing.T) {
	cfg := NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	sections := []Section{
		{Title: "Claude", Content: []string{"personal: 45% (5h)", "work: 80% (5h)"}},
		{Title: "Billing", Content: []string{"$142 this month"}},
	}

	result := layout.Render("", sections, nil)
	lines := strings.Split(result.Output, "\n")

	for i, line := range lines {
		vLen := visibleLen(line)
		if vLen > 80 {
			t.Errorf("line %d exceeds 80 cols (visible len %d): %q", i, vLen, line)
		}
	}
}

// TestOutputFitsWithin24Rows verifies compact mode fits in 24 rows.
func TestOutputFitsWithin24Rows(t *testing.T) {
	cfg := NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	// Create content that would exceed 24 lines.
	sections := []Section{
		{Title: "Section 1", Content: make([]string, 10)},
		{Title: "Section 2", Content: make([]string, 10)},
		{Title: "Section 3", Content: make([]string, 10)},
	}

	result := layout.Render("", sections, nil)
	lines := strings.Split(result.Output, "\n")

	if len(lines) > 24 {
		t.Errorf("output exceeds 24 rows: got %d rows", len(lines))
	}
}

// TestBuildInfoPanel verifies info panel construction.
func TestBuildInfoPanel(t *testing.T) {
	cfg := NewResponsiveConfig(120, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	sections := []Section{
		{Title: "Claude", Content: []string{"account1", "account2"}},
		{Title: "Billing", Content: []string{"total"}},
	}

	lines := layout.buildInfoPanel(sections)

	// Should contain section titles.
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Claude") {
		t.Error("info panel missing Claude title")
	}
	if !strings.Contains(joined, "Billing") {
		t.Error("info panel missing Billing title")
	}

	// Should contain indented content.
	if !strings.Contains(joined, "  account1") {
		t.Error("info panel missing indented content")
	}
}

// fakeImage generates a test image content string.
func fakeImage(width, height int) string {
	var lines []string
	row := strings.Repeat("X", width)
	for i := 0; i < height; i++ {
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// TestMax verifies max helper function.
func TestMax(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 2},
		{2, 1, 2},
		{0, 0, 0},
		{-1, -2, -1},
		{100, 100, 100},
	}

	for _, tt := range tests {
		got := max(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("max(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// TestLayoutConfigDefaults verifies default configuration values.
func TestLayoutConfigDefaults(t *testing.T) {
	cfg := NewResponsiveConfig(80, 24)

	// ColorEnabled should default to true.
	if !cfg.ColorEnabled {
		t.Error("ColorEnabled should default to true")
	}

	// Compact mode should have vertical stack.
	if !cfg.Features.VerticalStack {
		t.Error("Compact mode should use vertical stack")
	}

	// Compact mode should not show image.
	if cfg.Features.ShowImage {
		t.Error("Compact mode should not show image")
	}
}

// TestColumnSeparator verifies column separator rendering.
func TestColumnSeparator(t *testing.T) {
	cfg := NewResponsiveConfig(120, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	sep := layout.columnSeparator()

	// Should contain vertical bar.
	if !strings.Contains(sep, string(boxVertical)) {
		t.Error("separator missing vertical bar character")
	}

	// Should have spaces around it.
	if sep[0] != ' ' || sep[len(sep)-1] != ' ' {
		t.Error("separator should have space padding")
	}
}

// TestSectionTitle verifies section title rendering.
func TestSectionTitle(t *testing.T) {
	cfg := NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	title := layout.sectionTitle("Test Title")

	if title != "Test Title" {
		t.Errorf("sectionTitle = %q, want %q", title, "Test Title")
	}
}

// TestSectionTitleWithColor verifies colored section title flag handling.
// Note: Actual ANSI output depends on terminal detection by lipgloss/termenv,
// so in test environments without a TTY, colors may not be emitted.
func TestSectionTitleWithColor(t *testing.T) {
	cfg := NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = true
	layout := NewResponsiveLayout(cfg)

	title := layout.sectionTitle("Test Title")

	// Should still contain the title text regardless of terminal support.
	if !strings.Contains(title, "Test Title") {
		t.Error("colored section title should contain title text")
	}

	// Verify that the config correctly reflects color enabled.
	if !layout.Config().ColorEnabled {
		t.Error("ColorEnabled should be true in config")
	}
}

// TestUltraWideMode_WithBillingData verifies sparklines render with real data.
func TestUltraWideMode_WithBillingData(t *testing.T) {
	cfg := NewResponsiveConfig(200, 50)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	imageContent := fakeImage(22, 15)
	sections := []Section{
		{Title: "Claude", Content: []string{"personal: 45% (5h)"}},
		{Title: "Billing", Content: []string{"$142 this month"}},
		{Title: "Infrastructure", Content: []string{"ts: 4/5 online"}},
	}

	// Create billing data with history
	history := make([]collectors.DailySpend, 30)
	for i := 0; i < 30; i++ {
		history[i] = collectors.DailySpend{
			Date:     "2026-01-" + strconv.Itoa(i+1),
			SpendUSD: float64(100 + i*5),
		}
	}
	billing := &collectors.BillingData{
		History: &collectors.BillingHistory{
			TotalHistory: history,
		},
	}

	result := layout.Render(imageContent, sections, billing)

	// Should contain Trends section
	if !strings.Contains(result.Output, "Trends") {
		t.Error("ultra-wide output with billing data missing Trends section")
	}

	// Should contain actual sparkline characters (▁▂▃▄▅▆▇█)
	hasSparklineChars := false
	for _, r := range result.Output {
		if r >= '▁' && r <= '█' {
			hasSparklineChars = true
			break
		}
	}
	if !hasSparklineChars {
		t.Error("ultra-wide output with billing data should contain sparkline characters (▁▂▃▄▅▆▇█)")
	}

	// Should NOT contain placeholder text
	if strings.Contains(result.Output, "[sparkline]") {
		t.Error("ultra-wide output should not contain [sparkline] placeholder with real data")
	}
}
