// Package infra provides infrastructure collectors for prompt-pulse,
// including Kubernetes cluster metrics and Tailscale mesh status.
package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// kubectlCommand is the path to the kubectl binary. It is a package-level
// variable so tests can override it with a test script.
var kubectlCommand = "kubectl"

// KubeContextConfig holds the configuration needed to connect to a single
// Kubernetes cluster context.
type KubeContextConfig struct {
	Name         string
	Kubeconfig   string
	Namespace    string
	Platform     string
	DashboardURL string
}

// KubernetesFetcher defines the interface for fetching Kubernetes cluster metrics.
type KubernetesFetcher interface {
	FetchCluster(ctx context.Context, kubeContext KubeContextConfig) (*collectors.KubernetesCluster, error)
}

// KubectlClient implements KubernetesFetcher by shelling out to kubectl.
type KubectlClient struct {
	logger *slog.Logger
}

// NewKubectlClient creates a new KubectlClient with the given logger.
// If logger is nil, a default no-op logger is used.
func NewKubectlClient(logger *slog.Logger) *KubectlClient {
	if logger == nil {
		logger = slog.Default()
	}
	return &KubectlClient{logger: logger}
}

// FetchCluster gathers node status, resource usage, and pod counts for the
// given Kubernetes context. If kubectl is not available or the context is
// unreachable, the cluster is reported as "offline". If metrics-server is
// not installed, CPU and memory percentages are set to zero.
func (k *KubectlClient) FetchCluster(ctx context.Context, kubeCtx KubeContextConfig) (*collectors.KubernetesCluster, error) {
	cluster := &collectors.KubernetesCluster{
		Name:         kubeCtx.Name,
		Platform:     kubeCtx.Platform,
		DashboardURL: kubeCtx.DashboardURL,
	}

	// Check that kubectl exists.
	if _, err := exec.LookPath(kubectlCommand); err != nil {
		return nil, fmt.Errorf("kubectl not found: %w", err)
	}

	baseArgs := k.baseArgs(kubeCtx)

	// Step 1: Get nodes.
	nodeListJSON, err := k.runKubectl(ctx, append(baseArgs, "get", "nodes", "-o", "json"))
	if err != nil {
		cluster.Status = "offline"
		k.logger.Warn("failed to get nodes, marking cluster offline",
			"context", kubeCtx.Name, "error", err)
		return cluster, nil
	}

	var nodeList kubeNodeList
	if err := json.Unmarshal([]byte(nodeListJSON), &nodeList); err != nil {
		cluster.Status = "offline"
		k.logger.Warn("failed to parse node list JSON",
			"context", kubeCtx.Name, "error", err)
		return cluster, nil
	}

	// Build nodes slice and determine readiness.
	nodes := make([]collectors.KubernetesNode, 0, len(nodeList.Items))
	readyCount := 0
	for _, item := range nodeList.Items {
		status := parseNodeConditions(item.Status.Conditions)
		maxPods := parseAllocatable(item.Status.Allocatable)
		node := collectors.KubernetesNode{
			Name:    item.Metadata.Name,
			Status:  status,
			MaxPods: maxPods,
		}
		if status == "Ready" {
			readyCount++
		}
		nodes = append(nodes, node)
	}

	// Step 2: Get resource usage from metrics-server (best effort).
	topOutput, err := k.runKubectl(ctx, append(baseArgs, "top", "nodes", "--no-headers"))
	if err != nil {
		k.logger.Info("kubectl top nodes failed (metrics-server may not be installed)",
			"context", kubeCtx.Name, "error", err)
	} else {
		topMap := parseTopOutput(topOutput)
		for i := range nodes {
			if metrics, ok := topMap[nodes[i].Name]; ok {
				nodes[i].CPUPercent = metrics.cpuPercent
				nodes[i].MemPercent = metrics.memPercent
			}
		}
	}

	// Step 3: Count pods per node.
	podListJSON, err := k.runKubectl(ctx, append(baseArgs, "get", "pods", "-A", "-o", "json"))
	if err != nil {
		k.logger.Info("failed to get pods, pod counts will be zero",
			"context", kubeCtx.Name, "error", err)
	} else {
		var podList kubePodList
		if err := json.Unmarshal([]byte(podListJSON), &podList); err != nil {
			k.logger.Warn("failed to parse pod list JSON",
				"context", kubeCtx.Name, "error", err)
		} else {
			podCounts := countPodsPerNode(podList)
			for i := range nodes {
				nodes[i].PodCount = podCounts[nodes[i].Name]
			}
		}
	}

	// Step 4: Get API endpoint from cluster-info.
	cluster.APIEndpoint = k.fetchAPIEndpoint(ctx, baseArgs)

	// Assemble final result.
	cluster.Nodes = nodes
	cluster.TotalNodes = len(nodes)
	cluster.ReadyNodes = readyCount

	switch {
	case len(nodes) == 0:
		cluster.Status = "healthy" // empty cluster is not degraded
	case readyCount == len(nodes):
		cluster.Status = "healthy"
	default:
		cluster.Status = "degraded"
	}

	return cluster, nil
}

// baseArgs returns the common kubectl arguments for a context.
func (k *KubectlClient) baseArgs(kubeCtx KubeContextConfig) []string {
	var args []string
	if kubeCtx.Kubeconfig != "" {
		args = append(args, "--kubeconfig="+kubeCtx.Kubeconfig)
	}
	if kubeCtx.Name != "" {
		args = append(args, "--context="+kubeCtx.Name)
	}
	return args
}

// runKubectl executes kubectl with the given arguments and returns stdout.
func (k *KubectlClient) runKubectl(ctx context.Context, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, kubectlCommand, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("kubectl %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// fetchAPIEndpoint attempts to extract the API server URL from cluster-info.
func (k *KubectlClient) fetchAPIEndpoint(ctx context.Context, baseArgs []string) string {
	output, err := k.runKubectl(ctx, append(baseArgs, "cluster-info"))
	if err != nil {
		return ""
	}
	// cluster-info output contains ANSI escape codes. The first line is typically:
	// "Kubernetes control plane is running at https://..."
	// Strip ANSI codes and extract the URL.
	for _, line := range strings.Split(output, "\n") {
		cleaned := stripANSI(line)
		if strings.Contains(cleaned, "control plane") || strings.Contains(cleaned, "master") {
			parts := strings.Fields(cleaned)
			for _, part := range parts {
				if strings.HasPrefix(part, "https://") || strings.HasPrefix(part, "http://") {
					return part
				}
			}
		}
	}
	return ""
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\033' {
			// Skip escape sequence.
			i++
			if i < len(s) && s[i] == '[' {
				i++
				for i < len(s) && !((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
					i++
				}
				if i < len(s) {
					i++ // skip the final letter
				}
			}
			continue
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// ========== JSON parsing structs (unexported) ==========

type kubeNodeList struct {
	Items []kubeNode `json:"items"`
}

type kubeNode struct {
	Metadata kubeNodeMetadata `json:"metadata"`
	Status   kubeNodeStatus   `json:"status"`
}

type kubeNodeMetadata struct {
	Name string `json:"name"`
}

type kubeNodeStatus struct {
	Conditions  []kubeNodeCondition   `json:"conditions"`
	Allocatable kubeNodeAllocatable   `json:"allocatable"`
}

type kubeNodeCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type kubeNodeAllocatable struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Pods   string `json:"pods"`
}

type kubePodList struct {
	Items []kubePod `json:"items"`
}

type kubePod struct {
	Spec kubePodSpec `json:"spec"`
}

type kubePodSpec struct {
	NodeName string `json:"nodeName"`
}

type topMetrics struct {
	cpuPercent float64
	memPercent float64
}

// ========== Helper functions ==========

// parseNodeConditions extracts the node readiness status from its conditions list.
// Returns "Ready" if the Ready condition is True, "NotReady" if False, or "Unknown".
func parseNodeConditions(conditions []kubeNodeCondition) string {
	for _, cond := range conditions {
		if cond.Type == "Ready" {
			switch cond.Status {
			case "True":
				return "Ready"
			case "False":
				return "NotReady"
			default:
				return "Unknown"
			}
		}
	}
	return "Unknown"
}

// parseTopOutput parses the output of `kubectl top nodes --no-headers`.
// Each line has the format: NODE_NAME CPU(cores) CPU% MEMORY(bytes) MEMORY%
// Example: "node-1   250m   12%   1024Mi   40%"
// Returns a map from node name to metrics.
func parseTopOutput(output string) map[string]topMetrics {
	result := make(map[string]topMetrics)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		name := fields[0]
		cpuPct := parsePercentField(fields[2])
		memPct := parsePercentField(fields[4])
		result[name] = topMetrics{
			cpuPercent: cpuPct,
			memPercent: memPct,
		}
	}
	return result
}

// parsePercentField parses a percentage string like "12%" into a float64 (12.0).
func parsePercentField(s string) float64 {
	s = strings.TrimSuffix(s, "%")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

// countPodsPerNode counts the number of pods assigned to each node.
func countPodsPerNode(podList kubePodList) map[string]int {
	counts := make(map[string]int)
	for _, pod := range podList.Items {
		if pod.Spec.NodeName != "" {
			counts[pod.Spec.NodeName]++
		}
	}
	return counts
}

// parseAllocatable extracts the max pods value from a node's allocatable resources.
func parseAllocatable(alloc kubeNodeAllocatable) int {
	if alloc.Pods == "" {
		return 0
	}
	val, err := strconv.Atoi(alloc.Pods)
	if err != nil {
		return 0
	}
	return val
}
