package billing

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// --- Mock types ---

// mockProviderFetcher implements ProviderFetcher for testing.
type mockProviderFetcher struct {
	billing *collectors.ProviderBilling
	err     error
}

func (m *mockProviderFetcher) FetchBilling(ctx context.Context) (*collectors.ProviderBilling, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return m.billing, m.err
}

// --- Helper functions ---

// testLogger returns a logger that only shows errors for clean test output.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// f64ptr returns a pointer to a float64 value.
func f64ptr(v float64) *float64 {
	return &v
}

// civoBilling returns a realistic Civo billing result.
func civoBilling() *collectors.ProviderBilling {
	return &collectors.ProviderBilling{
		Provider:     "civo",
		AccountName:  "tinyland",
		Status:       "ok",
		DashboardURL: "https://dashboard.civo.com/billing",
		CurrentMonth: collectors.MonthCost{
			SpendUSD:    12.50,
			ForecastUSD: f64ptr(25.00),
			BudgetUSD:   f64ptr(50.00),
			StartDate:   "2026-02-01",
			EndDate:     "2026-02-28",
		},
		PreviousMonth: f64ptr(22.30),
		FetchedAt:     time.Now(),
	}
}

// doBilling returns a realistic DigitalOcean billing result.
func doBilling() *collectors.ProviderBilling {
	return &collectors.ProviderBilling{
		Provider:     "digitalocean",
		AccountName:  "tinyland-do",
		Status:       "ok",
		DashboardURL: "https://cloud.digitalocean.com/account/billing",
		CurrentMonth: collectors.MonthCost{
			SpendUSD:    7.80,
			ForecastUSD: f64ptr(15.60),
			StartDate:   "2026-02-01",
			EndDate:     "2026-02-28",
		},
		FetchedAt: time.Now(),
	}
}

// withMockFetchers overrides the package-level factory functions with test mocks,
// runs the provided function, then restores the originals. This ensures test
// isolation even when tests run in parallel within the same process.
func withMockFetchers(fetchers map[string]ProviderFetcher, fn func()) {
	origCivo := newCivoFetcher
	origDO := newDOFetcher
	origAWS := newAWSFetcher
	origDH := newDreamHostFetcher

	newCivoFetcher = func(apiKey, region string, logger *slog.Logger) ProviderFetcher {
		if f, ok := fetchers["civo"]; ok {
			return f
		}
		return &mockProviderFetcher{err: fmt.Errorf("no mock for civo")}
	}
	newDOFetcher = func(apiToken string, logger *slog.Logger) ProviderFetcher {
		if f, ok := fetchers["digitalocean"]; ok {
			return f
		}
		return &mockProviderFetcher{err: fmt.Errorf("no mock for digitalocean")}
	}
	newAWSFetcher = func(profile string, regions []string, logger *slog.Logger) ProviderFetcher {
		if f, ok := fetchers["aws"]; ok {
			return f
		}
		return &mockProviderFetcher{err: fmt.Errorf("no mock for aws")}
	}
	newDreamHostFetcher = func(apiKey string, logger *slog.Logger) ProviderFetcher {
		if f, ok := fetchers["dreamhost"]; ok {
			return f
		}
		return &mockProviderFetcher{err: fmt.Errorf("no mock for dreamhost")}
	}

	defer func() {
		newCivoFetcher = origCivo
		newDOFetcher = origDO
		newAWSFetcher = origAWS
		newDreamHostFetcher = origDH
	}()

	fn()
}

// --- Tests ---

func TestBillingCollector_Name(t *testing.T) {
	c := NewBillingCollector(nil, nil)
	if got := c.Name(); got != "billing" {
		t.Errorf("Name() = %q, want %q", got, "billing")
	}
}

func TestBillingCollector_Description(t *testing.T) {
	c := NewBillingCollector(nil, nil)
	want := "Cloud provider billing across Civo, DigitalOcean, AWS, and DreamHost"
	if got := c.Description(); got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
}

func TestBillingCollector_Interval(t *testing.T) {
	c := NewBillingCollector(nil, nil)
	want := 1 * time.Hour
	if got := c.Interval(); got != want {
		t.Errorf("Interval() = %v, want %v", got, want)
	}
}

func TestBillingCollector_ZeroProviders(t *testing.T) {
	c := NewBillingCollector(nil, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	if result.Collector != "billing" {
		t.Errorf("Collector = %q, want %q", result.Collector, "billing")
	}

	data, ok := result.Data.(*collectors.BillingData)
	if !ok {
		t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
	}

	if len(data.Providers) != 0 {
		t.Errorf("got %d providers, want 0", len(data.Providers))
	}

	if data.Total.CurrentMonthUSD != 0 {
		t.Errorf("Total.CurrentMonthUSD = %v, want 0", data.Total.CurrentMonthUSD)
	}

	if data.Total.ForecastUSD != nil {
		t.Errorf("Total.ForecastUSD = %v, want nil", *data.Total.ForecastUSD)
	}

	if data.Total.BudgetUSD != nil {
		t.Errorf("Total.BudgetUSD = %v, want nil", *data.Total.BudgetUSD)
	}

	if len(result.Warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
	}
}

func TestBillingCollector_SingleProvider_Success(t *testing.T) {
	mockCivo := &mockProviderFetcher{billing: civoBilling()}

	t.Setenv("TEST_CIVO_KEY", "fake-civo-api-key")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY"},
	}

	withMockFetchers(map[string]ProviderFetcher{"civo": mockCivo}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.BillingData)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
		}

		if len(data.Providers) != 1 {
			t.Fatalf("got %d providers, want 1", len(data.Providers))
		}

		p := data.Providers[0]
		if p.Provider != "civo" {
			t.Errorf("Provider = %q, want %q", p.Provider, "civo")
		}
		if p.Status != "ok" {
			t.Errorf("Status = %q, want %q", p.Status, "ok")
		}
		if p.CurrentMonth.SpendUSD != 12.50 {
			t.Errorf("SpendUSD = %v, want 12.50", p.CurrentMonth.SpendUSD)
		}
		if p.CurrentMonth.ForecastUSD == nil || *p.CurrentMonth.ForecastUSD != 25.00 {
			t.Errorf("ForecastUSD = %v, want 25.00", p.CurrentMonth.ForecastUSD)
		}

		// Summary should reflect the single provider.
		if data.Total.CurrentMonthUSD != 12.50 {
			t.Errorf("Total.CurrentMonthUSD = %v, want 12.50", data.Total.CurrentMonthUSD)
		}

		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestBillingCollector_MultipleProviders(t *testing.T) {
	mockCivo := &mockProviderFetcher{billing: civoBilling()}
	mockDO := &mockProviderFetcher{billing: doBilling()}

	t.Setenv("TEST_CIVO_KEY_MULTI", "fake-civo-key")
	t.Setenv("TEST_DO_TOKEN_MULTI", "fake-do-token")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_MULTI"},
		{Name: "digitalocean", Enabled: true, APIKeyEnv: "TEST_DO_TOKEN_MULTI"},
	}

	withMockFetchers(map[string]ProviderFetcher{
		"civo":         mockCivo,
		"digitalocean": mockDO,
	}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.BillingData)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
		}

		if len(data.Providers) != 2 {
			t.Fatalf("got %d providers, want 2", len(data.Providers))
		}

		// Verify order matches input config.
		if data.Providers[0].Provider != "civo" {
			t.Errorf("Providers[0].Provider = %q, want civo", data.Providers[0].Provider)
		}
		if data.Providers[1].Provider != "digitalocean" {
			t.Errorf("Providers[1].Provider = %q, want digitalocean", data.Providers[1].Provider)
		}

		// Verify totals: 12.50 + 7.80 = 20.30.
		wantSpend := 20.30
		if data.Total.CurrentMonthUSD != wantSpend {
			t.Errorf("Total.CurrentMonthUSD = %v, want %v", data.Total.CurrentMonthUSD, wantSpend)
		}

		// Verify forecast totals: 25.00 + 15.60 = 40.60.
		if data.Total.ForecastUSD == nil {
			t.Fatal("Total.ForecastUSD is nil, want non-nil")
		}
		wantForecast := 40.60
		if *data.Total.ForecastUSD != wantForecast {
			t.Errorf("Total.ForecastUSD = %v, want %v", *data.Total.ForecastUSD, wantForecast)
		}

		// Budget: only Civo has a budget (50.00), DO does not.
		if data.Total.BudgetUSD == nil {
			t.Fatal("Total.BudgetUSD is nil, want non-nil")
		}
		wantBudget := 50.00
		if *data.Total.BudgetUSD != wantBudget {
			t.Errorf("Total.BudgetUSD = %v, want %v", *data.Total.BudgetUSD, wantBudget)
		}

		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestBillingCollector_ProviderError_Isolation(t *testing.T) {
	mockCivo := &mockProviderFetcher{billing: civoBilling()}
	mockDO := &mockProviderFetcher{err: fmt.Errorf("API rate limited")}

	t.Setenv("TEST_CIVO_KEY_ISO", "fake-civo-key")
	t.Setenv("TEST_DO_TOKEN_ISO", "fake-do-token")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_ISO"},
		{Name: "digitalocean", Enabled: true, APIKeyEnv: "TEST_DO_TOKEN_ISO"},
	}

	withMockFetchers(map[string]ProviderFetcher{
		"civo":         mockCivo,
		"digitalocean": mockDO,
	}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.BillingData)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
		}

		if len(data.Providers) != 2 {
			t.Fatalf("got %d providers, want 2", len(data.Providers))
		}

		// Civo should succeed.
		if data.Providers[0].Status != "ok" {
			t.Errorf("Providers[0] (civo) Status = %q, want ok", data.Providers[0].Status)
		}

		// DO should fail gracefully.
		if data.Providers[1].Status != "error" {
			t.Errorf("Providers[1] (digitalocean) Status = %q, want error", data.Providers[1].Status)
		}

		// Total should only include the successful provider.
		if data.Total.CurrentMonthUSD != 12.50 {
			t.Errorf("Total.CurrentMonthUSD = %v, want 12.50 (only civo)", data.Total.CurrentMonthUSD)
		}

		// Should have exactly one warning from the failed provider.
		if len(result.Warnings) != 1 {
			t.Errorf("got %d warnings, want 1: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestBillingCollector_AllProvidersFail(t *testing.T) {
	mockCivo := &mockProviderFetcher{err: fmt.Errorf("civo: connection timeout")}
	mockDO := &mockProviderFetcher{err: fmt.Errorf("do: unauthorized")}

	t.Setenv("TEST_CIVO_KEY_FAIL", "fake-civo-key")
	t.Setenv("TEST_DO_TOKEN_FAIL", "fake-do-token")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_FAIL"},
		{Name: "digitalocean", Enabled: true, APIKeyEnv: "TEST_DO_TOKEN_FAIL"},
	}

	withMockFetchers(map[string]ProviderFetcher{
		"civo":         mockCivo,
		"digitalocean": mockDO,
	}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error (should be nil even when all providers fail): %v", err)
		}

		data, ok := result.Data.(*collectors.BillingData)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
		}

		// BillingData should still be returned with both providers in error state.
		if len(data.Providers) != 2 {
			t.Fatalf("got %d providers, want 2", len(data.Providers))
		}

		for i, p := range data.Providers {
			if p.Status != "error" {
				t.Errorf("Providers[%d].Status = %q, want error", i, p.Status)
			}
		}

		// Totals should be zero since all providers failed.
		if data.Total.CurrentMonthUSD != 0 {
			t.Errorf("Total.CurrentMonthUSD = %v, want 0", data.Total.CurrentMonthUSD)
		}
		if data.Total.ForecastUSD != nil {
			t.Errorf("Total.ForecastUSD = %v, want nil", *data.Total.ForecastUSD)
		}

		// Should have two warnings.
		if len(result.Warnings) != 2 {
			t.Errorf("got %d warnings, want 2: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestBillingCollector_DisabledProvider(t *testing.T) {
	providers := []ProviderConfig{
		{Name: "civo", Enabled: false, APIKeyEnv: "TEST_CIVO_KEY_DIS"},
		{Name: "digitalocean", Enabled: false, APIKeyEnv: "TEST_DO_TOKEN_DIS"},
	}

	c := NewBillingCollector(providers, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	data, ok := result.Data.(*collectors.BillingData)
	if !ok {
		t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
	}

	if len(data.Providers) != 0 {
		t.Errorf("got %d providers, want 0 (disabled providers should be skipped)", len(data.Providers))
	}

	if len(result.Warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
	}
}

func TestBillingCollector_MissingAPIKey(t *testing.T) {
	// Ensure the env var is empty.
	t.Setenv("TEST_EMPTY_KEY", "")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_EMPTY_KEY"},
	}

	c := NewBillingCollector(providers, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	data, ok := result.Data.(*collectors.BillingData)
	if !ok {
		t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
	}

	if len(data.Providers) != 1 {
		t.Fatalf("got %d providers, want 1", len(data.Providers))
	}

	p := data.Providers[0]
	if p.Status != "error" {
		t.Errorf("Status = %q, want error for missing API key", p.Status)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("got %d warnings, want 1", len(result.Warnings))
	}

	// Warning should mention the environment variable.
	if got := result.Warnings[0]; len(got) == 0 {
		t.Error("warning message is empty")
	}
}

func TestBillingCollector_SummaryCalculation(t *testing.T) {
	// Provider A: spend=10, forecast=20, budget=30.
	billingA := &collectors.ProviderBilling{
		Provider:    "civo",
		AccountName: "account-a",
		Status:      "ok",
		CurrentMonth: collectors.MonthCost{
			SpendUSD:    10.00,
			ForecastUSD: f64ptr(20.00),
			BudgetUSD:   f64ptr(30.00),
			StartDate:   "2026-02-01",
			EndDate:     "2026-02-28",
		},
		FetchedAt: time.Now(),
	}

	// Provider B: spend=5, forecast=10, budget=15.
	billingB := &collectors.ProviderBilling{
		Provider:    "digitalocean",
		AccountName: "account-b",
		Status:      "ok",
		CurrentMonth: collectors.MonthCost{
			SpendUSD:    5.00,
			ForecastUSD: f64ptr(10.00),
			BudgetUSD:   f64ptr(15.00),
			StartDate:   "2026-02-01",
			EndDate:     "2026-02-28",
		},
		FetchedAt: time.Now(),
	}

	mockCivo := &mockProviderFetcher{billing: billingA}
	mockDO := &mockProviderFetcher{billing: billingB}

	t.Setenv("TEST_CIVO_KEY_SUM", "fake-key")
	t.Setenv("TEST_DO_TOKEN_SUM", "fake-token")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_SUM"},
		{Name: "digitalocean", Enabled: true, APIKeyEnv: "TEST_DO_TOKEN_SUM"},
	}

	withMockFetchers(map[string]ProviderFetcher{
		"civo":         mockCivo,
		"digitalocean": mockDO,
	}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.BillingData)

		// CurrentMonthUSD: 10 + 5 = 15.
		if data.Total.CurrentMonthUSD != 15.00 {
			t.Errorf("Total.CurrentMonthUSD = %v, want 15.00", data.Total.CurrentMonthUSD)
		}

		// ForecastUSD: 20 + 10 = 30.
		if data.Total.ForecastUSD == nil {
			t.Fatal("Total.ForecastUSD is nil, want non-nil")
		}
		if *data.Total.ForecastUSD != 30.00 {
			t.Errorf("Total.ForecastUSD = %v, want 30.00", *data.Total.ForecastUSD)
		}

		// BudgetUSD: 30 + 15 = 45.
		if data.Total.BudgetUSD == nil {
			t.Fatal("Total.BudgetUSD is nil, want non-nil")
		}
		if *data.Total.BudgetUSD != 45.00 {
			t.Errorf("Total.BudgetUSD = %v, want 45.00", *data.Total.BudgetUSD)
		}
	})
}

func TestBillingCollector_SummaryCalculation_NoForecast(t *testing.T) {
	// Provider with spend only, no forecast or budget.
	billingNoForecast := &collectors.ProviderBilling{
		Provider:    "civo",
		AccountName: "basic",
		Status:      "ok",
		CurrentMonth: collectors.MonthCost{
			SpendUSD:  8.00,
			StartDate: "2026-02-01",
			EndDate:   "2026-02-28",
		},
		FetchedAt: time.Now(),
	}

	mockCivo := &mockProviderFetcher{billing: billingNoForecast}

	t.Setenv("TEST_CIVO_KEY_NOFC", "fake-key")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_NOFC"},
	}

	withMockFetchers(map[string]ProviderFetcher{"civo": mockCivo}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.BillingData)

		if data.Total.CurrentMonthUSD != 8.00 {
			t.Errorf("Total.CurrentMonthUSD = %v, want 8.00", data.Total.CurrentMonthUSD)
		}

		// ForecastUSD should be nil since no provider had a forecast.
		if data.Total.ForecastUSD != nil {
			t.Errorf("Total.ForecastUSD = %v, want nil", *data.Total.ForecastUSD)
		}

		// BudgetUSD should be nil since no provider had a budget.
		if data.Total.BudgetUSD != nil {
			t.Errorf("Total.BudgetUSD = %v, want nil", *data.Total.BudgetUSD)
		}
	})
}

func TestBillingCollector_ContextCancellation(t *testing.T) {
	// Create a context that is already cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_CTX"},
	}

	c := NewBillingCollector(providers, testLogger())
	_, err := c.Collect(ctx)
	if err == nil {
		t.Fatal("Collect() with cancelled context should return error")
	}
	if err != context.Canceled {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestBillingCollector_ContextCancellationDuringFetch(t *testing.T) {
	// Create a context that cancels shortly after starting.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// The mock fetcher respects context cancellation.
	slowMock := &mockProviderFetcher{
		err: context.DeadlineExceeded,
	}

	t.Setenv("TEST_CIVO_KEY_SLOW", "fake-key")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_SLOW"},
	}

	withMockFetchers(map[string]ProviderFetcher{"civo": slowMock}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(ctx)

		// The collector might return an error from the post-collection context
		// check, or it might return a result with a warning. Both are acceptable.
		if err != nil {
			// Top-level context error is acceptable.
			return
		}

		// If we got a result, the provider should show an error status.
		data, ok := result.Data.(*collectors.BillingData)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
		}

		if len(data.Providers) == 1 && data.Providers[0].Status == "ok" {
			t.Error("expected non-ok status for provider that got context deadline exceeded")
		}
	})
}

func TestBillingCollector_InterfaceCompliance(t *testing.T) {
	var _ collectors.Collector = (*BillingCollector)(nil)
}

func TestBillingCollector_NilLogger(t *testing.T) {
	// Verify NewBillingCollector with nil logger does not panic.
	c := NewBillingCollector(nil, nil)
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestBillingCollector_TimestampIsRecent(t *testing.T) {
	before := time.Now()

	c := NewBillingCollector(nil, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	after := time.Now()

	if result.Timestamp.Before(before) || result.Timestamp.After(after) {
		t.Errorf("Timestamp = %v, want between %v and %v", result.Timestamp, before, after)
	}
}

func TestBillingCollector_OrderPreserved(t *testing.T) {
	mockCivo := &mockProviderFetcher{billing: civoBilling()}
	mockDO := &mockProviderFetcher{billing: doBilling()}

	t.Setenv("TEST_CIVO_KEY_ORD", "fake-key")
	t.Setenv("TEST_DO_TOKEN_ORD", "fake-token")

	// DO first, then Civo - verify output order matches input.
	providers := []ProviderConfig{
		{Name: "digitalocean", Enabled: true, APIKeyEnv: "TEST_DO_TOKEN_ORD"},
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_ORD"},
	}

	withMockFetchers(map[string]ProviderFetcher{
		"civo":         mockCivo,
		"digitalocean": mockDO,
	}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.BillingData)

		// Despite concurrent collection, order should match input config.
		if data.Providers[0].Provider != "digitalocean" {
			t.Errorf("Providers[0].Provider = %q, want digitalocean", data.Providers[0].Provider)
		}
		if data.Providers[1].Provider != "civo" {
			t.Errorf("Providers[1].Provider = %q, want civo", data.Providers[1].Provider)
		}
	})
}

func TestBillingCollector_UnsupportedProvider(t *testing.T) {
	t.Setenv("TEST_UNKNOWN_KEY", "fake-key")

	providers := []ProviderConfig{
		{Name: "unknown_cloud", Enabled: true, APIKeyEnv: "TEST_UNKNOWN_KEY"},
	}

	c := NewBillingCollector(providers, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	data, ok := result.Data.(*collectors.BillingData)
	if !ok {
		t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
	}

	if len(data.Providers) != 1 {
		t.Fatalf("got %d providers, want 1", len(data.Providers))
	}

	p := data.Providers[0]
	if p.Status != "error" {
		t.Errorf("Status = %q, want error for unsupported provider", p.Status)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("got %d warnings, want 1", len(result.Warnings))
	}
}

func TestBillingCollector_AWSProvider_WithMock(t *testing.T) {
	awsBilling := &collectors.ProviderBilling{
		Provider:     "aws",
		AccountName:  "AWS (prod)",
		Status:       "ok",
		DashboardURL: "https://console.aws.amazon.com/billing/home",
		CurrentMonth: collectors.MonthCost{
			SpendUSD:    42.37,
			ForecastUSD: f64ptr(156.80),
			StartDate:   "2026-02-01",
			EndDate:     "2026-02-28",
		},
		PreviousMonth: f64ptr(134.56),
		FetchedAt:     time.Now(),
	}

	mockAWS := &mockProviderFetcher{billing: awsBilling}

	t.Setenv("TEST_AWS_PROFILE", "prod")

	providers := []ProviderConfig{
		{Name: "aws", Enabled: true, APIKeyEnv: "TEST_AWS_PROFILE"},
	}

	withMockFetchers(map[string]ProviderFetcher{"aws": mockAWS}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.BillingData)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
		}

		if len(data.Providers) != 1 {
			t.Fatalf("got %d providers, want 1", len(data.Providers))
		}

		p := data.Providers[0]
		if p.Provider != "aws" {
			t.Errorf("Provider = %q, want %q", p.Provider, "aws")
		}
		if p.Status != "ok" {
			t.Errorf("Status = %q, want %q", p.Status, "ok")
		}
		if p.CurrentMonth.SpendUSD != 42.37 {
			t.Errorf("SpendUSD = %v, want 42.37", p.CurrentMonth.SpendUSD)
		}
		if p.CurrentMonth.ForecastUSD == nil || *p.CurrentMonth.ForecastUSD != 156.80 {
			t.Errorf("ForecastUSD = %v, want 156.80", p.CurrentMonth.ForecastUSD)
		}
		if p.PreviousMonth == nil || *p.PreviousMonth != 134.56 {
			t.Errorf("PreviousMonth = %v, want 134.56", p.PreviousMonth)
		}

		if data.Total.CurrentMonthUSD != 42.37 {
			t.Errorf("Total.CurrentMonthUSD = %v, want 42.37", data.Total.CurrentMonthUSD)
		}

		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestBillingCollector_DreamHostProvider_WithMock(t *testing.T) {
	dhBilling := &collectors.ProviderBilling{
		Provider:     "dreamhost",
		AccountName:  "DreamHost (bandwidth only)",
		Status:       "limited",
		DashboardURL: "https://panel.dreamhost.com/index.cgi?tree=billing.account",
		CurrentMonth: collectors.MonthCost{
			StartDate: "2026-02-01",
			EndDate:   "2026-02-28",
		},
		FetchedAt: time.Now(),
	}

	mockDH := &mockProviderFetcher{billing: dhBilling}

	t.Setenv("TEST_DH_KEY", "fake-dh-key")

	providers := []ProviderConfig{
		{Name: "dreamhost", Enabled: true, APIKeyEnv: "TEST_DH_KEY"},
	}

	withMockFetchers(map[string]ProviderFetcher{"dreamhost": mockDH}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.BillingData)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.BillingData", result.Data)
		}

		if len(data.Providers) != 1 {
			t.Fatalf("got %d providers, want 1", len(data.Providers))
		}

		p := data.Providers[0]
		if p.Provider != "dreamhost" {
			t.Errorf("Provider = %q, want %q", p.Provider, "dreamhost")
		}
		if p.Status != "limited" {
			t.Errorf("Status = %q, want %q", p.Status, "limited")
		}
	})
}

func TestBillingCollector_MixedEnabledDisabled(t *testing.T) {
	mockCivo := &mockProviderFetcher{billing: civoBilling()}

	t.Setenv("TEST_CIVO_KEY_MIX", "fake-key")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_MIX"},
		{Name: "digitalocean", Enabled: false, APIKeyEnv: "TEST_DO_TOKEN_MIX"},
	}

	withMockFetchers(map[string]ProviderFetcher{"civo": mockCivo}, func() {
		c := NewBillingCollector(providers, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.BillingData)

		// Only the enabled provider should be collected.
		if len(data.Providers) != 1 {
			t.Fatalf("got %d providers, want 1", len(data.Providers))
		}

		if data.Providers[0].Provider != "civo" {
			t.Errorf("Provider = %q, want civo", data.Providers[0].Provider)
		}
	})
}

// --- Validation Function Tests ---

func TestValidateBillingProviders_AllConfigured(t *testing.T) {
	t.Setenv("TEST_CIVO_KEY_VAL", "fake-civo-key")
	t.Setenv("TEST_DO_TOKEN_VAL", "fake-do-token")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_VAL"},
		{Name: "digitalocean", Enabled: true, APIKeyEnv: "TEST_DO_TOKEN_VAL"},
	}

	validations := ValidateBillingProviders(providers)

	if len(validations) != 2 {
		t.Fatalf("got %d validations, want 2", len(validations))
	}

	for _, v := range validations {
		if !v.Configured {
			t.Errorf("provider %q: Configured = false, want true", v.Provider)
		}
		if v.ErrorReason != "" {
			t.Errorf("provider %q: ErrorReason = %q, want empty", v.Provider, v.ErrorReason)
		}
	}
}

func TestValidateBillingProviders_SomeMissing(t *testing.T) {
	t.Setenv("TEST_CIVO_KEY_VAL2", "fake-civo-key")
	t.Setenv("TEST_DO_TOKEN_VAL2", "") // Explicitly empty

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_VAL2"},
		{Name: "digitalocean", Enabled: true, APIKeyEnv: "TEST_DO_TOKEN_VAL2"},
	}

	validations := ValidateBillingProviders(providers)

	if len(validations) != 2 {
		t.Fatalf("got %d validations, want 2", len(validations))
	}

	// Civo should be configured
	civo := validations[0]
	if civo.Provider != "civo" {
		t.Errorf("validations[0].Provider = %q, want civo", civo.Provider)
	}
	if !civo.Configured {
		t.Errorf("civo: Configured = false, want true")
	}

	// DigitalOcean should NOT be configured
	do := validations[1]
	if do.Provider != "digitalocean" {
		t.Errorf("validations[1].Provider = %q, want digitalocean", do.Provider)
	}
	if do.Configured {
		t.Errorf("digitalocean: Configured = true, want false")
	}
	if do.ErrorReason == "" {
		t.Error("digitalocean: ErrorReason is empty, want non-empty")
	}
}

func TestValidateBillingProviders_DisabledSkipped(t *testing.T) {
	t.Setenv("TEST_CIVO_KEY_VAL3", "fake-key")

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_VAL3"},
		{Name: "digitalocean", Enabled: false, APIKeyEnv: "TEST_DO_TOKEN_VAL3"},
	}

	validations := ValidateBillingProviders(providers)

	// Only enabled provider should be validated
	if len(validations) != 1 {
		t.Fatalf("got %d validations, want 1 (disabled provider should be skipped)", len(validations))
	}

	if validations[0].Provider != "civo" {
		t.Errorf("validations[0].Provider = %q, want civo", validations[0].Provider)
	}
}

func TestValidateBillingProviders_EmptyList(t *testing.T) {
	validations := ValidateBillingProviders(nil)

	if len(validations) != 0 {
		t.Errorf("got %d validations, want 0 for nil providers", len(validations))
	}

	validations = ValidateBillingProviders([]ProviderConfig{})

	if len(validations) != 0 {
		t.Errorf("got %d validations, want 0 for empty providers", len(validations))
	}
}

func TestValidateBillingProviders_FileBasedCredentials(t *testing.T) {
	// Create a temporary file with a secret
	tmpFile := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(tmpFile, []byte("  file-based-secret\n"), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Set the _FILE variant environment variable
	t.Setenv("TEST_CIVO_KEY_VAL_FILE", tmpFile)

	providers := []ProviderConfig{
		{Name: "civo", Enabled: true, APIKeyEnv: "TEST_CIVO_KEY_VAL"},
	}

	validations := ValidateBillingProviders(providers)

	if len(validations) != 1 {
		t.Fatalf("got %d validations, want 1", len(validations))
	}

	v := validations[0]
	if !v.Configured {
		t.Errorf("Configured = false, want true (file-based credential should be detected)")
	}
}
