package claude

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// mockTransport is a configurable http.RoundTripper for testing.
type mockTransport struct {
	responses []*http.Response
	calls     int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.calls >= len(m.responses) {
		// Return the last response for any overflow calls.
		resp := m.responses[len(m.responses)-1]
		m.calls++
		return resp, nil
	}
	resp := m.responses[m.calls]
	m.calls++
	return resp, nil
}

func newResponse(statusCode int, headers map[string]string) *http.Response {
	h := http.Header{}
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func TestRetryTransport_NoRetryOn200(t *testing.T) {
	mock := &mockTransport{
		responses: []*http.Response{
			newResponse(200, nil),
		},
	}

	transport := &RetryTransport{
		Base:       mock,
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
		Logger:     testLogger(),
	}

	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if mock.calls != 1 {
		t.Errorf("expected 1 call, got %d", mock.calls)
	}
}

func TestRetryTransport_RetryOn429(t *testing.T) {
	mock := &mockTransport{
		responses: []*http.Response{
			newResponse(429, nil),
			newResponse(200, nil),
		},
	}

	transport := &RetryTransport{
		Base:       mock,
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond,
		Logger:     testLogger(),
	}

	req, _ := http.NewRequest(http.MethodPost, "https://example.com",
		strings.NewReader(`{"test": true}`))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if mock.calls != 2 {
		t.Errorf("expected 2 calls, got %d", mock.calls)
	}
}

func TestRetryTransport_RetryAfterHeader(t *testing.T) {
	mock := &mockTransport{
		responses: []*http.Response{
			newResponse(429, map[string]string{"retry-after": "1"}),
			newResponse(200, nil),
		},
	}

	transport := &RetryTransport{
		Base:       mock,
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond,
		Logger:     testLogger(),
	}

	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)

	start := time.Now()
	resp, err := transport.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// The retry-after header says 1 second, so elapsed should be >= 1s.
	if elapsed < 1*time.Second {
		t.Errorf("expected delay of at least 1s from retry-after header, got %v", elapsed)
	}
}

func TestRetryTransport_MaxRetriesExhausted(t *testing.T) {
	mock := &mockTransport{
		responses: []*http.Response{
			newResponse(429, nil),
			newResponse(429, nil),
			newResponse(429, nil),
			newResponse(429, nil),
		},
	}

	transport := &RetryTransport{
		Base:       mock,
		MaxRetries: 3,
		BaseDelay:  1 * time.Millisecond,
		Logger:     testLogger(),
	}

	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 429 {
		t.Errorf("expected status 429 after exhausted retries, got %d", resp.StatusCode)
	}

	// 1 initial + 3 retries = 4 total calls.
	if mock.calls != 4 {
		t.Errorf("expected 4 calls (1 + 3 retries), got %d", mock.calls)
	}
}

func TestRetryTransport_ContextCancellation(t *testing.T) {
	mock := &mockTransport{
		responses: []*http.Response{
			newResponse(429, nil),
			newResponse(429, nil),
			newResponse(200, nil),
		},
	}

	transport := &RetryTransport{
		Base:       mock,
		MaxRetries: 5,
		BaseDelay:  1 * time.Second, // Long delay so context cancels first.
		Logger:     testLogger(),
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	_, err := transport.RoundTrip(req)

	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRetryTransport_ExponentialBackoff(t *testing.T) {
	mock := &mockTransport{
		responses: []*http.Response{
			newResponse(429, nil),
			newResponse(429, nil),
			newResponse(429, nil),
			newResponse(200, nil),
		},
	}

	baseDelay := 10 * time.Millisecond
	transport := &RetryTransport{
		Base:       mock,
		MaxRetries: 3,
		BaseDelay:  baseDelay,
		Logger:     testLogger(),
	}

	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)

	start := time.Now()
	resp, err := transport.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Three retries with exponential backoff:
	// attempt 0: 10ms * 2^0 = 10ms
	// attempt 1: 10ms * 2^1 = 20ms
	// attempt 2: 10ms * 2^2 = 40ms
	// Minimum total: 70ms (without jitter).
	// With up to 25% jitter, max would be about 87.5ms.
	minExpected := 70 * time.Millisecond
	if elapsed < minExpected {
		t.Errorf("expected total delay >= %v (exponential backoff), got %v", minExpected, elapsed)
	}
}

func TestParseRateLimitHeaders_AllHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-requests-limit", "1000")
	h.Set("anthropic-ratelimit-requests-remaining", "999")
	h.Set("anthropic-ratelimit-requests-reset", "2025-01-15T12:00:00Z")
	h.Set("anthropic-ratelimit-tokens-limit", "100000")
	h.Set("anthropic-ratelimit-tokens-remaining", "99500")
	h.Set("anthropic-ratelimit-tokens-reset", "2025-01-15T12:01:00Z")

	limits := ParseRateLimitHeaders(h)
	if limits == nil {
		t.Fatal("expected non-nil rate limits")
	}

	if limits.RequestsLimit != 1000 {
		t.Errorf("expected RequestsLimit=1000, got %d", limits.RequestsLimit)
	}
	if limits.RequestsRemaining != 999 {
		t.Errorf("expected RequestsRemaining=999, got %d", limits.RequestsRemaining)
	}
	expectedReqReset := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	if !limits.RequestsReset.Equal(expectedReqReset) {
		t.Errorf("expected RequestsReset=%v, got %v", expectedReqReset, limits.RequestsReset)
	}
	if limits.TokensLimit != 100000 {
		t.Errorf("expected TokensLimit=100000, got %d", limits.TokensLimit)
	}
	if limits.TokensRemaining != 99500 {
		t.Errorf("expected TokensRemaining=99500, got %d", limits.TokensRemaining)
	}
	expectedTokReset := time.Date(2025, 1, 15, 12, 1, 0, 0, time.UTC)
	if !limits.TokensReset.Equal(expectedTokReset) {
		t.Errorf("expected TokensReset=%v, got %v", expectedTokReset, limits.TokensReset)
	}
}

func TestParseRateLimitHeaders_NoHeaders(t *testing.T) {
	h := http.Header{}
	limits := ParseRateLimitHeaders(h)
	if limits != nil {
		t.Errorf("expected nil for empty headers, got %+v", limits)
	}
}

func TestParseRateLimitHeaders_PartialHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-requests-limit", "500")
	h.Set("anthropic-ratelimit-requests-remaining", "250")
	// No tokens headers, no reset headers.

	limits := ParseRateLimitHeaders(h)
	if limits == nil {
		t.Fatal("expected non-nil rate limits with partial headers")
	}

	if limits.RequestsLimit != 500 {
		t.Errorf("expected RequestsLimit=500, got %d", limits.RequestsLimit)
	}
	if limits.RequestsRemaining != 250 {
		t.Errorf("expected RequestsRemaining=250, got %d", limits.RequestsRemaining)
	}
	if limits.TokensLimit != 0 {
		t.Errorf("expected TokensLimit=0 for missing header, got %d", limits.TokensLimit)
	}
	if limits.TokensRemaining != 0 {
		t.Errorf("expected TokensRemaining=0 for missing header, got %d", limits.TokensRemaining)
	}
	if !limits.RequestsReset.IsZero() {
		t.Errorf("expected zero RequestsReset for missing header, got %v", limits.RequestsReset)
	}
}

func TestParseRetryAfter_ValidInteger(t *testing.T) {
	h := http.Header{}
	h.Set("retry-after", "30")

	d := parseRetryAfter(h)
	expected := 30 * time.Second
	if d != expected {
		t.Errorf("expected %v, got %v", expected, d)
	}
}

func TestParseRetryAfter_Missing(t *testing.T) {
	h := http.Header{}

	d := parseRetryAfter(h)
	if d != 0 {
		t.Errorf("expected 0 for missing header, got %v", d)
	}
}
