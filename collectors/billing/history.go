// Package billing provides history tracking for billing data sparklines.
package billing

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// historyFileName is the cache file for billing history.
	historyFileName = "billing_history.json"
	// maxHistoryDays is the maximum number of days to retain in history.
	maxHistoryDays = 30
)

// HistoryStore manages persistent billing history for sparkline visualization.
type HistoryStore struct {
	cacheDir string
	logger   *slog.Logger
}

// NewHistoryStore creates a HistoryStore using the specified cache directory.
func NewHistoryStore(cacheDir string, logger *slog.Logger) *HistoryStore {
	return &HistoryStore{
		cacheDir: cacheDir,
		logger:   logger,
	}
}

// historyPath returns the full path to the history file.
func (h *HistoryStore) historyPath() string {
	return filepath.Join(h.cacheDir, historyFileName)
}

// Load reads the billing history from disk.
// Returns an empty history if the file does not exist or is corrupted.
func (h *HistoryStore) Load() *collectors.BillingHistory {
	data, err := os.ReadFile(h.historyPath())
	if err != nil {
		if !os.IsNotExist(err) {
			h.logger.Warn("billing history: failed to read", "error", err)
		}
		return &collectors.BillingHistory{
			ProviderHistory: make(map[string][]collectors.DailySpend),
			TotalHistory:    []collectors.DailySpend{},
		}
	}

	var history collectors.BillingHistory
	if err := json.Unmarshal(data, &history); err != nil {
		h.logger.Warn("billing history: failed to parse", "error", err)
		return &collectors.BillingHistory{
			ProviderHistory: make(map[string][]collectors.DailySpend),
			TotalHistory:    []collectors.DailySpend{},
		}
	}

	// Initialize map if nil.
	if history.ProviderHistory == nil {
		history.ProviderHistory = make(map[string][]collectors.DailySpend)
	}

	return &history
}

// Save writes the billing history to disk with atomic write.
func (h *HistoryStore) Save(history *collectors.BillingHistory) error {
	if history == nil {
		return nil
	}

	// Ensure cache directory exists.
	if err := os.MkdirAll(h.cacheDir, 0700); err != nil {
		return fmt.Errorf("billing history: create cache dir: %w", err)
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("billing history: marshal: %w", err)
	}

	// Atomic write via temp file and rename.
	tmpFile := h.historyPath() + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("billing history: write temp: %w", err)
	}
	if err := os.Rename(tmpFile, h.historyPath()); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("billing history: rename: %w", err)
	}

	return nil
}

// Update adds today's spend data to the history and prunes old entries.
// It merges the current provider data with existing history.
func (h *HistoryStore) Update(history *collectors.BillingHistory, providers []collectors.ProviderBilling) *collectors.BillingHistory {
	if history == nil {
		history = &collectors.BillingHistory{
			ProviderHistory: make(map[string][]collectors.DailySpend),
			TotalHistory:    []collectors.DailySpend{},
		}
	}

	today := time.Now().Format("2006-01-02")
	var totalToday float64

	// Update per-provider history.
	for _, p := range providers {
		if p.Status == "error" {
			continue
		}

		providerHistory := history.ProviderHistory[p.Provider]

		// Find or create today's entry.
		found := false
		for i := range providerHistory {
			if providerHistory[i].Date == today {
				providerHistory[i].SpendUSD = p.CurrentMonth.SpendUSD
				found = true
				break
			}
		}
		if !found {
			providerHistory = append(providerHistory, collectors.DailySpend{
				Date:     today,
				SpendUSD: p.CurrentMonth.SpendUSD,
			})
		}

		// Sort by date and prune old entries.
		providerHistory = pruneAndSort(providerHistory, maxHistoryDays)
		history.ProviderHistory[p.Provider] = providerHistory

		totalToday += p.CurrentMonth.SpendUSD
	}

	// Update total history.
	foundTotal := false
	for i := range history.TotalHistory {
		if history.TotalHistory[i].Date == today {
			history.TotalHistory[i].SpendUSD = totalToday
			foundTotal = true
			break
		}
	}
	if !foundTotal {
		history.TotalHistory = append(history.TotalHistory, collectors.DailySpend{
			Date:     today,
			SpendUSD: totalToday,
		})
	}
	history.TotalHistory = pruneAndSort(history.TotalHistory, maxHistoryDays)

	history.LastUpdated = time.Now()

	return history
}

// pruneAndSort sorts entries by date and removes entries older than maxDays.
func pruneAndSort(entries []collectors.DailySpend, maxDays int) []collectors.DailySpend {
	if len(entries) == 0 {
		return entries
	}

	// Sort by date ascending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date < entries[j].Date
	})

	// Calculate cutoff date.
	cutoff := time.Now().AddDate(0, 0, -maxDays).Format("2006-01-02")

	// Filter entries after cutoff.
	var pruned []collectors.DailySpend
	for _, e := range entries {
		if e.Date >= cutoff {
			pruned = append(pruned, e)
		}
	}

	return pruned
}

// CalculateForecast computes a linear forecast based on current spend and days elapsed.
// Returns the projected end-of-month spend.
func CalculateForecast(currentSpend float64, daysElapsed, daysInMonth int) float64 {
	if daysElapsed <= 0 {
		return currentSpend
	}
	dailyRate := currentSpend / float64(daysElapsed)
	return dailyRate * float64(daysInMonth)
}

// CalculateDaysInMonth returns the number of days in the current month.
func CalculateDaysInMonth() int {
	now := time.Now()
	return time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
}

// CalculateDaysElapsed returns the number of days elapsed in the current month.
func CalculateDaysElapsed() int {
	return time.Now().Day()
}

// GetProviderHistoryValues extracts spend values from provider history for sparkline rendering.
func GetProviderHistoryValues(history *collectors.BillingHistory, provider string) []float64 {
	if history == nil {
		return nil
	}
	entries, ok := history.ProviderHistory[provider]
	if !ok || len(entries) == 0 {
		return nil
	}
	return collectors.GetSpendValues(entries)
}

// GetTotalHistoryValues extracts spend values from total history for sparkline rendering.
func GetTotalHistoryValues(history *collectors.BillingHistory) []float64 {
	if history == nil || len(history.TotalHistory) == 0 {
		return nil
	}
	return collectors.GetSpendValues(history.TotalHistory)
}
