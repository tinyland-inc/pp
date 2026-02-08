// Package mocks provides mock data generators for testing prompt-pulse display widgets.
// These generators produce realistic data structures for multi-account scenarios,
// billing history, and infrastructure status without requiring actual API calls.
package mocks

import (
	"math/rand"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/widgets"
)

// AccountType specifies the type of Claude account to generate.
type AccountType string

const (
	AccountTypeSubscription AccountType = "subscription"
	AccountTypeAPI          AccountType = "api"
)

// ClaudeAccountConfig configures generation of a mock Claude account.
type ClaudeAccountConfig struct {
	// Name is the account label (e.g., "personal", "work").
	Name string
	// Type is "subscription" or "api".
	Type AccountType
	// Tier is the subscription or API tier.
	// Subscription: "pro", "max_5x", "max_20x"
	// API: "tier_1", "tier_2", "tier_3", "tier_4"
	Tier string
	// Status is the account state: "ok", "auth_failed", "rate_limited", "disabled".
	Status string
	// FiveHourUtil is the 5-hour utilization percentage (0-100).
	FiveHourUtil float64
	// SevenDayUtil is the 7-day utilization percentage (0-100).
	SevenDayUtil float64
	// ExtraUsageEnabled enables overuse credits.
	ExtraUsageEnabled bool
	// ExtraUsageUtil is the extra usage utilization percentage (0-100).
	ExtraUsageUtil float64
	// ExtraUsageLimit is the monthly limit in cents.
	ExtraUsageLimit int
	// APIRequestsUsed is the number of API requests used (API type only).
	APIRequestsUsed int
	// APIRequestsLimit is the API requests limit (API type only).
	APIRequestsLimit int
	// APITokensUsed is the number of API tokens used (API type only).
	APITokensUsed int
	// APITokensLimit is the API tokens limit (API type only).
	APITokensLimit int
}

// DefaultSubscriptionConfig returns default config for a subscription account.
func DefaultSubscriptionConfig(name string) ClaudeAccountConfig {
	return ClaudeAccountConfig{
		Name:              name,
		Type:              AccountTypeSubscription,
		Tier:              "pro",
		Status:            "ok",
		FiveHourUtil:      50.0,
		SevenDayUtil:      30.0,
		ExtraUsageEnabled: false,
	}
}

// DefaultAPIConfig returns default config for an API account.
func DefaultAPIConfig(name string) ClaudeAccountConfig {
	return ClaudeAccountConfig{
		Name:             name,
		Type:             AccountTypeAPI,
		Tier:             "tier_1",
		Status:           "ok",
		APIRequestsUsed:  500,
		APIRequestsLimit: 1000,
		APITokensUsed:    50000,
		APITokensLimit:   100000,
	}
}

// MockClaudeAccount generates a single Claude account from configuration.
func MockClaudeAccount(cfg ClaudeAccountConfig) collectors.ClaudeAccountUsage {
	acct := collectors.ClaudeAccountUsage{
		Name:   cfg.Name,
		Type:   string(cfg.Type),
		Tier:   cfg.Tier,
		Status: cfg.Status,
	}

	now := time.Now()

	if cfg.Type == AccountTypeSubscription {
		// Generate 5-hour usage period.
		acct.FiveHour = &collectors.UsagePeriod{
			Utilization: cfg.FiveHourUtil,
			ResetsAt:    now.Add(time.Duration(randInt(300)+60) * time.Minute),
		}

		// Generate 7-day usage period.
		acct.SevenDay = &collectors.UsagePeriod{
			Utilization: cfg.SevenDayUtil,
			ResetsAt:    now.Add(time.Duration(randInt(7*24)+24) * time.Hour),
		}

		// Generate extra usage if enabled.
		if cfg.ExtraUsageEnabled {
			limit := cfg.ExtraUsageLimit
			if limit <= 0 {
				limit = 10000 // $100.00 default
			}
			usedCents := float64(limit) * (cfg.ExtraUsageUtil / 100.0)
			acct.ExtraUsage = &collectors.ExtraUsage{
				Enabled:      true,
				MonthlyLimit: limit,
				UsedCredits:  usedCents,
				Utilization:  cfg.ExtraUsageUtil,
			}
		}
	} else if cfg.Type == AccountTypeAPI {
		// Generate rate limits.
		requestsLimit := cfg.APIRequestsLimit
		if requestsLimit <= 0 {
			requestsLimit = 1000
		}
		tokensLimit := cfg.APITokensLimit
		if tokensLimit <= 0 {
			tokensLimit = 100000
		}

		acct.RateLimits = &collectors.APIRateLimits{
			RequestsLimit:     requestsLimit,
			RequestsRemaining: requestsLimit - cfg.APIRequestsUsed,
			RequestsReset:     now.Add(time.Duration(randInt(60)+1) * time.Minute),
			TokensLimit:       tokensLimit,
			TokensRemaining:   tokensLimit - cfg.APITokensUsed,
			TokensReset:       now.Add(time.Duration(randInt(60)+1) * time.Minute),
		}
	}

	return acct
}

// MockClaudeAccounts generates multiple Claude accounts with realistic data.
// It creates a mix of subscription and API accounts with varying utilization levels.
func MockClaudeAccounts(count int) []collectors.ClaudeAccountUsage {
	if count <= 0 {
		return nil
	}

	accounts := make([]collectors.ClaudeAccountUsage, count)

	// Account name templates.
	names := []string{"personal", "work", "research", "dev", "prod"}
	tiers := []string{"pro", "max_5x", "max_20x"}
	apiTiers := []string{"tier_1", "tier_2", "tier_3", "tier_4"}

	for i := 0; i < count; i++ {
		name := names[i%len(names)]
		if i >= len(names) {
			name = name + "-" + intToStr(i/len(names)+1)
		}

		// Alternate between subscription and API accounts.
		if i%2 == 0 {
			// Subscription account.
			cfg := DefaultSubscriptionConfig(name)
			cfg.Tier = tiers[randInt(len(tiers))]
			cfg.FiveHourUtil = float64(randInt(100))
			cfg.SevenDayUtil = float64(randInt(100))

			// Add extra usage for some accounts.
			if randFloat32() < 0.3 {
				cfg.ExtraUsageEnabled = true
				cfg.ExtraUsageUtil = float64(randInt(80))
				cfg.ExtraUsageLimit = 5000 + randInt(20000)
			}

			accounts[i] = MockClaudeAccount(cfg)
		} else {
			// API account.
			cfg := DefaultAPIConfig(name)
			cfg.Tier = apiTiers[randInt(len(apiTiers))]
			cfg.APIRequestsLimit = 500 + randInt(4500)
			cfg.APIRequestsUsed = randInt(cfg.APIRequestsLimit)
			cfg.APITokensLimit = 50000 + randInt(200000)
			cfg.APITokensUsed = randInt(cfg.APITokensLimit)

			accounts[i] = MockClaudeAccount(cfg)
		}
	}

	return accounts
}

// MockClaudeUsage generates a complete ClaudeUsage struct with the specified number of accounts.
func MockClaudeUsage(accountCount int) *collectors.ClaudeUsage {
	return &collectors.ClaudeUsage{
		Accounts: MockClaudeAccounts(accountCount),
	}
}

// MockClaudeUsageWithError generates a ClaudeUsage with one account in error state.
func MockClaudeUsageWithError(totalAccounts int, errorIndex int) *collectors.ClaudeUsage {
	accounts := MockClaudeAccounts(totalAccounts)
	if errorIndex >= 0 && errorIndex < len(accounts) {
		accounts[errorIndex].Status = "auth_failed"
	}
	return &collectors.ClaudeUsage{Accounts: accounts}
}

// MockClaudeUsageRateLimited generates a ClaudeUsage with one account rate limited.
func MockClaudeUsageRateLimited(totalAccounts int, rateLimitedIndex int) *collectors.ClaudeUsage {
	accounts := MockClaudeAccounts(totalAccounts)
	if rateLimitedIndex >= 0 && rateLimitedIndex < len(accounts) {
		accounts[rateLimitedIndex].Status = "rate_limited"
	}
	return &collectors.ClaudeUsage{Accounts: accounts}
}

// ========== Billing Mock Data ==========

// ProviderConfig configures generation of a mock provider's billing data.
type ProviderConfig struct {
	// Provider identifies the cloud service.
	Provider string
	// AccountName is a human-readable label.
	AccountName string
	// CurrentSpend is the current month spend in USD.
	CurrentSpend float64
	// PreviousMonth is last month's total spend in USD, if available.
	PreviousMonth *float64
	// Forecast is the projected end-of-month spend in USD.
	Forecast *float64
	// Budget is the monthly budget in USD.
	Budget *float64
	// HistoryDays is the number of days of history to generate.
	HistoryDays int
	// Status is the data freshness: "ok", "error", "stale".
	Status string
}

// DefaultProviderConfigs returns default configurations for the 4 main providers.
func DefaultProviderConfigs() []ProviderConfig {
	forecast150 := 150.0
	forecast80 := 80.0
	forecast45 := 45.0
	forecast25 := 25.0

	budget200 := 200.0
	budget100 := 100.0
	budget50 := 50.0
	budget30 := 30.0

	// Previous month values: some higher (cost decreased), some lower (cost increased).
	prevCivo := 110.00       // current 125.50 is higher -> cost increase
	prevDO := 72.00          // current 65.25 is lower -> cost decrease
	prevAWS := 38.50         // current 38.75 is nearly the same -> flat
	prevDreamhost := 24.99   // current 19.99 is lower -> cost decrease

	return []ProviderConfig{
		{
			Provider:      "civo",
			AccountName:   "tinyland-prod",
			CurrentSpend:  125.50,
			PreviousMonth: &prevCivo,
			Forecast:      &forecast150,
			Budget:        &budget200,
			HistoryDays:   30,
			Status:        "ok",
		},
		{
			Provider:      "digitalocean",
			AccountName:   "tinyland-do",
			CurrentSpend:  65.25,
			PreviousMonth: &prevDO,
			Forecast:      &forecast80,
			Budget:        &budget100,
			HistoryDays:   30,
			Status:        "ok",
		},
		{
			Provider:      "aws",
			AccountName:   "tinyland-aws",
			CurrentSpend:  38.75,
			PreviousMonth: &prevAWS,
			Forecast:      &forecast45,
			Budget:        &budget50,
			HistoryDays:   30,
			Status:        "ok",
		},
		{
			Provider:      "dreamhost",
			AccountName:   "tinyland-hosting",
			CurrentSpend:  19.99,
			PreviousMonth: &prevDreamhost,
			Forecast:      &forecast25,
			Budget:        &budget30,
			HistoryDays:   30,
			Status:        "ok",
		},
	}
}

// MockProviderBilling generates a single provider's billing data.
func MockProviderBilling(cfg ProviderConfig) collectors.ProviderBilling {
	now := time.Now()

	billing := collectors.ProviderBilling{
		Provider:    cfg.Provider,
		AccountName: cfg.AccountName,
		Status:      cfg.Status,
		FetchedAt:   now,
		CurrentMonth: collectors.MonthCost{
			SpendUSD:    cfg.CurrentSpend,
			ForecastUSD: cfg.Forecast,
			BudgetUSD:   cfg.Budget,
			StartDate:   now.Format("2006-01-01"),
			EndDate:     time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02"),
		},
		DashboardURL: "https://" + cfg.Provider + ".com/billing",
	}

	// Use explicit PreviousMonth if provided, otherwise generate one.
	if cfg.PreviousMonth != nil {
		billing.PreviousMonth = cfg.PreviousMonth
	} else if cfg.CurrentSpend > 0 {
		prev := cfg.CurrentSpend * (0.8 + randFloat()*0.4) // 80-120% of current
		billing.PreviousMonth = &prev
	}

	return billing
}

// MockDailySpend generates daily spend history for sparkline rendering.
func MockDailySpend(days int, baseSpend float64) []collectors.DailySpend {
	if days <= 0 {
		return nil
	}

	history := make([]collectors.DailySpend, days)
	now := time.Now()

	// Generate with some variance and trend.
	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -days+i+1)
		// Add daily variance (70-130% of base daily rate).
		dailyRate := baseSpend / float64(days)
		variance := dailyRate * (0.7 + randFloat()*0.6)

		// Add slight upward trend.
		trend := float64(i) / float64(days) * dailyRate * 0.2

		history[i] = collectors.DailySpend{
			Date:     date.Format("2006-01-02"),
			SpendUSD: variance + trend,
		}
	}

	return history
}

// MockBillingHistory generates billing history for all providers.
func MockBillingHistory(providers []collectors.ProviderBilling, days int) *collectors.BillingHistory {
	if len(providers) == 0 || days <= 0 {
		return nil
	}

	history := &collectors.BillingHistory{
		ProviderHistory: make(map[string][]collectors.DailySpend),
		LastUpdated:     time.Now(),
	}

	// Generate per-provider history.
	for _, p := range providers {
		dailyHistory := MockDailySpend(days, p.CurrentMonth.SpendUSD)
		history.ProviderHistory[p.Provider] = dailyHistory
	}

	// Calculate total history.
	history.TotalHistory = make([]collectors.DailySpend, days)
	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -days+i+1).Format("2006-01-02")
		var total float64
		for _, providerHistory := range history.ProviderHistory {
			if i < len(providerHistory) {
				total += providerHistory[i].SpendUSD
			}
		}
		history.TotalHistory[i] = collectors.DailySpend{
			Date:     date,
			SpendUSD: total,
		}
	}

	return history
}

// MockBillingData generates complete billing data with all 4 providers and history.
func MockBillingData() *collectors.BillingData {
	configs := DefaultProviderConfigs()

	providers := make([]collectors.ProviderBilling, len(configs))
	var totalCurrent float64
	var totalForecast float64
	var totalBudget float64
	hasForecast := false
	hasBudget := false

	for i, cfg := range configs {
		providers[i] = MockProviderBilling(cfg)
		totalCurrent += cfg.CurrentSpend
		if cfg.Forecast != nil {
			totalForecast += *cfg.Forecast
			hasForecast = true
		}
		if cfg.Budget != nil {
			totalBudget += *cfg.Budget
			hasBudget = true
		}
	}

	summary := collectors.BillingSummary{
		CurrentMonthUSD: totalCurrent,
	}
	if hasForecast {
		summary.ForecastUSD = &totalForecast
	}
	if hasBudget {
		summary.BudgetUSD = &totalBudget
	}

	return &collectors.BillingData{
		Providers: providers,
		Total:     summary,
		History:   MockBillingHistory(providers, 30),
	}
}

// MockBillingDataEmpty generates empty billing data.
func MockBillingDataEmpty() *collectors.BillingData {
	return &collectors.BillingData{
		Providers: []collectors.ProviderBilling{},
		Total:     collectors.BillingSummary{},
	}
}

// MockBillingDataWithError generates billing data with one provider in error state.
func MockBillingDataWithError(errorProvider string) *collectors.BillingData {
	data := MockBillingData()
	for i := range data.Providers {
		if data.Providers[i].Provider == errorProvider {
			data.Providers[i].Status = "error"
			break
		}
	}
	return data
}

// ========== Infrastructure Mock Data ==========

// TailscaleNodeConfig configures generation of a mock Tailscale node.
type TailscaleNodeConfig struct {
	Name        string
	Hostname    string
	IP          string
	OS          string
	Online      bool
	Tags        []string
	CPUPercent  *float64
	RAMPercent  *float64
	DiskPercent *float64
}

// DefaultTailscaleNodes returns default Tailscale node configurations.
func DefaultTailscaleNodes() []TailscaleNodeConfig {
	cpu1, ram1, disk1 := 25.0, 45.0, 60.0
	cpu2, ram2, disk2 := 85.0, 70.0, 55.0 // High CPU
	cpu3, ram3, disk3 := 15.0, 30.0, 40.0
	cpu4, ram4, disk4 := 50.0, 55.0, 90.0 // High disk

	return []TailscaleNodeConfig{
		{
			Name:        "honey",
			Hostname:    "honey",
			IP:          "100.64.0.1",
			OS:          "linux",
			Online:      true,
			Tags:        []string{"tag:servers"},
			CPUPercent:  &cpu1,
			RAMPercent:  &ram1,
			DiskPercent: &disk1,
		},
		{
			Name:        "petting-zoo-mini",
			Hostname:    "petting-zoo-mini",
			IP:          "100.64.0.2",
			OS:          "darwin",
			Online:      true,
			Tags:        []string{"tag:workstations"},
			CPUPercent:  &cpu2,
			RAMPercent:  &ram2,
			DiskPercent: &disk2,
		},
		{
			Name:        "yoga",
			Hostname:    "yoga",
			IP:          "100.64.0.3",
			OS:          "linux",
			Online:      true,
			Tags:        []string{"tag:servers"},
			CPUPercent:  &cpu3,
			RAMPercent:  &ram3,
			DiskPercent: &disk3,
		},
		{
			Name:        "xoxd-bates",
			Hostname:    "xoxd-bates",
			IP:          "100.64.0.4",
			OS:          "linux",
			Online:      false, // Offline
			Tags:        []string{"tag:workstations"},
			CPUPercent:  &cpu4,
			RAMPercent:  &ram4,
			DiskPercent: &disk4,
		},
	}
}

// MockTailscaleNode generates a single Tailscale node.
func MockTailscaleNode(cfg TailscaleNodeConfig) collectors.TailscaleNode {
	lastSeen := time.Now()
	if !cfg.Online {
		lastSeen = lastSeen.Add(-time.Duration(randInt(24)+1) * time.Hour)
	}

	return collectors.TailscaleNode{
		Name:         cfg.Name,
		Hostname:     cfg.Hostname,
		IP:           cfg.IP,
		OS:           cfg.OS,
		Online:       cfg.Online,
		LastSeen:     lastSeen,
		Tags:         cfg.Tags,
		DashboardURL: "https://login.tailscale.com/admin/machines/" + cfg.Name,
		CPUPercent:   cfg.CPUPercent,
		RAMPercent:   cfg.RAMPercent,
		DiskPercent:  cfg.DiskPercent,
	}
}

// MockTailscaleStatus generates complete Tailscale mesh status.
func MockTailscaleStatus() *collectors.TailscaleStatus {
	configs := DefaultTailscaleNodes()
	nodes := make([]collectors.TailscaleNode, len(configs))

	var onlineCount int
	for i, cfg := range configs {
		nodes[i] = MockTailscaleNode(cfg)
		if cfg.Online {
			onlineCount++
		}
	}

	return &collectors.TailscaleStatus{
		Tailnet:     "tinyland.ts.net",
		OnlineCount: onlineCount,
		TotalCount:  len(nodes),
		Nodes:       nodes,
	}
}

// KubernetesClusterConfig configures generation of a mock K8s cluster.
type KubernetesClusterConfig struct {
	Name       string
	Platform   string
	Status     string
	TotalNodes int
	ReadyNodes int
	Nodes      []KubernetesNodeConfig
}

// KubernetesNodeConfig configures generation of a mock K8s node.
type KubernetesNodeConfig struct {
	Name       string
	Status     string
	CPUPercent float64
	MemPercent float64
	PodCount   int
	MaxPods    int
}

// DefaultKubernetesClusters returns default Kubernetes cluster configurations.
func DefaultKubernetesClusters() []KubernetesClusterConfig {
	return []KubernetesClusterConfig{
		{
			Name:       "bitter-darkness",
			Platform:   "civo",
			Status:     "healthy",
			TotalNodes: 3,
			ReadyNodes: 3,
			Nodes: []KubernetesNodeConfig{
				{Name: "bitter-darkness-worker-0", Status: "Ready", CPUPercent: 45, MemPercent: 60, PodCount: 12, MaxPods: 110},
				{Name: "bitter-darkness-worker-1", Status: "Ready", CPUPercent: 55, MemPercent: 70, PodCount: 15, MaxPods: 110},
				{Name: "bitter-darkness-worker-2", Status: "Ready", CPUPercent: 35, MemPercent: 50, PodCount: 8, MaxPods: 110},
			},
		},
		{
			Name:       "local-k3s",
			Platform:   "k3s",
			Status:     "degraded",
			TotalNodes: 2,
			ReadyNodes: 1,
			Nodes: []KubernetesNodeConfig{
				{Name: "local-k3s-master", Status: "Ready", CPUPercent: 30, MemPercent: 45, PodCount: 6, MaxPods: 50},
				{Name: "local-k3s-worker", Status: "NotReady", CPUPercent: 0, MemPercent: 0, PodCount: 0, MaxPods: 50},
			},
		},
	}
}

// MockKubernetesNode generates a single Kubernetes node.
func MockKubernetesNode(cfg KubernetesNodeConfig) collectors.KubernetesNode {
	return collectors.KubernetesNode{
		Name:       cfg.Name,
		Status:     cfg.Status,
		CPUPercent: cfg.CPUPercent,
		MemPercent: cfg.MemPercent,
		PodCount:   cfg.PodCount,
		MaxPods:    cfg.MaxPods,
	}
}

// MockKubernetesCluster generates a single Kubernetes cluster.
func MockKubernetesCluster(cfg KubernetesClusterConfig) collectors.KubernetesCluster {
	nodes := make([]collectors.KubernetesNode, len(cfg.Nodes))
	for i, nodeCfg := range cfg.Nodes {
		nodes[i] = MockKubernetesNode(nodeCfg)
	}

	return collectors.KubernetesCluster{
		Name:         cfg.Name,
		Platform:     cfg.Platform,
		Status:       cfg.Status,
		APIEndpoint:  "https://" + cfg.Name + ".k8s.local:6443",
		DashboardURL: "https://" + cfg.Platform + ".com/kubernetes/" + cfg.Name,
		Nodes:        nodes,
		TotalNodes:   cfg.TotalNodes,
		ReadyNodes:   cfg.ReadyNodes,
	}
}

// MockInfraStatus generates complete infrastructure status with Tailscale and Kubernetes.
func MockInfraStatus() *collectors.InfraStatus {
	configs := DefaultKubernetesClusters()
	clusters := make([]collectors.KubernetesCluster, len(configs))
	for i, cfg := range configs {
		clusters[i] = MockKubernetesCluster(cfg)
	}

	return &collectors.InfraStatus{
		Tailscale:  MockTailscaleStatus(),
		Kubernetes: clusters,
	}
}

// MockInfraStatusEmpty generates empty infrastructure status.
func MockInfraStatusEmpty() *collectors.InfraStatus {
	return &collectors.InfraStatus{}
}

// MockInfraStatusTailscaleOnly generates infrastructure status with only Tailscale.
func MockInfraStatusTailscaleOnly() *collectors.InfraStatus {
	return &collectors.InfraStatus{
		Tailscale: MockTailscaleStatus(),
	}
}

// MockInfraStatusKubernetesOnly generates infrastructure status with only Kubernetes.
func MockInfraStatusKubernetesOnly() *collectors.InfraStatus {
	configs := DefaultKubernetesClusters()
	clusters := make([]collectors.KubernetesCluster, len(configs))
	for i, cfg := range configs {
		clusters[i] = MockKubernetesCluster(cfg)
	}

	return &collectors.InfraStatus{
		Kubernetes: clusters,
	}
}

// ========== Widget Panel Data ==========

// MockBillingPanelData generates BillingPanelData for widget testing.
func MockBillingPanelData() widgets.BillingPanelData {
	billingData := MockBillingData()

	providers := make([]widgets.ProviderSpend, len(billingData.Providers))
	for i, p := range billingData.Providers {
		var history []float64
		if billingData.History != nil {
			if provHistory, ok := billingData.History.ProviderHistory[p.Provider]; ok {
				history = collectors.GetSpendValues(provHistory)
			}
		}

		providers[i] = widgets.ProviderSpend{
			Name:          p.Provider,
			Current:       p.CurrentMonth.SpendUSD,
			Forecast:      p.CurrentMonth.ForecastUSD,
			Budget:        p.CurrentMonth.BudgetUSD,
			PreviousMonth: p.PreviousMonth,
			History:       history,
			Status:        p.Status,
		}
	}

	return widgets.BillingPanelData{
		Providers:     providers,
		TotalCurrent:  billingData.Total.CurrentMonthUSD,
		TotalForecast: billingData.Total.ForecastUSD,
		TotalBudget:   billingData.Total.BudgetUSD,
		FetchedAt:     time.Now(),
	}
}

// ========== Edge Case Generators ==========

// MockClaudeUsageEmpty returns an empty ClaudeUsage.
func MockClaudeUsageEmpty() *collectors.ClaudeUsage {
	return &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{},
	}
}

// MockClaudeUsageNil returns nil.
func MockClaudeUsageNil() *collectors.ClaudeUsage {
	return nil
}

// MockClaudeUsageAllErrors generates usage where all accounts have errors.
func MockClaudeUsageAllErrors(count int) *collectors.ClaudeUsage {
	accounts := MockClaudeAccounts(count)
	statuses := []string{"auth_failed", "rate_limited", "disabled"}
	for i := range accounts {
		accounts[i].Status = statuses[i%len(statuses)]
	}
	return &collectors.ClaudeUsage{Accounts: accounts}
}

// MockClaudeUsageHighUtilization generates usage with all accounts at high utilization.
func MockClaudeUsageHighUtilization(count int) *collectors.ClaudeUsage {
	accounts := MockClaudeAccounts(count)
	for i := range accounts {
		if accounts[i].FiveHour != nil {
			accounts[i].FiveHour.Utilization = 90 + float64(randInt(10))
		}
		if accounts[i].SevenDay != nil {
			accounts[i].SevenDay.Utilization = 85 + float64(randInt(15))
		}
		if accounts[i].RateLimits != nil {
			// Set 90%+ usage
			accounts[i].RateLimits.RequestsRemaining = accounts[i].RateLimits.RequestsLimit / 10
			accounts[i].RateLimits.TokensRemaining = accounts[i].RateLimits.TokensLimit / 10
		}
	}
	return &collectors.ClaudeUsage{Accounts: accounts}
}

// MockBillingDataOverBudget generates billing data that exceeds budget.
func MockBillingDataOverBudget() *collectors.BillingData {
	data := MockBillingData()
	// Increase current spend to exceed budget
	for i := range data.Providers {
		if data.Providers[i].CurrentMonth.BudgetUSD != nil {
			data.Providers[i].CurrentMonth.SpendUSD = *data.Providers[i].CurrentMonth.BudgetUSD * 1.2
		}
	}
	// Recalculate totals
	var totalCurrent float64
	for _, p := range data.Providers {
		totalCurrent += p.CurrentMonth.SpendUSD
	}
	data.Total.CurrentMonthUSD = totalCurrent
	return data
}

// MockInfraStatusAllOffline generates infrastructure status with all nodes offline.
func MockInfraStatusAllOffline() *collectors.InfraStatus {
	ts := MockTailscaleStatus()
	for i := range ts.Nodes {
		ts.Nodes[i].Online = false
	}
	ts.OnlineCount = 0

	k8s := DefaultKubernetesClusters()
	clusters := make([]collectors.KubernetesCluster, len(k8s))
	for i, cfg := range k8s {
		cfg.Status = "offline"
		cfg.ReadyNodes = 0
		for j := range cfg.Nodes {
			cfg.Nodes[j].Status = "NotReady"
		}
		clusters[i] = MockKubernetesCluster(cfg)
	}

	return &collectors.InfraStatus{
		Tailscale:  ts,
		Kubernetes: clusters,
	}
}

// ========== Utility Functions ==========

// intToStr converts an integer to a string without importing strconv.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// Reverse.
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// rng is the random source used by mock generators.
// Can be overridden for reproducible tests.
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// SeedRandom creates a new random source with the given seed for reproducible tests.
func SeedRandom(seed int64) {
	rng = rand.New(rand.NewSource(seed))
}

// randInt returns a non-negative pseudo-random int from our local source.
func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	return rng.Intn(n)
}

// randFloat returns a pseudo-random float64 in [0.0, 1.0).
func randFloat() float64 {
	return rng.Float64()
}

// randFloat32 returns a pseudo-random float32 in [0.0, 1.0).
func randFloat32() float32 {
	return rng.Float32()
}

// ========== Fastfetch Mock Data ==========

// MockFastfetchData generates realistic system information data.
func MockFastfetchData() *collectors.FastfetchData {
	return &collectors.FastfetchData{
		OS: collectors.FastfetchModule{
			Type:   "OS",
			Key:    "OS",
			Result: "Rocky Linux 10.1 (Blue Onyx) x86_64",
		},
		Host: collectors.FastfetchModule{
			Type:   "Host",
			Key:    "Host",
			Result: "Lenovo ThinkPad X1 Yoga Gen 3",
		},
		Kernel: collectors.FastfetchModule{
			Type:   "Kernel",
			Key:    "Kernel",
			Result: "6.12.0-124.29.1.el10_1.x86_64",
		},
		Uptime: collectors.FastfetchModule{
			Type:   "Uptime",
			Key:    "Uptime",
			Result: "3 days, 14 hours, 22 mins",
		},
		CPU: collectors.FastfetchModule{
			Type:   "CPU",
			Key:    "CPU",
			Result: "Intel i7-8550U (8) @ 4.00 GHz",
		},
		GPU: collectors.FastfetchModule{
			Type:   "GPU",
			Key:    "GPU",
			Result: "Intel UHD Graphics 620",
		},
		Memory: collectors.FastfetchModule{
			Type:   "Memory",
			Key:    "Memory",
			Result: "5.2 GiB / 15.4 GiB (34%)",
		},
		Disk: collectors.FastfetchModule{
			Type:   "Disk",
			Key:    "Disk",
			Result: "98.3 GiB / 229.6 GiB (43%)",
		},
		Packages: collectors.FastfetchModule{
			Type:   "Packages",
			Key:    "Packages",
			Result: "1247 (rpm), 892 (nix)",
		},
		Shell: collectors.FastfetchModule{
			Type:   "Shell",
			Key:    "Shell",
			Result: "bash 5.2.26",
		},
		Terminal: collectors.FastfetchModule{
			Type:   "Terminal",
			Key:    "Terminal",
			Result: "Alacritty 0.15.1",
		},
		LocalIP: collectors.FastfetchModule{
			Type:   "LocalIP",
			Key:    "Local IP",
			Result: "100.64.0.3 (tailscale0)",
		},
	}
}

// MockFastfetchDataEmpty generates empty fastfetch data.
func MockFastfetchDataEmpty() *collectors.FastfetchData {
	return &collectors.FastfetchData{}
}

// ========== SysMetrics Mock Data ==========

// MockSysMetricsData generates realistic system metrics with 60-point history.
func MockSysMetricsData() *collectors.SysMetricsData {
	cpuHistory := make([]float64, 60)
	ramHistory := make([]float64, 60)
	diskHistory := make([]float64, 60)

	for i := 0; i < 60; i++ {
		// CPU: oscillates between 15-65% with some variance.
		cpuHistory[i] = 25.0 + float64(randInt(40))
		// RAM: slowly trends upward from ~40% to ~60%.
		ramHistory[i] = 40.0 + float64(i)*0.33 + float64(randInt(5))
		// Disk: stable around 43% with minimal drift.
		diskHistory[i] = 42.0 + float64(randInt(3))
	}

	return &collectors.SysMetricsData{
		CPU:         cpuHistory[59],
		RAM:         ramHistory[59],
		Disk:        diskHistory[59],
		LoadAvg1:    1.25 + randFloat()*0.5,
		LoadAvg5:    0.98 + randFloat()*0.3,
		LoadAvg15:   0.75 + randFloat()*0.2,
		CPUHistory:  cpuHistory,
		RAMHistory:  ramHistory,
		DiskHistory: diskHistory,
	}
}

// MockSysMetricsDataEmpty generates empty system metrics data.
func MockSysMetricsDataEmpty() *collectors.SysMetricsData {
	return &collectors.SysMetricsData{}
}

// MockSysMetricsDataHighUtilization generates system metrics with high utilization.
func MockSysMetricsDataHighUtilization() *collectors.SysMetricsData {
	cpuHistory := make([]float64, 60)
	ramHistory := make([]float64, 60)
	diskHistory := make([]float64, 60)

	for i := 0; i < 60; i++ {
		cpuHistory[i] = 85.0 + float64(randInt(15))
		ramHistory[i] = 90.0 + float64(randInt(10))
		diskHistory[i] = 88.0 + float64(randInt(12))
	}

	return &collectors.SysMetricsData{
		CPU:         95.2,
		RAM:         92.7,
		Disk:        91.3,
		LoadAvg1:    8.50,
		LoadAvg5:    7.25,
		LoadAvg15:   6.80,
		CPUHistory:  cpuHistory,
		RAMHistory:  ramHistory,
		DiskHistory: diskHistory,
	}
}
