package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// isQuitCmd executes a tea.Cmd and returns true if it produces a tea.QuitMsg.
func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	return ok
}

func TestNewModel(t *testing.T) {
	m := NewModel()

	if m.activeTab != TabClaude {
		t.Errorf("expected activeTab to be TabClaude, got %d", m.activeTab)
	}
	if m.width != 0 {
		t.Errorf("expected width to be 0, got %d", m.width)
	}
	if m.height != 0 {
		t.Errorf("expected height to be 0, got %d", m.height)
	}
	if m.ready {
		t.Error("expected ready to be false")
	}
	if m.claude != nil {
		t.Error("expected claude to be nil")
	}
	if m.billing != nil {
		t.Error("expected billing to be nil")
	}
	if m.infra != nil {
		t.Error("expected infra to be nil")
	}
}

func TestModel_Init(t *testing.T) {
	m := NewModel()
	cmd := m.Init()
	if cmd != nil {
		t.Error("expected Init() to return nil Cmd")
	}
}

func TestModel_Update_Quit(t *testing.T) {
	m := NewModel()
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)

	if !isQuitCmd(cmd) {
		t.Error("expected 'q' key to produce tea.Quit command")
	}
}

func TestModel_Update_CtrlC(t *testing.T) {
	m := NewModel()
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)

	if !isQuitCmd(cmd) {
		t.Error("expected ctrl+c to produce tea.Quit command")
	}
}

func TestModel_Update_NextTab(t *testing.T) {
	m := NewModel()

	// Claude -> Billing
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.activeTab != TabBilling {
		t.Errorf("expected TabBilling after first tab, got %d", m.activeTab)
	}

	// Billing -> Infra
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.activeTab != TabInfra {
		t.Errorf("expected TabInfra after second tab, got %d", m.activeTab)
	}

	// Infra -> Claude (wraps)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.activeTab != TabClaude {
		t.Errorf("expected TabClaude after third tab (wrap), got %d", m.activeTab)
	}
}

func TestModel_Update_PrevTab(t *testing.T) {
	m := NewModel()

	// Claude -> Infra (wraps backward)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.activeTab != TabInfra {
		t.Errorf("expected TabInfra after shift+tab from Claude, got %d", m.activeTab)
	}

	// Infra -> Billing
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.activeTab != TabBilling {
		t.Errorf("expected TabBilling after shift+tab from Infra, got %d", m.activeTab)
	}

	// Billing -> Claude
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.activeTab != TabClaude {
		t.Errorf("expected TabClaude after shift+tab from Billing, got %d", m.activeTab)
	}
}

func TestModel_Update_DirectTab(t *testing.T) {
	tests := []struct {
		key      rune
		expected Tab
	}{
		{'1', TabClaude},
		{'2', TabBilling},
		{'3', TabInfra},
	}

	for _, tt := range tests {
		m := NewModel()
		// Start from a different tab to ensure the jump works.
		m.activeTab = TabInfra

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
		m = updated.(Model)
		if m.activeTab != tt.expected {
			t.Errorf("pressing '%c': expected tab %d, got %d", tt.key, tt.expected, m.activeTab)
		}
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	m := NewModel()

	if m.ready {
		t.Fatal("expected ready to be false before WindowSizeMsg")
	}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	if !m.ready {
		t.Error("expected ready to be true after WindowSizeMsg")
	}
	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
}

func TestModel_View_NotReady(t *testing.T) {
	m := NewModel()
	view := m.View()

	if view != "Initializing..." {
		t.Errorf("expected 'Initializing...' when not ready, got %q", view)
	}
}

func TestModel_View_Ready(t *testing.T) {
	m := NewModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	view := m.View()

	// The view should contain the tab names.
	if !containsString(view, "Claude") {
		t.Error("expected view to contain 'Claude'")
	}
	if !containsString(view, "Billing") {
		t.Error("expected view to contain 'Billing'")
	}
	if !containsString(view, "Infra") {
		t.Error("expected view to contain 'Infra'")
	}
	// Should contain help text.
	if !containsString(view, "q: quit") {
		t.Error("expected view to contain help text 'q: quit'")
	}
}

func TestModel_SetData(t *testing.T) {
	m := NewModel()

	claude := &collectors.ClaudeUsage{
		Accounts: []collectors.ClaudeAccountUsage{
			{Name: "test", Type: "subscription", Tier: "pro", Status: "ok"},
		},
	}
	m.SetClaudeData(claude)
	if m.claude != claude {
		t.Error("SetClaudeData did not set claude field")
	}
	if m.lastUpdated.IsZero() {
		t.Error("SetClaudeData did not set lastUpdated")
	}

	billing := &collectors.BillingData{
		Total: collectors.BillingSummary{CurrentMonthUSD: 42.0},
	}
	m.SetBillingData(billing)
	if m.billing != billing {
		t.Error("SetBillingData did not set billing field")
	}

	infra := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "test.ts.net",
			OnlineCount: 3,
			TotalCount:  5,
		},
	}
	m.SetInfraData(infra)
	if m.infra != infra {
		t.Error("SetInfraData did not set infra field")
	}
}

func TestModel_TabWrapping(t *testing.T) {
	// Test next from last tab wraps to first.
	m := NewModel()
	m.activeTab = TabInfra
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.activeTab != TabClaude {
		t.Errorf("expected next from TabInfra to wrap to TabClaude, got %d", m.activeTab)
	}

	// Test prev from first tab wraps to last.
	m.activeTab = TabClaude
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.activeTab != TabInfra {
		t.Errorf("expected prev from TabClaude to wrap to TabInfra, got %d", m.activeTab)
	}
}

// containsString checks if substr appears anywhere in s.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
