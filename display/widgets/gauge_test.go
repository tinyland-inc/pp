package widgets

import (
	"strings"
	"testing"
)

func TestRenderGauge_DefaultConfig(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = 50

	result := RenderGauge(cfg)

	// At 50%, half the width (10) should be filled, half empty.
	if !strings.Contains(result, "50%") {
		t.Errorf("expected percentage text '50%%' in output, got: %q", result)
	}
	// Count raw block characters (before ANSI codes are applied).
	filledCount := strings.Count(result, "█")
	emptyCount := strings.Count(result, "░")
	if filledCount != 10 {
		t.Errorf("expected 10 filled chars at 50%%, got %d", filledCount)
	}
	if emptyCount != 10 {
		t.Errorf("expected 10 empty chars at 50%%, got %d", emptyCount)
	}
}

func TestRenderGauge_ZeroPercent(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = 0

	result := RenderGauge(cfg)

	filledCount := strings.Count(result, "█")
	emptyCount := strings.Count(result, "░")
	if filledCount != 0 {
		t.Errorf("expected 0 filled chars at 0%%, got %d", filledCount)
	}
	if emptyCount != 20 {
		t.Errorf("expected 20 empty chars at 0%%, got %d", emptyCount)
	}
	if !strings.Contains(result, "0%") {
		t.Errorf("expected '0%%' in output, got: %q", result)
	}
}

func TestRenderGauge_HundredPercent(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = 100

	result := RenderGauge(cfg)

	filledCount := strings.Count(result, "█")
	emptyCount := strings.Count(result, "░")
	if filledCount != 20 {
		t.Errorf("expected 20 filled chars at 100%%, got %d", filledCount)
	}
	if emptyCount != 0 {
		t.Errorf("expected 0 empty chars at 100%%, got %d", emptyCount)
	}
	if !strings.Contains(result, "100%") {
		t.Errorf("expected '100%%' in output, got: %q", result)
	}
}

func TestRenderGauge_OverHundred(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = 150

	result := RenderGauge(cfg)

	// Should be clamped to 100%.
	filledCount := strings.Count(result, "█")
	if filledCount != 20 {
		t.Errorf("expected 20 filled chars (clamped to 100%%), got %d", filledCount)
	}
	if !strings.Contains(result, "100%") {
		t.Errorf("expected '100%%' (clamped) in output, got: %q", result)
	}
}

func TestRenderGauge_Negative(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = -25

	result := RenderGauge(cfg)

	// Should be clamped to 0%.
	filledCount := strings.Count(result, "█")
	emptyCount := strings.Count(result, "░")
	if filledCount != 0 {
		t.Errorf("expected 0 filled chars (clamped to 0%%), got %d", filledCount)
	}
	if emptyCount != 20 {
		t.Errorf("expected 20 empty chars (clamped to 0%%), got %d", emptyCount)
	}
	if !strings.Contains(result, "0%") {
		t.Errorf("expected '0%%' (clamped) in output, got: %q", result)
	}
}

func TestRenderGauge_WithLabel(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = 50
	cfg.Label = "CPU"

	result := RenderGauge(cfg)

	if !strings.HasPrefix(result, "CPU ") {
		t.Errorf("expected output to start with 'CPU ', got: %q", result)
	}
}

func TestRenderGauge_NoPercent(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = 50
	cfg.ShowPercent = false

	result := RenderGauge(cfg)

	if strings.Contains(result, "%") {
		t.Errorf("expected no percentage text when ShowPercent=false, got: %q", result)
	}
}

func TestRenderGauge_CustomChars(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = 50
	cfg.FilledChar = "#"
	cfg.EmptyChar = "-"
	cfg.ShowPercent = false

	result := RenderGauge(cfg)

	hashCount := strings.Count(result, "#")
	dashCount := strings.Count(result, "-")
	if hashCount != 10 {
		t.Errorf("expected 10 '#' chars at 50%%, got %d", hashCount)
	}
	if dashCount != 10 {
		t.Errorf("expected 10 '-' chars at 50%%, got %d", dashCount)
	}
}

func TestRenderGauge_WarningThreshold(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = 75
	cfg.ShowPercent = false

	result := RenderGauge(cfg)

	// At 75%, above warning (70) but below danger (90), should have filled chars.
	filledCount := strings.Count(result, "█")
	if filledCount == 0 {
		t.Errorf("expected filled chars at 75%%, got none in: %q", result)
	}
	// Verify the correct color is selected for warning range.
	color := gaugeColor(75, 70, 90)
	if color != "#EAB308" {
		t.Errorf("expected yellow (#EAB308) for 75%%, got %s", string(color))
	}
}

func TestRenderGauge_DangerThreshold(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = 95
	cfg.ShowPercent = false

	result := RenderGauge(cfg)

	// At 95%, above danger (90), should have filled chars.
	filledCount := strings.Count(result, "█")
	if filledCount == 0 {
		t.Errorf("expected filled chars at 95%%, got none in: %q", result)
	}
	// Verify the correct color is selected for danger range.
	color := gaugeColor(95, 70, 90)
	if color != "#EF4444" {
		t.Errorf("expected red (#EF4444) for 95%%, got %s", string(color))
	}
}

func TestRenderGauge_NormalThreshold(t *testing.T) {
	cfg := DefaultGaugeConfig()
	cfg.Percent = 30
	cfg.ShowPercent = false

	result := RenderGauge(cfg)

	// At 30%, below warning (70), should have filled chars.
	filledCount := strings.Count(result, "█")
	if filledCount == 0 {
		t.Errorf("expected filled chars at 30%%, got none in: %q", result)
	}
	// Verify the correct color is selected for normal range.
	color := gaugeColor(30, 70, 90)
	if color != "#22C55E" {
		t.Errorf("expected green (#22C55E) for 30%%, got %s", string(color))
	}
}

func TestRenderMiniGauge_Basic(t *testing.T) {
	result := RenderMiniGauge(50, 10)

	filledCount := strings.Count(result, "█")
	emptyCount := strings.Count(result, "░")
	if filledCount != 5 {
		t.Errorf("expected 5 filled chars in mini gauge at 50%%, got %d", filledCount)
	}
	if emptyCount != 5 {
		t.Errorf("expected 5 empty chars in mini gauge at 50%%, got %d", emptyCount)
	}
	// No percentage text.
	if strings.Contains(result, "%") {
		t.Errorf("mini gauge should not contain '%%', got: %q", result)
	}
}

func TestRenderMiniGauge_Zero(t *testing.T) {
	result := RenderMiniGauge(0, 10)

	filledCount := strings.Count(result, "█")
	emptyCount := strings.Count(result, "░")
	if filledCount != 0 {
		t.Errorf("expected 0 filled chars in mini gauge at 0%%, got %d", filledCount)
	}
	if emptyCount != 10 {
		t.Errorf("expected 10 empty chars in mini gauge at 0%%, got %d", emptyCount)
	}
}

func TestDefaultGaugeConfig(t *testing.T) {
	cfg := DefaultGaugeConfig()

	if cfg.Width != 20 {
		t.Errorf("expected default Width=20, got %d", cfg.Width)
	}
	if !cfg.ShowPercent {
		t.Error("expected default ShowPercent=true")
	}
	if cfg.ThresholdWarning != 70 {
		t.Errorf("expected default ThresholdWarning=70, got %f", cfg.ThresholdWarning)
	}
	if cfg.ThresholdDanger != 90 {
		t.Errorf("expected default ThresholdDanger=90, got %f", cfg.ThresholdDanger)
	}
	if cfg.FilledChar != "█" {
		t.Errorf("expected default FilledChar='█', got %q", cfg.FilledChar)
	}
	if cfg.EmptyChar != "░" {
		t.Errorf("expected default EmptyChar='░', got %q", cfg.EmptyChar)
	}
}
