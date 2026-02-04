package collectors

import (
	"testing"
	"time"
)

// ========== ClaudeUsage.StarshipOutput Tests ==========

func TestClaudeUsage_StarshipOutput_SingleSubscription(t *testing.T) {
	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "personal",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &UsagePeriod{
					Utilization: 45,
					ResetsAt:    time.Date(2025, 1, 15, 5, 0, 0, 0, time.UTC),
				},
			},
		},
	}

	got := usage.StarshipOutput()
	want := "personal:pro 45%"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
	}
}

func TestClaudeUsage_StarshipOutput_MultipleAccounts(t *testing.T) {
	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "personal",
				Type:   "subscription",
				Tier:   "max_5x",
				Status: "ok",
				FiveHour: &UsagePeriod{
					Utilization: 80,
					ResetsAt:    time.Date(2025, 1, 15, 5, 0, 0, 0, time.UTC),
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
					RequestsReset:     time.Date(2025, 1, 15, 0, 1, 0, 0, time.UTC),
					TokensLimit:       100000,
					TokensRemaining:   80000,
					TokensReset:       time.Date(2025, 1, 15, 0, 1, 0, 0, time.UTC),
				},
			},
		},
	}

	got := usage.StarshipOutput()
	want := "personal:max_5x 80% | work:tier_2 25%"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
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
	usage := &ClaudeUsage{
		Accounts: []ClaudeAccountUsage{
			{
				Name:   "good",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &UsagePeriod{
					Utilization: 10,
					ResetsAt:    time.Date(2025, 1, 15, 5, 0, 0, 0, time.UTC),
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
	want := "good:pro 10% | bad:ERR"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
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
					RequestsReset:     time.Date(2025, 1, 15, 0, 1, 0, 0, time.UTC),
					TokensLimit:       200000,
					TokensRemaining:   50000,
					TokensReset:       time.Date(2025, 1, 15, 0, 1, 0, 0, time.UTC),
				},
			},
		},
	}

	got := usage.StarshipOutput()
	// (2000 - 500) / 2000 * 100 = 75%
	want := "api-acct:tier_4 75%"
	if got != want {
		t.Errorf("StarshipOutput() = %q, want %q", got, want)
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
