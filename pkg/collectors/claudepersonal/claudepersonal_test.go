package claudepersonal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	c := New(Config{})
	if c.Name() != "claudepersonal" {
		t.Errorf("Name() = %q, want %q", c.Name(), "claudepersonal")
	}
	if c.Interval() != DefaultScanInterval {
		t.Errorf("Interval() = %v, want %v", c.Interval(), DefaultScanInterval)
	}
	if !c.Healthy() {
		t.Error("new collector should be healthy")
	}
}

func TestBuildReport_Empty(t *testing.T) {
	c := New(Config{WindowHours: 5, MessageLimit: 45})
	c.state = &State{WindowHours: 5, MessageLimit: 45}

	report := c.buildReport(time.Now())
	if report.MessagesInWindow != 0 {
		t.Errorf("empty state: got %d messages, want 0", report.MessagesInWindow)
	}
	if report.MessageLimit != 45 {
		t.Errorf("limit = %d, want 45", report.MessageLimit)
	}
	if report.NextSlot != 0 {
		t.Errorf("NextSlot = %v, want 0", report.NextSlot)
	}
}

func TestBuildReport_MessagesInWindow(t *testing.T) {
	now := time.Now()
	c := New(Config{WindowHours: 5, MessageLimit: 45})
	c.state = &State{
		WindowHours:  5,
		MessageLimit: 45,
		Messages: []Message{
			{Timestamp: now.Add(-1 * time.Hour), Source: "jsonl"},
			{Timestamp: now.Add(-2 * time.Hour), Source: "jsonl"},
			{Timestamp: now.Add(-3 * time.Hour), Source: "manual"},
			// Outside window:
			{Timestamp: now.Add(-6 * time.Hour), Source: "jsonl"},
		},
	}

	report := c.buildReport(now)
	if report.MessagesInWindow != 3 {
		t.Errorf("got %d messages in window, want 3", report.MessagesInWindow)
	}
	if report.OldestInWindow.IsZero() {
		t.Error("OldestInWindow should not be zero")
	}
}

func TestBuildReport_NextSlot(t *testing.T) {
	now := time.Now()
	c := New(Config{WindowHours: 5, MessageLimit: 2})

	// 2 messages at limit, oldest is 4h ago -> expires in 1h.
	c.state = &State{
		WindowHours:  5,
		MessageLimit: 2,
		Messages: []Message{
			{Timestamp: now.Add(-4 * time.Hour), Source: "jsonl"},
			{Timestamp: now.Add(-1 * time.Hour), Source: "jsonl"},
		},
	}

	report := c.buildReport(now)
	if report.NextSlot <= 0 {
		t.Errorf("NextSlot = %v, want > 0", report.NextSlot)
	}
	// Should be ~1 hour.
	if report.NextSlot < 50*time.Minute || report.NextSlot > 70*time.Minute {
		t.Errorf("NextSlot = %v, want ~1h", report.NextSlot)
	}
}

func TestBuildReport_UnderLimit_NoNextSlot(t *testing.T) {
	now := time.Now()
	c := New(Config{WindowHours: 5, MessageLimit: 45})
	c.state = &State{
		WindowHours:  5,
		MessageLimit: 45,
		Messages: []Message{
			{Timestamp: now.Add(-1 * time.Hour), Source: "jsonl"},
		},
	}

	report := c.buildReport(now)
	if report.NextSlot != 0 {
		t.Errorf("under limit: NextSlot = %v, want 0", report.NextSlot)
	}
}

func TestPruneMessages(t *testing.T) {
	now := time.Now()
	c := New(Config{})
	c.state = &State{
		Messages: []Message{
			{Timestamp: now.Add(-25 * time.Hour), Source: "old"}, // should be pruned
			{Timestamp: now.Add(-23 * time.Hour), Source: "recent"},
			{Timestamp: now.Add(-1 * time.Hour), Source: "new"},
		},
	}

	c.pruneMessages()

	if len(c.state.Messages) != 2 {
		t.Errorf("after prune: %d messages, want 2", len(c.state.Messages))
	}
}

func TestCollect_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	c := New(Config{
		StateDir:  tmpDir,
		ClaudeDir: tmpDir, // no JSONL files here, that's OK
	})

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	report, ok := result.(*Report)
	if !ok {
		t.Fatalf("result type %T, want *Report", result)
	}

	if report.MessagesInWindow != 0 {
		t.Errorf("got %d messages, want 0 for empty dir", report.MessagesInWindow)
	}

	// State file should exist now.
	statePath := filepath.Join(tmpDir, "claude-personal.json")
	if _, err := os.Stat(statePath); err != nil {
		t.Errorf("state file not created: %v", err)
	}
}

func TestScanJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake project directory with a JSONL file.
	projectDir := filepath.Join(tmpDir, "projects", "test-project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	entries := []jsonlEntry{
		{Role: "user", Timestamp: now.Add(-2 * time.Hour)},
		{Role: "assistant", Timestamp: now.Add(-90 * time.Minute), Model: "opus"},
		{Role: "assistant", Timestamp: now.Add(-60 * time.Minute), Model: "sonnet"},
		{Role: "user", Timestamp: now.Add(-30 * time.Minute)},
		{Role: "assistant", Timestamp: now.Add(-15 * time.Minute), Model: "opus"},
	}

	f, err := os.Create(filepath.Join(projectDir, "session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	for _, e := range entries {
		_ = enc.Encode(e)
	}
	f.Close()

	c := New(Config{ClaudeDir: tmpDir})
	c.state = &State{FileOffsets: make(map[string]int64)}

	msgs := c.scanJSONL()
	if len(msgs) != 3 {
		t.Errorf("scanJSONL: got %d messages, want 3 (assistant only)", len(msgs))
	}

	// All should be source "jsonl".
	for _, m := range msgs {
		if m.Source != "jsonl" {
			t.Errorf("source = %q, want %q", m.Source, "jsonl")
		}
	}
}

func TestScanJSONL_IncrementalOffset(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "projects", "test-project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sessionPath := filepath.Join(projectDir, "session.jsonl")
	now := time.Now()

	// Write first batch.
	f, _ := os.Create(sessionPath)
	enc := json.NewEncoder(f)
	_ = enc.Encode(jsonlEntry{Role: "assistant", Timestamp: now.Add(-2 * time.Hour)})
	_ = enc.Encode(jsonlEntry{Role: "assistant", Timestamp: now.Add(-1 * time.Hour)})
	f.Close()

	c := New(Config{ClaudeDir: tmpDir})
	c.state = &State{FileOffsets: make(map[string]int64)}

	msgs1 := c.scanJSONL()
	if len(msgs1) != 2 {
		t.Fatalf("first scan: got %d, want 2", len(msgs1))
	}

	// Append more entries.
	f, _ = os.OpenFile(sessionPath, os.O_APPEND|os.O_WRONLY, 0o644)
	_ = json.NewEncoder(f).Encode(jsonlEntry{Role: "assistant", Timestamp: now.Add(-30 * time.Minute)})
	f.Close()

	// Second scan should only see the new entry.
	msgs2 := c.scanJSONL()
	if len(msgs2) != 1 {
		t.Errorf("second scan: got %d, want 1 (incremental)", len(msgs2))
	}
}

func TestRecordMessageIntegration(t *testing.T) {
	// This test validates the state file format by writing and reading back.
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "claude-personal.json")

	// Create initial state with one message.
	state := &State{
		WindowHours:  5,
		MessageLimit: 45,
		Messages: []Message{
			{Timestamp: time.Now().Add(-1 * time.Hour), Source: "jsonl"},
		},
		FileOffsets: make(map[string]int64),
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Read it back and verify.
	readBack := &State{}
	readData, _ := os.ReadFile(statePath)
	if err := json.Unmarshal(readData, readBack); err != nil {
		t.Fatal(err)
	}
	if len(readBack.Messages) != 1 {
		t.Errorf("readback: %d messages, want 1", len(readBack.Messages))
	}
	if readBack.WindowHours != 5 {
		t.Errorf("window hours = %d, want 5", readBack.WindowHours)
	}
}
