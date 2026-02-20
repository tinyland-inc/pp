package billing

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock clients
// ---------------------------------------------------------------------------

type mockCivoClient struct {
	charges   *CivoChargesResponse
	k8s       *CivoK8sResponse
	instances *CivoInstancesResponse
	sizes     *CivoSizesResponse

	chargesErr   error
	k8sErr       error
	instancesErr error
	sizesErr     error
}

func (m *mockCivoClient) GetCharges(ctx context.Context) (*CivoChargesResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.charges, m.chargesErr
}

func (m *mockCivoClient) GetKubernetes(ctx context.Context) (*CivoK8sResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.k8s, m.k8sErr
}

func (m *mockCivoClient) GetInstances(ctx context.Context) (*CivoInstancesResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.instances, m.instancesErr
}

func (m *mockCivoClient) GetSizes(ctx context.Context) (*CivoSizesResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.sizes, m.sizesErr
}

type mockDOClient struct {
	balance  *DOBalanceResponse
	k8s      *DOK8sResponse
	droplets *DODropletsResponse

	balanceErr  error
	k8sErr      error
	dropletsErr error
}

func (m *mockDOClient) GetBalance(ctx context.Context) (*DOBalanceResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.balance, m.balanceErr
}

func (m *mockDOClient) GetKubernetes(ctx context.Context) (*DOK8sResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.k8s, m.k8sErr
}

func (m *mockDOClient) GetDroplets(ctx context.Context) (*DODropletsResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return m.droplets, m.dropletsErr
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func buildCivoMock() *mockCivoClient {
	return &mockCivoClient{
		charges: &CivoChargesResponse{
			Items: []CivoCharge{
				{Code: "k8s-small", Label: "K8s Cluster", TotalCost: 25.50},
				{Code: "instance-1", Label: "Web Server", TotalCost: 10.00},
			},
		},
		k8s: &CivoK8sResponse{
			Items: []CivoK8sCluster{
				{ID: "k8s-1", Name: "k3s-cluster-1", Status: "ACTIVE", MonthlyCost: 20.00},
				{ID: "k8s-2", Name: "k3s-cluster-2", Status: "ACTIVE", MonthlyCost: 15.00},
			},
		},
		instances: &CivoInstancesResponse{
			Items: []CivoInstance{
				{ID: "inst-1", Hostname: "web-01", Status: "ACTIVE", MonthlyCost: 10.00},
			},
		},
	}
}

func buildDOMock() *mockDOClient {
	return &mockDOClient{
		balance: &DOBalanceResponse{
			MonthToDateBalance: "45.67",
			AccountBalance:     "100.00",
			MonthToDateUsage:   "45.67",
		},
		k8s: &DOK8sResponse{
			KubernetesClusters: []DOK8sCluster{
				{
					ID:   "doks-1",
					Name: "doks-production",
					Status: DOK8sStatus{State: "running"},
					NodePools: []DONodePool{
						{ID: "pool-1", Name: "worker-pool", Size: "s-2vcpu-4gb", Count: 3},
					},
				},
			},
		},
		droplets: &DODropletsResponse{
			Droplets: []DODroplet{
				{
					ID:   1,
					Name: "droplet-web-01",
					Size: DOSize{Slug: "s-1vcpu-1gb", PriceMonthly: 6.00, PriceHourly: 0.00893},
				},
				{
					ID:   2,
					Name: "droplet-db-01",
					Size: DOSize{Slug: "s-2vcpu-4gb", PriceMonthly: 24.00, PriceHourly: 0.03571},
				},
			},
		},
	}
}

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestName(t *testing.T) {
	c := New(Config{})
	if got := c.Name(); got != "billing" {
		t.Errorf("Name() = %q, want %q", got, "billing")
	}
}

func TestInterval_Default(t *testing.T) {
	c := New(Config{})
	if got := c.Interval(); got != DefaultInterval {
		t.Errorf("Interval() = %v, want %v", got, DefaultInterval)
	}
}

func TestInterval_Custom(t *testing.T) {
	want := 30 * time.Minute
	c := New(Config{Interval: want})
	if got := c.Interval(); got != want {
		t.Errorf("Interval() = %v, want %v", got, want)
	}
}

func TestInterval_ZeroUsesDefault(t *testing.T) {
	c := New(Config{Interval: 0})
	if got := c.Interval(); got != DefaultInterval {
		t.Errorf("Interval() = %v, want default %v", got, DefaultInterval)
	}
}

func TestCollect_CivoOnly(t *testing.T) {
	civo := buildCivoMock()
	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "test-key", Region: "NYC1"},
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report, ok := result.(*BillingReport)
	if !ok {
		t.Fatalf("Collect() returned %T, want *BillingReport", result)
	}

	if len(report.Providers) != 1 {
		t.Fatalf("Providers len = %d, want 1", len(report.Providers))
	}

	prov := report.Providers[0]
	if prov.Name != "civo" {
		t.Errorf("Provider.Name = %q, want %q", prov.Name, "civo")
	}
	if !prov.Connected {
		t.Error("Provider.Connected = false, want true")
	}
	if prov.Error != "" {
		t.Errorf("Provider.Error = %q, want empty", prov.Error)
	}

	// MonthToDate = charges (25.50+10.00) = 35.50 (charges preferred over estimation).
	if !floatEqual(prov.MonthToDate, 35.50) {
		t.Errorf("MonthToDate = %f, want 35.50", prov.MonthToDate)
	}

	// 2 k8s clusters + 1 instance = 3 resources
	if len(prov.Resources) != 3 {
		t.Fatalf("Resources len = %d, want 3", len(prov.Resources))
	}

	// Verify k8s cluster resources.
	if prov.Resources[0].Name != "k3s-cluster-1" {
		t.Errorf("Resources[0].Name = %q, want %q", prov.Resources[0].Name, "k3s-cluster-1")
	}
	if prov.Resources[0].Type != "kubernetes" {
		t.Errorf("Resources[0].Type = %q, want %q", prov.Resources[0].Type, "kubernetes")
	}
	if !floatEqual(prov.Resources[0].MonthlyCost, 20.00) {
		t.Errorf("Resources[0].MonthlyCost = %f, want 20.00", prov.Resources[0].MonthlyCost)
	}

	// Verify instance resource.
	if prov.Resources[2].Name != "web-01" {
		t.Errorf("Resources[2].Name = %q, want %q", prov.Resources[2].Name, "web-01")
	}
	if prov.Resources[2].Type != "instance" {
		t.Errorf("Resources[2].Type = %q, want %q", prov.Resources[2].Type, "instance")
	}

	// TotalMonthlyUSD should equal Civo month-to-date (charges-based).
	if !floatEqual(report.TotalMonthlyUSD, 35.50) {
		t.Errorf("TotalMonthlyUSD = %f, want 35.50", report.TotalMonthlyUSD)
	}
}

func TestCollect_DOOnly(t *testing.T) {
	do := buildDOMock()
	c := newWithClients(Config{
		DigitalOcean: &DOConfig{APIToken: "test-token"},
	}, nil, do)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)

	if len(report.Providers) != 1 {
		t.Fatalf("Providers len = %d, want 1", len(report.Providers))
	}

	prov := report.Providers[0]
	if prov.Name != "digitalocean" {
		t.Errorf("Provider.Name = %q, want %q", prov.Name, "digitalocean")
	}
	if !prov.Connected {
		t.Error("Provider.Connected = false, want true")
	}

	// Month-to-date from balance API.
	if !floatEqual(prov.MonthToDate, 45.67) {
		t.Errorf("MonthToDate = %f, want 45.67", prov.MonthToDate)
	}

	// Account balance.
	if !floatEqual(prov.Balance, 100.00) {
		t.Errorf("Balance = %f, want 100.00", prov.Balance)
	}

	// 1 DOKS cluster + 2 droplets = 3 resources.
	if len(prov.Resources) != 3 {
		t.Fatalf("Resources len = %d, want 3", len(prov.Resources))
	}

	// Check droplet pricing.
	foundWeb := false
	for _, r := range prov.Resources {
		if r.Name == "droplet-web-01" {
			foundWeb = true
			if !floatEqual(r.MonthlyCost, 6.00) {
				t.Errorf("droplet-web-01 MonthlyCost = %f, want 6.00", r.MonthlyCost)
			}
			if !floatEqual(r.HourlyCost, 0.00893) {
				t.Errorf("droplet-web-01 HourlyCost = %f, want 0.00893", r.HourlyCost)
			}
			if r.Type != "droplet" {
				t.Errorf("droplet-web-01 Type = %q, want %q", r.Type, "droplet")
			}
		}
	}
	if !foundWeb {
		t.Error("droplet-web-01 not found in resources")
	}
}

func TestCollect_BothProviders(t *testing.T) {
	civo := buildCivoMock()
	do := buildDOMock()

	c := newWithClients(Config{
		Civo:         &CivoConfig{APIKey: "key"},
		DigitalOcean: &DOConfig{APIToken: "token"},
		BudgetUSD:    200.00,
	}, civo, do)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)

	if len(report.Providers) != 2 {
		t.Fatalf("Providers len = %d, want 2", len(report.Providers))
	}

	// Total = Civo charges (35.50) + DO (45.67) = 81.17
	expectedTotal := 35.50 + 45.67
	if !floatEqual(report.TotalMonthlyUSD, expectedTotal) {
		t.Errorf("TotalMonthlyUSD = %f, want %f", report.TotalMonthlyUSD, expectedTotal)
	}
}

func TestCollect_CivoError_DOStillWorks(t *testing.T) {
	civo := &mockCivoClient{
		k8sErr: errors.New("civo API unavailable"),
	}
	do := buildDOMock()

	c := newWithClients(Config{
		Civo:         &CivoConfig{APIKey: "key"},
		DigitalOcean: &DOConfig{APIToken: "token"},
	}, civo, do)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)

	if len(report.Providers) != 2 {
		t.Fatalf("Providers len = %d, want 2", len(report.Providers))
	}

	// Find Civo provider.
	var civoProv, doProv *ProviderBilling
	for i := range report.Providers {
		switch report.Providers[i].Name {
		case "civo":
			civoProv = &report.Providers[i]
		case "digitalocean":
			doProv = &report.Providers[i]
		}
	}

	if civoProv == nil {
		t.Fatal("civo provider not found")
	}
	if civoProv.Connected {
		t.Error("civo.Connected = true, want false (API error)")
	}
	if civoProv.Error == "" {
		t.Error("civo.Error is empty, want error message")
	}

	if doProv == nil {
		t.Fatal("digitalocean provider not found")
	}
	if !doProv.Connected {
		t.Error("digitalocean.Connected = false, want true")
	}

	// TotalMonthlyUSD should only include DO (Civo failed).
	if !floatEqual(report.TotalMonthlyUSD, 45.67) {
		t.Errorf("TotalMonthlyUSD = %f, want 45.67 (DO only)", report.TotalMonthlyUSD)
	}

	// Collector should still be healthy since DO succeeded.
	if !c.Healthy() {
		t.Error("Healthy() = false, want true (at least one provider worked)")
	}
}

func TestCollect_DOError_CivoStillWorks(t *testing.T) {
	civo := buildCivoMock()
	do := &mockDOClient{
		balanceErr: errors.New("DO API unavailable"),
	}

	c := newWithClients(Config{
		Civo:         &CivoConfig{APIKey: "key"},
		DigitalOcean: &DOConfig{APIToken: "token"},
	}, civo, do)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)

	var civoProv, doProv *ProviderBilling
	for i := range report.Providers {
		switch report.Providers[i].Name {
		case "civo":
			civoProv = &report.Providers[i]
		case "digitalocean":
			doProv = &report.Providers[i]
		}
	}

	if civoProv == nil || !civoProv.Connected {
		t.Error("civo should be connected")
	}
	if doProv == nil || doProv.Connected {
		t.Error("digitalocean should be disconnected")
	}
	if doProv != nil && doProv.Error == "" {
		t.Error("digitalocean.Error should not be empty")
	}

	// TotalMonthlyUSD should only include Civo charges (25.50+10.00 = 35.50).
	if !floatEqual(report.TotalMonthlyUSD, 35.50) {
		t.Errorf("TotalMonthlyUSD = %f, want 35.50 (Civo only)", report.TotalMonthlyUSD)
	}

	if !c.Healthy() {
		t.Error("Healthy() = false, want true (Civo succeeded)")
	}
}

func TestCollect_BudgetPercentage(t *testing.T) {
	civo := buildCivoMock()

	c := newWithClients(Config{
		Civo:      &CivoConfig{APIKey: "key"},
		BudgetUSD: 100.00,
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)

	if !floatEqual(report.BudgetUSD, 100.00) {
		t.Errorf("BudgetUSD = %f, want 100.00", report.BudgetUSD)
	}

	// Civo charges = 35.50, budget = 100 => 35.5%
	expectedPercent := 35.50
	if !floatEqual(report.BudgetPercent, expectedPercent) {
		t.Errorf("BudgetPercent = %f, want %f", report.BudgetPercent, expectedPercent)
	}
}

func TestCollect_ZeroBudget(t *testing.T) {
	civo := buildCivoMock()

	c := newWithClients(Config{
		Civo:      &CivoConfig{APIKey: "key"},
		BudgetUSD: 0,
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)

	if report.BudgetPercent != 0 {
		t.Errorf("BudgetPercent = %f, want 0 (no budget set)", report.BudgetPercent)
	}
	if report.BudgetUSD != 0 {
		t.Errorf("BudgetUSD = %f, want 0", report.BudgetUSD)
	}
}

func TestCollect_BothProvidersDisabled(t *testing.T) {
	c := newWithClients(Config{}, nil, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)

	if len(report.Providers) != 0 {
		t.Errorf("Providers len = %d, want 0", len(report.Providers))
	}
	if report.TotalMonthlyUSD != 0 {
		t.Errorf("TotalMonthlyUSD = %f, want 0", report.TotalMonthlyUSD)
	}

	// Should be healthy when nothing is configured (no failure).
	if !c.Healthy() {
		t.Error("Healthy() = false, want true (nothing to fail)")
	}
}

func TestCollect_SingleProviderCivoNilDOSet(t *testing.T) {
	do := buildDOMock()
	c := newWithClients(Config{
		DigitalOcean: &DOConfig{APIToken: "token"},
	}, nil, do)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)

	if len(report.Providers) != 1 {
		t.Fatalf("Providers len = %d, want 1", len(report.Providers))
	}
	if report.Providers[0].Name != "digitalocean" {
		t.Errorf("Provider.Name = %q, want %q", report.Providers[0].Name, "digitalocean")
	}
}

func TestHealthy_AfterSuccess(t *testing.T) {
	civo := buildCivoMock()
	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "key"},
	}, civo, nil)

	// Initially healthy.
	if !c.Healthy() {
		t.Fatal("Healthy() = false before first collection, want true")
	}

	_, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if !c.Healthy() {
		t.Error("Healthy() = false after success, want true")
	}
}

func TestHealthy_AfterAllProvidersFail(t *testing.T) {
	civo := &mockCivoClient{k8sErr: errors.New("fail")}
	do := &mockDOClient{balanceErr: errors.New("fail")}

	c := newWithClients(Config{
		Civo:         &CivoConfig{APIKey: "key"},
		DigitalOcean: &DOConfig{APIToken: "token"},
	}, civo, do)

	_, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() should not return error: %v", err)
	}

	if c.Healthy() {
		t.Error("Healthy() = true after all providers failed, want false")
	}
}

func TestHealthy_RecoverAfterFailure(t *testing.T) {
	civo := &mockCivoClient{k8sErr: errors.New("temporary")}
	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "key"},
	}, civo, nil)

	// Fail first.
	_, _ = c.Collect(context.Background())
	if c.Healthy() {
		t.Error("Healthy() = true after failure, want false")
	}

	// Fix the mock.
	civo.k8sErr = nil
	civo.k8s = &CivoK8sResponse{}
	civo.instances = &CivoInstancesResponse{}

	_, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if !c.Healthy() {
		t.Error("Healthy() = false after recovery, want true")
	}
}

func TestCollect_ContextCancellation(t *testing.T) {
	civo := buildCivoMock()
	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "key"},
	}, civo, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := c.Collect(ctx)
	if err == nil {
		t.Fatal("Collect() should have returned an error for cancelled context")
	}

	if c.Healthy() {
		t.Error("Healthy() = true after context cancellation, want false")
	}
}

func TestCollect_EmptyResourceLists(t *testing.T) {
	civo := &mockCivoClient{
		charges:   &CivoChargesResponse{Items: []CivoCharge{}},
		k8s:       &CivoK8sResponse{Items: []CivoK8sCluster{}},
		instances: &CivoInstancesResponse{Items: []CivoInstance{}},
	}
	do := &mockDOClient{
		balance:  &DOBalanceResponse{MonthToDateBalance: "0.00", AccountBalance: "50.00"},
		k8s:      &DOK8sResponse{KubernetesClusters: []DOK8sCluster{}},
		droplets: &DODropletsResponse{Droplets: []DODroplet{}},
	}

	c := newWithClients(Config{
		Civo:         &CivoConfig{APIKey: "key"},
		DigitalOcean: &DOConfig{APIToken: "token"},
	}, civo, do)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)

	for _, prov := range report.Providers {
		if !prov.Connected {
			t.Errorf("Provider %q.Connected = false, want true", prov.Name)
		}
		if len(prov.Resources) != 0 {
			t.Errorf("Provider %q.Resources len = %d, want 0", prov.Name, len(prov.Resources))
		}
	}

	if !floatEqual(report.TotalMonthlyUSD, 0) {
		t.Errorf("TotalMonthlyUSD = %f, want 0", report.TotalMonthlyUSD)
	}
}

func TestCollect_Timestamp(t *testing.T) {
	c := newWithClients(Config{}, nil, nil)

	before := time.Now()
	result, err := c.Collect(context.Background())
	after := time.Now()

	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	if report.Timestamp.Before(before) || report.Timestamp.After(after) {
		t.Errorf("Timestamp = %v, want between %v and %v", report.Timestamp, before, after)
	}
}

func TestCollect_CivoK8sError_PartialFailure(t *testing.T) {
	civo := &mockCivoClient{
		k8sErr: errors.New("k8s endpoint down"),
	}

	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "key"},
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	prov := report.Providers[0]

	// K8s error means civo is disconnected.
	if prov.Connected {
		t.Error("civo.Connected = true, want false (k8s endpoint failed)")
	}
	if prov.Error == "" {
		t.Error("civo.Error should contain k8s error")
	}
}

func TestCollect_DODropletsError_PartialFailure(t *testing.T) {
	do := &mockDOClient{
		balance:     &DOBalanceResponse{MonthToDateBalance: "10.00", AccountBalance: "50.00"},
		k8s:         &DOK8sResponse{KubernetesClusters: []DOK8sCluster{}},
		dropletsErr: errors.New("droplets endpoint down"),
	}

	c := newWithClients(Config{
		DigitalOcean: &DOConfig{APIToken: "token"},
	}, nil, do)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	prov := report.Providers[0]

	if prov.Connected {
		t.Error("digitalocean.Connected = true, want false (droplets endpoint failed)")
	}
	if prov.Error == "" {
		t.Error("digitalocean.Error should contain droplets error")
	}
}

func TestCollect_ProvidersListNeverNil(t *testing.T) {
	c := newWithClients(Config{}, nil, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	if report.Providers == nil {
		t.Error("Providers is nil, want empty slice")
	}
}

func TestDOBalanceResponse_ParseMonthToDate(t *testing.T) {
	tests := []struct {
		name    string
		resp    *DOBalanceResponse
		want    float64
		wantErr bool
	}{
		{
			name: "valid",
			resp: &DOBalanceResponse{MonthToDateBalance: "12.34"},
			want: 12.34,
		},
		{
			name: "zero",
			resp: &DOBalanceResponse{MonthToDateBalance: "0.00"},
			want: 0,
		},
		{
			name: "empty string",
			resp: &DOBalanceResponse{MonthToDateBalance: ""},
			want: 0,
		},
		{
			name: "nil response",
			resp: nil,
			want: 0,
		},
		{
			name:    "invalid",
			resp:    &DOBalanceResponse{MonthToDateBalance: "not-a-number"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.resp.ParseMonthToDate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMonthToDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !floatEqual(got, tt.want) {
				t.Errorf("ParseMonthToDate() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestDOBalanceResponse_ParseAccountBalance(t *testing.T) {
	tests := []struct {
		name    string
		resp    *DOBalanceResponse
		want    float64
		wantErr bool
	}{
		{
			name: "valid",
			resp: &DOBalanceResponse{AccountBalance: "50.00"},
			want: 50.00,
		},
		{
			name: "nil response",
			resp: nil,
			want: 0,
		},
		{
			name:    "invalid",
			resp:    &DOBalanceResponse{AccountBalance: "abc"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.resp.ParseAccountBalance()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAccountBalance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !floatEqual(got, tt.want) {
				t.Errorf("ParseAccountBalance() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestCollect_CivoZeroCost_EnrichedFromSizes(t *testing.T) {
	civo := &mockCivoClient{
		k8s: &CivoK8sResponse{
			Items: []CivoK8sCluster{
				{
					ID:              "k8s-1",
					Name:            "bitter-darkness",
					Status:          "ACTIVE",
					MonthlyCost:     0, // API returns 0
					NumTargetNodes:  3,
					TargetNodesSize: "g4s.kube.large",
				},
			},
		},
		instances: &CivoInstancesResponse{
			Items: []CivoInstance{},
		},
		sizes: &CivoSizesResponse{
			Items: []CivoSize{
				{Name: "g4s.kube.small", PriceMonthly: 10.00},
				{Name: "g4s.kube.medium", PriceMonthly: 20.00},
				{Name: "g4s.kube.large", PriceMonthly: 40.00},
			},
		},
	}

	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "test-key"},
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	prov := report.Providers[0]

	// 3 nodes * $40/mo = $120.00
	if !floatEqual(prov.MonthToDate, 120.00) {
		t.Errorf("MonthToDate = %f, want 120.00 (3 * g4s.kube.large @ $40)", prov.MonthToDate)
	}

	if len(prov.Resources) != 1 {
		t.Fatalf("Resources len = %d, want 1", len(prov.Resources))
	}
	if !floatEqual(prov.Resources[0].MonthlyCost, 120.00) {
		t.Errorf("Resources[0].MonthlyCost = %f, want 120.00", prov.Resources[0].MonthlyCost)
	}
}

func TestCollect_CivoZeroCost_FallbackPricing(t *testing.T) {
	civo := &mockCivoClient{
		k8s: &CivoK8sResponse{
			Items: []CivoK8sCluster{
				{
					ID:              "k8s-1",
					Name:            "test-cluster",
					Status:          "ACTIVE",
					MonthlyCost:     0,
					NumTargetNodes:  4,
					TargetNodesSize: "g4p.kube.medium",
				},
			},
		},
		instances: &CivoInstancesResponse{},
		// Sizes API returns no pricing (or errors out).
		sizes:    &CivoSizesResponse{Items: []CivoSize{}},
		sizesErr: nil,
	}

	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "test-key"},
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	prov := report.Providers[0]

	// 4 nodes * $60/mo (fallback price for g4p.kube.medium) = $240.00
	if !floatEqual(prov.MonthToDate, 240.00) {
		t.Errorf("MonthToDate = %f, want 240.00 (4 * g4p.kube.medium @ $60 fallback)", prov.MonthToDate)
	}
}

func TestCollect_CivoZeroCost_SizesAPIError(t *testing.T) {
	civo := &mockCivoClient{
		k8s: &CivoK8sResponse{
			Items: []CivoK8sCluster{
				{
					ID:              "k8s-1",
					Name:            "test-cluster",
					Status:          "ACTIVE",
					MonthlyCost:     0,
					NumTargetNodes:  2,
					TargetNodesSize: "g4s.kube.large",
				},
			},
		},
		instances: &CivoInstancesResponse{},
		sizesErr:  errors.New("sizes API unavailable"),
	}

	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "test-key"},
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	prov := report.Providers[0]

	// Sizes API failed, falls back to hardcoded: 2 * $40 = $80
	if !floatEqual(prov.MonthToDate, 80.00) {
		t.Errorf("MonthToDate = %f, want 80.00 (fallback pricing)", prov.MonthToDate)
	}

	// Should still be connected (sizes failure is non-fatal).
	if !prov.Connected {
		t.Error("civo.Connected = false, want true (sizes error is best-effort)")
	}
}

func TestCollect_CivoNonZeroCost_NotEnriched(t *testing.T) {
	civo := &mockCivoClient{
		k8s: &CivoK8sResponse{
			Items: []CivoK8sCluster{
				{
					ID:              "k8s-1",
					Name:            "has-cost",
					Status:          "ACTIVE",
					MonthlyCost:     50.00, // API returns real cost
					NumTargetNodes:  3,
					TargetNodesSize: "g4s.kube.large",
				},
			},
		},
		instances: &CivoInstancesResponse{},
		sizes:     &CivoSizesResponse{Items: []CivoSize{}},
	}

	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "test-key"},
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	prov := report.Providers[0]

	// Should use the API-provided cost, not enrichment.
	if !floatEqual(prov.MonthToDate, 50.00) {
		t.Errorf("MonthToDate = %f, want 50.00 (API-provided cost)", prov.MonthToDate)
	}
}

// Compile-time check that Collector satisfies the collector interface contract.
// Duck-typed to avoid importing pkg/collectors.
type collectorIface interface {
	Name() string
	Collect(ctx context.Context) (interface{}, error)
	Interval() time.Duration
	Healthy() bool
}

var _ collectorIface = (*Collector)(nil)

// Ensure mock clients satisfy their interfaces.
var _ CivoClient = (*mockCivoClient)(nil)
var _ DOClient = (*mockDOClient)(nil)

// ---------------------------------------------------------------------------
// CIVO Charges API integration tests
// ---------------------------------------------------------------------------

func TestCollect_CivoChargesPreferred(t *testing.T) {
	// When charges API returns data, MonthToDate should use charges total
	// instead of resource estimation.
	civo := &mockCivoClient{
		charges: &CivoChargesResponse{
			Items: []CivoCharge{
				{Code: "k8s-cluster", Label: "K8s Cluster", TotalCost: 35.50},
				{Code: "lb-1", Label: "Load Balancer", TotalCost: 10.00},
			},
		},
		k8s: &CivoK8sResponse{
			Items: []CivoK8sCluster{
				{ID: "k8s-1", Name: "test-cluster", MonthlyCost: 40.00},
			},
		},
		instances: &CivoInstancesResponse{Items: []CivoInstance{}},
	}

	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "test-key"},
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	prov := report.Providers[0]

	// Charges total = 35.50 + 10.00 = 45.50 (NOT the resource estimate of 40.00)
	if !floatEqual(prov.MonthToDate, 45.50) {
		t.Errorf("MonthToDate = %f, want 45.50 (charges preferred over estimation)", prov.MonthToDate)
	}

	// Resources should still be populated for breakdown.
	if len(prov.Resources) != 1 {
		t.Errorf("Resources len = %d, want 1", len(prov.Resources))
	}
}

func TestCollect_CivoChargesErrorFallback(t *testing.T) {
	// When charges API fails, should fall back to resource estimation.
	civo := &mockCivoClient{
		chargesErr: errors.New("charges endpoint unavailable"),
		k8s: &CivoK8sResponse{
			Items: []CivoK8sCluster{
				{ID: "k8s-1", Name: "test-cluster", MonthlyCost: 60.00},
			},
		},
		instances: &CivoInstancesResponse{
			Items: []CivoInstance{
				{ID: "inst-1", Hostname: "web-01", MonthlyCost: 10.00},
			},
		},
	}

	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "test-key"},
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	prov := report.Providers[0]

	// Charges failed, so estimation: 60.00 + 10.00 = 70.00
	if !floatEqual(prov.MonthToDate, 70.00) {
		t.Errorf("MonthToDate = %f, want 70.00 (fallback to estimation)", prov.MonthToDate)
	}
	if !prov.Connected {
		t.Error("civo should still be connected (charges error is non-fatal)")
	}
}

func TestCollect_CivoEmptyChargesFallback(t *testing.T) {
	// When charges API returns empty items, should fall back to estimation.
	civo := &mockCivoClient{
		charges: &CivoChargesResponse{Items: []CivoCharge{}},
		k8s: &CivoK8sResponse{
			Items: []CivoK8sCluster{
				{ID: "k8s-1", Name: "test-cluster", MonthlyCost: 25.00},
			},
		},
		instances: &CivoInstancesResponse{Items: []CivoInstance{}},
	}

	c := newWithClients(Config{
		Civo: &CivoConfig{APIKey: "test-key"},
	}, civo, nil)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	report := result.(*BillingReport)
	prov := report.Providers[0]

	// Empty charges → fallback to estimation: 25.00
	if !floatEqual(prov.MonthToDate, 25.00) {
		t.Errorf("MonthToDate = %f, want 25.00 (empty charges = fallback)", prov.MonthToDate)
	}
}
