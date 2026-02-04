package infra

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeKubeTestScript creates an executable shell script in a temp directory.
func writeKubeTestScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\n" + content
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write test script %s: %v", path, err)
	}
	return path
}

// setKubectl overrides the kubectlCommand for testing and returns a cleanup function.
func setKubectl(t *testing.T, path string) {
	t.Helper()
	old := kubectlCommand
	kubectlCommand = path
	t.Cleanup(func() { kubectlCommand = old })
}

// canned JSON responses for testing

const healthyNodesJSON = `{
  "items": [
    {
      "metadata": {"name": "node-1"},
      "status": {
        "conditions": [{"type": "Ready", "status": "True"}],
        "allocatable": {"cpu": "4", "memory": "8Gi", "pods": "110"}
      }
    },
    {
      "metadata": {"name": "node-2"},
      "status": {
        "conditions": [{"type": "Ready", "status": "True"}],
        "allocatable": {"cpu": "4", "memory": "8Gi", "pods": "110"}
      }
    },
    {
      "metadata": {"name": "node-3"},
      "status": {
        "conditions": [{"type": "Ready", "status": "True"}],
        "allocatable": {"cpu": "8", "memory": "16Gi", "pods": "250"}
      }
    }
  ]
}`

const degradedNodesJSON = `{
  "items": [
    {
      "metadata": {"name": "node-1"},
      "status": {
        "conditions": [{"type": "Ready", "status": "True"}],
        "allocatable": {"cpu": "4", "memory": "8Gi", "pods": "110"}
      }
    },
    {
      "metadata": {"name": "node-2"},
      "status": {
        "conditions": [{"type": "Ready", "status": "True"}],
        "allocatable": {"cpu": "4", "memory": "8Gi", "pods": "110"}
      }
    },
    {
      "metadata": {"name": "node-3"},
      "status": {
        "conditions": [{"type": "Ready", "status": "False"}],
        "allocatable": {"cpu": "8", "memory": "16Gi", "pods": "250"}
      }
    }
  ]
}`

const emptyNodesJSON = `{"items": []}`

const topNodesOutput = `node-1   250m   12%   1024Mi   40%
node-2   500m   25%   2048Mi   60%
node-3   100m   5%    512Mi    10%
`

const podsJSON = `{
  "items": [
    {"spec": {"nodeName": "node-1"}},
    {"spec": {"nodeName": "node-1"}},
    {"spec": {"nodeName": "node-1"}},
    {"spec": {"nodeName": "node-2"}},
    {"spec": {"nodeName": "node-2"}},
    {"spec": {"nodeName": "node-3"}}
  ]
}`

const clusterInfoOutput = "Kubernetes control plane is running at https://10.0.0.1:6443\n"

// buildMockKubectl creates a script that dispatches based on the kubectl
// subcommand. It uses grep-based matching since shell case patterns cannot
// contain unquoted spaces portably. The responses map keys are grep-compatible
// patterns (e.g., "get nodes", "top nodes", "cluster-info").
func buildMockKubectl(t *testing.T, dir string, responses map[string]struct {
	output   string
	exitCode int
}) string {
	t.Helper()

	// Write each response as a separate file and build if/elif chain.
	var chain string
	i := 0
	for keyword, resp := range responses {
		respFile := filepath.Join(dir, "resp_"+itoa(i))
		if err := os.WriteFile(respFile, []byte(resp.output), 0644); err != nil {
			t.Fatalf("failed to write response file: %v", err)
		}

		prefix := "elif"
		if i == 0 {
			prefix = "if"
		}
		chain += prefix + " echo \"$ARGS\" | grep -q '" + keyword + "'; then\n"
		chain += "  cat '" + respFile + "'\n"
		chain += "  exit " + itoa(resp.exitCode) + "\n"
		i++
	}

	scriptBody := `ARGS="$*"
` + chain + `else
  echo "mock kubectl: unhandled args: $ARGS" >&2
  exit 1
fi
`
	return writeKubeTestScript(t, dir, "kubectl", scriptBody)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

func TestFetchCluster_HealthyCluster(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires Unix shell scripts")
	}

	dir := t.TempDir()
	mock := buildMockKubectl(t, dir, map[string]struct {
		output   string
		exitCode int
	}{
		"get nodes":    {output: healthyNodesJSON, exitCode: 0},
		"top nodes":    {output: topNodesOutput, exitCode: 0},
		"get pods":     {output: podsJSON, exitCode: 0},
		"cluster-info": {output: clusterInfoOutput, exitCode: 0},
	})
	setKubectl(t, mock)

	client := NewKubectlClient(nil)
	ctx := context.Background()
	kubeCtx := KubeContextConfig{
		Name:         "test-context",
		Kubeconfig:   "/tmp/fake-kubeconfig",
		Platform:     "civo",
		DashboardURL: "https://dashboard.example.com",
	}

	cluster, err := client.FetchCluster(ctx, kubeCtx)
	if err != nil {
		t.Fatalf("FetchCluster() unexpected error: %v", err)
	}

	if cluster.Name != "test-context" {
		t.Errorf("Name = %q, want %q", cluster.Name, "test-context")
	}
	if cluster.Platform != "civo" {
		t.Errorf("Platform = %q, want %q", cluster.Platform, "civo")
	}
	if cluster.Status != "healthy" {
		t.Errorf("Status = %q, want %q", cluster.Status, "healthy")
	}
	if cluster.DashboardURL != "https://dashboard.example.com" {
		t.Errorf("DashboardURL = %q, want %q", cluster.DashboardURL, "https://dashboard.example.com")
	}
	if cluster.TotalNodes != 3 {
		t.Errorf("TotalNodes = %d, want 3", cluster.TotalNodes)
	}
	if cluster.ReadyNodes != 3 {
		t.Errorf("ReadyNodes = %d, want 3", cluster.ReadyNodes)
	}
	if len(cluster.Nodes) != 3 {
		t.Fatalf("len(Nodes) = %d, want 3", len(cluster.Nodes))
	}

	// Verify node-1 metrics.
	n1 := cluster.Nodes[0]
	if n1.Name != "node-1" {
		t.Errorf("Nodes[0].Name = %q, want %q", n1.Name, "node-1")
	}
	if n1.Status != "Ready" {
		t.Errorf("Nodes[0].Status = %q, want %q", n1.Status, "Ready")
	}
	if n1.CPUPercent != 12 {
		t.Errorf("Nodes[0].CPUPercent = %f, want 12", n1.CPUPercent)
	}
	if n1.MemPercent != 40 {
		t.Errorf("Nodes[0].MemPercent = %f, want 40", n1.MemPercent)
	}
	if n1.PodCount != 3 {
		t.Errorf("Nodes[0].PodCount = %d, want 3", n1.PodCount)
	}
	if n1.MaxPods != 110 {
		t.Errorf("Nodes[0].MaxPods = %d, want 110", n1.MaxPods)
	}

	// Verify node-3 (fewer pods, larger node).
	n3 := cluster.Nodes[2]
	if n3.PodCount != 1 {
		t.Errorf("Nodes[2].PodCount = %d, want 1", n3.PodCount)
	}
	if n3.MaxPods != 250 {
		t.Errorf("Nodes[2].MaxPods = %d, want 250", n3.MaxPods)
	}

	// Verify API endpoint was parsed.
	if cluster.APIEndpoint != "https://10.0.0.1:6443" {
		t.Errorf("APIEndpoint = %q, want %q", cluster.APIEndpoint, "https://10.0.0.1:6443")
	}
}

func TestFetchCluster_DegradedCluster(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires Unix shell scripts")
	}

	dir := t.TempDir()
	mock := buildMockKubectl(t, dir, map[string]struct {
		output   string
		exitCode int
	}{
		"get nodes":    {output: degradedNodesJSON, exitCode: 0},
		"top nodes":    {output: topNodesOutput, exitCode: 0},
		"get pods":     {output: podsJSON, exitCode: 0},
		"cluster-info": {output: clusterInfoOutput, exitCode: 0},
	})
	setKubectl(t, mock)

	client := NewKubectlClient(nil)
	cluster, err := client.FetchCluster(context.Background(), KubeContextConfig{
		Name: "degraded-ctx",
	})
	if err != nil {
		t.Fatalf("FetchCluster() unexpected error: %v", err)
	}

	if cluster.Status != "degraded" {
		t.Errorf("Status = %q, want %q", cluster.Status, "degraded")
	}
	if cluster.TotalNodes != 3 {
		t.Errorf("TotalNodes = %d, want 3", cluster.TotalNodes)
	}
	if cluster.ReadyNodes != 2 {
		t.Errorf("ReadyNodes = %d, want 2", cluster.ReadyNodes)
	}

	// Verify node-3 is NotReady.
	if cluster.Nodes[2].Status != "NotReady" {
		t.Errorf("Nodes[2].Status = %q, want %q", cluster.Nodes[2].Status, "NotReady")
	}
}

func TestFetchCluster_MetricsUnavailable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires Unix shell scripts")
	}

	dir := t.TempDir()
	mock := buildMockKubectl(t, dir, map[string]struct {
		output   string
		exitCode int
	}{
		"get nodes":    {output: healthyNodesJSON, exitCode: 0},
		"top nodes":    {output: "error: Metrics API not available", exitCode: 1},
		"get pods":     {output: podsJSON, exitCode: 0},
		"cluster-info": {output: clusterInfoOutput, exitCode: 0},
	})
	setKubectl(t, mock)

	client := NewKubectlClient(nil)
	cluster, err := client.FetchCluster(context.Background(), KubeContextConfig{
		Name: "no-metrics-ctx",
	})
	if err != nil {
		t.Fatalf("FetchCluster() unexpected error: %v", err)
	}

	// Cluster should still be healthy; metrics are best-effort.
	if cluster.Status != "healthy" {
		t.Errorf("Status = %q, want %q", cluster.Status, "healthy")
	}
	if cluster.TotalNodes != 3 {
		t.Errorf("TotalNodes = %d, want 3", cluster.TotalNodes)
	}

	// CPU and memory should be zero since top failed.
	for i, node := range cluster.Nodes {
		if node.CPUPercent != 0 {
			t.Errorf("Nodes[%d].CPUPercent = %f, want 0", i, node.CPUPercent)
		}
		if node.MemPercent != 0 {
			t.Errorf("Nodes[%d].MemPercent = %f, want 0", i, node.MemPercent)
		}
	}
}

func TestFetchCluster_KubectlNotFound(t *testing.T) {
	setKubectl(t, "/nonexistent/path/kubectl")

	client := NewKubectlClient(nil)
	_, err := client.FetchCluster(context.Background(), KubeContextConfig{
		Name: "test",
	})
	if err == nil {
		t.Fatal("FetchCluster() expected error for missing kubectl, got nil")
	}
	if !contains(err.Error(), "kubectl not found") {
		t.Errorf("error = %q, want message containing %q", err.Error(), "kubectl not found")
	}
}

func TestFetchCluster_ContextUnreachable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires Unix shell scripts")
	}

	dir := t.TempDir()
	// All kubectl commands fail (context does not exist).
	mock := buildMockKubectl(t, dir, map[string]struct {
		output   string
		exitCode int
	}{
		"get nodes":    {output: "error: context not found", exitCode: 1},
		"top nodes":    {output: "", exitCode: 1},
		"get pods":     {output: "", exitCode: 1},
		"cluster-info": {output: "", exitCode: 1},
	})
	setKubectl(t, mock)

	client := NewKubectlClient(nil)
	cluster, err := client.FetchCluster(context.Background(), KubeContextConfig{
		Name: "bad-context",
	})
	if err != nil {
		t.Fatalf("FetchCluster() unexpected error: %v", err)
	}

	if cluster.Status != "offline" {
		t.Errorf("Status = %q, want %q", cluster.Status, "offline")
	}
	if cluster.TotalNodes != 0 {
		t.Errorf("TotalNodes = %d, want 0", cluster.TotalNodes)
	}
}

func TestFetchCluster_EmptyCluster(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires Unix shell scripts")
	}

	dir := t.TempDir()
	mock := buildMockKubectl(t, dir, map[string]struct {
		output   string
		exitCode int
	}{
		"get nodes":    {output: emptyNodesJSON, exitCode: 0},
		"top nodes":    {output: "", exitCode: 0},
		"get pods":     {output: `{"items": []}`, exitCode: 0},
		"cluster-info": {output: clusterInfoOutput, exitCode: 0},
	})
	setKubectl(t, mock)

	client := NewKubectlClient(nil)
	cluster, err := client.FetchCluster(context.Background(), KubeContextConfig{
		Name: "empty-ctx",
	})
	if err != nil {
		t.Fatalf("FetchCluster() unexpected error: %v", err)
	}

	if cluster.Status != "healthy" {
		t.Errorf("Status = %q, want %q", cluster.Status, "healthy")
	}
	if cluster.TotalNodes != 0 {
		t.Errorf("TotalNodes = %d, want 0", cluster.TotalNodes)
	}
	if cluster.ReadyNodes != 0 {
		t.Errorf("ReadyNodes = %d, want 0", cluster.ReadyNodes)
	}
	if len(cluster.Nodes) != 0 {
		t.Errorf("len(Nodes) = %d, want 0", len(cluster.Nodes))
	}
}

func TestFetchCluster_PodsDistributed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires Unix shell scripts")
	}

	// Custom pod distribution: 5 pods on node-1, 2 on node-2, 0 on node-3.
	customPods := `{
  "items": [
    {"spec": {"nodeName": "node-1"}},
    {"spec": {"nodeName": "node-1"}},
    {"spec": {"nodeName": "node-1"}},
    {"spec": {"nodeName": "node-1"}},
    {"spec": {"nodeName": "node-1"}},
    {"spec": {"nodeName": "node-2"}},
    {"spec": {"nodeName": "node-2"}}
  ]
}`

	dir := t.TempDir()
	mock := buildMockKubectl(t, dir, map[string]struct {
		output   string
		exitCode int
	}{
		"get nodes":    {output: healthyNodesJSON, exitCode: 0},
		"top nodes":    {output: "", exitCode: 1},
		"get pods":     {output: customPods, exitCode: 0},
		"cluster-info": {output: "", exitCode: 1},
	})
	setKubectl(t, mock)

	client := NewKubectlClient(nil)
	cluster, err := client.FetchCluster(context.Background(), KubeContextConfig{
		Name: "pod-dist-ctx",
	})
	if err != nil {
		t.Fatalf("FetchCluster() unexpected error: %v", err)
	}

	if cluster.Nodes[0].PodCount != 5 {
		t.Errorf("Nodes[0].PodCount = %d, want 5", cluster.Nodes[0].PodCount)
	}
	if cluster.Nodes[1].PodCount != 2 {
		t.Errorf("Nodes[1].PodCount = %d, want 2", cluster.Nodes[1].PodCount)
	}
	if cluster.Nodes[2].PodCount != 0 {
		t.Errorf("Nodes[2].PodCount = %d, want 0", cluster.Nodes[2].PodCount)
	}
}

func TestFetchCluster_ZeroAllocatablePods(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test requires Unix shell scripts")
	}

	noPodsAllocJSON := `{
  "items": [
    {
      "metadata": {"name": "node-1"},
      "status": {
        "conditions": [{"type": "Ready", "status": "True"}],
        "allocatable": {"cpu": "4", "memory": "8Gi", "pods": "0"}
      }
    }
  ]
}`

	dir := t.TempDir()
	mock := buildMockKubectl(t, dir, map[string]struct {
		output   string
		exitCode int
	}{
		"get nodes":    {output: noPodsAllocJSON, exitCode: 0},
		"top nodes":    {output: "", exitCode: 1},
		"get pods":     {output: `{"items": []}`, exitCode: 0},
		"cluster-info": {output: "", exitCode: 1},
	})
	setKubectl(t, mock)

	client := NewKubectlClient(nil)
	cluster, err := client.FetchCluster(context.Background(), KubeContextConfig{
		Name: "zero-pods-ctx",
	})
	if err != nil {
		t.Fatalf("FetchCluster() unexpected error: %v", err)
	}

	if cluster.Nodes[0].MaxPods != 0 {
		t.Errorf("Nodes[0].MaxPods = %d, want 0", cluster.Nodes[0].MaxPods)
	}
}

// ========== Helper function unit tests ==========

func TestParseTopOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantNode string
		wantCPU  float64
		wantMem  float64
	}{
		{
			name:     "standard output",
			input:    "node-1   250m   12%   1024Mi   40%\nnode-2   500m   25%   2048Mi   60%\n",
			wantLen:  2,
			wantNode: "node-1",
			wantCPU:  12,
			wantMem:  40,
		},
		{
			name:    "empty output",
			input:   "",
			wantLen: 0,
		},
		{
			name:    "whitespace only",
			input:   "   \n  \n",
			wantLen: 0,
		},
		{
			name:     "single node",
			input:    "master   1000m   50%   4096Mi   80%\n",
			wantLen:  1,
			wantNode: "master",
			wantCPU:  50,
			wantMem:  80,
		},
		{
			name:     "fractional percentages",
			input:    "node-x   100m   3%   256Mi   7%\n",
			wantLen:  1,
			wantNode: "node-x",
			wantCPU:  3,
			wantMem:  7,
		},
		{
			name:    "malformed line (too few fields)",
			input:   "node-1   250m\n",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTopOutput(tt.input)
			if len(result) != tt.wantLen {
				t.Errorf("len(result) = %d, want %d", len(result), tt.wantLen)
				return
			}
			if tt.wantNode != "" {
				metrics, ok := result[tt.wantNode]
				if !ok {
					t.Errorf("node %q not found in result", tt.wantNode)
					return
				}
				if metrics.cpuPercent != tt.wantCPU {
					t.Errorf("cpuPercent = %f, want %f", metrics.cpuPercent, tt.wantCPU)
				}
				if metrics.memPercent != tt.wantMem {
					t.Errorf("memPercent = %f, want %f", metrics.memPercent, tt.wantMem)
				}
			}
		})
	}
}

func TestCountPodsPerNode(t *testing.T) {
	tests := []struct {
		name string
		pods kubePodList
		want map[string]int
	}{
		{
			name: "pods across multiple nodes",
			pods: kubePodList{Items: []kubePod{
				{Spec: kubePodSpec{NodeName: "node-1"}},
				{Spec: kubePodSpec{NodeName: "node-1"}},
				{Spec: kubePodSpec{NodeName: "node-2"}},
			}},
			want: map[string]int{"node-1": 2, "node-2": 1},
		},
		{
			name: "empty pod list",
			pods: kubePodList{Items: []kubePod{}},
			want: map[string]int{},
		},
		{
			name: "pods with no node name (pending)",
			pods: kubePodList{Items: []kubePod{
				{Spec: kubePodSpec{NodeName: ""}},
				{Spec: kubePodSpec{NodeName: "node-1"}},
			}},
			want: map[string]int{"node-1": 1},
		},
		{
			name: "all pods on one node",
			pods: kubePodList{Items: []kubePod{
				{Spec: kubePodSpec{NodeName: "node-1"}},
				{Spec: kubePodSpec{NodeName: "node-1"}},
				{Spec: kubePodSpec{NodeName: "node-1"}},
			}},
			want: map[string]int{"node-1": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countPodsPerNode(tt.pods)
			if len(result) != len(tt.want) {
				t.Errorf("len(result) = %d, want %d", len(result), len(tt.want))
			}
			for node, count := range tt.want {
				if result[node] != count {
					t.Errorf("count[%q] = %d, want %d", node, result[node], count)
				}
			}
		})
	}
}

func TestParseNodeConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions []kubeNodeCondition
		want       string
	}{
		{
			name: "ready node",
			conditions: []kubeNodeCondition{
				{Type: "MemoryPressure", Status: "False"},
				{Type: "DiskPressure", Status: "False"},
				{Type: "Ready", Status: "True"},
			},
			want: "Ready",
		},
		{
			name: "not ready node",
			conditions: []kubeNodeCondition{
				{Type: "Ready", Status: "False"},
			},
			want: "NotReady",
		},
		{
			name: "unknown status",
			conditions: []kubeNodeCondition{
				{Type: "Ready", Status: "Unknown"},
			},
			want: "Unknown",
		},
		{
			name:       "no conditions",
			conditions: []kubeNodeCondition{},
			want:       "Unknown",
		},
		{
			name: "no ready condition present",
			conditions: []kubeNodeCondition{
				{Type: "MemoryPressure", Status: "False"},
				{Type: "DiskPressure", Status: "False"},
			},
			want: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNodeConditions(tt.conditions)
			if result != tt.want {
				t.Errorf("parseNodeConditions() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestParseAllocatable(t *testing.T) {
	tests := []struct {
		name  string
		alloc kubeNodeAllocatable
		want  int
	}{
		{
			name:  "standard value",
			alloc: kubeNodeAllocatable{Pods: "110"},
			want:  110,
		},
		{
			name:  "large value",
			alloc: kubeNodeAllocatable{Pods: "250"},
			want:  250,
		},
		{
			name:  "zero",
			alloc: kubeNodeAllocatable{Pods: "0"},
			want:  0,
		},
		{
			name:  "empty string",
			alloc: kubeNodeAllocatable{Pods: ""},
			want:  0,
		},
		{
			name:  "non-numeric",
			alloc: kubeNodeAllocatable{Pods: "abc"},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAllocatable(tt.alloc)
			if result != tt.want {
				t.Errorf("parseAllocatable() = %d, want %d", result, tt.want)
			}
		})
	}
}

func TestParsePercentField(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"12%", 12},
		{"0%", 0},
		{"100%", 100},
		{"5", 5},
		{"abc", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parsePercentField(tt.input)
			if result != tt.want {
				t.Errorf("parsePercentField(%q) = %f, want %f", tt.input, result, tt.want)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no escape codes",
			input: "plain text",
			want:  "plain text",
		},
		{
			name:  "color codes",
			input: "\033[32mKubernetes control plane\033[0m is running at https://10.0.0.1:6443",
			want:  "Kubernetes control plane is running at https://10.0.0.1:6443",
		},
		{
			name:  "bold and color",
			input: "\033[1;33mWarning\033[0m: something",
			want:  "Warning: something",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripANSI(tt.input)
			if result != tt.want {
				t.Errorf("stripANSI() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestNewKubectlClient_NilLogger(t *testing.T) {
	client := NewKubectlClient(nil)
	if client == nil {
		t.Fatal("NewKubectlClient(nil) returned nil")
	}
}

// contains is a helper to check substring membership.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
