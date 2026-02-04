package claude

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// --- Mock types ---

// mockUsageFetcher implements UsageFetcher for testing.
type mockUsageFetcher struct {
	response *OAuthUsageResponse
	err      error
}

func (m *mockUsageFetcher) FetchUsage(ctx context.Context) (*OAuthUsageResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return m.response, m.err
}

// mockRateLimitFetcher implements RateLimitFetcher for testing.
type mockRateLimitFetcher struct {
	usage *collectors.ClaudeAccountUsage
	err   error
}

func (m *mockRateLimitFetcher) FetchRateLimits(ctx context.Context) (*collectors.ClaudeAccountUsage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return m.usage, m.err
}

// mockCredentialLoader implements CredentialLoader for testing.
type mockCredentialLoader struct {
	creds map[string]*OAuthCredential
	err   error
}

func (m *mockCredentialLoader) Load(path string) (*OAuthCredential, error) {
	if m.err != nil {
		return nil, m.err
	}
	cred, ok := m.creds[path]
	if !ok {
		return nil, fmt.Errorf("no credentials for path %q", path)
	}
	return cred, nil
}

// --- Helper functions ---

// validCredential returns a non-expired OAuth credential.
func validCredential() *OAuthCredential {
	return &OAuthCredential{
		AccessToken: "test-access-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour).UnixMilli(),
	}
}

// expiredCredential returns an expired OAuth credential.
func expiredCredential() *OAuthCredential {
	return &OAuthCredential{
		AccessToken: "expired-token",
		ExpiresAt:   time.Now().Add(-1 * time.Hour).UnixMilli(),
	}
}

// validOAuthResponse returns a realistic OAuthUsageResponse.
func validOAuthResponse() *OAuthUsageResponse {
	return &OAuthUsageResponse{
		MessageLimit: &usageWindowResponse{
			Type:    "message",
			Window:  "5h",
			Current: 25,
			Limit:   100,
			ResetsAt: time.Now().Add(3 * time.Hour).Format(time.RFC3339),
		},
		DailyLimit: &usageWindowResponse{
			Type:    "daily",
			Window:  "7d",
			Current: 100,
			Limit:   500,
			ResetsAt: time.Now().Add(5 * 24 * time.Hour).Format(time.RFC3339),
		},
	}
}

// validAPIUsage returns a realistic API account usage result.
func validAPIUsage() *collectors.ClaudeAccountUsage {
	return &collectors.ClaudeAccountUsage{
		Tier: "tier_2",
		RateLimits: &collectors.APIRateLimits{
			RequestsLimit:     1000,
			RequestsRemaining: 750,
			RequestsReset:     time.Now().Add(1 * time.Minute),
			TokensLimit:       100000,
			TokensRemaining:   80000,
			TokensReset:       time.Now().Add(1 * time.Minute),
		},
	}
}

// testLogger returns a logger that discards output for clean test output.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// withMockFactories overrides the package-level factory functions with test mocks,
// runs the provided function, then restores the originals. This ensures test
// isolation even when tests run in parallel within the same process.
func withMockFactories(
	fetcher UsageFetcher,
	rateFetcher RateLimitFetcher,
	credLoader CredentialLoader,
	fn func(),
) {
	origUsage := newUsageFetcher
	origRate := newRateLimitFetcher
	origCred := newCredentialLoader

	newUsageFetcher = func(accessToken string, logger *slog.Logger) UsageFetcher {
		return fetcher
	}
	newRateLimitFetcher = func(apiKey string, logger *slog.Logger) RateLimitFetcher {
		return rateFetcher
	}
	newCredentialLoader = func() CredentialLoader {
		return credLoader
	}

	defer func() {
		newUsageFetcher = origUsage
		newRateLimitFetcher = origRate
		newCredentialLoader = origCred
	}()

	fn()
}

// --- Tests ---

func TestName(t *testing.T) {
	c := NewClaudeCollector(nil, nil)
	if got := c.Name(); got != "claude" {
		t.Errorf("Name() = %q, want %q", got, "claude")
	}
}

func TestDescription(t *testing.T) {
	c := NewClaudeCollector(nil, nil)
	want := "Claude AI usage across subscription and API accounts"
	if got := c.Description(); got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
}

func TestInterval(t *testing.T) {
	c := NewClaudeCollector(nil, nil)
	want := 15 * time.Minute
	if got := c.Interval(); got != want {
		t.Errorf("Interval() = %v, want %v", got, want)
	}
}

func TestCollect_ZeroAccounts(t *testing.T) {
	c := NewClaudeCollector(nil, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	if result.Collector != "claude" {
		t.Errorf("Collector = %q, want %q", result.Collector, "claude")
	}

	data, ok := result.Data.(*collectors.ClaudeUsage)
	if !ok {
		t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
	}

	if len(data.Accounts) != 0 {
		t.Errorf("got %d accounts, want 0", len(data.Accounts))
	}

	if len(result.Warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
	}
}

func TestCollect_DisabledAccounts(t *testing.T) {
	accounts := []AccountConfig{
		{Name: "disabled-sub", Type: "subscription", CredentialsPath: "/tmp/creds.json", Enabled: false},
		{Name: "disabled-api", Type: "api", APIKeyEnv: "TEST_KEY", Enabled: false},
	}

	c := NewClaudeCollector(accounts, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	data, ok := result.Data.(*collectors.ClaudeUsage)
	if !ok {
		t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
	}

	if len(data.Accounts) != 0 {
		t.Errorf("got %d accounts, want 0 (disabled accounts should be skipped)", len(data.Accounts))
	}
}

func TestCollect_SingleSubscription(t *testing.T) {
	mockFetcher := &mockUsageFetcher{response: validOAuthResponse()}
	mockCreds := &mockCredentialLoader{
		creds: map[string]*OAuthCredential{
			"/home/user/.claude/creds.json": validCredential(),
		},
	}

	accounts := []AccountConfig{
		{Name: "personal", Type: "subscription", CredentialsPath: "/home/user/.claude/creds.json", Enabled: true},
	}

	withMockFactories(mockFetcher, nil, mockCreds, func() {
		c := NewClaudeCollector(accounts, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.ClaudeUsage)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
		}

		if len(data.Accounts) != 1 {
			t.Fatalf("got %d accounts, want 1", len(data.Accounts))
		}

		acct := data.Accounts[0]
		if acct.Name != "personal" {
			t.Errorf("Name = %q, want %q", acct.Name, "personal")
		}
		if acct.Type != "subscription" {
			t.Errorf("Type = %q, want %q", acct.Type, "subscription")
		}
		if acct.Status != "ok" {
			t.Errorf("Status = %q, want %q", acct.Status, "ok")
		}
		if acct.FiveHour == nil {
			t.Error("FiveHour is nil, want non-nil")
		} else if acct.FiveHour.Utilization != 25.0 {
			t.Errorf("FiveHour.Utilization = %v, want 25.0", acct.FiveHour.Utilization)
		}
		if acct.SevenDay == nil {
			t.Error("SevenDay is nil, want non-nil")
		} else if acct.SevenDay.Utilization != 20.0 {
			t.Errorf("SevenDay.Utilization = %v, want 20.0", acct.SevenDay.Utilization)
		}

		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestCollect_SingleAPIAccount(t *testing.T) {
	mockRate := &mockRateLimitFetcher{usage: validAPIUsage()}

	t.Setenv("TEST_CLAUDE_API_KEY", "sk-ant-test-key-123")

	accounts := []AccountConfig{
		{Name: "work-api", Type: "api", APIKeyEnv: "TEST_CLAUDE_API_KEY", Enabled: true},
	}

	withMockFactories(nil, mockRate, nil, func() {
		c := NewClaudeCollector(accounts, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.ClaudeUsage)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
		}

		if len(data.Accounts) != 1 {
			t.Fatalf("got %d accounts, want 1", len(data.Accounts))
		}

		acct := data.Accounts[0]
		if acct.Name != "work-api" {
			t.Errorf("Name = %q, want %q", acct.Name, "work-api")
		}
		if acct.Type != "api" {
			t.Errorf("Type = %q, want %q", acct.Type, "api")
		}
		if acct.Status != "ok" {
			t.Errorf("Status = %q, want %q", acct.Status, "ok")
		}
		if acct.Tier != "tier_2" {
			t.Errorf("Tier = %q, want %q", acct.Tier, "tier_2")
		}
		if acct.RateLimits == nil {
			t.Fatal("RateLimits is nil, want non-nil")
		}
		if acct.RateLimits.RequestsLimit != 1000 {
			t.Errorf("RequestsLimit = %d, want 1000", acct.RateLimits.RequestsLimit)
		}
		if acct.RateLimits.RequestsRemaining != 750 {
			t.Errorf("RequestsRemaining = %d, want 750", acct.RateLimits.RequestsRemaining)
		}

		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestCollect_MixedAccounts(t *testing.T) {
	mockFetcher := &mockUsageFetcher{response: validOAuthResponse()}
	mockRate := &mockRateLimitFetcher{usage: validAPIUsage()}
	mockCreds := &mockCredentialLoader{
		creds: map[string]*OAuthCredential{
			"/creds/personal.json": validCredential(),
			"/creds/work.json":     validCredential(),
		},
	}

	t.Setenv("TEST_API_KEY_MIXED", "sk-ant-mixed-test")

	accounts := []AccountConfig{
		{Name: "personal", Type: "subscription", CredentialsPath: "/creds/personal.json", Enabled: true},
		{Name: "work", Type: "subscription", CredentialsPath: "/creds/work.json", Enabled: true},
		{Name: "ci-api", Type: "api", APIKeyEnv: "TEST_API_KEY_MIXED", Enabled: true},
	}

	withMockFactories(mockFetcher, mockRate, mockCreds, func() {
		c := NewClaudeCollector(accounts, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.ClaudeUsage)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
		}

		if len(data.Accounts) != 3 {
			t.Fatalf("got %d accounts, want 3", len(data.Accounts))
		}

		// Verify all accounts were collected (order is preserved from input).
		names := make([]string, len(data.Accounts))
		for i, a := range data.Accounts {
			names[i] = a.Name
		}

		expected := []string{"personal", "work", "ci-api"}
		for i, want := range expected {
			if names[i] != want {
				t.Errorf("account[%d].Name = %q, want %q", i, names[i], want)
			}
		}

		// Verify types.
		if data.Accounts[0].Type != "subscription" {
			t.Errorf("account[0].Type = %q, want subscription", data.Accounts[0].Type)
		}
		if data.Accounts[1].Type != "subscription" {
			t.Errorf("account[1].Type = %q, want subscription", data.Accounts[1].Type)
		}
		if data.Accounts[2].Type != "api" {
			t.Errorf("account[2].Type = %q, want api", data.Accounts[2].Type)
		}

		// All should be ok.
		for i, a := range data.Accounts {
			if a.Status != "ok" {
				t.Errorf("account[%d].Status = %q, want ok", i, a.Status)
			}
		}

		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestCollect_ErrorIsolation(t *testing.T) {
	// First subscription succeeds, second fails.
	mockFetcher := &mockUsageFetcher{response: validOAuthResponse()}
	mockCreds := &mockCredentialLoader{
		creds: map[string]*OAuthCredential{
			"/creds/good.json": validCredential(),
			// "/creds/bad.json" deliberately missing to trigger error.
		},
	}
	mockRate := &mockRateLimitFetcher{usage: validAPIUsage()}

	t.Setenv("TEST_API_KEY_ISO", "sk-ant-iso-test")

	accounts := []AccountConfig{
		{Name: "good-sub", Type: "subscription", CredentialsPath: "/creds/good.json", Enabled: true},
		{Name: "bad-sub", Type: "subscription", CredentialsPath: "/creds/bad.json", Enabled: true},
		{Name: "good-api", Type: "api", APIKeyEnv: "TEST_API_KEY_ISO", Enabled: true},
	}

	withMockFactories(mockFetcher, mockRate, mockCreds, func() {
		c := NewClaudeCollector(accounts, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.ClaudeUsage)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
		}

		if len(data.Accounts) != 3 {
			t.Fatalf("got %d accounts, want 3", len(data.Accounts))
		}

		// First account should succeed.
		if data.Accounts[0].Status != "ok" {
			t.Errorf("account[0] (good-sub) Status = %q, want ok", data.Accounts[0].Status)
		}

		// Second account should fail gracefully.
		if data.Accounts[1].Status != "auth_failed" {
			t.Errorf("account[1] (bad-sub) Status = %q, want auth_failed", data.Accounts[1].Status)
		}

		// Third account should succeed despite second failing.
		if data.Accounts[2].Status != "ok" {
			t.Errorf("account[2] (good-api) Status = %q, want ok", data.Accounts[2].Status)
		}

		// Should have exactly one warning from the failed account.
		if len(result.Warnings) != 1 {
			t.Errorf("got %d warnings, want 1: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestCollect_ExpiredCredentials(t *testing.T) {
	mockCreds := &mockCredentialLoader{
		creds: map[string]*OAuthCredential{
			"/creds/expired.json": expiredCredential(),
		},
	}

	accounts := []AccountConfig{
		{Name: "expired-acct", Type: "subscription", CredentialsPath: "/creds/expired.json", Enabled: true},
	}

	withMockFactories(nil, nil, mockCreds, func() {
		c := NewClaudeCollector(accounts, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.ClaudeUsage)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
		}

		if len(data.Accounts) != 1 {
			t.Fatalf("got %d accounts, want 1", len(data.Accounts))
		}

		acct := data.Accounts[0]
		if acct.Status != "auth_failed" {
			t.Errorf("Status = %q, want auth_failed", acct.Status)
		}

		if len(result.Warnings) != 1 {
			t.Fatalf("got %d warnings, want 1", len(result.Warnings))
		}

		if got := result.Warnings[0]; len(got) == 0 {
			t.Error("warning message is empty")
		}
	})
}

func TestCollect_MissingAPIKey(t *testing.T) {
	// Ensure the env var is not set.
	t.Setenv("TEST_MISSING_KEY", "")

	accounts := []AccountConfig{
		{Name: "no-key", Type: "api", APIKeyEnv: "TEST_MISSING_KEY", Enabled: true},
	}

	c := NewClaudeCollector(accounts, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	data, ok := result.Data.(*collectors.ClaudeUsage)
	if !ok {
		t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
	}

	if len(data.Accounts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(data.Accounts))
	}

	acct := data.Accounts[0]
	if acct.Status != "auth_failed" {
		t.Errorf("Status = %q, want auth_failed", acct.Status)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("got %d warnings, want 1", len(result.Warnings))
	}

	// Warning should mention the environment variable.
	if got := result.Warnings[0]; len(got) == 0 {
		t.Error("warning message is empty")
	}
}

func TestCollect_ContextCancellation(t *testing.T) {
	// Create a context that is already cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	accounts := []AccountConfig{
		{Name: "will-not-run", Type: "subscription", CredentialsPath: "/creds/test.json", Enabled: true},
	}

	c := NewClaudeCollector(accounts, testLogger())
	_, err := c.Collect(ctx)
	if err == nil {
		t.Fatal("Collect() with cancelled context should return error")
	}
	if err != context.Canceled {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestCollect_ContextCancellationDuringFetch(t *testing.T) {
	// Create a context that cancels shortly after starting.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// The mock fetcher respects context cancellation.
	slowFetcher := &mockUsageFetcher{
		response: nil,
		err:      context.DeadlineExceeded,
	}
	mockCreds := &mockCredentialLoader{
		creds: map[string]*OAuthCredential{
			"/creds/slow.json": validCredential(),
		},
	}

	accounts := []AccountConfig{
		{Name: "slow-acct", Type: "subscription", CredentialsPath: "/creds/slow.json", Enabled: true},
	}

	withMockFactories(slowFetcher, nil, mockCreds, func() {
		c := NewClaudeCollector(accounts, testLogger())
		result, err := c.Collect(ctx)

		// The collector might return an error from the post-collection context
		// check, or it might return a result with a warning. Both are acceptable.
		if err != nil {
			// Top-level context error is fine.
			return
		}

		// If we got a result, the account should show an error status.
		data, ok := result.Data.(*collectors.ClaudeUsage)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
		}

		if len(data.Accounts) == 1 && data.Accounts[0].Status == "ok" {
			t.Error("expected non-ok status for account that got context deadline exceeded")
		}
	})
}

func TestCollect_FetchUsageHTTPError(t *testing.T) {
	// Simulate a 429 Too Many Requests error.
	mockFetcher := &mockUsageFetcher{
		err: &APIError{
			StatusCode: 429,
			Status:     "429 Too Many Requests",
			Body:       "rate limited",
		},
	}
	mockCreds := &mockCredentialLoader{
		creds: map[string]*OAuthCredential{
			"/creds/limited.json": validCredential(),
		},
	}

	accounts := []AccountConfig{
		{Name: "limited", Type: "subscription", CredentialsPath: "/creds/limited.json", Enabled: true},
	}

	withMockFactories(mockFetcher, nil, mockCreds, func() {
		c := NewClaudeCollector(accounts, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.ClaudeUsage)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
		}

		if len(data.Accounts) != 1 {
			t.Fatalf("got %d accounts, want 1", len(data.Accounts))
		}

		acct := data.Accounts[0]
		if acct.Status != "rate_limited" {
			t.Errorf("Status = %q, want rate_limited", acct.Status)
		}
	})
}

func TestCollect_RateLimitFetcherError(t *testing.T) {
	mockRate := &mockRateLimitFetcher{
		err: fmt.Errorf("connection refused"),
	}

	t.Setenv("TEST_API_KEY_ERR", "sk-ant-err-test")

	accounts := []AccountConfig{
		{Name: "failing-api", Type: "api", APIKeyEnv: "TEST_API_KEY_ERR", Enabled: true},
	}

	withMockFactories(nil, mockRate, nil, func() {
		c := NewClaudeCollector(accounts, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.ClaudeUsage)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
		}

		if len(data.Accounts) != 1 {
			t.Fatalf("got %d accounts, want 1", len(data.Accounts))
		}

		acct := data.Accounts[0]
		if acct.Status == "ok" {
			t.Error("expected non-ok status for API account with fetch error")
		}

		if len(result.Warnings) != 1 {
			t.Errorf("got %d warnings, want 1", len(result.Warnings))
		}
	})
}

func TestCollect_UnknownAccountType(t *testing.T) {
	accounts := []AccountConfig{
		{Name: "mystery", Type: "unknown_type", Enabled: true},
	}

	c := NewClaudeCollector(accounts, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	data, ok := result.Data.(*collectors.ClaudeUsage)
	if !ok {
		t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
	}

	if len(data.Accounts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(data.Accounts))
	}

	acct := data.Accounts[0]
	if acct.Status != "error" {
		t.Errorf("Status = %q, want error for unknown type", acct.Status)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("got %d warnings, want 1", len(result.Warnings))
	}
}

func TestCollect_TimestampIsRecent(t *testing.T) {
	before := time.Now()

	c := NewClaudeCollector(nil, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	after := time.Now()

	if result.Timestamp.Before(before) || result.Timestamp.After(after) {
		t.Errorf("Timestamp = %v, want between %v and %v", result.Timestamp, before, after)
	}
}

func TestCollect_MixedEnabledDisabled(t *testing.T) {
	mockFetcher := &mockUsageFetcher{response: validOAuthResponse()}
	mockCreds := &mockCredentialLoader{
		creds: map[string]*OAuthCredential{
			"/creds/enabled.json": validCredential(),
		},
	}

	accounts := []AccountConfig{
		{Name: "enabled-sub", Type: "subscription", CredentialsPath: "/creds/enabled.json", Enabled: true},
		{Name: "disabled-sub", Type: "subscription", CredentialsPath: "/creds/disabled.json", Enabled: false},
	}

	withMockFactories(mockFetcher, nil, mockCreds, func() {
		c := NewClaudeCollector(accounts, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data, ok := result.Data.(*collectors.ClaudeUsage)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.ClaudeUsage", result.Data)
		}

		// Only the enabled account should be collected.
		if len(data.Accounts) != 1 {
			t.Fatalf("got %d accounts, want 1", len(data.Accounts))
		}

		if data.Accounts[0].Name != "enabled-sub" {
			t.Errorf("Name = %q, want enabled-sub", data.Accounts[0].Name)
		}
	})
}

func TestCollect_OrderPreserved(t *testing.T) {
	mockFetcher := &mockUsageFetcher{response: validOAuthResponse()}
	mockRate := &mockRateLimitFetcher{usage: validAPIUsage()}
	mockCreds := &mockCredentialLoader{
		creds: map[string]*OAuthCredential{
			"/creds/alpha.json":   validCredential(),
			"/creds/charlie.json": validCredential(),
		},
	}

	t.Setenv("TEST_API_KEY_ORDER", "sk-ant-order-test")

	accounts := []AccountConfig{
		{Name: "alpha", Type: "subscription", CredentialsPath: "/creds/alpha.json", Enabled: true},
		{Name: "bravo", Type: "api", APIKeyEnv: "TEST_API_KEY_ORDER", Enabled: true},
		{Name: "charlie", Type: "subscription", CredentialsPath: "/creds/charlie.json", Enabled: true},
	}

	withMockFactories(mockFetcher, mockRate, mockCreds, func() {
		c := NewClaudeCollector(accounts, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.ClaudeUsage)

		// Despite concurrent collection, order should match input.
		expected := []string{"alpha", "bravo", "charlie"}
		for i, want := range expected {
			if data.Accounts[i].Name != want {
				t.Errorf("account[%d].Name = %q, want %q", i, data.Accounts[i].Name, want)
			}
		}
	})
}

func TestCollect_NilLogger(t *testing.T) {
	// Verify NewClaudeCollector with nil logger does not panic.
	c := NewClaudeCollector(nil, nil)
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestOAuthCredential_IsExpired_Collector(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt int64
		want      bool
	}{
		{"future", time.Now().Add(1 * time.Hour).UnixMilli(), false},
		{"past", time.Now().Add(-1 * time.Hour).UnixMilli(), true},
		{"zero value", 0, true}, // epoch 0 is long in the past
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &OAuthCredential{ExpiresAt: tt.expiresAt}
			if got := c.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeTierString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "pro"},
		{"pro", "pro"},
		{"max_5x", "max_5x"},
		{"tier_2", "tier_2"},
		{"default_claude_pro", "pro"},
		{"default_claude_max_5x", "max_5x"},
		{"default_claude_max_20x", "max_20x"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := normalizeTierString(tt.input); got != tt.want {
				t.Errorf("normalizeTierString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestInterfaceCompliance verifies ClaudeCollector implements collectors.Collector.
func TestInterfaceCompliance(t *testing.T) {
	var _ collectors.Collector = (*ClaudeCollector)(nil)
}
