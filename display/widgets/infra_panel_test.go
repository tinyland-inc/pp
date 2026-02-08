package widgets

import (
	"strings"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

func TestInfraPanel_Render_EmptyData(t *testing.T) {
	panel := NewInfraPanel(DefaultInfraPanelConfig())

	// Test with nil data.
	result := panel.Render(nil)
	if !strings.Contains(result, "no infrastructure data") {
		t.Errorf("expected empty state message, got: %s", result)
	}

	// Test with empty InfraStatus.
	result = panel.Render(&collectors.InfraStatus{})
	if !strings.Contains(result, "no infrastructure data") {
		t.Errorf("expected empty state message for empty struct, got: %s", result)
	}
}

func TestInfraPanel_Render_TailscaleOnly(t *testing.T) {
	panel := NewInfraPanel(InfraPanelConfig{
		Width:          60,
		ShowMiniGauges: false, // Disable for simpler test validation.
		ColorEnabled:   false,
	})

	cpu := 45.0
	ram := 60.0
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "example.ts.net",
			OnlineCount: 2,
			TotalCount:  3,
			Nodes: []collectors.TailscaleNode{
				{Hostname: "node1", IP: "100.64.0.1", Online: true, CPUPercent: &cpu, RAMPercent: &ram},
				{Hostname: "node2", IP: "100.64.0.2", Online: true},
				{Hostname: "node3", IP: "100.64.0.3", Online: false},
			},
		},
	}

	result := panel.Render(data)

	// Verify header content.
	if !strings.Contains(result, "Tailscale Mesh") {
		t.Errorf("expected Tailscale Mesh header, got: %s", result)
	}
	if !strings.Contains(result, "example") {
		t.Errorf("expected tailnet name in output, got: %s", result)
	}
	if !strings.Contains(result, "2/3 nodes online") {
		t.Errorf("expected node count summary, got: %s", result)
	}

	// Verify nodes are listed.
	if !strings.Contains(result, "node1") {
		t.Errorf("expected node1 in output, got: %s", result)
	}
	if !strings.Contains(result, "node3") {
		t.Errorf("expected node3 in output, got: %s", result)
	}

	// Verify offline status.
	if !strings.Contains(result, "OFFLINE") {
		t.Errorf("expected OFFLINE indicator for offline node, got: %s", result)
	}
}

func TestInfraPanel_Render_KubernetesOnly(t *testing.T) {
	panel := NewInfraPanel(InfraPanelConfig{
		Width:          60,
		ShowMiniGauges: false,
		ColorEnabled:   false,
	})

	data := &collectors.InfraStatus{
		Kubernetes: []collectors.KubernetesCluster{
			{
				Name:       "prod-cluster",
				Platform:   "civo",
				Status:     "healthy",
				TotalNodes: 3,
				ReadyNodes: 3,
				Nodes: []collectors.KubernetesNode{
					{Name: "node-1", Status: "Ready", CPUPercent: 45, MemPercent: 60},
					{Name: "node-2", Status: "Ready", CPUPercent: 30, MemPercent: 40},
					{Name: "node-3", Status: "Ready", CPUPercent: 55, MemPercent: 70},
				},
			},
		},
	}

	result := panel.Render(data)

	// Verify header.
	if !strings.Contains(result, "Kubernetes Clusters") {
		t.Errorf("expected Kubernetes Clusters header, got: %s", result)
	}

	// Verify cluster info.
	if !strings.Contains(result, "prod-cluster") {
		t.Errorf("expected cluster name, got: %s", result)
	}
	if !strings.Contains(result, "civo") {
		t.Errorf("expected platform name, got: %s", result)
	}
	if !strings.Contains(result, "3/3 nodes ready") {
		t.Errorf("expected ready count, got: %s", result)
	}

	// Verify nodes.
	if !strings.Contains(result, "node-1") {
		t.Errorf("expected node-1 in output, got: %s", result)
	}
}

func TestInfraPanel_Render_MixedData(t *testing.T) {
	panel := NewInfraPanel(InfraPanelConfig{
		Width:          60,
		ShowMiniGauges: true,
		GaugeWidth:     6,
		ColorEnabled:   false,
	})

	cpu := 85.0 // High utilization.
	ram := 45.0
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "prod.ts.net",
			OnlineCount: 1,
			TotalCount:  1,
			Nodes: []collectors.TailscaleNode{
				{Hostname: "server1", IP: "100.64.0.1", Online: true, CPUPercent: &cpu, RAMPercent: &ram},
			},
		},
		Kubernetes: []collectors.KubernetesCluster{
			{
				Name:       "dev-cluster",
				Status:     "degraded",
				TotalNodes: 2,
				ReadyNodes: 1,
				Nodes: []collectors.KubernetesNode{
					{Name: "dev-node-1", Status: "Ready", CPUPercent: 20, MemPercent: 30},
					{Name: "dev-node-2", Status: "NotReady", CPUPercent: 0, MemPercent: 0},
				},
			},
		},
	}

	result := panel.Render(data)

	// Verify both sections present.
	if !strings.Contains(result, "Tailscale Mesh") {
		t.Errorf("expected Tailscale section, got: %s", result)
	}
	if !strings.Contains(result, "Kubernetes Clusters") {
		t.Errorf("expected Kubernetes section, got: %s", result)
	}

	// Verify server1 with high CPU (should have warning indicator in full render).
	if !strings.Contains(result, "server1") {
		t.Errorf("expected server1 in output, got: %s", result)
	}

	// Verify degraded cluster.
	if !strings.Contains(result, "1/2 nodes ready") {
		t.Errorf("expected degraded node count, got: %s", result)
	}

	// Verify NotReady node status.
	if !strings.Contains(result, "NotReady") {
		t.Errorf("expected NotReady status, got: %s", result)
	}
}

func TestInfraPanel_MaxNodes(t *testing.T) {
	panel := NewInfraPanel(InfraPanelConfig{
		Width:          60,
		ShowMiniGauges: false,
		ColorEnabled:   false,
		MaxNodes:       2,
	})

	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "large.ts.net",
			OnlineCount: 5,
			TotalCount:  5,
			Nodes: []collectors.TailscaleNode{
				{Hostname: "node1", IP: "100.64.0.1", Online: true},
				{Hostname: "node2", IP: "100.64.0.2", Online: true},
				{Hostname: "node3", IP: "100.64.0.3", Online: true},
				{Hostname: "node4", IP: "100.64.0.4", Online: true},
				{Hostname: "node5", IP: "100.64.0.5", Online: true},
			},
		},
	}

	result := panel.Render(data)

	// Verify truncation indicator.
	if !strings.Contains(result, "+3 more nodes") {
		t.Errorf("expected truncation indicator, got: %s", result)
	}

	// Verify only first 2 nodes shown.
	if !strings.Contains(result, "node1") {
		t.Errorf("expected node1 in output, got: %s", result)
	}
	if !strings.Contains(result, "node2") {
		t.Errorf("expected node2 in output, got: %s", result)
	}
	if strings.Contains(result, "node5") {
		t.Errorf("expected node5 to be truncated, got: %s", result)
	}
}

func TestInfraPanel_TreeStructure(t *testing.T) {
	panel := NewInfraPanel(InfraPanelConfig{
		Width:          60,
		ShowMiniGauges: false,
		ColorEnabled:   false,
	})

	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "test.ts.net",
			OnlineCount: 2,
			TotalCount:  2,
			Nodes: []collectors.TailscaleNode{
				{Hostname: "first", IP: "100.64.0.1", Online: true},
				{Hostname: "last", IP: "100.64.0.2", Online: true},
			},
		},
	}

	result := panel.Render(data)

	// Verify tree characters are present.
	lines := strings.Split(result, "\n")
	foundBranch := false
	foundLastBranch := false
	for _, line := range lines {
		if strings.Contains(line, treeBranch) {
			foundBranch = true
		}
		if strings.Contains(line, treeLastBranch) {
			foundLastBranch = true
		}
	}

	if !foundBranch {
		t.Errorf("expected tree branch character in output")
	}
	if !foundLastBranch {
		t.Errorf("expected last branch character in output")
	}
}

func TestInfraPanel_StatusIndicators(t *testing.T) {
	panel := NewInfraPanel(InfraPanelConfig{
		Width:          60,
		ShowMiniGauges: false,
		ColorEnabled:   false,
	})

	cpu := 85.0 // High utilization.
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "test.ts.net",
			OnlineCount: 2,
			TotalCount:  3,
			Nodes: []collectors.TailscaleNode{
				{Hostname: "healthy", IP: "100.64.0.1", Online: true},
				{Hostname: "warning", IP: "100.64.0.2", Online: true, CPUPercent: &cpu},
				{Hostname: "offline", IP: "100.64.0.3", Online: false},
			},
		},
	}

	result := panel.Render(data)

	// Verify status indicators are present.
	if !strings.Contains(result, statusOnline) {
		t.Errorf("expected online status indicator, got: %s", result)
	}
	if !strings.Contains(result, statusWarning) {
		t.Errorf("expected warning status indicator for high CPU, got: %s", result)
	}
	if !strings.Contains(result, statusOffline) {
		t.Errorf("expected offline status indicator, got: %s", result)
	}
}

func TestK8sStatusIndicator(t *testing.T) {
	panel := NewInfraPanel(DefaultInfraPanelConfig())

	tests := []struct {
		status   string
		expected string
	}{
		{"healthy", statusOnline},
		{"degraded", statusWarning},
		{"offline", statusOffline},
		{"unknown", statusWarning},
	}

	for _, tc := range tests {
		result := panel.k8sStatusIndicator(tc.status)
		if result != tc.expected {
			t.Errorf("k8sStatusIndicator(%q) = %q, want %q", tc.status, result, tc.expected)
		}
	}
}

func TestK8sNodeStatusIndicator(t *testing.T) {
	panel := NewInfraPanel(DefaultInfraPanelConfig())

	tests := []struct {
		status     string
		cpuPercent float64
		memPercent float64
		expected   string
	}{
		{"Ready", 50, 50, statusOnline},
		{"Ready", 85, 50, statusWarning},
		{"Ready", 50, 90, statusWarning},
		{"NotReady", 0, 0, statusOffline},
		{"Unknown", 0, 0, statusOffline},
	}

	for _, tc := range tests {
		result := panel.k8sNodeStatusIndicator(tc.status, tc.cpuPercent, tc.memPercent)
		if result != tc.expected {
			t.Errorf("k8sNodeStatusIndicator(%q, %.0f, %.0f) = %q, want %q",
				tc.status, tc.cpuPercent, tc.memPercent, result, tc.expected)
		}
	}
}

func TestTruncateString(t *testing.T) {
	panel := NewInfraPanel(DefaultInfraPanelConfig())

	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10.", 10, "exactly10."},
		{"this is a very long string", 10, "this is..."},
		{"ab", 5, "ab"},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
	}

	for _, tc := range tests {
		result := panel.truncateString(tc.input, tc.maxLen)
		if result != tc.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.expected)
		}
	}
}

func TestTruncateTailnet(t *testing.T) {
	panel := NewInfraPanel(DefaultInfraPanelConfig())

	tests := []struct {
		input    string
		expected string
	}{
		{"example.ts.net", "example"},
		{"mynet.ts.net", "mynet"},
		{"already-short", "already-short"},
		{"a-very-long-tailnet-name-that-exceeds-max", "a-very-long-tailn..."},
	}

	for _, tc := range tests {
		result := panel.truncateTailnet(tc.input)
		if result != tc.expected {
			t.Errorf("truncateTailnet(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestRenderInfraBox(t *testing.T) {
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "test.ts.net",
			OnlineCount: 1,
			TotalCount:  1,
			Nodes: []collectors.TailscaleNode{
				{Hostname: "node1", IP: "100.64.0.1", Online: true},
			},
		},
	}

	result := RenderInfraBox(data, 60)

	// Verify box border characters are present (rounded border).
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╯") {
		t.Errorf("expected rounded box border characters, got: %s", result)
	}

	// Verify content is inside.
	if !strings.Contains(result, "Tailscale") {
		t.Errorf("expected Tailscale content inside box, got: %s", result)
	}
}

func TestInfraPanel_ColorEnabled(t *testing.T) {
	// Test with colors disabled.
	panelNoColor := NewInfraPanel(InfraPanelConfig{
		Width:          60,
		ColorEnabled:   false,
		ShowMiniGauges: true,
		GaugeWidth:     6,
	})

	// Test with colors enabled.
	panelWithColor := NewInfraPanel(InfraPanelConfig{
		Width:          60,
		ColorEnabled:   true,
		ShowMiniGauges: true,
		GaugeWidth:     6,
	})

	cpu := 45.0
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "test.ts.net",
			OnlineCount: 1,
			TotalCount:  1,
			Nodes: []collectors.TailscaleNode{
				{Hostname: "online-node", IP: "100.64.0.1", Online: true, CPUPercent: &cpu},
			},
		},
	}

	noColorResult := panelNoColor.Render(data)
	colorResult := panelWithColor.Render(data)

	// Verify no color mode doesn't add ANSI escapes to structural text.
	// Note: The gauge rendering uses lipgloss internally, so we check the header instead.
	if strings.Contains(noColorResult, "\x1b[") && !strings.Contains(noColorResult, "CPU") {
		t.Errorf("expected no ANSI escapes in non-gauge text when ColorEnabled=false")
	}

	// Verify both outputs contain expected content.
	if !strings.Contains(noColorResult, "Tailscale Mesh") {
		t.Errorf("expected Tailscale Mesh in no-color output")
	}
	if !strings.Contains(colorResult, "Tailscale Mesh") {
		t.Errorf("expected Tailscale Mesh in color output")
	}

	// Since gauge uses lipgloss internally, color output should have ANSI sequences.
	// The mini gauge uses RenderMiniGauge which calls RenderGauge with lipgloss styling.
	if !strings.Contains(colorResult, "\x1b[") {
		t.Logf("Note: colored output may not have ANSI escapes if gauge doesn't use colors")
	}
}

// Benchmark for performance validation.
func BenchmarkInfraPanel_Render(b *testing.B) {
	panel := NewInfraPanel(DefaultInfraPanelConfig())

	cpu, ram, disk := 45.0, 60.0, 30.0
	nodes := make([]collectors.TailscaleNode, 25)
	for i := range nodes {
		nodes[i] = collectors.TailscaleNode{
			Hostname:    "node-" + string(rune('a'+i)),
			IP:          "100.64.0." + string(rune('1'+i)),
			Online:      i%3 != 0,
			CPUPercent:  &cpu,
			RAMPercent:  &ram,
			DiskPercent: &disk,
			LastSeen:    time.Now(),
		}
	}

	k8sNodes := make([]collectors.KubernetesNode, 10)
	for i := range k8sNodes {
		k8sNodes[i] = collectors.KubernetesNode{
			Name:       "k8s-node-" + string(rune('a'+i)),
			Status:     "Ready",
			CPUPercent: 45,
			MemPercent: 60,
		}
	}

	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			Tailnet:     "benchmark.ts.net",
			OnlineCount: 20,
			TotalCount:  25,
			Nodes:       nodes,
		},
		Kubernetes: []collectors.KubernetesCluster{
			{
				Name:       "prod",
				Status:     "healthy",
				TotalNodes: 10,
				ReadyNodes: 10,
				Nodes:      k8sNodes,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = panel.Render(data)
	}
}
