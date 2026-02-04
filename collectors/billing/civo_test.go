package billing

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// sampleCharges returns a deterministic set of charges for testing.
func sampleCharges() []civoCharge {
	return []civoCharge{
		{
			Code:             "instance",
			Label:            "Instance (g4s.kube.medium)",
			From:             "2024-01-01T00:00:00Z",
			To:               "2024-01-31T23:59:59Z",
			NumHours:         744,
			UnitPricePerHour: 0.01,
			Total:            7.44,
		},
		{
			Code:             "kubernetes",
			Label:            "Kubernetes cluster",
			From:             "2024-01-01T00:00:00Z",
			To:               "2024-01-31T23:59:59Z",
			NumHours:         744,
			UnitPricePerHour: 0.005,
			Total:            3.72,
		},
	}
}

func TestCivoClient_FetchBilling_Success(t *testing.T) {
	charges := sampleCharges()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(charges)
	}))
	defer server.Close()

	client := NewCivoClient("test-api-key", "NYC1", nil)
	client.baseURL = server.URL

	result, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Provider != "civo" {
		t.Errorf("expected provider 'civo', got %q", result.Provider)
	}
	if result.AccountName != "NYC1" {
		t.Errorf("expected account name 'NYC1', got %q", result.AccountName)
	}
	if result.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", result.Status)
	}
	if result.DashboardURL != civoDashboardURL {
		t.Errorf("expected dashboard URL %q, got %q", civoDashboardURL, result.DashboardURL)
	}

	expectedSpend := 7.44 + 3.72
	if math.Abs(result.CurrentMonth.SpendUSD-expectedSpend) > 0.01 {
		t.Errorf("expected spend %.2f, got %.2f", expectedSpend, result.CurrentMonth.SpendUSD)
	}

	if result.CurrentMonth.ForecastUSD == nil {
		t.Error("expected forecast to be set, got nil")
	}

	if result.CurrentMonth.StartDate == "" {
		t.Error("expected start date to be set")
	}
	if result.CurrentMonth.EndDate == "" {
		t.Error("expected end date to be set")
	}
}

func TestCivoClient_FetchBilling_WrappedFormat(t *testing.T) {
	charges := sampleCharges()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		wrapped := civoWrappedResponse{Charges: charges}
		json.NewEncoder(w).Encode(wrapped)
	}))
	defer server.Close()

	client := NewCivoClient("test-api-key", "NYC1", nil)
	client.baseURL = server.URL

	result, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSpend := 7.44 + 3.72
	if math.Abs(result.CurrentMonth.SpendUSD-expectedSpend) > 0.01 {
		t.Errorf("expected spend %.2f, got %.2f", expectedSpend, result.CurrentMonth.SpendUSD)
	}
}

func TestCivoClient_FetchBilling_EmptyCharges(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewCivoClient("test-api-key", "NYC1", nil)
	client.baseURL = server.URL

	result, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.CurrentMonth.SpendUSD != 0 {
		t.Errorf("expected spend 0, got %.2f", result.CurrentMonth.SpendUSD)
	}
}

func TestCivoClient_FetchBilling_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"access denied"}`))
	}))
	defer server.Close()

	client := NewCivoClient("bad-api-key", "NYC1", nil)
	client.baseURL = server.URL

	_, err := client.FetchBilling(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}

	apiErr, ok := err.(*CivoAPIError)
	if !ok {
		t.Fatalf("expected *CivoAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status code 401, got %d", apiErr.StatusCode)
	}
}

func TestCivoClient_FetchBilling_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"internal error"}`))
	}))
	defer server.Close()

	client := NewCivoClient("test-api-key", "NYC1", nil)
	client.baseURL = server.URL

	_, err := client.FetchBilling(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}

	apiErr, ok := err.(*CivoAPIError)
	if !ok {
		t.Fatalf("expected *CivoAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status code 500, got %d", apiErr.StatusCode)
	}
}

func TestCivoClient_FetchBilling_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{this is not valid json`))
	}))
	defer server.Close()

	client := NewCivoClient("test-api-key", "NYC1", nil)
	client.baseURL = server.URL

	_, err := client.FetchBilling(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestCivoClient_FetchBilling_ForecastCalculation(t *testing.T) {
	// Use a fixed set of charges to verify forecast math.
	// If we have $10 spent over 10 days in a 30-day month, forecast should be $30.
	charges := []civoCharge{
		{
			Code:  "instance",
			Label: "Test instance",
			Total: 10.0,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(charges)
	}))
	defer server.Close()

	client := NewCivoClient("test-api-key", "NYC1", nil)
	client.baseURL = server.URL

	result, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.CurrentMonth.SpendUSD != 10.0 {
		t.Errorf("expected spend 10.0, got %.2f", result.CurrentMonth.SpendUSD)
	}

	// The forecast depends on when in the month the test runs.
	// Verify the math: forecast = (spend / daysElapsed) * daysInMonth
	now := time.Now().UTC()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	daysElapsed := now.Sub(startOfMonth).Hours() / 24.0
	totalDays := float64(DaysInMonth(now.Year(), now.Month()))
	expectedForecast := RoundCents((10.0 / daysElapsed) * totalDays)

	if result.CurrentMonth.ForecastUSD == nil {
		t.Fatal("expected forecast to be set, got nil")
	}

	if math.Abs(*result.CurrentMonth.ForecastUSD-expectedForecast) > 0.02 {
		t.Errorf("expected forecast ~%.2f, got %.2f", expectedForecast, *result.CurrentMonth.ForecastUSD)
	}
}

func TestCivoClient_FetchPreviousMonth_Success(t *testing.T) {
	charges := sampleCharges()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(charges)
	}))
	defer server.Close()

	client := NewCivoClient("test-api-key", "NYC1", nil)
	client.baseURL = server.URL

	result, err := client.FetchPreviousMonth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	expectedSpend := 7.44 + 3.72
	if math.Abs(*result-expectedSpend) > 0.01 {
		t.Errorf("expected previous month spend %.2f, got %.2f", expectedSpend, *result)
	}
}

func TestCivoClient_RequestHeaders(t *testing.T) {
	var capturedHeaders http.Header
	var capturedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		capturedMethod = r.Method

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewCivoClient("my-secret-key", "NYC1", nil)
	client.baseURL = server.URL

	_, err := client.FetchBilling(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedMethod != http.MethodGet {
		t.Errorf("expected GET method, got %q", capturedMethod)
	}

	auth := capturedHeaders.Get("Authorization")
	if auth != "bearer my-secret-key" {
		t.Errorf("expected Authorization 'bearer my-secret-key', got %q", auth)
	}

	ua := capturedHeaders.Get("User-Agent")
	if ua != civoUserAgent {
		t.Errorf("expected User-Agent %q, got %q", civoUserAgent, ua)
	}

	accept := capturedHeaders.Get("Accept")
	if accept != "application/json" {
		t.Errorf("expected Accept 'application/json', got %q", accept)
	}
}

func TestCivoClient_FetchBilling_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"message":"rate limited"}`))
	}))
	defer server.Close()

	client := NewCivoClient("test-api-key", "NYC1", nil)
	client.baseURL = server.URL

	_, err := client.FetchBilling(context.Background())
	if err == nil {
		t.Fatal("expected error for 429 response, got nil")
	}

	apiErr, ok := err.(*CivoAPIError)
	if !ok {
		t.Fatalf("expected *CivoAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status code 429, got %d", apiErr.StatusCode)
	}
}

func TestCivoClient_FetchBilling_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay to allow context cancellation to take effect.
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewCivoClient("test-api-key", "NYC1", nil)
	client.baseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := client.FetchBilling(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

