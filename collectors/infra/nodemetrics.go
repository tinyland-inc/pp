// Package infra provides infrastructure status collectors for prompt-pulse.
package infra

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// defaultSSHTimeout is the per-node SSH command timeout.
	defaultSSHTimeout = 5 * time.Second

	// defaultMaxConcurrent is the maximum concurrent SSH connections.
	defaultMaxConcurrent = 5
)

// NodeMetricsCollector gathers system metrics from Tailscale nodes via SSH.
// It connects using the Tailscale IP address and runs lightweight commands
// to gather CPU, RAM, and disk utilization.
type NodeMetricsCollector struct {
	sshUser       string
	sshTimeout    time.Duration
	maxConcurrent int
	logger        *slog.Logger
}

// NodeMetricsConfig holds configuration for the node metrics collector.
type NodeMetricsConfig struct {
	// SSHUser is the username for SSH connections.
	// Defaults to the current user if empty.
	SSHUser string

	// SSHTimeout is the per-node SSH command timeout.
	// Defaults to 5 seconds.
	SSHTimeout time.Duration

	// MaxConcurrent is the maximum number of concurrent SSH connections.
	// Defaults to 5.
	MaxConcurrent int
}

// NewNodeMetricsCollector creates a NodeMetricsCollector with the given config.
// If logger is nil, a no-op logger is used.
func NewNodeMetricsCollector(cfg NodeMetricsConfig, logger *slog.Logger) *NodeMetricsCollector {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	timeout := cfg.SSHTimeout
	if timeout == 0 {
		timeout = defaultSSHTimeout
	}

	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrent
	}

	return &NodeMetricsCollector{
		sshUser:       cfg.SSHUser,
		sshTimeout:    timeout,
		maxConcurrent: maxConcurrent,
		logger:        logger,
	}
}

// CollectAll gathers metrics from all online nodes in the given status.
// It modifies the nodes in-place, populating CPUPercent, RAMPercent, and DiskPercent.
// Offline nodes are skipped. Errors for individual nodes are logged but do not
// prevent collection from other nodes.
func (c *NodeMetricsCollector) CollectAll(ctx context.Context, status *collectors.TailscaleStatus) {
	if status == nil || len(status.Nodes) == 0 {
		return
	}

	// Filter to online nodes only.
	var onlineIndices []int
	for i, node := range status.Nodes {
		if node.Online && node.IP != "" {
			onlineIndices = append(onlineIndices, i)
		}
	}

	if len(onlineIndices) == 0 {
		c.logger.Debug("no online nodes with IPs for metrics collection")
		return
	}

	// Use a semaphore to limit concurrent SSH connections.
	sem := make(chan struct{}, c.maxConcurrent)
	var wg sync.WaitGroup

	for _, idx := range onlineIndices {
		wg.Add(1)
		go func(nodeIdx int) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
				defer func() { <-sem }()
			}

			node := &status.Nodes[nodeIdx]
			c.collectNodeMetrics(ctx, node)
		}(idx)
	}

	wg.Wait()
}

// collectNodeMetrics gathers metrics for a single node via SSH.
// It runs a shell command that outputs CPU, RAM, and disk percentages.
func (c *NodeMetricsCollector) collectNodeMetrics(ctx context.Context, node *collectors.TailscaleNode) {
	// Create a context with timeout for this specific node.
	nodeCtx, cancel := context.WithTimeout(ctx, c.sshTimeout)
	defer cancel()

	// Build SSH target.
	target := node.IP
	if c.sshUser != "" {
		target = c.sshUser + "@" + node.IP
	}

	// The metrics command outputs three lines:
	// 1. CPU utilization (from /proc/stat or top)
	// 2. RAM utilization percentage
	// 3. Disk utilization percentage for /
	//
	// This is a cross-platform-ish approach that works on most Linux and macOS systems.
	metricsCmd := `
	if [ -f /proc/stat ]; then
		# Linux: Calculate CPU from /proc/stat
		cpu_idle=$(awk '/^cpu / {print $5}' /proc/stat)
		cpu_total=$(awk '/^cpu / {sum=0; for(i=2;i<=NF;i++) sum+=$i; print sum}' /proc/stat)
		if [ "$cpu_total" -gt 0 ]; then
			cpu_pct=$((100 - (cpu_idle * 100 / cpu_total)))
			echo "$cpu_pct"
		else
			echo "-1"
		fi
		# Linux: RAM from /proc/meminfo
		mem_total=$(awk '/^MemTotal:/ {print $2}' /proc/meminfo)
		mem_avail=$(awk '/^MemAvailable:/ {print $2}' /proc/meminfo)
		if [ "$mem_total" -gt 0 ]; then
			mem_pct=$(( (mem_total - mem_avail) * 100 / mem_total ))
			echo "$mem_pct"
		else
			echo "-1"
		fi
	else
		# macOS fallback
		top -l 1 | awk '/CPU usage/ {gsub(/%/,"",$3); print int($3)}' 2>/dev/null || echo "-1"
		vm_stat | awk 'BEGIN{free=0;active=0;inactive=0;wired=0;comp=0} /^Pages free:/{free=$3} /^Pages active:/{active=$3} /^Pages inactive:/{inactive=$3} /^Pages wired down:/{wired=$4} /^Pages occupied by compressor:/{comp=$5} END{gsub(/\./,"",free);gsub(/\./,"",active);gsub(/\./,"",inactive);gsub(/\./,"",wired);gsub(/\./,"",comp);total=free+active+inactive+wired+comp;if(total>0){used=active+wired+comp;print int(used*100/total)}else{print -1}}' 2>/dev/null || echo "-1"
	fi
	# Disk usage for / (works on both Linux and macOS)
	df -P / 2>/dev/null | awk 'NR==2 {gsub(/%/,"",$5); print $5}' || echo "-1"
`

	cmd := exec.CommandContext(nodeCtx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=3",
		"-o", "StrictHostKeyChecking=accept-new",
		target,
		"sh", "-c", metricsCmd,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	c.logger.Debug("collecting metrics from node",
		"hostname", node.Hostname,
		"ip", node.IP,
	)

	err := cmd.Run()
	if err != nil {
		c.logger.Warn("failed to collect metrics from node",
			"hostname", node.Hostname,
			"ip", node.IP,
			"error", err,
			"stderr", strings.TrimSpace(stderr.String()),
		)
		return
	}

	// Parse the output.
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) >= 3 {
		if cpu, err := strconv.ParseFloat(strings.TrimSpace(lines[0]), 64); err == nil && cpu >= 0 {
			node.CPUPercent = &cpu
		}
		if ram, err := strconv.ParseFloat(strings.TrimSpace(lines[1]), 64); err == nil && ram >= 0 {
			node.RAMPercent = &ram
		}
		if disk, err := strconv.ParseFloat(strings.TrimSpace(lines[2]), 64); err == nil && disk >= 0 {
			node.DiskPercent = &disk
		}
	}

	c.logger.Debug("collected metrics from node",
		"hostname", node.Hostname,
		"cpu", node.CPUPercent,
		"ram", node.RAMPercent,
		"disk", node.DiskPercent,
	)
}
