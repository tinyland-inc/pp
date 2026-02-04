package infra

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// --- Mock types ---

// mockTailscaleFetcher implements TailscaleFetcher for testing.
type mockTailscaleFetcher struct {
	status *collectors.TailscaleStatus
	err    error
}

func (m *mockTailscaleFetcher) FetchStatus(ctx context.Context) (*collectors.TailscaleStatus, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return m.status, m.err
}

// mockKubernetesFetcher implements KubernetesFetcher for testing.
type mockKubernetesFetcher struct {
	// clusters maps context name to result. If the value's error is non-nil,
	// FetchCluster returns that error.
	clusters map[string]*collectors.KubernetesCluster
	errors   map[string]error
}

func (m *mockKubernetesFetcher) FetchCluster(ctx context.Context, kubeContext KubeContextConfig) (*collectors.KubernetesCluster, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if err, ok := m.errors[kubeContext.Name]; ok && err != nil {
		return nil, err
	}
	if cluster, ok := m.clusters[kubeContext.Name]; ok {
		return cluster, nil
	}
	return nil, fmt.Errorf("no mock for context %q", kubeContext.Name)
}

// --- Helper functions ---

// testLogger returns a logger that only shows errors for clean test output.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// sampleTailscaleStatus returns a realistic TailscaleStatus for testing.
func sampleTailscaleStatus() *collectors.TailscaleStatus {
	return &collectors.TailscaleStatus{
		Tailnet:     "tinyland.ts.net",
		OnlineCount: 3,
		TotalCount:  5,
		Nodes: []collectors.TailscaleNode{
			{Name: "honey", Hostname: "honey", IP: "100.64.0.1", OS: "linux", Online: true},
			{Name: "yoga", Hostname: "yoga", IP: "100.64.0.2", OS: "linux", Online: true},
			{Name: "petting-zoo-mini", Hostname: "petting-zoo-mini", IP: "100.64.0.3", OS: "darwin", Online: true},
			{Name: "macbook", Hostname: "macbook", IP: "100.64.0.4", OS: "darwin", Online: false},
			{Name: "tablet", Hostname: "tablet", IP: "100.64.0.5", OS: "android", Online: false},
		},
	}
}

// sampleK8sCluster returns a realistic KubernetesCluster for testing.
func sampleK8sCluster(name, platform string) *collectors.KubernetesCluster {
	return &collectors.KubernetesCluster{
		Name:         name,
		Platform:     platform,
		Status:       "healthy",
		APIEndpoint:  "https://api." + name + ".example.com:6443",
		DashboardURL: "https://dashboard.example.com/" + name,
		Nodes: []collectors.KubernetesNode{
			{Name: "node-1", Status: "Ready", CPUPercent: 25.0, MemPercent: 40.0, PodCount: 12, MaxPods: 110},
			{Name: "node-2", Status: "Ready", CPUPercent: 30.0, MemPercent: 55.0, PodCount: 8, MaxPods: 110},
		},
		TotalNodes: 2,
		ReadyNodes: 2,
	}
}

// withMockFetchers overrides the package-level factory functions with test mocks,
// runs the provided function, then restores the originals. This ensures test
// isolation even when tests run in parallel within the same process.
func withMockFetchers(tsMock TailscaleFetcher, tsCliMock TailscaleFetcher, k8sMock KubernetesFetcher, fn func()) {
	origAPIFetcher := newTailscaleAPIFetcher
	origCLIFetcher := newTailscaleCLIFetcher
	origK8sFetcher := newKubernetesFetcher

	newTailscaleAPIFetcher = func(tailnet, apiKey string, logger *slog.Logger) TailscaleFetcher {
		if tsMock != nil {
			return tsMock
		}
		return &mockTailscaleFetcher{err: fmt.Errorf("no API mock configured")}
	}
	newTailscaleCLIFetcher = func(logger *slog.Logger) TailscaleFetcher {
		if tsCliMock != nil {
			return tsCliMock
		}
		return &mockTailscaleFetcher{err: fmt.Errorf("no CLI mock configured")}
	}
	newKubernetesFetcher = func(logger *slog.Logger) KubernetesFetcher {
		if k8sMock != nil {
			return k8sMock
		}
		return &mockKubernetesFetcher{
			clusters: map[string]*collectors.KubernetesCluster{},
			errors:   map[string]error{},
		}
	}

	defer func() {
		newTailscaleAPIFetcher = origAPIFetcher
		newTailscaleCLIFetcher = origCLIFetcher
		newKubernetesFetcher = origK8sFetcher
	}()

	fn()
}

// --- Tests ---

func TestInfraCollector_Name(t *testing.T) {
	c := NewInfraCollector(InfraCollectorConfig{}, nil)
	if got := c.Name(); got != "infra" {
		t.Errorf("Name() = %q, want %q", got, "infra")
	}
}

func TestInfraCollector_Description(t *testing.T) {
	c := NewInfraCollector(InfraCollectorConfig{}, nil)
	want := "Infrastructure status across Tailscale mesh and Kubernetes clusters"
	if got := c.Description(); got != want {
		t.Errorf("Description() = %q, want %q", got, want)
	}
}

func TestInfraCollector_Interval(t *testing.T) {
	c := NewInfraCollector(InfraCollectorConfig{}, nil)
	want := 5 * time.Minute
	if got := c.Interval(); got != want {
		t.Errorf("Interval() = %v, want %v", got, want)
	}
}

func TestInfraCollector_BothSucceed(t *testing.T) {
	tsStatus := sampleTailscaleStatus()
	civoCluster := sampleK8sCluster("civo-prod", "civo")

	tsMock := &mockTailscaleFetcher{status: tsStatus}
	k8sMock := &mockKubernetesFetcher{
		clusters: map[string]*collectors.KubernetesCluster{
			"civo-prod": civoCluster,
		},
		errors: map[string]error{},
	}

	config := InfraCollectorConfig{
		Tailnet:         "tinyland.ts.net",
		TailscaleAPIKey: "tskey-api-fake",
		KubeContexts: []KubeContextConfig{
			{Name: "civo-prod", Platform: "civo", DashboardURL: "https://dashboard.civo.com"},
		},
	}

	withMockFetchers(tsMock, nil, k8sMock, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		if result.Collector != "infra" {
			t.Errorf("Collector = %q, want %q", result.Collector, "infra")
		}

		data, ok := result.Data.(*collectors.InfraStatus)
		if !ok {
			t.Fatalf("Data is %T, want *collectors.InfraStatus", result.Data)
		}

		// Verify Tailscale.
		if data.Tailscale == nil {
			t.Fatal("Tailscale is nil, want non-nil")
		}
		if data.Tailscale.OnlineCount != 3 {
			t.Errorf("Tailscale.OnlineCount = %d, want 3", data.Tailscale.OnlineCount)
		}
		if data.Tailscale.TotalCount != 5 {
			t.Errorf("Tailscale.TotalCount = %d, want 5", data.Tailscale.TotalCount)
		}

		// Verify Kubernetes.
		if len(data.Kubernetes) != 1 {
			t.Fatalf("got %d clusters, want 1", len(data.Kubernetes))
		}
		if data.Kubernetes[0].Name != "civo-prod" {
			t.Errorf("Kubernetes[0].Name = %q, want %q", data.Kubernetes[0].Name, "civo-prod")
		}
		if data.Kubernetes[0].Status != "healthy" {
			t.Errorf("Kubernetes[0].Status = %q, want %q", data.Kubernetes[0].Status, "healthy")
		}

		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_TailscaleAPIFails_CLIFallbackSucceeds(t *testing.T) {
	tsStatus := sampleTailscaleStatus()

	apiMock := &mockTailscaleFetcher{err: fmt.Errorf("API unauthorized")}
	cliMock := &mockTailscaleFetcher{status: tsStatus}

	config := InfraCollectorConfig{
		Tailnet:         "tinyland.ts.net",
		TailscaleAPIKey: "tskey-api-bad",
		UseCLIFallback:  true,
	}

	withMockFetchers(apiMock, cliMock, nil, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		if data.Tailscale == nil {
			t.Fatal("Tailscale is nil, want non-nil (CLI fallback should succeed)")
		}
		if data.Tailscale.OnlineCount != 3 {
			t.Errorf("Tailscale.OnlineCount = %d, want 3", data.Tailscale.OnlineCount)
		}

		// Should have a warning from the API failure.
		if len(result.Warnings) != 1 {
			t.Errorf("got %d warnings, want 1: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_TailscaleAPIFails_CLIFallbackAlsoFails(t *testing.T) {
	apiMock := &mockTailscaleFetcher{err: fmt.Errorf("API unauthorized")}
	cliMock := &mockTailscaleFetcher{err: fmt.Errorf("CLI not found")}

	config := InfraCollectorConfig{
		Tailnet:         "tinyland.ts.net",
		TailscaleAPIKey: "tskey-api-bad",
		UseCLIFallback:  true,
	}

	withMockFetchers(apiMock, cliMock, nil, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		// Tailscale should be nil since both failed.
		if data.Tailscale != nil {
			t.Errorf("Tailscale = %v, want nil when both API and CLI fail", data.Tailscale)
		}

		// Should have two warnings: one from API, one from CLI.
		if len(result.Warnings) != 2 {
			t.Errorf("got %d warnings, want 2: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_TailscaleAPIFails_CLIFallbackDisabled(t *testing.T) {
	apiMock := &mockTailscaleFetcher{err: fmt.Errorf("API unauthorized")}

	config := InfraCollectorConfig{
		Tailnet:         "tinyland.ts.net",
		TailscaleAPIKey: "tskey-api-bad",
		UseCLIFallback:  false,
	}

	withMockFetchers(apiMock, nil, nil, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		// Tailscale should be nil since API failed and CLI fallback is disabled.
		if data.Tailscale != nil {
			t.Errorf("Tailscale = %v, want nil when API fails and CLI disabled", data.Tailscale)
		}

		// Should have one warning from the API failure.
		if len(result.Warnings) != 1 {
			t.Errorf("got %d warnings, want 1: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_NoTailscaleConfigured(t *testing.T) {
	// No API key and no CLI fallback: Tailscale is simply not configured.
	config := InfraCollectorConfig{
		TailscaleAPIKey: "",
		UseCLIFallback:  false,
	}

	withMockFetchers(nil, nil, nil, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		if data.Tailscale != nil {
			t.Errorf("Tailscale = %v, want nil when not configured", data.Tailscale)
		}

		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_TailscaleCLIOnly(t *testing.T) {
	// No API key, but CLI fallback is enabled.
	tsStatus := sampleTailscaleStatus()
	cliMock := &mockTailscaleFetcher{status: tsStatus}

	config := InfraCollectorConfig{
		TailscaleAPIKey: "",
		UseCLIFallback:  true,
	}

	withMockFetchers(nil, cliMock, nil, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		if data.Tailscale == nil {
			t.Fatal("Tailscale is nil, want non-nil from CLI fallback")
		}
		if data.Tailscale.TotalCount != 5 {
			t.Errorf("Tailscale.TotalCount = %d, want 5", data.Tailscale.TotalCount)
		}

		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_MultipleK8sClusters_PartialFailure(t *testing.T) {
	civoCluster := sampleK8sCluster("civo-prod", "civo")

	k8sMock := &mockKubernetesFetcher{
		clusters: map[string]*collectors.KubernetesCluster{
			"civo-prod": civoCluster,
		},
		errors: map[string]error{
			"rke2-local": fmt.Errorf("connection refused"),
		},
	}

	config := InfraCollectorConfig{
		KubeContexts: []KubeContextConfig{
			{Name: "civo-prod", Platform: "civo", DashboardURL: "https://dashboard.civo.com"},
			{Name: "rke2-local", Platform: "rke2", DashboardURL: "https://local.example.com"},
		},
	}

	withMockFetchers(nil, nil, k8sMock, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		if len(data.Kubernetes) != 2 {
			t.Fatalf("got %d clusters, want 2", len(data.Kubernetes))
		}

		// First cluster should succeed.
		if data.Kubernetes[0].Name != "civo-prod" {
			t.Errorf("Kubernetes[0].Name = %q, want %q", data.Kubernetes[0].Name, "civo-prod")
		}
		if data.Kubernetes[0].Status != "healthy" {
			t.Errorf("Kubernetes[0].Status = %q, want %q", data.Kubernetes[0].Status, "healthy")
		}

		// Second cluster should be offline.
		if data.Kubernetes[1].Name != "rke2-local" {
			t.Errorf("Kubernetes[1].Name = %q, want %q", data.Kubernetes[1].Name, "rke2-local")
		}
		if data.Kubernetes[1].Status != "offline" {
			t.Errorf("Kubernetes[1].Status = %q, want %q", data.Kubernetes[1].Status, "offline")
		}
		if data.Kubernetes[1].DashboardURL != "https://local.example.com" {
			t.Errorf("Kubernetes[1].DashboardURL = %q, want %q", data.Kubernetes[1].DashboardURL, "https://local.example.com")
		}

		// Should have one warning from the failed cluster.
		if len(result.Warnings) != 1 {
			t.Errorf("got %d warnings, want 1: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_AllK8sFail(t *testing.T) {
	k8sMock := &mockKubernetesFetcher{
		clusters: map[string]*collectors.KubernetesCluster{},
		errors: map[string]error{
			"civo-prod":  fmt.Errorf("API server timeout"),
			"rke2-local": fmt.Errorf("connection refused"),
		},
	}

	config := InfraCollectorConfig{
		KubeContexts: []KubeContextConfig{
			{Name: "civo-prod", Platform: "civo"},
			{Name: "rke2-local", Platform: "rke2"},
		},
	}

	withMockFetchers(nil, nil, k8sMock, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		if len(data.Kubernetes) != 2 {
			t.Fatalf("got %d clusters, want 2", len(data.Kubernetes))
		}

		for i, cluster := range data.Kubernetes {
			if cluster.Status != "offline" {
				t.Errorf("Kubernetes[%d].Status = %q, want %q", i, cluster.Status, "offline")
			}
		}

		if len(result.Warnings) != 2 {
			t.Errorf("got %d warnings, want 2: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_NoK8sContexts(t *testing.T) {
	config := InfraCollectorConfig{
		KubeContexts: nil,
	}

	withMockFetchers(nil, nil, nil, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		if data.Kubernetes != nil {
			t.Errorf("Kubernetes = %v, want nil for empty config", data.Kubernetes)
		}

		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	config := InfraCollectorConfig{
		TailscaleAPIKey: "tskey-api-fake",
		KubeContexts: []KubeContextConfig{
			{Name: "civo-prod", Platform: "civo"},
		},
	}

	c := NewInfraCollector(config, testLogger())
	_, err := c.Collect(ctx)
	if err == nil {
		t.Fatal("Collect() with cancelled context should return error")
	}
	if err != context.Canceled {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestInfraCollector_MixedResults(t *testing.T) {
	// Tailscale succeeds, one K8s succeeds, one K8s fails.
	tsStatus := sampleTailscaleStatus()
	civoCluster := sampleK8sCluster("civo-prod", "civo")

	tsMock := &mockTailscaleFetcher{status: tsStatus}
	k8sMock := &mockKubernetesFetcher{
		clusters: map[string]*collectors.KubernetesCluster{
			"civo-prod": civoCluster,
		},
		errors: map[string]error{
			"rke2-local": fmt.Errorf("kubectl not found"),
		},
	}

	config := InfraCollectorConfig{
		Tailnet:         "tinyland.ts.net",
		TailscaleAPIKey: "tskey-api-fake",
		KubeContexts: []KubeContextConfig{
			{Name: "civo-prod", Platform: "civo", DashboardURL: "https://dashboard.civo.com"},
			{Name: "rke2-local", Platform: "rke2"},
		},
	}

	withMockFetchers(tsMock, nil, k8sMock, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		// Tailscale should succeed.
		if data.Tailscale == nil {
			t.Fatal("Tailscale is nil, want non-nil")
		}
		if data.Tailscale.OnlineCount != 3 {
			t.Errorf("Tailscale.OnlineCount = %d, want 3", data.Tailscale.OnlineCount)
		}

		// Two K8s clusters: one healthy, one offline.
		if len(data.Kubernetes) != 2 {
			t.Fatalf("got %d clusters, want 2", len(data.Kubernetes))
		}
		if data.Kubernetes[0].Status != "healthy" {
			t.Errorf("Kubernetes[0].Status = %q, want healthy", data.Kubernetes[0].Status)
		}
		if data.Kubernetes[1].Status != "offline" {
			t.Errorf("Kubernetes[1].Status = %q, want offline", data.Kubernetes[1].Status)
		}

		// One warning from the failed K8s cluster.
		if len(result.Warnings) != 1 {
			t.Errorf("got %d warnings, want 1: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_K8sOrderPreserved(t *testing.T) {
	cluster1 := sampleK8sCluster("alpha", "civo")
	cluster2 := sampleK8sCluster("beta", "rke2")
	cluster3 := sampleK8sCluster("gamma", "doks")

	k8sMock := &mockKubernetesFetcher{
		clusters: map[string]*collectors.KubernetesCluster{
			"alpha": cluster1,
			"beta":  cluster2,
			"gamma": cluster3,
		},
		errors: map[string]error{},
	}

	config := InfraCollectorConfig{
		KubeContexts: []KubeContextConfig{
			{Name: "alpha", Platform: "civo"},
			{Name: "beta", Platform: "rke2"},
			{Name: "gamma", Platform: "doks"},
		},
	}

	withMockFetchers(nil, nil, k8sMock, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		if len(data.Kubernetes) != 3 {
			t.Fatalf("got %d clusters, want 3", len(data.Kubernetes))
		}

		// Verify order matches input config despite concurrent collection.
		expectedNames := []string{"alpha", "beta", "gamma"}
		for i, want := range expectedNames {
			if data.Kubernetes[i].Name != want {
				t.Errorf("Kubernetes[%d].Name = %q, want %q", i, data.Kubernetes[i].Name, want)
			}
		}
	})
}

func TestInfraCollector_NilLogger(t *testing.T) {
	c := NewInfraCollector(InfraCollectorConfig{}, nil)
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestInfraCollector_TimestampIsRecent(t *testing.T) {
	before := time.Now()

	c := NewInfraCollector(InfraCollectorConfig{}, testLogger())
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() returned unexpected error: %v", err)
	}

	after := time.Now()

	if result.Timestamp.Before(before) || result.Timestamp.After(after) {
		t.Errorf("Timestamp = %v, want between %v and %v", result.Timestamp, before, after)
	}
}

func TestInfraCollector_InterfaceCompliance(t *testing.T) {
	var _ collectors.Collector = (*InfraCollector)(nil)
}

func TestInfraCollector_EmptyConfig(t *testing.T) {
	// Completely empty config: no Tailscale, no K8s.
	config := InfraCollectorConfig{}

	withMockFetchers(nil, nil, nil, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		if data.Tailscale != nil {
			t.Errorf("Tailscale = %v, want nil for empty config", data.Tailscale)
		}
		if data.Kubernetes != nil {
			t.Errorf("Kubernetes = %v, want nil for empty config", data.Kubernetes)
		}
		if len(result.Warnings) != 0 {
			t.Errorf("got %d warnings, want 0: %v", len(result.Warnings), result.Warnings)
		}
	})
}

func TestInfraCollector_FailedK8sPreservesDashboardURL(t *testing.T) {
	k8sMock := &mockKubernetesFetcher{
		clusters: map[string]*collectors.KubernetesCluster{},
		errors: map[string]error{
			"civo-prod": fmt.Errorf("connection refused"),
		},
	}

	config := InfraCollectorConfig{
		KubeContexts: []KubeContextConfig{
			{
				Name:         "civo-prod",
				Platform:     "civo",
				DashboardURL: "https://dashboard.civo.com/k8s/abc123",
			},
		},
	}

	withMockFetchers(nil, nil, k8sMock, func() {
		c := NewInfraCollector(config, testLogger())
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect() returned unexpected error: %v", err)
		}

		data := result.Data.(*collectors.InfraStatus)

		if len(data.Kubernetes) != 1 {
			t.Fatalf("got %d clusters, want 1", len(data.Kubernetes))
		}

		// Even when offline, the dashboard URL and platform should be preserved.
		k := data.Kubernetes[0]
		if k.Status != "offline" {
			t.Errorf("Status = %q, want offline", k.Status)
		}
		if k.Platform != "civo" {
			t.Errorf("Platform = %q, want civo", k.Platform)
		}
		if k.DashboardURL != "https://dashboard.civo.com/k8s/abc123" {
			t.Errorf("DashboardURL = %q, want https://dashboard.civo.com/k8s/abc123", k.DashboardURL)
		}
	})
}
