package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// dreamhostAPIURL is the DreamHost API endpoint.
	dreamhostAPIURL = "https://api.dreamhost.com/"

	// dreamhostDashboardURL is the DreamHost billing panel link.
	dreamhostDashboardURL = "https://panel.dreamhost.com/index.cgi?tree=billing.account"

	// dreamhostRequestTimeout is the per-request timeout.
	dreamhostRequestTimeout = 15 * time.Second

	// dreamhostMaxResponseBytes caps the response body size.
	dreamhostMaxResponseBytes = 1 << 20 // 1 MiB
)

// DreamHostClient provides billing data from the DreamHost API.
// Note: DreamHost's API primarily exposes bandwidth usage rather than
// direct billing amounts. The bandwidth data is summed and presented
// as usage metrics.
//
// Available commands:
//   - account-status: Returns account status and billing cycle dates
//   - account-domain_usage: Returns bandwidth usage per domain
//   - account-list_rewards: Returns rewards/credits balance
type DreamHostClient struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
	baseURL    string // overridable for testing
}

// NewDreamHostClient creates a DreamHostClient with the given API key.
// If logger is nil, a no-op logger is used.
func NewDreamHostClient(apiKey string, logger *slog.Logger) *DreamHostClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &DreamHostClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: dreamhostRequestTimeout,
		},
		logger:  logger,
		baseURL: dreamhostAPIURL,
	}
}

// dreamhostResponse is the generic JSON response wrapper from the DreamHost API.
type dreamhostResponse struct {
	Result string          `json:"result"` // "success" or "error"
	Data   json.RawMessage `json:"data"`
	Reason string          `json:"reason,omitempty"` // error message if result="error"
}

// accountStatusData represents the account-status response data.
type accountStatusData struct {
	AccountStatus     string `json:"account_status"`      // "active", "suspended", etc.
	BillingCycleStart string `json:"billing_cycle_start"` // YYYY-MM-DD
	BillingCycleEnd   string `json:"billing_cycle_end"`   // YYYY-MM-DD
}

// domainUsageData represents a single domain's bandwidth usage.
type domainUsageData struct {
	Domain     string `json:"domain"`
	Type       string `json:"type"` // "http", "ftp", etc.
	BwUsed     string `json:"bw_used"`
	BwQuota    string `json:"bw_quota,omitempty"`
	BwUsedHTTP string `json:"bw_used_http,omitempty"`
	BwUsedFTP  string `json:"bw_used_ftp,omitempty"`
}

// rewardsData represents a rewards/credits entry.
type rewardsData struct {
	Description string `json:"description"`
	Amount      string `json:"amount"` // Dollar amount as string
	ValidUntil  string `json:"valid_until,omitempty"`
}

// FetchBilling retrieves account status, bandwidth usage, and rewards from DreamHost.
// Returns a ProviderBilling with aggregated bandwidth usage as the "spend" metric,
// since DreamHost's API doesn't expose actual billing amounts.
func (c *DreamHostClient) FetchBilling(ctx context.Context) (*collectors.ProviderBilling, error) {
	c.logger.Debug("fetching DreamHost billing data")

	// Default billing period.
	start, end := CurrentMonthRange()

	result := &collectors.ProviderBilling{
		Provider:     "dreamhost",
		AccountName:  "DreamHost",
		Status:       "ok",
		DashboardURL: dreamhostDashboardURL,
		CurrentMonth: collectors.MonthCost{
			StartDate: start,
			EndDate:   end,
		},
		FetchedAt: time.Now(),
	}

	// Fetch account status (gets billing cycle dates).
	statusData, err := c.fetchAccountStatus(ctx)
	if err != nil {
		c.logger.Warn("failed to fetch DreamHost account status", "error", err)
		// Continue with defaults.
	} else if statusData != nil {
		if statusData.BillingCycleStart != "" {
			result.CurrentMonth.StartDate = statusData.BillingCycleStart
		}
		if statusData.BillingCycleEnd != "" {
			result.CurrentMonth.EndDate = statusData.BillingCycleEnd
		}
	}

	// Fetch domain bandwidth usage.
	totalBandwidthGB, err := c.fetchDomainUsage(ctx)
	if err != nil {
		c.logger.Warn("failed to fetch DreamHost domain usage", "error", err)
	} else {
		// DreamHost typically charges $0.08/GB for bandwidth overage.
		// We report the bandwidth as a proxy for usage.
		// Note: This is NOT actual billing, just bandwidth consumption.
		c.logger.Debug("DreamHost bandwidth usage", "total_gb", totalBandwidthGB)
	}

	// Fetch rewards/credits balance.
	creditsUSD, err := c.fetchRewards(ctx)
	if err != nil {
		c.logger.Warn("failed to fetch DreamHost rewards", "error", err)
	} else if creditsUSD > 0 {
		// Show credits as a negative forecast (money owed to user).
		forecast := -creditsUSD
		result.CurrentMonth.ForecastUSD = &forecast
		c.logger.Debug("DreamHost credits", "amount", creditsUSD)
	}

	// DreamHost API doesn't expose actual billing amounts, so SpendUSD remains 0.
	// The dashboard link should be used for actual billing information.
	result.Status = "limited" // Indicate this is not full billing data.

	return result, nil
}

// fetchAccountStatus retrieves the account status and billing cycle dates.
func (c *DreamHostClient) fetchAccountStatus(ctx context.Context) (*accountStatusData, error) {
	body, err := c.doRequest(ctx, "account-status")
	if err != nil {
		return nil, err
	}

	var resp dreamhostResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing account-status response: %w", err)
	}

	if resp.Result != "success" {
		return nil, fmt.Errorf("account-status failed: %s", resp.Reason)
	}

	// account-status returns an array with a single object.
	var statusList []accountStatusData
	if err := json.Unmarshal(resp.Data, &statusList); err != nil {
		return nil, fmt.Errorf("parsing account-status data: %w", err)
	}

	if len(statusList) == 0 {
		return nil, nil
	}

	return &statusList[0], nil
}

// fetchDomainUsage retrieves bandwidth usage for all domains and returns total GB.
func (c *DreamHostClient) fetchDomainUsage(ctx context.Context) (float64, error) {
	body, err := c.doRequest(ctx, "account-domain_usage")
	if err != nil {
		return 0, err
	}

	var resp dreamhostResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("parsing domain_usage response: %w", err)
	}

	if resp.Result != "success" {
		return 0, fmt.Errorf("domain_usage failed: %s", resp.Reason)
	}

	var usageList []domainUsageData
	if err := json.Unmarshal(resp.Data, &usageList); err != nil {
		return 0, fmt.Errorf("parsing domain_usage data: %w", err)
	}

	var totalBytes int64
	for _, usage := range usageList {
		if bytes, err := strconv.ParseInt(strings.TrimSpace(usage.BwUsed), 10, 64); err == nil {
			totalBytes += bytes
		}
	}

	// Convert to GB.
	totalGB := float64(totalBytes) / (1024 * 1024 * 1024)
	return totalGB, nil
}

// fetchRewards retrieves account credits/rewards and returns total USD.
func (c *DreamHostClient) fetchRewards(ctx context.Context) (float64, error) {
	body, err := c.doRequest(ctx, "account-list_rewards")
	if err != nil {
		return 0, err
	}

	var resp dreamhostResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("parsing list_rewards response: %w", err)
	}

	if resp.Result != "success" {
		return 0, fmt.Errorf("list_rewards failed: %s", resp.Reason)
	}

	var rewardsList []rewardsData
	if err := json.Unmarshal(resp.Data, &rewardsList); err != nil {
		return 0, fmt.Errorf("parsing list_rewards data: %w", err)
	}

	var totalCredits float64
	for _, reward := range rewardsList {
		if amount, err := strconv.ParseFloat(strings.TrimSpace(reward.Amount), 64); err == nil {
			totalCredits += amount
		}
	}

	return totalCredits, nil
}

// doRequest executes a DreamHost API request and returns the response body.
func (c *DreamHostClient) doRequest(ctx context.Context, cmd string) ([]byte, error) {
	base := c.baseURL
	if base == "" {
		base = dreamhostAPIURL
	}

	params := url.Values{}
	params.Set("key", c.apiKey)
	params.Set("cmd", cmd)
	params.Set("format", "json")

	reqURL := base + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", "prompt-pulse/0.1.0")

	c.logger.Debug("executing DreamHost API request", "cmd", cmd)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, dreamhostMaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &DHAPIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}

	return body, nil
}

// DHAPIError represents a non-200 HTTP response from the DreamHost API.
type DHAPIError struct {
	StatusCode int
	Status     string
	Body       string
}

// Error returns a human-readable description of the API error.
func (e *DHAPIError) Error() string {
	return fmt.Sprintf("DreamHost API error: %s (body: %s)", e.Status, e.Body)
}
