package sysmetrics

import (
	"context"
	"io"
	"strings"
	"syscall"
	"testing"
	"time"
)

// stringReadCloser wraps a strings.Reader to implement io.ReadCloser.
type stringReadCloser struct {
	*strings.Reader
}

func (s *stringReadCloser) Close() error { return nil }

func newReadCloser(content string) io.ReadCloser {
	return &stringReadCloser{strings.NewReader(content)}
}

// TestAppendAndTrim verifies the ring buffer trimming logic.
func TestAppendAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    []float64
		value    float64
		wantLen  int
		wantLast float64
	}{
		{
			name:     "empty history appends one",
			input:    nil,
			value:    42.0,
			wantLen:  1,
			wantLast: 42.0,
		},
		{
			name:     "under max keeps all",
			input:    []float64{1, 2, 3},
			value:    4.0,
			wantLen:  4,
			wantLast: 4.0,
		},
		{
			name:     "at max trims oldest",
			input:    make([]float64, MaxHistorySamples),
			value:    99.0,
			wantLen:  MaxHistorySamples,
			wantLast: 99.0,
		},
		{
			name:     "over max trims to max",
			input:    make([]float64, MaxHistorySamples+5),
			value:    77.0,
			wantLen:  MaxHistorySamples,
			wantLast: 77.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendAndTrim(tt.input, tt.value)
			if len(result) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(result), tt.wantLen)
			}
			if len(result) > 0 && result[len(result)-1] != tt.wantLast {
				t.Errorf("last = %f, want %f", result[len(result)-1], tt.wantLast)
			}
		})
	}
}

// TestReadCPU verifies CPU percentage parsing from mock /proc/stat data.
func TestReadCPU(t *testing.T) {
	// First reading seeds the counters and returns 0.
	c := NewSysMetricsCollector("", nil)
	c.openProcStat = func() (io.ReadCloser, error) {
		return newReadCloser("cpu  100 0 50 800 10 5 3 0 0 0\n"), nil
	}

	cpuPct, warn := c.readCPU()
	if warn != "" {
		t.Errorf("first read warning: %s", warn)
	}
	if cpuPct != 0 {
		t.Errorf("first read CPU = %f, want 0 (seeding)", cpuPct)
	}

	// Second reading computes delta.
	// Delta: total=+100, idle=+50 => CPU% = (1 - 50/100) * 100 = 50%
	c.openProcStat = func() (io.ReadCloser, error) {
		return newReadCloser("cpu  150 0 75 850 20 10 6 0 0 0\n"), nil
	}

	cpuPct, warn = c.readCPU()
	if warn != "" {
		t.Errorf("second read warning: %s", warn)
	}

	// Delta total = (150+0+75+850+20+10+6) - (100+0+50+800+10+5+3)
	// = 1111 - 968 = 143
	// Delta idle = 850 - 800 = 50
	// CPU% = (1 - 50/143) * 100 = 65.03...
	if cpuPct < 60 || cpuPct > 70 {
		t.Errorf("second read CPU = %f, want ~65%%", cpuPct)
	}
}

// TestReadRAM verifies RAM percentage parsing from mock /proc/meminfo data.
func TestReadRAM(t *testing.T) {
	c := NewSysMetricsCollector("", nil)
	c.openProcMeminfo = func() (io.ReadCloser, error) {
		content := `MemTotal:       16000000 kB
MemFree:         2000000 kB
MemAvailable:    4000000 kB
Buffers:          500000 kB
Cached:          3000000 kB
`
		return newReadCloser(content), nil
	}

	ramPct, warn := c.readRAM()
	if warn != "" {
		t.Errorf("warning: %s", warn)
	}

	// Used = 16000000 - 4000000 = 12000000
	// RAM% = 12000000 / 16000000 * 100 = 75%
	if ramPct != 75.0 {
		t.Errorf("RAM = %f, want 75.0", ramPct)
	}
}

// TestReadRAMEdgeCases tests edge cases for RAM parsing.
func TestReadRAMEdgeCases(t *testing.T) {
	t.Run("missing MemTotal", func(t *testing.T) {
		c := NewSysMetricsCollector("", nil)
		c.openProcMeminfo = func() (io.ReadCloser, error) {
			return newReadCloser("MemAvailable:    4000000 kB\n"), nil
		}

		_, warn := c.readRAM()
		if warn == "" {
			t.Error("expected warning for missing MemTotal")
		}
	})

	t.Run("missing MemAvailable", func(t *testing.T) {
		c := NewSysMetricsCollector("", nil)
		c.openProcMeminfo = func() (io.ReadCloser, error) {
			return newReadCloser("MemTotal:       16000000 kB\n"), nil
		}

		_, warn := c.readRAM()
		if warn == "" {
			t.Error("expected warning for missing MemAvailable")
		}
	})
}

// TestReadLoadAvg verifies load average parsing from mock /proc/loadavg.
func TestReadLoadAvg(t *testing.T) {
	c := NewSysMetricsCollector("", nil)
	c.openProcLoadavg = func() (io.ReadCloser, error) {
		return newReadCloser("1.50 2.75 3.00 1/234 5678\n"), nil
	}

	load1, load5, load15, warn := c.readLoadAvg()
	if warn != "" {
		t.Errorf("warning: %s", warn)
	}
	if load1 != 1.50 {
		t.Errorf("load1 = %f, want 1.50", load1)
	}
	if load5 != 2.75 {
		t.Errorf("load5 = %f, want 2.75", load5)
	}
	if load15 != 3.00 {
		t.Errorf("load15 = %f, want 3.00", load15)
	}
}

// TestCollectIntegration runs a full Collect cycle with mock proc files.
func TestCollectIntegration(t *testing.T) {
	c := NewSysMetricsCollector("", nil)

	// Mock all proc file readers.
	c.openProcStat = func() (io.ReadCloser, error) {
		return newReadCloser("cpu  1000 0 500 8000 100 50 30 0 0 0\n"), nil
	}
	c.openProcMeminfo = func() (io.ReadCloser, error) {
		return newReadCloser("MemTotal:       16000000 kB\nMemAvailable:    4000000 kB\n"), nil
	}
	c.openProcLoadavg = func() (io.ReadCloser, error) {
		return newReadCloser("0.50 1.00 1.50 1/100 1234\n"), nil
	}
	c.statfsFunc = func(path string, buf *syscall.Statfs_t) error {
		buf.Blocks = 1000000
		buf.Bfree = 400000
		buf.Bavail = 350000
		return nil
	}

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	if result.Collector != "sysmetrics" {
		t.Errorf("Collector = %q, want %q", result.Collector, "sysmetrics")
	}

	data, ok := result.Data.(*SysMetricsData)
	if !ok {
		t.Fatalf("Data type = %T, want *SysMetricsData", result.Data)
	}

	// First CPU read returns 0 (seeding).
	if data.CPU != 0 {
		t.Errorf("CPU = %f, want 0 (first read)", data.CPU)
	}

	// RAM should be 75%.
	if data.RAM != 75.0 {
		t.Errorf("RAM = %f, want 75.0", data.RAM)
	}

	// Disk: used = 1000000 - 400000 = 600000, total = 600000 + 350000 = 950000
	// Disk% = 600000 / 950000 * 100 = ~63.16%
	if data.Disk < 60 || data.Disk > 65 {
		t.Errorf("Disk = %f, want ~63%%", data.Disk)
	}

	// Load averages.
	if data.LoadAvg1 != 0.50 {
		t.Errorf("LoadAvg1 = %f, want 0.50", data.LoadAvg1)
	}
	if data.LoadAvg5 != 1.00 {
		t.Errorf("LoadAvg5 = %f, want 1.00", data.LoadAvg5)
	}
	if data.LoadAvg15 != 1.50 {
		t.Errorf("LoadAvg15 = %f, want 1.50", data.LoadAvg15)
	}

	// History should have exactly 1 sample after first collection.
	if len(data.CPUHistory) != 1 {
		t.Errorf("CPUHistory len = %d, want 1", len(data.CPUHistory))
	}
	if len(data.RAMHistory) != 1 {
		t.Errorf("RAMHistory len = %d, want 1", len(data.RAMHistory))
	}
	if len(data.DiskHistory) != 1 {
		t.Errorf("DiskHistory len = %d, want 1", len(data.DiskHistory))
	}
}

// TestCollectCancelled verifies that Collect respects context cancellation.
func TestCollectCancelled(t *testing.T) {
	c := NewSysMetricsCollector("", nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := c.Collect(ctx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// TestCollectorInterface verifies Name, Description, and Interval.
func TestCollectorInterface(t *testing.T) {
	c := NewSysMetricsCollector("", nil)

	if c.Name() != "sysmetrics" {
		t.Errorf("Name() = %q, want %q", c.Name(), "sysmetrics")
	}
	if c.Description() == "" {
		t.Error("Description() should not be empty")
	}
	if c.Interval() != 1*time.Minute {
		t.Errorf("Interval() = %v, want 1m", c.Interval())
	}
}

// TestReadDiskValues verifies disk percentage is within valid range.
func TestReadDiskValues(t *testing.T) {
	c := NewSysMetricsCollector("", nil)
	c.statfsFunc = func(path string, buf *syscall.Statfs_t) error {
		buf.Blocks = 1000
		buf.Bfree = 200
		buf.Bavail = 150
		return nil
	}

	diskPct, warn := c.readDisk()
	if warn != "" {
		t.Errorf("warning: %s", warn)
	}
	if diskPct < 0 || diskPct > 100 {
		t.Errorf("disk = %f, want 0-100", diskPct)
	}
}

// TestHistoryAccumulatesAcrossCollections verifies the ring buffer grows
// across multiple Collect calls.
func TestHistoryAccumulatesAcrossCollections(t *testing.T) {
	c := NewSysMetricsCollector("", nil)

	// Mock with static data.
	mockStat := func() (io.ReadCloser, error) {
		return newReadCloser("cpu  1000 0 500 8000 100 50 30 0 0 0\n"), nil
	}
	c.openProcStat = mockStat
	c.openProcMeminfo = func() (io.ReadCloser, error) {
		return newReadCloser("MemTotal:       16000000 kB\nMemAvailable:    8000000 kB\n"), nil
	}
	c.openProcLoadavg = func() (io.ReadCloser, error) {
		return newReadCloser("1.00 1.00 1.00 1/100 1234\n"), nil
	}
	c.statfsFunc = func(path string, buf *syscall.Statfs_t) error {
		buf.Blocks = 1000
		buf.Bfree = 500
		buf.Bavail = 450
		return nil
	}

	// Collect 3 times.
	for i := 0; i < 3; i++ {
		result, err := c.Collect(context.Background())
		if err != nil {
			t.Fatalf("Collect #%d error: %v", i, err)
		}

		data := result.Data.(*SysMetricsData)
		expectedLen := i + 1
		if len(data.RAMHistory) != expectedLen {
			t.Errorf("after %d collections: RAMHistory len = %d, want %d", i+1, len(data.RAMHistory), expectedLen)
		}
	}
}
