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
	want := "broken:ERR"
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
	// First account should show usage with reset time; second should show ERR.
	if !containsSubstrings(got, []string{"good:pro 10%", "bad:ERR"}) {
		t.Errorf("StarshipOutput() = %q, expected good:pro 10%% and bad:ERR", got)
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
		},
	}

	got := billing.StarshipOutput()
	want := "$75 ($150 forecast)"
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
		},
	}

	got := billing.StarshipOutput()
	want := "$120 ($200 forecast) OVER BUDGET"
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
		},
	}

	got := billing.StarshipOutput()
	// Under budget: no OVER BUDGET suffix.
	want := "$50 ($80 forecast)"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestBillingData_StarshipOutput_NoForecast(t *testing.T) {
	billing := &BillingData{
		Total: BillingSummary{
			CurrentMonthUSD: 42,
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
		},
	}

	got := billing.StarshipOutput()
	// No budget set, so no OVER BUDGET even if spend is high.
	want := "$80 ($100 forecast)"
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
		},
	}

	got := billing.StarshipOutput()
	want := "$0"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestBillingData_StarshipOutput_ExactBudget(t *testing.T) {
	// Spend equals budget exactly -- should NOT show OVER BUDGET.
	budget := 100.0
	billing := &BillingData{
		Total: BillingSummary{
			CurrentMonthUSD: 100,
			BudgetUSD:       &budget,
		},
	}

	got := billing.StarshipOutput()
	want := "$100"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
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
			{Name: "civo", Status: "healthy"},
		},
	}

	got := infra.StarshipOutput()
	want := "ts:5/8 k8s:civo:healthy"
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
			{Name: "prod", Status: "healthy"},
		},
	}

	got := infra.StarshipOutput()
	want := "k8s:prod:healthy"
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
			{Name: "civo", Status: "healthy"},
			{Name: "rke2", Status: "degraded"},
			{Name: "kind", Status: "offline"},
		},
	}

	got := infra.StarshipOutput()
	want := "ts:10/12 k8s:civo:healthy k8s:rke2:degraded k8s:kind:offline"
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
