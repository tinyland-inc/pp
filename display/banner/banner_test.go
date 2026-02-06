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

	// InfraPanel widget renders tree-structured output with multiple lines.
	// Should have at least 2 lines (header + node info).
	if len(result) < 2 {
		t.Errorf("Expected at least 2 lines from InfraPanel, got %d: %v", len(result), result)
	}

	// Join all lines and check for expected content across the full output.
	allOutput := strings.Join(result, "\n")

	// Should contain node hostname
	if !strings.Contains(allOutput, "honey") {
		t.Errorf("Expected hostname 'honey' in output, got: %s", allOutput)
	}

	// Should contain online count
	if !strings.Contains(allOutput, "online") {
		t.Errorf("Expected 'online' in output, got: %s", allOutput)
	}

	// Should contain gauge characters (█ or ░) from InfraPanel mini gauges
	hasGaugeChars := strings.ContainsAny(allOutput, "█░")
	if !hasGaugeChars {
		t.Errorf("Expected gauge characters in InfraPanel output, got: %s", allOutput)
	}

	// Should contain metric labels
	if !strings.Contains(allOutput, "CPU") {
		t.Errorf("Expected CPU metric in output, got: %s", allOutput)
	}
	if !strings.Contains(allOutput, "RAM") {
		t.Errorf("Expected RAM metric in output, got: %s", allOutput)
	}
	if !strings.Contains(allOutput, "Disk") {
		t.Errorf("Expected Disk metric in output, got: %s", allOutput)
	}
}
