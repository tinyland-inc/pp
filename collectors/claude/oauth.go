// Package claude provides HTTP clients for fetching usage data from Claude
// subscription and API accounts.
package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// usageEndpoint is the Claude OAuth usage API URL.
	// Note: The correct domain is claude.ai, not api.claude.ai (which returns NXDOMAIN).
	usageEndpoint = "https://claude.ai/api/auth/usage"

	// userAgent identifies prompt-pulse in request headers.
	userAgent = "prompt-pulse/0.1.0"

	// requestTimeout is the per-request timeout for the HTTP client.
	requestTimeout = 10 * time.Second

	// maxResponseBytes limits the response body size to prevent unbounded reads.
	maxResponseBytes = 1 << 20 // 1 MiB
)

// OAuthClient fetches subscription usage data from the Claude OAuth API.
type OAuthClient struct {
	httpClient  *http.Client
	accessToken string
	baseURL     string
	logger      *slog.Logger
}

// NewOAuthClient creates an OAuthClient with the given access token.
// The logger is used for structured diagnostic output. If nil, a no-op
// logger is used.
func NewOAuthClient(accessToken string, logger *slog.Logger) *OAuthClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &OAuthClient{
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		accessToken: accessToken,
		baseURL:     usageEndpoint,
		logger:      logger,
	}
}

// APIError represents a non-success HTTP response from the Claude API.
type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *APIError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("claude API error: %s (body: %s)", e.Status, e.Body)
	}
	return fmt.Sprintf("claude API error: %s", e.Status)
}

// usageWindowResponse is the raw JSON shape for a single rolling usage window.
type usageWindowResponse struct {
	Type     string  `json:"type"`
	Window   string  `json:"window"`
	Current  float64 `json:"current"`
	Limit    float64 `json:"limit"`
	ResetsAt string  `json:"resetsAt"`
}

// extraUsageResponse is the raw JSON shape for extra/overuse credits.
type extraUsageResponse struct {
	Enabled          bool    `json:"enabled"`
	MonthlyLimitCents int    `json:"monthlyLimitCents"`
	UsedCents        float64 `json:"usedCents"`
}

// OAuthUsageResponse is the raw JSON response from the Claude usage endpoint.
// Fields are kept flexible with pointer types to handle partial responses
// gracefully, since the exact API shape is not fully documented.
type OAuthUsageResponse struct {
	MessageLimit *usageWindowResponse `json:"messageLimit"`
	DailyLimit   *usageWindowResponse `json:"dailyLimit"`
	ExtraUsage   *extraUsageResponse  `json:"extraUsage"`
}

// FetchUsage retrieves the current subscription usage from the Claude OAuth API.
// It returns the raw parsed response on success. The caller is responsible for
// converting this to the collectors.ClaudeAccountUsage model via ToAccountUsage.
func (c *OAuthClient) FetchUsage(ctx context.Context) (*OAuthUsageResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	c.logger.Debug("fetching Claude usage", "url", c.baseURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}

	var usage OAuthUsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil, fmt.Errorf("parsing response JSON: %w", err)
	}

	c.logger.Debug("fetched Claude usage successfully")
	return &usage, nil
}

// ToAccountUsage converts the raw API response into a collectors.ClaudeAccountUsage
// value suitable for caching and display. The accountName is the user-defined label
// from configuration.
func (r *OAuthUsageResponse) ToAccountUsage(accountName string) collectors.ClaudeAccountUsage {
	usage := collectors.ClaudeAccountUsage{
		Name:   accountName,
		Type:   "subscription",
		Tier:   "pro",
		Status: "ok",
	}

	if r.MessageLimit != nil {
		period := windowToPeriod(r.MessageLimit)
		if period != nil {
			usage.FiveHour = period
		}
	}

	if r.DailyLimit != nil {
		period := windowToPeriod(r.DailyLimit)
		if period != nil {
			usage.SevenDay = period
		}
	}

	if r.ExtraUsage != nil {
		extra := &collectors.ExtraUsage{
			Enabled:      r.ExtraUsage.Enabled,
			MonthlyLimit: r.ExtraUsage.MonthlyLimitCents,
			UsedCredits:  r.ExtraUsage.UsedCents,
		}
		if extra.MonthlyLimit > 0 {
			extra.Utilization = (extra.UsedCredits / float64(extra.MonthlyLimit)) * 100
		}
		usage.ExtraUsage = extra
	}

	return usage
}

// windowToPeriod converts a raw usage window response to a UsagePeriod.
// Returns nil if the window has no limit (avoids division by zero).
func windowToPeriod(w *usageWindowResponse) *collectors.UsagePeriod {
	if w == nil || w.Limit <= 0 {
		return nil
	}

	period := &collectors.UsagePeriod{
		Utilization: (w.Current / w.Limit) * 100,
	}

	if w.ResetsAt != "" {
		if t, err := time.Parse(time.RFC3339, w.ResetsAt); err == nil {
			period.ResetsAt = t
		}
	}

	return period
}

// StatusFromError examines an error returned by FetchUsage and returns an
// appropriate status string for ClaudeAccountUsage.Status.
func StatusFromError(err error) string {
	if err == nil {
		return collectors.StatusOK
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		// Check for network errors
		if isNetworkError(err) {
			return collectors.StatusNetworkError
		}
		// Check for Cloudflare errors
		if isCloudflareError(err) {
			return collectors.StatusCloudflare
		}
		return collectors.StatusError
	}

	// Check if Cloudflare protection is active (403/503 with Cloudflare in body)
	if (apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusServiceUnavailable) &&
		isCloudflareBody(apiErr.Body) {
		return collectors.StatusCloudflare
	}

	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		return collectors.StatusAuthFailed
	case http.StatusForbidden:
		return collectors.StatusAuthFailed
	case http.StatusTooManyRequests:
		return collectors.StatusRateLimited
	default:
		return collectors.StatusError
	}
}

// isNetworkError checks if an error is a network-related error.
func isNetworkError(err error) bool {
	// Check for common network error strings
	errStr := err.Error()
	networkErrors := []string{
		"network timeout",
		"connection refused",
		"connection reset",
		"no such host",
		"i/o timeout",
		"EOF",
		"broken pipe",
	}
	for _, netErr := range networkErrors {
		if strings.Contains(strings.ToLower(errStr), netErr) {
			return true
		}
	}
	return false
}

// isCloudflareError checks if an error is related to Cloudflare protection.
func isCloudflareError(err error) bool {
	errStr := err.Error()
	cloudflareErrors := []string{
		"cloudflare",
		"cf-ray",
		"checking your browser",
		"ddos protection",
	}
	for _, cfErr := range cloudflareErrors {
		if strings.Contains(strings.ToLower(errStr), cfErr) {
			return true
		}
	}
	return false
}

// isCloudflareBody checks if a response body contains Cloudflare challenge markers.
func isCloudflareBody(body string) bool {
	cloudflareMarkers := []string{
		"just a moment",
		"checking your browser",
		"cloudflare",
		"cf-ray",
		"enable javascript and cookies",
		"ddos protection",
	}
	bodyLower := strings.ToLower(body)
	for _, marker := range cloudflareMarkers {
		if strings.Contains(bodyLower, marker) {
			return true
		}
	}
	return false
}
