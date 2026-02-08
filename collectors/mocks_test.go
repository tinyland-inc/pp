package collectors

import (
	"testing"
)

func TestMockClaudeUsage(t *testing.T) {
	mock := MockClaudeUsage()

	if mock == nil {
		t.Fatal("MockClaudeUsage returned nil")
	}

	if len(mock.Accounts) == 0 {
		t.Fatal("MockClaudeUsage has no accounts")
	}

	acct := mock.Accounts[0]
	if acct.Name != "primary" {
		t.Errorf("Expected account name 'primary', got %q", acct.Name)
	}

	if acct.Type != "subscription" {
		t.Errorf("Expected type 'subscription', got %q", acct.Type)
	}

	if acct.Status != StatusOK {
		t.Errorf("Expected status %q, got %q", StatusOK, acct.Status)
	}

	if acct.FiveHour == nil {
		t.Error("FiveHour should not be nil for mock data")
	}

	if acct.SevenDay == nil {
		t.Error("SevenDay should not be nil for mock data")
	}

	if acct.FiveHour != nil && acct.FiveHour.Utilization != 0.0 {
		t.Errorf("Expected zero utilization, got %.2f", acct.FiveHour.Utilization)
	}
}

func TestMockBillingData(t *testing.T) {
	mock := MockBillingData()

	if mock == nil {
		t.Fatal("MockBillingData returned nil")
	}

	if len(mock.Providers) == 0 {
		t.Fatal("MockBillingData has no providers")
	}

	if mock.Total.CurrentMonthUSD != 0.0 {
		t.Errorf("Expected zero total spend, got %.2f", mock.Total.CurrentMonthUSD)
	}

	if mock.History == nil {
		t.Fatal("History should not be nil")
	}

	if len(mock.History.TotalHistory) != 30 {
		t.Errorf("Expected 30 days of history, got %d", len(mock.History.TotalHistory))
	}

	// Verify all history entries have zero spend
	for i, day := range mock.History.TotalHistory {
		if day.SpendUSD != 0.0 {
			t.Errorf("Day %d: expected zero spend, got %.2f", i, day.SpendUSD)
		}
	}

	if mock.Total.SuccessCount != 2 {
		t.Errorf("Expected 2 successful providers, got %d", mock.Total.SuccessCount)
	}

	if mock.Total.ErrorCount != 0 {
		t.Errorf("Expected 0 errors, got %d", mock.Total.ErrorCount)
	}
}

func TestMockBillingDataWithHistory(t *testing.T) {
	mock := MockBillingDataWithHistory()

	if mock == nil {
		t.Fatal("MockBillingDataWithHistory returned nil")
	}

	if mock.History == nil {
		t.Fatal("History should not be nil")
	}

	if len(mock.History.TotalHistory) != 30 {
		t.Errorf("Expected 30 days of history, got %d", len(mock.History.TotalHistory))
	}

	// Verify spend increases over time (not all zero)
	firstDay := mock.History.TotalHistory[0].SpendUSD
	lastDay := mock.History.TotalHistory[len(mock.History.TotalHistory)-1].SpendUSD

	if firstDay >= lastDay {
		t.Errorf("Expected spend to increase over time: first=%.2f, last=%.2f", firstDay, lastDay)
	}

	// Verify total matches sum of providers
	expectedTotal := 0.0
	for _, provider := range mock.Providers {
		expectedTotal += provider.CurrentMonth.SpendUSD
	}

	if mock.Total.CurrentMonthUSD != expectedTotal {
		t.Errorf("Total mismatch: got %.2f, expected %.2f", mock.Total.CurrentMonthUSD, expectedTotal)
	}
}

func TestMockInfraStatus(t *testing.T) {
	mock := MockInfraStatus()

	if mock == nil {
		t.Fatal("MockInfraStatus returned nil")
	}

	if mock.Tailscale == nil {
		t.Fatal("Tailscale should not be nil")
	}

	if mock.Tailscale.OnlineCount != 0 {
		t.Errorf("Expected 0 online nodes, got %d", mock.Tailscale.OnlineCount)
	}

	if len(mock.Tailscale.Nodes) == 0 {
		t.Fatal("Should have at least one node")
	}

	// Verify all nodes have nil metrics
	for i, node := range mock.Tailscale.Nodes {
		if node.CPUPercent != nil {
			t.Errorf("Node %d: CPUPercent should be nil", i)
		}
		if node.RAMPercent != nil {
			t.Errorf("Node %d: RAMPercent should be nil", i)
		}
		if node.DiskPercent != nil {
			t.Errorf("Node %d: DiskPercent should be nil", i)
		}
	}

	if len(mock.Kubernetes) == 0 {
		t.Fatal("Should have at least one Kubernetes cluster")
	}
}

func TestMockInfraStatusWithMetrics(t *testing.T) {
	mock := MockInfraStatusWithMetrics()

	if mock == nil {
		t.Fatal("MockInfraStatusWithMetrics returned nil")
	}

	if mock.Tailscale == nil {
		t.Fatal("Tailscale should not be nil")
	}

	if mock.Tailscale.OnlineCount == 0 {
		t.Error("Expected online nodes")
	}

	// Verify all nodes have metrics
	for i, node := range mock.Tailscale.Nodes {
		if !node.Online {
			t.Errorf("Node %d: should be online", i)
		}
		if node.CPUPercent == nil {
			t.Errorf("Node %d: CPUPercent should not be nil", i)
		}
		if node.RAMPercent == nil {
			t.Errorf("Node %d: RAMPercent should not be nil", i)
		}
		if node.DiskPercent == nil {
			t.Errorf("Node %d: DiskPercent should not be nil", i)
		}

		// Verify metrics are in valid range (0-100)
		if node.CPUPercent != nil && (*node.CPUPercent < 0 || *node.CPUPercent > 100) {
			t.Errorf("Node %d: CPUPercent out of range: %.2f", i, *node.CPUPercent)
		}
		if node.RAMPercent != nil && (*node.RAMPercent < 0 || *node.RAMPercent > 100) {
			t.Errorf("Node %d: RAMPercent out of range: %.2f", i, *node.RAMPercent)
		}
		if node.DiskPercent != nil && (*node.DiskPercent < 0 || *node.DiskPercent > 100) {
			t.Errorf("Node %d: DiskPercent out of range: %.2f", i, *node.DiskPercent)
		}
	}

	// Verify Kubernetes cluster has nodes
	if len(mock.Kubernetes) == 0 {
		t.Fatal("Should have at least one Kubernetes cluster")
	}

	k8s := mock.Kubernetes[0]
	if k8s.Status != "healthy" {
		t.Errorf("Expected healthy cluster, got %q", k8s.Status)
	}

	if len(k8s.Nodes) == 0 {
		t.Error("Cluster should have nodes")
	}
}

func TestMockFastfetchData(t *testing.T) {
	mock := MockFastfetchData()

	if mock == nil {
		t.Fatal("MockFastfetchData returned nil")
	}

	if mock.IsEmpty() {
		t.Error("Mock data should not be empty")
	}

	if mock.OS.Result == "" {
		t.Error("OS should be populated")
	}

	if mock.Kernel.Result == "" {
		t.Error("Kernel should be populated")
	}

	if mock.CPU.Result == "" {
		t.Error("CPU should be populated")
	}

	if mock.Memory.Result == "" {
		t.Error("Memory should be populated")
	}

	// Test FormatCompact works
	lines := mock.FormatCompact()
	if len(lines) == 0 {
		t.Error("FormatCompact should return lines")
	}

	// Test FormatForDisplay works
	displayLines := mock.FormatForDisplay()
	if len(displayLines) == 0 {
		t.Error("FormatForDisplay should return lines")
	}
}

// Benchmark mock data generation
func BenchmarkMockClaudeUsage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = MockClaudeUsage()
	}
}

func BenchmarkMockBillingData(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = MockBillingData()
	}
}

func BenchmarkMockBillingDataWithHistory(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = MockBillingDataWithHistory()
	}
}

func BenchmarkMockInfraStatus(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = MockInfraStatus()
	}
}

func BenchmarkMockFastfetchData(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = MockFastfetchData()
	}
}
