// Package claudepersonal tracks Claude personal plan usage by scanning
// local JSONL session files from Claude Code. Since there is no programmatic
// API for claude.ai Pro plan usage, this collector parses assistant message
// entries from ~/.claude/projects/*/*.jsonl to count messages in a rolling
// time window.
//
// State is persisted to ~/.cache/prompt-pulse/claude-personal.json and pruned
// of entries older than 24 hours on every write.
package claudepersonal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultWindowHours is the rolling window for message counting (Claude Pro
// resets usage in 5-hour windows).
const DefaultWindowHours = 5

// DefaultMessageLimit is the assumed message limit per window for Claude Pro.
const DefaultMessageLimit = 45

// DefaultScanInterval is how often the collector rescans JSONL files.
const DefaultScanInterval = 30 * time.Second

// retentionDuration is how long messages are kept in the state file.
const retentionDuration = 24 * time.Hour

// Message represents a single tracked Claude interaction.
type Message struct {
	Timestamp time.Time `json:"ts"`
	Model     string    `json:"model,omitempty"`
	Source    string    `json:"source"` // "jsonl" or "manual"
}

// State is the persisted state for the claude personal tracker.
type State struct {
	Messages     []Message `json:"messages"`
	WindowHours  int       `json:"window_hours"`
	MessageLimit int       `json:"message_limit"`
	LastScan     time.Time `json:"last_scan"`

	// FileOffsets tracks the last read position per JSONL file so we only
	// read new bytes on subsequent scans.
	FileOffsets map[string]int64 `json:"file_offsets,omitempty"`
}

// Report is the data emitted by the collector for the TUI widget.
type Report struct {
	// MessagesInWindow is the count of messages in the current rolling window.
	MessagesInWindow int
	// MessageLimit is the configured limit per window.
	MessageLimit int
	// WindowHours is the rolling window duration.
	WindowHours int
	// NextSlot is the time until the oldest message in the window expires,
	// opening a new slot. Zero if under the limit.
	NextSlot time.Duration
	// OldestInWindow is the timestamp of the oldest message in the current window.
	OldestInWindow time.Time
}

// Config holds settings for the claude personal collector.
type Config struct {
	// Interval controls how often the collector runs.
	Interval time.Duration
	// WindowHours is the rolling window in hours.
	WindowHours int
	// MessageLimit is the max messages per window.
	MessageLimit int
	// StateDir is the directory for the state file.
	StateDir string
	// ClaudeDir is the base directory for Claude Code session files.
	// Defaults to ~/.claude.
	ClaudeDir string
}

// Collector implements the collectors.Collector interface for Claude personal
// plan tracking.
type Collector struct {
	cfg     Config
	mu      sync.Mutex
	state   *State
	healthy bool
	lastErr error
}

// New creates a new claude personal collector with the given config.
func New(cfg Config) *Collector {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultScanInterval
	}
	if cfg.WindowHours <= 0 {
		cfg.WindowHours = DefaultWindowHours
	}
	if cfg.MessageLimit <= 0 {
		cfg.MessageLimit = DefaultMessageLimit
	}
	if cfg.StateDir == "" {
		home, _ := os.UserHomeDir()
		cfg.StateDir = filepath.Join(home, ".cache", "prompt-pulse")
	}
	if cfg.ClaudeDir == "" {
		home, _ := os.UserHomeDir()
		cfg.ClaudeDir = filepath.Join(home, ".claude")
	}

	return &Collector{
		cfg:     cfg,
		healthy: true,
		state:   &State{WindowHours: cfg.WindowHours, MessageLimit: cfg.MessageLimit},
	}
}

// Name returns the collector identifier.
func (c *Collector) Name() string { return "claudepersonal" }

// Interval returns the collection interval.
func (c *Collector) Interval() time.Duration { return c.cfg.Interval }

// Healthy returns whether the collector is functioning.
func (c *Collector) Healthy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.healthy
}

// Collect performs one collection cycle: loads state, scans for new JSONL
// entries, prunes old messages, saves state, and returns a Report.
func (c *Collector) Collect(_ context.Context) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Load persisted state.
	if err := c.loadState(); err != nil {
		// Not fatal — start fresh.
		c.state = &State{
			WindowHours:  c.cfg.WindowHours,
			MessageLimit: c.cfg.MessageLimit,
			FileOffsets:  make(map[string]int64),
		}
	}

	// Scan JSONL files for new messages.
	newMsgs := c.scanJSONL()
	c.state.Messages = append(c.state.Messages, newMsgs...)
	c.state.LastScan = time.Now()

	// Prune messages older than retention.
	c.pruneMessages()

	// Save state.
	if err := c.saveState(); err != nil {
		c.healthy = false
		c.lastErr = err
		return nil, fmt.Errorf("save state: %w", err)
	}

	c.healthy = true
	c.lastErr = nil

	return c.buildReport(time.Now()), nil
}

// RecordMessage records a manual message timestamp to the state file. This is
// called from the --claude-msg CLI flag for tracking non-Code usage. The model
// parameter is optional (pass "" for default "sonnet").
func RecordMessage(model string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	stateDir := filepath.Join(home, ".cache", "prompt-pulse")
	statePath := filepath.Join(stateDir, "claude-personal.json")

	// Ensure directory exists.
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	// Load existing state.
	state := &State{
		WindowHours:  DefaultWindowHours,
		MessageLimit: DefaultMessageLimit,
		FileOffsets:  make(map[string]int64),
	}
	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, state)
	}

	// Add the manual message.
	msg := Message{
		Timestamp: time.Now(),
		Model:     model,
		Source:    "manual",
	}
	state.Messages = append(state.Messages, msg)

	// Prune old entries.
	cutoff := time.Now().Add(-retentionDuration)
	filtered := state.Messages[:0]
	for _, m := range state.Messages {
		if m.Timestamp.After(cutoff) {
			filtered = append(filtered, m)
		}
	}
	state.Messages = filtered

	// Save.
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	// Report current usage.
	windowStart := time.Now().Add(-time.Duration(state.WindowHours) * time.Hour)
	count := 0
	for _, m := range state.Messages {
		if m.Timestamp.After(windowStart) {
			count++
		}
	}
	remaining := state.MessageLimit - count
	if remaining < 0 {
		remaining = 0
	}
	fmt.Printf("Recorded message (model: %s). Window: %d/%d, remaining: %d\n",
		cmp(model, "sonnet"), count, state.MessageLimit, remaining)
	return nil
}

// cmp returns b if a is empty, else a. Helper for default model display.
func cmp(a, b string) string {
	if a == "" {
		return b
	}
	return a
}

// statePath returns the path to the state file.
func (c *Collector) statePath() string {
	return filepath.Join(c.cfg.StateDir, "claude-personal.json")
}

// loadState reads the persisted state from disk.
func (c *Collector) loadState() error {
	data, err := os.ReadFile(c.statePath())
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c.state)
}

// saveState writes the current state to disk.
func (c *Collector) saveState() error {
	if err := os.MkdirAll(c.cfg.StateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.statePath(), data, 0o644)
}

// pruneMessages removes messages older than retentionDuration.
func (c *Collector) pruneMessages() {
	cutoff := time.Now().Add(-retentionDuration)
	filtered := c.state.Messages[:0]
	for _, m := range c.state.Messages {
		if m.Timestamp.After(cutoff) {
			filtered = append(filtered, m)
		}
	}
	c.state.Messages = filtered
}

// buildReport computes the current usage report from state.
func (c *Collector) buildReport(now time.Time) *Report {
	windowStart := now.Add(-time.Duration(c.state.WindowHours) * time.Hour)

	var inWindow []Message
	for _, m := range c.state.Messages {
		if m.Timestamp.After(windowStart) {
			inWindow = append(inWindow, m)
		}
	}

	// Sort by timestamp to find the oldest.
	sort.Slice(inWindow, func(i, j int) bool {
		return inWindow[i].Timestamp.Before(inWindow[j].Timestamp)
	})

	report := &Report{
		MessagesInWindow: len(inWindow),
		MessageLimit:     c.state.MessageLimit,
		WindowHours:      c.state.WindowHours,
	}

	if len(inWindow) > 0 {
		report.OldestInWindow = inWindow[0].Timestamp
		// NextSlot is when the oldest message in the window expires.
		expiresAt := inWindow[0].Timestamp.Add(time.Duration(c.state.WindowHours) * time.Hour)
		if expiresAt.After(now) && len(inWindow) >= c.state.MessageLimit {
			report.NextSlot = expiresAt.Sub(now)
		}
	}

	return report
}

// jsonlEntry is a minimal representation of a Claude Code JSONL line.
// We only care about assistant messages to count interactions.
type jsonlEntry struct {
	Type      string    `json:"type"`
	Role      string    `json:"role"`
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model"`
}

// scanJSONL scans Claude Code session files for new assistant messages.
func (c *Collector) scanJSONL() []Message {
	if c.state.FileOffsets == nil {
		c.state.FileOffsets = make(map[string]int64)
	}

	// Find all JSONL files under ~/.claude/projects/
	projectsDir := filepath.Join(c.cfg.ClaudeDir, "projects")
	pattern := filepath.Join(projectsDir, "*", "*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}

	var newMsgs []Message
	seen := make(map[string]bool) // dedup by "ts:file"

	for _, path := range matches {
		offset := c.state.FileOffsets[path]

		info, err := os.Stat(path)
		if err != nil || info.Size() <= offset {
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			continue
		}

		// Seek to last known offset.
		if offset > 0 {
			if _, err := f.Seek(offset, 0); err != nil {
				f.Close()
				continue
			}
		}

		// Read remaining bytes.
		buf := make([]byte, info.Size()-offset)
		n, err := f.Read(buf)
		f.Close()
		if err != nil || n == 0 {
			continue
		}

		// Update offset.
		c.state.FileOffsets[path] = offset + int64(n)

		// Parse lines.
		lines := strings.Split(string(buf[:n]), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var entry jsonlEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}

			// Only count assistant messages.
			if entry.Role != "assistant" {
				continue
			}

			// Require a timestamp.
			if entry.Timestamp.IsZero() {
				continue
			}

			// Dedup key.
			key := fmt.Sprintf("%d:%s", entry.Timestamp.UnixNano(), filepath.Base(path))
			if seen[key] {
				continue
			}
			seen[key] = true

			newMsgs = append(newMsgs, Message{
				Timestamp: entry.Timestamp,
				Model:     entry.Model,
				Source:    "jsonl",
			})
		}
	}

	return newMsgs
}
