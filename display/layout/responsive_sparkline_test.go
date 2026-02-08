package layout

import (
	"strings"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

func TestBuildActualSparklines_Nil(t *testing.T) {
	cfg := NewResponsiveConfig(200, 80)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	lines := layout.buildActualSparklines(nil)
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines for nil billing, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Trends") {
		t.Errorf("Expected 'Trends' title, got %s", lines[0])
	}
	if lines[1] != "  (no data)" {
		t.Errorf("Expected '  (no data)', got %s", lines[1])
	}
}

func TestBuildActualSparklines_NoHistory(t *testing.T) {
	cfg := NewResponsiveConfig(200, 80)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	billing := &collectors.BillingData{
		History: nil,
	}

	lines := layout.buildActualSparklines(billing)
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines for no history, got %d", len(lines))
	}
	if lines[1] != "  (no data)" {
		t.Errorf("Expected '  (no data)', got %s", lines[1])
	}
}

func TestBuildActualSparklines_WithTotalHistory(t *testing.T) {
	cfg := NewResponsiveConfig(200, 80)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	// Create 30 days of billing history
	history := make([]collectors.DailySpend, 30)
	for i := 0; i < 30; i++ {
		history[i] = collectors.DailySpend{
			Date:     time.Now().AddDate(0, 0, -29+i).Format("2006-01-02"),
			SpendUSD: float64(100 + i*5), // Increasing spend
		}
	}

	billing := &collectors.BillingData{
		History: &collectors.BillingHistory{
			TotalHistory: history,
		},
	}

	lines := layout.buildActualSparklines(billing)

	// Should have at least Trends title + Total sparkline
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 lines, got %d", len(lines))
	}

	// Check for Trends title
	if !strings.Contains(lines[0], "Trends") {
		t.Errorf("Expected 'Trends' title, got %s", lines[0])
	}

	// Check for Total sparkline (should contain sparkline characters or label)
	foundSparkline := false
	for _, line := range lines {
		if strings.Contains(line, "Total") {
			foundSparkline = true
			// Should contain sparkline Unicode characters (▁▂▃▄▅▆▇█)
			hasSparklineChars := false
			for _, r := range line {
				if r >= '▁' && r <= '█' {
					hasSparklineChars = true
					break
				}
			}
			if !hasSparklineChars {
				t.Errorf("Expected sparkline characters in: %s", line)
			}
			break
		}
	}
	if !foundSparkline {
		t.Errorf("Expected 'Total' sparkline in output: %v", lines)
	}
}

func TestBuildActualSparklines_WithProviderHistory(t *testing.T) {
	cfg := NewResponsiveConfig(200, 80)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	// Create provider histories
	civoHistory := make([]collectors.DailySpend, 30)
	doHistory := make([]collectors.DailySpend, 30)
	for i := 0; i < 30; i++ {
		civoHistory[i] = collectors.DailySpend{
			Date:     time.Now().AddDate(0, 0, -29+i).Format("2006-01-02"),
			SpendUSD: float64(50 + i*2),
		}
		doHistory[i] = collectors.DailySpend{
			Date:     time.Now().AddDate(0, 0, -29+i).Format("2006-01-02"),
			SpendUSD: float64(30 + i),
		}
	}

	billing := &collectors.BillingData{
		History: &collectors.BillingHistory{
			TotalHistory: civoHistory, // Use civo as total
			ProviderHistory: map[string][]collectors.DailySpend{
				"civo":         civoHistory,
				"digitalocean": doHistory,
			},
		},
	}

	lines := layout.buildActualSparklines(billing)

	// Should have Trends title + Total + top 2 providers (max 4 lines)
	if len(lines) < 3 {
		t.Fatalf("Expected at least 3 lines (Trends + Total + provider), got %d", len(lines))
	}

	// Check for provider sparklines
	foundCivo := false
	foundDO := false
	for _, line := range lines {
		if strings.Contains(line, "civo") {
			foundCivo = true
		}
		if strings.Contains(line, "digitalocean") {
			foundDO = true
		}
	}
	if !foundCivo {
		t.Errorf("Expected 'civo' provider sparkline in output: %v", lines)
	}
	if !foundDO {
		t.Errorf("Expected 'digitalocean' provider sparkline in output: %v", lines)
	}
}

func TestBuildActualSparklines_EmptyHistory(t *testing.T) {
	cfg := NewResponsiveConfig(200, 80)
	cfg.ColorEnabled = false
	layout := NewResponsiveLayout(cfg)

	billing := &collectors.BillingData{
		History: &collectors.BillingHistory{
			TotalHistory:    []collectors.DailySpend{},
			ProviderHistory: map[string][]collectors.DailySpend{},
		},
	}

	lines := layout.buildActualSparklines(billing)

	// Should have Trends title + "(no history)"
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines for empty history, got %d", len(lines))
	}
	if lines[1] != "  (no history)" {
		t.Errorf("Expected '  (no history)', got %s", lines[1])
	}
}
