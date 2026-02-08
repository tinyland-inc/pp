package claude

import (
	"bytes"
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
	// messagesEndpoint is the Anthropic Messages API URL.
	messagesEndpoint = "https://api.anthropic.com/v1/messages"

	// anthropicVersion is the API version header value.
	anthropicVersion = "2023-06-01"

	// apiRequestTimeout is the per-request timeout for API calls.
	apiRequestTimeout = 30 * time.Second

	// retryMaxRetries is the default number of retries for rate-limited requests.
	retryMaxRetries = 3

	// retryBaseDelay is the base delay for exponential backoff.
	retryBaseDelay = 5 * time.Second
)

// APIClient handles API-type Claude accounts. It probes the Anthropic Messages
// API with a minimal request to extract rate limit headers without incurring
// meaningful cost (<$0.001 per probe).
type APIClient struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewAPIClient creates an APIClient configured with retry logic via
// RetryTransport. The transport wraps http.DefaultTransport with
// MaxRetries=3 and BaseDelay=5s for exponential backoff on 429/529 responses.
func NewAPIClient(apiKey string, logger *slog.Logger) *APIClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	transport := &RetryTransport{
		Base:       http.DefaultTransport,
		MaxRetries: retryMaxRetries,
		BaseDelay:  retryBaseDelay,
		Logger:     logger,
	}

	return &APIClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   apiRequestTimeout,
		},
		logger: logger,
	}
}

// probeRequest is the minimal request body sent to the Messages API.
// It uses the cheapest possible parameters to minimize cost.
type probeRequest struct {
	Model     string         `json:"model"`
	MaxTokens int            `json:"max_tokens"`
	Messages  []probeMessage `json:"messages"`
}

// probeMessage is a single message in the probe request.
type probeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// FetchRateLimits sends a minimal request to the Anthropic Messages API and
// extracts rate limit information from the response headers. The probe uses
// claude-sonnet-4-20250514 with max_tokens=1 and a single "ping" message,
// costing less than $0.001.
//
// The returned ClaudeAccountUsage has Type="api" and Status set to one of:
//   - "ok" for successful responses (200)
//   - "auth_failed" for authentication errors (401)
//   - "rate_limited" for rate limit errors (429)
//
// Server errors (5xx) are returned as Go errors.
func (c *APIClient) FetchRateLimits(ctx context.Context) (*collectors.ClaudeAccountUsage, error) {
	probe := probeRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1,
		Messages: []probeMessage{
			{Role: "user", Content: "ping"},
		},
	}

	body, err := json.Marshal(probe)
	if err != nil {
		return nil, fmt.Errorf("marshaling probe request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, messagesEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("probing Anthropic API for rate limits", "url", messagesEndpoint)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Drain the response body to allow connection reuse.
	io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))

	// Parse rate limit headers regardless of status code.
	rateLimits := ParseRateLimitHeaders(resp.Header)

	usage := &collectors.ClaudeAccountUsage{
		Type:       "api",
		RateLimits: rateLimits,
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		usage.Status = "ok"
		c.logger.Debug("API probe successful", "status", resp.StatusCode)

	case resp.StatusCode == http.StatusUnauthorized:
		usage.Status = "auth_failed"
		c.logger.Warn("API authentication failed", "status", resp.StatusCode)

	case resp.StatusCode == http.StatusTooManyRequests:
		usage.Status = "rate_limited"
		c.logger.Warn("API rate limited", "status", resp.StatusCode)

	case resp.StatusCode >= 500:
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}

	default:
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	return usage, nil
}
