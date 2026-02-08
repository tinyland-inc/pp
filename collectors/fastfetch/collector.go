package fastfetch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// collectorName is the unique identifier for this collector.
	collectorName = "fastfetch"

	// collectorDescription describes what this collector gathers.
	collectorDescription = "System information via fastfetch (OS, CPU, Memory, Disk, etc.)"

	// defaultInterval is the recommended polling interval.
	// Fastfetch data is relatively static, so we poll infrequently.
	defaultInterval = 10 * time.Minute

	// defaultTimeout is the maximum time allowed for fastfetch execution.
	defaultTimeout = 5 * time.Second
)

// FastfetchCollectorConfig holds configuration for the fastfetch collector.
type FastfetchCollectorConfig struct {
	// Binary is the path to the fastfetch binary.
	// If empty, it will be looked up in PATH.
	Binary string

	// Modules is the list of modules to request from fastfetch.
	// If empty, a default set of core modules is used.
	Modules []string

	// Timeout is the maximum duration for fastfetch execution.
	Timeout time.Duration
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() FastfetchCollectorConfig {
	return FastfetchCollectorConfig{
		Binary:  "fastfetch",
		Modules: defaultModules,
		Timeout: defaultTimeout,
	}
}

// defaultModules is the list of modules to request by default.
// These are the 12 core modules specified in the requirements.
var defaultModules = []string{
	"os",
	"host",
	"kernel",
	"uptime",
	"packages",
	"shell",
	"terminal",
	"cpu",
	"gpu",
	"memory",
	"disk",
	"localip",
}

// FastfetchCollector implements collectors.Collector for system information.
type FastfetchCollector struct {
	config FastfetchCollectorConfig
	logger *slog.Logger

	// lookPath allows injection of exec.LookPath for testing.
	lookPath func(string) (string, error)

	// execCommand allows injection of command execution for testing.
	execCommand func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewFastfetchCollector creates a new FastfetchCollector with the given configuration.
// If logger is nil, a no-op logger is used.
func NewFastfetchCollector(config FastfetchCollectorConfig, logger *slog.Logger) *FastfetchCollector {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	if config.Binary == "" {
		config.Binary = "fastfetch"
	}
	if len(config.Modules) == 0 {
		config.Modules = defaultModules
	}
	if config.Timeout == 0 {
		config.Timeout = defaultTimeout
	}

	return &FastfetchCollector{
		config:      config,
		logger:      logger,
		lookPath:    exec.LookPath,
		execCommand: exec.CommandContext,
	}
}

// Name returns the collector's unique identifier.
func (c *FastfetchCollector) Name() string {
	return collectorName
}

// Description returns a human-readable description of what this collector gathers.
func (c *FastfetchCollector) Description() string {
	return collectorDescription
}

// Interval returns the recommended polling interval for this collector.
func (c *FastfetchCollector) Interval() time.Duration {
	return defaultInterval
}

// Collect gathers system information via fastfetch.
// It executes fastfetch with --json output and parses the result.
// Returns a graceful fallback with warnings if fastfetch is not available.
func (c *FastfetchCollector) Collect(ctx context.Context) (*collectors.CollectResult, error) {
	// Check for context cancellation before starting.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var warnings []string

	// Verify fastfetch is available.
	binaryPath, err := c.findBinary()
	if err != nil {
		c.logger.Debug("fastfetch binary not found", "error", err)
		warnings = append(warnings, "fastfetch not installed")

		// Return empty data with warning instead of error.
		return &collectors.CollectResult{
			Collector: collectorName,
			Timestamp: time.Now(),
			Data:      &FastfetchData{},
			Warnings:  warnings,
		}, nil
	}

	// Create context with timeout.
	execCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// Execute fastfetch with JSON output.
	data, execWarnings := c.executeFastfetch(execCtx, binaryPath)
	warnings = append(warnings, execWarnings...)

	return &collectors.CollectResult{
		Collector: collectorName,
		Timestamp: time.Now(),
		Data:      data,
		Warnings:  warnings,
	}, nil
}

// findBinary locates the fastfetch binary.
// Returns the full path or an error if not found.
func (c *FastfetchCollector) findBinary() (string, error) {
	// If an absolute path is configured, verify it exists.
	if strings.HasPrefix(c.config.Binary, "/") {
		_, err := c.lookPath(c.config.Binary)
		if err != nil {
			return "", err
		}
		return c.config.Binary, nil
	}

	// Look up in PATH.
	path, err := c.lookPath(c.config.Binary)
	if err != nil {
		return "", err
	}
	return path, nil
}

// executeFastfetch runs fastfetch and parses the JSON output.
func (c *FastfetchCollector) executeFastfetch(ctx context.Context, binaryPath string) (*FastfetchData, []string) {
	var warnings []string

	// Build command arguments.
	args := []string{"--json"}

	// Add module structure argument if available in fastfetch version.
	// Modern fastfetch (>=2.0) supports --structure for selecting modules.
	if len(c.config.Modules) > 0 {
		args = append(args, "--structure", strings.Join(c.config.Modules, ":"))
	}

	c.logger.Debug("executing fastfetch",
		"binary", binaryPath,
		"args", args,
	)

	cmd := c.execCommand(ctx, binaryPath, args...)
	output, err := cmd.Output()

	if err != nil {
		// Check if it's a context timeout/cancellation.
		if errors.Is(err, context.DeadlineExceeded) {
			c.logger.Warn("fastfetch execution timed out", "timeout", c.config.Timeout)
			warnings = append(warnings, "fastfetch timed out")
			return &FastfetchData{}, warnings
		}

		// Check if it's a command execution error.
		if exitErr, ok := err.(*exec.ExitError); ok {
			c.logger.Warn("fastfetch exited with error",
				"exit_code", exitErr.ExitCode(),
				"stderr", string(exitErr.Stderr),
			)
			warnings = append(warnings, "fastfetch execution failed")
		} else {
			c.logger.Warn("fastfetch execution error", "error", err)
			warnings = append(warnings, "fastfetch error: "+err.Error())
		}

		return &FastfetchData{}, warnings
	}

	// Parse JSON output.
	data, parseWarnings := c.parseOutput(output)
	warnings = append(warnings, parseWarnings...)

	return data, warnings
}

// parseOutput parses the fastfetch JSON output.
// Fastfetch outputs a JSON array of module objects.
func (c *FastfetchCollector) parseOutput(output []byte) (*FastfetchData, []string) {
	var warnings []string

	if len(output) == 0 {
		warnings = append(warnings, "fastfetch returned empty output")
		return &FastfetchData{}, warnings
	}

	// Try parsing as an array of modules (standard format).
	var modules []FastfetchRawModule
	if err := json.Unmarshal(output, &modules); err != nil {
		// Try parsing as an object with modules array (alternate format).
		var rawOutput FastfetchRawOutput
		if err2 := json.Unmarshal(output, &rawOutput); err2 != nil {
			c.logger.Warn("failed to parse fastfetch JSON output",
				"error", err,
				"output_preview", truncateString(string(output), 200),
			)
			warnings = append(warnings, "failed to parse fastfetch output")
			return &FastfetchData{}, warnings
		}
		modules = rawOutput.Modules
	}

	if len(modules) == 0 {
		warnings = append(warnings, "fastfetch returned no modules")
		return &FastfetchData{}, warnings
	}

	// Convert raw modules to structured data.
	data := parseRawModules(modules)

	c.logger.Debug("parsed fastfetch output",
		"module_count", len(modules),
		"has_os", data.OS.Type != "",
		"has_cpu", data.CPU.Type != "",
		"has_memory", data.Memory.Type != "",
	)

	return data, warnings
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// IsAvailable checks if fastfetch is installed and accessible.
func (c *FastfetchCollector) IsAvailable() bool {
	_, err := c.findBinary()
	return err == nil
}

// Compile-time interface compliance check.
var _ collectors.Collector = (*FastfetchCollector)(nil)
