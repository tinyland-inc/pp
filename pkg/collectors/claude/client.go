package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// defaultBaseURL is the Anthropic API base URL.
	defaultBaseURL = "https://api.anthropic.com"

	// anthropicVersion is the API version header value.
	anthropicVersion = "2023-06-01"

	// httpTimeout is the default HTTP client timeout.
	httpTimeout = 30 * time.Second
)

// APIClient abstracts the Anthropic Admin API for testability. The real
// implementation makes HTTP calls; tests inject a mock.
type APIClient interface {
	// GetUsage retrieves token usage for the given organization and date range.
	// startDate and endDate are in YYYY-MM-DD format (inclusive).
	GetUsage(ctx context.Context, orgID, apiKey, startDate, endDate string) (*APIUsageResponse, error)

	// GetOrganizations lists organizations accessible by the given admin key.
	GetOrganizations(ctx context.Context, apiKey string) ([]Organization, error)
}

// Organization represents an Anthropic organization.
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// OrganizationsResponse represents the list organizations API response.
type OrganizationsResponse struct {
	Data []Organization `json:"data"`
}

// APIUsageResponse represents the JSON response from the Anthropic usage API.
// Unknown fields are silently ignored by encoding/json.
type APIUsageResponse struct {
	Data []APIUsageEntry `json:"data"`
}

// APIUsageEntry represents a single usage entry in the API response.
// The Anthropic API returns per-model, per-day breakdowns.
type APIUsageEntry struct {
	Date                string `json:"date"`
	Model               string `json:"model"`
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_input_tokens"`
	CacheReadTokens     int64  `json:"cache_read_input_tokens"`
}

// HTTPClient implements APIClient using real HTTP calls to the Anthropic
// Admin API.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPClient creates an HTTPClient with sensible defaults. The baseURL
// parameter is optional; pass empty string to use the default.
func NewHTTPClient(baseURL string) *HTTPClient {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// GetUsage calls the Anthropic Admin API to retrieve usage data.
func (c *HTTPClient) GetUsage(ctx context.Context, orgID, apiKey, startDate, endDate string) (*APIUsageResponse, error) {
	url := fmt.Sprintf("%s/v1/organizations/%s/usage?start_date=%s&end_date=%s",
		c.baseURL, orgID, startDate, endDate)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result APIUsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// GetOrganizations calls the Anthropic Admin API to list organizations.
func (c *HTTPClient) GetOrganizations(ctx context.Context, apiKey string) ([]Organization, error) {
	url := fmt.Sprintf("%s/v1/organizations", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result OrganizationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return result.Data, nil
}
