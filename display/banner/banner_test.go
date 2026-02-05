package banner

import (
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
