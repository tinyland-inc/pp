package claude

import (
	"bytes"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// RetryTransport wraps http.RoundTripper with retry logic for rate-limited responses.
// When a 429 (Too Many Requests) or 529 (Overloaded) response is received, the
// transport will sleep and retry the request up to MaxRetries times using
// exponential backoff with jitter.
type RetryTransport struct {
	Base       http.RoundTripper
	MaxRetries int
	BaseDelay  time.Duration
	Logger     *slog.Logger
}

// RoundTrip executes the HTTP request, retrying on 429 and 529 responses.
// It clones the request body before each attempt so that retries can re-send
// the original payload. Between retries it respects the retry-after header
// if present, otherwise falls back to exponential backoff with 0-25% jitter.
// The context attached to the request is checked between retries to allow
// early cancellation.
func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	logger := t.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// Read and buffer the request body so we can replay it on retries.
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= t.MaxRetries; attempt++ {
		if attempt > 0 {
			// Restore the body for retry attempts.
			if bodyBytes != nil {
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		resp, err = base.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		// Only retry on 429 (rate limited) or 529 (overloaded).
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != 529 {
			return resp, nil
		}

		// If we have exhausted all retries, return the final response as-is.
		if attempt == t.MaxRetries {
			return resp, nil
		}

		// Determine how long to wait before retrying.
		delay := parseRetryAfter(resp.Header)
		if delay <= 0 {
			// Exponential backoff: baseDelay * 2^attempt
			delay = t.BaseDelay * (1 << uint(attempt))
		}

		// Add 0-25% jitter to prevent thundering herd.
		jitter := time.Duration(rand.Int64N(int64(delay) / 4))
		delay += jitter

		logger.Warn("rate limited, retrying",
			"attempt", attempt+1,
			"max_retries", t.MaxRetries,
			"status", resp.StatusCode,
			"delay", delay,
		)

		// Drain and close the response body before retrying.
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// Wait for the delay or context cancellation.
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
		}
	}

	return resp, err
}

// ParseRateLimitHeaders extracts Anthropic rate limit information from HTTP
// response headers. The Anthropic API returns these headers on every response:
//
//   - anthropic-ratelimit-requests-limit
//   - anthropic-ratelimit-requests-remaining
//   - anthropic-ratelimit-requests-reset (RFC 3339)
//   - anthropic-ratelimit-tokens-limit
//   - anthropic-ratelimit-tokens-remaining
//   - anthropic-ratelimit-tokens-reset (RFC 3339)
//
// Returns nil if no rate limit headers are present.
func ParseRateLimitHeaders(h http.Header) *collectors.APIRateLimits {
	requestsLimit := h.Get("anthropic-ratelimit-requests-limit")
	requestsRemaining := h.Get("anthropic-ratelimit-requests-remaining")
	requestsReset := h.Get("anthropic-ratelimit-requests-reset")
	tokensLimit := h.Get("anthropic-ratelimit-tokens-limit")
	tokensRemaining := h.Get("anthropic-ratelimit-tokens-remaining")
	tokensReset := h.Get("anthropic-ratelimit-tokens-reset")

	// If no rate limit headers are present, return nil.
	if requestsLimit == "" && requestsRemaining == "" && tokensLimit == "" && tokensRemaining == "" {
		return nil
	}

	limits := &collectors.APIRateLimits{}

	if v, err := strconv.Atoi(requestsLimit); err == nil {
		limits.RequestsLimit = v
	}
	if v, err := strconv.Atoi(requestsRemaining); err == nil {
		limits.RequestsRemaining = v
	}
	if t, err := time.Parse(time.RFC3339, requestsReset); err == nil {
		limits.RequestsReset = t
	}
	if v, err := strconv.Atoi(tokensLimit); err == nil {
		limits.TokensLimit = v
	}
	if v, err := strconv.Atoi(tokensRemaining); err == nil {
		limits.TokensRemaining = v
	}
	if t, err := time.Parse(time.RFC3339, tokensReset); err == nil {
		limits.TokensReset = t
	}

	return limits
}

// parseRetryAfter extracts the retry-after header value as a duration.
// The header is expected to contain an integer number of seconds.
// Returns zero if the header is missing or cannot be parsed.
func parseRetryAfter(h http.Header) time.Duration {
	val := h.Get("retry-after")
	if val == "" {
		return 0
	}

	seconds, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}

	return time.Duration(seconds) * time.Second
}
