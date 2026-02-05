package collectors

import (
	"testing"
	"time"
)

// ========== ClaudeUsage.StarshipOutput Tests ==========

func TestClaudeUsage_StarshipOutput_SingleSubscription(t *testing.T) {
	// Use a future reset time to test the relative time formatting.
	futureReset := time.Now().Add(2*time.Hour + 15*time.Minute)

	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "personal",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &UsagePeriod{
					Utilization: 45,
					ResetsAt:    futureReset,
				},
			},
		},
	}

	got := usage.StarshipOutput()
	// Should include a reset time like "(2h 15m)" and no warning since < 80%.
	if !containsSubstrings(got, []string{"personal:pro 45%", "(2h"}) {
		t.Errorf("StarshipOutput() = %q, expected to contain 'personal:pro 45%%' and reset time", got)
	}
	if containsSubstrings(got, []string{"⚠️"}) {
		t.Errorf("StarshipOutput() = %q, should not contain warning emoji for 45%%", got)
	}
}

// containsSubstrings returns true if s contains all the given substrings.
func containsSubstrings(s string, subs []string) bool {
	for _, sub := range subs {
		if !containsString(s, sub) {
			return false
		}
	}
	return true
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStringHelper(s, sub))
}

func containsStringHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestClaudeUsage_StarshipOutput_MultipleAccounts(t *testing.T) {
	futureReset := time.Now().Add(1 * time.Hour)

	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "personal",
				Type:   "subscription",
				Tier:   "max_5x",
				Status: "ok",
				FiveHour: &UsagePeriod{
					Utilization: 80,
					ResetsAt:    futureReset,
				},
			},
			{
				Name:   "work",
				Type:   "api",
				Tier:   "tier_2",
				Status: "ok",
				RateLimits: &APIRateLimits{
					RequestsLimit:     1000,
					RequestsRemaining: 750,
					RequestsReset:     futureReset,
					TokensLimit:       100000,
					TokensRemaining:   80000,
					TokensReset:       futureReset,
				},
			},
		},
	}

	got := usage.StarshipOutput()
	// 80% should trigger warning emoji; 25% should not.
	if !containsSubstrings(got, []string{"personal:max_5x 80%", "⚠️", "work:tier_2 25%"}) {
		t.Errorf("StarshipOutput() = %q, expected both accounts with warning for 80%%", got)
	}
}

func TestClaudeUsage_StarshipOutput_ErrorStatus(t *testing.T) {
	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "broken",
				Type:   "subscription",
				Tier:   "pro",
				Status: "auth_failed",
			},
		},
	}

	got := usage.StarshipOutput()
	want := "broken:AUTH" // auth_failed now maps to AUTH, not ERR
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestClaudeUsage_StarshipOutput_MixedOkAndError(t *testing.T) {
	futureReset := time.Now().Add(3 * time.Hour)

	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "good",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &UsagePeriod{
					Utilization: 10,
					ResetsAt:    futureReset,
				},
			},
			{
				Name:   "bad",
				Type:   "api",
				Tier:   "tier_1",
				Status: "rate_limited",
			},
		},
	}

	got := usage.StarshipOutput()
	// First account should show usage with reset time; second should show RATE (rate_limited).
	if !containsSubstrings(got, []string{"good:pro 10%", "bad:RATE"}) {
		t.Errorf("StarshipOutput() = %q, expected good:pro 10%% and bad:RATE", got)
	}
}

func TestClaudeUsage_StarshipOutput_Empty(t *testing.T) {
	usage := &ClaudeUsage{Accounts: []ClaudeAccountUsage{}}
	got := usage.StarshipOutput()
	if got != "" {
		t.Errorf("StarshipOutput() with empty accounts = %q, want empty string", got)
	}
}

func TestClaudeUsage_StarshipOutput_Nil(t *testing.T) {
	var usage *ClaudeUsage
	got := usage.StarshipOutput()
	if got != "" {
		t.Errorf("StarshipOutput() on nil = %q, want empty string", got)
	}
}

func TestClaudeUsage_StarshipOutput_APIUtilization(t *testing.T) {
	// Verify rate limit utilization calculation: (limit - remaining) / limit * 100.
	futureReset := time.Now().Add(30 * time.Minute)

	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "api-acct",
				Type:   "api",
				Tier:   "tier_4",
				Status: "ok",
				RateLimits: &APIRateLimits{
					RequestsLimit:     2000,
					RequestsRemaining: 500,
					RequestsReset:     futureReset,
					TokensLimit:       200000,
					TokensRemaining:   50000,
					TokensReset:       futureReset,
				},
			},
		},
	}

	got := usage.StarshipOutput()
	// (2000 - 500) / 2000 * 100 = 75%, should not trigger warning (< 80%).
	if !containsSubstrings(got, []string{"api-acct:tier_4 75%"}) {
		t.Errorf("StarshipOutput() = %q, expected 75%% utilization", got)
	}
	if containsSubstrings(got, []string{"⚠️"}) {
		t.Errorf("StarshipOutput() = %q, should not contain warning for 75%%", got)
	}
}

func TestClaudeUsage_StarshipOutput_SubscriptionNoFiveHour(t *testing.T) {
	// Subscription with nil FiveHour should show 0% utilization.
	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "new-sub",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
			},
		},
	}

	got := usage.StarshipOutput()
	want := "new-sub:pro 0%"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestClaudeUsage_StarshipOutput_APINoRateLimits(t *testing.T) {
	// API account with nil RateLimits should show 0% utilization.
	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "new-api",
				Type:   "api",
				Tier:   "tier_1",
				Status: "ok",
			},
		},
	}

	got := usage.StarshipOutput()
	want := "new-api:tier_1 0%"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestClaudeUsage_StarshipOutput_APIZeroLimit(t *testing.T) {
	// API account with zero RequestsLimit should show 0% (avoid division by zero).
	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "zero-limit",
				Type:   "api",
				Tier:   "tier_1",
				Status: "ok",
				RateLimits: &APIRateLimits{
					RequestsLimit:     0,
					RequestsRemaining: 0,
				},
			},
		},
	}

	got := usage.StarshipOutput()
	want := "zero-limit:tier_1 0%"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

// ========== BillingData.StarshipOutput Tests ==========

func TestBillingData_StarshipOutput_NormalSpend(t *testing.T) {
	forecast := 150.0
	billing := &BillingData{
		Total: BillingSummary{
			CurrentMonthUSD: 75,
			ForecastUSD:     &forecast,
			SuccessCount:    1,
			TotalConfigured: 1,
		},
	}

	got := billing.StarshipOutput()
	want := "$75 ~$150"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestBillingData_StarshipOutput_OverBudget(t *testing.T) {
	forecast := 200.0
	budget := 100.0
	billing := &BillingData{
		Total: BillingSummary{
			CurrentMonthUSD: 120,
			ForecastUSD:     &forecast,
			BudgetUSD:       &budget,
			SuccessCount:    1, // At least one provider succeeded
			TotalConfigured: 1,
		},
	}

	got := billing.StarshipOutput()
	want := "$120 120%🔴 ~$200"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestBillingData_StarshipOutput_UnderBudget(t *testing.T) {
	forecast := 80.0
	budget := 200.0
	billing := &BillingData{
		Total: BillingSummary{
			CurrentMonthUSD: 50,
			ForecastUSD:     &forecast,
			BudgetUSD:       &budget,
			SuccessCount:    1,
			TotalConfigured: 1,
		},
	}

	got := billing.StarshipOutput()
	// Under budget: shows percentage but no warning emoji.
	want := "$50 25% ~$80"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestBillingData_StarshipOutput_NoForecast(t *testing.T) {
	billing := &BillingData{
		Total: BillingSummary{
			CurrentMonthUSD: 42,
			SuccessCount:    1,
			TotalConfigured: 1,
		},
	}

	got := billing.StarshipOutput()
	want := "$42"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestBillingData_StarshipOutput_NoBudget(t *testing.T) {
	forecast := 100.0
	billing := &BillingData{
		Total: BillingSummary{
			CurrentMonthUSD: 80,
			ForecastUSD:     &forecast,
			SuccessCount:    1,
			TotalConfigured: 1,
		},
	}

	got := billing.StarshipOutput()
	// No budget set, so no budget percentage shown.
	want := "$80 ~$100"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestBillingData_StarshipOutput_Nil(t *testing.T) {
	var billing *BillingData
	got := billing.StarshipOutput()
	if got != "" {
		t.Errorf("StarshipOutput() on nil = %q, want empty string", got)
	}
}

func TestBillingData_StarshipOutput_ZeroSpend(t *testing.T) {
	billing := &BillingData{
		Total: BillingSummary{
			CurrentMonthUSD: 0,
			SuccessCount:    1,
			TotalConfigured: 1,
		},
	}

	got := billing.StarshipOutput()
	want := "$0"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestBillingData_StarshipOutput_ExactBudget(t *testing.T) {
	// Spend equals budget exactly -- shows 100% with critical emoji.
	budget := 100.0
	billing := &BillingData{
		Total: BillingSummary{
			CurrentMonthUSD: 100,
			BudgetUSD:       &budget,
			SuccessCount:    1,
			TotalConfigured: 1,
		},
	}

	got := billing.StarshipOutput()
	want := "$100 100%🔴"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestBillingData_StarshipOutput_WithSparkline(t *testing.T) {
	forecast := 150.0
	billing := &BillingData{
		Total: BillingSummary{
			CurrentMonthUSD: 75,
			ForecastUSD:     &forecast,
			SuccessCount:    1,
			TotalConfigured: 1,
		},
		History: &BillingHistory{
			TotalHistory: []DailySpend{
				{Date: "2026-01-01", SpendUSD: 10},
				{Date: "2026-01-02", SpendUSD: 20},
				{Date: "2026-01-03", SpendUSD: 30},
				{Date: "2026-01-04", SpendUSD: 40},
				{Date: "2026-01-05", SpendUSD: 50},
			},
		},
	}

	got := billing.StarshipOutput()
	// Should include sparkline (specific characters depend on normalization)
	if !containsString(got, "$75") {
		t.Errorf("StarshipOutput() = %q, should contain $75", got)
	}
	if !containsString(got, "~$150") {
		t.Errorf("StarshipOutput() = %q, should contain ~$150", got)
	}
	// Verify sparkline is present (should be 5 Unicode block characters)
	if len(got) < 20 {
		t.Errorf("StarshipOutput() = %q, expected sparkline to be included", got)
	}
}

func TestRenderSparkline_IncreasingTrend(t *testing.T) {
	values := []float64{10, 20, 30, 40, 50}
	sparkline := renderSparkline(values)

	// Should render as increasing bars: ▁▂▃▅█ or similar
	runes := []rune(sparkline)
	if len(runes) != 5 {
		t.Errorf("renderSparkline() produced %d runes, want 5", len(runes))
	}

	// First should be lower than last (comparing rune values)
	if runes[0] > runes[4] {
		t.Errorf("renderSparkline() = %q, expected increasing trend", sparkline)
	}
}

func TestRenderSparkline_FlatLine(t *testing.T) {
	values := []float64{50, 50, 50, 50, 50}
	sparkline := renderSparkline(values)

	// Flat line should render as middle height bars: ▄▄▄▄▄
	expected := "▄▄▄▄▄"
	if sparkline != expected {
		t.Errorf("renderSparkline() = %q, want %q for flat line", sparkline, expected)
	}
}

func TestRenderSparkline_Empty(t *testing.T) {
	values := []float64{}
	sparkline := renderSparkline(values)

	if sparkline != "" {
		t.Errorf("renderSparkline() = %q, want empty string for empty input", sparkline)
	}
}

func TestCalculateBudgetAlert_UnderThreshold(t *testing.T) {
	alert := calculateBudgetAlert(50, 100, 70)
	if alert != "" {
		t.Errorf("calculateBudgetAlert(50, 100, 70) = %q, want empty (no alert)", alert)
	}
}

func TestCalculateBudgetAlert_AtThreshold(t *testing.T) {
	alert := calculateBudgetAlert(70, 100, 70)
	if alert != "⚠️" {
		t.Errorf("calculateBudgetAlert(70, 100, 70) = %q, want ⚠️", alert)
	}
}

func TestCalculateBudgetAlert_OverBudget(t *testing.T) {
	alert := calculateBudgetAlert(120, 100, 70)
	if alert != "🔴" {
		t.Errorf("calculateBudgetAlert(120, 100, 70) = %q, want 🔴", alert)
	}
}

func TestCalculateBudgetAlert_ExactlyAtBudget(t *testing.T) {
	alert := calculateBudgetAlert(100, 100, 70)
	if alert != "🔴" {
		t.Errorf("calculateBudgetAlert(100, 100, 70) = %q, want 🔴", alert)
	}
}

// ========== InfraStatus.StarshipOutput Tests ==========

func TestInfraStatus_StarshipOutput_TailscaleAndK8s(t *testing.T) {
	infra := &InfraStatus{
		Tailscale: &TailscaleStatus{
			OnlineCount: 5,
			TotalCount:  8,
		},
		Kubernetes: []KubernetesCluster{
			{
				Name:        "civo",
				Status:      "healthy",
				TotalNodes:  3,
				ReadyNodes:  3,
				TotalPods:   45,
				RunningPods: 42,
			},
		},
	}

	got := infra.StarshipOutput()
	want := "ts:5/8 k8s:civo(3/3 nodes, 42/45 pods)"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestInfraStatus_StarshipOutput_OnlyTailscale(t *testing.T) {
	infra := &InfraStatus{
		Tailscale: &TailscaleStatus{
			OnlineCount: 3,
			TotalCount:  3,
		},
	}

	got := infra.StarshipOutput()
	want := "ts:3/3"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestInfraStatus_StarshipOutput_OnlyK8s(t *testing.T) {
	infra := &InfraStatus{
		Kubernetes: []KubernetesCluster{
			{
				Name:        "prod",
				Status:      "healthy",
				TotalNodes:  5,
				ReadyNodes:  5,
				TotalPods:   120,
				RunningPods: 115,
			},
		},
	}

	got := infra.StarshipOutput()
	want := "k8s:prod(5/5 nodes, 115/120 pods)"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestInfraStatus_StarshipOutput_MultipleK8s(t *testing.T) {
	infra := &InfraStatus{
		Tailscale: &TailscaleStatus{
			OnlineCount: 10,
			TotalCount:  12,
		},
		Kubernetes: []KubernetesCluster{
			{
				Name:        "civo",
				Status:      "healthy",
				TotalNodes:  3,
				ReadyNodes:  3,
				TotalPods:   48,
				RunningPods: 45,
			},
			{
				Name:        "rke2",
				Status:      "degraded",
				TotalNodes:  5,
				ReadyNodes:  4,
				TotalPods:   80,
				RunningPods: 75,
			},
			{
				Name:   "kind",
				Status: "offline",
			},
		},
	}

	got := infra.StarshipOutput()
	want := "ts:10/12 k8s:civo(3/3 nodes, 45/48 pods) k8s:rke2(4/5 nodes, 75/80 pods) k8s:kind(offline)"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestInfraStatus_StarshipOutput_Nil(t *testing.T) {
	var infra *InfraStatus
	got := infra.StarshipOutput()
	if got != "" {
		t.Errorf("StarshipOutput() on nil = %q, want empty string", got)
	}
}

func TestInfraStatus_StarshipOutput_Empty(t *testing.T) {
	infra := &InfraStatus{}

	got := infra.StarshipOutput()
	if got != "" {
		t.Errorf("StarshipOutput() on empty struct = %q, want empty string", got)
	}
}

func TestInfraStatus_StarshipOutput_AllNodesOnline(t *testing.T) {
	infra := &InfraStatus{
		Tailscale: &TailscaleStatus{
			OnlineCount: 0,
			TotalCount:  0,
		},
	}

	got := infra.StarshipOutput()
	want := "ts:0/0"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}
