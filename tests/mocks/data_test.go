package mocks

import (
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/widgets"
)

func TestMockClaudeAccounts_VariousCount(t *testing.T) {
	SeedRandom(42)

	tests := []struct {
		name  string
		count int
	}{
		{"1 account", 1},
		{"3 accounts", 3},
		{"5 accounts", 5},
		{"10 accounts", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accounts := MockClaudeAccounts(tt.count)
			if len(accounts) != tt.count {
				t.Errorf("MockClaudeAccounts(%d) returned %d accounts, want %d", tt.count, len(accounts), tt.count)
			}

			// Verify each account has required fields
			for i, acct := range accounts {
				if acct.Name == "" {
					t.Errorf("Account %d has empty name", i)
				}
				if acct.Type != "subscription" && acct.Type != "api" {
					t.Errorf("Account %d has invalid type: %s", i, acct.Type)
				}
				if acct.Status == "" {
					t.Errorf("Account %d has empty status", i)
				}
				if acct.Tier == "" {
					t.Errorf("Account %d has empty tier", i)
				}

				// Verify type-specific data
				if acct.Type == "subscription" {
					if acct.FiveHour == nil {
						t.Errorf("Subscription account %d missing FiveHour data", i)
					}
					if acct.SevenDay == nil {
						t.Errorf("Subscription account %d missing SevenDay data", i)
					}
				} else if acct.Type == "api" {
					if acct.RateLimits == nil {
						t.Errorf("API account %d missing RateLimits data", i)
					}
				}
			}
		})
	}
}

func TestMockClaudeAccounts_ZeroCount(t *testing.T) {
	accounts := MockClaudeAccounts(0)
	if accounts != nil {
		t.Errorf("MockClaudeAccounts(0) should return nil, got %v", accounts)
	}

	accounts = MockClaudeAccounts(-1)
	if accounts != nil {
		t.Errorf("MockClaudeAccounts(-1) should return nil, got %v", accounts)
	}
}

func TestMockClaudeAccount_Subscription(t *testing.T) {
	cfg := DefaultSubscriptionConfig("test-sub")
	cfg.FiveHourUtil = 75.0
	cfg.SevenDayUtil = 50.0
	cfg.ExtraUsageEnabled = true
	cfg.ExtraUsageUtil = 30.0
	cfg.ExtraUsageLimit = 10000

	acct := MockClaudeAccount(cfg)

	if acct.Name != "test-sub" {
		t.Errorf("Name = %s, want test-sub", acct.Name)
	}
	if acct.Type != "subscription" {
		t.Errorf("Type = %s, want subscription", acct.Type)
	}
	if acct.FiveHour == nil {
		t.Fatal("FiveHour is nil")
	}
	if acct.FiveHour.Utilization != 75.0 {
		t.Errorf("FiveHour.Utilization = %f, want 75.0", acct.FiveHour.Utilization)
	}
	if acct.SevenDay == nil {
		t.Fatal("SevenDay is nil")
	}
	if acct.SevenDay.Utilization != 50.0 {
		t.Errorf("SevenDay.Utilization = %f, want 50.0", acct.SevenDay.Utilization)
	}
	if acct.ExtraUsage == nil {
		t.Fatal("ExtraUsage is nil")
	}
	if !acct.ExtraUsage.Enabled {
		t.Error("ExtraUsage.Enabled = false, want true")
	}
}

func TestMockClaudeAccount_API(t *testing.T) {
	cfg := DefaultAPIConfig("test-api")
	cfg.APIRequestsUsed = 500
	cfg.APIRequestsLimit = 1000
	cfg.APITokensUsed = 50000
	cfg.APITokensLimit = 100000

	acct := MockClaudeAccount(cfg)

	if acct.Name != "test-api" {
		t.Errorf("Name = %s, want test-api", acct.Name)
	}
	if acct.Type != "api" {
		t.Errorf("Type = %s, want api", acct.Type)
	}
	if acct.RateLimits == nil {
		t.Fatal("RateLimits is nil")
	}
	if acct.RateLimits.RequestsLimit != 1000 {
		t.Errorf("RequestsLimit = %d, want 1000", acct.RateLimits.RequestsLimit)
	}
	if acct.RateLimits.RequestsRemaining != 500 {
		t.Errorf("RequestsRemaining = %d, want 500", acct.RateLimits.RequestsRemaining)
	}
	if acct.RateLimits.TokensLimit != 100000 {
		t.Errorf("TokensLimit = %d, want 100000", acct.RateLimits.TokensLimit)
	}
	if acct.RateLimits.TokensRemaining != 50000 {
		t.Errorf("TokensRemaining = %d, want 50000", acct.RateLimits.TokensRemaining)
	}
}

func TestMockClaudeUsageWithError(t *testing.T) {
	usage := MockClaudeUsageWithError(3, 1)

	if len(usage.Accounts) != 3 {
		t.Fatalf("Expected 3 accounts, got %d", len(usage.Accounts))
	}

	if usage.Accounts[1].Status != "auth_failed" {
		t.Errorf("Account 1 status = %s, want auth_failed", usage.Accounts[1].Status)
	}
	if usage.Accounts[0].Status == "auth_failed" {
		t.Error("Account 0 should not have auth_failed status")
	}
}

func TestMockClaudeUsageRateLimited(t *testing.T) {
	usage := MockClaudeUsageRateLimited(3, 2)

	if len(usage.Accounts) != 3 {
		t.Fatalf("Expected 3 accounts, got %d", len(usage.Accounts))
	}

	if usage.Accounts[2].Status != "rate_limited" {
		t.Errorf("Account 2 status = %s, want rate_limited", usage.Accounts[2].Status)
	}
}

func TestMockBillingData(t *testing.T) {
	SeedRandom(42)
	data := MockBillingData()

	if data == nil {
		t.Fatal("MockBillingData returned nil")
	}

	if len(data.Providers) != 4 {
		t.Errorf("Expected 4 providers, got %d", len(data.Providers))
	}

	// Verify provider names
	expectedProviders := map[string]bool{
		"civo":         true,
		"digitalocean": true,
		"aws":          true,
		"dreamhost":    true,
	}
	for _, p := range data.Providers {
		if !expectedProviders[p.Provider] {
			t.Errorf("Unexpected provider: %s", p.Provider)
		}
		if p.Status != "ok" {
			t.Errorf("Provider %s status = %s, want ok", p.Provider, p.Status)
		}
		if p.CurrentMonth.SpendUSD <= 0 {
			t.Errorf("Provider %s has zero spend", p.Provider)
		}
	}

	// Verify total is sum of providers
	var sum float64
	for _, p := range data.Providers {
		sum += p.CurrentMonth.SpendUSD
	}
	if data.Total.CurrentMonthUSD != sum {
		t.Errorf("Total = %f, want %f", data.Total.CurrentMonthUSD, sum)
	}

	// Verify history exists
	if data.History == nil {
		t.Error("History is nil")
	} else if len(data.History.TotalHistory) != 30 {
		t.Errorf("TotalHistory has %d days, want 30", len(data.History.TotalHistory))
	}
}

func TestMockBillingDataWithError(t *testing.T) {
	data := MockBillingDataWithError("civo")

	var civoFound bool
	for _, p := range data.Providers {
		if p.Provider == "civo" {
			civoFound = true
			if p.Status != "error" {
				t.Errorf("Civo status = %s, want error", p.Status)
			}
		} else {
			if p.Status == "error" {
				t.Errorf("Provider %s should not have error status", p.Provider)
			}
		}
	}
	if !civoFound {
		t.Error("Civo provider not found")
	}
}

func TestMockBillingDataOverBudget(t *testing.T) {
	data := MockBillingDataOverBudget()

	for _, p := range data.Providers {
		if p.CurrentMonth.BudgetUSD != nil {
			if p.CurrentMonth.SpendUSD <= *p.CurrentMonth.BudgetUSD {
				t.Errorf("Provider %s should be over budget", p.Provider)
			}
		}
	}
}

func TestMockDailySpend(t *testing.T) {
	history := MockDailySpend(30, 100.0)

	if len(history) != 30 {
		t.Errorf("Expected 30 days, got %d", len(history))
	}

	for _, day := range history {
		if day.Date == "" {
			t.Error("Day has empty date")
		}
		if day.SpendUSD < 0 {
			t.Errorf("Day %s has negative spend: %f", day.Date, day.SpendUSD)
		}
	}
}

func TestMockTailscaleStatus(t *testing.T) {
	ts := MockTailscaleStatus()

	if ts == nil {
		t.Fatal("MockTailscaleStatus returned nil")
	}

	if ts.Tailnet != "tinyland.ts.net" {
		t.Errorf("Tailnet = %s, want tinyland.ts.net", ts.Tailnet)
	}

	if ts.TotalCount != len(ts.Nodes) {
		t.Errorf("TotalCount = %d, but have %d nodes", ts.TotalCount, len(ts.Nodes))
	}

	// Count online nodes
	var online int
	for _, n := range ts.Nodes {
		if n.Online {
			online++
		}
		// Verify node has required fields
		if n.Name == "" {
			t.Error("Node has empty name")
		}
		if n.IP == "" {
			t.Error("Node has empty IP")
		}
	}

	if ts.OnlineCount != online {
		t.Errorf("OnlineCount = %d, but counted %d online", ts.OnlineCount, online)
	}
}

func TestMockKubernetesCluster(t *testing.T) {
	cfgs := DefaultKubernetesClusters()
	if len(cfgs) == 0 {
		t.Fatal("No default clusters")
	}

	cluster := MockKubernetesCluster(cfgs[0])

	if cluster.Name == "" {
		t.Error("Cluster name is empty")
	}
	if cluster.Platform == "" {
		t.Error("Cluster platform is empty")
	}
	if cluster.Status == "" {
		t.Error("Cluster status is empty")
	}
	if len(cluster.Nodes) != len(cfgs[0].Nodes) {
		t.Errorf("Expected %d nodes, got %d", len(cfgs[0].Nodes), len(cluster.Nodes))
	}
}

func TestMockInfraStatus(t *testing.T) {
	infra := MockInfraStatus()

	if infra == nil {
		t.Fatal("MockInfraStatus returned nil")
	}

	if infra.Tailscale == nil {
		t.Error("Tailscale is nil")
	}

	if len(infra.Kubernetes) == 0 {
		t.Error("Kubernetes is empty")
	}
}

func TestMockInfraStatusAllOffline(t *testing.T) {
	infra := MockInfraStatusAllOffline()

	if infra.Tailscale.OnlineCount != 0 {
		t.Errorf("Expected 0 online nodes, got %d", infra.Tailscale.OnlineCount)
	}

	for _, n := range infra.Tailscale.Nodes {
		if n.Online {
			t.Errorf("Node %s should be offline", n.Name)
		}
	}

	for _, c := range infra.Kubernetes {
		if c.Status != "offline" {
			t.Errorf("Cluster %s status = %s, want offline", c.Name, c.Status)
		}
		if c.ReadyNodes != 0 {
			t.Errorf("Cluster %s has %d ready nodes, want 0", c.Name, c.ReadyNodes)
		}
	}
}

func TestMockBillingPanelData(t *testing.T) {
	data := MockBillingPanelData()

	if len(data.Providers) != 4 {
		t.Errorf("Expected 4 providers, got %d", len(data.Providers))
	}

	if data.TotalCurrent <= 0 {
		t.Error("TotalCurrent is zero or negative")
	}

	if data.FetchedAt.IsZero() {
		t.Error("FetchedAt is zero")
	}

	// Verify each provider has history for sparklines
	for _, p := range data.Providers {
		if len(p.History) == 0 {
			t.Errorf("Provider %s has no history for sparkline", p.Name)
		}
	}
}

func TestEdgeCases_EmptyData(t *testing.T) {
	// Empty Claude usage
	claude := MockClaudeUsageEmpty()
	if claude == nil || len(claude.Accounts) != 0 {
		t.Error("MockClaudeUsageEmpty should return empty accounts slice")
	}

	// Nil Claude usage
	claudeNil := MockClaudeUsageNil()
	if claudeNil != nil {
		t.Error("MockClaudeUsageNil should return nil")
	}

	// Empty billing
	billing := MockBillingDataEmpty()
	if billing == nil || len(billing.Providers) != 0 {
		t.Error("MockBillingDataEmpty should return empty providers slice")
	}

	// Empty infra
	infra := MockInfraStatusEmpty()
	if infra == nil {
		t.Error("MockInfraStatusEmpty should not return nil")
	}
	if infra.Tailscale != nil {
		t.Error("Empty infra should have nil Tailscale")
	}
	if len(infra.Kubernetes) != 0 {
		t.Error("Empty infra should have no Kubernetes clusters")
	}
}

func TestMockClaudeUsageAllErrors(t *testing.T) {
	usage := MockClaudeUsageAllErrors(3)

	for i, acct := range usage.Accounts {
		if acct.Status == "ok" {
			t.Errorf("Account %d should have error status", i)
		}
		validStatuses := map[string]bool{
			"auth_failed":  true,
			"rate_limited": true,
			"disabled":     true,
		}
		if !validStatuses[acct.Status] {
			t.Errorf("Account %d has invalid error status: %s", i, acct.Status)
		}
	}
}

func TestMockClaudeUsageHighUtilization(t *testing.T) {
	usage := MockClaudeUsageHighUtilization(4)

	for i, acct := range usage.Accounts {
		util := acct.GetPrimaryUtilization()
		if util < 80 {
			t.Errorf("Account %d utilization = %f, want >= 80", i, util)
		}
	}
}

func TestSeedRandom_Reproducibility(t *testing.T) {
	SeedRandom(12345)
	accounts1 := MockClaudeAccounts(3)

	SeedRandom(12345)
	accounts2 := MockClaudeAccounts(3)

	for i := 0; i < len(accounts1); i++ {
		if accounts1[i].Name != accounts2[i].Name {
			t.Errorf("Account %d name mismatch: %s vs %s", i, accounts1[i].Name, accounts2[i].Name)
		}
		if accounts1[i].Tier != accounts2[i].Tier {
			t.Errorf("Account %d tier mismatch: %s vs %s", i, accounts1[i].Tier, accounts2[i].Tier)
		}
	}
}

// ========== Widget Integration Tests ==========

func TestMockDataWithGaugeWidget(t *testing.T) {
	SeedRandom(42)
	usage := MockClaudeUsage(3)

	for i, acct := range usage.Accounts {
		util := acct.GetPrimaryUtilization()
		cfg := widgets.DefaultGaugeConfig()
		cfg.Percent = util
		cfg.Label = acct.Name

		result := widgets.RenderGauge(cfg)
		if result == "" {
			t.Errorf("Account %d gauge rendered empty", i)
		}
	}
}

func TestMockDataWithSparklineWidget(t *testing.T) {
	panelData := MockBillingPanelData()

	for _, provider := range panelData.Providers {
		if len(provider.History) > 0 {
			result := widgets.RenderSparkline(widgets.SparklineConfig{
				Data:  provider.History,
				Width: 20,
			})
			if result == "" {
				t.Errorf("Provider %s sparkline rendered empty", provider.Name)
			}
		}
	}
}

func TestMockDataWithBillingPanel(t *testing.T) {
	panelData := MockBillingPanelData()
	cfg := widgets.DefaultBillingPanelConfig()

	result := widgets.RenderBillingPanel(panelData, cfg)
	if result == "" {
		t.Error("BillingPanel rendered empty")
	}

	// Verify it contains expected content
	if !containsString(result, "Billing Dashboard") {
		t.Error("BillingPanel missing title")
	}
	if !containsString(result, "Total:") {
		t.Error("BillingPanel missing total summary")
	}
}

func TestMockDataWithInfraPanel(t *testing.T) {
	infra := MockInfraStatus()
	panel := widgets.NewInfraPanel(widgets.DefaultInfraPanelConfig())

	result := panel.Render(infra)
	if result == "" {
		t.Error("InfraPanel rendered empty")
	}

	// Verify it contains expected content
	if !containsString(result, "Tailscale") {
		t.Error("InfraPanel missing Tailscale section")
	}
	if !containsString(result, "Kubernetes") {
		t.Error("InfraPanel missing Kubernetes section")
	}
}

func TestMockDataWithTableWidget(t *testing.T) {
	usage := MockClaudeUsage(3)

	// Build table from Claude data
	columns := []widgets.Column{
		{Title: "Account", Width: 15},
		{Title: "Type", Width: 12},
		{Title: "Status", Width: 10},
		{Title: "Utilization", Width: 12},
	}

	rows := make([][]string, len(usage.Accounts))
	for i, acct := range usage.Accounts {
		rows[i] = []string{
			acct.Name,
			acct.Type,
			acct.Status,
			intToStr(int(acct.GetPrimaryUtilization())) + "%",
		}
	}

	cfg := widgets.DefaultTableConfig()
	cfg.Columns = columns
	cfg.Rows = rows

	result := widgets.RenderTable(cfg)

	if result == "" {
		t.Error("Table rendered empty")
	}

	// Verify it contains account data
	for _, acct := range usage.Accounts {
		if !containsString(result, acct.Name) {
			t.Errorf("Table missing account name: %s", acct.Name)
		}
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ========== Multi-Account Display Tests ==========

func TestMultiAccountDisplay(t *testing.T) {
	SeedRandom(42)

	tests := []struct {
		name     string
		accounts int
		cols     int
	}{
		{"1 account - compact", 1, 80},
		{"3 accounts - standard", 3, 120},
		{"5 accounts - wide", 5, 160},
		{"5 accounts - ultrawide", 5, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := MockClaudeUsage(tt.accounts)

			// Verify correct number of accounts
			if len(usage.Accounts) != tt.accounts {
				t.Errorf("Got %d accounts, want %d", len(usage.Accounts), tt.accounts)
			}

			// Simulate width-aware rendering by adjusting gauge width
			gaugeWidth := tt.cols / 4
			if gaugeWidth > 30 {
				gaugeWidth = 30
			}
			if gaugeWidth < 8 {
				gaugeWidth = 8
			}

			for _, acct := range usage.Accounts {
				cfg := widgets.DefaultGaugeConfig()
				cfg.Width = gaugeWidth
				cfg.Percent = acct.GetPrimaryUtilization()

				result := widgets.RenderGauge(cfg)
				if result == "" {
					t.Errorf("Gauge for account %s rendered empty at width %d", acct.Name, tt.cols)
				}
			}
		})
	}
}

func TestMockDataResetTimes(t *testing.T) {
	SeedRandom(42)
	usage := MockClaudeUsage(3)

	for i, acct := range usage.Accounts {
		schedule := acct.GetResetSchedule()

		if acct.Type == "subscription" {
			// Subscription accounts should have session reset in the future
			if !schedule.SessionResets.IsZero() && schedule.SessionResets.Before(time.Now()) {
				t.Errorf("Account %d session reset in the past", i)
			}
			// Weekly reset should be in the future
			if !schedule.WeeklyResets.IsZero() && schedule.WeeklyResets.Before(time.Now()) {
				t.Errorf("Account %d weekly reset in the past", i)
			}
		} else if acct.Type == "api" {
			// API accounts should have request reset in the future
			if !schedule.SessionResets.IsZero() && schedule.SessionResets.Before(time.Now()) {
				t.Errorf("Account %d (API) request reset in the past", i)
			}
		}
	}
}

func TestProviderBillingHistory(t *testing.T) {
	data := MockBillingData()

	// Verify history consistency
	if data.History == nil {
		t.Fatal("History is nil")
	}

	// Each provider should have history
	for _, p := range data.Providers {
		history, ok := data.History.ProviderHistory[p.Provider]
		if !ok {
			t.Errorf("Missing history for provider %s", p.Provider)
			continue
		}
		if len(history) != 30 {
			t.Errorf("Provider %s has %d days of history, want 30", p.Provider, len(history))
		}
	}

	// Total history should aggregate all providers
	if len(data.History.TotalHistory) != 30 {
		t.Errorf("TotalHistory has %d days, want 30", len(data.History.TotalHistory))
	}

	// Verify total is sum of providers for each day
	for i := 0; i < 30; i++ {
		var sum float64
		for _, provHistory := range data.History.ProviderHistory {
			sum += provHistory[i].SpendUSD
		}
		totalDay := data.History.TotalHistory[i].SpendUSD
		// Allow small floating point variance
		diff := sum - totalDay
		if diff < -0.01 || diff > 0.01 {
			t.Errorf("Day %d: total %f != sum %f", i, totalDay, sum)
		}
	}
}

func TestGetSpendValues(t *testing.T) {
	history := MockDailySpend(10, 100.0)
	values := collectors.GetSpendValues(history)

	if len(values) != 10 {
		t.Errorf("Expected 10 values, got %d", len(values))
	}

	for i, v := range values {
		if v != history[i].SpendUSD {
			t.Errorf("Value %d mismatch: %f vs %f", i, v, history[i].SpendUSD)
		}
	}
}

func TestNodeHighUtilizationDetection(t *testing.T) {
	configs := DefaultTailscaleNodes()

	// Find the node with high CPU (petting-zoo-mini has 85% CPU)
	var highCPUNode *TailscaleNodeConfig
	for i := range configs {
		if configs[i].CPUPercent != nil && *configs[i].CPUPercent >= 80 {
			highCPUNode = &configs[i]
			break
		}
	}

	if highCPUNode == nil {
		t.Skip("No high-CPU node in default configs")
	}

	node := MockTailscaleNode(*highCPUNode)
	if !node.HasHighUtilization() {
		t.Error("Node with 85% CPU should report high utilization")
	}
}
