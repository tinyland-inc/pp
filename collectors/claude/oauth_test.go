package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// validUsageJSON is a representative response from the Claude usage endpoint.
const validUsageJSON = `{
	"messageLimit": {
		"type": "rolling_window",
		"window": "5h",
		"current": 45,
		"limit": 100,
		"resetsAt": "2026-02-03T15:00:00Z"
	},
	"dailyLimit": {
		"type": "rolling_window",
		"window": "24h",
		"current": 200,
		"limit": 500,
		"resetsAt": "2026-02-04T00:00:00Z"
	},
	"extraUsage": {
		"enabled": true,
		"monthlyLimitCents": 10000,
		"usedCents": 2500
	}
}`

func TestFetchUsage_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request properties.
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("User-Agent"); got != userAgent {
			t.Errorf("User-Agent header = %q, want %q", got, userAgent)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept header = %q, want %q", got, "application/json")
		}
		if r.Method != http.MethodGet {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validUsageJSON))
	}))
	defer srv.Close()

	client := NewOAuthClient("test-token", nil)
	client.baseURL = srv.URL

	resp, err := client.FetchUsage(context.Background())
	if err != nil {
		t.Fatalf("FetchUsage() error = %v", err)
	}

	// Verify message limit parsing.
	if resp.MessageLimit == nil {
		t.Fatal("MessageLimit is nil")
	}
	if resp.MessageLimit.Current != 45 {
		t.Errorf("MessageLimit.Current = %v, want 45", resp.MessageLimit.Current)
	}
	if resp.MessageLimit.Limit != 100 {
		t.Errorf("MessageLimit.Limit = %v, want 100", resp.MessageLimit.Limit)
	}
	if resp.MessageLimit.ResetsAt != "2026-02-03T15:00:00Z" {
		t.Errorf("MessageLimit.ResetsAt = %q, want %q", resp.MessageLimit.ResetsAt, "2026-02-03T15:00:00Z")
	}

	// Verify daily limit parsing.
	if resp.DailyLimit == nil {
		t.Fatal("DailyLimit is nil")
	}
	if resp.DailyLimit.Current != 200 {
		t.Errorf("DailyLimit.Current = %v, want 200", resp.DailyLimit.Current)
	}
	if resp.DailyLimit.Limit != 500 {
		t.Errorf("DailyLimit.Limit = %v, want 500", resp.DailyLimit.Limit)
	}

	// Verify extra usage parsing.
	if resp.ExtraUsage == nil {
		t.Fatal("ExtraUsage is nil")
	}
	if !resp.ExtraUsage.Enabled {
		t.Error("ExtraUsage.Enabled = false, want true")
	}
	if resp.ExtraUsage.MonthlyLimitCents != 10000 {
		t.Errorf("ExtraUsage.MonthlyLimitCents = %d, want 10000", resp.ExtraUsage.MonthlyLimitCents)
	}
	if resp.ExtraUsage.UsedCents != 2500 {
		t.Errorf("ExtraUsage.UsedCents = %v, want 2500", resp.ExtraUsage.UsedCents)
	}
}

func TestFetchUsage_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
	}))
	defer srv.Close()

	client := NewOAuthClient("bad-token", nil)
	client.baseURL = srv.URL

	_, err := client.FetchUsage(context.Background())
	if err == nil {
		t.Fatal("FetchUsage() expected error for 401 response")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
	if !strings.Contains(apiErr.Body, "invalid_token") {
		t.Errorf("Body = %q, want to contain %q", apiErr.Body, "invalid_token")
	}
	if got := StatusFromError(err); got != "auth_failed" {
		t.Errorf("StatusFromError() = %q, want %q", got, "auth_failed")
	}
}

func TestFetchUsage_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	client := NewOAuthClient("token", nil)
	client.baseURL = srv.URL

	_, err := client.FetchUsage(context.Background())
	if err == nil {
		t.Fatal("FetchUsage() expected error for 403 response")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusForbidden)
	}
	if got := StatusFromError(err); got != "auth_failed" {
		t.Errorf("StatusFromError() = %q, want %q", got, "auth_failed")
	}
}

func TestFetchUsage_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate_limited"}`))
	}))
	defer srv.Close()

	client := NewOAuthClient("token", nil)
	client.baseURL = srv.URL

	_, err := client.FetchUsage(context.Background())
	if err == nil {
		t.Fatal("FetchUsage() expected error for 429 response")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusTooManyRequests)
	}
	if got := StatusFromError(err); got != "rate_limited" {
		t.Errorf("StatusFromError() = %q, want %q", got, "rate_limited")
	}
}

func TestFetchUsage_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal_error"}`))
	}))
	defer srv.Close()

	client := NewOAuthClient("token", nil)
	client.baseURL = srv.URL

	_, err := client.FetchUsage(context.Background())
	if err == nil {
		t.Fatal("FetchUsage() expected error for 500 response")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusInternalServerError)
	}
	if got := StatusFromError(err); got != "error" {
		t.Errorf("StatusFromError() = %q, want %q", got, "error")
	}
}

func TestFetchUsage_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	client := NewOAuthClient("token", nil)
	client.baseURL = srv.URL

	_, err := client.FetchUsage(context.Background())
	if err == nil {
		t.Fatal("FetchUsage() expected error for malformed JSON")
	}
	if strings.Contains(err.Error(), "claude API error") {
		t.Errorf("malformed JSON should not produce APIError, got: %v", err)
	}
	if !strings.Contains(err.Error(), "parsing response JSON") {
		t.Errorf("error should mention JSON parsing, got: %v", err)
	}
}

func TestFetchUsage_ContextCancellation(t *testing.T) {
	// Server that blocks until context is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := NewOAuthClient("token", nil)
	client.baseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := client.FetchUsage(ctx)
	if err == nil {
		t.Fatal("FetchUsage() expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") &&
		!strings.Contains(err.Error(), "context") {
		t.Errorf("error should mention context, got: %v", err)
	}
}

func TestFetchUsage_Timeout(t *testing.T) {
	// Server that never responds.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(30 * time.Second):
			return
		}
	}))
	defer srv.Close()

	client := NewOAuthClient("token", nil)
	client.baseURL = srv.URL
	// Use a very short timeout for the test.
	client.httpClient.Timeout = 50 * time.Millisecond

	_, err := client.FetchUsage(context.Background())
	if err == nil {
		t.Fatal("FetchUsage() expected error for timeout")
	}
	if !strings.Contains(err.Error(), "deadline exceeded") &&
		!strings.Contains(err.Error(), "Timeout") &&
		!strings.Contains(err.Error(), "timeout") {
		t.Errorf("error should mention timeout or deadline, got: %v", err)
	}
}

func TestToAccountUsage_FullResponse(t *testing.T) {
	var resp OAuthUsageResponse
	if err := json.Unmarshal([]byte(validUsageJSON), &resp); err != nil {
		t.Fatalf("failed to unmarshal test JSON: %v", err)
	}

	usage := resp.ToAccountUsage("work-account")

	if usage.Name != "work-account" {
		t.Errorf("Name = %q, want %q", usage.Name, "work-account")
	}
	if usage.Type != "subscription" {
		t.Errorf("Type = %q, want %q", usage.Type, "subscription")
	}
	if usage.Status != "ok" {
		t.Errorf("Status = %q, want %q", usage.Status, "ok")
	}

	// 5-hour window: 45/100 = 45%.
	if usage.FiveHour == nil {
		t.Fatal("FiveHour is nil")
	}
	if usage.FiveHour.Utilization != 45 {
		t.Errorf("FiveHour.Utilization = %v, want 45", usage.FiveHour.Utilization)
	}
	expectedReset, _ := time.Parse(time.RFC3339, "2026-02-03T15:00:00Z")
	if !usage.FiveHour.ResetsAt.Equal(expectedReset) {
		t.Errorf("FiveHour.ResetsAt = %v, want %v", usage.FiveHour.ResetsAt, expectedReset)
	}

	// Daily window: 200/500 = 40%.
	if usage.SevenDay == nil {
		t.Fatal("SevenDay is nil")
	}
	if usage.SevenDay.Utilization != 40 {
		t.Errorf("SevenDay.Utilization = %v, want 40", usage.SevenDay.Utilization)
	}

	// Extra usage: 2500/10000 = 25%.
	if usage.ExtraUsage == nil {
		t.Fatal("ExtraUsage is nil")
	}
	if !usage.ExtraUsage.Enabled {
		t.Error("ExtraUsage.Enabled = false, want true")
	}
	if usage.ExtraUsage.MonthlyLimit != 10000 {
		t.Errorf("ExtraUsage.MonthlyLimit = %d, want 10000", usage.ExtraUsage.MonthlyLimit)
	}
	if usage.ExtraUsage.UsedCredits != 2500 {
		t.Errorf("ExtraUsage.UsedCredits = %v, want 2500", usage.ExtraUsage.UsedCredits)
	}
	if usage.ExtraUsage.Utilization != 25 {
		t.Errorf("ExtraUsage.Utilization = %v, want 25", usage.ExtraUsage.Utilization)
	}
}

func TestToAccountUsage_PartialResponse(t *testing.T) {
	// API returns only messageLimit, no dailyLimit or extraUsage.
	resp := &OAuthUsageResponse{
		MessageLimit: &usageWindowResponse{
			Type:     "rolling_window",
			Window:   "5h",
			Current:  10,
			Limit:    100,
			ResetsAt: "2026-02-03T15:00:00Z",
		},
	}

	usage := resp.ToAccountUsage("partial")

	if usage.FiveHour == nil {
		t.Fatal("FiveHour should not be nil")
	}
	if usage.FiveHour.Utilization != 10 {
		t.Errorf("FiveHour.Utilization = %v, want 10", usage.FiveHour.Utilization)
	}
	if usage.SevenDay != nil {
		t.Error("SevenDay should be nil for partial response")
	}
	if usage.ExtraUsage != nil {
		t.Error("ExtraUsage should be nil for partial response")
	}
}

func TestToAccountUsage_EmptyResponse(t *testing.T) {
	resp := &OAuthUsageResponse{}
	usage := resp.ToAccountUsage("empty")

	if usage.Name != "empty" {
		t.Errorf("Name = %q, want %q", usage.Name, "empty")
	}
	if usage.Status != "ok" {
		t.Errorf("Status = %q, want %q", usage.Status, "ok")
	}
	if usage.FiveHour != nil {
		t.Error("FiveHour should be nil for empty response")
	}
	if usage.SevenDay != nil {
		t.Error("SevenDay should be nil for empty response")
	}
	if usage.ExtraUsage != nil {
		t.Error("ExtraUsage should be nil for empty response")
	}
}

func TestToAccountUsage_ZeroLimit(t *testing.T) {
	// Window with zero limit should not produce a period (avoids division by zero).
	resp := &OAuthUsageResponse{
		MessageLimit: &usageWindowResponse{
			Current: 10,
			Limit:   0,
		},
	}

	usage := resp.ToAccountUsage("zero-limit")
	if usage.FiveHour != nil {
		t.Error("FiveHour should be nil when limit is zero")
	}
}

func TestToAccountUsage_ExtraUsageZeroLimit(t *testing.T) {
	resp := &OAuthUsageResponse{
		ExtraUsage: &extraUsageResponse{
			Enabled:          true,
			MonthlyLimitCents: 0,
			UsedCents:        100,
		},
	}

	usage := resp.ToAccountUsage("zero-extra")
	if usage.ExtraUsage == nil {
		t.Fatal("ExtraUsage should not be nil")
	}
	if usage.ExtraUsage.Utilization != 0 {
		t.Errorf("Utilization = %v, want 0 (no division by zero)", usage.ExtraUsage.Utilization)
	}
}

func TestStatusFromError_NilError(t *testing.T) {
	if got := StatusFromError(nil); got != "ok" {
		t.Errorf("StatusFromError(nil) = %q, want %q", got, "ok")
	}
}

func TestStatusFromError_NonAPIError(t *testing.T) {
	err := fmt.Errorf("network unreachable")
	if got := StatusFromError(err); got != "error" {
		t.Errorf("StatusFromError(network error) = %q, want %q", got, "error")
	}
}

func TestStatusFromError_StatusCodes(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{http.StatusUnauthorized, "auth_failed"},
		{http.StatusForbidden, "auth_failed"},
		{http.StatusTooManyRequests, "rate_limited"},
		{http.StatusInternalServerError, "error"},
		{http.StatusBadGateway, "error"},
		{http.StatusServiceUnavailable, "error"},
	}

	for _, tt := range tests {
		err := &APIError{StatusCode: tt.code, Status: http.StatusText(tt.code)}
		if got := StatusFromError(err); got != tt.want {
			t.Errorf("StatusFromError(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestAPIError_Error(t *testing.T) {
	t.Run("with body", func(t *testing.T) {
		err := &APIError{StatusCode: 401, Status: "401 Unauthorized", Body: `{"error":"bad"}`}
		got := err.Error()
		if !strings.Contains(got, "401 Unauthorized") {
			t.Errorf("Error() = %q, want to contain status", got)
		}
		if !strings.Contains(got, `{"error":"bad"}`) {
			t.Errorf("Error() = %q, want to contain body", got)
		}
	})

	t.Run("without body", func(t *testing.T) {
		err := &APIError{StatusCode: 500, Status: "500 Internal Server Error", Body: ""}
		got := err.Error()
		if !strings.Contains(got, "500 Internal Server Error") {
			t.Errorf("Error() = %q, want to contain status", got)
		}
		if strings.Contains(got, "body") {
			t.Errorf("Error() = %q, should not mention body when empty", got)
		}
	})
}

func TestFetchUsage_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewOAuthClient("token", nil)
	client.baseURL = srv.URL

	resp, err := client.FetchUsage(context.Background())
	if err != nil {
		t.Fatalf("FetchUsage() error = %v", err)
	}
	if resp.MessageLimit != nil {
		t.Error("MessageLimit should be nil for empty JSON object")
	}
	if resp.DailyLimit != nil {
		t.Error("DailyLimit should be nil for empty JSON object")
	}
	if resp.ExtraUsage != nil {
		t.Error("ExtraUsage should be nil for empty JSON object")
	}
}
