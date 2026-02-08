package tui

import (
	"strings"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

func TestRenderClaudeContent_Nil(t *testing.T) {
	result := renderClaudeContent(nil, 80, 24)
	if result != "No Claude usage data available" {
		t.Errorf("expected placeholder for nil data, got: %s", result)
	}
}

func TestRenderClaudeContent_Empty(t *testing.T) {
	data := &collectors.ClaudeUsage{Accounts: []collectors.ClaudeAccountUsage{}}
	result := renderClaudeContent(data, 80, 24)
	if result != "No Claude usage data available" {
		t.Errorf("expected placeholder for empty accounts, got: %s", result)
	}
}

func TestRenderClaudeContent_SubscriptionOK(t *testing.T) {
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "personal",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 45.0,
					ResetsAt:    time.Now().Add(30 * time.Minute),
				},
				SevenDay: &collectors.UsagePeriod{
					Utilization: 72.0,
					ResetsAt:    time.Now().Add(3 * time.Hour),
				},
			},
		},
	}

	result := renderClaudeContent(data, 80, 24)

	// Should contain account name.
	if !strings.Contains(result, "personal") {
		t.Error("expected account name 'personal' in output")
	}

	// Should contain type badge.
	if !strings.Contains(result, "SUB") {
		t.Error("expected SUB badge in output")
	}

	// Should contain tier.
	if !strings.Contains(result, "pro") {
		t.Error("expected tier 'pro' in output")
	}

	// Should contain gauge bar characters.
	if !strings.Contains(result, "\u2588") && !strings.Contains(result, "\u2591") {
		t.Error("expected gauge bar characters in output")
	}

	// Should contain usage labels.
	if !strings.Contains(result, "5h usage") {
		t.Error("expected '5h usage' label")
	}
	if !strings.Contains(result, "7d usage") {
		t.Error("expected '7d usage' label")
	}

	// Should contain percentage.
	if !strings.Contains(result, "45%") {
		t.Error("expected '45%' in output")
	}

	// Should contain section title.
	if !strings.Contains(result, "Claude AI Usage") {
		t.Error("expected 'Claude AI Usage' title")
	}
}

func TestRenderClaudeContent_APIAccount(t *testing.T) {
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "work-api",
				Type:   "api",
				Tier:   "tier_2",
				Status: "ok",
				RateLimits: &collectors.APIRateLimits{
					RequestsLimit:     1000,
					RequestsRemaining: 750,
					RequestsReset:     time.Now().Add(15 * time.Minute),
					TokensLimit:       100000,
					TokensRemaining:   60000,
					TokensReset:       time.Now().Add(15 * time.Minute),
				},
			},
		},
	}

	result := renderClaudeContent(data, 80, 24)

	// Should contain API badge.
	if !strings.Contains(result, "API") {
		t.Error("expected API badge in output")
	}

	// Should contain request/token labels.
	if !strings.Contains(result, "Requests") {
		t.Error("expected 'Requests' label")
	}
	if !strings.Contains(result, "Tokens") {
		t.Error("expected 'Tokens' label")
	}

	// Should show used/limit counts.
	if !strings.Contains(result, "250 / 1000") {
		t.Error("expected '250 / 1000' requests used")
	}
	if !strings.Contains(result, "40000 / 100000") {
		t.Error("expected '40000 / 100000' tokens used")
	}
}

func TestRenderClaudeContent_AuthFailed(t *testing.T) {
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "broken",
				Type:   "subscription",
				Tier:   "pro",
				Status: "auth_failed",
			},
		},
	}

	result := renderClaudeContent(data, 80, 24)

	// Should contain the account name.
	if !strings.Contains(result, "broken") {
		t.Error("expected account name 'broken' in output")
	}

	// Should contain auth_failed status text.
	if !strings.Contains(result, "auth_failed") {
		t.Error("expected 'auth_failed' status in output")
	}
}

func TestRenderClaudeContent_MultipleAccounts(t *testing.T) {
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "personal",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 20.0,
					ResetsAt:    time.Now().Add(2 * time.Hour),
				},
			},
			{
				Name:   "work",
				Type:   "api",
				Tier:   "tier_3",
				Status: "ok",
				RateLimits: &collectors.APIRateLimits{
					RequestsLimit:     2000,
					RequestsRemaining: 1500,
					RequestsReset:     time.Now().Add(10 * time.Minute),
					TokensLimit:       200000,
					TokensRemaining:   150000,
					TokensReset:       time.Now().Add(10 * time.Minute),
				},
			},
		},
	}

	result := renderClaudeContent(data, 80, 24)

	// Should contain both account names.
	if !strings.Contains(result, "personal") {
		t.Error("expected 'personal' account in output")
	}
	if !strings.Contains(result, "work") {
		t.Error("expected 'work' account in output")
	}

	// Should contain separator line between accounts.
	if !strings.Contains(result, "\u2500") {
		t.Error("expected horizontal separator between accounts")
	}

	// Should have both SUB and API badges.
	if !strings.Contains(result, "SUB") {
		t.Error("expected SUB badge")
	}
	if !strings.Contains(result, "API") {
		t.Error("expected API badge")
	}
}

func TestRenderClaudeContent_ExtraUsage(t *testing.T) {
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "maxuser",
				Type:   "subscription",
				Tier:   "max_5x",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 95.0,
					ResetsAt:    time.Now().Add(10 * time.Minute),
				},
				ExtraUsage: &collectors.ExtraUsage{
					Enabled:      true,
					MonthlyLimit: 5000, // $50.00
					UsedCredits:  2500, // $25.00
					Utilization:  50.0,
				},
			},
		},
	}

	result := renderClaudeContent(data, 80, 24)

	// Should contain extra usage section.
	if !strings.Contains(result, "Extra") {
		t.Error("expected 'Extra' label for extra usage gauge")
	}

	// Should show dollar amounts.
	if !strings.Contains(result, "$25.00") {
		t.Error("expected '$25.00' used credits")
	}
	if !strings.Contains(result, "$50.00") {
		t.Error("expected '$50.00' monthly limit")
	}
}

func TestRenderClaudeContent_NilPeriods(t *testing.T) {
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:     "nodata",
				Type:     "subscription",
				Tier:     "pro",
				Status:   "ok",
				FiveHour: nil,
				SevenDay: nil,
			},
		},
	}

	result := renderClaudeContent(data, 80, 24)

	// Should contain account name and still render.
	if !strings.Contains(result, "nodata") {
		t.Error("expected account name 'nodata' in output")
	}

	// Should show a message about missing data.
	if !strings.Contains(result, "No usage period data") {
		t.Error("expected 'No usage period data' for nil periods")
	}
}

func TestRenderClaudeContent_WidthAdaptation(t *testing.T) {
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "test",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 50.0,
					ResetsAt:    time.Now().Add(30 * time.Minute),
				},
			},
		},
	}

	narrow := renderClaudeContent(data, 40, 24)
	wide := renderClaudeContent(data, 120, 24)

	// Both should render without errors.
	if len(narrow) == 0 {
		t.Error("narrow render produced empty output")
	}
	if len(wide) == 0 {
		t.Error("wide render produced empty output")
	}

	// Wide should be at least as long (or longer with wider gauges).
	// This is a soft check; both must contain the account name.
	if !strings.Contains(narrow, "test") {
		t.Error("narrow render missing account name")
	}
	if !strings.Contains(wide, "test") {
		t.Error("wide render missing account name")
	}
}

func TestFormatResetTime_Soon(t *testing.T) {
	result := formatResetTime(time.Now().Add(25 * time.Minute))
	if !strings.Contains(result, "Resets in") {
		t.Errorf("expected 'Resets in' for near-future time, got: %s", result)
	}
	if !strings.Contains(result, "m") {
		t.Errorf("expected minutes suffix, got: %s", result)
	}
}

func TestFormatResetTime_Later(t *testing.T) {
	result := formatResetTime(time.Now().Add(2 * time.Hour))
	if !strings.Contains(result, "Resets at") {
		t.Errorf("expected 'Resets at' for distant time, got: %s", result)
	}
}

func TestFormatResetTime_Zero(t *testing.T) {
	result := formatResetTime(time.Time{})
	if result != "" {
		t.Errorf("expected empty string for zero time, got: %s", result)
	}
}

func TestFormatResetTime_Past(t *testing.T) {
	result := formatResetTime(time.Now().Add(-5 * time.Minute))
	if result != "Reset pending" {
		t.Errorf("expected 'Reset pending' for past time, got: %s", result)
	}
}
