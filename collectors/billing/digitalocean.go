// Package billing provides cloud provider billing collectors for prompt-pulse.
// Each collector fetches spend data from a provider's API and returns a
// canonical ProviderBilling struct for dashboard rendering.
package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// doBalanceEndpoint is the DigitalOcean balance API URL.
	doBalanceEndpoint = "https://api.digitalocean.com/v2/customers/my/balance"

	// doInvoicesEndpoint is the DigitalOcean invoices API URL.
	doInvoicesEndpoint = "https://api.digitalocean.com/v2/customers/my/invoices"

	// doDashboardURL is the DigitalOcean billing dashboard link.
	doDashboardURL = "https://cloud.digitalocean.com/account/billing"

	// doRequestTimeout is the per-client HTTP timeout.
	doRequestTimeout = 10 * time.Second

	// doUserAgent identifies prompt-pulse in request headers.
	doUserAgent = "prompt-pulse/0.1.0"

	// doMaxResponseBytes caps response body reads to prevent unbounded memory use.
	doMaxResponseBytes = 1 << 20 // 1 MiB
)

// doBalanceResponse maps the DigitalOcean /v2/customers/my/balance JSON.
// All monetary amounts are returned as strings by the API.
type doBalanceResponse struct {
	MonthToDateBalance string `json:"month_to_date_balance"`
	AccountBalance     string `json:"account_balance"`
	MonthToDateUsage   string `json:"month_to_date_usage"`
	GeneratedAt        string `json:"generated_at"`
}

// doInvoicesResponse maps the DigitalOcean /v2/customers/my/invoices JSON.
type doInvoicesResponse struct {
	Invoices       []doInvoice `json:"invoices"`
	InvoicePreview *doInvoice  `json:"invoice_preview"`
}

// doInvoice represents a single invoice entry from DigitalOcean.
type doInvoice struct {
	InvoiceUUID   string `json:"invoice_uuid"`
	Amount        string `json:"amount"`
	InvoicePeriod string `json:"invoice_period"` // "YYYY-MM"
	UpdatedAt     string `json:"updated_at"`
}

// DOClient fetches billing data from the DigitalOcean API.
type DOClient struct {
	apiToken   string
	httpClient *http.Client
	baseURL    string
	logger     *slog.Logger
}

// NewDOClient creates a DOClient with a 10-second HTTP timeout.
// If logger is nil, a no-op logger is used.
func NewDOClient(apiToken string, logger *slog.Logger) *DOClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &DOClient{
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: doRequestTimeout,
		},
		logger: logger,
	}
}

// FetchBilling retrieves current billing data from DigitalOcean by calling
// the balance and invoices endpoints. The balance call is required; an
// invoice failure is non-fatal and produces a warning instead of an error.
//
// The returned ProviderBilling includes:
//   - Current month spend from the balance endpoint (month_to_date_usage)
//   - A linear forecast extrapolating current spend to end of month
//   - Previous month's invoice amount if available
func (c *DOClient) FetchBilling(ctx context.Context) (*collectors.ProviderBilling, error) {
	now := time.Now().UTC()

	// Fetch current balance (required).
	balance, err := c.fetchBalance(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching DO balance: %w", err)
	}

	// Parse the month-to-date usage string to float64.
	currentSpend, err := strconv.ParseFloat(balance.MonthToDateUsage, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing month_to_date_usage %q: %w", balance.MonthToDateUsage, err)
	}

	// Calculate billing period boundaries.
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, -1)
	daysInMonth := float64(endOfMonth.Day())

	// Days elapsed is at least 1 to avoid division by zero on the first day.
	daysElapsed := float64(now.Day())
	if daysElapsed < 1 {
		daysElapsed = 1
	}

	// Linear forecast: (currentSpend / daysElapsed) * daysInMonth.
	forecast := (currentSpend / daysElapsed) * daysInMonth

	billing := &collectors.ProviderBilling{
		Provider:     "digitalocean",
		AccountName:  "digitalocean",
		Status:       "ok",
		DashboardURL: doDashboardURL,
		CurrentMonth: collectors.MonthCost{
			SpendUSD:    currentSpend,
			ForecastUSD: &forecast,
			StartDate:   startOfMonth.Format("2006-01-02"),
			EndDate:     endOfMonth.Format("2006-01-02"),
		},
		FetchedAt: now,
	}

	// Fetch invoices (non-fatal).
	prevMonth := c.fetchPreviousMonth(ctx, now)
	if prevMonth != nil {
		billing.PreviousMonth = prevMonth
	}

	return billing, nil
}

// fetchBalance calls the DigitalOcean balance endpoint and returns the
// parsed response. Returns an error on non-200 status codes or JSON
// parse failures.
func (c *DOClient) fetchBalance(ctx context.Context) (*doBalanceResponse, error) {
	url := c.balanceURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating balance request: %w", err)
	}

	c.setHeaders(req)
	c.logger.Debug("fetching DO balance", "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing balance request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, doMaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading balance response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &DOAPIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}

	var balance doBalanceResponse
	if err := json.Unmarshal(body, &balance); err != nil {
		return nil, fmt.Errorf("parsing balance JSON: %w", err)
	}

	return &balance, nil
}

// fetchInvoices calls the DigitalOcean invoices endpoint and returns the
// parsed response. Returns an error on non-200 status codes or JSON
// parse failures.
func (c *DOClient) fetchInvoices(ctx context.Context) (*doInvoicesResponse, error) {
	url := c.invoicesURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating invoices request: %w", err)
	}

	c.setHeaders(req)
	c.logger.Debug("fetching DO invoices", "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing invoices request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, doMaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading invoices response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &DOAPIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}

	var invoices doInvoicesResponse
	if err := json.Unmarshal(body, &invoices); err != nil {
		return nil, fmt.Errorf("parsing invoices JSON: %w", err)
	}

	return &invoices, nil
}

// fetchPreviousMonth attempts to find the previous calendar month's invoice
// amount. Returns nil if the invoice is not found or the fetch fails.
func (c *DOClient) fetchPreviousMonth(ctx context.Context, now time.Time) *float64 {
	invoices, err := c.fetchInvoices(ctx)
	if err != nil {
		c.logger.Warn("failed to fetch invoices, skipping previous month", "error", err)
		return nil
	}

	// Build the expected period string for the previous month (YYYY-MM).
	prevMonthTime := now.AddDate(0, -1, 0)
	expectedPeriod := prevMonthTime.Format("2006-01")

	for _, inv := range invoices.Invoices {
		if inv.InvoicePeriod == expectedPeriod {
			amount, err := strconv.ParseFloat(inv.Amount, 64)
			if err != nil {
				c.logger.Warn("failed to parse invoice amount",
					"period", inv.InvoicePeriod,
					"amount", inv.Amount,
					"error", err,
				)
				return nil
			}
			return &amount
		}
	}

	c.logger.Debug("no invoice found for previous month", "expected_period", expectedPeriod)
	return nil
}

// setHeaders applies authentication and identification headers to a request.
func (c *DOClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", doUserAgent)
}

// balanceURL returns the balance endpoint URL, using baseURL for testing
// or the production URL by default.
func (c *DOClient) balanceURL() string {
	if c.baseURL != "" {
		return c.baseURL + "/v2/customers/my/balance"
	}
	return doBalanceEndpoint
}

// invoicesURL returns the invoices endpoint URL, using baseURL for testing
// or the production URL by default.
func (c *DOClient) invoicesURL() string {
	if c.baseURL != "" {
		return c.baseURL + "/v2/customers/my/invoices"
	}
	return doInvoicesEndpoint
}

// DOAPIError represents a non-200 HTTP response from the DigitalOcean API.
type DOAPIError struct {
	StatusCode int
	Status     string
	Body       string
}

// Error returns a human-readable description of the API error.
func (e *DOAPIError) Error() string {
	return fmt.Sprintf("DigitalOcean API error: %s (body: %s)", e.Status, e.Body)
}
