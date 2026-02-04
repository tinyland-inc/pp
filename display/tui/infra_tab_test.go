package tui

import (
	"strings"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

func TestRenderInfraContent_Nil(t *testing.T) {
	got := renderInfraContent(nil, 100, 40)
	if got != "No infrastructure data available" {
		t.Errorf("renderInfraContent(nil) = %q, want placeholder", got)
	}
}

func TestRenderInfraContent_TailscaleOnly(t *testing.T) {
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "tinyland.ts.net",
			OnlineCount: 3,
			TotalCount:  5,
			Nodes: []collectors.TailscaleNode{
				{
					Name:     "honey",
					Hostname: "honey",
					IP:       "100.64.0.1",
					OS:       "linux",
					Online:   true,
					LastSeen: time.Now(),
				},
			},
		},
	}

	got := renderInfraContent(data, 100, 40)

	if !strings.Contains(got, "Infrastructure Status") {
		t.Error("missing section title")
	}
	if !strings.Contains(got, "Tailscale Mesh") {
		t.Error("missing tailscale header")
	}
	if !strings.Contains(got, "tinyland.ts.net") {
		t.Error("missing tailnet name")
	}
	if !strings.Contains(got, "honey") {
		t.Error("missing node name")
	}
	// Should not contain Kubernetes content.
	if strings.Contains(got, "K8s:") {
		t.Error("unexpected kubernetes section for tailscale-only data")
	}
}

func TestRenderInfraContent_K8sOnly(t *testing.T) {
	data := &collectors.InfraStatus{
		Kubernetes: []collectors.KubernetesCluster{
			{
				Name:       "prod",
				Platform:   "civo",
				Status:     "healthy",
				TotalNodes: 3,
				ReadyNodes: 3,
				Nodes: []collectors.KubernetesNode{
					{
						Name:       "node-1",
						Status:     "Ready",
						CPUPercent: 45.0,
						MemPercent: 60.0,
						PodCount:   22,
						MaxPods:    110,
					},
				},
			},
		},
	}

	got := renderInfraContent(data, 100, 40)

	if !strings.Contains(got, "Infrastructure Status") {
		t.Error("missing section title")
	}
	if !strings.Contains(got, "K8s: ") {
		t.Error("missing kubernetes header")
	}
	if !strings.Contains(got, "prod") {
		t.Error("missing cluster name")
	}
	if !strings.Contains(got, "civo") {
		t.Error("missing platform name")
	}
	if !strings.Contains(got, "node-1") {
		t.Error("missing node name")
	}
	// Should not contain Tailscale content.
	if strings.Contains(got, "Tailscale Mesh") {
		t.Error("unexpected tailscale section for k8s-only data")
	}
}

func TestRenderInfraContent_Both(t *testing.T) {
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "tinyland.ts.net",
			OnlineCount: 2,
			TotalCount:  4,
			Nodes: []collectors.TailscaleNode{
				{
					Name:     "honey",
					Hostname: "honey",
					IP:       "100.64.0.1",
					OS:       "linux",
					Online:   true,
					LastSeen: time.Now(),
				},
			},
		},
		Kubernetes: []collectors.KubernetesCluster{
			{
				Name:       "prod",
				Platform:   "civo",
				Status:     "healthy",
				TotalNodes: 2,
				ReadyNodes: 2,
				Nodes: []collectors.KubernetesNode{
					{
						Name:       "worker-1",
						Status:     "Ready",
						CPUPercent: 30.0,
						MemPercent: 50.0,
						PodCount:   15,
						MaxPods:    110,
					},
				},
			},
		},
	}

	got := renderInfraContent(data, 100, 40)

	if !strings.Contains(got, "Tailscale Mesh") {
		t.Error("missing tailscale section")
	}
	if !strings.Contains(got, "K8s: ") {
		t.Error("missing kubernetes section")
	}
	// Should contain a separator (horizontal rule) between sections.
	if !strings.Contains(got, "\u2500") {
		t.Error("missing separator between sections")
	}
}

func TestRenderInfraContent_MultipleK8sClusters(t *testing.T) {
	data := &collectors.InfraStatus{
		Kubernetes: []collectors.KubernetesCluster{
			{
				Name:       "prod",
				Platform:   "civo",
				Status:     "healthy",
				TotalNodes: 3,
				ReadyNodes: 3,
				Nodes: []collectors.KubernetesNode{
					{Name: "prod-1", Status: "Ready", CPUPercent: 20, MemPercent: 30, PodCount: 10, MaxPods: 110},
				},
			},
			{
				Name:       "staging",
				Platform:   "rke2",
				Status:     "healthy",
				TotalNodes: 1,
				ReadyNodes: 1,
				Nodes: []collectors.KubernetesNode{
					{Name: "staging-1", Status: "Ready", CPUPercent: 10, MemPercent: 20, PodCount: 5, MaxPods: 110},
				},
			},
		},
	}

	got := renderInfraContent(data, 100, 40)

	if !strings.Contains(got, "prod") {
		t.Error("missing first cluster name")
	}
	if !strings.Contains(got, "staging") {
		t.Error("missing second cluster name")
	}
	if !strings.Contains(got, "civo") {
		t.Error("missing first cluster platform")
	}
	if !strings.Contains(got, "rke2") {
		t.Error("missing second cluster platform")
	}
}

func TestRenderInfraContent_OfflineNodes(t *testing.T) {
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "test.ts.net",
			OnlineCount: 1,
			TotalCount:  2,
			Nodes: []collectors.TailscaleNode{
				{
					Name:     "online-node",
					Hostname: "online-node",
					IP:       "100.64.0.1",
					OS:       "linux",
					Online:   true,
					LastSeen: time.Now(),
				},
				{
					Name:     "offline-node",
					Hostname: "offline-node",
					IP:       "100.64.0.2",
					OS:       "linux",
					Online:   false,
					LastSeen: time.Now().Add(-2 * time.Hour),
				},
			},
		},
	}

	got := renderInfraContent(data, 100, 40)

	if !strings.Contains(got, "online-node") {
		t.Error("missing online node")
	}
	if !strings.Contains(got, "offline-node") {
		t.Error("missing offline node")
	}
	// Both online and offline nodes should be present.
	// The widgets package uses filled circle (U+25CF) for OK and Critical levels,
	// differentiated by color. In non-TTY test output, both render as plain dots.
	if !strings.Contains(got, "\u25CF") {
		t.Error("missing status indicator dot")
	}
	// Verify both nodes are in the output.
	if !strings.Contains(got, "100.64.0.1") {
		t.Error("missing online node IP")
	}
	if !strings.Contains(got, "100.64.0.2") {
		t.Error("missing offline node IP")
	}
}

func TestRenderInfraContent_DegradedCluster(t *testing.T) {
	data := &collectors.InfraStatus{
		Kubernetes: []collectors.KubernetesCluster{
			{
				Name:       "degraded-cluster",
				Platform:   "doks",
				Status:     "degraded",
				TotalNodes: 3,
				ReadyNodes: 1,
				Nodes: []collectors.KubernetesNode{
					{Name: "node-1", Status: "Ready", CPUPercent: 80, MemPercent: 90, PodCount: 50, MaxPods: 110},
					{Name: "node-2", Status: "NotReady", CPUPercent: 0, MemPercent: 0, PodCount: 0, MaxPods: 110},
					{Name: "node-3", Status: "NotReady", CPUPercent: 0, MemPercent: 0, PodCount: 0, MaxPods: 110},
				},
			},
		},
	}

	got := renderInfraContent(data, 100, 40)

	if !strings.Contains(got, "degraded") {
		t.Error("missing degraded status text")
	}
	if !strings.Contains(got, "1/3") {
		t.Error("missing ready/total nodes count")
	}
}

func TestRenderInfraContent_EmptyTailscale(t *testing.T) {
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "empty.ts.net",
			OnlineCount: 0,
			TotalCount:  0,
			Nodes:       nil,
		},
	}

	got := renderInfraContent(data, 100, 40)

	if !strings.Contains(got, "Tailscale Mesh") {
		t.Error("missing tailscale header for empty tailnet")
	}
	if !strings.Contains(got, "empty.ts.net") {
		t.Error("missing tailnet name")
	}
	if !strings.Contains(got, "(0/0 online)") {
		t.Error("missing zero count indicator")
	}
}

func TestRenderInfraContent_EmptyK8s(t *testing.T) {
	data := &collectors.InfraStatus{
		Kubernetes: []collectors.KubernetesCluster{},
	}

	got := renderInfraContent(data, 100, 40)

	if !strings.Contains(got, "Infrastructure Status") {
		t.Error("missing section title")
	}
	// With no clusters and no tailscale, just the title should appear.
	if strings.Contains(got, "K8s:") {
		t.Error("unexpected kubernetes section for empty slice")
	}
}

func TestRenderInfraContent_WithDashboardURLs(t *testing.T) {
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "linked.ts.net",
			OnlineCount: 1,
			TotalCount:  1,
			Nodes: []collectors.TailscaleNode{
				{
					Name:         "linked-node",
					Hostname:     "linked-node",
					IP:           "100.64.0.1",
					OS:           "linux",
					Online:       true,
					LastSeen:     time.Now(),
					DashboardURL: "https://login.tailscale.com/admin/machines/linked-node",
				},
			},
		},
		Kubernetes: []collectors.KubernetesCluster{
			{
				Name:         "linked-cluster",
				Platform:     "civo",
				Status:       "healthy",
				DashboardURL: "https://dashboard.civo.com/kubernetes/linked-cluster",
				TotalNodes:   1,
				ReadyNodes:   1,
				Nodes: []collectors.KubernetesNode{
					{Name: "node-1", Status: "Ready", CPUPercent: 10, MemPercent: 20, PodCount: 5, MaxPods: 110},
				},
			},
		},
	}

	got := renderInfraContent(data, 100, 40)

	// Check for OSC 8 escape sequences.
	if !strings.Contains(got, "\033]8;;") {
		t.Error("missing OSC 8 hyperlink escape sequences")
	}
	if !strings.Contains(got, "https://login.tailscale.com") {
		t.Error("missing tailscale dashboard URL in OSC 8 link")
	}
	if !strings.Contains(got, "https://dashboard.civo.com") {
		t.Error("missing kubernetes dashboard URL in OSC 8 link")
	}
}
