package tui

import (
	"strings"
	"testing"
	"time"
)

func TestDetectLayout_Compact(t *testing.T) {
	tests := []int{10, 30, 59}
	for _, width := range tests {
		got := DetectLayout(width)
		if got != LayoutCompact {
			t.Errorf("DetectLayout(%d) = %d, want LayoutCompact (%d)", width, got, LayoutCompact)
		}
	}
}

func TestDetectLayout_Normal(t *testing.T) {
	tests := []int{60, 80, 100, 120}
	for _, width := range tests {
		got := DetectLayout(width)
		if got != LayoutNormal {
			t.Errorf("DetectLayout(%d) = %d, want LayoutNormal (%d)", width, got, LayoutNormal)
		}
	}
}

func TestDetectLayout_Wide(t *testing.T) {
	tests := []int{121, 150, 200}
	for _, width := range tests {
		got := DetectLayout(width)
		if got != LayoutWide {
			t.Errorf("DetectLayout(%d) = %d, want LayoutWide (%d)", width, got, LayoutWide)
		}
	}
}

func TestLayoutForSize_Compact(t *testing.T) {
	cfg := LayoutForSize(LayoutCompact, 50)

	if cfg.GaugeWidth != 10 {
		t.Errorf("Compact GaugeWidth = %d, want 10", cfg.GaugeWidth)
	}
	if cfg.TableMaxWidth != 46 {
		t.Errorf("Compact TableMaxWidth = %d, want 46", cfg.TableMaxWidth)
	}
	if cfg.ShowSparklines {
		t.Error("Compact ShowSparklines should be false")
	}
	if cfg.ShowMiniGauges {
		t.Error("Compact ShowMiniGauges should be false")
	}
	if cfg.ContentPadding != 1 {
		t.Errorf("Compact ContentPadding = %d, want 1", cfg.ContentPadding)
	}
}

func TestLayoutForSize_Normal(t *testing.T) {
	cfg := LayoutForSize(LayoutNormal, 100)

	if cfg.GaugeWidth != 20 {
		t.Errorf("Normal GaugeWidth = %d, want 20", cfg.GaugeWidth)
	}
	if cfg.TableMaxWidth != 92 {
		t.Errorf("Normal TableMaxWidth = %d, want 92", cfg.TableMaxWidth)
	}
	if !cfg.ShowSparklines {
		t.Error("Normal ShowSparklines should be true")
	}
	if cfg.ShowMiniGauges {
		t.Error("Normal ShowMiniGauges should be false")
	}
	if cfg.ContentPadding != 2 {
		t.Errorf("Normal ContentPadding = %d, want 2", cfg.ContentPadding)
	}
}

func TestLayoutForSize_Wide(t *testing.T) {
	cfg := LayoutForSize(LayoutWide, 150)

	if cfg.GaugeWidth != 30 {
		t.Errorf("Wide GaugeWidth = %d, want 30", cfg.GaugeWidth)
	}
	if cfg.TableMaxWidth != 138 {
		t.Errorf("Wide TableMaxWidth = %d, want 138", cfg.TableMaxWidth)
	}
	if !cfg.ShowSparklines {
		t.Error("Wide ShowSparklines should be true")
	}
	if !cfg.ShowMiniGauges {
		t.Error("Wide ShowMiniGauges should be true")
	}
	if cfg.ContentPadding != 3 {
		t.Errorf("Wide ContentPadding = %d, want 3", cfg.ContentPadding)
	}
}

func TestTruncateText_Short(t *testing.T) {
	got := truncateText("hello", 10)
	if got != "hello" {
		t.Errorf("truncateText(\"hello\", 10) = %q, want %q", got, "hello")
	}
}

func TestTruncateText_Exact(t *testing.T) {
	got := truncateText("hello", 5)
	if got != "hello" {
		t.Errorf("truncateText(\"hello\", 5) = %q, want %q", got, "hello")
	}
}

func TestTruncateText_Long(t *testing.T) {
	got := truncateText("hello world", 8)
	if got != "hello..." {
		t.Errorf("truncateText(\"hello world\", 8) = %q, want %q", got, "hello...")
	}
}

func TestTruncateText_VeryShort(t *testing.T) {
	// maxWidth < 4 should hard-truncate without ellipsis.
	got := truncateText("hello", 3)
	if got != "hel" {
		t.Errorf("truncateText(\"hello\", 3) = %q, want %q", got, "hel")
	}

	got = truncateText("hello", 0)
	if got != "" {
		t.Errorf("truncateText(\"hello\", 0) = %q, want %q", got, "")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		input    time.Time
		contains string
	}{
		{"zero time", time.Time{}, "never"},
		{"just now", now, "just now"},
		{"30 seconds", now.Add(-30 * time.Second), "30s ago"},
		{"5 minutes", now.Add(-5 * time.Minute), "5m ago"},
		{"2 hours", now.Add(-2 * time.Hour), "2h ago"},
		{"3 days", now.Add(-3 * 24 * time.Hour), "3d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRelativeTime(tt.input)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("formatRelativeTime() = %q, want to contain %q", got, tt.contains)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name  string
		input time.Duration
		want  string
	}{
		{"zero", 0, "0s"},
		{"sub-second", 500 * time.Millisecond, "0s"},
		{"seconds", 45 * time.Second, "45s"},
		{"minutes", 5*time.Minute + 30*time.Second, "5m 30s"},
		{"hours", 2*time.Hour + 15*time.Minute, "2h 15m"},
		{"days", 3*24*time.Hour + 4*time.Hour, "3d 4h"},
		{"negative", -5 * time.Minute, "5m 0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHorizontalRule(t *testing.T) {
	got := horizontalRule(10)
	if len([]rune(got)) != 10 {
		t.Errorf("horizontalRule(10) length = %d, want 10", len([]rune(got)))
	}
	for _, r := range got {
		if r != '\u2500' {
			t.Errorf("horizontalRule(10) contains unexpected rune %U", r)
		}
	}

	// Zero width should return empty.
	got = horizontalRule(0)
	if got != "" {
		t.Errorf("horizontalRule(0) = %q, want empty", got)
	}
}

func TestSectionTitle(t *testing.T) {
	got := sectionTitle("Test", 20)

	if !strings.Contains(got, "Test") {
		t.Errorf("sectionTitle(\"Test\", 20) = %q, missing title text", got)
	}

	// Should contain box-drawing characters on both sides.
	if !strings.Contains(got, "\u2500") {
		t.Errorf("sectionTitle(\"Test\", 20) = %q, missing horizontal rule chars", got)
	}

	// Total rune length should equal width.
	runeLen := len([]rune(got))
	if runeLen != 20 {
		t.Errorf("sectionTitle(\"Test\", 20) rune length = %d, want 20", runeLen)
	}
}
