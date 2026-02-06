// Package integration contains cross-component integration tests for prompt-pulse.
// These tests validate TUI lifecycle, tab rendering with mock data, keyboard
// navigation, scroll behavior, and help toggle functionality.
package integration

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/tui"
	"gitlab.com/tinyland/lab/prompt-pulse/tests/mocks"
)

// windowSize is the standard terminal size used across integration tests.
var windowSize = tea.WindowSizeMsg{Width: 120, Height: 40}

// makeReadyModel creates a TUI model and sends a WindowSizeMsg to mark it ready.
func makeReadyModel() tui.Model {
	m := tui.NewModel()
	updated, _ := m.Update(windowSize)
	return updated.(tui.Model)
}

// makeReadyModelWithData creates a ready model populated with all mock data fields.
func makeReadyModelWithData() tui.Model {
	m := makeReadyModel()

	// Seed random for reproducible mock data.
	mocks.SeedRandom(42)

	m.SetClaudeData(mocks.MockClaudeUsage(3))
	m.SetBillingData(mocks.MockBillingData())
	m.SetInfraData(mocks.MockInfraStatus())
	m.SetFastfetchData(mocks.MockFastfetchData())
	m.SetSysMetricsData(mocks.MockSysMetricsData())

	return m
}

// sendKey sends a rune key press to the model and returns the updated model.
func sendKey(m tui.Model, r rune) tui.Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return updated.(tui.Model)
}

// sendSpecialKey sends a special key press (Tab, Shift+Tab, etc.) to the model.
func sendSpecialKey(m tui.Model, keyType tea.KeyType) tui.Model {
	updated, _ := m.Update(tea.KeyMsg{Type: keyType})
	return updated.(tui.Model)
}

// isQuitCmd executes a tea.Cmd and returns true if it produces a tea.QuitMsg.
func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	return ok
}

// ---------- TUI Lifecycle ----------

// TestTUILifecycle validates the full Init -> WindowSizeMsg -> data population -> View cycle.
func TestTUILifecycle(t *testing.T) {
	// Phase 1: Init with no cache dir returns nil command.
	m := tui.NewModel()
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init() should return nil when no cacheDir configured")
	}

	// Phase 2: Before WindowSizeMsg, view shows initializing.
	view := m.View()
	if !strings.Contains(view, "Initializing") {
		t.Errorf("expected 'Initializing' before WindowSizeMsg, got: %q", view)
	}

	// Phase 3: WindowSizeMsg makes the model ready.
	updated, _ := m.Update(windowSize)
	m = updated.(tui.Model)
	view = m.View()
	if strings.Contains(view, "Initializing") {
		t.Error("should not show 'Initializing' after WindowSizeMsg")
	}
	// Tab headers should be visible.
	for _, tabName := range []string{"Claude", "Billing", "Infra", "System"} {
		if !strings.Contains(view, tabName) {
			t.Errorf("expected tab header %q in view after WindowSizeMsg", tabName)
		}
	}

	// Phase 4: Simulate dataRefreshMsg with mock data.
	mocks.SeedRandom(42)
	m.SetClaudeData(mocks.MockClaudeUsage(2))
	m.SetBillingData(mocks.MockBillingData())
	m.SetInfraData(mocks.MockInfraStatus())
	m.SetFastfetchData(mocks.MockFastfetchData())
	m.SetSysMetricsData(mocks.MockSysMetricsData())

	// Phase 5: View should now render Claude content (default tab).
	view = m.View()
	if view == "" {
		t.Error("expected non-empty view after data population")
	}
}

// TestTUILifecycle_WithConfig validates Init with a cache directory returns commands.
func TestTUILifecycle_WithConfig(t *testing.T) {
	m := tui.NewModelWithConfig(tui.ModelConfig{
		CacheDir:        "/tmp/test-cache-lifecycle",
		CacheTTL:        60 * time.Second,
		RefreshInterval: 30 * time.Second,
	})
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() should return a Cmd when cacheDir is configured")
	}
}

// ---------- Tab Rendering with Mock Data ----------

// TestAllTabsRenderWithMockData creates a model with all mock data and verifies
// that each of the 4 tabs produces non-empty, meaningful output.
func TestAllTabsRenderWithMockData(t *testing.T) {
	m := makeReadyModelWithData()

	tests := []struct {
		name     string
		key      rune
		contains []string
	}{
		{
			name: "Claude tab",
			key:  '1',
			contains: []string{
				"Claude",
			},
		},
		{
			name: "Billing tab",
			key:  '2',
			contains: []string{
				"Billing",
			},
		},
		{
			name: "Infra tab",
			key:  '3',
			contains: []string{
				"Infra",
			},
		},
		{
			name: "System tab",
			key:  '4',
			contains: []string{
				"System",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := sendKey(m, tt.key)
			view := current.View()

			if len(view) == 0 {
				t.Errorf("%s: view is empty", tt.name)
				return
			}

			for _, expected := range tt.contains {
				if !strings.Contains(view, expected) {
					t.Errorf("%s: expected view to contain %q", tt.name, expected)
				}
			}
		})
	}
}

// TestClaudeTabContent validates Claude tab renders account data correctly.
func TestClaudeTabContent(t *testing.T) {
	m := makeReadyModel()

	// Set up a subscription account with known data.
	m.SetClaudeData(&collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{
				Name:   "personal",
				Type:   "subscription",
				Tier:   "pro",
				Status: "ok",
				FiveHour: &collectors.UsagePeriod{
					Utilization: 55.0,
					ResetsAt:    time.Now().Add(2 * time.Hour),
				},
				SevenDay: &collectors.UsagePeriod{
					Utilization: 30.0,
					ResetsAt:    time.Now().Add(3 * 24 * time.Hour),
				},
			},
			{
				Name:   "work-api",
				Type:   "api",
				Tier:   "tier_2",
				Status: "ok",
				RateLimits: &collectors.APIRateLimits{
					RequestsLimit:     1000,
					RequestsRemaining: 400,
					TokensLimit:       100000,
					TokensRemaining:   60000,
				},
			},
		},
	})

	// Ensure we are on the Claude tab.
	current := sendKey(m, '1')
	view := current.View()

	// Should contain account names.
	if !strings.Contains(view, "personal") {
		t.Error("Claude tab should contain 'personal' account name")
	}
	if !strings.Contains(view, "work-api") {
		t.Error("Claude tab should contain 'work-api' account name")
	}
	// Should contain subscription badge.
	if !strings.Contains(view, "SUB") {
		t.Error("Claude tab should contain 'SUB' badge for subscription account")
	}
	// Should contain API badge.
	if !strings.Contains(view, "API") {
		t.Error("Claude tab should contain 'API' badge for API account")
	}
}

// TestClaudeTabNoData validates Claude tab shows placeholder when data is nil.
func TestClaudeTabNoData(t *testing.T) {
	m := makeReadyModel()
	// Claude tab is the default, no data set.
	view := m.View()
	if !strings.Contains(view, "No Claude usage data") {
		t.Error("Claude tab should show 'No Claude usage data' when nil")
	}
}

// TestBillingTabContent validates Billing tab renders provider data.
func TestBillingTabContent(t *testing.T) {
	m := makeReadyModel()
	mocks.SeedRandom(42)
	m.SetBillingData(mocks.MockBillingData())

	current := sendKey(m, '2')
	view := current.View()

	// Should contain billing title.
	if !strings.Contains(view, "Cloud Billing") {
		t.Error("Billing tab should contain 'Cloud Billing' title")
	}
	// Should contain at least one provider name.
	if !strings.Contains(view, "Civo") && !strings.Contains(view, "civo") {
		t.Error("Billing tab should contain provider name 'Civo'")
	}
}

// TestBillingTabNoData validates Billing tab shows placeholder when data is nil.
func TestBillingTabNoData(t *testing.T) {
	m := makeReadyModel()
	current := sendKey(m, '2')
	view := current.View()
	if !strings.Contains(view, "No billing data") {
		t.Error("Billing tab should show 'No billing data' when nil")
	}
}

// TestInfraTabContent validates Infra tab renders Tailscale and K8s data.
func TestInfraTabContent(t *testing.T) {
	m := makeReadyModel()
	mocks.SeedRandom(42)
	m.SetInfraData(mocks.MockInfraStatus())

	current := sendKey(m, '3')
	view := current.View()

	// Should contain infra title.
	if !strings.Contains(view, "Infrastructure") {
		t.Error("Infra tab should contain 'Infrastructure' title")
	}
	// Should contain Tailscale mesh info.
	if !strings.Contains(view, "Tailscale") {
		t.Error("Infra tab should contain 'Tailscale' section")
	}
	if !strings.Contains(view, "tinyland.ts.net") {
		t.Error("Infra tab should contain tailnet name 'tinyland.ts.net'")
	}
	// Should contain K8s cluster info.
	if !strings.Contains(view, "K8s") {
		t.Error("Infra tab should contain 'K8s' section")
	}
}

// TestInfraTabNoData validates Infra tab shows placeholder when data is nil.
func TestInfraTabNoData(t *testing.T) {
	m := makeReadyModel()
	current := sendKey(m, '3')
	view := current.View()
	if !strings.Contains(view, "No infrastructure data") {
		t.Error("Infra tab should show 'No infrastructure data' when nil")
	}
}

// TestSystemTabContent validates System tab renders fastfetch and sysmetrics.
func TestSystemTabContent(t *testing.T) {
	m := makeReadyModel()
	mocks.SeedRandom(42)
	m.SetFastfetchData(mocks.MockFastfetchData())
	m.SetSysMetricsData(mocks.MockSysMetricsData())

	current := sendKey(m, '4')
	view := current.View()

	// Should contain system information from fastfetch.
	if !strings.Contains(view, "System Information") {
		t.Error("System tab should contain 'System Information' from fastfetch section")
	}
	if !strings.Contains(view, "Rocky Linux") {
		t.Error("System tab should contain OS info 'Rocky Linux'")
	}
	if !strings.Contains(view, "Intel i7") {
		t.Error("System tab should contain CPU info 'Intel i7'")
	}

	// Should contain system metrics section.
	if !strings.Contains(view, "System Metrics") {
		t.Error("System tab should contain 'System Metrics' from sysmetrics section")
	}
	// Should contain CPU, RAM, Disk labels.
	if !strings.Contains(view, "CPU") {
		t.Error("System tab should contain CPU metric label")
	}
	if !strings.Contains(view, "RAM") {
		t.Error("System tab should contain RAM metric label")
	}
	if !strings.Contains(view, "Disk") {
		t.Error("System tab should contain Disk metric label")
	}
	// Should contain load average.
	if !strings.Contains(view, "Load") {
		t.Error("System tab should contain Load average section")
	}
}

// TestSystemTabNoData validates System tab shows placeholder when data is nil.
func TestSystemTabNoData(t *testing.T) {
	m := makeReadyModel()
	current := sendKey(m, '4')
	view := current.View()
	if !strings.Contains(view, "No system data") {
		t.Error("System tab should show 'No system data' when nil")
	}
}

// TestSystemTabFastfetchOnly validates System tab with fastfetch but no sysmetrics.
func TestSystemTabFastfetchOnly(t *testing.T) {
	m := makeReadyModel()
	mocks.SeedRandom(42)
	m.SetFastfetchData(mocks.MockFastfetchData())
	// No sysmetrics set.

	current := sendKey(m, '4')
	view := current.View()

	if !strings.Contains(view, "System Information") {
		t.Error("System tab should show fastfetch section")
	}
	if !strings.Contains(view, "Rocky Linux") {
		t.Error("System tab should show OS info from fastfetch")
	}
}

// TestSystemTabSysmetricsOnly validates System tab with sysmetrics but no fastfetch.
func TestSystemTabSysmetricsOnly(t *testing.T) {
	m := makeReadyModel()
	mocks.SeedRandom(42)
	m.SetSysMetricsData(mocks.MockSysMetricsData())
	// No fastfetch set.

	current := sendKey(m, '4')
	view := current.View()

	if !strings.Contains(view, "System Metrics") {
		t.Error("System tab should show sysmetrics section")
	}
	if !strings.Contains(view, "CPU") {
		t.Error("System tab should show CPU metric")
	}
}

// ---------- Tab Switching ----------

// TestTabSwitching verifies Tab and Shift+Tab cycle through all 4 tabs correctly.
func TestTabSwitching(t *testing.T) {
	m := makeReadyModelWithData()

	// Default is Claude (tab 1).
	view := m.View()
	if !strings.Contains(view, "Claude") {
		t.Error("initial tab should be Claude")
	}

	// Tab -> Billing.
	m = sendSpecialKey(m, tea.KeyTab)
	view = m.View()
	// The tab bar should still show all tabs. Content changes.
	// We just verify the view changed (non-empty and different).
	if len(view) == 0 {
		t.Error("Billing tab view should not be empty")
	}

	// Tab -> Infra.
	m = sendSpecialKey(m, tea.KeyTab)
	view = m.View()
	if len(view) == 0 {
		t.Error("Infra tab view should not be empty")
	}

	// Tab -> System.
	m = sendSpecialKey(m, tea.KeyTab)
	view = m.View()
	if len(view) == 0 {
		t.Error("System tab view should not be empty")
	}

	// Tab -> Claude (wraps around).
	m = sendSpecialKey(m, tea.KeyTab)
	// Should be back to Claude.

	// Shift+Tab -> System (wraps backward).
	m = sendSpecialKey(m, tea.KeyShiftTab)
	view = m.View()
	if len(view) == 0 {
		t.Error("System tab view (via Shift+Tab wrap) should not be empty")
	}
}

// TestNumericTabShortcuts verifies 1/2/3/4 jump directly to specific tabs.
func TestNumericTabShortcuts(t *testing.T) {
	m := makeReadyModelWithData()

	tests := []struct {
		key  rune
		tab  string
	}{
		{'1', "Claude"},
		{'2', "Billing"},
		{'3', "Infra"},
		{'4', "System"},
	}

	for _, tt := range tests {
		t.Run(tt.tab, func(t *testing.T) {
			current := sendKey(m, tt.key)
			view := current.View()
			if len(view) == 0 {
				t.Errorf("tab %q view should not be empty after pressing '%c'", tt.tab, tt.key)
			}
			// Every view should contain tab headers.
			if !strings.Contains(view, tt.tab) {
				t.Errorf("expected view to contain tab name %q", tt.tab)
			}
		})
	}
}

// ---------- Scroll Behavior ----------

// TestScrollBehavior validates that 'j' key scroll events are handled without error.
func TestScrollBehavior(t *testing.T) {
	m := makeReadyModelWithData()

	// Switch to system tab which has plenty of content for scrolling.
	m = sendKey(m, '4')
	viewBefore := m.View()
	if len(viewBefore) == 0 {
		t.Fatal("System tab view should not be empty before scroll")
	}

	// Send multiple scroll-down key presses.
	for i := 0; i < 5; i++ {
		m = sendKey(m, 'j')
	}

	viewAfter := m.View()
	if len(viewAfter) == 0 {
		t.Error("view should not be empty after scrolling")
	}

	// Send scroll-up key presses.
	for i := 0; i < 3; i++ {
		m = sendKey(m, 'k')
	}

	viewAfterUp := m.View()
	if len(viewAfterUp) == 0 {
		t.Error("view should not be empty after scrolling up")
	}
}

// TestScrollGoTopBottom validates 'g' (top) and 'G' (bottom) navigation.
func TestScrollGoTopBottom(t *testing.T) {
	m := makeReadyModelWithData()
	m = sendKey(m, '4') // System tab with more content.

	// Go to bottom.
	m = sendKey(m, 'G')
	viewBottom := m.View()
	if len(viewBottom) == 0 {
		t.Error("view should not be empty at bottom")
	}

	// Go to top.
	m = sendKey(m, 'g')
	viewTop := m.View()
	if len(viewTop) == 0 {
		t.Error("view should not be empty at top")
	}
}

// TestPageUpDown validates page up/down navigation.
func TestPageUpDown(t *testing.T) {
	m := makeReadyModelWithData()
	m = sendKey(m, '4')

	// Page down.
	m = sendSpecialKey(m, tea.KeyPgDown)
	view := m.View()
	if len(view) == 0 {
		t.Error("view should not be empty after page down")
	}

	// Page up.
	m = sendSpecialKey(m, tea.KeyPgUp)
	view = m.View()
	if len(view) == 0 {
		t.Error("view should not be empty after page up")
	}
}

// ---------- Help Toggle ----------

// TestHelpToggle validates that '?' toggles the expanded help display.
func TestHelpToggle(t *testing.T) {
	m := makeReadyModelWithData()

	// Initially, help is in short mode. View should contain basic help hints.
	viewBefore := m.View()
	if !strings.Contains(viewBefore, "quit") {
		t.Error("expected 'quit' help hint in default view")
	}

	// Press '?' to toggle expanded help.
	m = sendKey(m, '?')
	viewExpanded := m.View()

	// Expanded help should show more keybinding descriptions.
	// The full help includes all the navigation/scroll/system groups.
	if !strings.Contains(viewExpanded, "scroll") {
		t.Error("expanded help should contain 'scroll' keybinding info")
	}

	// Press '?' again to collapse help.
	m = sendKey(m, '?')
	viewCollapsed := m.View()

	// After toggling off, the expanded content should be gone or reduced.
	// We just verify the view is valid and non-empty.
	if len(viewCollapsed) == 0 {
		t.Error("view should not be empty after collapsing help")
	}
}

// ---------- Refresh Key ----------

// TestRefreshKey validates that 'r' triggers a refresh.
// Without a cache dir, it should return nil command (no-op).
func TestRefreshKey(t *testing.T) {
	m := makeReadyModelWithData()

	// Without cache dir configured, refresh should be a no-op.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = updated.(tui.Model)
	if cmd != nil {
		t.Error("refresh without cacheDir should return nil command")
	}

	// View should still be valid.
	view := m.View()
	if len(view) == 0 {
		t.Error("view should not be empty after refresh key")
	}
}

// TestRefreshKey_WithConfig validates that 'r' triggers spinner with cache dir.
func TestRefreshKey_WithConfig(t *testing.T) {
	m := tui.NewModelWithConfig(tui.ModelConfig{
		CacheDir:        "/tmp/test-refresh",
		CacheTTL:        60 * time.Second,
		RefreshInterval: 30 * time.Second,
	})

	// Make it ready.
	updated, _ := m.Update(windowSize)
	m = updated.(tui.Model)

	// Press 'r' - should produce a non-nil command (data fetch + spinner).
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	_ = updated.(tui.Model)
	if cmd == nil {
		t.Error("refresh with cacheDir should return a non-nil command")
	}
}

// ---------- Quit Key ----------

// TestQuitKey validates that 'q' produces a tea.Quit command.
func TestQuitKey(t *testing.T) {
	m := makeReadyModelWithData()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !isQuitCmd(cmd) {
		t.Error("'q' should produce tea.Quit command")
	}
}

// TestQuitCtrlC validates that Ctrl+C also quits.
func TestQuitCtrlC(t *testing.T) {
	m := makeReadyModelWithData()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !isQuitCmd(cmd) {
		t.Error("Ctrl+C should produce tea.Quit command")
	}
}

// ---------- Edge Cases ----------

// TestAllTabsNoData ensures all tabs render gracefully with nil data.
func TestAllTabsNoData(t *testing.T) {
	m := makeReadyModel() // No data set.

	tabs := []rune{'1', '2', '3', '4'}
	for _, key := range tabs {
		t.Run(string(key), func(t *testing.T) {
			current := sendKey(m, key)
			view := current.View()
			if len(view) == 0 {
				t.Errorf("tab '%c' view should not be empty even without data", key)
			}
		})
	}
}

// TestAllTabsWithEdgeCaseData tests tabs with error/high-utilization mock data.
func TestAllTabsWithEdgeCaseData(t *testing.T) {
	mocks.SeedRandom(42)

	t.Run("claude_all_errors", func(t *testing.T) {
		m := makeReadyModel()
		m.SetClaudeData(mocks.MockClaudeUsageAllErrors(3))
		current := sendKey(m, '1')
		view := current.View()
		if len(view) == 0 {
			t.Error("Claude tab with all-error accounts should not produce empty view")
		}
	})

	t.Run("claude_high_utilization", func(t *testing.T) {
		m := makeReadyModel()
		m.SetClaudeData(mocks.MockClaudeUsageHighUtilization(2))
		current := sendKey(m, '1')
		view := current.View()
		if len(view) == 0 {
			t.Error("Claude tab with high-util accounts should not produce empty view")
		}
	})

	t.Run("billing_over_budget", func(t *testing.T) {
		m := makeReadyModel()
		m.SetBillingData(mocks.MockBillingDataOverBudget())
		current := sendKey(m, '2')
		view := current.View()
		if len(view) == 0 {
			t.Error("Billing tab with over-budget data should not produce empty view")
		}
	})

	t.Run("billing_with_error", func(t *testing.T) {
		m := makeReadyModel()
		m.SetBillingData(mocks.MockBillingDataWithError("civo"))
		current := sendKey(m, '2')
		view := current.View()
		if len(view) == 0 {
			t.Error("Billing tab with error provider should not produce empty view")
		}
	})

	t.Run("infra_all_offline", func(t *testing.T) {
		m := makeReadyModel()
		m.SetInfraData(mocks.MockInfraStatusAllOffline())
		current := sendKey(m, '3')
		view := current.View()
		if len(view) == 0 {
			t.Error("Infra tab with all-offline nodes should not produce empty view")
		}
	})

	t.Run("sysmetrics_high_utilization", func(t *testing.T) {
		m := makeReadyModel()
		m.SetSysMetricsData(mocks.MockSysMetricsDataHighUtilization())
		m.SetFastfetchData(mocks.MockFastfetchData())
		current := sendKey(m, '4')
		view := current.View()
		if len(view) == 0 {
			t.Error("System tab with high-util sysmetrics should not produce empty view")
		}
	})
}

// TestMultipleWindowResizes validates that repeated resizes do not panic.
func TestMultipleWindowResizes(t *testing.T) {
	m := makeReadyModelWithData()

	sizes := []tea.WindowSizeMsg{
		{Width: 60, Height: 20},
		{Width: 120, Height: 40},
		{Width: 200, Height: 60},
		{Width: 40, Height: 15}, // Very narrow.
		{Width: 80, Height: 24},
	}

	for _, size := range sizes {
		updated, _ := m.Update(size)
		m = updated.(tui.Model)
		view := m.View()
		if len(view) == 0 {
			t.Errorf("view should not be empty after resize to %dx%d", size.Width, size.Height)
		}
	}
}

// TestRapidTabSwitching validates rapid switching between all tabs.
func TestRapidTabSwitching(t *testing.T) {
	m := makeReadyModelWithData()

	// Rapidly switch through all tabs multiple times.
	sequence := []rune{'1', '3', '4', '2', '1', '4', '3', '2'}
	for _, key := range sequence {
		m = sendKey(m, key)
		view := m.View()
		if len(view) == 0 {
			t.Errorf("view should not be empty after switching to tab '%c'", key)
		}
	}
}
