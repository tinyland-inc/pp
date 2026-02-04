package billing

import (
	"context"
	"strings"
	"testing"
)

func TestDreamHostClient_FetchBilling_Stub(t *testing.T) {
	client := NewDreamHostClient("test-api-key", nil)

	result, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("FetchBilling() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("FetchBilling() returned nil")
	}
	if result.Provider != "dreamhost" {
		t.Errorf("Provider = %q, want %q", result.Provider, "dreamhost")
	}
	if result.Status != "limited" {
		t.Errorf("Status = %q, want %q", result.Status, "limited")
	}
	if !strings.Contains(result.AccountName, "DreamHost") {
		t.Errorf("AccountName = %q, want string containing %q",
			result.AccountName, "DreamHost")
	}
	if result.CurrentMonth.StartDate == "" {
		t.Error("CurrentMonth.StartDate is empty")
	}
	if result.CurrentMonth.EndDate == "" {
		t.Error("CurrentMonth.EndDate is empty")
	}
	if result.FetchedAt.IsZero() {
		t.Error("FetchedAt is zero")
	}
}

func TestDreamHostClient_DashboardURL(t *testing.T) {
	client := NewDreamHostClient("test-api-key", nil)

	result, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("FetchBilling() unexpected error: %v", err)
	}
	if !strings.Contains(result.DashboardURL, "dreamhost.com") {
		t.Errorf("DashboardURL = %q, want URL containing %q",
			result.DashboardURL, "dreamhost.com")
	}
}

func TestDreamHostClient_NilLogger(t *testing.T) {
	// Ensure NewDreamHostClient does not panic with nil logger.
	client := NewDreamHostClient("key", nil)
	if client == nil {
		t.Fatal("NewDreamHostClient returned nil")
	}
}
