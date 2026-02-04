// Package claude provides OAuth token refresh functionality for Claude credentials.
package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// tokenEndpoint is the Claude OAuth token refresh endpoint.
	tokenEndpoint = "https://api.claude.ai/api/auth/oauth/token"

	// refreshBuffer is the time before expiration when a refresh is recommended.
	// Tokens expiring within this window will report NeedsRefresh() = true.
	refreshBuffer = 5 * time.Minute

	// refreshTimeout is the per-request timeout for token refresh operations.
	refreshTimeout = 30 * time.Second
)

// TokenRefreshResponse represents the OAuth token refresh response from Claude.
type TokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// TokenRefresher handles OAuth token refresh operations for Claude credentials.
type TokenRefresher struct {
	client *http.Client
	logger *slog.Logger
}

// NewTokenRefresher creates a TokenRefresher with the given logger.
// If logger is nil, a no-op logger is used.
func NewTokenRefresher(logger *slog.Logger) *TokenRefresher {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &TokenRefresher{
		client: &http.Client{Timeout: refreshTimeout},
		logger: logger,
	}
}

// RefreshToken exchanges a refresh token for new access and refresh tokens.
// On success, returns the new token response. The caller is responsible for
// persisting the new tokens to the credential file.
func (r *TokenRefresher) RefreshToken(ctx context.Context, refreshToken string) (*TokenRefreshResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

	r.logger.Debug("refreshing OAuth token", "endpoint", tokenEndpoint)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(body),
		}
	}

	var tokenResp TokenRefreshResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parsing refresh response: %w", err)
	}

	r.logger.Debug("token refresh successful", "expires_in", tokenResp.ExpiresIn)
	return &tokenResp, nil
}

// UpdateCredentialFile reads the credential file at path, updates it with the
// new tokens from the refresh response, and writes it back atomically.
// The ExpiresAt field is calculated from ExpiresIn relative to the current time.
func (r *TokenRefresher) UpdateCredentialFile(path string, tokens *TokenRefreshResponse) error {
	// Read the existing credential file.
	creds, err := LoadCredentials(path)
	if err != nil {
		return fmt.Errorf("loading existing credentials: %w", err)
	}

	if creds.ClaudeAiOauth == nil {
		return fmt.Errorf("credential file missing claudeAiOauth key")
	}

	// Update the OAuth credentials with new tokens.
	creds.ClaudeAiOauth.AccessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		creds.ClaudeAiOauth.RefreshToken = tokens.RefreshToken
	}
	// Calculate ExpiresAt as Unix milliseconds from now + ExpiresIn seconds.
	creds.ClaudeAiOauth.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second).UnixMilli()

	// Marshal the updated credentials.
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling updated credentials: %w", err)
	}

	// Write atomically: write to temp file, then rename.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("writing temp credential file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		// Clean up temp file on rename failure.
		os.Remove(tmpPath)
		return fmt.Errorf("renaming credential file: %w", err)
	}

	r.logger.Info("credential file updated", "path", path, "expires_at", time.UnixMilli(creds.ClaudeAiOauth.ExpiresAt))
	return nil
}

// RefreshAndPersist performs a token refresh and updates the credential file
// in a single operation. This is a convenience method combining RefreshToken
// and UpdateCredentialFile.
func (r *TokenRefresher) RefreshAndPersist(ctx context.Context, credPath string, refreshToken string) (*TokenRefreshResponse, error) {
	tokens, err := r.RefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	if err := r.UpdateCredentialFile(credPath, tokens); err != nil {
		return tokens, fmt.Errorf("persisting refreshed tokens: %w", err)
	}

	return tokens, nil
}
