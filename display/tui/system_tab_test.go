package tui

import (
	"strings"
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

func TestRenderSystemContent_BothNil(t *testing.T) {
	result := renderSystemContent(nil, nil, 80, 24)
	if !strings.Contains(result, "No system data available") {
		t.Errorf("expected placeholder for nil data, got: %s", result)
	}
}

func TestRenderSystemContent_EmptyFastfetchNilMetrics(t *testing.T) {
	// Empty fastfetch (all modules blank) and nil metrics should show placeholder.
	ff := &collectors.FastfetchData{}
	result := renderSystemContent(ff, nil, 80, 24)
	if !strings.Contains(result, "No system data available") {
		t.Errorf("expected placeholder for empty fastfetch + nil metrics, got: %s", result)
	}
}

func TestRenderSystemContent_FastfetchOnly(t *testing.T) {
	ff := &collectors.FastfetchData{
		OS:     collectors.FastfetchModule{Type: "OS", Result: "Rocky Linux 10.1"},
		Kernel: collectors.FastfetchModule{Type: "Kernel", Result: "6.12.0"},
		Uptime: collectors.FastfetchModule{Type: "Uptime", Result: "3 days"},
		CPU:    collectors.FastfetchModule{Type: "CPU", Result: "AMD Ryzen 9 (16)"},
		Memory: collectors.FastfetchModule{Type: "Memory", Result: "16.00 GiB / 32.00 GiB"},
	}

	result := renderSystemContent(ff, nil, 100, 24)

	if !strings.Contains(result, "System Information") {
		t.Error("expected 'System Information' title")
	}
	if !strings.Contains(result, "Rocky Linux 10.1") {
		t.Error("expected OS value in output")
	}
	if !strings.Contains(result, "6.12.0") {
		t.Error("expected Kernel value in output")
	}
	// Should NOT contain system metrics section.
	if strings.Contains(result, "System Metrics") {
		t.Error("expected no 'System Metrics' section when metrics are nil")
	}
}

func TestRenderSystemContent_MetricsOnly(t *testing.T) {
	metrics := &collectors.SysMetricsData{
		CPU:       45.2,
		RAM:       62.8,
		Disk:      33.5,
		LoadAvg1:  1.23,
		LoadAvg5:  2.45,
		LoadAvg15: 3.67,
	}

	result := renderSystemContent(nil, metrics, 100, 24)

	// Should NOT contain fastfetch section.
	if strings.Contains(result, "System Information") {
		t.Error("expected no 'System Information' section when fastfetch is nil")
	}

	// Should contain metrics section.
	if !strings.Contains(result, "System Metrics") {
		t.Error("expected 'System Metrics' title")
	}
	if !strings.Contains(result, "CPU") {
		t.Error("expected 'CPU' label")
	}
	if !strings.Contains(result, "45.2%") {
		t.Error("expected CPU value '45.2%'")
	}
	if !strings.Contains(result, "RAM") {
		t.Error("expected 'RAM' label")
	}
	if !strings.Contains(result, "62.8%") {
		t.Error("expected RAM value '62.8%'")
	}
	if !strings.Contains(result, "Disk") {
		t.Error("expected 'Disk' label")
	}
	if !strings.Contains(result, "33.5%") {
		t.Error("expected Disk value '33.5%'")
	}
}

func TestRenderSystemContent_BothPresent(t *testing.T) {
	ff := &collectors.FastfetchData{
		OS:     collectors.FastfetchModule{Type: "OS", Result: "Rocky Linux 10.1"},
		Kernel: collectors.FastfetchModule{Type: "Kernel", Result: "6.12.0"},
		CPU:    collectors.FastfetchModule{Type: "CPU", Result: "AMD Ryzen 9"},
		Memory: collectors.FastfetchModule{Type: "Memory", Result: "16 GiB / 32 GiB"},
	}
	metrics := &collectors.SysMetricsData{
		CPU:       75.0,
		RAM:       80.5,
		Disk:      55.0,
		LoadAvg1:  4.00,
		LoadAvg5:  3.50,
		LoadAvg15: 2.00,
	}

	result := renderSystemContent(ff, metrics, 120, 24)

	// Both sections should be present.
	if !strings.Contains(result, "System Information") {
		t.Error("expected 'System Information' title")
	}
	if !strings.Contains(result, "System Metrics") {
		t.Error("expected 'System Metrics' title")
	}

	// Fastfetch data.
	if !strings.Contains(result, "Rocky Linux 10.1") {
		t.Error("expected OS in output")
	}

	// Metrics data.
	if !strings.Contains(result, "75.0%") {
		t.Error("expected CPU value '75.0%'")
	}
	if !strings.Contains(result, "80.5%") {
		t.Error("expected RAM value '80.5%'")
	}

	// Separator between sections (box-drawing horizontal line).
	if !strings.Contains(result, "\u2500") {
		t.Error("expected separator between fastfetch and metrics sections")
	}
}

func TestRenderSystemContent_LoadAverage(t *testing.T) {
	metrics := &collectors.SysMetricsData{
		CPU:       10.0,
		RAM:       20.0,
		Disk:      30.0,
		LoadAvg1:  1.23,
		LoadAvg5:  2.45,
		LoadAvg15: 3.67,
	}

	result := renderSystemContent(nil, metrics, 100, 24)

	if !strings.Contains(result, "Load:") {
		t.Error("expected 'Load:' label")
	}
	if !strings.Contains(result, "1.23") {
		t.Error("expected 1-minute load average '1.23'")
	}
	if !strings.Contains(result, "2.45") {
		t.Error("expected 5-minute load average '2.45'")
	}
	if !strings.Contains(result, "3.67") {
		t.Error("expected 15-minute load average '3.67'")
	}
}

func TestRenderSystemContent_WithSparklineHistory(t *testing.T) {
	metrics := &collectors.SysMetricsData{
		CPU:        45.0,
		RAM:        60.0,
		Disk:       70.0,
		LoadAvg1:   1.0,
		LoadAvg5:   1.0,
		LoadAvg15:  1.0,
		CPUHistory: []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 45},
		RAMHistory: []float64{50, 55, 60, 58, 62, 60},
	}

	result := renderSystemContent(nil, metrics, 100, 24)

	// Sparkline characters should be present for metrics with history.
	sparkChars := "▁▂▃▄▅▆▇█"
	hasSparkChar := false
	for _, r := range result {
		if strings.ContainsRune(sparkChars, r) {
			hasSparkChar = true
			break
		}
	}
	if !hasSparkChar {
		t.Error("expected sparkline block characters in output for metrics with history")
	}
}

func TestRenderSystemContent_EmptyHistory(t *testing.T) {
	metrics := &collectors.SysMetricsData{
		CPU:       45.0,
		RAM:       60.0,
		Disk:      70.0,
		LoadAvg1:  1.0,
		LoadAvg5:  1.0,
		LoadAvg15: 1.0,
		// No history data: all history slices are nil.
	}

	result := renderSystemContent(nil, metrics, 100, 24)

	// Should still render metrics without sparklines (placeholder dashes).
	if !strings.Contains(result, "CPU") {
		t.Error("expected 'CPU' label even without history")
	}
	if !strings.Contains(result, "45.0%") {
		t.Error("expected CPU value '45.0%' even without history")
	}
	// Should contain horizontal rule characters as placeholder.
	if !strings.Contains(result, "\u2500") {
		t.Error("expected dash placeholder when history is empty")
	}
}

func TestRenderSystemContent_NarrowWidth(t *testing.T) {
	metrics := &collectors.SysMetricsData{
		CPU:        90.0,
		RAM:        95.0,
		Disk:       88.0,
		LoadAvg1:   8.0,
		LoadAvg5:   7.5,
		LoadAvg15:  6.0,
		CPUHistory: []float64{80, 85, 90, 92, 90},
	}

	// Should not panic or produce garbled output at narrow widths.
	result := renderSystemContent(nil, metrics, 50, 24)

	if !strings.Contains(result, "CPU") {
		t.Error("expected 'CPU' label at narrow width")
	}
	if !strings.Contains(result, "90.0%") {
		t.Error("expected CPU value at narrow width")
	}
}

func TestMetricColorForValue(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{0, "#22C55E"},   // green
		{50, "#22C55E"},  // green
		{69.9, "#22C55E"}, // green
		{70, "#EAB308"},  // yellow
		{89.9, "#EAB308"}, // yellow
		{90, "#EF4444"},  // red
		{100, "#EF4444"}, // red
	}

	for _, tt := range tests {
		color := metricColorForValue(tt.value)
		if string(color) != tt.want {
			t.Errorf("metricColorForValue(%.1f) = %s, want %s", tt.value, string(color), tt.want)
		}
	}
}

func TestRenderSystemContent_OptionalFastfetchModules(t *testing.T) {
	ff := &collectors.FastfetchData{
		OS:      collectors.FastfetchModule{Type: "OS", Result: "Rocky Linux 10.1"},
		CPU:     collectors.FastfetchModule{Type: "CPU", Result: "AMD Ryzen 9"},
		Memory:  collectors.FastfetchModule{Type: "Memory", Result: "16 GiB / 32 GiB"},
		Battery: collectors.FastfetchModule{Type: "Battery", Result: "95% [Charging]"},
		WM:      collectors.FastfetchModule{Type: "WM", Result: "sway"},
	}

	result := renderSystemContent(ff, nil, 100, 24)

	if !strings.Contains(result, "Battery:") {
		t.Error("expected optional 'Battery' module in output")
	}
	if !strings.Contains(result, "95% [Charging]") {
		t.Error("expected battery value in output")
	}
	if !strings.Contains(result, "WM:") {
		t.Error("expected optional 'WM' module in output")
	}
}

func TestRenderFastfetchSection_SkipsEmptyModules(t *testing.T) {
	ff := &collectors.FastfetchData{
		OS:     collectors.FastfetchModule{Type: "OS", Result: "Rocky Linux 10.1"},
		Host:   collectors.FastfetchModule{}, // empty, should be skipped
		Kernel: collectors.FastfetchModule{Type: "Kernel", Result: "6.12.0"},
		GPU:    collectors.FastfetchModule{}, // empty, should be skipped
	}

	lines := renderFastfetchSection(ff, 100)
	output := strings.Join(lines, "\n")

	if !strings.Contains(output, "OS:") {
		t.Error("expected 'OS:' label")
	}
	if !strings.Contains(output, "Kernel:") {
		t.Error("expected 'Kernel:' label")
	}
	// Host and GPU have empty Type/Result, so should not appear.
	if strings.Contains(output, "Host:") {
		t.Error("expected empty Host module to be skipped")
	}
	if strings.Contains(output, "GPU:") {
		t.Error("expected empty GPU module to be skipped")
	}
}

func TestRenderSysMetricsSection_AllFields(t *testing.T) {
	data := &collectors.SysMetricsData{
		CPU:         42.5,
		RAM:         71.3,
		Disk:        92.1,
		LoadAvg1:    2.50,
		LoadAvg5:    1.75,
		LoadAvg15:   1.25,
		CPUHistory:  []float64{30, 35, 40, 42, 45, 42},
		RAMHistory:  []float64{65, 68, 70, 71, 71},
		DiskHistory: []float64{91, 91, 92, 92, 92},
	}

	lines := renderSysMetricsSection(data, 120)
	output := strings.Join(lines, "\n")

	// Check all metric labels present.
	for _, label := range []string{"CPU", "RAM", "Disk"} {
		if !strings.Contains(output, label) {
			t.Errorf("expected '%s' label in sysmetrics section", label)
		}
	}

	// Check values.
	if !strings.Contains(output, "42.5%") {
		t.Error("expected CPU value '42.5%'")
	}
	if !strings.Contains(output, "71.3%") {
		t.Error("expected RAM value '71.3%'")
	}
	if !strings.Contains(output, "92.1%") {
		t.Error("expected Disk value '92.1%'")
	}

	// Check load average.
	if !strings.Contains(output, "2.50") {
		t.Error("expected load average 1-minute value '2.50'")
	}
	if !strings.Contains(output, "1.75") {
		t.Error("expected load average 5-minute value '1.75'")
	}
	if !strings.Contains(output, "1.25") {
		t.Error("expected load average 15-minute value '1.25'")
	}

	// Check sparkline characters are present (from history data).
	sparkChars := "▁▂▃▄▅▆▇█"
	hasSparkChar := false
	for _, r := range output {
		if strings.ContainsRune(sparkChars, r) {
			hasSparkChar = true
			break
		}
	}
	if !hasSparkChar {
		t.Error("expected sparkline characters in sysmetrics output with history")
	}

	// Check gauge characters are present.
	if !strings.Contains(output, "█") || !strings.Contains(output, "░") {
		t.Error("expected gauge bar characters in sysmetrics output")
	}
}
