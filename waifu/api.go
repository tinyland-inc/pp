// Package waifu provides an HTTP client for the waifu.pics API,
// supporting single and batch image URL fetching plus image downloads.
package waifu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.waifu.pics"
	defaultTimeout = 15 * time.Second
	maxBodySize    = 10 * 1024 * 1024 // 10MB for image downloads
)

// baseURL is the API base URL. It can be overridden in tests.
var baseURL = defaultBaseURL

// ValidCategories lists all valid SFW categories supported by the waifu.pics API.
var ValidCategories = []string{
	"waifu", "neko", "shinobu", "megumin", "bully", "cuddle", "cry",
	"hug", "awoo", "kiss", "lick", "pat", "smug", "bonk", "yeet",
	"blush", "smile", "wave", "highfive", "handhold", "nom", "bite",
	"glomp", "slap", "kill", "kick", "happy", "wink", "poke", "dance",
	"cringe",
}

// validCategorySet is a lookup map built from ValidCategories for O(1) checks.
var validCategorySet map[string]bool

func init() {
	validCategorySet = make(map[string]bool, len(ValidCategories))
	for _, c := range ValidCategories {
		validCategorySet[c] = true
	}
}

// IsValidCategory reports whether the given category is a recognized SFW category.
func IsValidCategory(category string) bool {
	return validCategorySet[category]
}

// APIError represents a non-success HTTP response from the waifu.pics API.
type APIError struct {
	StatusCode int
	Status     string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("waifu.pics API error: %s", e.Status)
}

// singleResponse is the JSON shape returned by GET /sfw/{category}.
type singleResponse struct {
	URL string `json:"url"`
}

// manyResponse is the JSON shape returned by POST /many/sfw/{category}.
type manyResponse struct {
	Files []string `json:"files"`
}

// APIClient communicates with the waifu.pics API to fetch image URLs
// and download image data.
type APIClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewAPIClient creates an APIClient with the default base URL and a 15-second
// timeout. If logger is nil, a no-op logger is used.
func NewAPIClient(logger *slog.Logger) *APIClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &APIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		logger: logger,
	}
}

// FetchImageURL fetches a single random image URL for the given SFW category.
// It calls GET {baseURL}/sfw/{category} and returns the URL from the JSON response.
func (c *APIClient) FetchImageURL(ctx context.Context, category string) (string, error) {
	if !IsValidCategory(category) {
		return "", fmt.Errorf("invalid waifu category: %q", category)
	}

	url := fmt.Sprintf("%s/sfw/%s", c.baseURL, category)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	c.logger.Debug("fetching waifu image URL", "category", category, "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	var result singleResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing response JSON: %w", err)
	}

	if result.URL == "" {
		return "", fmt.Errorf("empty URL in response")
	}

	c.logger.Debug("fetched waifu image URL", "category", category, "image_url", result.URL)

	return result.URL, nil
}

// FetchMultipleURLs fetches up to count random image URLs for the given SFW category.
// It calls POST {baseURL}/many/sfw/{category} with an empty JSON body. The API
// returns up to 30 URLs; if count is less than that, the result is truncated.
func (c *APIClient) FetchMultipleURLs(ctx context.Context, category string, count int) ([]string, error) {
	if !IsValidCategory(category) {
		return nil, fmt.Errorf("invalid waifu category: %q", category)
	}

	if count <= 0 {
		return nil, nil
	}

	url := fmt.Sprintf("%s/many/sfw/%s", c.baseURL, category)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("fetching multiple waifu image URLs", "category", category, "count", count, "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	var result manyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response JSON: %w", err)
	}

	// Truncate to the requested count.
	if count < len(result.Files) {
		result.Files = result.Files[:count]
	}

	c.logger.Debug("fetched multiple waifu image URLs", "category", category, "returned", len(result.Files))

	return result.Files, nil
}

// allowedImageTypes lists the content types accepted for image downloads.
var allowedImageTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

// DownloadImage downloads an image from the given URL and returns the raw bytes,
// the content type, and any error. It enforces a 10MB size limit and validates
// that the response content type is an image.
func (c *APIClient) DownloadImage(ctx context.Context, imageURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}

	c.logger.Debug("downloading waifu image", "url", imageURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Drain and discard body on error.
		io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodySize))
		return nil, "", &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}

	// Validate content type before reading the full body.
	contentType := resp.Header.Get("Content-Type")
	// Handle content types with parameters like "image/png; charset=utf-8".
	baseCT := strings.SplitN(contentType, ";", 2)[0]
	baseCT = strings.TrimSpace(baseCT)

	if !allowedImageTypes[baseCT] {
		io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodySize))
		return nil, "", fmt.Errorf("unexpected content type: %q (expected image)", contentType)
	}

	// Read with a limit of maxBodySize + 1 byte to detect oversized responses.
	limitedReader := io.LimitReader(resp.Body, maxBodySize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, "", fmt.Errorf("reading image data: %w", err)
	}

	if len(data) > maxBodySize {
		return nil, "", fmt.Errorf("image too large: exceeds %d bytes", maxBodySize)
	}

	c.logger.Debug("downloaded waifu image", "url", imageURL, "size", len(data), "content_type", baseCT)

	return data, baseCT, nil
}
