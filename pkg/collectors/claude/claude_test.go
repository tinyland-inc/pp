package claude

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

// mockAPIClient is a test double for APIClient.
type mockAPIClient struct {
	// responses maps "orgID:startDate:endDate" to a response.
	responses map[string]*APIUsageResponse
	// errs maps "orgID:startDate:endDate" to an error.
	errs map[string]error
	// calls tracks all calls made for assertion.
	calls []mockCall
}

type mockCall struct {
	OrgID     string
	APIKey    string
	StartDate string
	EndDate   string
}

func newMockAPIClient() *mockAPIClient {
	return &mockAPIClient{
		responses: make(map[string]*APIUsageResponse),
		errs:      make(map[string]error),
	}
}

func (m *mockAPIClient) setResponse(orgID, startDate, endDate string, resp *APIUsageResponse) {
	key := orgID + ":" + startDate + ":" + endDate
	m.responses[key] = resp
}

func (m *mockAPIClient) setError(orgID, startDate, endDate string, err error) {
	key := orgID + ":" + startDate + ":" + endDate
	m.errs[key] = err
}

func (m *mockAPIClient) GetOrganizations(ctx context.Context, apiKey string) ([]Organization, error) {
	return nil, errors.New("mock: not configured")
}

func (m *mockAPIClient) GetUsage(ctx context.Context, orgID, apiKey, startDate, endDate string) (*APIUsageResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.calls = append(m.calls, mockCall{
		OrgID:     orgID,
		APIKey:    apiKey,
		StartDate: startDate,
		EndDate:   endDate,
	})
	key := orgID + ":" + startDate + ":" + endDate
	if err, ok := m.errs[key]; ok {
		return nil, err
	}
	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}
	return &APIUsageResponse{}, nil
}

// fixedNow returns a function that always returns the same time, for
// deterministic date range calculations.
func fixedNow() time.Time {
	return time.Date(2026, 2, 9, 15, 30, 0, 0, time.UTC)
}

// buildSingleAccountUsageResponse creates a typical usage response with
// two models over two days.
func buildSingleAccountUsageResponse() *APIUsageResponse {
	return &APIUsageResponse{
		Data: []APIUsageEntry{
			{
				Date:                "2026-02-01",
				Model:               "claude-sonnet-4-5-20250929",
				InputTokens:         500_000,
				OutputTokens:        100_000,
				CacheCreationTokens: 50_000,
				CacheReadTokens:     200_000,
			},
			{
				Date:                "2026-02-02",
				Model:               "claude-opus-4-6-20260115",
				InputTokens:         100_000,
				OutputTokens:        50_000,
				CacheCreationTokens: 10_000,
				CacheReadTokens:     30_000,
			},
		},
	}
}

// --- Tests ---

func TestName(t *testing.T) {
	c := New(Config{}, newMockAPIClient())
	if got := c.Name(); got != "claude" {
		t.Errorf("Name() = %q, want %q", got, "claude")
	}
}

func TestInterval_Default(t *testing.T) {
	c := New(Config{}, newMockAPIClient())
	if got := c.Interval(); got != DefaultInterval {
		t.Errorf("Interval() = %v, want %v", got, DefaultInterval)
	}
}

func TestInterval_Custom(t *testing.T) {
	want := 10 * time.Minute
	c := New(Config{Interval: want}, newMockAPIClient())
	if got := c.Interval(); got != want {
		t.Errorf("Interval() = %v, want %v", got, want)
	}
}

func TestInterval_ZeroUsesDefault(t *testing.T) {
	c := New(Config{Interval: 0}, newMockAPIClient())
	if got := c.Interval(); got != DefaultInterval {
		t.Errorf("Interval() = %v, want default %v", got, DefaultInterval)
	}
}

func TestDefaultConfigValues(t *testing.T) {
	cfg := Config{}
	if cfg.Interval != 0 {
		t.Errorf("default Config.Interval = %v, want 0", cfg.Interval)
	}
	if len(cfg.Accounts) != 0 {
		t.Errorf("default Config.Accounts len = %d, want 0", len(cfg.Accounts))
	}
}

func TestCollect_SingleAccount(t *testing.T) {
	mock := newMockAPIClient()
	resp := buildSingleAccountUsageResponse()

	// Set responses for current month.
	mock.setResponse("org-personal", "2026-02-01", "2026-02-09", resp)

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "personal", AdminAPIKey: "sk-ant-admin-test", OrganizationID: "org-personal"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report, ok := result.(*UsageReport)
	if !ok {
		t.Fatalf("Collect() returned %T, want *UsageReport", result)
	}

	if len(report.Accounts) != 1 {
		t.Fatalf("Accounts len = %d, want 1", len(report.Accounts))
	}

	acct := report.Accounts[0]
	if acct.Name != "personal" {
		t.Errorf("Account.Name = %q, want %q", acct.Name, "personal")
	}
	if !acct.Connected {
		t.Error("Account.Connected = false, want true")
	}
	if acct.Error != "" {
		t.Errorf("Account.Error = %q, want empty", acct.Error)
	}

	// Check current month aggregation.
	if acct.CurrentMonth.InputTokens != 600_000 {
		t.Errorf("CurrentMonth.InputTokens = %d, want 600000", acct.CurrentMonth.InputTokens)
	}
	if acct.CurrentMonth.OutputTokens != 150_000 {
		t.Errorf("CurrentMonth.OutputTokens = %d, want 150000", acct.CurrentMonth.OutputTokens)
	}
	if acct.CurrentMonth.CacheCreationTokens != 60_000 {
		t.Errorf("CurrentMonth.CacheCreationTokens = %d, want 60000", acct.CurrentMonth.CacheCreationTokens)
	}
	if acct.CurrentMonth.CacheReadTokens != 230_000 {
		t.Errorf("CurrentMonth.CacheReadTokens = %d, want 230000", acct.CurrentMonth.CacheReadTokens)
	}

	// Check model breakdown.
	if len(acct.Models) != 2 {
		t.Fatalf("Models len = %d, want 2", len(acct.Models))
	}

	// Verify Sonnet model entry.
	sonnet := acct.Models[0]
	if sonnet.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("Models[0].Model = %q, want claude-sonnet-4-5-20250929", sonnet.Model)
	}
	if sonnet.InputTokens != 500_000 {
		t.Errorf("Models[0].InputTokens = %d, want 500000", sonnet.InputTokens)
	}
	if sonnet.OutputTokens != 100_000 {
		t.Errorf("Models[0].OutputTokens = %d, want 100000", sonnet.OutputTokens)
	}
}

func TestCollect_MultiAccount(t *testing.T) {
	mock := newMockAPIClient()

	// Account 1: personal
	mock.setResponse("org-personal", "2026-02-01", "2026-02-09", &APIUsageResponse{
		Data: []APIUsageEntry{
			{
				Date:         "2026-02-01",
				Model:        "claude-sonnet-4-5-20250929",
				InputTokens:  1_000_000,
				OutputTokens: 500_000,
			},
		},
	})

	// Account 2: work
	mock.setResponse("org-work", "2026-02-01", "2026-02-09", &APIUsageResponse{
		Data: []APIUsageEntry{
			{
				Date:         "2026-02-01",
				Model:        "claude-opus-4-6-20260115",
				InputTokens:  200_000,
				OutputTokens: 100_000,
			},
		},
	})

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "personal", AdminAPIKey: "sk-1", OrganizationID: "org-personal"},
			{Name: "work", AdminAPIKey: "sk-2", OrganizationID: "org-work"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*UsageReport)

	if len(report.Accounts) != 2 {
		t.Fatalf("Accounts len = %d, want 2", len(report.Accounts))
	}

	if report.Accounts[0].Name != "personal" {
		t.Errorf("Accounts[0].Name = %q, want personal", report.Accounts[0].Name)
	}
	if report.Accounts[1].Name != "work" {
		t.Errorf("Accounts[1].Name = %q, want work", report.Accounts[1].Name)
	}

	// Both should be connected.
	if !report.Accounts[0].Connected {
		t.Error("Accounts[0].Connected = false, want true")
	}
	if !report.Accounts[1].Connected {
		t.Error("Accounts[1].Connected = false, want true")
	}

	// TotalCostUSD should be the sum of both accounts' current month costs.
	personalCost := report.Accounts[0].CurrentMonth.CostUSD
	workCost := report.Accounts[1].CurrentMonth.CostUSD
	expectedTotal := personalCost + workCost

	if math.Abs(report.TotalCostUSD-expectedTotal) > 0.001 {
		t.Errorf("TotalCostUSD = %f, want %f", report.TotalCostUSD, expectedTotal)
	}
}

func TestCollect_APIErrorOnOneAccount(t *testing.T) {
	mock := newMockAPIClient()

	// Account 1: works fine.
	mock.setResponse("org-good", "2026-02-01", "2026-02-09", &APIUsageResponse{
		Data: []APIUsageEntry{
			{
				Date:         "2026-02-01",
				Model:        "claude-sonnet-4-5-20250929",
				InputTokens:  1_000_000,
				OutputTokens: 200_000,
			},
		},
	})

	// Account 2: API error.
	mock.setError("org-bad", "2026-02-01", "2026-02-09", errors.New("API returned status 401: unauthorized"))

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "good", AdminAPIKey: "sk-good", OrganizationID: "org-good"},
			{Name: "bad", AdminAPIKey: "sk-bad", OrganizationID: "org-bad"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() should not return error when partial accounts fail: %v", err)
	}

	report := result.(*UsageReport)

	// Good account should be connected.
	if !report.Accounts[0].Connected {
		t.Error("good account: Connected = false, want true")
	}
	if report.Accounts[0].Error != "" {
		t.Errorf("good account: Error = %q, want empty", report.Accounts[0].Error)
	}

	// Bad account should be disconnected with error.
	if report.Accounts[1].Connected {
		t.Error("bad account: Connected = true, want false")
	}
	if report.Accounts[1].Error == "" {
		t.Error("bad account: Error is empty, want error message")
	}

	// Collector should still be healthy (one account works).
	if !c.Healthy() {
		t.Error("Healthy() = false, want true (one account succeeded)")
	}
}

func TestCollect_AllAccountsFail(t *testing.T) {
	mock := newMockAPIClient()

	mock.setError("org-a", "2026-02-01", "2026-02-09", errors.New("network error"))
	mock.setError("org-b", "2026-02-01", "2026-02-09", errors.New("auth error"))

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "a", AdminAPIKey: "sk-a", OrganizationID: "org-a"},
			{Name: "b", AdminAPIKey: "sk-b", OrganizationID: "org-b"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() should return result even when all accounts fail: %v", err)
	}

	report := result.(*UsageReport)

	for i, acct := range report.Accounts {
		if acct.Connected {
			t.Errorf("Accounts[%d].Connected = true, want false", i)
		}
		if acct.Error == "" {
			t.Errorf("Accounts[%d].Error is empty, want error message", i)
		}
	}

	if c.Healthy() {
		t.Error("Healthy() = true, want false (all accounts failed)")
	}
}

func TestCostCalculation_Sonnet(t *testing.T) {
	// Sonnet: $3/M input, $15/M output
	cost := CalculateCost("claude-sonnet-4-5-20250929", 1_000_000, 1_000_000, 0, 0)
	expected := 3.0 + 15.0 // $18
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("Sonnet cost = %f, want %f", cost, expected)
	}
}

func TestCostCalculation_Opus(t *testing.T) {
	// Opus: $15/M input, $75/M output
	cost := CalculateCost("claude-opus-4-6", 1_000_000, 1_000_000, 0, 0)
	expected := 15.0 + 75.0 // $90
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("Opus cost = %f, want %f", cost, expected)
	}
}

func TestCostCalculation_Haiku(t *testing.T) {
	// Haiku 4.5: $0.80/M input, $4/M output
	cost := CalculateCost("claude-haiku-4-5-20251001", 1_000_000, 1_000_000, 0, 0)
	expected := 0.80 + 4.0 // $4.80
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("Haiku cost = %f, want %f", cost, expected)
	}
}

func TestCostCalculation_WithCache(t *testing.T) {
	// Sonnet: cache creation $3.75/M, cache read $0.30/M
	cost := CalculateCost("claude-sonnet-4-5-20250929", 0, 0, 1_000_000, 1_000_000)
	expected := 3.75 + 0.30
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("Cache cost = %f, want %f", cost, expected)
	}
}

func TestCostCalculation_UnknownModel(t *testing.T) {
	// Unknown model should use fallback (Sonnet-tier) pricing.
	cost := CalculateCost("claude-future-5-0-99990101", 1_000_000, 1_000_000, 0, 0)
	expected := 3.0 + 15.0 // fallback = Sonnet pricing
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("Unknown model cost = %f, want %f (fallback)", cost, expected)
	}
}

func TestCostCalculation_ZeroTokens(t *testing.T) {
	cost := CalculateCost("claude-sonnet-4-5-20250929", 0, 0, 0, 0)
	if cost != 0.0 {
		t.Errorf("Zero tokens cost = %f, want 0", cost)
	}
}

func TestLookupPricing_ExactMatch(t *testing.T) {
	p := LookupPricing("claude-opus-4-6")
	if p.InputPer1M != 15.0 {
		t.Errorf("Opus InputPer1M = %f, want 15.0", p.InputPer1M)
	}
}

func TestLookupPricing_PrefixMatch(t *testing.T) {
	// "claude-sonnet-4-5-20250929" should prefix-match "claude-sonnet-4-5"
	p := LookupPricing("claude-sonnet-4-5-20250929")
	if p.InputPer1M != 3.0 {
		t.Errorf("Sonnet variant InputPer1M = %f, want 3.0", p.InputPer1M)
	}
}

func TestLookupPricing_LongestPrefixWins(t *testing.T) {
	// "claude-3-5-sonnet-20241022" should match "claude-3-5-sonnet" not "claude-3"
	p := LookupPricing("claude-3-5-sonnet-20241022")
	if p.InputPer1M != 3.0 {
		t.Errorf("3.5 Sonnet InputPer1M = %f, want 3.0", p.InputPer1M)
	}
}

func TestLookupPricing_Fallback(t *testing.T) {
	p := LookupPricing("totally-unknown-model")
	if p.InputPer1M != fallbackPricing.InputPer1M {
		t.Errorf("Fallback InputPer1M = %f, want %f", p.InputPer1M, fallbackPricing.InputPer1M)
	}
}

func TestDateRange_CurrentMonth(t *testing.T) {
	now := time.Date(2026, 2, 9, 15, 30, 0, 0, time.UTC)
	start, end := currentMonthRange(now)
	if start != "2026-02-01" {
		t.Errorf("currentMonthRange start = %q, want 2026-02-01", start)
	}
	if end != "2026-02-09" {
		t.Errorf("currentMonthRange end = %q, want 2026-02-09", end)
	}
}

func TestDateRange_PreviousMonth(t *testing.T) {
	now := time.Date(2026, 2, 9, 15, 30, 0, 0, time.UTC)
	start, end := previousMonthRange(now)
	if start != "2026-01-01" {
		t.Errorf("previousMonthRange start = %q, want 2026-01-01", start)
	}
	if end != "2026-01-31" {
		t.Errorf("previousMonthRange end = %q, want 2026-01-31", end)
	}
}

func TestDateRange_JanuaryPreviousMonth(t *testing.T) {
	// January should look back to December of previous year.
	now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	start, end := previousMonthRange(now)
	if start != "2025-12-01" {
		t.Errorf("previousMonthRange start = %q, want 2025-12-01", start)
	}
	if end != "2025-12-31" {
		t.Errorf("previousMonthRange end = %q, want 2025-12-31", end)
	}
}

func TestDateRange_FirstDayOfMonth(t *testing.T) {
	// On the 1st, current month range is just that one day.
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	start, end := currentMonthRange(now)
	if start != "2026-03-01" {
		t.Errorf("start = %q, want 2026-03-01", start)
	}
	if end != "2026-03-01" {
		t.Errorf("end = %q, want 2026-03-01", end)
	}
}

func TestCollect_EmptyUsageResponse(t *testing.T) {
	mock := newMockAPIClient()
	mock.setResponse("org-empty", "2026-02-01", "2026-02-09", &APIUsageResponse{
		Data: []APIUsageEntry{},
	})

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "empty", AdminAPIKey: "sk-empty", OrganizationID: "org-empty"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*UsageReport)
	acct := report.Accounts[0]

	if !acct.Connected {
		t.Error("Connected = false, want true (empty response is still a success)")
	}
	if acct.CurrentMonth.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", acct.CurrentMonth.InputTokens)
	}
	if acct.CurrentMonth.CostUSD != 0.0 {
		t.Errorf("CostUSD = %f, want 0", acct.CurrentMonth.CostUSD)
	}
	if len(acct.Models) != 0 {
		t.Errorf("Models len = %d, want 0", len(acct.Models))
	}
}

func TestHealthy_InitiallyTrue(t *testing.T) {
	c := New(Config{}, newMockAPIClient())
	if !c.Healthy() {
		t.Error("Healthy() = false before first collection, want true")
	}
}

func TestHealthy_ToggleOnFailThenRecover(t *testing.T) {
	mock := newMockAPIClient()

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "test", AdminAPIKey: "sk", OrganizationID: "org-test"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	// Fail the first call.
	mock.setError("org-test", "2026-02-01", "2026-02-09", errors.New("down"))

	_, _ = c.Collect(context.Background())
	if c.Healthy() {
		t.Error("Healthy() = true after failure, want false")
	}

	// Recover.
	delete(mock.errs, "org-test:2026-02-01:2026-02-09")
	mock.setResponse("org-test", "2026-02-01", "2026-02-09", &APIUsageResponse{})

	_, _ = c.Collect(context.Background())
	if !c.Healthy() {
		t.Error("Healthy() = false after recovery, want true")
	}
}

func TestCollect_ContextCancellation(t *testing.T) {
	mock := newMockAPIClient()

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "test", AdminAPIKey: "sk", OrganizationID: "org-test"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := c.Collect(ctx)
	if err == nil {
		t.Fatal("Collect() should return error for cancelled context")
	}

	if c.Healthy() {
		t.Error("Healthy() = true after context cancellation, want false")
	}
}

func TestCollect_NoAccounts(t *testing.T) {
	mock := newMockAPIClient()

	c := New(Config{Accounts: nil}, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*UsageReport)
	if len(report.Accounts) != 0 {
		t.Errorf("Accounts len = %d, want 0", len(report.Accounts))
	}
	if report.TotalCostUSD != 0.0 {
		t.Errorf("TotalCostUSD = %f, want 0", report.TotalCostUSD)
	}

	// No accounts means healthy by default.
	if !c.Healthy() {
		t.Error("Healthy() = false with no accounts, want true")
	}
}

func TestCollect_PreviousMonthPopulated(t *testing.T) {
	mock := newMockAPIClient()

	// Current month.
	mock.setResponse("org-1", "2026-02-01", "2026-02-09", &APIUsageResponse{
		Data: []APIUsageEntry{
			{Model: "claude-sonnet-4-5-20250929", InputTokens: 100_000, OutputTokens: 50_000},
		},
	})

	// Previous month.
	mock.setResponse("org-1", "2026-01-01", "2026-01-31", &APIUsageResponse{
		Data: []APIUsageEntry{
			{Model: "claude-sonnet-4-5-20250929", InputTokens: 2_000_000, OutputTokens: 1_000_000},
		},
	})

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "test", AdminAPIKey: "sk", OrganizationID: "org-1"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*UsageReport)
	acct := report.Accounts[0]

	if acct.PreviousMonth.InputTokens != 2_000_000 {
		t.Errorf("PreviousMonth.InputTokens = %d, want 2000000", acct.PreviousMonth.InputTokens)
	}
	if acct.PreviousMonth.OutputTokens != 1_000_000 {
		t.Errorf("PreviousMonth.OutputTokens = %d, want 1000000", acct.PreviousMonth.OutputTokens)
	}
	if acct.PreviousMonth.CostUSD <= 0 {
		t.Error("PreviousMonth.CostUSD should be > 0")
	}
}

func TestCollect_PreviousMonthError_StillSucceeds(t *testing.T) {
	mock := newMockAPIClient()

	// Current month works.
	mock.setResponse("org-1", "2026-02-01", "2026-02-09", &APIUsageResponse{
		Data: []APIUsageEntry{
			{Model: "claude-sonnet-4-5-20250929", InputTokens: 100_000, OutputTokens: 50_000},
		},
	})

	// Previous month fails.
	mock.setError("org-1", "2026-01-01", "2026-01-31", errors.New("not available"))

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "test", AdminAPIKey: "sk", OrganizationID: "org-1"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*UsageReport)
	acct := report.Accounts[0]

	// Should still be connected (current month succeeded).
	if !acct.Connected {
		t.Error("Connected = false, want true (current month worked)")
	}

	// Previous month should be zero.
	if acct.PreviousMonth.InputTokens != 0 {
		t.Errorf("PreviousMonth.InputTokens = %d, want 0", acct.PreviousMonth.InputTokens)
	}
}

func TestCollect_Timestamp(t *testing.T) {
	mock := newMockAPIClient()
	c := New(Config{}, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*UsageReport)
	expected := fixedNow()
	if !report.Timestamp.Equal(expected) {
		t.Errorf("Timestamp = %v, want %v", report.Timestamp, expected)
	}
}

func TestCollect_APIKeyPassedCorrectly(t *testing.T) {
	mock := newMockAPIClient()

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "test", AdminAPIKey: "sk-ant-admin-secret-123", OrganizationID: "org-xyz"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	_, _ = c.Collect(context.Background())

	// At least one call should have been made with the correct key.
	found := false
	for _, call := range mock.calls {
		if call.APIKey == "sk-ant-admin-secret-123" && call.OrgID == "org-xyz" {
			found = true
			break
		}
	}
	if !found {
		t.Error("API key was not passed to the client correctly")
	}
}

func TestCollect_ModelAggregation(t *testing.T) {
	mock := newMockAPIClient()

	// Same model appears on multiple days -- should be aggregated.
	mock.setResponse("org-1", "2026-02-01", "2026-02-09", &APIUsageResponse{
		Data: []APIUsageEntry{
			{Date: "2026-02-01", Model: "claude-sonnet-4-5-20250929", InputTokens: 100_000, OutputTokens: 50_000},
			{Date: "2026-02-02", Model: "claude-sonnet-4-5-20250929", InputTokens: 200_000, OutputTokens: 100_000},
			{Date: "2026-02-01", Model: "claude-opus-4-6", InputTokens: 10_000, OutputTokens: 5_000},
		},
	})

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "test", AdminAPIKey: "sk", OrganizationID: "org-1"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*UsageReport)
	acct := report.Accounts[0]

	if len(acct.Models) != 2 {
		t.Fatalf("Models len = %d, want 2", len(acct.Models))
	}

	// Sonnet should be aggregated across days.
	var sonnet *ModelUsage
	for i := range acct.Models {
		if acct.Models[i].Model == "claude-sonnet-4-5-20250929" {
			sonnet = &acct.Models[i]
			break
		}
	}
	if sonnet == nil {
		t.Fatal("Sonnet model not found in aggregation")
	}
	if sonnet.InputTokens != 300_000 {
		t.Errorf("Sonnet aggregated InputTokens = %d, want 300000", sonnet.InputTokens)
	}
	if sonnet.OutputTokens != 150_000 {
		t.Errorf("Sonnet aggregated OutputTokens = %d, want 150000", sonnet.OutputTokens)
	}
}

func TestCollect_NilClient(t *testing.T) {
	// Passing nil client should create a default HTTPClient (not panic).
	c := New(Config{}, nil)
	if c.client == nil {
		t.Fatal("client should not be nil when nil is passed to New")
	}
}

func TestCollect_BurnRate(t *testing.T) {
	mock := newMockAPIClient()

	// Usage on Feb 9 with known costs.
	mock.setResponse("org-1", "2026-02-01", "2026-02-09", &APIUsageResponse{
		Data: []APIUsageEntry{
			{
				Date:         "2026-02-01",
				Model:        "claude-sonnet-4-5-20250929",
				InputTokens:  1_000_000,
				OutputTokens: 500_000,
			},
		},
	})

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "test", AdminAPIKey: "sk-admin", OrganizationID: "org-1"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow // Feb 9

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*UsageReport)
	acct := report.Accounts[0]

	// Verify burn rate fields are populated.
	if acct.DailyBurnRate <= 0 {
		t.Errorf("DailyBurnRate = %f, want > 0", acct.DailyBurnRate)
	}
	if acct.ProjectedMonthly <= 0 {
		t.Errorf("ProjectedMonthly = %f, want > 0", acct.ProjectedMonthly)
	}

	// Feb 9: 9 days elapsed, 28 days in Feb 2026.
	expectedBurnRate := acct.CurrentMonth.CostUSD / 9.0
	if math.Abs(acct.DailyBurnRate-expectedBurnRate) > 0.001 {
		t.Errorf("DailyBurnRate = %f, want %f", acct.DailyBurnRate, expectedBurnRate)
	}

	expectedProjected := expectedBurnRate * 28.0
	if math.Abs(acct.ProjectedMonthly-expectedProjected) > 0.001 {
		t.Errorf("ProjectedMonthly = %f, want %f", acct.ProjectedMonthly, expectedProjected)
	}

	// Days remaining: 28 - 9 = 19.
	if acct.DaysRemaining != 19 {
		t.Errorf("DaysRemaining = %d, want 19", acct.DaysRemaining)
	}
}

func TestCollect_BurnRate_DisconnectedAccount(t *testing.T) {
	mock := newMockAPIClient()
	mock.setError("org-fail", "2026-02-01", "2026-02-09", errors.New("auth error"))

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "fail", AdminAPIKey: "sk-admin", OrganizationID: "org-fail"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = fixedNow

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*UsageReport)
	acct := report.Accounts[0]

	// Disconnected accounts should have zero burn rate.
	if acct.DailyBurnRate != 0 {
		t.Errorf("DailyBurnRate = %f, want 0 (disconnected)", acct.DailyBurnRate)
	}
	if acct.ProjectedMonthly != 0 {
		t.Errorf("ProjectedMonthly = %f, want 0 (disconnected)", acct.ProjectedMonthly)
	}
	if acct.DaysRemaining != 0 {
		t.Errorf("DaysRemaining = %d, want 0 (disconnected)", acct.DaysRemaining)
	}
}

func TestCollect_BurnRate_FirstDayOfMonth(t *testing.T) {
	mock := newMockAPIClient()

	// March 1st — only 1 day elapsed.
	march1 := func() time.Time {
		return time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	}

	mock.setResponse("org-1", "2026-03-01", "2026-03-01", &APIUsageResponse{
		Data: []APIUsageEntry{
			{
				Date:         "2026-03-01",
				Model:        "claude-sonnet-4-5-20250929",
				InputTokens:  100_000,
				OutputTokens: 50_000,
			},
		},
	})

	cfg := Config{
		Accounts: []AccountConfig{
			{Name: "test", AdminAPIKey: "sk-admin", OrganizationID: "org-1"},
		},
	}

	c := New(cfg, mock)
	c.nowFunc = march1

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*UsageReport)
	acct := report.Accounts[0]

	// Burn rate = cost / 1 day.
	if math.Abs(acct.DailyBurnRate-acct.CurrentMonth.CostUSD) > 0.001 {
		t.Errorf("DailyBurnRate = %f, want %f (1 day)", acct.DailyBurnRate, acct.CurrentMonth.CostUSD)
	}

	// March has 31 days; remaining = 30.
	if acct.DaysRemaining != 30 {
		t.Errorf("DaysRemaining = %d, want 30 (March 1)", acct.DaysRemaining)
	}
}

// Compile-time check that Collector satisfies the duck-typed interface.
type collectorIface interface {
	Name() string
	Collect(ctx context.Context) (interface{}, error)
	Interval() time.Duration
	Healthy() bool
}

var _ collectorIface = (*Collector)(nil)

// Ensure the mock satisfies APIClient.
var _ APIClient = (*mockAPIClient)(nil)
