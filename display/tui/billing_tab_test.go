package tui

import (
	"strings"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

func TestRenderBillingContent_Nil(t *testing.T) {
	result := renderBillingContent(nil, 80, 24)
	if result != "No billing data available" {
		t.Errorf("expected placeholder for nil data, got: %s", result)
	}
}

func TestRenderBillingContent_SingleProvider(t *testing.T) {
	data := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider:    "civo",
				AccountName: "tinyland",
				Status:      "ok",
				CurrentMonth: collectors.MonthCost{
					SpendUSD:  42.50,
					StartDate: "2026-02-01",
					EndDate:   "2026-02-28",
				},
				FetchedAt: time.Date(2026, 2, 3, 14, 30, 0, 0, time.UTC),
			},
		},
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 42.50,
		},
	}

	result := renderBillingContent(data, 100, 24)

	// Should contain title.
	if !strings.Contains(result, "Cloud Billing") {
		t.Error("expected 'Cloud Billing' title")
	}

	// Should contain provider name.
	if !strings.Contains(result, "civo") {
		t.Error("expected provider 'civo' in output")
	}

	// Should contain spend amount.
	if !strings.Contains(result, "$42.50") {
		t.Error("expected '$42.50' spend in output")
	}

	// Should contain total.
	if !strings.Contains(result, "Current month") {
		t.Error("expected 'Current month' label")
	}
}

func TestRenderBillingContent_MultipleProviders(t *testing.T) {
	forecast := 150.0
	data := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider:    "civo",
				AccountName: "tinyland",
				Status:      "ok",
				CurrentMonth: collectors.MonthCost{
					SpendUSD:  42.50,
					StartDate: "2026-02-01",
					EndDate:   "2026-02-28",
				},
				FetchedAt: time.Date(2026, 2, 3, 14, 30, 0, 0, time.UTC),
			},
			{
				Provider:    "digitalocean",
				AccountName: "tinyland-do",
				Status:      "ok",
				CurrentMonth: collectors.MonthCost{
					SpendUSD:    85.00,
					ForecastUSD: &forecast,
					StartDate:   "2026-02-01",
					EndDate:     "2026-02-28",
				},
				FetchedAt: time.Date(2026, 2, 3, 14, 30, 0, 0, time.UTC),
			},
		},
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 127.50,
		},
	}

	result := renderBillingContent(data, 120, 24)

	// Should contain both providers.
	if !strings.Contains(result, "civo") {
		t.Error("expected 'civo' in output")
	}
	if !strings.Contains(result, "digitalocean") {
		t.Error("expected 'digitalocean' in output")
	}

	// Should contain total.
	if !strings.Contains(result, "$127.50") {
		t.Error("expected total '$127.50' in output")
	}
}

func TestRenderBillingContent_WithForecast(t *testing.T) {
	forecast := 200.0
	totalForecast := 200.0
	data := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider: "aws",
				Status:   "ok",
				CurrentMonth: collectors.MonthCost{
					SpendUSD:    95.00,
					ForecastUSD: &forecast,
					StartDate:   "2026-02-01",
					EndDate:     "2026-02-28",
				},
				FetchedAt: time.Now(),
			},
		},
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 95.00,
			ForecastUSD:     &totalForecast,
		},
	}

	result := renderBillingContent(data, 100, 24)

	// Should show forecast in summary.
	if !strings.Contains(result, "Forecast") {
		t.Error("expected 'Forecast' label in summary")
	}
	if !strings.Contains(result, "$200.00") {
		t.Error("expected forecast amount '$200.00'")
	}
}

func TestRenderBillingContent_OverBudget(t *testing.T) {
	budget := 100.0
	data := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider: "aws",
				Status:   "ok",
				CurrentMonth: collectors.MonthCost{
					SpendUSD:  125.00,
					BudgetUSD: &budget,
					StartDate: "2026-02-01",
					EndDate:   "2026-02-28",
				},
				FetchedAt: time.Now(),
			},
		},
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 125.00,
			BudgetUSD:       &budget,
		},
	}

	result := renderBillingContent(data, 100, 24)

	// Should indicate over budget.
	if !strings.Contains(result, "OVER BUDGET") {
		t.Error("expected 'OVER BUDGET' indicator")
	}

	// Should show the overage amount.
	if !strings.Contains(result, "$25.00") {
		t.Error("expected overage amount '$25.00'")
	}
}

func TestRenderBillingContent_ErrorProvider(t *testing.T) {
	data := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider: "dreamhost",
				Status:   "error",
				CurrentMonth: collectors.MonthCost{
					SpendUSD:  0,
					StartDate: "2026-02-01",
					EndDate:   "2026-02-28",
				},
				FetchedAt: time.Now(),
			},
		},
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 0,
		},
	}

	result := renderBillingContent(data, 100, 24)

	// Should show the provider.
	if !strings.Contains(result, "dreamhost") {
		t.Error("expected 'dreamhost' in output")
	}

	// Should contain error status text.
	if !strings.Contains(result, "error") {
		t.Error("expected 'error' status text")
	}
}

func TestRenderBillingContent_WithDashboardURL(t *testing.T) {
	data := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider:     "civo",
				Status:       "ok",
				DashboardURL: "https://dashboard.civo.com/billing",
				CurrentMonth: collectors.MonthCost{
					SpendUSD:  42.50,
					StartDate: "2026-02-01",
					EndDate:   "2026-02-28",
				},
				FetchedAt: time.Now(),
			},
		},
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 42.50,
		},
	}

	result := renderBillingContent(data, 100, 24)

	// OSC 8 links use escape sequences: \033]8;;URL\033\TEXT\033]8;;\033\.
	if !strings.Contains(result, "\033]8;;") {
		t.Error("expected OSC 8 escape sequence for dashboard link")
	}

	if !strings.Contains(result, "https://dashboard.civo.com/billing") {
		t.Error("expected dashboard URL in OSC 8 link")
	}

	if !strings.Contains(result, "civo dashboard") {
		t.Error("expected 'civo dashboard' link text")
	}
}

func TestRenderBillingContent_NoPreviousMonth(t *testing.T) {
	data := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider:      "aws",
				Status:        "ok",
				PreviousMonth: nil,
				CurrentMonth: collectors.MonthCost{
					SpendUSD:  50.00,
					StartDate: "2026-02-01",
					EndDate:   "2026-02-28",
				},
				FetchedAt: time.Now(),
			},
		},
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 50.00,
		},
	}

	result := renderBillingContent(data, 100, 24)

	// Should render without errors.
	if !strings.Contains(result, "aws") {
		t.Error("expected 'aws' in output")
	}

	// The Previous column should show "-" for nil previous month.
	// The table renders the row with the "-" placeholder we set.
	if !strings.Contains(result, "-") {
		t.Error("expected '-' placeholder for nil previous month")
	}
}

func TestRenderBillingContent_UnderBudget(t *testing.T) {
	budget := 200.0
	data := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider: "aws",
				Status:   "ok",
				CurrentMonth: collectors.MonthCost{
					SpendUSD:  50.00,
					StartDate: "2026-02-01",
					EndDate:   "2026-02-28",
				},
				FetchedAt: time.Now(),
			},
		},
		Total: collectors.BillingSummary{
			CurrentMonthUSD: 50.00,
			BudgetUSD:       &budget,
		},
	}

	result := renderBillingContent(data, 100, 24)

	// Should show under budget message.
	if !strings.Contains(result, "Under budget") {
		t.Error("expected 'Under budget' indicator")
	}
	if !strings.Contains(result, "$150.00 remaining") {
		t.Error("expected remaining amount '$150.00 remaining'")
	}
}
