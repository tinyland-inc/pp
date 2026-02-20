package claude

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Default configuration values.
const (
	DefaultInterval = 5 * time.Minute
)

// Config holds the configuration for the Claude/Anthropic usage collector.
type Config struct {
	// Interval is how often collection runs. Zero uses DefaultInterval.
	Interval time.Duration

	// Accounts is the list of Anthropic accounts to monitor.
	Accounts []AccountConfig
}

// AccountConfig identifies a single Anthropic account.
type AccountConfig struct {
	// Name is a human-readable label (e.g., "personal", "work").
	Name string

	// AdminAPIKey is the Anthropic Admin API key for this account.
	AdminAPIKey string

	// OrganizationID is the Anthropic organization identifier.
	OrganizationID string
}

// UsageReport is the top-level data returned by a single Collect call.
type UsageReport struct {
	Accounts    []AccountUsage `json:"accounts"`
	TotalCostUSD float64       `json:"total_cost_usd"`
	Timestamp   time.Time      `json:"timestamp"`
}

// AccountUsage holds usage data for a single Anthropic account.
type AccountUsage struct {
	Name           string           `json:"name"`
	OrganizationID string           `json:"organization_id"`
	Connected      bool             `json:"connected"`
	Error          string           `json:"error,omitempty"`
	CurrentMonth   MonthUsage       `json:"current_month"`
	PreviousMonth  MonthUsage       `json:"previous_month"`
	Models         []ModelUsage     `json:"models"`
	Workspaces     []WorkspaceUsage `json:"workspaces"`
}

// MonthUsage aggregates token counts and cost for a calendar month.
type MonthUsage struct {
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	CostUSD             float64 `json:"cost_usd"`
}

// ModelUsage breaks down usage by model within a single month.
type ModelUsage struct {
	Model        string  `json:"model"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// WorkspaceUsage breaks down usage by workspace. Currently populated as a
// placeholder; the Anthropic Admin API may add workspace-level data in the
// future.
type WorkspaceUsage struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// Collector gathers Anthropic API usage and cost data.
type Collector struct {
	client   APIClient
	accounts []AccountConfig
	interval time.Duration

	// nowFunc allows tests to inject a deterministic clock.
	nowFunc func() time.Time

	mu      sync.Mutex
	healthy bool
}

// New creates a new Claude/Anthropic usage collector. If cfg.Interval is zero,
// DefaultInterval is used. If client is nil, a default HTTPClient is created.
func New(cfg Config, client APIClient) *Collector {
	interval := cfg.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	if client == nil {
		client = NewHTTPClient("")
	}
	return &Collector{
		client:   client,
		accounts: cfg.Accounts,
		interval: interval,
		nowFunc:  time.Now,
		healthy:  true,
	}
}

// Name returns the collector identifier.
func (c *Collector) Name() string {
	return "claude"
}

// Interval returns how often this collector should run.
func (c *Collector) Interval() time.Duration {
	return c.interval
}

// Healthy returns whether at least one account connected successfully on the
// last collection cycle.
func (c *Collector) Healthy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.healthy
}

// setHealthy updates the internal healthy flag under the mutex.
func (c *Collector) setHealthy(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.healthy = v
}

// Collect queries all configured accounts and returns a UsageReport. Accounts
// that fail are marked as disconnected; the collector continues to the next.
// The collector is healthy as long as at least one account succeeds.
func (c *Collector) Collect(ctx context.Context) (interface{}, error) {
	if err := ctx.Err(); err != nil {
		c.setHealthy(false)
		return nil, fmt.Errorf("claude collect: %w", err)
	}

	// Auto-discover org IDs for accounts that don't have one set.
	c.resolveOrgIDs(ctx)

	now := c.nowFunc()
	curStart, curEnd := currentMonthRange(now)
	prevStart, prevEnd := previousMonthRange(now)

	report := &UsageReport{
		Accounts:  make([]AccountUsage, 0, len(c.accounts)),
		Timestamp: now,
	}

	anyConnected := false

	for _, acct := range c.accounts {
		if err := ctx.Err(); err != nil {
			c.setHealthy(false)
			return nil, fmt.Errorf("claude collect: %w", err)
		}

		au := c.collectAccount(ctx, acct, curStart, curEnd, prevStart, prevEnd)
		report.Accounts = append(report.Accounts, au)
		if au.Connected {
			anyConnected = true
		}
		report.TotalCostUSD += au.CurrentMonth.CostUSD
	}

	c.setHealthy(anyConnected || len(c.accounts) == 0)
	return report, nil
}

// resolveOrgIDs auto-discovers organization IDs for accounts missing them.
func (c *Collector) resolveOrgIDs(ctx context.Context) {
	for i := range c.accounts {
		if c.accounts[i].OrganizationID != "" || c.accounts[i].AdminAPIKey == "" {
			continue
		}
		if strings.HasPrefix(c.accounts[i].AdminAPIKey, "sk-ant-api") {
			continue
		}
		orgs, err := c.client.GetOrganizations(ctx, c.accounts[i].AdminAPIKey)
		if err != nil {
			continue
		}
		if len(orgs) > 0 {
			c.accounts[i].OrganizationID = orgs[0].ID
			if c.accounts[i].Name == "default" && orgs[0].Name != "" {
				c.accounts[i].Name = orgs[0].Name
			}
		}
	}
}

// collectAccount fetches usage for a single account, returning an
// AccountUsage. Errors are captured in the struct rather than propagated.
func (c *Collector) collectAccount(
	ctx context.Context,
	acct AccountConfig,
	curStart, curEnd, prevStart, prevEnd string,
) AccountUsage {
	au := AccountUsage{
		Name:           acct.Name,
		OrganizationID: acct.OrganizationID,
	}

	// Admin API requires admin keys (sk-ant-admin01-*). Regular API keys
	// (sk-ant-api03-*) cannot access /v1/organizations endpoints.
	if strings.HasPrefix(acct.AdminAPIKey, "sk-ant-api") {
		au.Error = "key is not an admin key (requires sk-ant-admin01-*); get one at console.anthropic.com"
		return au
	}

	// Fetch current month usage.
	curResp, err := c.client.GetUsage(ctx, acct.OrganizationID, acct.AdminAPIKey, curStart, curEnd)
	if err != nil {
		au.Error = err.Error()
		return au
	}

	au.Connected = true
	au.CurrentMonth = aggregateMonth(curResp)
	au.Models = aggregateModels(curResp)

	// Fetch previous month usage (best-effort).
	prevResp, err := c.client.GetUsage(ctx, acct.OrganizationID, acct.AdminAPIKey, prevStart, prevEnd)
	if err == nil {
		au.PreviousMonth = aggregateMonth(prevResp)
	}

	return au
}

// aggregateMonth sums all entries in an API response into a single MonthUsage.
func aggregateMonth(resp *APIUsageResponse) MonthUsage {
	if resp == nil {
		return MonthUsage{}
	}
	var mu MonthUsage
	for _, entry := range resp.Data {
		mu.InputTokens += entry.InputTokens
		mu.OutputTokens += entry.OutputTokens
		mu.CacheCreationTokens += entry.CacheCreationTokens
		mu.CacheReadTokens += entry.CacheReadTokens
		mu.CostUSD += CalculateCost(
			entry.Model,
			entry.InputTokens,
			entry.OutputTokens,
			entry.CacheCreationTokens,
			entry.CacheReadTokens,
		)
	}
	return mu
}

// aggregateModels builds per-model usage summaries from the API response.
func aggregateModels(resp *APIUsageResponse) []ModelUsage {
	if resp == nil {
		return nil
	}

	// Aggregate by model name.
	type modelAcc struct {
		input  int64
		output int64
		cost   float64
	}
	byModel := make(map[string]*modelAcc)
	order := make([]string, 0)

	for _, entry := range resp.Data {
		acc, ok := byModel[entry.Model]
		if !ok {
			acc = &modelAcc{}
			byModel[entry.Model] = acc
			order = append(order, entry.Model)
		}
		acc.input += entry.InputTokens
		acc.output += entry.OutputTokens
		acc.cost += CalculateCost(
			entry.Model,
			entry.InputTokens,
			entry.OutputTokens,
			entry.CacheCreationTokens,
			entry.CacheReadTokens,
		)
	}

	models := make([]ModelUsage, 0, len(byModel))
	for _, name := range order {
		acc := byModel[name]
		models = append(models, ModelUsage{
			Model:        name,
			InputTokens:  acc.input,
			OutputTokens: acc.output,
			CostUSD:      acc.cost,
		})
	}
	return models
}

// currentMonthRange returns the start (1st of month) and end (today) dates
// as YYYY-MM-DD strings for the current month.
func currentMonthRange(now time.Time) (start, end string) {
	year, month, _ := now.Date()
	loc := now.Location()
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	return first.Format("2006-01-02"), now.Format("2006-01-02")
}

// previousMonthRange returns the start and end dates for the previous month.
func previousMonthRange(now time.Time) (start, end string) {
	year, month, _ := now.Date()
	loc := now.Location()
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	prevLast := first.AddDate(0, 0, -1) // last day of previous month
	prevFirst := time.Date(prevLast.Year(), prevLast.Month(), 1, 0, 0, 0, 0, loc)
	return prevFirst.Format("2006-01-02"), prevLast.Format("2006-01-02")
}
