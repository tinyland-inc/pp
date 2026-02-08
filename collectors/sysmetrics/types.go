// Package sysmetrics provides a local system metrics collector for prompt-pulse.
// It reads CPU, RAM, Disk, and Load Average from /proc (Linux) and maintains
// a circular buffer of historical samples for sparkline rendering.
package sysmetrics

// SysMetricsData holds current system metrics plus history ring buffers.
type SysMetricsData struct {
	// CPU is the current CPU usage percentage (0-100).
	CPU float64 `json:"cpu"`

	// RAM is the current RAM usage percentage (0-100).
	RAM float64 `json:"ram"`

	// Disk is the current root filesystem usage percentage (0-100).
	Disk float64 `json:"disk"`

	// LoadAvg1 is the 1-minute load average.
	LoadAvg1 float64 `json:"load_avg_1"`

	// LoadAvg5 is the 5-minute load average.
	LoadAvg5 float64 `json:"load_avg_5"`

	// LoadAvg15 is the 15-minute load average.
	LoadAvg15 float64 `json:"load_avg_15"`

	// CPUHistory is a ring buffer of CPU usage samples, max MaxHistorySamples.
	CPUHistory []float64 `json:"cpu_history"`

	// RAMHistory is a ring buffer of RAM usage samples, max MaxHistorySamples.
	RAMHistory []float64 `json:"ram_history"`

	// DiskHistory is a ring buffer of Disk usage samples, max MaxHistorySamples.
	DiskHistory []float64 `json:"disk_history"`
}

// MaxHistorySamples is the maximum number of historical samples retained
// in each ring buffer. At a 1-minute collection interval this covers 1 hour.
const MaxHistorySamples = 60

// appendAndTrim appends a value to a history slice and trims it to
// MaxHistorySamples, discarding the oldest entries.
func appendAndTrim(history []float64, value float64) []float64 {
	history = append(history, value)
	if len(history) > MaxHistorySamples {
		history = history[len(history)-MaxHistorySamples:]
	}
	return history
}
