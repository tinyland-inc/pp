package status

import (
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// --- Helpers ---

func ptrFloat64(v float64) *float64 { return &v }

func makeAccount(name, status string, fiveHourUtil float64) collectors.ClaudeAccountUsage {
	return collectors.ClaudeAccountUsage{
		Name:   name,
		Type:   "subscription",
		Tier:   "pro",
		Status: status,
		FiveHour: &collectors.UsagePeriod{
			Utilization: fiveHourUtil,
			ResetsAt:    time.Now().Add(5 * time.Hour),
		},
	}
}

func makeClaudeUsage(accounts ...collectors.ClaudeAccountUsage) *collectors.ClaudeUsage {
	return &collectors.ClaudeUsage{Accounts: accounts}
}

func makeBillingData(spend float64, budget, forecast *float64, providers ...collectors.ProviderBilling) *collectors.BillingData {
	return &collectors.BillingData{
		Providers: providers,
		Total: collectors.BillingSummary{
			CurrentMonthUSD: spend,
			BudgetUSD:       budget,
			ForecastUSD:     forecast,
		},
	}
}

func makeProvider(name, status string) collectors.ProviderBilling {
	return collectors.ProviderBilling{
		Provider:    name,
		AccountName: name + "-account",
		Status:      status,
		CurrentMonth: collectors.MonthCost{
			SpendUSD:  10.0,
			StartDate: "2026-02-01",
			EndDate:   "2026-02-28",
		},
		FetchedAt: time.Now(),
	}
}

func makeInfraStatus(tsOnline, tsTotal int, clusters ...collectors.KubernetesCluster) *collectors.InfraStatus {
	infra := &collectors.InfraStatus{
		Kubernetes: clusters,
	}
	if tsTotal >= 0 {
		infra.Tailscale = &collectors.TailscaleStatus{
			Tailnet:     "test.ts.net",
			OnlineCount: tsOnline,
			TotalCount:  tsTotal,
		}
	}
	return infra
}

func makeCluster(name, status string, ready, total int) collectors.KubernetesCluster {
	return collectors.KubernetesCluster{
		Name:       name,
		Platform:   "test",
		Status:     status,
		TotalNodes: total,
		ReadyNodes: ready,
	}
}

// --- Tests ---

func TestDefaultEvaluatorConfig(t *testing.T) {
	cfg := DefaultEvaluatorConfig()

	checks := []struct {
		name  string
		value float64
	}{
		{"ClaudeWarningPercent", cfg.ClaudeWarningPercent},
		{"ClaudeCriticalPercent", cfg.ClaudeCriticalPercent},
		{"BillingBudgetWarningPercent", cfg.BillingBudgetWarningPercent},
		{"BillingBudgetCriticalPercent", cfg.BillingBudgetCriticalPercent},
		{"TailscaleWarningPercent", cfg.TailscaleWarningPercent},
		{"K8sNodeReadyMinPercent", cfg.K8sNodeReadyMinPercent},
	}
	for _, c := range checks {
		if c.value == 0 {
			t.Errorf("DefaultEvaluatorConfig().%s should be non-zero", c.name)
		}
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelHealthy, "healthy"},
		{LevelWarning, "warning"},
		{LevelCritical, "critical"},
		{LevelUnknown, "unknown"},
		{Level(99), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

func TestEvaluateClaudeNilData(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	result := e.evaluateClaude(nil)

	if result.Level != LevelUnknown {
		t.Errorf("expected LevelUnknown, got %v", result.Level)
	}
	if result.Reason != "no data" {
		t.Errorf("expected reason 'no data', got %q", result.Reason)
	}
	if result.Component != "claude" {
		t.Errorf("expected component 'claude', got %q", result.Component)
	}
}

func TestEvaluateClaudeAllHealthy(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := makeClaudeUsage(
		makeAccount("personal", "ok", 30.0),
		makeAccount("work", "ok", 50.0),
	)

	result := e.evaluateClaude(data)

	if result.Level != LevelHealthy {
		t.Errorf("expected LevelHealthy, got %v", result.Level)
	}
	if result.Reason != "all accounts normal" {
		t.Errorf("expected reason 'all accounts normal', got %q", result.Reason)
	}
}

func TestEvaluateClaudeOneAccountWarning(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := makeClaudeUsage(
		makeAccount("personal", "ok", 30.0),
		makeAccount("work", "ok", 85.0),
	)

	result := e.evaluateClaude(data)

	if result.Level != LevelWarning {
		t.Errorf("expected LevelWarning, got %v", result.Level)
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestEvaluateClaudeOneAccountCritical(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := makeClaudeUsage(
		makeAccount("personal", "ok", 30.0),
		makeAccount("work", "ok", 97.0),
	)

	result := e.evaluateClaude(data)

	if result.Level != LevelCritical {
		t.Errorf("expected LevelCritical, got %v", result.Level)
	}
}

func TestEvaluateClaudeAuthFailedStatus(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	acct := collectors.ClaudeAccountUsage{
		Name:   "broken",
		Type:   "subscription",
		Tier:   "pro",
		Status: "auth_failed",
	}
	data := makeClaudeUsage(acct)

	result := e.evaluateClaude(data)

	if result.Level != LevelWarning {
		t.Errorf("expected LevelWarning for auth_failed, got %v", result.Level)
	}
	if result.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestEvaluateClaudeMultipleAccountsWorstWins(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := makeClaudeUsage(
		makeAccount("healthy", "ok", 10.0),
		makeAccount("warning", "ok", 85.0),
		makeAccount("critical", "ok", 98.0),
	)

	result := e.evaluateClaude(data)

	if result.Level != LevelCritical {
		t.Errorf("expected LevelCritical (worst wins), got %v", result.Level)
	}
}

func TestEvaluateBillingNilData(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	result := e.evaluateBilling(nil)

	if result.Level != LevelUnknown {
		t.Errorf("expected LevelUnknown, got %v", result.Level)
	}
	if result.Reason != "no data" {
		t.Errorf("expected reason 'no data', got %q", result.Reason)
	}
	if result.Component != "billing" {
		t.Errorf("expected component 'billing', got %q", result.Component)
	}
}

func TestEvaluateBillingHealthySpend(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := makeBillingData(50.0, ptrFloat64(200.0), nil,
		makeProvider("civo", "ok"),
	)

	result := e.evaluateBilling(data)

	if result.Level != LevelHealthy {
		t.Errorf("expected LevelHealthy, got %v", result.Level)
	}
}

func TestEvaluateBillingApproachingBudget(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	// 85% of 200 = 170, which exceeds 80% warning threshold (160).
	data := makeBillingData(170.0, ptrFloat64(200.0), nil,
		makeProvider("civo", "ok"),
	)

	result := e.evaluateBilling(data)

	if result.Level != LevelWarning {
		t.Errorf("expected LevelWarning, got %v", result.Level)
	}
}

func TestEvaluateBillingOverBudget(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	// 210 > 200 * 100% = 200.
	data := makeBillingData(210.0, ptrFloat64(200.0), nil,
		makeProvider("civo", "ok"),
	)

	result := e.evaluateBilling(data)

	if result.Level != LevelCritical {
		t.Errorf("expected LevelCritical, got %v", result.Level)
	}
}

func TestEvaluateBillingForecastExceedsBudget(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	// Spend is fine (50), but forecast (250) exceeds budget (200).
	data := makeBillingData(50.0, ptrFloat64(200.0), ptrFloat64(250.0),
		makeProvider("civo", "ok"),
	)

	result := e.evaluateBilling(data)

	if result.Level != LevelWarning {
		t.Errorf("expected LevelWarning for forecast exceeding budget, got %v", result.Level)
	}
}

func TestEvaluateBillingProviderError(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := makeBillingData(50.0, nil, nil,
		makeProvider("civo", "ok"),
		makeProvider("aws", "error"),
	)

	result := e.evaluateBilling(data)

	if result.Level != LevelWarning {
		t.Errorf("expected LevelWarning for provider error, got %v", result.Level)
	}
}

func TestEvaluateBillingNoBudgetSet(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := makeBillingData(150.0, nil, nil,
		makeProvider("civo", "ok"),
	)

	result := e.evaluateBilling(data)

	if result.Level != LevelHealthy {
		t.Errorf("expected LevelHealthy when no budget set, got %v", result.Level)
	}
}

func TestEvaluateInfraNilData(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	result := e.evaluateInfra(nil)

	if result.Level != LevelUnknown {
		t.Errorf("expected LevelUnknown, got %v", result.Level)
	}
	if result.Reason != "no data" {
		t.Errorf("expected reason 'no data', got %q", result.Reason)
	}
	if result.Component != "infra" {
		t.Errorf("expected component 'infra', got %q", result.Component)
	}
}

func TestEvaluateInfraTailscaleAllOnline(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := makeInfraStatus(5, 5)

	result := e.evaluateInfra(data)

	if result.Level != LevelHealthy {
		t.Errorf("expected LevelHealthy, got %v", result.Level)
	}
}

func TestEvaluateInfraTailscaleLowOnline(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	// 2/10 = 20%, below 50% warning threshold.
	data := makeInfraStatus(2, 10)

	result := e.evaluateInfra(data)

	if result.Level != LevelWarning {
		t.Errorf("expected LevelWarning, got %v", result.Level)
	}
}

func TestEvaluateInfraTailscaleAllOffline(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := makeInfraStatus(0, 5)

	result := e.evaluateInfra(data)

	if result.Level != LevelCritical {
		t.Errorf("expected LevelCritical when all nodes offline, got %v", result.Level)
	}
}

func TestEvaluateInfraK8sHealthyCluster(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := makeInfraStatus(-1, -1, // no tailscale
		makeCluster("prod", "healthy", 3, 3),
	)
	// Clear tailscale since we used -1 sentinel.
	data.Tailscale = nil

	result := e.evaluateInfra(data)

	if result.Level != LevelHealthy {
		t.Errorf("expected LevelHealthy, got %v", result.Level)
	}
}

func TestEvaluateInfraK8sDegradedCluster(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := &collectors.InfraStatus{
		Kubernetes: []collectors.KubernetesCluster{
			makeCluster("staging", "degraded", 3, 3),
		},
	}

	result := e.evaluateInfra(data)

	if result.Level != LevelWarning {
		t.Errorf("expected LevelWarning for degraded cluster, got %v", result.Level)
	}
}

func TestEvaluateInfraK8sOfflineCluster(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := &collectors.InfraStatus{
		Kubernetes: []collectors.KubernetesCluster{
			makeCluster("prod", "offline", 0, 3),
		},
	}

	result := e.evaluateInfra(data)

	if result.Level != LevelCritical {
		t.Errorf("expected LevelCritical for offline cluster, got %v", result.Level)
	}
}

func TestEvaluateInfraK8sNodesBelowThreshold(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	// 1/3 ready = 33%, below 80% threshold.
	data := &collectors.InfraStatus{
		Kubernetes: []collectors.KubernetesCluster{
			makeCluster("prod", "healthy", 1, 3),
		},
	}

	result := e.evaluateInfra(data)

	if result.Level != LevelWarning {
		t.Errorf("expected LevelWarning for low node readiness, got %v", result.Level)
	}
}

func TestEvaluateAggregatesWorstAcrossComponents(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())

	// Claude: critical, Billing: healthy, Infra: healthy.
	claude := makeClaudeUsage(makeAccount("work", "ok", 98.0))
	billing := makeBillingData(10.0, ptrFloat64(200.0), nil, makeProvider("civo", "ok"))
	infra := makeInfraStatus(5, 5)

	result := e.Evaluate(claude, billing, infra)

	if result.Overall != LevelCritical {
		t.Errorf("expected overall LevelCritical, got %v", result.Overall)
	}
	if len(result.Components) != 3 {
		t.Errorf("expected 3 components, got %d", len(result.Components))
	}
}

func TestEvaluateAllNil(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	result := e.Evaluate(nil, nil, nil)

	if result.Overall != LevelUnknown {
		t.Errorf("expected overall LevelUnknown, got %v", result.Overall)
	}
	for _, c := range result.Components {
		if c.Level != LevelUnknown {
			t.Errorf("expected component %s to be LevelUnknown, got %v", c.Component, c.Level)
		}
	}
}

func TestEvaluateMixedLevels(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())

	// Claude: warning, Billing: healthy, Infra: critical.
	claude := makeClaudeUsage(makeAccount("work", "ok", 85.0))
	billing := makeBillingData(10.0, ptrFloat64(200.0), nil, makeProvider("civo", "ok"))
	infra := &collectors.InfraStatus{
		Kubernetes: []collectors.KubernetesCluster{
			makeCluster("prod", "offline", 0, 3),
		},
	}

	result := e.Evaluate(claude, billing, infra)

	if result.Overall != LevelCritical {
		t.Errorf("expected overall LevelCritical (worst of mixed), got %v", result.Overall)
	}
}

func TestEvaluateCustomThresholds(t *testing.T) {
	cfg := EvaluatorConfig{
		ClaudeWarningPercent:         50.0, // Much lower than default
		ClaudeCriticalPercent:        70.0,
		BillingBudgetWarningPercent:  60.0,
		BillingBudgetCriticalPercent: 80.0,
		TailscaleWarningPercent:      80.0, // Much higher than default
		K8sNodeReadyMinPercent:       90.0,
	}
	e := NewEvaluator(cfg)

	// 55% would be healthy with defaults but warning with custom thresholds.
	claude := makeClaudeUsage(makeAccount("work", "ok", 55.0))
	result := e.evaluateClaude(claude)
	if result.Level != LevelWarning {
		t.Errorf("expected LevelWarning with custom threshold, got %v", result.Level)
	}

	// 75% would be warning with defaults but critical with custom thresholds.
	claude2 := makeClaudeUsage(makeAccount("work", "ok", 75.0))
	result2 := e.evaluateClaude(claude2)
	if result2.Level != LevelCritical {
		t.Errorf("expected LevelCritical with custom threshold, got %v", result2.Level)
	}
}

func TestEvaluateTimestamp(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	before := time.Now()
	result := e.Evaluate(nil, nil, nil)
	after := time.Now()

	if result.EvaluatedAt.Before(before) || result.EvaluatedAt.After(after) {
		t.Errorf("EvaluatedAt %v not between %v and %v", result.EvaluatedAt, before, after)
	}
}

func TestEvaluateComponentNames(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	result := e.Evaluate(nil, nil, nil)

	expected := []string{"claude", "billing", "infra"}
	if len(result.Components) != len(expected) {
		t.Fatalf("expected %d components, got %d", len(expected), len(result.Components))
	}
	for i, name := range expected {
		if result.Components[i].Component != name {
			t.Errorf("component[%d] = %q, want %q", i, result.Components[i].Component, name)
		}
	}
}

func TestWorstLevel(t *testing.T) {
	tests := []struct {
		name string
		a, b Level
		want Level
	}{
		{"healthy+healthy", LevelHealthy, LevelHealthy, LevelHealthy},
		{"healthy+warning", LevelHealthy, LevelWarning, LevelWarning},
		{"warning+critical", LevelWarning, LevelCritical, LevelCritical},
		{"unknown+warning", LevelUnknown, LevelWarning, LevelWarning},
		{"unknown+healthy", LevelUnknown, LevelHealthy, LevelUnknown},
		{"critical+unknown", LevelCritical, LevelUnknown, LevelCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := worstLevel(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("worstLevel(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestEvaluateClaudeTableDriven(t *testing.T) {
	tests := []struct {
		name      string
		data      *collectors.ClaudeUsage
		wantLevel Level
	}{
		{
			name:      "nil data",
			data:      nil,
			wantLevel: LevelUnknown,
		},
		{
			name:      "empty accounts",
			data:      &collectors.ClaudeUsage{Accounts: nil},
			wantLevel: LevelHealthy,
		},
		{
			name:      "single healthy account",
			data:      makeClaudeUsage(makeAccount("a", "ok", 10.0)),
			wantLevel: LevelHealthy,
		},
		{
			name:      "at exact warning boundary",
			data:      makeClaudeUsage(makeAccount("a", "ok", 80.0)),
			wantLevel: LevelHealthy, // Not exceeded, equal is not over.
		},
		{
			name:      "just above warning",
			data:      makeClaudeUsage(makeAccount("a", "ok", 80.1)),
			wantLevel: LevelWarning,
		},
		{
			name:      "at exact critical boundary",
			data:      makeClaudeUsage(makeAccount("a", "ok", 95.0)),
			wantLevel: LevelWarning, // 95 is not > 95, still warning range.
		},
		{
			name:      "just above critical",
			data:      makeClaudeUsage(makeAccount("a", "ok", 95.1)),
			wantLevel: LevelCritical,
		},
		{
			name:      "disabled status",
			data:      makeClaudeUsage(collectors.ClaudeAccountUsage{Name: "x", Status: "disabled"}),
			wantLevel: LevelWarning,
		},
		{
			name:      "rate_limited status",
			data:      makeClaudeUsage(collectors.ClaudeAccountUsage{Name: "x", Status: "rate_limited"}),
			wantLevel: LevelWarning,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEvaluator(DefaultEvaluatorConfig())
			result := e.evaluateClaude(tt.data)
			if result.Level != tt.wantLevel {
				t.Errorf("got %v, want %v (reason: %s)", result.Level, tt.wantLevel, result.Reason)
			}
		})
	}
}

func TestEvaluateInfraNoTailscaleNoK8s(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	data := &collectors.InfraStatus{}

	result := e.evaluateInfra(data)

	if result.Level != LevelHealthy {
		t.Errorf("expected LevelHealthy for empty infra, got %v", result.Level)
	}
}

func TestEvaluateInfraCombinedTailscaleAndK8s(t *testing.T) {
	e := NewEvaluator(DefaultEvaluatorConfig())
	// Tailscale: all online (healthy), K8s: offline (critical).
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "test.ts.net",
			OnlineCount: 5,
			TotalCount:  5,
		},
		Kubernetes: []collectors.KubernetesCluster{
			makeCluster("prod", "offline", 0, 3),
		},
	}

	result := e.evaluateInfra(data)

	if result.Level != LevelCritical {
		t.Errorf("expected LevelCritical (worst of tailscale+k8s), got %v", result.Level)
	}
}
