// Package billing provides cloud provider billing collectors for prompt-pulse.
// Each provider client fetches billing data from its respective API and returns
// a normalized ProviderBilling result for aggregation and display.
package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// civoChargesEndpoint is the Civo billing charges API URL.
	civoChargesEndpoint = "https://api.civo.com/v2/charges"

	// civoDashboardURL is the Civo billing dashboard link.
	civoDashboardURL = "https://dashboard.civo.com/billing"

	// civoUserAgent identifies prompt-pulse in request headers.
	civoUserAgent = "prompt-pulse/0.1.0"

	// civoRequestTimeout is the per-request timeout for the HTTP client.
	civoRequestTimeout = 10 * time.Second

	// civoMaxResponseBytes limits the response body size to prevent unbounded reads.
	civoMaxResponseBytes = 1 << 20 // 1 MiB
)

// civoCharge represents a single line item from the Civo charges API.
type civoCharge struct {
	Code             string   `json:"code"`
	Label            string   `json:"label"`
	From             string   `json:"from"`
	To               string   `json:"to"`
	NumHours         int      `json:"num_hours"`
	SizeGB           *float64 `json:"size_gb"`
	UnitPricePerHour float64  `json:"unit_price_per_hour"`
	Total            float64  `json:"total"`
}

// civoWrappedResponse handles the alternative API response format where charges
// are wrapped in a JSON object rather than returned as a bare array.
type civoWrappedResponse struct {
	Charges []civoCharge `json:"charges"`
}

// CivoAPIError represents a non-success HTTP response from the Civo API.
type CivoAPIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *CivoAPIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("civo API error: %s (body: %s)", e.Status, e.Body)
	}
	return fmt.Sprintf("civo API error: %s", e.Status)
}

// CivoClient fetches billing data from the Civo API.
type CivoClient struct {
	apiKey     string
	region     string
	httpClient *http.Client
	baseURL    string
	logger     *slog.Logger
}

// NewCivoClient creates a CivoClient configured with a 10-second timeout.
// If logger is nil, a no-op logger is used.
func NewCivoClient(apiKey, region string, logger *slog.Logger) *CivoClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &CivoClient{
		apiKey: apiKey,
		region: region,
		httpClient: &http.Client{
			Timeout: civoRequestTimeout,
		},
		baseURL: civoChargesEndpoint,
		logger:  logger,
	}
}

// FetchBilling retrieves the current month's billing data from the Civo charges API.
// It calculates the spend by summing all charge totals and produces a linear forecast
// based on the daily spend rate projected over the full month.
//
// The returned ProviderBilling has Provider="civo" and Status set to one of:
//   - "ok" for successful responses
//   - "auth_failed" for authentication errors (401)
//   - "rate_limited" for rate limit errors (429)
//   - "error" for server errors (5xx) or other failures
func (c *CivoClient) FetchBilling(ctx context.Context) (*collectors.ProviderBilling, error) {
	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	charges, err := c.fetchCharges(ctx, startOfMonth, now)
	if err != nil {
		return nil, err
	}

	// Sum all charge totals for current month spend.
	var totalSpend float64
	for _, charge := range charges {
		totalSpend += charge.Total
	}

	// Calculate forecast: (currentSpend / daysElapsed) * daysInMonth.
	// daysElapsed is fractional to account for partial days.
	var forecast *float64
	elapsed := now.Sub(startOfMonth)
	daysElapsed := elapsed.Hours() / 24.0
	if daysElapsed > 0 {
		totalDays := float64(DaysInMonth(now.Year(), now.Month()))
		f := RoundCents((totalSpend / daysElapsed) * totalDays)
		forecast = &f
	}

	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)

	return &collectors.ProviderBilling{
		Provider:     "civo",
		AccountName:  c.region,
		Status:       "ok",
		DashboardURL: civoDashboardURL,
		CurrentMonth: collectors.MonthCost{
			SpendUSD:    RoundCents(totalSpend),
			ForecastUSD: forecast,
			StartDate:   startOfMonth.Format("2006-01-02"),
			EndDate:     endOfMonth.Format("2006-01-02"),
		},
		FetchedAt: now,
	}, nil
}

// FetchPreviousMonth retrieves the previous month's total billing from the Civo
// charges API. Returns the total spend as a pointer, or nil on error.
func (c *CivoClient) FetchPreviousMonth(ctx context.Context) (*float64, error) {
	now := time.Now().UTC()
	startOfPrevMonth := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	endOfPrevMonth := time.Date(now.Year(), now.Month(), 0, 23, 59, 59, 0, time.UTC)

	charges, err := c.fetchCharges(ctx, startOfPrevMonth, endOfPrevMonth)
	if err != nil {
		return nil, err
	}

	var totalSpend float64
	for _, charge := range charges {
		totalSpend += charge.Total
	}

	totalSpend = RoundCents(totalSpend)
	return &totalSpend, nil
}

// fetchCharges calls the Civo charges API for the given date range and returns
// the parsed charge list. It handles both the bare array and wrapped object
// response formats.
func (c *CivoClient) fetchCharges(ctx context.Context, from, to time.Time) ([]civoCharge, error) {
	url := fmt.Sprintf("%s?from=%s&to=%s",
		c.baseURL,
		from.Format("2006-01-02T15:04:05Z"),
		to.Format("2006-01-02T15:04:05Z"),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "bearer "+c.apiKey)
	req.Header.Set("User-Agent", civoUserAgent)
	req.Header.Set("Accept", "application/json")

	c.logger.Debug("fetching Civo charges", "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, civoMaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		// Parse below.

	case resp.StatusCode == http.StatusUnauthorized:
		return nil, &CivoAPIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}

	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, &CivoAPIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}

	case resp.StatusCode >= 500:
		return nil, &CivoAPIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}

	default:
		return nil, &CivoAPIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}

	// Try parsing as a bare JSON array first.
	var charges []civoCharge
	if err := json.Unmarshal(body, &charges); err == nil {
		c.logger.Debug("parsed Civo charges (bare array)", "count", len(charges))
		return charges, nil
	}

	// Fall back to the wrapped {"charges": [...]} format.
	var wrapped civoWrappedResponse
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, fmt.Errorf("parsing Civo charges response: %w", err)
	}

	c.logger.Debug("parsed Civo charges (wrapped format)", "count", len(wrapped.Charges))
	return wrapped.Charges, nil
}

