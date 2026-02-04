package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAPIClient_FetchRateLimits_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("anthropic-ratelimit-requests-limit", "1000")
		w.Header().Set("anthropic-ratelimit-requests-remaining", "999")
		w.Header().Set("anthropic-ratelimit-requests-reset", "2025-01-15T12:00:00Z")
		w.Header().Set("anthropic-ratelimit-tokens-limit", "100000")
		w.Header().Set("anthropic-ratelimit-tokens-remaining", "99500")
		w.Header().Set("anthropic-ratelimit-tokens-reset", "2025-01-15T12:01:00Z")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","content":[{"type":"text","text":"p"}],"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":1}}`))
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL, "test-key")
	usage, err := client.FetchRateLimits(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usage.Type != "api" {
		t.Errorf("expected Type=api, got %s", usage.Type)
	}
	if usage.Status != "ok" {
		t.Errorf("expected Status=ok, got %s", usage.Status)
	}
	if usage.RateLimits == nil {
		t.Fatal("expected non-nil RateLimits")
	}
	if usage.RateLimits.RequestsLimit != 1000 {
		t.Errorf("expected RequestsLimit=1000, got %d", usage.RateLimits.RequestsLimit)
	}
	if usage.RateLimits.RequestsRemaining != 999 {
		t.Errorf("expected RequestsRemaining=999, got %d", usage.RateLimits.RequestsRemaining)
	}
	if usage.RateLimits.TokensLimit != 100000 {
		t.Errorf("expected TokensLimit=100000, got %d", usage.RateLimits.TokensLimit)
	}
	if usage.RateLimits.TokensRemaining != 99500 {
		t.Errorf("expected TokensRemaining=99500, got %d", usage.RateLimits.TokensRemaining)
	}
}

func TestAPIClient_FetchRateLimits_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL, "bad-key")
	usage, err := client.FetchRateLimits(context.Background())
	if err != nil {
		t.Fatalf("unexpected error for 401: %v", err)
	}

	if usage.Status != "auth_failed" {
		t.Errorf("expected Status=auth_failed, got %s", usage.Status)
	}
}

func TestAPIClient_FetchRateLimits_RateLimited(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("anthropic-ratelimit-requests-limit", "100")
		w.Header().Set("anthropic-ratelimit-requests-remaining", "0")
		w.Header().Set("retry-after", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`))
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL, "test-key")
	usage, err := client.FetchRateLimits(context.Background())
	if err != nil {
		t.Fatalf("unexpected error for 429: %v", err)
	}

	if usage.Status != "rate_limited" {
		t.Errorf("expected Status=rate_limited, got %s", usage.Status)
	}
	if usage.RateLimits == nil {
		t.Fatal("expected non-nil RateLimits even on 429")
	}
	if usage.RateLimits.RequestsRemaining != 0 {
		t.Errorf("expected RequestsRemaining=0, got %d", usage.RateLimits.RequestsRemaining)
	}
}

func TestAPIClient_FetchRateLimits_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"internal error"}}`))
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL, "test-key")
	_, err := client.FetchRateLimits(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("expected StatusCode=500, got %d", apiErr.StatusCode)
	}
}

func TestAPIClient_RequestFormat(t *testing.T) {
	var capturedReq *http.Request
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		capturedBody, _ = io.ReadAll(r.Body)

		w.Header().Set("anthropic-ratelimit-requests-limit", "1000")
		w.Header().Set("anthropic-ratelimit-requests-remaining", "999")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","content":[{"type":"text","text":"p"}],"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":1}}`))
	}))
	defer server.Close()

	client := newTestAPIClient(server.URL, "sk-ant-test-key-123")
	_, err := client.FetchRateLimits(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify HTTP method.
	if capturedReq.Method != http.MethodPost {
		t.Errorf("expected POST method, got %s", capturedReq.Method)
	}

	// Verify headers.
	if got := capturedReq.Header.Get("x-api-key"); got != "sk-ant-test-key-123" {
		t.Errorf("expected x-api-key=sk-ant-test-key-123, got %s", got)
	}
	if got := capturedReq.Header.Get("anthropic-version"); got != "2023-06-01" {
		t.Errorf("expected anthropic-version=2023-06-01, got %s", got)
	}
	if got := capturedReq.Header.Get("User-Agent"); got != "prompt-pulse/0.1.0" {
		t.Errorf("expected User-Agent=prompt-pulse/0.1.0, got %s", got)
	}
	if got := capturedReq.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %s", got)
	}

	// Verify request body structure.
	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	if model, ok := body["model"].(string); !ok || model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model=claude-sonnet-4-20250514, got %v", body["model"])
	}
	if maxTokens, ok := body["max_tokens"].(float64); !ok || int(maxTokens) != 1 {
		t.Errorf("expected max_tokens=1, got %v", body["max_tokens"])
	}

	messages, ok := body["messages"].([]interface{})
	if !ok || len(messages) != 1 {
		t.Fatalf("expected 1 message, got %v", body["messages"])
	}
	msg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected message to be a map")
	}
	if msg["role"] != "user" {
		t.Errorf("expected role=user, got %v", msg["role"])
	}
	if msg["content"] != "ping" {
		t.Errorf("expected content=ping, got %v", msg["content"])
	}
}

// newTestAPIClient creates an APIClient pointed at a test server URL.
// It bypasses the retry transport to use a minimal base delay for fast tests.
func newTestAPIClient(baseURL string, apiKey string) *APIClient {
	logger := testLogger()
	transport := &RetryTransport{
		Base:       http.DefaultTransport,
		MaxRetries: retryMaxRetries,
		BaseDelay:  1 * time.Millisecond, // Fast retries for tests.
		Logger:     logger,
	}

	client := &APIClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Transport: transport,
		},
		logger: logger,
	}

	// Override the messages endpoint by patching FetchRateLimits to use the test server.
	// We do this by replacing the httpClient's transport to rewrite URLs.
	client.httpClient.Transport = &urlRewriteTransport{
		base:    transport,
		fromURL: messagesEndpoint,
		toURL:   baseURL,
	}

	return client
}

// urlRewriteTransport rewrites request URLs for testing.
type urlRewriteTransport struct {
	base    http.RoundTripper
	fromURL string
	toURL   string
}

func (t *urlRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() == t.fromURL {
		newReq := req.Clone(req.Context())
		u, _ := req.URL.Parse(t.toURL)
		newReq.URL = u
		return t.base.RoundTrip(newReq)
	}
	return t.base.RoundTrip(req)
}
