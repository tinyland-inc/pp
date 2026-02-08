package widgets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderSparkline_BasicData(t *testing.T) {
	cfg := SparklineConfig{
		Data: []float64{1, 2, 3, 4, 5, 6, 7, 8},
	}

	result := RenderSparkline(cfg)

	if len(result) == 0 {
		t.Fatal("expected non-empty sparkline for ascending data")
	}

	// Ascending data should produce ascending block characters.
	runes := []rune(result)
	for i := 1; i < len(runes); i++ {
		if runes[i] < runes[i-1] {
			t.Errorf("expected ascending blocks, but rune at %d (%c) < rune at %d (%c)",
				i, runes[i], i-1, runes[i-1])
		}
	}
}

func TestRenderSparkline_EmptyData(t *testing.T) {
	cfg := SparklineConfig{
		Data: []float64{},
	}

	result := RenderSparkline(cfg)

	if result != "" {
		t.Errorf("expected empty string for empty data, got: %q", result)
	}
}

func TestRenderSparkline_SinglePoint(t *testing.T) {
	cfg := SparklineConfig{
		Data: []float64{42},
	}

	result := RenderSparkline(cfg)

	// Single point: all values equal, should use mid-level block.
	runes := []rune(result)
	if len(runes) != 1 {
		t.Errorf("expected 1 character for single point, got %d: %q", len(runes), result)
	}
}

func TestRenderSparkline_AllEqual(t *testing.T) {
	cfg := SparklineConfig{
		Data: []float64{5, 5, 5, 5, 5},
	}

	result := RenderSparkline(cfg)

	// All equal values should produce identical mid-level blocks.
	runes := []rune(result)
	if len(runes) != 5 {
		t.Errorf("expected 5 characters, got %d: %q", len(runes), result)
	}
	expected := sparkBlocks[len(sparkBlocks)/2]
	for i, r := range runes {
		if r != expected {
			t.Errorf("position %d: expected mid-level block %c, got %c", i, expected, r)
		}
	}
}

func TestRenderSparkline_AutoScale(t *testing.T) {
	// Min == Max (both 0) triggers auto-scaling.
	cfg := SparklineConfig{
		Data: []float64{10, 20, 30},
		Min:  0,
		Max:  0,
	}

	result := RenderSparkline(cfg)

	runes := []rune(result)
	if len(runes) != 3 {
		t.Errorf("expected 3 characters, got %d: %q", len(runes), result)
	}
	// First should be lowest block, last should be highest.
	if runes[0] != sparkBlocks[0] {
		t.Errorf("expected lowest block for min value, got %c", runes[0])
	}
	if runes[2] != sparkBlocks[len(sparkBlocks)-1] {
		t.Errorf("expected highest block for max value, got %c", runes[2])
	}
}

func TestRenderSparkline_ManualScale(t *testing.T) {
	cfg := SparklineConfig{
		Data: []float64{50},
		Min:  0,
		Max:  100,
	}

	result := RenderSparkline(cfg)

	runes := []rune(result)
	if len(runes) != 1 {
		t.Fatalf("expected 1 character, got %d: %q", len(runes), result)
	}
	// 50 out of 0-100 is 0.5, which maps to block index 3 or 4 (midrange).
	midIdx := int(0.5 * float64(len(sparkBlocks)-1))
	expected := sparkBlocks[midIdx]
	if runes[0] != expected {
		t.Errorf("expected mid-range block %c for 50/100, got %c", expected, runes[0])
	}
}

func TestRenderSparkline_Truncation(t *testing.T) {
	cfg := SparklineConfig{
		Data:  []float64{1, 2, 3, 4, 5, 6, 7, 8},
		Width: 4,
	}

	result := RenderSparkline(cfg)

	// Should take last 4 points: 5, 6, 7, 8.
	runes := []rune(result)
	if len(runes) != 4 {
		t.Errorf("expected 4 characters after truncation, got %d: %q", len(runes), result)
	}
}

func TestRenderSparkline_Padding(t *testing.T) {
	cfg := SparklineConfig{
		Data:  []float64{1, 2, 3},
		Width: 6,
	}

	result := RenderSparkline(cfg)

	// Should be 3 spaces + 3 block characters = 6 characters total.
	runes := []rune(result)
	if len(runes) != 6 {
		t.Errorf("expected 6 characters with padding, got %d: %q", len(runes), result)
	}
	// First 3 should be spaces.
	for i := 0; i < 3; i++ {
		if runes[i] != ' ' {
			t.Errorf("expected space at position %d, got %c", i, runes[i])
		}
	}
}

func TestRenderSparkline_WithLabel(t *testing.T) {
	cfg := SparklineConfig{
		Data:  []float64{1, 2, 3},
		Label: "CPU",
	}

	result := RenderSparkline(cfg)

	if !strings.HasPrefix(result, "CPU ") {
		t.Errorf("expected output to start with 'CPU ', got: %q", result)
	}
}

func TestRenderSparkline_NegativeValues(t *testing.T) {
	cfg := SparklineConfig{
		Data: []float64{-10, -5, 0, 5, 10},
	}

	result := RenderSparkline(cfg)

	runes := []rune(result)
	if len(runes) != 5 {
		t.Errorf("expected 5 characters for negative values, got %d: %q", len(runes), result)
	}
	// First value (-10) is min, should be lowest block.
	if runes[0] != sparkBlocks[0] {
		t.Errorf("expected lowest block for -10, got %c", runes[0])
	}
	// Last value (10) is max, should be highest block.
	if runes[4] != sparkBlocks[len(sparkBlocks)-1] {
		t.Errorf("expected highest block for 10, got %c", runes[4])
	}
}

func TestRenderSparklineWithRange(t *testing.T) {
	data := []float64{1, 5, 3, 8, 2}

	result := RenderSparklineWithRange(data, 5)

	// Should start with min (1) and end with max (8).
	if !strings.HasPrefix(result, "1") {
		t.Errorf("expected output to start with min value '1', got: %q", result)
	}
	if !strings.HasSuffix(result, "8") {
		t.Errorf("expected output to end with max value '8', got: %q", result)
	}
}

func TestRenderSparklineWithRange_EmptyData(t *testing.T) {
	result := RenderSparklineWithRange([]float64{}, 5)

	if result != "" {
		t.Errorf("expected empty string for empty data, got: %q", result)
	}
}

func TestSparkBlocks_Length(t *testing.T) {
	if len(sparkBlocks) != 8 {
		t.Errorf("expected exactly 8 spark block characters, got %d", len(sparkBlocks))
	}
}

func TestRenderSparkline_WithColor(t *testing.T) {
	cfg := SparklineConfig{
		Data:  []float64{1, 2, 3},
		Color: lipgloss.Color("#22C55E"),
	}

	result := RenderSparkline(cfg)

	// The result should be non-empty and contain sparkline characters.
	// Note: lipgloss may strip ANSI codes in non-TTY environments,
	// so we verify the color config is accepted and output is rendered.
	if len(result) == 0 {
		t.Error("expected non-empty output when Color is set")
	}
	// Verify sparkline blocks are present in the output.
	hasBlock := false
	for _, r := range result {
		for _, b := range sparkBlocks {
			if r == b {
				hasBlock = true
				break
			}
		}
	}
	if !hasBlock {
		t.Errorf("expected sparkline block characters in output, got: %q", result)
	}
}
