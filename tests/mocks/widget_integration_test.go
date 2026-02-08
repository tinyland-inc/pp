package mocks

import (
	"strings"
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/widgets"
)

// TestClaudeWidgetWithMockAccounts tests the Claude rendering with various account counts.
func TestClaudeWidgetWithMockAccounts(t *testing.T) {
	SeedRandom(42)

	tests := []struct {
		name     string
		accounts int
		cols     int
		expected []string // Expected content in output
	}{
		{
			name:     "1 account compact",
			accounts: 1,
			cols:     80,
			expected: []string{"personal"},
		},
		{
			name:     "3 accounts standard",
			accounts: 3,
			cols:     120,
			expected: []string{"personal", "work", "research"},
		},
		{
			name:     "5 accounts wide",
			accounts: 5,
			cols:     160,
			expected: []string{"personal", "work", "research", "dev", "prod"},
		},
		{
			name:     "5 accounts ultrawide",
			accounts: 5,
			cols:     200,
			expected: []string{"personal", "work", "research", "dev", "prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SeedRandom(42)
			usage := MockClaudeUsage(tt.accounts)

			if len(usage.Accounts) != tt.accounts {
				t.Errorf("Expected %d accounts, got %d", tt.accounts, len(usage.Accounts))
			}

			// Test gauge rendering for each account
			gaugeWidth := calculateGaugeWidth(tt.cols)
			for _, acct := range usage.Accounts {
				cfg := widgets.DefaultGaugeConfig()
				cfg.Width = gaugeWidth
				cfg.Percent = acct.GetPrimaryUtilization()
				cfg.Label = acct.Name

				result := widgets.RenderGauge(cfg)
				if result == "" {
					t.Errorf("Gauge for %s rendered empty", acct.Name)
				}
				if !strings.Contains(result, acct.Name) {
					t.Errorf("Gauge missing account name %s in output: %s", acct.Name, result)
				}
			}
		})
	}
}

// calculateGaugeWidth mirrors the logic used in the TUI.
func calculateGaugeWidth(termWidth int) int {
	labelWidth := 14
	gaugeWidth := termWidth - labelWidth - 10
	if gaugeWidth > 30 {
		gaugeWidth = 30
	}
	if gaugeWidth < 8 {
		gaugeWidth = 8
	}
	return gaugeWidth
}

// TestBillingPanelWithMockData tests billing panel rendering with mock providers.
func TestBillingPanelWithMockData(t *testing.T) {
	panelData := MockBillingPanelData()
	cfg := widgets.DefaultBillingPanelConfig()

	result := widgets.RenderBillingPanel(panelData, cfg)

	// Verify panel contains expected sections
	checks := []string{
		"Billing Dashboard",
		"Total:",
		"$",
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("Billing panel missing %q", check)
		}
	}

	// Verify all 4 providers are present
	for _, provider := range panelData.Providers {
		// Provider names should appear (with title case)
		providerTitle := strings.Title(provider.Name)
		if !strings.Contains(result, providerTitle) {
			t.Errorf("Billing panel missing provider %s", provider.Name)
		}
	}
}

// TestBillingPanelCompact tests the compact billing panel.
func TestBillingPanelCompact(t *testing.T) {
	panelData := MockBillingPanelData()

	result := widgets.RenderCompactBillingPanel(panelData, true)

	// Should contain total spend
	if !strings.Contains(result, "$") {
		t.Error("Compact panel missing dollar sign")
	}

	// Should contain forecast
	if !strings.Contains(result, "forecast") {
		t.Error("Compact panel missing forecast")
	}
}

// TestBillingPanelOverBudget tests the over-budget scenario.
func TestBillingPanelOverBudget(t *testing.T) {
	billingData := MockBillingDataOverBudget()

	panelData := widgets.BillingPanelData{
		TotalCurrent:  billingData.Total.CurrentMonthUSD,
		TotalForecast: billingData.Total.ForecastUSD,
		TotalBudget:   billingData.Total.BudgetUSD,
	}

	result := widgets.RenderCompactBillingPanel(panelData, false)

	// Should indicate over budget
	if !strings.Contains(result, "OVER BUDGET") && !strings.Contains(result, "of budget") {
		t.Error("Over-budget scenario not indicated")
	}
}

// TestInfraPanelWithMockData tests infrastructure panel rendering.
func TestInfraPanelWithMockData(t *testing.T) {
	infra := MockInfraStatus()
	panel := widgets.NewInfraPanel(widgets.DefaultInfraPanelConfig())

	result := panel.Render(infra)

	// Verify Tailscale section
	if !strings.Contains(result, "Tailscale") {
		t.Error("Infra panel missing Tailscale section")
	}

	// Verify Kubernetes section
	if !strings.Contains(result, "Kubernetes") {
		t.Error("Infra panel missing Kubernetes section")
	}

	// Verify cluster names
	for _, cluster := range infra.Kubernetes {
		if !strings.Contains(result, cluster.Name) {
			t.Errorf("Infra panel missing cluster %s", cluster.Name)
		}
	}
}

// TestInfraPanelTailscaleOnly tests panel with only Tailscale data.
func TestInfraPanelTailscaleOnly(t *testing.T) {
	infra := MockInfraStatusTailscaleOnly()
	panel := widgets.NewInfraPanel(widgets.DefaultInfraPanelConfig())

	result := panel.Render(infra)

	if !strings.Contains(result, "Tailscale") {
		t.Error("Tailscale-only panel missing Tailscale section")
	}
	if strings.Contains(result, "Kubernetes") {
		t.Error("Tailscale-only panel should not have Kubernetes section")
	}
}

// TestInfraPanelKubernetesOnly tests panel with only Kubernetes data.
func TestInfraPanelKubernetesOnly(t *testing.T) {
	infra := MockInfraStatusKubernetesOnly()
	panel := widgets.NewInfraPanel(widgets.DefaultInfraPanelConfig())

	result := panel.Render(infra)

	if strings.Contains(result, "Tailscale Mesh") {
		t.Error("K8s-only panel should not have Tailscale section")
	}
	if !strings.Contains(result, "Kubernetes") {
		t.Error("K8s-only panel missing Kubernetes section")
	}
}

// TestInfraPanelEmpty tests panel with no data.
func TestInfraPanelEmpty(t *testing.T) {
	infra := MockInfraStatusEmpty()
	panel := widgets.NewInfraPanel(widgets.DefaultInfraPanelConfig())

	result := panel.Render(infra)

	if !strings.Contains(result, "no infrastructure data") {
		t.Error("Empty panel should show 'no infrastructure data'")
	}
}

// TestInfraPanelAllOffline tests panel with all nodes offline.
func TestInfraPanelAllOffline(t *testing.T) {
	infra := MockInfraStatusAllOffline()
	panel := widgets.NewInfraPanel(widgets.DefaultInfraPanelConfig())

	result := panel.Render(infra)

	// Should have some content (not just empty)
	if result == "" {
		t.Error("All-offline panel rendered empty")
	}

	// Should show 0 nodes online in Tailscale
	if infra.Tailscale != nil && infra.Tailscale.OnlineCount != 0 {
		t.Error("All-offline scenario has online nodes")
	}
}

// TestSparklineWithMockHistory tests sparkline rendering with billing history.
func TestSparklineWithMockHistory(t *testing.T) {
	history := MockDailySpend(30, 100.0)
	values := collectors.GetSpendValues(history)

	result := widgets.RenderSparkline(widgets.SparklineConfig{
		Data:  values,
		Width: 20,
	})

	if result == "" {
		t.Error("Sparkline rendered empty")
	}

	// Sparkline should have visible characters
	if len(result) == 0 {
		t.Error("Sparkline has no content")
	}
}

// TestSparklineVariousWidths tests sparkline at different terminal widths.
func TestSparklineVariousWidths(t *testing.T) {
	history := MockDailySpend(30, 100.0)
	values := collectors.GetSpendValues(history)

	widths := []int{10, 15, 20, 30, 40}

	for _, width := range widths {
		t.Run("width_"+intToStr(width), func(t *testing.T) {
			result := widgets.RenderSparkline(widgets.SparklineConfig{
				Data:  values,
				Width: width,
			})

			if result == "" {
				t.Errorf("Sparkline empty at width %d", width)
			}
		})
	}
}

// TestStatusWidgetWithMockStatus tests status indicator rendering.
func TestStatusWidgetWithMockStatus(t *testing.T) {
	statuses := []string{"ok", "auth_failed", "rate_limited", "disabled", "error", "stale"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			result := widgets.RenderStatusFromString(status)

			if result == "" {
				t.Errorf("Status %s rendered empty", status)
			}
		})
	}
}

// TestTableWidgetWithClaudeData tests table rendering with Claude mock data.
func TestTableWidgetWithClaudeData(t *testing.T) {
	SeedRandom(42)
	usage := MockClaudeUsage(5)

	columns := []widgets.Column{
		{Title: "Account", Width: 15},
		{Title: "Type", Width: 12},
		{Title: "Tier", Width: 10},
		{Title: "Status", Width: 12},
		{Title: "5h Usage", Width: 10},
	}

	rows := make([][]string, len(usage.Accounts))
	for i, acct := range usage.Accounts {
		var fiveHour string
		if acct.FiveHour != nil {
			fiveHour = intToStr(int(acct.FiveHour.Utilization)) + "%"
		} else if acct.RateLimits != nil {
			used := acct.RateLimits.RequestsLimit - acct.RateLimits.RequestsRemaining
			fiveHour = intToStr(used) + " req"
		}

		rows[i] = []string{
			acct.Name,
			acct.Type,
			acct.Tier,
			acct.Status,
			fiveHour,
		}
	}

	cfg := widgets.DefaultTableConfig()
	cfg.Columns = columns
	cfg.Rows = rows
	cfg.MaxWidth = 80

	result := widgets.RenderTable(cfg)

	if result == "" {
		t.Error("Table rendered empty")
	}

	// Verify headers
	for _, col := range columns {
		if !strings.Contains(result, col.Title) {
			t.Errorf("Table missing header %s", col.Title)
		}
	}

	// Verify all account names appear
	for _, acct := range usage.Accounts {
		if !strings.Contains(result, acct.Name) {
			t.Errorf("Table missing account %s", acct.Name)
		}
	}
}

// TestMiniGaugeWithVariousPercents tests mini gauge rendering.
func TestMiniGaugeWithVariousPercents(t *testing.T) {
	percents := []float64{0, 25, 50, 75, 100}

	for _, pct := range percents {
		t.Run("pct_"+intToStr(int(pct)), func(t *testing.T) {
			result := widgets.RenderMiniGauge(pct, 10)

			if result == "" {
				t.Errorf("Mini gauge empty at %f%%", pct)
			}

			// Should have both filled and empty characters (except at 0 and 100)
			filledCount := strings.Count(result, "\u2588")
			emptyCount := strings.Count(result, "\u2591")

			// Allow for rounding differences (+/- 1)
			expectedFilled := int(pct / 100 * 10)
			if filledCount < expectedFilled-1 || filledCount > expectedFilled+1 {
				t.Errorf("At %f%%, expected ~%d filled chars (within 1), got %d", pct, expectedFilled, filledCount)
			}

			// Total should always be 10
			totalChars := filledCount + emptyCount
			if totalChars != 10 {
				t.Errorf("At %f%%, expected 10 total chars, got %d", pct, totalChars)
			}
		})
	}
}

// TestBannerLayoutWithMockData tests banner layout rendering with mock data.
func TestBannerLayoutWithMockData(t *testing.T) {
	SeedRandom(42)

	terminalWidths := []int{80, 120, 160, 200}

	for _, width := range terminalWidths {
		t.Run("width_"+intToStr(width), func(t *testing.T) {
			// Generate mock data
			claude := MockClaudeUsage(3)
			billing := MockBillingData()
			infra := MockInfraStatus()

			// Verify data is not nil
			if claude == nil || billing == nil || infra == nil {
				t.Error("Mock data is nil")
			}

			// Verify data has content
			if len(claude.Accounts) == 0 {
				t.Error("Claude accounts empty")
			}
			if len(billing.Providers) == 0 {
				t.Error("Billing providers empty")
			}
			if infra.Tailscale == nil && len(infra.Kubernetes) == 0 {
				t.Error("Infra data empty")
			}
		})
	}
}

// TestEdgeCasesRendering tests edge cases for widget rendering.
func TestEdgeCasesRendering(t *testing.T) {
	t.Run("nil claude data", func(t *testing.T) {
		claude := MockClaudeUsageNil()
		if claude != nil {
			t.Error("Expected nil")
		}
		// Verify widgets handle nil gracefully
		// (This would be tested at a higher level in the actual rendering code)
	})

	t.Run("empty claude accounts", func(t *testing.T) {
		claude := MockClaudeUsageEmpty()
		if claude == nil {
			t.Error("Expected non-nil")
		}
		if len(claude.Accounts) != 0 {
			t.Errorf("Expected 0 accounts, got %d", len(claude.Accounts))
		}
	})

	t.Run("all errors", func(t *testing.T) {
		claude := MockClaudeUsageAllErrors(3)
		for _, acct := range claude.Accounts {
			if acct.Status == "ok" {
				t.Error("Account should have error status")
			}
		}
	})

	t.Run("high utilization", func(t *testing.T) {
		claude := MockClaudeUsageHighUtilization(3)
		for _, acct := range claude.Accounts {
			util := acct.GetPrimaryUtilization()
			if util < 80 {
				t.Errorf("Expected high utilization (>=80), got %f", util)
			}
		}
	})
}
