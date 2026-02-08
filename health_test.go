package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteHealthFile(t *testing.T) {
	dir := t.TempDir()
	names := []string{"claude", "billing", "infra", "fastfetch"}

	if err := writeHealthFile(dir, names); err != nil {
		t.Fatalf("writeHealthFile: %v", err)
	}

	path := filepath.Join(dir, healthFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read health file: %v", err)
	}

	var status HealthStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if status.Status != "ok" {
		t.Errorf("status = %q, want %q", status.Status, "ok")
	}
	if len(status.Collectors) != 4 {
		t.Errorf("collectors count = %d, want 4", len(status.Collectors))
	}
	for _, name := range names {
		if status.Collectors[name] != "ok" {
			t.Errorf("collector %q = %q, want %q", name, status.Collectors[name], "ok")
		}
	}
	if time.Since(status.LastPoll) > time.Minute {
		t.Error("last_poll should be recent")
	}
}

func TestReadHealthFile(t *testing.T) {
	dir := t.TempDir()

	// Write a health file.
	if err := writeHealthFile(dir, []string{"claude"}); err != nil {
		t.Fatalf("writeHealthFile: %v", err)
	}

	// Read it back.
	status, err := readHealthFile(dir)
	if err != nil {
		t.Fatalf("readHealthFile: %v", err)
	}
	if status.Status != "ok" {
		t.Errorf("status = %q, want %q", status.Status, "ok")
	}
}

func TestReadHealthFile_Missing(t *testing.T) {
	dir := t.TempDir()

	_, err := readHealthFile(dir)
	if err == nil {
		t.Error("expected error for missing health file")
	}
}

func TestCheckHealth_Missing(t *testing.T) {
	dir := t.TempDir()
	code := checkHealth(dir, 15*time.Minute, false)
	if code != 1 {
		t.Errorf("expected exit code 1 for missing health, got %d", code)
	}
}

func TestCheckHealth_Fresh(t *testing.T) {
	dir := t.TempDir()
	if err := writeHealthFile(dir, []string{"claude"}); err != nil {
		t.Fatalf("writeHealthFile: %v", err)
	}

	code := checkHealth(dir, 15*time.Minute, false)
	if code != 0 {
		t.Errorf("expected exit code 0 for fresh health, got %d", code)
	}
}

func TestCheckHealth_Stale(t *testing.T) {
	dir := t.TempDir()

	// Write a health file with an old timestamp.
	status := HealthStatus{
		Status:     "ok",
		LastPoll:   time.Now().Add(-1 * time.Hour),
		Collectors: map[string]string{"claude": "ok"},
	}
	data, _ := json.MarshalIndent(status, "", "  ")
	path := filepath.Join(dir, healthFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write stale health: %v", err)
	}

	code := checkHealth(dir, 15*time.Minute, false)
	if code != 1 {
		t.Errorf("expected exit code 1 for stale health, got %d", code)
	}
}
