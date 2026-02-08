package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors/claude"
)

// runClaudeDiagnostics performs comprehensive diagnostics on Claude credentials
// and API connectivity, providing actionable feedback for users.
func runClaudeDiagnostics() {
	fmt.Println("🔍 Claude Code Diagnostics")
	fmt.Println("============================================================")
	fmt.Println()

	// Check credential file existence
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("❌ Failed to get home directory: %v\n", err)
		return
	}

	credPath := filepath.Join(homeDir, ".claude", ".credentials.json")
	fmt.Printf("📁 Credential file: %s\n", credPath)

	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		fmt.Println("   ❌ File not found")
		fmt.Println()
		fmt.Println("💡 Solution: Run 'claude login' to authenticate")
		return
	}
	fmt.Println("   ✅ File exists")

	// Read and parse credentials
	data, err := os.ReadFile(credPath)
	if err != nil {
		fmt.Printf("   ⚠️  Cannot read file: %v\n", err)
		return
	}

	// Simple JSON parsing for OAuth credentials
	type oauthCreds struct {
		ClaudeAIOAuth struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresAt    int64  `json:"expiresAt"` // Unix timestamp in milliseconds
		} `json:"claudeAiOauth"`
	}

	var creds oauthCreds
	if err := json.Unmarshal(data, &creds); err != nil {
		fmt.Printf("   ⚠️  Cannot parse JSON: %v\n", err)
		return
	}

	fmt.Println()

	// Check OAuth credentials
	if creds.ClaudeAIOAuth.AccessToken == "" {
		fmt.Println("🔑 OAuth Credentials: ❌ Not found")
		fmt.Println()
		fmt.Println("💡 Solution: Run 'claude login' to authenticate")
		return
	}

	fmt.Println("🔑 OAuth Credentials")
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("   Access Token:  ✅ Present (%d chars)\n", len(creds.ClaudeAIOAuth.AccessToken))

	if creds.ClaudeAIOAuth.RefreshToken == "" {
		fmt.Println("   Refresh Token: ❌ Empty")
	} else {
		fmt.Printf("   Refresh Token: ✅ Present (%d chars)\n", len(creds.ClaudeAIOAuth.RefreshToken))
	}

	// Check token expiration
	if creds.ClaudeAIOAuth.ExpiresAt == 0 {
		fmt.Println("   Expiration:    ⚠️  Not set")
	} else {
		expiresAt := time.UnixMilli(creds.ClaudeAIOAuth.ExpiresAt)
		now := time.Now()
		timeUntil := expiresAt.Sub(now)

		if timeUntil < 0 {
			fmt.Printf("   Expiration:    ❌ EXPIRED (%s ago)\n", formatDiagDuration(-timeUntil))
			fmt.Println()
			fmt.Println("💡 Solution: Run 'claude login' to refresh your token")
		} else if timeUntil < 1*time.Hour {
			fmt.Printf("   Expiration:    ⚠️  Soon (%s remaining)\n", formatDiagDuration(timeUntil))
			fmt.Println()
			fmt.Println("💡 Tip: Token expires soon, consider refreshing")
		} else {
			fmt.Printf("   Expiration:    ✅ Valid (%s remaining)\n", formatDiagDuration(timeUntil))
		}
	}

	fmt.Println()

	// Test API connectivity
	fmt.Println("🌐 API Connectivity")
	fmt.Println("------------------------------------------------------------")

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn, // Only show warnings during diagnostics
	}))

	client := claude.NewOAuthClient(creds.ClaudeAIOAuth.AccessToken, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Print("   Testing connection... ")

	usage, err := client.FetchUsage(ctx)
	if err != nil {
		fmt.Println("❌ FAILED")
		fmt.Println()

		// Provide specific error diagnostics
		status := claude.StatusFromError(err)
		switch status {
		case "auth_failed":
			fmt.Println("   Error: Authentication failed")
			fmt.Printf("   Details: %v\n", err)
			fmt.Println()
			fmt.Println("💡 Solution: Run 'claude login' to re-authenticate")

		case "rate_limited":
			fmt.Println("   Error: Rate limited by Claude API")
			fmt.Printf("   Details: %v\n", err)
			fmt.Println()
			fmt.Println("💡 Solution: Wait a few minutes and try again")

		case "cloudflare":
			fmt.Println("   Error: Cloudflare protection active")
			fmt.Printf("   Details: %v\n", err)
			fmt.Println()
			fmt.Println("💡 Note: Usage API may be protected by Cloudflare")
			fmt.Println("   This is expected. prompt-pulse will fall back to credentials-only mode.")

		case "network_error":
			fmt.Println("   Error: Network connectivity issue")
			fmt.Printf("   Details: %v\n", err)
			fmt.Println()
			fmt.Println("💡 Solution: Check your internet connection and try again")

		default:
			fmt.Println("   Error: Unknown error")
			fmt.Printf("   Details: %v\n", err)
			fmt.Println()
			fmt.Println("💡 Solution: Check logs or try 'claude login' to re-authenticate")
		}
		return
	}

	fmt.Println("✅ SUCCESS")
	fmt.Println()

	// Display usage data
	fmt.Println("📊 Usage Data Retrieved")
	fmt.Println("------------------------------------------------------------")
	if usage.MessageLimit != nil {
		utilization := (usage.MessageLimit.Current / usage.MessageLimit.Limit) * 100
		fmt.Printf("   Message limit:  %.0f/%.0f (%.1f%% utilization)\n",
			usage.MessageLimit.Current, usage.MessageLimit.Limit, utilization)
		if usage.MessageLimit.ResetsAt != "" {
			if resetTime, err := time.Parse(time.RFC3339, usage.MessageLimit.ResetsAt); err == nil {
				fmt.Printf("   Resets in:      %s\n", formatDiagDuration(time.Until(resetTime)))
			}
		}
	}
	if usage.DailyLimit != nil {
		utilization := (usage.DailyLimit.Current / usage.DailyLimit.Limit) * 100
		fmt.Printf("   Daily limit:    %.0f/%.0f (%.1f%% utilization)\n",
			usage.DailyLimit.Current, usage.DailyLimit.Limit, utilization)
	}

	fmt.Println()
	fmt.Println("✨ All diagnostics passed! prompt-pulse should work correctly.")
}

// runBillingProviderCheck validates billing provider API keys.
func runBillingProviderCheck() {
	fmt.Println("💰 Billing Provider Configuration Check")
	fmt.Println("======================================================================")
	fmt.Println()

	type provider struct {
		Name        string
		EnvVar      string
		FileVar     string
		Description string
	}

	providers := []provider{
		{"Civo", "CIVO_API_KEY", "CIVO_API_KEY_FILE", "Kubernetes cloud provider"},
		{"DigitalOcean", "DIGITALOCEAN_TOKEN", "DIGITALOCEAN_TOKEN_FILE", "Cloud infrastructure"},
		{"DreamHost", "DREAMHOST_API_KEY", "DREAMHOST_API_KEY_FILE", "Web hosting"},
		{"AWS", "AWS_PROFILE", "", "Amazon Web Services (uses AWS CLI credentials)"},
	}

	var configured, missing int

	for _, p := range providers {
		fmt.Printf("📦 %s (%s)\n", p.Name, p.Description)
		fmt.Println("----------------------------------------------------------------------")

		// Check direct environment variable
		apiKey := os.Getenv(p.EnvVar)
		if apiKey != "" {
			fmt.Printf("   %s: ✅ Set (%d chars)\n", p.EnvVar, len(apiKey))
			configured++
			fmt.Println()
			continue
		}

		// Check file-based variant (sops-nix pattern)
		if p.FileVar != "" {
			filePath := os.Getenv(p.FileVar)
			if filePath != "" {
				if data, err := os.ReadFile(filePath); err == nil && len(data) > 0 {
					fmt.Printf("   %s: ✅ Set (via %s)\n", p.EnvVar, p.FileVar)
					fmt.Printf("   File: %s (%d bytes)\n", filePath, len(data))
					configured++
					fmt.Println()
					continue
				}
			}
		}

		// Special handling for AWS
		if p.Name == "AWS" {
			homeDir, _ := os.UserHomeDir()
			awsCredsFile := filepath.Join(homeDir, ".aws", "credentials")
			if _, err := os.Stat(awsCredsFile); err == nil {
				fmt.Printf("   %s: ✅ AWS credentials file exists\n", p.EnvVar)
				fmt.Printf("   File: %s\n", awsCredsFile)
				configured++
				fmt.Println()
				continue
			}
		}

		// Not configured
		fmt.Printf("   %s: ❌ Not set\n", p.EnvVar)
		if p.FileVar != "" {
			fmt.Printf("   %s: ❌ Not set\n", p.FileVar)
		}
		missing++
		fmt.Println()
		fmt.Printf("   💡 To configure: export %s='your-api-key'\n", p.EnvVar)
		if p.FileVar != "" {
			fmt.Printf("   💡 Or (sops-nix): export %s='/path/to/secret/file'\n", p.FileVar)
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("======================================================================")
	fmt.Printf("Summary: %d/%d providers configured\n", configured, len(providers))
	fmt.Println()

	if missing > 0 {
		fmt.Println("⚠️  Some providers are missing API keys")
		fmt.Println()
		fmt.Println("Why this matters:")
		fmt.Println("  • Providers without API keys will show Status=\"error\"")
		fmt.Println("  • Failed providers are excluded from billing totals")
		fmt.Println("  • If ALL providers fail, banner shows \"$0 this month\"")
		fmt.Println()
		fmt.Println("To fix:")
		fmt.Println("  1. Set environment variables for providers you use")
		fmt.Println("  2. Restart prompt-pulse daemon: systemctl --user restart prompt-pulse")
		fmt.Println("  3. Check banner: prompt-pulse --banner")
	} else {
		fmt.Println("✅ All billing providers are configured!")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  • Check billing data: prompt-pulse --banner")
		fmt.Println("  • View details in TUI: prompt-pulse")
		fmt.Println("  • Monitor daemon logs: journalctl --user -u prompt-pulse -f")
	}
}

// formatDiagDuration formats a duration for diagnostic output.
func formatDiagDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// testAPIConnectivity performs a real API test to diagnose connectivity issues.
func testAPIConnectivity() error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", "https://claude.ai/api/auth/usage", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("rate limited (HTTP 429)")
	}

	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error (HTTP %d)", resp.StatusCode)
	}

	return nil
}
