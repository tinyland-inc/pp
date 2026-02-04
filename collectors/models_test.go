package collectors

import (
	"encoding/json"
	"testing"
	"time"
)

// refTime is a fixed reference time used across model tests.
var refTime = time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

// f64p returns a pointer to a float64 value.
func f64p(v float64) *float64 {
	return &v
}

// ========== CollectResult JSON Round-Trip Tests ==========

func TestCollectResult_JSONRoundTrip_ClaudeUsage(t *testing.T) {
	original := &CollectResult{
		Collector: "claude",
		Timestamp: refTime,
		Data: &ClaudeUsage{
			Accounts: []ClaudeAccountUsage{
				{
					Name:   "personal",
					Type:   "subscription",
					Tier:   "pro",
					Status: "ok",
					FiveHour: &UsagePeriod{
						Utilization: 45.5,
						ResetsAt:    refTime.Add(3 * time.Hour),
					},
				},
			},
		},
		Warnings: []string{"test warning"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded CollectResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Collector != "claude" {
		t.Errorf("Collector = %q, want %q", decoded.Collector, "claude")
	}
	if !decoded.Timestamp.Equal(refTime) {
		t.Errorf("Timestamp = %v, want %v", decoded.Timestamp, refTime)
	}
	if len(decoded.Warnings) != 1 || decoded.Warnings[0] != "test warning" {
		t.Errorf("Warnings = %v, want [test warning]", decoded.Warnings)
	}

	// Data round-trips as a generic map due to interface{} typing.
	// Verify the structure is preserved by re-marshaling and decoding into the typed struct.
	dataBytes, err := json.Marshal(decoded.Data)
	if err != nil {
		t.Fatalf("json.Marshal(decoded.Data) failed: %v", err)
	}

	var usage ClaudeUsage
	if err := json.Unmarshal(dataBytes, &usage); err != nil {
		t.Fatalf("json.Unmarshal into ClaudeUsage failed: %v", err)
	}

	if len(usage.Accounts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(usage.Accounts))
	}
	if usage.Accounts[0].Name != "personal" {
		t.Errorf("Account.Name = %q, want %q", usage.Accounts[0].Name, "personal")
	}
	if usage.Accounts[0].FiveHour == nil {
		t.Fatal("FiveHour is nil after round-trip")
	}
	if usage.Accounts[0].FiveHour.Utilization != 45.5 {
		t.Errorf("FiveHour.Utilization = %v, want 45.5", usage.Accounts[0].FiveHour.Utilization)
	}
}

func TestCollectResult_JSONRoundTrip_BillingData(t *testing.T) {
	original := &CollectResult{
		Collector: "billing",
		Timestamp: refTime,
		Data: &BillingData{
			Providers: []ProviderBilling{
				{
					Provider:     "civo",
					AccountName:  "tinyland",
					Status:       "ok",
					DashboardURL: "https://dashboard.civo.com/billing",
					CurrentMonth: MonthCost{
						SpendUSD:    12.50,
						ForecastUSD: f64p(25.00),
						BudgetUSD:   f64p(50.00),
						StartDate:   "2025-01-01",
						EndDate:     "2025-01-31",
					},
					PreviousMonth: f64p(22.30),
					FetchedAt:     refTime,
				},
			},
			Total: BillingSummary{
				CurrentMonthUSD: 12.50,
				ForecastUSD:     f64p(25.00),
				BudgetUSD:       f64p(50.00),
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded CollectResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Collector != "billing" {
		t.Errorf("Collector = %q, want %q", decoded.Collector, "billing")
	}

	// Re-marshal Data and decode into BillingData.
	dataBytes, err := json.Marshal(decoded.Data)
	if err != nil {
		t.Fatalf("json.Marshal(decoded.Data) failed: %v", err)
	}

	var billing BillingData
	if err := json.Unmarshal(dataBytes, &billing); err != nil {
		t.Fatalf("json.Unmarshal into BillingData failed: %v", err)
	}

	if len(billing.Providers) != 1 {
		t.Fatalf("got %d providers, want 1", len(billing.Providers))
	}
	if billing.Providers[0].Provider != "civo" {
		t.Errorf("Provider = %q, want %q", billing.Providers[0].Provider, "civo")
	}
	if billing.Total.CurrentMonthUSD != 12.50 {
		t.Errorf("Total.CurrentMonthUSD = %v, want 12.50", billing.Total.CurrentMonthUSD)
	}
	if billing.Total.ForecastUSD == nil || *billing.Total.ForecastUSD != 25.00 {
		t.Errorf("Total.ForecastUSD = %v, want 25.00", billing.Total.ForecastUSD)
	}
}

func TestCollectResult_JSONRoundTrip_InfraStatus(t *testing.T) {
	original := &CollectResult{
		Collector: "infra",
		Timestamp: refTime,
		Data: &InfraStatus{
			Tailscale: &TailscaleStatus{
				Tailnet:     "tinyland.ts.net",
				OnlineCount: 5,
				TotalCount:  8,
				Nodes: []TailscaleNode{
					{
						Name:     "honey",
						Hostname: "honey",
						IP:       "100.64.0.1",
						OS:       "linux",
						Online:   true,
						LastSeen: refTime,
					},
				},
			},
			Kubernetes: []KubernetesCluster{
				{
					Name:       "civo-prod",
					Platform:   "civo",
					Status:     "healthy",
					TotalNodes: 3,
					ReadyNodes: 3,
				},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded CollectResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Re-marshal Data and decode into InfraStatus.
	dataBytes, err := json.Marshal(decoded.Data)
	if err != nil {
		t.Fatalf("json.Marshal(decoded.Data) failed: %v", err)
	}

	var infra InfraStatus
	if err := json.Unmarshal(dataBytes, &infra); err != nil {
		t.Fatalf("json.Unmarshal into InfraStatus failed: %v", err)
	}

	if infra.Tailscale == nil {
		t.Fatal("Tailscale is nil after round-trip")
	}
	if infra.Tailscale.Tailnet != "tinyland.ts.net" {
		t.Errorf("Tailnet = %q, want %q", infra.Tailscale.Tailnet, "tinyland.ts.net")
	}
	if infra.Tailscale.OnlineCount != 5 {
		t.Errorf("OnlineCount = %d, want 5", infra.Tailscale.OnlineCount)
	}
	if len(infra.Tailscale.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(infra.Tailscale.Nodes))
	}
	if infra.Tailscale.Nodes[0].Name != "honey" {
		t.Errorf("Node.Name = %q, want %q", infra.Tailscale.Nodes[0].Name, "honey")
	}
	if len(infra.Kubernetes) != 1 {
		t.Fatalf("got %d clusters, want 1", len(infra.Kubernetes))
	}
	if infra.Kubernetes[0].Name != "civo-prod" {
		t.Errorf("Cluster.Name = %q, want %q", infra.Kubernetes[0].Name, "civo-prod")
	}
}

// ========== ProviderBilling Tests ==========

func TestProviderBilling_WithForecast(t *testing.T) {
	pb := ProviderBilling{
		Provider:     "digitalocean",
		AccountName:  "tinyland-do",
		Status:       "ok",
		DashboardURL: "https://cloud.digitalocean.com/account/billing",
		CurrentMonth: MonthCost{
			SpendUSD:    45.67,
			ForecastUSD: f64p(91.34),
			BudgetUSD:   f64p(100.00),
			StartDate:   "2025-01-01",
			EndDate:     "2025-01-31",
		},
		PreviousMonth: f64p(88.20),
		FetchedAt:     refTime,
	}

	// Verify all fields survive JSON round-trip.
	data, err := json.Marshal(pb)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ProviderBilling
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Provider != "digitalocean" {
		t.Errorf("Provider = %q, want %q", decoded.Provider, "digitalocean")
	}
	if decoded.AccountName != "tinyland-do" {
		t.Errorf("AccountName = %q, want %q", decoded.AccountName, "tinyland-do")
	}
	if decoded.Status != "ok" {
		t.Errorf("Status = %q, want %q", decoded.Status, "ok")
	}
	if decoded.DashboardURL != "https://cloud.digitalocean.com/account/billing" {
		t.Errorf("DashboardURL = %q, want the DO billing URL", decoded.DashboardURL)
	}
	if decoded.CurrentMonth.SpendUSD != 45.67 {
		t.Errorf("SpendUSD = %v, want 45.67", decoded.CurrentMonth.SpendUSD)
	}
	if decoded.CurrentMonth.ForecastUSD == nil || *decoded.CurrentMonth.ForecastUSD != 91.34 {
		t.Errorf("ForecastUSD = %v, want 91.34", decoded.CurrentMonth.ForecastUSD)
	}
	if decoded.CurrentMonth.BudgetUSD == nil || *decoded.CurrentMonth.BudgetUSD != 100.00 {
		t.Errorf("BudgetUSD = %v, want 100.00", decoded.CurrentMonth.BudgetUSD)
	}
	if decoded.CurrentMonth.StartDate != "2025-01-01" {
		t.Errorf("StartDate = %q, want %q", decoded.CurrentMonth.StartDate, "2025-01-01")
	}
	if decoded.CurrentMonth.EndDate != "2025-01-31" {
		t.Errorf("EndDate = %q, want %q", decoded.CurrentMonth.EndDate, "2025-01-31")
	}
	if decoded.PreviousMonth == nil || *decoded.PreviousMonth != 88.20 {
		t.Errorf("PreviousMonth = %v, want 88.20", decoded.PreviousMonth)
	}
	if !decoded.FetchedAt.Equal(refTime) {
		t.Errorf("FetchedAt = %v, want %v", decoded.FetchedAt, refTime)
	}
}

func TestProviderBilling_MinimalFields(t *testing.T) {
	// Only required fields, no optional pointers.
	pb := ProviderBilling{
		Provider:    "aws",
		AccountName: "sandbox",
		Status:      "ok",
		CurrentMonth: MonthCost{
			SpendUSD:  3.21,
			StartDate: "2025-01-01",
			EndDate:   "2025-01-31",
		},
		FetchedAt: refTime,
	}

	data, err := json.Marshal(pb)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ProviderBilling
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.CurrentMonth.ForecastUSD != nil {
		t.Errorf("ForecastUSD = %v, want nil", decoded.CurrentMonth.ForecastUSD)
	}
	if decoded.CurrentMonth.BudgetUSD != nil {
		t.Errorf("BudgetUSD = %v, want nil", decoded.CurrentMonth.BudgetUSD)
	}
	if decoded.PreviousMonth != nil {
		t.Errorf("PreviousMonth = %v, want nil", decoded.PreviousMonth)
	}
	if decoded.DashboardURL != "" {
		t.Errorf("DashboardURL = %q, want empty", decoded.DashboardURL)
	}
}

// ========== ClaudeAccountUsage Tests ==========

func TestClaudeAccountUsage_AllFields(t *testing.T) {
	acct := ClaudeAccountUsage{
		Name:   "personal",
		Type:   "subscription",
		Tier:   "max_5x",
		Status: "ok",
		FiveHour: &UsagePeriod{
			Utilization: 67.5,
			ResetsAt:    refTime.Add(2 * time.Hour),
		},
		SevenDay: &UsagePeriod{
			Utilization: 30.0,
			ResetsAt:    refTime.Add(5 * 24 * time.Hour),
		},
		ExtraUsage: &ExtraUsage{
			Enabled:      true,
			MonthlyLimit: 10000,
			UsedCredits:  2500.50,
			Utilization:  25.005,
		},
	}

	data, err := json.Marshal(acct)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ClaudeAccountUsage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Name != "personal" {
		t.Errorf("Name = %q, want %q", decoded.Name, "personal")
	}
	if decoded.Type != "subscription" {
		t.Errorf("Type = %q, want %q", decoded.Type, "subscription")
	}
	if decoded.Tier != "max_5x" {
		t.Errorf("Tier = %q, want %q", decoded.Tier, "max_5x")
	}
	if decoded.Status != "ok" {
		t.Errorf("Status = %q, want %q", decoded.Status, "ok")
	}

	// FiveHour
	if decoded.FiveHour == nil {
		t.Fatal("FiveHour is nil after round-trip")
	}
	if decoded.FiveHour.Utilization != 67.5 {
		t.Errorf("FiveHour.Utilization = %v, want 67.5", decoded.FiveHour.Utilization)
	}

	// SevenDay
	if decoded.SevenDay == nil {
		t.Fatal("SevenDay is nil after round-trip")
	}
	if decoded.SevenDay.Utilization != 30.0 {
		t.Errorf("SevenDay.Utilization = %v, want 30.0", decoded.SevenDay.Utilization)
	}

	// ExtraUsage
	if decoded.ExtraUsage == nil {
		t.Fatal("ExtraUsage is nil after round-trip")
	}
	if !decoded.ExtraUsage.Enabled {
		t.Error("ExtraUsage.Enabled = false, want true")
	}
	if decoded.ExtraUsage.MonthlyLimit != 10000 {
		t.Errorf("ExtraUsage.MonthlyLimit = %d, want 10000", decoded.ExtraUsage.MonthlyLimit)
	}
	if decoded.ExtraUsage.UsedCredits != 2500.50 {
		t.Errorf("ExtraUsage.UsedCredits = %v, want 2500.50", decoded.ExtraUsage.UsedCredits)
	}
	if decoded.ExtraUsage.Utilization != 25.005 {
		t.Errorf("ExtraUsage.Utilization = %v, want 25.005", decoded.ExtraUsage.Utilization)
	}
}

func TestClaudeAccountUsage_APIWithRateLimits(t *testing.T) {
	resetTime := refTime.Add(1 * time.Minute)
	acct := ClaudeAccountUsage{
		Name:   "ci-pipeline",
		Type:   "api",
		Tier:   "tier_3",
		Status: "ok",
		RateLimits: &APIRateLimits{
			RequestsLimit:     4000,
			RequestsRemaining: 3200,
			RequestsReset:     resetTime,
			TokensLimit:       400000,
			TokensRemaining:   350000,
			TokensReset:       resetTime,
		},
	}

	data, err := json.Marshal(acct)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ClaudeAccountUsage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.RateLimits == nil {
		t.Fatal("RateLimits is nil after round-trip")
	}
	if decoded.RateLimits.RequestsLimit != 4000 {
		t.Errorf("RequestsLimit = %d, want 4000", decoded.RateLimits.RequestsLimit)
	}
	if decoded.RateLimits.RequestsRemaining != 3200 {
		t.Errorf("RequestsRemaining = %d, want 3200", decoded.RateLimits.RequestsRemaining)
	}
	if decoded.RateLimits.TokensLimit != 400000 {
		t.Errorf("TokensLimit = %d, want 400000", decoded.RateLimits.TokensLimit)
	}
	if decoded.RateLimits.TokensRemaining != 350000 {
		t.Errorf("TokensRemaining = %d, want 350000", decoded.RateLimits.TokensRemaining)
	}

	// Verify optional fields are nil when not set.
	if decoded.FiveHour != nil {
		t.Errorf("FiveHour = %v, want nil for API account", decoded.FiveHour)
	}
	if decoded.SevenDay != nil {
		t.Errorf("SevenDay = %v, want nil for API account", decoded.SevenDay)
	}
	if decoded.ExtraUsage != nil {
		t.Errorf("ExtraUsage = %v, want nil for API account", decoded.ExtraUsage)
	}
}

// ========== KubernetesCluster Status Tests ==========

func TestKubernetesCluster_HealthyStatus(t *testing.T) {
	cluster := KubernetesCluster{
		Name:       "civo-prod",
		Platform:   "civo",
		Status:     "healthy",
		TotalNodes: 3,
		ReadyNodes: 3,
		Nodes: []KubernetesNode{
			{Name: "node-1", Status: "Ready", CPUPercent: 45, MemPercent: 60, PodCount: 25, MaxPods: 110},
			{Name: "node-2", Status: "Ready", CPUPercent: 30, MemPercent: 55, PodCount: 18, MaxPods: 110},
			{Name: "node-3", Status: "Ready", CPUPercent: 50, MemPercent: 70, PodCount: 30, MaxPods: 110},
		},
	}

	data, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded KubernetesCluster
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Status != "healthy" {
		t.Errorf("Status = %q, want %q", decoded.Status, "healthy")
	}
	if decoded.TotalNodes != 3 {
		t.Errorf("TotalNodes = %d, want 3", decoded.TotalNodes)
	}
	if decoded.ReadyNodes != 3 {
		t.Errorf("ReadyNodes = %d, want 3", decoded.ReadyNodes)
	}
	if len(decoded.Nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(decoded.Nodes))
	}
}

func TestKubernetesCluster_DegradedStatus(t *testing.T) {
	cluster := KubernetesCluster{
		Name:       "rke2-home",
		Platform:   "rke2",
		Status:     "degraded",
		TotalNodes: 3,
		ReadyNodes: 2,
		Nodes: []KubernetesNode{
			{Name: "cp-1", Status: "Ready"},
			{Name: "worker-1", Status: "Ready"},
			{Name: "worker-2", Status: "NotReady"},
		},
	}

	data, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded KubernetesCluster
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Status != "degraded" {
		t.Errorf("Status = %q, want %q", decoded.Status, "degraded")
	}
	if decoded.ReadyNodes != 2 {
		t.Errorf("ReadyNodes = %d, want 2", decoded.ReadyNodes)
	}

	// Verify the NotReady node is preserved.
	notReadyFound := false
	for _, n := range decoded.Nodes {
		if n.Status == "NotReady" {
			notReadyFound = true
			if n.Name != "worker-2" {
				t.Errorf("NotReady node Name = %q, want %q", n.Name, "worker-2")
			}
		}
	}
	if !notReadyFound {
		t.Error("no NotReady node found after round-trip")
	}
}

func TestKubernetesCluster_OfflineStatus(t *testing.T) {
	cluster := KubernetesCluster{
		Name:       "dev-kind",
		Platform:   "kind",
		Status:     "offline",
		TotalNodes: 1,
		ReadyNodes: 0,
	}

	data, err := json.Marshal(cluster)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded KubernetesCluster
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Status != "offline" {
		t.Errorf("Status = %q, want %q", decoded.Status, "offline")
	}
	if decoded.ReadyNodes != 0 {
		t.Errorf("ReadyNodes = %d, want 0", decoded.ReadyNodes)
	}
}

// ========== MonthCost Tests ==========

func TestMonthCost_AllFields(t *testing.T) {
	mc := MonthCost{
		SpendUSD:    156.78,
		ForecastUSD: f64p(312.00),
		BudgetUSD:   f64p(400.00),
		StartDate:   "2025-01-01",
		EndDate:     "2025-01-31",
	}

	data, err := json.Marshal(mc)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded MonthCost
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.SpendUSD != 156.78 {
		t.Errorf("SpendUSD = %v, want 156.78", decoded.SpendUSD)
	}
	if decoded.ForecastUSD == nil || *decoded.ForecastUSD != 312.00 {
		t.Errorf("ForecastUSD = %v, want 312.00", decoded.ForecastUSD)
	}
	if decoded.BudgetUSD == nil || *decoded.BudgetUSD != 400.00 {
		t.Errorf("BudgetUSD = %v, want 400.00", decoded.BudgetUSD)
	}
	if decoded.StartDate != "2025-01-01" {
		t.Errorf("StartDate = %q, want %q", decoded.StartDate, "2025-01-01")
	}
	if decoded.EndDate != "2025-01-31" {
		t.Errorf("EndDate = %q, want %q", decoded.EndDate, "2025-01-31")
	}
}

func TestMonthCost_OmitsNilOptionals(t *testing.T) {
	mc := MonthCost{
		SpendUSD:  10.00,
		StartDate: "2025-01-01",
		EndDate:   "2025-01-31",
	}

	data, err := json.Marshal(mc)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Verify omitempty works: the JSON should not contain forecast_usd or budget_usd.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal into map failed: %v", err)
	}

	if _, exists := raw["forecast_usd"]; exists {
		t.Error("forecast_usd should be omitted from JSON when nil")
	}
	if _, exists := raw["budget_usd"]; exists {
		t.Error("budget_usd should be omitted from JSON when nil")
	}
}

// ========== TailscaleNode Tests ==========

func TestTailscaleNode_AllFields(t *testing.T) {
	node := TailscaleNode{
		Name:         "honey",
		Hostname:     "honey.tinyland.ts.net",
		IP:           "100.64.0.1",
		OS:           "linux",
		Online:       true,
		LastSeen:     refTime,
		Tags:         []string{"tag:server", "tag:gpu"},
		DashboardURL: "https://login.tailscale.com/admin/machines/honey",
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded TailscaleNode
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Name != "honey" {
		t.Errorf("Name = %q, want %q", decoded.Name, "honey")
	}
	if decoded.IP != "100.64.0.1" {
		t.Errorf("IP = %q, want %q", decoded.IP, "100.64.0.1")
	}
	if !decoded.Online {
		t.Error("Online = false, want true")
	}
	if len(decoded.Tags) != 2 {
		t.Errorf("got %d tags, want 2", len(decoded.Tags))
	}
	if !decoded.LastSeen.Equal(refTime) {
		t.Errorf("LastSeen = %v, want %v", decoded.LastSeen, refTime)
	}
}

func TestTailscaleNode_OmitsNilTags(t *testing.T) {
	node := TailscaleNode{
		Name:   "laptop",
		Online: false,
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal into map failed: %v", err)
	}

	if _, exists := raw["tags"]; exists {
		t.Error("tags should be omitted from JSON when nil")
	}
}

// ========== CollectResult Warnings Omit Empty Test ==========

func TestCollectResult_WarningsOmitEmpty(t *testing.T) {
	result := CollectResult{
		Collector: "test",
		Timestamp: refTime,
		Data:      "hello",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal into map failed: %v", err)
	}

	if _, exists := raw["warnings"]; exists {
		t.Error("warnings should be omitted from JSON when empty")
	}
}
