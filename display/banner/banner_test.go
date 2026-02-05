package banner

import (
	"strings"
	"testing"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

func TestFormatFastfetchForSection_Nil(t *testing.T) {
	b := &Banner{}
	result := b.formatFastfetchForSection(nil)
	if len(result) != 1 || result[0] != "(no data)" {
		t.Errorf("Expected [(no data)], got %v", result)
	}
}

func TestFormatFastfetchForSection_Empty(t *testing.T) {
	b := &Banner{}
	data := &collectors.FastfetchData{}
	result := b.formatFastfetchForSection(data)
	if len(result) != 1 || result[0] != "(no data)" {
		t.Errorf("Expected [(no data)] for empty data, got %v", result)
	}
}

func TestFormatFastfetchForSection_WithData(t *testing.T) {
	b := &Banner{}
	data := &collectors.FastfetchData{
		OS: collectors.FastfetchModule{
			Type:   "OS",
			Result: "Rocky Linux 10.1",
		},
		Kernel: collectors.FastfetchModule{
			Type:   "Kernel",
			Result: "6.12.0",
		},
		CPU: collectors.FastfetchModule{
			Type:   "CPU",
			Result: "Intel i7-8550U",
		},
	}
	result := b.formatFastfetchForSection(data)

	// Should have multiple lines with system info
	if len(result) < 3 {
		t.Errorf("Expected at least 3 lines, got %d", len(result))
	}

	// Check that FormatCompact was called and returned data
	found := false
	for _, line := range result {
		if line != "(no data)" && line != "" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected formatted system info, got %v", result)
	}
}

func TestFormatInfraForSection_NoNodeMetrics(t *testing.T) {
	b := &Banner{}
	cpu := 45.0
	ram := 67.0
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			OnlineCount: 1,
			TotalCount:  2,
			Nodes: []collectors.TailscaleNode{
				{
					Hostname:   "honey",
					Online:     true,
					CPUPercent: &cpu,
					RAMPercent: &ram,
				},
			},
		},
	}

	result := b.formatInfraForSection(data, false)

	// Should have summary but NOT node metrics when showNodeMetrics=false
	if len(result) != 1 {
		t.Errorf("Expected 1 line (summary only), got %d: %v", len(result), result)
	}
	if !strings.Contains(result[0], "1/2 online") {
		t.Errorf("Expected summary line, got: %s", result[0])
	}
}

func TestFormatInfraForSection_WithNodeMetrics(t *testing.T) {
	b := &Banner{}
	cpu := 45.0
	ram := 67.0
	disk := 32.0
	data := &collectors.InfraStatus{
		Tailscale: &collectors.TailscaleStatus{
			OnlineCount: 1,
			TotalCount:  2,
			Nodes: []collectors.TailscaleNode{
				{
					Hostname:    "honey",
					Online:      true,
					CPUPercent:  &cpu,
					RAMPercent:  &ram,
					DiskPercent: &disk,
				},
			},
		},
	}

	result := b.formatInfraForSection(data, true)

	// Should have summary + node metrics
	if len(result) != 2 {
		t.Errorf("Expected 2 lines (summary + node), got %d: %v", len(result), result)
	}

	// First line should be summary
	if !strings.Contains(result[0], "1/2 online") {
		t.Errorf("Expected summary line, got: %s", result[0])
	}

	// Second line should have node metrics with gauges
	nodeLine := result[1]
	if !strings.Contains(nodeLine, "honey") {
		t.Errorf("Expected hostname in node line, got: %s", nodeLine)
	}

	// Check for gauge characters (█ or ░)
	hasGaugeChars := strings.ContainsAny(nodeLine, "█░")
	if !hasGaugeChars {
		t.Errorf("Expected gauge characters in node metrics, got: %s", nodeLine)
	}

	// Should contain all three metrics
	if !strings.Contains(nodeLine, "CPU") {
		t.Errorf("Expected CPU metric, got: %s", nodeLine)
	}
	if !strings.Contains(nodeLine, "RAM") {
		t.Errorf("Expected RAM metric, got: %s", nodeLine)
	}
	if !strings.Contains(nodeLine, "Disk") {
		t.Errorf("Expected Disk metric, got: %s", nodeLine)
	}

	// Should contain percentage values
	if !strings.Contains(nodeLine, "45%") {
		t.Errorf("Expected CPU percentage, got: %s", nodeLine)
	}
	if !strings.Contains(nodeLine, "67%") {
		t.Errorf("Expected RAM percentage, got: %s", nodeLine)
	}
	if !strings.Contains(nodeLine, "32%") {
		t.Errorf("Expected Disk percentage, got: %s", nodeLine)
	}
}
