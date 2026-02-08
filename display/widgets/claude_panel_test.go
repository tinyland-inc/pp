package widgets

import (
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

func TestNewClaudePanel_NilData(t *testing.T) {
	panel := NewClaudePanel(nil, 80)
	if panel == nil {
		t.Fatal("NewClaudePanel should return non-nil for nil data")
	}
	if len(panel.Accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(panel.Accounts))
	}
}

func TestNewClaudePanel_EmptyAccounts(t *testing.T) {
	data := &collectors.ClaudeUsage{Accounts: []collectors.ClaudeAccountUsage{}}
	panel := NewClaudePanel(data, 80)

	if len(panel.Accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(panel.Accounts))
	}
}

func TestNewClaudePanel_SingleSubscription(t *testing.T) {
	now := time.Now()
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "personal",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 45.0,
					ResetsAt:    now.Add(2 * time.Hour),
				},
				SevenDay: &collectors.UsagePeriod{
					Utilization: 30.0,
					ResetsAt:    now.Add(3 * 24 * time.Hour),
				},
			},
		},
	}

	panel := NewClaudePanel(data, 80)

	if len(panel.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(panel.Accounts))
	}

	acct := panel.Accounts[0]
	if acct.Name != "personal" {
		t.Errorf("expected name 'personal', got %q", acct.Name)
	}
	if acct.SessionPercent != 45.0 {
		t.Errorf("expected session percent 45.0, got %f", acct.SessionPercent)
	}
	if acct.WeeklyPercent != 30.0 {
		t.Errorf("expected weekly percent 30.0, got %f", acct.WeeklyPercent)
	}
	if acct.SessionReset == "" {
		t.Error("expected non-empty session reset")
	}
	if acct.WeeklyReset == "" {
		t.Error("expected non-empty weekly reset")
	}
}

func TestNewClaudePanel_APIAccount(t *testing.T) {
	now := time.Now()
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "work-api",
				Type:   "api",
				Tier:   "tier_2",
				Status: "ok",
				RateLimits: &collectors.APIRateLimits{
					RequestsLimit:     1000,
					RequestsRemaining: 600,
					RequestsReset:     now.Add(1 * time.Hour),
					TokensLimit:       100000,
					TokensRemaining:   75000,
					TokensReset:       now.Add(2 * time.Hour),
				},
			},
		},
	}

	panel := NewClaudePanel(data, 80)

	if len(panel.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(panel.Accounts))
	}

	acct := panel.Accounts[0]
	if acct.Type != "api" {
		t.Errorf("expected type 'api', got %q", acct.Type)
	}
	// 400/1000 = 40%
	if acct.SessionPercent != 40.0 {
		t.Errorf("expected session percent 40.0, got %f", acct.SessionPercent)
	}
	// 25000/100000 = 25%
	if acct.WeeklyPercent != 25.0 {
		t.Errorf("expected weekly percent 25.0, got %f", acct.WeeklyPercent)
	}
}

func TestNewClaudePanel_MaxFiveAccounts(t *testing.T) {
	accounts := make([]collectors.ClaudeAccountUsage, 7)
	for i := range accounts {
		accounts[i] = collectors.ClaudeAccountUsage{
			Name:   "account" + string(rune('0'+i)),
			Type:   "subscription",
			Tier:   "pro",
			Status: "ok",
		}
	}

	data := &collectors.ClaudeUsage{Accounts: accounts}
	panel := NewClaudePanel(data, 80)

	if len(panel.Accounts) != 5 {
		t.Errorf("expected max 5 accounts, got %d", len(panel.Accounts))
	}
}

func TestClaudePanel_Render_NoAccounts(t *testing.T) {
	panel := &ClaudePanel{Width: 80}
	output := panel.Render()

	if output == "" {
		t.Error("Render should return non-empty for no accounts")
	}
}

func TestClaudePanel_Render_WithAccounts(t *testing.T) {
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "test",
				Type:   "subscription",
				Tier:   "max_5x",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 75.0,
					ResetsAt:    time.Now().Add(2 * time.Hour),
				},
			},
		},
	}

	panel := NewClaudePanel(data, 80)
	output := panel.Render()

	if output == "" {
		t.Error("Render should return non-empty output")
	}
	if len(output) < 10 {
		t.Error("Render output too short")
	}
}

func TestClaudePanel_RenderCompact(t *testing.T) {
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "work",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 50.0,
					ResetsAt:    time.Now().Add(3 * time.Hour),
				},
			},
		},
	}

	panel := NewClaudePanel(data, 80)
	output := panel.RenderCompact()

	if output == "" {
		t.Error("RenderCompact should return non-empty output")
	}
}

func TestClaudePanel_RenderCompact_ErrorStatus(t *testing.T) {
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

	panel := NewClaudePanel(data, 80)
	output := panel.RenderCompact()

	if output == "" {
		t.Error("RenderCompact should return non-empty for error status")
	}
}

func TestFormatCountdown_Future(t *testing.T) {
	// Test that FormatCountdown produces reasonable output formats.
	// We add extra time buffer to account for test execution time.

	// Test hours format (Xh Ym)
	future := time.Now().Add(2*time.Hour + 16*time.Minute) // extra minute buffer
	result := FormatCountdown(future)
	if len(result) < 4 || result[0] != '2' || result[1] != 'h' {
		t.Errorf("FormatCountdown(2h+) = %q, expected format like '2h XXm'", result)
	}

	// Test days format (Xd Yh)
	future = time.Now().Add(3*24*time.Hour + 13*time.Hour) // extra hour buffer
	result = FormatCountdown(future)
	if len(result) < 4 || result[0] != '3' || result[1] != 'd' {
		t.Errorf("FormatCountdown(3d+) = %q, expected format like '3d XXh'", result)
	}

	// Test minutes format (Xm)
	future = time.Now().Add(46 * time.Minute) // extra minute buffer
	result = FormatCountdown(future)
	if len(result) < 2 || result[len(result)-1] != 'm' {
		t.Errorf("FormatCountdown(45m+) = %q, expected format like 'XXm'", result)
	}
}

func TestFormatCountdown_Past(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	result := FormatCountdown(past)

	if result != "now" {
		t.Errorf("FormatCountdown(past) = %q, want 'now'", result)
	}
}

func TestFormatCountdown_Zero(t *testing.T) {
	result := FormatCountdown(time.Time{})
	if result != "" {
		t.Errorf("FormatCountdown(zero) = %q, want empty string", result)
	}
}

func TestGetUtilizationColor(t *testing.T) {
	tests := []struct {
		percent float64
		want    string
	}{
		{0, "green"},
		{50, "green"},
		{69, "green"},
		{70, "yellow"},
		{85, "yellow"},
		{89, "yellow"},
		{90, "red"},
		{95, "red"},
		{100, "red"},
	}

	for _, tt := range tests {
		color := GetUtilizationColor(tt.percent)
		var got string
		switch color {
		case claudeColorSuccess:
			got = "green"
		case claudeColorWarning:
			got = "yellow"
		case claudeColorDanger:
			got = "red"
		}
		if got != tt.want {
			t.Errorf("GetUtilizationColor(%.0f) = %q, want %q", tt.percent, got, tt.want)
		}
	}
}

func TestRenderUtilizationBar(t *testing.T) {
	// Just verify it produces output
	bar := RenderUtilizationBar(50, 10)
	if bar == "" {
		t.Error("RenderUtilizationBar should return non-empty")
	}
}

func TestRenderUtilizationBar_MinWidth(t *testing.T) {
	bar := RenderUtilizationBar(50, 2) // Below minimum
	if bar == "" {
		t.Error("RenderUtilizationBar should handle small width")
	}
}

func TestAccountDisplay_StatusIcon_HighUtilization(t *testing.T) {
	data := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "high",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 95.0,
				},
			},
		},
	}

	panel := NewClaudePanel(data, 80)
	if len(panel.Accounts) == 0 {
		t.Fatal("expected at least 1 account")
	}

	// Status icon should indicate critical (red)
	if panel.Accounts[0].StatusIcon == "" {
		t.Error("expected non-empty status icon")
	}
}

func TestAccountDisplay_StatusIcon_AuthFailed(t *testing.T) {
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

	panel := NewClaudePanel(data, 80)
	if len(panel.Accounts) == 0 {
		t.Fatal("expected at least 1 account")
	}

	// Status icon should indicate error
	if panel.Accounts[0].StatusIcon == "" {
		t.Error("expected non-empty status icon for auth_failed")
	}
}
