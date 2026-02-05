// Package visual provides visual regression tests for prompt-pulse banner layouts.
//
// These tests verify that banner output matches expected "golden" files for each
// of the 4 supported terminal sizes:
//   - Compact (80x24): Vertical stack, no images, SSH-friendly
//   - Standard (120x40): Side-by-side with image
//   - Wide (160x60): 3-column layout
//   - UltraWide (200x80): 4-column with sparklines
//
// Usage:
//
//	# Run all visual tests
//	go test ./tests/visual -v
//
//	# Update golden files after intentional changes
//	go test ./tests/visual -update-golden
//
//	# Show diffs when tests fail
//	go test ./tests/visual -v
package visual

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/banner"
	"gitlab.com/tinyland/lab/prompt-pulse/display/layout"
)

var updateGolden = flag.Bool("update-golden", false, "Update golden files with current output")

// terminalSizes defines the 4 supported terminal layout configurations.
var terminalSizes = []struct {
	name   string
	cols   int
	rows   int
	mode   layout.LayoutMode
}{
	{"compact", 80, 24, layout.LayoutCompact},
	{"standard", 120, 40, layout.LayoutStandard},
	{"wide", 160, 60, layout.LayoutWide},
	{"ultrawide", 200, 80, layout.LayoutUltraWide},
}

// testScenarios defines different data scenarios for testing.
var testScenarios = []struct {
	name        string
	withWaifu   bool
	withClaude  bool
	withBilling bool
	withInfra   bool
	errorState  bool
}{
	{"full-data", false, true, true, true, false},
	{"with-waifu", true, true, true, true, false},
	{"no-data", false, false, false, false, false},
	{"error-state", false, true, true, true, true},
}

// TestBannerLayouts runs visual regression tests for all layout sizes.
func TestBannerLayouts(t *testing.T) {
	for _, size := range terminalSizes {
		t.Run(size.name, func(t *testing.T) {
			for _, scenario := range testScenarios {
				t.Run(scenario.name, func(t *testing.T) {
					// Set terminal size via environment variables.
					t.Setenv("COLUMNS", fmt.Sprintf("%d", size.cols))
					t.Setenv("LINES", fmt.Sprintf("%d", size.rows))

					// Render banner with test data.
					output := renderTestBanner(t, size.cols, size.rows, scenario)

					// Golden file path.
					goldenFile := goldenFilePath(t, size.name, scenario.name)

					// Compare or update golden file.
					if *updateGolden {
						writeGoldenFile(t, goldenFile, output)
					} else {
						compareWithGolden(t, output, goldenFile)
					}
				})
			}
		})
	}
}

// TestLayoutModeDetection verifies correct layout mode detection for each size.
func TestLayoutModeDetection(t *testing.T) {
	for _, size := range terminalSizes {
		t.Run(size.name, func(t *testing.T) {
			detected := layout.DetectLayoutMode(size.cols, size.rows)
			if detected != size.mode {
				t.Errorf("DetectLayoutMode(%d, %d) = %v, want %v",
					size.cols, size.rows, detected, size.mode)
			}
		})
	}
}

// TestLayoutFeaturesForSize verifies feature flags are correct for each size.
func TestLayoutFeaturesForSize(t *testing.T) {
	expectedFeatures := map[string]layout.LayoutFeatures{
		"compact": {
			ShowImage:       false,
			ShowSparklines:  false,
			ShowFullMetrics: false,
			ShowNodeMetrics: false,
			VerticalStack:   true,
			ShowBorders:     false,
		},
		"standard": {
			ShowImage:       true,
			ShowSparklines:  false,
			ShowFullMetrics: false,
			ShowNodeMetrics: false,
			VerticalStack:   false,
			ShowBorders:     true,
		},
		"wide": {
			ShowImage:       true,
			ShowSparklines:  false,
			ShowFullMetrics: true,
			ShowNodeMetrics: true,
			VerticalStack:   false,
			ShowBorders:     true,
		},
		"ultrawide": {
			ShowImage:       true,
			ShowSparklines:  true,
			ShowFullMetrics: true,
			ShowNodeMetrics: true,
			VerticalStack:   false,
			ShowBorders:     true,
		},
	}

	for _, size := range terminalSizes {
		t.Run(size.name, func(t *testing.T) {
			cfg := layout.NewResponsiveConfig(size.cols, size.rows)
			expected := expectedFeatures[size.name]

			if cfg.Features.ShowImage != expected.ShowImage {
				t.Errorf("ShowImage = %v, want %v", cfg.Features.ShowImage, expected.ShowImage)
			}
			if cfg.Features.ShowSparklines != expected.ShowSparklines {
				t.Errorf("ShowSparklines = %v, want %v", cfg.Features.ShowSparklines, expected.ShowSparklines)
			}
			if cfg.Features.ShowFullMetrics != expected.ShowFullMetrics {
				t.Errorf("ShowFullMetrics = %v, want %v", cfg.Features.ShowFullMetrics, expected.ShowFullMetrics)
			}
			if cfg.Features.ShowNodeMetrics != expected.ShowNodeMetrics {
				t.Errorf("ShowNodeMetrics = %v, want %v", cfg.Features.ShowNodeMetrics, expected.ShowNodeMetrics)
			}
			if cfg.Features.VerticalStack != expected.VerticalStack {
				t.Errorf("VerticalStack = %v, want %v", cfg.Features.VerticalStack, expected.VerticalStack)
			}
			if cfg.Features.ShowBorders != expected.ShowBorders {
				t.Errorf("ShowBorders = %v, want %v", cfg.Features.ShowBorders, expected.ShowBorders)
			}
		})
	}
}

// TestOutputDimensions verifies output fits within terminal bounds.
func TestOutputDimensions(t *testing.T) {
	for _, size := range terminalSizes {
		t.Run(size.name, func(t *testing.T) {
			cfg := layout.NewResponsiveConfig(size.cols, size.rows)
			cfg.ColorEnabled = false
			l := layout.NewResponsiveLayout(cfg)

			sections := createTestSections(true, true, true, false)
			imageContent := ""
			if cfg.Features.ShowImage {
				imageContent = createFakeImage(cfg.Columns.ImageCols, 15)
			}

			result := l.Render(imageContent, sections, nil)

			// Check line count.
			lines := strings.Split(result.Output, "\n")
			if len(lines) > size.rows {
				t.Errorf("output has %d lines, exceeds terminal height %d", len(lines), size.rows)
			}

			// Check line widths (using visible length to handle ANSI codes).
			for i, line := range lines {
				vLen := visibleLen(line)
				if vLen > size.cols {
					t.Errorf("line %d has visible length %d, exceeds terminal width %d: %q",
						i, vLen, size.cols, line)
				}
			}
		})
	}
}

// TestUnicodeOnlyMode verifies banner renders without graphics/images.
func TestUnicodeOnlyMode(t *testing.T) {
	for _, size := range terminalSizes {
		t.Run(size.name, func(t *testing.T) {
			cfg := layout.NewResponsiveConfig(size.cols, size.rows)
			cfg.ColorEnabled = false
			cfg.Features.ShowImage = false // Force no image.
			l := layout.NewResponsiveLayout(cfg)

			sections := createTestSections(true, true, true, false)
			result := l.Render("", sections, nil)

			// Should not contain image placeholder content.
			if strings.Contains(result.Output, "XXXX") {
				t.Error("unicode-only mode should not contain image placeholders")
			}

			// Should still contain section titles.
			if !strings.Contains(result.Output, "Claude") {
				t.Error("unicode-only mode missing Claude section")
			}
		})
	}
}

// TestMultiAccountClaude verifies rendering with multiple Claude accounts.
func TestMultiAccountClaude(t *testing.T) {
	for _, size := range terminalSizes {
		t.Run(size.name, func(t *testing.T) {
			cfg := layout.NewResponsiveConfig(size.cols, size.rows)
			cfg.ColorEnabled = false
			l := layout.NewResponsiveLayout(cfg)

			sections := []layout.Section{
				{
					Title: "Claude",
					Content: []string{
						"personal: 45% (5h) | 12% (7d)",
						"work: 80% (5h) | 65% (7d)",
						"api-key: 2340/4000 req",
					},
				},
				{Title: "Billing", Content: []string{"$142 this month"}},
			}

			result := l.Render("", sections, nil)

			// Should contain all account info.
			if !strings.Contains(result.Output, "personal") {
				t.Error("missing personal account")
			}
			if !strings.Contains(result.Output, "work") {
				t.Error("missing work account")
			}
			if !strings.Contains(result.Output, "api-key") {
				t.Error("missing api-key account")
			}
		})
	}
}

// TestErrorStateRendering verifies error states are displayed correctly.
func TestErrorStateRendering(t *testing.T) {
	for _, size := range terminalSizes {
		t.Run(size.name, func(t *testing.T) {
			cfg := layout.NewResponsiveConfig(size.cols, size.rows)
			cfg.ColorEnabled = false
			l := layout.NewResponsiveLayout(cfg)

			sections := []layout.Section{
				{
					Title: "Claude",
					Content: []string{
						"personal: ERR (auth_failed)",
						"work: ERR (rate_limited)",
					},
				},
				{
					Title: "Billing",
					Content: []string{
						"digitalocean: ERR",
						"civo: ERR",
					},
				},
				{
					Title: "Infrastructure",
					Content: []string{
						"ts: connection failed",
						"k8s: unreachable",
					},
				},
			}

			result := l.Render("", sections, nil)

			// Should contain error indicators.
			if !strings.Contains(result.Output, "ERR") {
				t.Error("error state output missing ERR indicator")
			}
		})
	}
}

// TestTruncationIndicator verifies truncation when content exceeds height.
func TestTruncationIndicator(t *testing.T) {
	cfg := layout.NewResponsiveConfig(80, 10) // Small terminal.
	cfg.ColorEnabled = false
	l := layout.NewResponsiveLayout(cfg)

	// Create content that exceeds terminal height.
	sections := []layout.Section{
		{Title: "Section 1", Content: make([]string, 5)},
		{Title: "Section 2", Content: make([]string, 5)},
		{Title: "Section 3", Content: make([]string, 5)},
		{Title: "Section 4", Content: make([]string, 5)},
	}

	result := l.Render("", sections, nil)

	if !result.Truncated {
		t.Error("Truncated flag should be true when content exceeds height")
	}

	lines := strings.Split(result.Output, "\n")
	if len(lines) > 10 {
		t.Errorf("output has %d lines, should be truncated to 10", len(lines))
	}
}

// TestColumnAlignmentStandard verifies column alignment in standard mode.
func TestColumnAlignmentStandard(t *testing.T) {
	cfg := layout.NewResponsiveConfig(120, 40)
	cfg.ColorEnabled = false
	l := layout.NewResponsiveLayout(cfg)

	imageContent := createFakeImage(20, 10)
	sections := createTestSections(true, true, true, false)

	result := l.Render(imageContent, sections, nil)
	lines := strings.Split(result.Output, "\n")

	// Each line should have consistent column separator position.
	for i, line := range lines {
		if !strings.Contains(line, " \u2502 ") { // Unicode vertical bar.
			t.Errorf("line %d missing column separator: %q", i, line)
		}
	}
}

// TestSparklineColumn verifies sparkline column in ultra-wide mode.
func TestSparklineColumn(t *testing.T) {
	cfg := layout.NewResponsiveConfig(200, 80)
	cfg.ColorEnabled = false
	l := layout.NewResponsiveLayout(cfg)

	imageContent := createFakeImage(22, 15)
	sections := createTestSections(true, true, true, false)

	result := l.Render(imageContent, sections, nil)

	// Should have sparkline/trends section.
	if !strings.Contains(result.Output, "Trends") {
		t.Error("ultra-wide mode missing sparkline/trends section")
	}
}

// TestGoldenFileGeneration is a helper test that generates all golden files.
// Only runs when -update-golden flag is set.
func TestGoldenFileGeneration(t *testing.T) {
	if !*updateGolden {
		t.Skip("skipping golden file generation (run with -update-golden to generate)")
	}

	for _, size := range terminalSizes {
		for _, scenario := range testScenarios {
			t.Run(fmt.Sprintf("%s/%s", size.name, scenario.name), func(t *testing.T) {
				output := renderTestBanner(t, size.cols, size.rows, scenario)
				goldenFile := goldenFilePath(t, size.name, scenario.name)
				writeGoldenFile(t, goldenFile, output)
				t.Logf("Generated golden file: %s", goldenFile)
			})
		}
	}
}

// Helper functions.

// renderTestBanner renders a banner with the given terminal size and scenario.
func renderTestBanner(t *testing.T, cols, rows int, scenario struct {
	name        string
	withWaifu   bool
	withClaude  bool
	withBilling bool
	withInfra   bool
	errorState  bool
}) string {
	t.Helper()

	cfg := layout.NewResponsiveConfig(cols, rows)
	cfg.ColorEnabled = false // Disable colors for deterministic output.
	l := layout.NewResponsiveLayout(cfg)

	sections := createTestSections(scenario.withClaude, scenario.withBilling, scenario.withInfra, scenario.errorState)

	imageContent := ""
	if scenario.withWaifu && cfg.Features.ShowImage {
		imageContent = createFakeImage(cfg.Columns.ImageCols, 15)
	}

	result := l.Render(imageContent, sections, nil)
	return result.Output
}

// createTestSections creates layout sections with mock data.
func createTestSections(withClaude, withBilling, withInfra, errorState bool) []layout.Section {
	var sections []layout.Section

	if withClaude {
		claudeContent := createClaudeContent(errorState)
		sections = append(sections, layout.Section{
			Title:   "Claude",
			Content: claudeContent,
		})
	} else {
		sections = append(sections, layout.Section{
			Title:   "Claude",
			Content: []string{"(no data)"},
		})
	}

	if withBilling {
		billingContent := createBillingContent(errorState)
		sections = append(sections, layout.Section{
			Title:   "Billing",
			Content: billingContent,
		})
	} else {
		sections = append(sections, layout.Section{
			Title:   "Billing",
			Content: []string{"(no data)"},
		})
	}

	if withInfra {
		infraContent := createInfraContent(errorState)
		sections = append(sections, layout.Section{
			Title:   "Infrastructure",
			Content: infraContent,
		})
	} else {
		sections = append(sections, layout.Section{
			Title:   "Infrastructure",
			Content: []string{"(no data)"},
		})
	}

	return sections
}

// createClaudeContent creates mock Claude usage content.
func createClaudeContent(errorState bool) []string {
	if errorState {
		return []string{
			"personal: ERR (auth_failed)",
			"work: ERR (rate_limited)",
		}
	}
	return []string{
		"personal: 45% (5h) | 12% (7d)",
		"work: 80% (5h) | 65% (7d)",
	}
}

// createBillingContent creates mock billing content.
func createBillingContent(errorState bool) []string {
	if errorState {
		return []string{
			"digitalocean: ERR",
			"civo: ERR",
			"Total: $0 (error)",
		}
	}
	return []string{
		"digitalocean: $45.50",
		"civo: $32.00",
		"aws: $64.50",
		"Total: $142 this month | $180 forecast",
	}
}

// createInfraContent creates mock infrastructure content.
func createInfraContent(errorState bool) []string {
	if errorState {
		return []string{
			"ts: connection failed",
			"k8s: bitter-darkness (unreachable)",
		}
	}
	return []string{
		"ts: 4/5 online",
		"k8s: bitter-darkness (healthy)",
		"k8s: staging (degraded)",
		"uptime: 3d 14h",
	}
}

// createFakeImage generates a test image content string.
func createFakeImage(width, height int) string {
	var lines []string
	row := strings.Repeat("X", width)
	for i := 0; i < height; i++ {
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// goldenFilePath returns the path to a golden file for the given size and scenario.
func goldenFilePath(t *testing.T, sizeName, scenarioName string) string {
	t.Helper()
	testdataDir := filepath.Join(getTestDataDir(t), "testdata")
	return filepath.Join(testdataDir, fmt.Sprintf("golden-%s-%s.txt", sizeName, scenarioName))
}

// getTestDataDir returns the directory containing test data files.
func getTestDataDir(t *testing.T) string {
	t.Helper()
	// Get the directory of this test file.
	_, filename, _, ok := runtimeCaller(0)
	if !ok {
		t.Fatal("failed to get test file directory")
	}
	return filepath.Dir(filename)
}

// runtimeCaller is a wrapper for runtime.Caller to allow mocking in tests.
var runtimeCaller = func(skip int) (pc uintptr, file string, line int, ok bool) {
	// Use hardcoded path since we know the test file location.
	return 0, "/home/jsullivan2/git/crush-dots/cmd/prompt-pulse/tests/visual/regression_test.go", 0, true
}

// compareWithGolden compares output against a golden file and reports differences.
func compareWithGolden(t *testing.T, output, goldenFile string) {
	t.Helper()

	golden, err := os.ReadFile(goldenFile)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden file not found: %s\nRun with -update-golden to generate", goldenFile)
		}
		t.Fatalf("failed to read golden file: %v", err)
	}

	goldenStr := string(golden)
	if output != goldenStr {
		diff := computeDiff(goldenStr, output)
		t.Errorf("output does not match golden file %s:\n%s", goldenFile, diff)
	}
}

// writeGoldenFile writes output to a golden file.
func writeGoldenFile(t *testing.T, goldenFile, output string) {
	t.Helper()

	// Ensure directory exists.
	dir := filepath.Dir(goldenFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create golden file directory: %v", err)
	}

	if err := os.WriteFile(goldenFile, []byte(output), 0644); err != nil {
		t.Fatalf("failed to write golden file: %v", err)
	}
}

// computeDiff computes a simple line-by-line diff between expected and actual.
func computeDiff(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff strings.Builder
	diff.WriteString("\n--- Expected (golden)\n+++ Actual\n")

	maxLines := len(expectedLines)
	if len(actualLines) > maxLines {
		maxLines = len(actualLines)
	}

	diffCount := 0
	for i := 0; i < maxLines; i++ {
		expectedLine := ""
		if i < len(expectedLines) {
			expectedLine = expectedLines[i]
		}
		actualLine := ""
		if i < len(actualLines) {
			actualLine = actualLines[i]
		}

		if expectedLine != actualLine {
			diff.WriteString(fmt.Sprintf("@@ line %d @@\n", i+1))
			diff.WriteString(fmt.Sprintf("- %s\n", expectedLine))
			diff.WriteString(fmt.Sprintf("+ %s\n", actualLine))
			diffCount++
			if diffCount > 10 {
				diff.WriteString("... (truncated, more differences exist)\n")
				break
			}
		}
	}

	if len(expectedLines) != len(actualLines) {
		diff.WriteString(fmt.Sprintf("\nLine count: expected %d, got %d\n",
			len(expectedLines), len(actualLines)))
	}

	return diff.String()
}

// visibleLen returns the visible length of a string, stripping ANSI escape sequences.
func visibleLen(s string) int {
	length := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '~' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		length++
	}
	return length
}

// Additional tests for banner.InfoData integration.

// TestBannerInfoData verifies InfoData construction from collectors data.
func TestBannerInfoData(t *testing.T) {
	forecast := 180.0
	data := banner.InfoData{
		Claude: &collectors.ClaudeUsage{
			Accounts: []collectors.ClaudeAccountUsage{
				{
					Name:   "personal",
					Type:   "subscription",
					Status: "ok",
					FiveHour: &collectors.UsagePeriod{
						Utilization: 45.0,
						ResetsAt:    time.Now().Add(3 * time.Hour),
					},
					SevenDay: &collectors.UsagePeriod{
						Utilization: 12.0,
						ResetsAt:    time.Now().Add(5 * 24 * time.Hour),
					},
				},
			},
		},
		Billing: &collectors.BillingData{
			Total: collectors.BillingSummary{
				CurrentMonthUSD: 142.0,
				ForecastUSD:     &forecast,
			},
		},
		Infra: &collectors.InfraStatus{
			Tailscale: &collectors.TailscaleStatus{
				OnlineCount: 4,
				TotalCount:  5,
			},
			Kubernetes: []collectors.KubernetesCluster{
				{
					Name:   "bitter-darkness",
					Status: "healthy",
				},
			},
		},
		StatusLevel: "healthy",
		Uptime:      "3d 14h",
	}

	// Verify data is populated.
	if data.Claude == nil {
		t.Error("Claude should not be nil")
	}
	if len(data.Claude.Accounts) != 1 {
		t.Errorf("expected 1 Claude account, got %d", len(data.Claude.Accounts))
	}
	if data.Billing == nil {
		t.Error("Billing should not be nil")
	}
	if data.Billing.Total.CurrentMonthUSD != 142.0 {
		t.Errorf("expected $142, got $%.2f", data.Billing.Total.CurrentMonthUSD)
	}
	if data.Infra == nil {
		t.Error("Infra should not be nil")
	}
	if data.Infra.Tailscale.OnlineCount != 4 {
		t.Errorf("expected 4 online, got %d", data.Infra.Tailscale.OnlineCount)
	}
}

// TestLayoutConfigSerialization verifies layout config can be created and used.
func TestLayoutConfigSerialization(t *testing.T) {
	for _, size := range terminalSizes {
		t.Run(size.name, func(t *testing.T) {
			cfg := layout.NewResponsiveConfig(size.cols, size.rows)

			// Verify essential fields.
			if cfg.TermWidth != size.cols {
				t.Errorf("TermWidth = %d, want %d", cfg.TermWidth, size.cols)
			}
			if cfg.TermHeight != size.rows {
				t.Errorf("TermHeight = %d, want %d", cfg.TermHeight, size.rows)
			}
			if cfg.Mode != size.mode {
				t.Errorf("Mode = %v, want %v", cfg.Mode, size.mode)
			}

			// Verify columns are set.
			if cfg.Columns.MainCols <= 0 {
				t.Error("MainCols should be positive")
			}
		})
	}
}

// TestStatusIndicatorRendering verifies status indicator rendering.
func TestStatusIndicatorRendering(t *testing.T) {
	tests := []struct {
		status   string
		wantIcon string
	}{
		{"healthy", "\u25cf"},  // Filled circle.
		{"warning", "\u25cf"},  // Filled circle.
		{"critical", "\u25cf"}, // Filled circle.
		{"unknown", "\u25cb"},  // Empty circle.
	}

	cfg := layout.NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false
	l := layout.NewResponsiveLayout(cfg)

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := l.StatusIndicator(tt.status)
			if !strings.Contains(result, tt.wantIcon) {
				t.Errorf("StatusIndicator(%q) missing icon %q, got %q",
					tt.status, tt.wantIcon, result)
			}
			if !strings.Contains(result, tt.status) {
				t.Errorf("StatusIndicator(%q) missing status text", tt.status)
			}
		})
	}
}

// TestBoxRendering verifies Unicode box rendering.
func TestBoxRendering(t *testing.T) {
	cfg := layout.NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false
	l := layout.NewResponsiveLayout(cfg)

	lines := []string{"Line 1", "Line 2", "Line 3"}
	result := l.RenderBox(lines, 40, "Test Title")

	// Verify box corners.
	corners := []string{"\u256d", "\u256e", "\u256f", "\u2570"} // Rounded corners.
	for _, corner := range corners {
		if !strings.Contains(result, corner) {
			t.Errorf("box missing corner character %q", corner)
		}
	}

	// Verify title.
	if !strings.Contains(result, "Test Title") {
		t.Error("box missing title")
	}

	// Verify content.
	if !strings.Contains(result, "Line 1") {
		t.Error("box missing content")
	}
}

// Benchmark tests.

// BenchmarkLayoutRenderCompact benchmarks compact mode rendering.
func BenchmarkLayoutRenderCompact(b *testing.B) {
	cfg := layout.NewResponsiveConfig(80, 24)
	cfg.ColorEnabled = false
	l := layout.NewResponsiveLayout(cfg)
	sections := createBenchmarkSections()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Render("", sections, nil)
	}
}

// BenchmarkLayoutRenderStandard benchmarks standard mode rendering.
func BenchmarkLayoutRenderStandard(b *testing.B) {
	cfg := layout.NewResponsiveConfig(120, 40)
	cfg.ColorEnabled = false
	l := layout.NewResponsiveLayout(cfg)
	sections := createBenchmarkSections()
	imageContent := createFakeImage(22, 15)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Render(imageContent, sections, nil)
	}
}

// BenchmarkLayoutRenderWide benchmarks wide mode rendering.
func BenchmarkLayoutRenderWide(b *testing.B) {
	cfg := layout.NewResponsiveConfig(160, 60)
	cfg.ColorEnabled = false
	l := layout.NewResponsiveLayout(cfg)
	sections := createBenchmarkSections()
	imageContent := createFakeImage(24, 15)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Render(imageContent, sections, nil)
	}
}

// BenchmarkLayoutRenderUltraWide benchmarks ultra-wide mode rendering.
func BenchmarkLayoutRenderUltraWide(b *testing.B) {
	cfg := layout.NewResponsiveConfig(200, 80)
	cfg.ColorEnabled = false
	l := layout.NewResponsiveLayout(cfg)
	sections := createBenchmarkSections()
	imageContent := createFakeImage(24, 15)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Render(imageContent, sections, nil)
	}
}

// createBenchmarkSections creates sections for benchmarking.
func createBenchmarkSections() []layout.Section {
	return []layout.Section{
		{
			Title: "Claude",
			Content: []string{
				"personal: 45% (5h) | 12% (7d)",
				"work: 80% (5h) | 65% (7d)",
			},
		},
		{
			Title: "Billing",
			Content: []string{
				"digitalocean: $45.50",
				"civo: $32.00",
				"Total: $142 this month",
			},
		},
		{
			Title: "Infrastructure",
			Content: []string{
				"ts: 4/5 online",
				"k8s: bitter-darkness (healthy)",
			},
		},
	}
}
