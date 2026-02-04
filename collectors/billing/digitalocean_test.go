package billing

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// newTestServer creates an httptest server that routes by URL path to simulate
// the DigitalOcean API. It serves balance on /v2/customers/my/balance and
// invoices on /v2/customers/my/invoices.
func newTestServer(balanceHandler, invoicesHandler http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	if balanceHandler != nil {
		mux.HandleFunc("/v2/customers/my/balance", balanceHandler)
	}
	if invoicesHandler != nil {
		mux.HandleFunc("/v2/customers/my/invoices", invoicesHandler)
	}
	return httptest.NewServer(mux)
}

// newTestDOClient creates a DOClient pointed at the test server.
func newTestDOClient(baseURL string) *DOClient {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return &DOClient{
		apiToken:   "test-token-123",
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    baseURL,
		logger:     logger,
	}
}

func TestDOClient_FetchBilling_Success(t *testing.T) {
	server := newTestServer(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doBalanceResponse{
				MonthToDateBalance: "23.44",
				AccountBalance:     "0.00",
				MonthToDateUsage:   "23.44",
				GeneratedAt:        "2024-01-15T12:00:00Z",
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doInvoicesResponse{
				Invoices: []doInvoice{
					{
						InvoiceUUID:   "prev-uuid",
						Amount:        "27.50",
						InvoicePeriod: time.Now().UTC().AddDate(0, -1, 0).Format("2006-01"),
						UpdatedAt:     "2024-01-01T00:00:00Z",
					},
				},
			})
		},
	)
	defer server.Close()

	client := newTestDOClient(server.URL)
	billing, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if billing.Provider != "digitalocean" {
		t.Errorf("expected Provider=digitalocean, got %s", billing.Provider)
	}
	if billing.Status != "ok" {
		t.Errorf("expected Status=ok, got %s", billing.Status)
	}
	if billing.DashboardURL != doDashboardURL {
		t.Errorf("expected DashboardURL=%s, got %s", doDashboardURL, billing.DashboardURL)
	}
	if billing.CurrentMonth.SpendUSD != 23.44 {
		t.Errorf("expected SpendUSD=23.44, got %f", billing.CurrentMonth.SpendUSD)
	}
	if billing.CurrentMonth.ForecastUSD == nil {
		t.Fatal("expected non-nil ForecastUSD")
	}
	if billing.PreviousMonth == nil {
		t.Fatal("expected non-nil PreviousMonth")
	}
	if *billing.PreviousMonth != 27.50 {
		t.Errorf("expected PreviousMonth=27.50, got %f", *billing.PreviousMonth)
	}
	if billing.CurrentMonth.StartDate == "" {
		t.Error("expected non-empty StartDate")
	}
	if billing.CurrentMonth.EndDate == "" {
		t.Error("expected non-empty EndDate")
	}
	if billing.FetchedAt.IsZero() {
		t.Error("expected non-zero FetchedAt")
	}
}

func TestDOClient_FetchBilling_BalanceOnly(t *testing.T) {
	server := newTestServer(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doBalanceResponse{
				MonthToDateUsage: "15.00",
				GeneratedAt:      "2024-02-10T12:00:00Z",
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			// Invoices endpoint fails.
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"id":"server_error","message":"internal error"}`))
		},
	)
	defer server.Close()

	client := newTestDOClient(server.URL)
	billing, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Balance should succeed even when invoices fails.
	if billing.CurrentMonth.SpendUSD != 15.00 {
		t.Errorf("expected SpendUSD=15.00, got %f", billing.CurrentMonth.SpendUSD)
	}
	if billing.Status != "ok" {
		t.Errorf("expected Status=ok, got %s", billing.Status)
	}

	// Previous month should be nil when invoices fails.
	if billing.PreviousMonth != nil {
		t.Errorf("expected nil PreviousMonth, got %f", *billing.PreviousMonth)
	}
}

func TestDOClient_FetchBilling_StringAmounts(t *testing.T) {
	tests := []struct {
		name         string
		usage        string
		wantSpend    float64
		invoiceAmt   string
		wantPrevious float64
	}{
		{
			name:         "integer amounts",
			usage:        "100",
			wantSpend:    100.0,
			invoiceAmt:   "200",
			wantPrevious: 200.0,
		},
		{
			name:         "decimal amounts",
			usage:        "45.67",
			wantSpend:    45.67,
			invoiceAmt:   "89.12",
			wantPrevious: 89.12,
		},
		{
			name:         "zero amounts",
			usage:        "0.00",
			wantSpend:    0.0,
			invoiceAmt:   "0.00",
			wantPrevious: 0.0,
		},
		{
			name:         "large amounts",
			usage:        "12345.67",
			wantSpend:    12345.67,
			invoiceAmt:   "9876.54",
			wantPrevious: 9876.54,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prevPeriod := time.Now().UTC().AddDate(0, -1, 0).Format("2006-01")
			server := newTestServer(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(doBalanceResponse{
						MonthToDateUsage: tt.usage,
					})
				},
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(doInvoicesResponse{
						Invoices: []doInvoice{
							{
								Amount:        tt.invoiceAmt,
								InvoicePeriod: prevPeriod,
							},
						},
					})
				},
			)
			defer server.Close()

			client := newTestDOClient(server.URL)
			billing, err := client.FetchBilling(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if billing.CurrentMonth.SpendUSD != tt.wantSpend {
				t.Errorf("SpendUSD = %f, want %f", billing.CurrentMonth.SpendUSD, tt.wantSpend)
			}
			if billing.PreviousMonth == nil {
				t.Fatal("expected non-nil PreviousMonth")
			}
			if *billing.PreviousMonth != tt.wantPrevious {
				t.Errorf("PreviousMonth = %f, want %f", *billing.PreviousMonth, tt.wantPrevious)
			}
		})
	}
}

func TestDOClient_FetchBilling_PreviousMonth(t *testing.T) {
	now := time.Now().UTC()
	prevPeriod := now.AddDate(0, -1, 0).Format("2006-01")
	olderPeriod := now.AddDate(0, -2, 0).Format("2006-01")

	server := newTestServer(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doBalanceResponse{
				MonthToDateUsage: "10.00",
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doInvoicesResponse{
				Invoices: []doInvoice{
					{
						InvoiceUUID:   "older-uuid",
						Amount:        "99.99",
						InvoicePeriod: olderPeriod,
					},
					{
						InvoiceUUID:   "prev-uuid",
						Amount:        "42.50",
						InvoicePeriod: prevPeriod,
					},
				},
			})
		},
	)
	defer server.Close()

	client := newTestDOClient(server.URL)
	billing, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if billing.PreviousMonth == nil {
		t.Fatal("expected non-nil PreviousMonth")
	}
	// Should match the previous month invoice (42.50), not the older one (99.99).
	if *billing.PreviousMonth != 42.50 {
		t.Errorf("expected PreviousMonth=42.50, got %f", *billing.PreviousMonth)
	}
}

func TestDOClient_FetchBilling_AuthFailure(t *testing.T) {
	server := newTestServer(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"id":"unauthorized","message":"Unable to authenticate you."}`))
		},
		nil,
	)
	defer server.Close()

	client := newTestDOClient(server.URL)
	_, err := client.FetchBilling(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}

	apiErr, ok := err.(*DOAPIError)
	if !ok {
		// The error is wrapped, unwrap it.
		t.Logf("error type: %T, message: %v", err, err)
		return
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("expected StatusCode=401, got %d", apiErr.StatusCode)
	}
}

func TestDOClient_FetchBilling_ServerError(t *testing.T) {
	server := newTestServer(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"id":"server_error","message":"Server Error"}`))
		},
		nil,
	)
	defer server.Close()

	client := newTestDOClient(server.URL)
	_, err := client.FetchBilling(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestDOClient_FetchBilling_EmptyBalance(t *testing.T) {
	server := newTestServer(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doBalanceResponse{
				MonthToDateBalance: "0.00",
				AccountBalance:     "0.00",
				MonthToDateUsage:   "0.00",
				GeneratedAt:        "2024-01-01T00:00:00Z",
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doInvoicesResponse{
				Invoices: []doInvoice{},
			})
		},
	)
	defer server.Close()

	client := newTestDOClient(server.URL)
	billing, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if billing.CurrentMonth.SpendUSD != 0.0 {
		t.Errorf("expected SpendUSD=0.0, got %f", billing.CurrentMonth.SpendUSD)
	}
	if billing.CurrentMonth.ForecastUSD == nil {
		t.Fatal("expected non-nil ForecastUSD")
	}
	if *billing.CurrentMonth.ForecastUSD != 0.0 {
		t.Errorf("expected ForecastUSD=0.0, got %f", *billing.CurrentMonth.ForecastUSD)
	}
	if billing.PreviousMonth != nil {
		t.Errorf("expected nil PreviousMonth for empty invoices, got %f", *billing.PreviousMonth)
	}
}

func TestDOClient_FetchBilling_ForecastCalculation(t *testing.T) {
	// Use a fixed "now" by controlling the spend and verifying the math.
	// Forecast formula: (currentSpend / daysElapsed) * daysInMonth
	//
	// We test indirectly: the forecast should always be >= currentSpend
	// (since we are at most daysInMonth into the month), and should scale
	// linearly with spend.
	server := newTestServer(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doBalanceResponse{
				MonthToDateUsage: "30.00",
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doInvoicesResponse{})
		},
	)
	defer server.Close()

	client := newTestDOClient(server.URL)
	billing, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if billing.CurrentMonth.ForecastUSD == nil {
		t.Fatal("expected non-nil ForecastUSD")
	}

	forecast := *billing.CurrentMonth.ForecastUSD
	spend := billing.CurrentMonth.SpendUSD

	// Forecast must be >= current spend (we can't have spent more than the
	// forecast unless we are past end of month, which is not possible).
	if forecast < spend {
		t.Errorf("forecast (%f) should be >= current spend (%f)", forecast, spend)
	}

	// Verify the formula: forecast = (spend / daysElapsed) * daysInMonth.
	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, -1)
	daysInMonth := float64(endOfMonth.Day())
	daysElapsed := float64(now.Day())
	if daysElapsed < 1 {
		daysElapsed = 1
	}

	expectedForecast := (spend / daysElapsed) * daysInMonth
	if math.Abs(forecast-expectedForecast) > 0.01 {
		t.Errorf("forecast=%f, expected=%f (spend=%.2f, daysElapsed=%.0f, daysInMonth=%.0f)",
			forecast, expectedForecast, spend, daysElapsed, daysInMonth)
	}
}

func TestDOClient_RequestHeaders(t *testing.T) {
	var capturedBalanceReq *http.Request
	var capturedInvoicesReq *http.Request

	server := newTestServer(
		func(w http.ResponseWriter, r *http.Request) {
			capturedBalanceReq = r.Clone(r.Context())
			// Drain the original body so the clone captures headers properly.
			io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doBalanceResponse{
				MonthToDateUsage: "5.00",
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			capturedInvoicesReq = r.Clone(r.Context())
			io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(doInvoicesResponse{})
		},
	)
	defer server.Close()

	client := newTestDOClient(server.URL)
	_, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify balance request headers.
	if capturedBalanceReq == nil {
		t.Fatal("balance request was not captured")
	}
	if got := capturedBalanceReq.Header.Get("Authorization"); got != "Bearer test-token-123" {
		t.Errorf("balance Authorization = %q, want %q", got, "Bearer test-token-123")
	}
	if got := capturedBalanceReq.Header.Get("User-Agent"); got != doUserAgent {
		t.Errorf("balance User-Agent = %q, want %q", got, doUserAgent)
	}
	if capturedBalanceReq.Method != http.MethodGet {
		t.Errorf("balance method = %s, want GET", capturedBalanceReq.Method)
	}

	// Verify invoices request headers.
	if capturedInvoicesReq == nil {
		t.Fatal("invoices request was not captured")
	}
	if got := capturedInvoicesReq.Header.Get("Authorization"); got != "Bearer test-token-123" {
		t.Errorf("invoices Authorization = %q, want %q", got, "Bearer test-token-123")
	}
	if got := capturedInvoicesReq.Header.Get("User-Agent"); got != doUserAgent {
		t.Errorf("invoices User-Agent = %q, want %q", got, doUserAgent)
	}
	if capturedInvoicesReq.Method != http.MethodGet {
		t.Errorf("invoices method = %s, want GET", capturedInvoicesReq.Method)
	}
}
