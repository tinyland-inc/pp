package billing

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

func TestHistoryStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	store := NewHistoryStore(dir, logger)

	history := store.Load()

	if history == nil {
		t.Fatal("expected non-nil history")
	}
	if history.ProviderHistory == nil {
		t.Error("expected initialized ProviderHistory map")
	}
	if len(history.TotalHistory) != 0 {
		t.Error("expected empty TotalHistory")
	}
}

func TestHistoryStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	store := NewHistoryStore(dir, logger)

	history := &collectors.BillingHistory{
		ProviderHistory: map[string][]collectors.DailySpend{
			"civo": {
				{Date: "2026-01-01", SpendUSD: 10.0},
				{Date: "2026-01-02", SpendUSD: 20.0},
			},
		},
		TotalHistory: []collectors.DailySpend{
			{Date: "2026-01-01", SpendUSD: 10.0},
			{Date: "2026-01-02", SpendUSD: 20.0},
		},
		LastUpdated: time.Now(),
	}

	// Save.
	if err := store.Save(history); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(dir, historyFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected history file to exist")
	}

	// Load.
	loaded := store.Load()
	if loaded == nil {
		t.Fatal("Load returned nil")
	}

	if len(loaded.ProviderHistory["civo"]) != 2 {
		t.Errorf("expected 2 civo entries, got %d", len(loaded.ProviderHistory["civo"]))
	}
	if len(loaded.TotalHistory) != 2 {
		t.Errorf("expected 2 total entries, got %d", len(loaded.TotalHistory))
	}
}

func TestHistoryStore_Update(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	store := NewHistoryStore(dir, logger)

	// Start with empty history.
	history := store.Load()

	// Simulate provider data.
	providers := []collectors.ProviderBilling{
		{
			Provider: "civo",
			Status:   "ok",
			CurrentMonth: collectors.MonthCost{
				SpendUSD: 50.0,
			},
		},
		{
			Provider: "digitalocean",
			Status:   "ok",
			CurrentMonth: collectors.MonthCost{
				SpendUSD: 30.0,
			},
		},
	}

	// Update history.
	updated := store.Update(history, providers)

	// Verify provider histories.
	if len(updated.ProviderHistory["civo"]) != 1 {
		t.Errorf("expected 1 civo entry, got %d", len(updated.ProviderHistory["civo"]))
	}
	if len(updated.ProviderHistory["digitalocean"]) != 1 {
		t.Errorf("expected 1 digitalocean entry, got %d", len(updated.ProviderHistory["digitalocean"]))
	}

	// Verify total history.
	if len(updated.TotalHistory) != 1 {
		t.Errorf("expected 1 total entry, got %d", len(updated.TotalHistory))
	}
	if updated.TotalHistory[0].SpendUSD != 80.0 {
		t.Errorf("expected total spend 80.0, got %f", updated.TotalHistory[0].SpendUSD)
	}
}

func TestHistoryStore_UpdateReplacesToday(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	store := NewHistoryStore(dir, logger)

	today := time.Now().Format("2006-01-02")

	// Pre-populate with today's data.
	history := &collectors.BillingHistory{
		ProviderHistory: map[string][]collectors.DailySpend{
			"civo": {
				{Date: today, SpendUSD: 10.0}, // Old value.
			},
		},
		TotalHistory: []collectors.DailySpend{
			{Date: today, SpendUSD: 10.0},
		},
	}

	// Update with new value.
	providers := []collectors.ProviderBilling{
		{
			Provider: "civo",
			Status:   "ok",
			CurrentMonth: collectors.MonthCost{
				SpendUSD: 50.0, // New value.
			},
		},
	}

	updated := store.Update(history, providers)

	// Should replace, not append.
	if len(updated.ProviderHistory["civo"]) != 1 {
		t.Errorf("expected 1 civo entry (replaced), got %d", len(updated.ProviderHistory["civo"]))
	}
	if updated.ProviderHistory["civo"][0].SpendUSD != 50.0 {
		t.Errorf("expected spend 50.0, got %f", updated.ProviderHistory["civo"][0].SpendUSD)
	}
}

func TestHistoryStore_UpdateIgnoresErrors(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	store := NewHistoryStore(dir, logger)

	history := store.Load()

	providers := []collectors.ProviderBilling{
		{
			Provider: "civo",
			Status:   "ok",
			CurrentMonth: collectors.MonthCost{
				SpendUSD: 50.0,
			},
		},
		{
			Provider: "aws",
			Status:   "error", // Should be ignored.
			CurrentMonth: collectors.MonthCost{
				SpendUSD: 100.0,
			},
		},
	}

	updated := store.Update(history, providers)

	// AWS should not be in history.
	if _, exists := updated.ProviderHistory["aws"]; exists {
		t.Error("expected error provider to be excluded from history")
	}

	// Total should only include civo.
	if updated.TotalHistory[0].SpendUSD != 50.0 {
		t.Errorf("expected total 50.0 (civo only), got %f", updated.TotalHistory[0].SpendUSD)
	}
}

func TestPruneAndSort(t *testing.T) {
	// Create entries spanning more than 30 days.
	entries := []collectors.DailySpend{
		{Date: "2025-12-01", SpendUSD: 10.0}, // Should be pruned.
		{Date: "2026-01-15", SpendUSD: 30.0},
		{Date: "2026-01-10", SpendUSD: 20.0}, // Out of order.
	}

	result := pruneAndSort(entries, 30)

	// Should be sorted.
	if len(result) > 1 && result[0].Date > result[1].Date {
		t.Error("expected entries to be sorted by date")
	}

	// Old entries should be pruned (depending on current date).
	for _, e := range result {
		if e.Date < time.Now().AddDate(0, 0, -30).Format("2006-01-02") {
			t.Errorf("expected old entry %s to be pruned", e.Date)
		}
	}
}

func TestCalculateForecast(t *testing.T) {
	tests := []struct {
		name         string
		currentSpend float64
		daysElapsed  int
		daysInMonth  int
		expected     float64
	}{
		{"mid-month", 50.0, 15, 30, 100.0},
		{"first-day", 10.0, 1, 30, 300.0},
		{"zero-days", 10.0, 0, 30, 10.0},
		{"end-of-month", 100.0, 30, 30, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateForecast(tt.currentSpend, tt.daysElapsed, tt.daysInMonth)
			if result != tt.expected {
				t.Errorf("got %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestGetProviderHistoryValues(t *testing.T) {
	history := &collectors.BillingHistory{
		ProviderHistory: map[string][]collectors.DailySpend{
			"civo": {
				{Date: "2026-01-01", SpendUSD: 10.0},
				{Date: "2026-01-02", SpendUSD: 20.0},
				{Date: "2026-01-03", SpendUSD: 30.0},
			},
		},
	}

	values := GetProviderHistoryValues(history, "civo")

	if len(values) != 3 {
		t.Errorf("expected 3 values, got %d", len(values))
	}
	if values[0] != 10.0 || values[1] != 20.0 || values[2] != 30.0 {
		t.Errorf("unexpected values: %v", values)
	}
}

func TestGetProviderHistoryValues_Missing(t *testing.T) {
	history := &collectors.BillingHistory{
		ProviderHistory: map[string][]collectors.DailySpend{},
	}

	values := GetProviderHistoryValues(history, "nonexistent")

	if values != nil {
		t.Errorf("expected nil for missing provider, got %v", values)
	}
}

func TestGetTotalHistoryValues(t *testing.T) {
	history := &collectors.BillingHistory{
		TotalHistory: []collectors.DailySpend{
			{Date: "2026-01-01", SpendUSD: 100.0},
			{Date: "2026-01-02", SpendUSD: 200.0},
		},
	}

	values := GetTotalHistoryValues(history)

	if len(values) != 2 {
		t.Errorf("expected 2 values, got %d", len(values))
	}
}

func TestCalculateDaysInMonth(t *testing.T) {
	days := CalculateDaysInMonth()
	if days < 28 || days > 31 {
		t.Errorf("expected days between 28-31, got %d", days)
	}
}

func TestCalculateDaysElapsed(t *testing.T) {
	days := CalculateDaysElapsed()
	if days < 1 || days > 31 {
		t.Errorf("expected days between 1-31, got %d", days)
	}
}
