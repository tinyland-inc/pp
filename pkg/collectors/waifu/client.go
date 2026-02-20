package waifu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client abstracts HTTP calls to the waifu mirror API. This interface exists
// so tests can inject a mock implementation.
type Client interface {
	// RandomImage fetches a random image URL from the mirror API.
	RandomImage(ctx context.Context, category string) (*ImageMeta, error)

	// DownloadImage fetches image bytes from the given URL.
	DownloadImage(ctx context.Context, url string) ([]byte, error)
}

// ImageMeta is the JSON response from the mirror's /api/random endpoint.
type ImageMeta struct {
	URL    string `json:"url"`
	ID     string `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Hash   string `json:"hash"`
}

// httpClient is the production Client backed by net/http.
type httpClient struct {
	base     string
	hc       *http.Client
}

// NewClient creates a Client that talks to the waifu mirror at endpoint.
func NewClient(endpoint string) Client {
	return &httpClient{
		base: endpoint,
		hc: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *httpClient) RandomImage(ctx context.Context, category string) (*ImageMeta, error) {
	url := fmt.Sprintf("%s/api/random?category=%s", c.base, category)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("waifu: build request: %w", err)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("waifu: fetch random: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("waifu: random returned %d", resp.StatusCode)
	}

	var meta ImageMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("waifu: decode response: %w", err)
	}
	return &meta, nil
}

func (c *httpClient) DownloadImage(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("waifu: build download request: %w", err)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("waifu: download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("waifu: download returned %d", resp.StatusCode)
	}

	// Limit reads to 10MB to prevent abuse.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("waifu: read image body: %w", err)
	}
	return data, nil
}
