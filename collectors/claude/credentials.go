package claude

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"
)

// CredentialFile represents the on-disk credential format stored at
// ~/.claude/.credentials.json by the Claude Code CLI.
type CredentialFile struct {
	ClaudeAiOauth *OAuthCredential `json:"claudeAiOauth"`
}

// OAuthCredential holds the raw OAuth token data as stored on disk.
// The ExpiresAt field is a Unix timestamp in milliseconds, matching the
// format written by the Claude Code CLI. Use ToOAuthCredential to convert
// this to the runtime OAuthCredential type used by the collector.
type OAuthCredential struct {
	AccessToken      string   `json:"accessToken"`
	RefreshToken     string   `json:"refreshToken"`
	ExpiresAt        int64    `json:"expiresAt"` // Unix timestamp in milliseconds
	Scopes           []string `json:"scopes"`
	SubscriptionType string   `json:"subscriptionType"`
	RateLimitTier    string   `json:"rateLimitTier"`
}

// tierMapping maps raw rateLimitTier strings from the credential file to
// the normalized short-form tier names used throughout prompt-pulse.
var tierMapping = map[string]string{
	"default_claude_pro":     "pro",
	"default_claude_max_5x":  "max_5x",
	"default_claude_max_20x": "max_20x",
}

// LoadCredentials reads and parses a Claude credential JSON file from the
// given path. It returns an error if the file cannot be read or contains
// invalid JSON. A nil ClaudeAiOauth field is not treated as an error at
// this stage; callers should check for it explicitly.
func LoadCredentials(path string) (*CredentialFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading credential file: %w", err)
	}

	var creds CredentialFile
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parsing credential file: %w", err)
	}

	return &creds, nil
}

// IsExpired reports whether the access token has expired. The ExpiresAt
// field (in milliseconds) is compared against the current time.
func (j *OAuthCredential) IsExpired() bool {
	return time.Now().UnixMilli() >= j.ExpiresAt
}

// ExpiresIn returns the duration until the access token expires. If the
// token has already expired, the returned duration is negative.
func (j *OAuthCredential) ExpiresIn() time.Duration {
	expiresTime := time.UnixMilli(j.ExpiresAt)
	return time.Until(expiresTime)
}

// NormalizeTier converts the raw rateLimitTier string from the credential
// file to a short-form tier name. Known tiers are mapped as follows:
//
//   - "default_claude_pro"     -> "pro"
//   - "default_claude_max_5x"  -> "max_5x"
//   - "default_claude_max_20x" -> "max_20x"
//
// Unknown tier strings are returned unchanged.
func (j *OAuthCredential) NormalizeTier() string {
	if normalized, ok := tierMapping[j.RateLimitTier]; ok {
		return normalized
	}
	return j.RateLimitTier
}

// NormalizeSubscriptionType returns a canonical subscription type string.
// Both "pro" and "max" subscription types are normalized to "subscription".
// Unknown types are returned unchanged.
func (j *OAuthCredential) NormalizeSubscriptionType() string {
	lower := strings.ToLower(j.SubscriptionType)
	switch lower {
	case "pro", "max":
		return "subscription"
	default:
		return j.SubscriptionType
	}
}

// ValidateCredentialPath checks that the file at path exists, is readable,
// contains valid JSON with the expected structure, and warns about insecure
// file permissions. It returns an error describing the first problem found.
func ValidateCredentialPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("credential file does not exist: %s", path)
		}
		return fmt.Errorf("cannot stat credential file: %w", err)
	}

	// Check for world-readable permissions (last octet non-zero).
	mode := info.Mode().Perm()
	if mode&0o007 != 0 {
		return fmt.Errorf("credential file %s has insecure permissions %04o: world-readable", path, mode)
	}

	creds, err := LoadCredentials(path)
	if err != nil {
		return err
	}

	if creds.ClaudeAiOauth == nil {
		return fmt.Errorf("credential file missing claudeAiOauth key")
	}

	if creds.ClaudeAiOauth.AccessToken == "" {
		return fmt.Errorf("credential file has empty access token")
	}

	return nil
}

// FilePermissionWarning checks whether the credential file at path has
// permissions that are too open. It returns a human-readable warning
// string if the file is group- or world-readable, or empty string if
// permissions are acceptable. This is a non-fatal check suitable for
// inclusion in CollectResult.Warnings.
func FilePermissionWarning(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}

	mode := info.Mode().Perm()
	if mode&0o077 != 0 {
		return fmt.Sprintf("credential file %s has loose permissions %04o; recommend 0600", path, mode)
	}

	return ""
}

// DefaultCredentialPath returns the default path to the Claude credential
// file, expanding the user's home directory.
func DefaultCredentialPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return home + "/.claude/.credentials.json", nil
}

// fileCredentialLoader implements the CredentialLoader interface using
// LoadCredentials to read from the filesystem.
type fileCredentialLoader struct{}

// Load reads the credential file at path and returns the OAuthCredential
// for use by the collector. It returns an error if the file is missing,
// malformed, or lacks the claudeAiOauth key.
func (f *fileCredentialLoader) Load(path string) (*OAuthCredential, error) {
	creds, err := LoadCredentials(path)
	if err != nil {
		return nil, err
	}

	if creds.ClaudeAiOauth == nil {
		return nil, fmt.Errorf("credential file missing claudeAiOauth key")
	}

	return creds.ClaudeAiOauth, nil
}

// isWorldReadable is a helper that checks whether a file's permissions
// allow any access to "other" users.
func isWorldReadable(mode fs.FileMode) bool {
	return mode.Perm()&0o007 != 0
}
