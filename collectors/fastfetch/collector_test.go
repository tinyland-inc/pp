package fastfetch

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// TestFastfetchCollector_Name verifies the collector's name.
func TestFastfetchCollector_Name(t *testing.T) {
	c := NewFastfetchCollector(DefaultConfig(), nil)
	if got := c.Name(); got != "fastfetch" {
		t.Errorf("Name() = %q, want %q", got, "fastfetch")
	}
}

// TestFastfetchCollector_Description verifies the collector's description.
func TestFastfetchCollector_Description(t *testing.T) {
	c := NewFastfetchCollector(DefaultConfig(), nil)
	if got := c.Description(); got == "" {
		t.Error("Description() returned empty string")
	}
}

// TestFastfetchCollector_Interval verifies the polling interval.
func TestFastfetchCollector_Interval(t *testing.T) {
	c := NewFastfetchCollector(DefaultConfig(), nil)
	if got := c.Interval(); got != 10*time.Minute {
		t.Errorf("Interval() = %v, want %v", got, 10*time.Minute)
	}
}

// TestFastfetchCollector_Collect_BinaryNotFound tests graceful fallback.
func TestFastfetchCollector_Collect_BinaryNotFound(t *testing.T) {
	c := NewFastfetchCollector(DefaultConfig(), nil)

	// Mock lookPath to always fail.
	c.lookPath = func(string) (string, error) {
		return "", exec.ErrNotFound
	}

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("Collect() returned nil result")
	}

	// Should have warning about binary not found.
	if len(result.Warnings) == 0 {
		t.Error("expected warning about fastfetch not installed")
	}

	// Data should be empty but not nil.
	data, ok := result.Data.(*FastfetchData)
	if !ok {
		t.Fatalf("Data type = %T, want *FastfetchData", result.Data)
	}
	if !data.IsEmpty() {
		t.Error("expected empty FastfetchData when binary not found")
	}
}

// TestFastfetchCollector_Collect_ContextCancelled tests context cancellation.
func TestFastfetchCollector_Collect_ContextCancelled(t *testing.T) {
	c := NewFastfetchCollector(DefaultConfig(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := c.Collect(ctx)
	if err == nil {
		t.Error("Collect() should return error for cancelled context")
	}
}

// TestFastfetchCollector_parseOutput_ValidJSON tests JSON parsing.
func TestFastfetchCollector_parseOutput_ValidJSON(t *testing.T) {
	c := NewFastfetchCollector(DefaultConfig(), nil)

	// Sample fastfetch JSON output (array format).
	jsonData := `[
		{"type": "OS", "key": "OS", "result": "Rocky Linux 10.1"},
		{"type": "Host", "key": "Host", "result": "Lenovo ThinkPad X1"},
		{"type": "Kernel", "key": "Kernel", "result": "6.12.0-124.29.1.el10_1.x86_64"},
		{"type": "Uptime", "key": "Uptime", "result": "5 hours, 13 mins"},
		{"type": "CPU", "key": "CPU", "result": "Intel i7-8550U (8) @ 4.0GHz"},
		{"type": "Memory", "key": "Memory", "result": "4.5 GiB / 15.4 GiB (29%)"},
		{"type": "Disk", "key": "Disk", "result": "45 GiB / 230 GiB (20%)"}
	]`

	data, warnings := c.parseOutput([]byte(jsonData))

	if len(warnings) > 0 {
		t.Errorf("parseOutput() warnings = %v, want none", warnings)
	}

	if data.OS.Result != "Rocky Linux 10.1" {
		t.Errorf("OS.Result = %q, want %q", data.OS.Result, "Rocky Linux 10.1")
	}

	if data.Kernel.Result != "6.12.0-124.29.1.el10_1.x86_64" {
		t.Errorf("Kernel.Result = %q, want %q", data.Kernel.Result, "6.12.0-124.29.1.el10_1.x86_64")
	}

	if data.CPU.Result != "Intel i7-8550U (8) @ 4.0GHz" {
		t.Errorf("CPU.Result = %q, want %q", data.CPU.Result, "Intel i7-8550U (8) @ 4.0GHz")
	}
}

// TestFastfetchCollector_parseOutput_EmptyOutput tests empty output handling.
func TestFastfetchCollector_parseOutput_EmptyOutput(t *testing.T) {
	c := NewFastfetchCollector(DefaultConfig(), nil)

	data, warnings := c.parseOutput([]byte{})

	if len(warnings) == 0 {
		t.Error("expected warning for empty output")
	}

	if !data.IsEmpty() {
		t.Error("expected empty FastfetchData for empty output")
	}
}

// TestFastfetchCollector_parseOutput_InvalidJSON tests invalid JSON handling.
func TestFastfetchCollector_parseOutput_InvalidJSON(t *testing.T) {
	c := NewFastfetchCollector(DefaultConfig(), nil)

	data, warnings := c.parseOutput([]byte("not valid json"))

	if len(warnings) == 0 {
		t.Error("expected warning for invalid JSON")
	}

	if !data.IsEmpty() {
		t.Error("expected empty FastfetchData for invalid JSON")
	}
}

// TestFastfetchData_FormatForDisplay tests display formatting.
func TestFastfetchData_FormatForDisplay(t *testing.T) {
	data := &FastfetchData{
		OS:     FastfetchModule{Type: "OS", Result: "Rocky Linux 10.1"},
		CPU:    FastfetchModule{Type: "CPU", Result: "Intel i7-8550U"},
		Memory: FastfetchModule{Type: "Memory", Result: "4.5 GiB / 15.4 GiB"},
	}

	lines := data.FormatForDisplay()

	if len(lines) < 3 {
		t.Errorf("FormatForDisplay() returned %d lines, want at least 3", len(lines))
	}

	// Verify first line is OS.
	if lines[0] != "OS: Rocky Linux 10.1" {
		t.Errorf("first line = %q, want %q", lines[0], "OS: Rocky Linux 10.1")
	}
}

// TestFastfetchData_FormatCompact tests compact formatting.
func TestFastfetchData_FormatCompact(t *testing.T) {
	data := &FastfetchData{
		OS:     FastfetchModule{Type: "OS", Result: "Rocky Linux 10.1"},
		Kernel: FastfetchModule{Type: "Kernel", Result: "6.12.0"},
		CPU:    FastfetchModule{Type: "CPU", Result: "Intel i7-8550U"},
		Memory: FastfetchModule{Type: "Memory", Result: "4.5 GiB / 15.4 GiB"},
		Disk:   FastfetchModule{Type: "Disk", Result: "45 GiB / 230 GiB"},
		Uptime: FastfetchModule{Type: "Uptime", Result: "5h 13m"},
	}

	lines := data.FormatCompact()

	if len(lines) != 6 {
		t.Errorf("FormatCompact() returned %d lines, want 6", len(lines))
	}

	// Verify compact format.
	expected := []string{
		"OS: Rocky Linux 10.1",
		"Kernel: 6.12.0",
		"CPU: Intel i7-8550U",
		"RAM: 4.5 GiB / 15.4 GiB",
		"Disk: 45 GiB / 230 GiB",
		"Uptime: 5h 13m",
	}

	for i, want := range expected {
		if i < len(lines) && lines[i] != want {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}
}

// TestFastfetchData_GetCoreModules tests core module extraction.
func TestFastfetchData_GetCoreModules(t *testing.T) {
	data := &FastfetchData{
		OS:       FastfetchModule{Type: "OS", Result: "Rocky Linux"},
		Host:     FastfetchModule{Type: "Host", Result: "ThinkPad"},
		Kernel:   FastfetchModule{Type: "Kernel", Result: "6.12.0"},
		Uptime:   FastfetchModule{Type: "Uptime", Result: "5h"},
		Packages: FastfetchModule{Type: "Packages", Result: "1234"},
		Shell:    FastfetchModule{Type: "Shell", Result: "zsh"},
		Terminal: FastfetchModule{Type: "Terminal", Result: "ghostty"},
		CPU:      FastfetchModule{Type: "CPU", Result: "i7"},
		GPU:      FastfetchModule{Type: "GPU", Result: "Intel"},
		Memory:   FastfetchModule{Type: "Memory", Result: "4GB"},
		Disk:     FastfetchModule{Type: "Disk", Result: "100GB"},
		LocalIP:  FastfetchModule{Type: "LocalIP", Result: "192.168.1.1"},
	}

	modules := data.GetCoreModules()

	if len(modules) != 12 {
		t.Errorf("GetCoreModules() returned %d modules, want 12", len(modules))
	}
}

// TestFastfetchData_IsEmpty tests emptiness check.
func TestFastfetchData_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		data *FastfetchData
		want bool
	}{
		{
			name: "empty data",
			data: &FastfetchData{},
			want: true,
		},
		{
			name: "with OS only",
			data: &FastfetchData{
				OS: FastfetchModule{Type: "OS", Result: "Linux"},
			},
			want: false,
		},
		{
			name: "with CPU only",
			data: &FastfetchData{
				CPU: FastfetchModule{Type: "CPU", Result: "i7"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFastfetchCollector_IsAvailable tests binary availability check.
func TestFastfetchCollector_IsAvailable(t *testing.T) {
	c := NewFastfetchCollector(DefaultConfig(), nil)

	// Mock lookPath to succeed.
	c.lookPath = func(string) (string, error) {
		return "/usr/bin/fastfetch", nil
	}

	if !c.IsAvailable() {
		t.Error("IsAvailable() = false, want true when binary exists")
	}

	// Mock lookPath to fail.
	c.lookPath = func(string) (string, error) {
		return "", exec.ErrNotFound
	}

	if c.IsAvailable() {
		t.Error("IsAvailable() = true, want false when binary not found")
	}
}

// TestParseRawModules tests the module parsing function.
func TestParseRawModules(t *testing.T) {
	modules := []FastfetchRawModule{
		{Type: "OS", Key: "OS", Result: "Rocky Linux"},
		{Type: "CPU", Key: "CPU", Result: "Intel i7"},
		{Type: "memory", Key: "Memory", Result: "8GB"}, // lowercase type
		{Type: "localIp", Key: "Local IP", Result: "192.168.1.1"},
	}

	data := parseRawModules(modules)

	if data.OS.Result != "Rocky Linux" {
		t.Errorf("OS.Result = %q, want %q", data.OS.Result, "Rocky Linux")
	}

	if data.CPU.Result != "Intel i7" {
		t.Errorf("CPU.Result = %q, want %q", data.CPU.Result, "Intel i7")
	}

	if data.Memory.Result != "8GB" {
		t.Errorf("Memory.Result = %q, want %q", data.Memory.Result, "8GB")
	}

	if data.LocalIP.Result != "192.168.1.1" {
		t.Errorf("LocalIP.Result = %q, want %q", data.LocalIP.Result, "192.168.1.1")
	}
}

// TestTruncateString tests the string truncation helper.
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "needs truncation",
			input:  "hello world",
			maxLen: 5,
			want:   "hello...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateString(tt.input, tt.maxLen); got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
