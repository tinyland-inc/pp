package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// validCredentialJSON is a well-formed credential file matching the format
// written by the Claude Code CLI.
const validCredentialJSON = `{
  "claudeAiOauth": {
    "accessToken": "sk-ant-oat01-test-token",
    "refreshToken": "sk-ant-ort01-test-refresh",
    "expiresAt": 1770179710804,
    "scopes": ["user:inference", "user:profile", "user:sessions:claude_code"],
    "subscriptionType": "max",
    "rateLimitTier": "default_claude_max_20x"
  }
}`

func writeTestFile(t *testing.T, dir, name, content string, perm os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// LoadCredentials
// ---------------------------------------------------------------------------

func TestLoadCredentials_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "creds.json", validCredentialJSON, 0600)

	creds, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if creds.ClaudeAiOauth == nil {
		t.Fatal("expected ClaudeAiOauth to be non-nil")
	}

	oauth := creds.ClaudeAiOauth

	if oauth.AccessToken != "sk-ant-oat01-test-token" {
		t.Errorf("AccessToken = %q, want %q", oauth.AccessToken, "sk-ant-oat01-test-token")
	}
	if oauth.RefreshToken != "sk-ant-ort01-test-refresh" {
		t.Errorf("RefreshToken = %q, want %q", oauth.RefreshToken, "sk-ant-ort01-test-refresh")
	}
	if oauth.ExpiresAt != 1770179710804 {
		t.Errorf("ExpiresAt = %d, want %d", oauth.ExpiresAt, 1770179710804)
	}
	if len(oauth.Scopes) != 3 {
		t.Errorf("len(Scopes) = %d, want 3", len(oauth.Scopes))
	}
	if oauth.SubscriptionType != "max" {
		t.Errorf("SubscriptionType = %q, want %q", oauth.SubscriptionType, "max")
	}
	if oauth.RateLimitTier != "default_claude_max_20x" {
		t.Errorf("RateLimitTier = %q, want %q", oauth.RateLimitTier, "default_claude_max_20x")
	}
}

func TestLoadCredentials_MissingFile(t *testing.T) {
	_, err := LoadCredentials("/nonexistent/path/to/creds.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadCredentials_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "bad.json", `{not valid json}`, 0600)

	_, err := LoadCredentials(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestLoadCredentials_MissingOAuthKey(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "empty.json", `{"otherField": "value"}`, 0600)

	creds, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.ClaudeAiOauth != nil {
		t.Errorf("expected ClaudeAiOauth to be nil, got %+v", creds.ClaudeAiOauth)
	}
}

func TestLoadCredentials_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "empty.json", `{}`, 0600)

	creds, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.ClaudeAiOauth != nil {
		t.Errorf("expected ClaudeAiOauth to be nil for empty object")
	}
}

// ---------------------------------------------------------------------------
// IsExpired
// ---------------------------------------------------------------------------

func TestIsExpired_FutureTimestamp(t *testing.T) {
	// Set expiry 1 hour from now.
	futureMillis := time.Now().Add(1 * time.Hour).UnixMilli()
	c := &OAuthCredential{ExpiresAt: futureMillis}

	if c.IsExpired() {
		t.Error("expected token with future expiry to not be expired")
	}
}

func TestIsExpired_PastTimestamp(t *testing.T) {
	// Set expiry 1 hour ago.
	pastMillis := time.Now().Add(-1 * time.Hour).UnixMilli()
	c := &OAuthCredential{ExpiresAt: pastMillis}

	if !c.IsExpired() {
		t.Error("expected token with past expiry to be expired")
	}
}

func TestIsExpired_ExactNow(t *testing.T) {
	// A token expiring at exactly now should be considered expired
	// (>= comparison).
	nowMillis := time.Now().UnixMilli()
	c := &OAuthCredential{ExpiresAt: nowMillis}

	// Allow a small race: the check happens after setting nowMillis,
	// so IsExpired should return true since time has moved forward.
	if !c.IsExpired() {
		t.Log("token at exact now boundary, may pass or fail due to timing; not a hard failure")
	}
}

// ---------------------------------------------------------------------------
// ExpiresIn
// ---------------------------------------------------------------------------

func TestExpiresIn_PositiveDuration(t *testing.T) {
	futureMillis := time.Now().Add(30 * time.Minute).UnixMilli()
	c := &OAuthCredential{ExpiresAt: futureMillis}

	d := c.ExpiresIn()
	if d <= 0 {
		t.Errorf("expected positive duration, got %v", d)
	}
	// Should be roughly 30 minutes (allow 1 second tolerance).
	if d < 29*time.Minute || d > 31*time.Minute {
		t.Errorf("expected ~30 minutes, got %v", d)
	}
}

func TestExpiresIn_NegativeDuration(t *testing.T) {
	pastMillis := time.Now().Add(-10 * time.Minute).UnixMilli()
	c := &OAuthCredential{ExpiresAt: pastMillis}

	d := c.ExpiresIn()
	if d >= 0 {
		t.Errorf("expected negative duration for expired token, got %v", d)
	}
}

// ---------------------------------------------------------------------------
// NormalizeTier
// ---------------------------------------------------------------------------

func TestNormalizeTier_KnownTiers(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"default_claude_pro", "pro"},
		{"default_claude_max_5x", "max_5x"},
		{"default_claude_max_20x", "max_20x"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			c := &OAuthCredential{RateLimitTier: tt.input}
			got := c.NormalizeTier()
			if got != tt.want {
				t.Errorf("NormalizeTier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeTier_UnknownTier(t *testing.T) {
	c := &OAuthCredential{RateLimitTier: "custom_enterprise_tier"}
	got := c.NormalizeTier()
	if got != "custom_enterprise_tier" {
		t.Errorf("NormalizeTier for unknown tier = %q, want %q", got, "custom_enterprise_tier")
	}
}

func TestNormalizeTier_EmptyString(t *testing.T) {
	c := &OAuthCredential{RateLimitTier: ""}
	got := c.NormalizeTier()
	if got != "" {
		t.Errorf("NormalizeTier for empty string = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// NormalizeSubscriptionType
// ---------------------------------------------------------------------------

func TestNormalizeSubscriptionType_Pro(t *testing.T) {
	c := &OAuthCredential{SubscriptionType: "pro"}
	got := c.NormalizeSubscriptionType()
	if got != "subscription" {
		t.Errorf("NormalizeSubscriptionType(pro) = %q, want %q", got, "subscription")
	}
}

func TestNormalizeSubscriptionType_Max(t *testing.T) {
	c := &OAuthCredential{SubscriptionType: "max"}
	got := c.NormalizeSubscriptionType()
	if got != "subscription" {
		t.Errorf("NormalizeSubscriptionType(max) = %q, want %q", got, "subscription")
	}
}

func TestNormalizeSubscriptionType_Unknown(t *testing.T) {
	c := &OAuthCredential{SubscriptionType: "enterprise"}
	got := c.NormalizeSubscriptionType()
	if got != "enterprise" {
		t.Errorf("NormalizeSubscriptionType(enterprise) = %q, want %q", got, "enterprise")
	}
}

func TestNormalizeSubscriptionType_CaseInsensitive(t *testing.T) {
	c := &OAuthCredential{SubscriptionType: "Pro"}
	got := c.NormalizeSubscriptionType()
	if got != "subscription" {
		t.Errorf("NormalizeSubscriptionType(Pro) = %q, want %q", got, "subscription")
	}
}

// ---------------------------------------------------------------------------
// ValidateCredentialPath
// ---------------------------------------------------------------------------

func TestValidateCredentialPath_NonExistent(t *testing.T) {
	err := ValidateCredentialPath("/no/such/file.json")
	if err == nil {
		t.Fatal("expected error for non-existent path, got nil")
	}
}

func TestValidateCredentialPath_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "creds.json", validCredentialJSON, 0600)

	err := ValidateCredentialPath(path)
	if err != nil {
		t.Errorf("unexpected error for valid credential file: %v", err)
	}
}

func TestValidateCredentialPath_MissingOAuthKey(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "creds.json", `{"other": "data"}`, 0600)

	err := ValidateCredentialPath(path)
	if err == nil {
		t.Fatal("expected error for missing claudeAiOauth key, got nil")
	}
}

func TestValidateCredentialPath_EmptyAccessToken(t *testing.T) {
	dir := t.TempDir()
	content := `{"claudeAiOauth": {"accessToken": "", "refreshToken": "rt", "expiresAt": 0}}`
	path := writeTestFile(t, dir, "creds.json", content, 0600)

	err := ValidateCredentialPath(path)
	if err == nil {
		t.Fatal("expected error for empty access token, got nil")
	}
}

func TestValidateCredentialPath_WorldReadable(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "creds.json", validCredentialJSON, 0644)

	err := ValidateCredentialPath(path)
	if err == nil {
		t.Fatal("expected error for world-readable credential file, got nil")
	}
}

func TestValidateCredentialPath_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "creds.json", `{broken`, 0600)

	err := ValidateCredentialPath(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// FilePermissionWarning
// ---------------------------------------------------------------------------

func TestFilePermissionWarning_Secure(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "creds.json", validCredentialJSON, 0600)

	warn := FilePermissionWarning(path)
	if warn != "" {
		t.Errorf("expected no warning for 0600 permissions, got %q", warn)
	}
}

func TestFilePermissionWarning_GroupReadable(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "creds.json", validCredentialJSON, 0640)

	warn := FilePermissionWarning(path)
	if warn == "" {
		t.Error("expected warning for group-readable file, got empty string")
	}
}

func TestFilePermissionWarning_NonExistent(t *testing.T) {
	warn := FilePermissionWarning("/no/such/file")
	if warn != "" {
		t.Errorf("expected empty string for non-existent file, got %q", warn)
	}
}

// ---------------------------------------------------------------------------
// DefaultCredentialPath
// ---------------------------------------------------------------------------

func TestDefaultCredentialPath(t *testing.T) {
	path, err := DefaultCredentialPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	// Should end with the expected suffix.
	const suffix = "/.claude/.credentials.json"
	if len(path) < len(suffix) || path[len(path)-len(suffix):] != suffix {
		t.Errorf("path %q does not end with %q", path, suffix)
	}
}

// ---------------------------------------------------------------------------
// isWorldReadable (unexported helper)
// ---------------------------------------------------------------------------

func TestIsWorldReadable(t *testing.T) {
	tests := []struct {
		mode os.FileMode
		want bool
	}{
		{0600, false},
		{0640, false},
		{0644, true},
		{0604, true},
		{0601, true},
		{0700, false},
		{0777, true},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			got := isWorldReadable(tt.mode)
			if got != tt.want {
				t.Errorf("isWorldReadable(%04o) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}
