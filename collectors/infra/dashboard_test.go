package infra

import (
	"fmt"
	"strings"
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// --- OSC 8 escape sequence helpers ---

// osc8Start returns the OSC 8 opening sequence for a URL.
func osc8Start(url string) string {
	return fmt.Sprintf("\033]8;;%s\033\\", url)
}

// osc8End returns the OSC 8 closing sequence.
func osc8End() string {
	return "\033]8;;\033\\"
}

// --- Tailscale link tests ---

func TestGenerateTailscaleLink_WithStatus(t *testing.T) {
	status := &collectors.TailscaleStatus{
		Tailnet:     "tinyland.ts.net",
		OnlineCount: 3,
		TotalCount:  5,
	}

	got := GenerateTailscaleLink(status)

	// Verify the link contains the correct URL.
	if !strings.Contains(got, tailscaleAdminURL) {
		t.Errorf("link does not contain Tailscale admin URL %q: %q", tailscaleAdminURL, got)
	}

	// Verify the link text shows the correct counts.
	if !strings.Contains(got, "ts:3/5") {
		t.Errorf("link text does not contain ts:3/5: %q", got)
	}

	// Verify proper OSC 8 structure.
	wantPrefix := osc8Start(tailscaleAdminURL)
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("link does not start with OSC 8 open sequence")
	}
	wantSuffix := osc8End()
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("link does not end with OSC 8 close sequence")
	}
}

func TestGenerateTailscaleLink_AllOnline(t *testing.T) {
	status := &collectors.TailscaleStatus{
		OnlineCount: 10,
		TotalCount:  10,
	}

	got := GenerateTailscaleLink(status)
	if !strings.Contains(got, "ts:10/10") {
		t.Errorf("link text does not contain ts:10/10: %q", got)
	}
}

func TestGenerateTailscaleLink_NoneOnline(t *testing.T) {
	status := &collectors.TailscaleStatus{
		OnlineCount: 0,
		TotalCount:  3,
	}

	got := GenerateTailscaleLink(status)
	if !strings.Contains(got, "ts:0/3") {
		t.Errorf("link text does not contain ts:0/3: %q", got)
	}
}

func TestGenerateTailscaleLink_Nil(t *testing.T) {
	got := GenerateTailscaleLink(nil)
	if got != "" {
		t.Errorf("GenerateTailscaleLink(nil) = %q, want empty string", got)
	}
}

func TestGenerateTailscaleLink_ZeroCounts(t *testing.T) {
	status := &collectors.TailscaleStatus{
		OnlineCount: 0,
		TotalCount:  0,
	}

	got := GenerateTailscaleLink(status)
	if !strings.Contains(got, "ts:0/0") {
		t.Errorf("link text does not contain ts:0/0: %q", got)
	}
}

// --- Kubernetes link tests ---

func TestGenerateK8sLinks_WithClusters(t *testing.T) {
	clusters := []collectors.KubernetesCluster{
		{
			Name:         "civo-prod",
			Status:       "healthy",
			DashboardURL: "https://dashboard.civo.com/k8s/abc123",
		},
		{
			Name:         "rke2-local",
			Status:       "degraded",
			DashboardURL: "https://rancher.local/dashboard",
		},
	}

	got := GenerateK8sLinks(clusters)

	if len(got) != 2 {
		t.Fatalf("got %d links, want 2", len(got))
	}

	// First link: civo-prod.
	if !strings.Contains(got[0], "civo-prod:healthy") {
		t.Errorf("link[0] does not contain cluster name and status: %q", got[0])
	}
	if !strings.Contains(got[0], "https://dashboard.civo.com/k8s/abc123") {
		t.Errorf("link[0] does not contain dashboard URL: %q", got[0])
	}

	// Second link: rke2-local.
	if !strings.Contains(got[1], "rke2-local:degraded") {
		t.Errorf("link[1] does not contain cluster name and status: %q", got[1])
	}
}

func TestGenerateK8sLinks_NoDashboardURL(t *testing.T) {
	clusters := []collectors.KubernetesCluster{
		{Name: "civo-prod", Status: "healthy", DashboardURL: "https://dashboard.civo.com"},
		{Name: "local-kind", Status: "healthy", DashboardURL: ""}, // No dashboard URL.
	}

	got := GenerateK8sLinks(clusters)

	// Only the cluster with a DashboardURL should generate a link.
	if len(got) != 1 {
		t.Fatalf("got %d links, want 1 (cluster without DashboardURL should be skipped)", len(got))
	}

	if !strings.Contains(got[0], "civo-prod:healthy") {
		t.Errorf("link[0] does not contain civo-prod:healthy: %q", got[0])
	}
}

func TestGenerateK8sLinks_OfflineCluster(t *testing.T) {
	clusters := []collectors.KubernetesCluster{
		{Name: "broken", Status: "offline", DashboardURL: "https://dashboard.example.com"},
	}

	got := GenerateK8sLinks(clusters)

	if len(got) != 1 {
		t.Fatalf("got %d links, want 1", len(got))
	}
	if !strings.Contains(got[0], "broken:offline") {
		t.Errorf("link does not contain broken:offline: %q", got[0])
	}
}

func TestGenerateK8sLinks_Empty(t *testing.T) {
	got := GenerateK8sLinks(nil)
	if got != nil {
		t.Errorf("GenerateK8sLinks(nil) = %v, want nil", got)
	}

	got = GenerateK8sLinks([]collectors.KubernetesCluster{})
	if got != nil {
		t.Errorf("GenerateK8sLinks([]) = %v, want nil", got)
	}
}

func TestGenerateK8sLinks_AllWithoutDashboardURL(t *testing.T) {
	clusters := []collectors.KubernetesCluster{
		{Name: "local-1", Status: "healthy"},
		{Name: "local-2", Status: "healthy"},
	}

	got := GenerateK8sLinks(clusters)
	if got != nil {
		t.Errorf("got %v, want nil when no clusters have dashboard URLs", got)
	}
}

func TestGenerateK8sLinks_OSC8Format(t *testing.T) {
	clusters := []collectors.KubernetesCluster{
		{
			Name:         "test",
			Status:       "healthy",
			DashboardURL: "https://example.com/cluster",
		},
	}

	got := GenerateK8sLinks(clusters)
	if len(got) != 1 {
		t.Fatalf("got %d links, want 1", len(got))
	}

	// Verify full OSC 8 structure.
	want := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\",
		"https://example.com/cluster", "test:healthy")
	if got[0] != want {
		t.Errorf("got %q, want %q", got[0], want)
	}
}

// --- Billing link tests ---

func TestGenerateBillingLinks_WithProviders(t *testing.T) {
	billing := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider:     "civo",
				DashboardURL: "https://dashboard.civo.com/billing",
				CurrentMonth: collectors.MonthCost{SpendUSD: 12.50},
			},
			{
				Provider:     "digitalocean",
				DashboardURL: "https://cloud.digitalocean.com/account/billing",
				CurrentMonth: collectors.MonthCost{SpendUSD: 7.80},
			},
		},
	}

	got := GenerateBillingLinks(billing)

	if len(got) != 2 {
		t.Fatalf("got %d links, want 2", len(got))
	}

	if !strings.Contains(got[0], "civo:$12.50") {
		t.Errorf("link[0] does not contain civo:$12.50: %q", got[0])
	}
	if !strings.Contains(got[0], "https://dashboard.civo.com/billing") {
		t.Errorf("link[0] does not contain dashboard URL: %q", got[0])
	}

	if !strings.Contains(got[1], "digitalocean:$7.80") {
		t.Errorf("link[1] does not contain digitalocean:$7.80: %q", got[1])
	}
}

func TestGenerateBillingLinks_FallbackURL(t *testing.T) {
	// Provider without DashboardURL set should use the known fallback.
	billing := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider:     "aws",
				DashboardURL: "", // No explicit URL.
				CurrentMonth: collectors.MonthCost{SpendUSD: 45.00},
			},
		},
	}

	got := GenerateBillingLinks(billing)

	if len(got) != 1 {
		t.Fatalf("got %d links, want 1", len(got))
	}

	if !strings.Contains(got[0], awsDashboardURL) {
		t.Errorf("link does not contain fallback AWS dashboard URL: %q", got[0])
	}
	if !strings.Contains(got[0], "aws:$45.00") {
		t.Errorf("link does not contain aws:$45.00: %q", got[0])
	}
}

func TestGenerateBillingLinks_UnknownProviderNoDashboard(t *testing.T) {
	// An unknown provider with no DashboardURL should be skipped.
	billing := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider:     "gcp", // Not in providerDashboardURLs.
				DashboardURL: "",
				CurrentMonth: collectors.MonthCost{SpendUSD: 100.00},
			},
			{
				Provider:     "civo",
				DashboardURL: "https://dashboard.civo.com/billing",
				CurrentMonth: collectors.MonthCost{SpendUSD: 12.50},
			},
		},
	}

	got := GenerateBillingLinks(billing)

	if len(got) != 1 {
		t.Fatalf("got %d links, want 1 (unknown provider without URL should be skipped)", len(got))
	}
	if !strings.Contains(got[0], "civo") {
		t.Errorf("remaining link should be civo: %q", got[0])
	}
}

func TestGenerateBillingLinks_ZeroSpend(t *testing.T) {
	billing := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider:     "civo",
				DashboardURL: "https://dashboard.civo.com/billing",
				CurrentMonth: collectors.MonthCost{SpendUSD: 0.00},
			},
		},
	}

	got := GenerateBillingLinks(billing)
	if len(got) != 1 {
		t.Fatalf("got %d links, want 1", len(got))
	}
	if !strings.Contains(got[0], "civo:$0.00") {
		t.Errorf("link does not contain civo:$0.00: %q", got[0])
	}
}

func TestGenerateBillingLinks_Nil(t *testing.T) {
	got := GenerateBillingLinks(nil)
	if got != nil {
		t.Errorf("GenerateBillingLinks(nil) = %v, want nil", got)
	}
}

func TestGenerateBillingLinks_EmptyProviders(t *testing.T) {
	billing := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{},
	}

	got := GenerateBillingLinks(billing)
	if got != nil {
		t.Errorf("GenerateBillingLinks(empty) = %v, want nil", got)
	}
}

func TestGenerateBillingLinks_OSC8Format(t *testing.T) {
	billing := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{
				Provider:     "dreamhost",
				DashboardURL: dreamhostDashboardURL,
				CurrentMonth: collectors.MonthCost{SpendUSD: 9.95},
			},
		},
	}

	got := GenerateBillingLinks(billing)
	if len(got) != 1 {
		t.Fatalf("got %d links, want 1", len(got))
	}

	want := fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\",
		dreamhostDashboardURL, "dreamhost:$9.95")
	if got[0] != want {
		t.Errorf("got %q, want %q", got[0], want)
	}
}

// --- GenerateAllLinks tests ---

func TestGenerateAllLinks_AllPopulated(t *testing.T) {
	tsStatus := &collectors.TailscaleStatus{
		OnlineCount: 3,
		TotalCount:  5,
	}
	clusters := []collectors.KubernetesCluster{
		{Name: "prod", Status: "healthy", DashboardURL: "https://example.com/k8s"},
	}
	billing := &collectors.BillingData{
		Providers: []collectors.ProviderBilling{
			{Provider: "civo", DashboardURL: "https://dashboard.civo.com", CurrentMonth: collectors.MonthCost{SpendUSD: 10}},
		},
	}

	links := GenerateAllLinks(tsStatus, clusters, billing)

	if links.Tailscale == "" {
		t.Error("Tailscale link is empty, want non-empty")
	}
	if len(links.Kubernetes) != 1 {
		t.Errorf("got %d K8s links, want 1", len(links.Kubernetes))
	}
	if len(links.Billing) != 1 {
		t.Errorf("got %d billing links, want 1", len(links.Billing))
	}
}

func TestGenerateAllLinks_AllNil(t *testing.T) {
	links := GenerateAllLinks(nil, nil, nil)

	if links.Tailscale != "" {
		t.Errorf("Tailscale = %q, want empty", links.Tailscale)
	}
	if links.Kubernetes != nil {
		t.Errorf("Kubernetes = %v, want nil", links.Kubernetes)
	}
	if links.Billing != nil {
		t.Errorf("Billing = %v, want nil", links.Billing)
	}
}

func TestGenerateAllLinks_PartialData(t *testing.T) {
	// Only Tailscale populated.
	tsStatus := &collectors.TailscaleStatus{
		OnlineCount: 1,
		TotalCount:  1,
	}

	links := GenerateAllLinks(tsStatus, nil, nil)

	if links.Tailscale == "" {
		t.Error("Tailscale link is empty, want non-empty")
	}
	if links.Kubernetes != nil {
		t.Errorf("Kubernetes = %v, want nil", links.Kubernetes)
	}
	if links.Billing != nil {
		t.Errorf("Billing = %v, want nil", links.Billing)
	}
}

func TestGenerateBillingLinks_AllKnownProviders(t *testing.T) {
	// Verify all known providers can get fallback URLs.
	knownProviders := []string{"civo", "digitalocean", "aws", "dreamhost"}

	for _, provider := range knownProviders {
		billing := &collectors.BillingData{
			Providers: []collectors.ProviderBilling{
				{
					Provider:     provider,
					DashboardURL: "", // No explicit URL, use fallback.
					CurrentMonth: collectors.MonthCost{SpendUSD: 1.00},
				},
			},
		}

		got := GenerateBillingLinks(billing)
		if len(got) != 1 {
			t.Errorf("provider %q: got %d links, want 1", provider, len(got))
			continue
		}
		if got[0] == "" {
			t.Errorf("provider %q: link is empty", provider)
		}
	}
}
