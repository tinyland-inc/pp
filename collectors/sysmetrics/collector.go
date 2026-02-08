package sysmetrics

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

const (
	// collectorName is the unique identifier for this collector.
	collectorName = "sysmetrics"

	// collectorDescription describes what this collector gathers.
	collectorDescription = "Local system metrics (CPU, RAM, Disk, Load Average)"

	// defaultInterval is the recommended polling interval.
	// One minute gives good resolution for the 60-sample ring buffer.
	defaultInterval = 1 * time.Minute
)

// SysMetricsCollector implements collectors.Collector for local system metrics.
// It reads from /proc on Linux to collect CPU, RAM, Disk, and Load Average.
// Historical values are maintained in ring buffers that survive daemon restarts
// by loading previous data from the cache.
type SysMetricsCollector struct {
	logger *slog.Logger

	// cacheDir is the path to the prompt-pulse cache directory.
	// Used to load previous history on startup.
	cacheDir string

	// prevIdle and prevTotal track the last CPU sample for delta computation.
	prevIdle  uint64
	prevTotal uint64
	firstRun  bool

	// Internal ring buffers maintained across Collect calls.
	cpuHistory  []float64
	ramHistory  []float64
	diskHistory []float64

	// Overridable file openers for testing.
	openProcStat    func() (io.ReadCloser, error)
	openProcMeminfo func() (io.ReadCloser, error)
	openProcLoadavg func() (io.ReadCloser, error)
	statfsFunc      func(path string, buf *syscall.Statfs_t) error
}

// NewSysMetricsCollector creates a SysMetricsCollector.
// If logger is nil, a no-op logger is used.
func NewSysMetricsCollector(cacheDir string, logger *slog.Logger) *SysMetricsCollector {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &SysMetricsCollector{
		logger:   logger,
		cacheDir: cacheDir,
		firstRun: true,
		openProcStat: func() (io.ReadCloser, error) {
			return os.Open("/proc/stat")
		},
		openProcMeminfo: func() (io.ReadCloser, error) {
			return os.Open("/proc/meminfo")
		},
		openProcLoadavg: func() (io.ReadCloser, error) {
			return os.Open("/proc/loadavg")
		},
		statfsFunc: syscall.Statfs,
	}
}

// Name returns the collector's unique identifier.
func (c *SysMetricsCollector) Name() string {
	return collectorName
}

// Description returns a human-readable description of what this collector gathers.
func (c *SysMetricsCollector) Description() string {
	return collectorDescription
}

// Interval returns the recommended polling interval for this collector.
func (c *SysMetricsCollector) Interval() time.Duration {
	return defaultInterval
}

// Collect gathers CPU, RAM, Disk, and Load Average metrics.
// On the first run it also loads previous history from the cache to maintain
// ring buffer continuity across daemon restarts.
func (c *SysMetricsCollector) Collect(ctx context.Context) (*collectors.CollectResult, error) {
	// Check for context cancellation before starting.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var warnings []string

	// Load previous history from cache on first run to recover
	// ring buffer state across daemon restarts.
	if c.firstRun {
		if prevData := c.loadPreviousData(); prevData != nil {
			c.cpuHistory = prevData.CPUHistory
			c.ramHistory = prevData.RAMHistory
			c.diskHistory = prevData.DiskHistory
		}
		c.firstRun = false
	}

	data := &SysMetricsData{}

	// Read CPU usage from /proc/stat.
	cpuPct, cpuWarn := c.readCPU()
	if cpuWarn != "" {
		warnings = append(warnings, cpuWarn)
	}
	data.CPU = cpuPct

	// Read RAM usage from /proc/meminfo.
	ramPct, ramWarn := c.readRAM()
	if ramWarn != "" {
		warnings = append(warnings, ramWarn)
	}
	data.RAM = ramPct

	// Read Disk usage via statfs.
	diskPct, diskWarn := c.readDisk()
	if diskWarn != "" {
		warnings = append(warnings, diskWarn)
	}
	data.Disk = diskPct

	// Read Load Average from /proc/loadavg.
	load1, load5, load15, loadWarn := c.readLoadAvg()
	if loadWarn != "" {
		warnings = append(warnings, loadWarn)
	}
	data.LoadAvg1 = load1
	data.LoadAvg5 = load5
	data.LoadAvg15 = load15

	// Append current samples to internal ring buffers and copy to result.
	c.cpuHistory = appendAndTrim(c.cpuHistory, data.CPU)
	c.ramHistory = appendAndTrim(c.ramHistory, data.RAM)
	c.diskHistory = appendAndTrim(c.diskHistory, data.Disk)

	data.CPUHistory = make([]float64, len(c.cpuHistory))
	copy(data.CPUHistory, c.cpuHistory)
	data.RAMHistory = make([]float64, len(c.ramHistory))
	copy(data.RAMHistory, c.ramHistory)
	data.DiskHistory = make([]float64, len(c.diskHistory))
	copy(data.DiskHistory, c.diskHistory)

	c.logger.Debug("sysmetrics collected",
		"cpu", fmt.Sprintf("%.1f%%", data.CPU),
		"ram", fmt.Sprintf("%.1f%%", data.RAM),
		"disk", fmt.Sprintf("%.1f%%", data.Disk),
		"load", fmt.Sprintf("%.2f %.2f %.2f", data.LoadAvg1, data.LoadAvg5, data.LoadAvg15),
		"history_len", len(data.CPUHistory),
	)

	return &collectors.CollectResult{
		Collector: collectorName,
		Timestamp: time.Now(),
		Data:      data,
		Warnings:  warnings,
	}, nil
}

// loadPreviousData reads the sysmetrics cache file to recover ring buffer history.
func (c *SysMetricsCollector) loadPreviousData() *SysMetricsData {
	if c.cacheDir == "" {
		return nil
	}

	path := c.cacheDir + "/sysmetrics.json"
	raw, err := os.ReadFile(path)
	if err != nil {
		c.logger.Debug("no previous sysmetrics cache", "error", err)
		return nil
	}

	var data SysMetricsData
	if err := json.Unmarshal(raw, &data); err != nil {
		c.logger.Warn("failed to parse previous sysmetrics cache", "error", err)
		return nil
	}

	c.logger.Debug("loaded previous sysmetrics history",
		"cpu_samples", len(data.CPUHistory),
		"ram_samples", len(data.RAMHistory),
		"disk_samples", len(data.DiskHistory),
	)

	return &data
}

// readCPU reads /proc/stat to compute CPU usage as a percentage.
// It calculates the delta between the current and previous readings.
// On the first call it returns 0 and seeds the counters.
func (c *SysMetricsCollector) readCPU() (float64, string) {
	f, err := c.openProcStat()
	if err != nil {
		return 0, fmt.Sprintf("sysmetrics: open /proc/stat: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, "sysmetrics: /proc/stat cpu line too short"
		}

		// Fields: cpu user nice system idle iowait irq softirq steal ...
		var total uint64
		var idle uint64
		for i := 1; i < len(fields); i++ {
			val, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				return 0, fmt.Sprintf("sysmetrics: parse /proc/stat field %d: %v", i, err)
			}
			total += val
			if i == 4 { // idle field
				idle = val
			}
		}

		// First reading: seed counters, return 0.
		if c.prevTotal == 0 {
			c.prevIdle = idle
			c.prevTotal = total
			return 0, ""
		}

		// Compute delta.
		deltaTotal := total - c.prevTotal
		deltaIdle := idle - c.prevIdle

		c.prevIdle = idle
		c.prevTotal = total

		if deltaTotal == 0 {
			return 0, ""
		}

		cpuPct := (1.0 - float64(deltaIdle)/float64(deltaTotal)) * 100.0
		if cpuPct < 0 {
			cpuPct = 0
		}
		if cpuPct > 100 {
			cpuPct = 100
		}

		return cpuPct, ""
	}

	return 0, "sysmetrics: cpu line not found in /proc/stat"
}

// readRAM reads /proc/meminfo to compute RAM usage as a percentage.
// Usage = (MemTotal - MemAvailable) / MemTotal * 100
func (c *SysMetricsCollector) readRAM() (float64, string) {
	f, err := c.openProcMeminfo()
	if err != nil {
		return 0, fmt.Sprintf("sysmetrics: open /proc/meminfo: %v", err)
	}
	defer f.Close()

	var memTotal, memAvailable uint64
	var foundTotal, foundAvailable bool

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "MemTotal:") {
			val, err := parseMemInfoLine(line)
			if err != nil {
				return 0, fmt.Sprintf("sysmetrics: parse MemTotal: %v", err)
			}
			memTotal = val
			foundTotal = true
		} else if strings.HasPrefix(line, "MemAvailable:") {
			val, err := parseMemInfoLine(line)
			if err != nil {
				return 0, fmt.Sprintf("sysmetrics: parse MemAvailable: %v", err)
			}
			memAvailable = val
			foundAvailable = true
		}

		if foundTotal && foundAvailable {
			break
		}
	}

	if !foundTotal {
		return 0, "sysmetrics: MemTotal not found in /proc/meminfo"
	}
	if !foundAvailable {
		return 0, "sysmetrics: MemAvailable not found in /proc/meminfo"
	}
	if memTotal == 0 {
		return 0, "sysmetrics: MemTotal is zero"
	}

	used := memTotal - memAvailable
	ramPct := float64(used) / float64(memTotal) * 100.0
	if ramPct < 0 {
		ramPct = 0
	}
	if ramPct > 100 {
		ramPct = 100
	}

	return ramPct, ""
}

// parseMemInfoLine extracts the numeric kB value from a /proc/meminfo line.
// Format: "MemTotal:       16384000 kB"
func parseMemInfoLine(line string) (uint64, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, fmt.Errorf("too few fields: %q", line)
	}
	return strconv.ParseUint(fields[1], 10, 64)
}

// readDisk uses statfs to compute root filesystem usage as a percentage.
func (c *SysMetricsCollector) readDisk() (float64, string) {
	var stat syscall.Statfs_t
	if err := c.statfsFunc("/", &stat); err != nil {
		return 0, fmt.Sprintf("sysmetrics: statfs /: %v", err)
	}

	if stat.Blocks == 0 {
		return 0, "sysmetrics: filesystem reports zero blocks"
	}

	// Available blocks for non-root users.
	used := stat.Blocks - stat.Bfree
	total := stat.Blocks - stat.Bfree + stat.Bavail

	if total == 0 {
		return 0, ""
	}

	diskPct := float64(used) / float64(total) * 100.0
	if diskPct < 0 {
		diskPct = 0
	}
	if diskPct > 100 {
		diskPct = 100
	}

	return diskPct, ""
}

// readLoadAvg reads /proc/loadavg to get 1, 5, and 15 minute load averages.
func (c *SysMetricsCollector) readLoadAvg() (float64, float64, float64, string) {
	f, err := c.openProcLoadavg()
	if err != nil {
		return 0, 0, 0, fmt.Sprintf("sysmetrics: open /proc/loadavg: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, 0, 0, "sysmetrics: /proc/loadavg is empty"
	}

	line := scanner.Text()
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 0, 0, 0, "sysmetrics: /proc/loadavg too few fields"
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, fmt.Sprintf("sysmetrics: parse load1: %v", err)
	}

	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, fmt.Sprintf("sysmetrics: parse load5: %v", err)
	}

	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, fmt.Sprintf("sysmetrics: parse load15: %v", err)
	}

	return load1, load5, load15, ""
}

// Compile-time interface compliance check.
var _ collectors.Collector = (*SysMetricsCollector)(nil)
