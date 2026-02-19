package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/config"
)

// shortSockDir creates a short temporary directory suitable for Unix socket
// paths. macOS limits Unix socket paths to ~104 characters, which t.TempDir()
// can exceed due to long test names. This helper creates a directory under
// /tmp with a short unique name.
func shortSockDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ppd")
	if err != nil {
		t.Fatalf("MkdirTemp() error: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// ---------------------------------------------------------------------------
// PID file tests
// ---------------------------------------------------------------------------

func TestAcquirePID_CreatesFileWithCorrectPID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")

	if err := AcquirePID(pidPath); err != nil {
		t.Fatalf("AcquirePID() error: %v", err)
	}
	defer ReleasePID(pidPath)

	got, err := ReadPID(pidPath)
	if err != nil {
		t.Fatalf("ReadPID() error: %v", err)
	}
	if got != os.Getpid() {
		t.Errorf("ReadPID() = %d, want %d", got, os.Getpid())
	}
}

func TestAcquirePID_PreventsDoubleAcquire(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")

	if err := AcquirePID(pidPath); err != nil {
		t.Fatalf("first AcquirePID() error: %v", err)
	}
	defer ReleasePID(pidPath)

	// Second acquire should fail because the current process is still alive.
	err := AcquirePID(pidPath)
	if err == nil {
		t.Fatal("second AcquirePID() should fail but returned nil")
	}
}

func TestAcquirePID_CleansStalePID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")

	// Write a PID that almost certainly does not exist.
	stalePID := 4999999
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(stalePID)), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// AcquirePID should detect the stale process and succeed.
	if err := AcquirePID(pidPath); err != nil {
		t.Fatalf("AcquirePID() with stale PID error: %v", err)
	}
	defer ReleasePID(pidPath)

	got, err := ReadPID(pidPath)
	if err != nil {
		t.Fatalf("ReadPID() error: %v", err)
	}
	if got != os.Getpid() {
		t.Errorf("ReadPID() = %d, want %d (stale PID should be replaced)", got, os.Getpid())
	}
}

func TestReleasePID_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")

	if err := AcquirePID(pidPath); err != nil {
		t.Fatalf("AcquirePID() error: %v", err)
	}

	if err := ReleasePID(pidPath); err != nil {
		t.Fatalf("ReleasePID() error: %v", err)
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Errorf("PID file still exists after ReleasePID(); stat err = %v", err)
	}
}

func TestReleasePID_NoErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "nonexistent.pid")

	if err := ReleasePID(pidPath); err != nil {
		t.Errorf("ReleasePID() on missing file should not error, got: %v", err)
	}
}

func TestReadPID_ReturnsCorrectPID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")

	expected := 12345
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(expected)), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	got, err := ReadPID(pidPath)
	if err != nil {
		t.Fatalf("ReadPID() error: %v", err)
	}
	if got != expected {
		t.Errorf("ReadPID() = %d, want %d", got, expected)
	}
}

func TestReadPID_ErrorOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "nonexistent.pid")

	_, err := ReadPID(pidPath)
	if err == nil {
		t.Fatal("ReadPID() should error on missing file")
	}
}

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	if !IsProcessAlive(os.Getpid()) {
		t.Error("IsProcessAlive(os.Getpid()) = false, want true")
	}
}

func TestIsProcessAlive_BogusPID(t *testing.T) {
	// PID 4999999 almost certainly does not exist.
	if IsProcessAlive(4999999) {
		t.Error("IsProcessAlive(4999999) = true, want false")
	}
}

func TestIsProcessAlive_ZeroPID(t *testing.T) {
	if IsProcessAlive(0) {
		t.Error("IsProcessAlive(0) = true, want false")
	}
}

func TestIsProcessAlive_NegativePID(t *testing.T) {
	if IsProcessAlive(-1) {
		t.Error("IsProcessAlive(-1) = true, want false")
	}
}

// ---------------------------------------------------------------------------
// Health file tests
// ---------------------------------------------------------------------------

func TestHealthFile_WriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "health.json")

	now := time.Now().Truncate(time.Second)
	original := &HealthStatus{
		PID:       os.Getpid(),
		Uptime:    5 * time.Minute,
		StartedAt: now.Add(-5 * time.Minute),
		Collectors: map[string]CollectorHealth{
			"sysmetrics": {
				Name:       "sysmetrics",
				Healthy:    true,
				LastRun:    now,
				ErrorCount: 0,
			},
			"claude": {
				Name:       "claude",
				Healthy:    false,
				LastRun:    now.Add(-1 * time.Minute),
				ErrorCount: 3,
			},
		},
		LastUpdate: now,
	}

	if err := WriteHealthFile(healthPath, original); err != nil {
		t.Fatalf("WriteHealthFile() error: %v", err)
	}

	got, err := ReadHealthFile(healthPath)
	if err != nil {
		t.Fatalf("ReadHealthFile() error: %v", err)
	}

	if got.PID != original.PID {
		t.Errorf("PID = %d, want %d", got.PID, original.PID)
	}
	if got.Uptime != original.Uptime {
		t.Errorf("Uptime = %v, want %v", got.Uptime, original.Uptime)
	}
	if len(got.Collectors) != 2 {
		t.Fatalf("len(Collectors) = %d, want 2", len(got.Collectors))
	}

	sys := got.Collectors["sysmetrics"]
	if !sys.Healthy {
		t.Error("sysmetrics.Healthy = false, want true")
	}
	if sys.ErrorCount != 0 {
		t.Errorf("sysmetrics.ErrorCount = %d, want 0", sys.ErrorCount)
	}

	cl := got.Collectors["claude"]
	if cl.Healthy {
		t.Error("claude.Healthy = true, want false")
	}
	if cl.ErrorCount != 3 {
		t.Errorf("claude.ErrorCount = %d, want 3", cl.ErrorCount)
	}
}

func TestHealthFile_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "health.json")

	// Write a health file and verify no temp file remains.
	status := &HealthStatus{
		PID:        1234,
		Collectors: map[string]CollectorHealth{},
		LastUpdate: time.Now(),
	}

	if err := WriteHealthFile(healthPath, status); err != nil {
		t.Fatalf("WriteHealthFile() error: %v", err)
	}

	// Verify main file exists.
	if _, err := os.Stat(healthPath); err != nil {
		t.Errorf("health file does not exist: %v", err)
	}

	// Verify temp file was cleaned up.
	tmpPath := healthPath + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file %s still exists after write", tmpPath)
	}
}

func TestReadHealthFile_ErrorOnMissing(t *testing.T) {
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "nonexistent.json")

	_, err := ReadHealthFile(healthPath)
	if err == nil {
		t.Fatal("ReadHealthFile() should error on missing file")
	}
}

func TestHealthFile_JSONIsHumanReadable(t *testing.T) {
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "health.json")

	status := &HealthStatus{
		PID:        42,
		Collectors: map[string]CollectorHealth{},
		LastUpdate: time.Now(),
	}

	if err := WriteHealthFile(healthPath, status); err != nil {
		t.Fatalf("WriteHealthFile() error: %v", err)
	}

	data, err := os.ReadFile(healthPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	// Human-readable JSON should contain newlines (from MarshalIndent).
	content := string(data)
	if len(content) < 10 {
		t.Fatalf("health file is suspiciously short: %q", content)
	}

	// Verify it starts with { and is valid JSON.
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Errorf("health file is not valid JSON: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Banner cache tests
// ---------------------------------------------------------------------------

func TestBannerCache_PutGetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "banner.json")
	bc := NewBannerCache(cachePath)

	entry := &BannerEntry{
		Rendered:  "Hello, terminal!",
		Width:     80,
		Height:    24,
		Protocol:  "kitty",
		Timestamp: time.Now().Truncate(time.Second),
	}

	if err := bc.Put(entry); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	got, ok := bc.Get(80, 24, "kitty")
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if got.Rendered != entry.Rendered {
		t.Errorf("Rendered = %q, want %q", got.Rendered, entry.Rendered)
	}
	if got.Width != 80 {
		t.Errorf("Width = %d, want 80", got.Width)
	}
	if got.Height != 24 {
		t.Errorf("Height = %d, want 24", got.Height)
	}
	if got.Protocol != "kitty" {
		t.Errorf("Protocol = %q, want %q", got.Protocol, "kitty")
	}
	if got.Hash == "" {
		t.Error("Hash is empty, should have been auto-computed")
	}
}

func TestBannerCache_MissForDifferentDimensions(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "banner.json")
	bc := NewBannerCache(cachePath)

	entry := &BannerEntry{
		Rendered:  "Banner 80x24",
		Width:     80,
		Height:    24,
		Protocol:  "kitty",
		Timestamp: time.Now(),
	}

	if err := bc.Put(entry); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Different width.
	_, ok := bc.Get(120, 24, "kitty")
	if ok {
		t.Error("Get(120, 24, kitty) returned true, want false (different width)")
	}

	// Different height.
	_, ok = bc.Get(80, 40, "kitty")
	if ok {
		t.Error("Get(80, 40, kitty) returned true, want false (different height)")
	}

	// Different protocol.
	_, ok = bc.Get(80, 24, "sixel")
	if ok {
		t.Error("Get(80, 24, sixel) returned true, want false (different protocol)")
	}
}

func TestBannerCache_Invalidation(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "banner.json")
	bc := NewBannerCache(cachePath)

	entry := &BannerEntry{
		Rendered:  "cached banner",
		Width:     80,
		Height:    24,
		Protocol:  "kitty",
		Timestamp: time.Now(),
	}

	if err := bc.Put(entry); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if err := bc.Invalidate(); err != nil {
		t.Fatalf("Invalidate() error: %v", err)
	}

	_, ok := bc.Get(80, 24, "kitty")
	if ok {
		t.Error("Get() returned true after Invalidate(), want false")
	}
}

func TestBannerCache_InvalidateNoFile(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "nonexistent-banner.json")
	bc := NewBannerCache(cachePath)

	// Should not error when file does not exist.
	if err := bc.Invalidate(); err != nil {
		t.Errorf("Invalidate() on missing file error: %v", err)
	}
}

func TestBannerCache_Staleness(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "banner.json")
	bc := NewBannerCache(cachePath)

	// No file means stale.
	if !bc.IsStale(time.Hour) {
		t.Error("IsStale() with no file should return true")
	}

	// Put an entry with a past timestamp.
	entry := &BannerEntry{
		Rendered:  "old banner",
		Width:     80,
		Height:    24,
		Protocol:  "kitty",
		Timestamp: time.Now().Add(-2 * time.Hour),
	}
	if err := bc.Put(entry); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Should be stale with 1 hour maxAge since the entry is 2 hours old.
	if !bc.IsStale(time.Hour) {
		t.Error("IsStale(1h) should return true for 2-hour-old entry")
	}

	// Should not be stale with 3 hour maxAge.
	if bc.IsStale(3 * time.Hour) {
		t.Error("IsStale(3h) should return false for 2-hour-old entry")
	}
}

func TestBannerCache_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "banner.json")
	bc := NewBannerCache(cachePath)

	entries := []*BannerEntry{
		{Rendered: "80x24 kitty", Width: 80, Height: 24, Protocol: "kitty", Timestamp: time.Now()},
		{Rendered: "120x40 sixel", Width: 120, Height: 40, Protocol: "sixel", Timestamp: time.Now()},
		{Rendered: "80x24 iterm", Width: 80, Height: 24, Protocol: "iterm", Timestamp: time.Now()},
	}

	for _, e := range entries {
		if err := bc.Put(e); err != nil {
			t.Fatalf("Put() error: %v", err)
		}
	}

	// Verify each entry is independently retrievable.
	for _, e := range entries {
		got, ok := bc.Get(e.Width, e.Height, e.Protocol)
		if !ok {
			t.Errorf("Get(%d, %d, %s) returned false", e.Width, e.Height, e.Protocol)
			continue
		}
		if got.Rendered != e.Rendered {
			t.Errorf("Get(%d, %d, %s).Rendered = %q, want %q", e.Width, e.Height, e.Protocol, got.Rendered, e.Rendered)
		}
	}
}

func TestBannerCache_SurvivesReload(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "banner.json")

	// Write with first cache instance.
	bc1 := NewBannerCache(cachePath)
	entry := &BannerEntry{
		Rendered:  "persistent banner",
		Width:     80,
		Height:    24,
		Protocol:  "kitty",
		Timestamp: time.Now().Truncate(time.Second),
	}
	if err := bc1.Put(entry); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Read with second cache instance (simulates daemon restart).
	bc2 := NewBannerCache(cachePath)
	got, ok := bc2.Get(80, 24, "kitty")
	if !ok {
		t.Fatal("Get() on reloaded cache returned false, want true")
	}
	if got.Rendered != "persistent banner" {
		t.Errorf("Rendered = %q, want %q", got.Rendered, "persistent banner")
	}
}

// ---------------------------------------------------------------------------
// IPC tests
// ---------------------------------------------------------------------------

// testHandler implements IPCHandler for testing.
type testHandler struct{}

func (h *testHandler) HandleCommand(cmd string, args map[string]string) (string, error) {
	switch cmd {
	case "HEALTH":
		return `{"status":"ok","pid":1234}`, nil
	case "PING":
		return `{"pong":true}`, nil
	default:
		return "", fmt.Errorf("unknown command: %s", cmd)
	}
}

func TestIPCServer_StartStop(t *testing.T) {
	dir := shortSockDir(t)
	sockPath := filepath.Join(dir, "test.sock")

	srv := NewIPCServer(sockPath, &testHandler{})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Verify socket file exists.
	info, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("socket file does not exist: %v", err)
	}

	// Verify socket permissions (0600).
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("socket permissions = %o, want %o", perm, 0o600)
	}

	srv.Stop()

	// Verify socket file is cleaned up.
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Errorf("socket file still exists after Stop(); stat err = %v", err)
	}
}

func TestIPCClient_SendCommand(t *testing.T) {
	dir := shortSockDir(t)
	sockPath := filepath.Join(dir, "test.sock")

	srv := NewIPCServer(sockPath, &testHandler{})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer srv.Stop()

	client := NewIPCClient(sockPath)
	resp, err := client.SendCommand("HEALTH")
	if err != nil {
		t.Fatalf("SendCommand() error: %v", err)
	}

	// Parse the response to verify it's valid JSON.
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &m); err != nil {
		t.Fatalf("response is not valid JSON: %v\nresponse: %q", err, resp)
	}

	if m["status"] != "ok" {
		t.Errorf("response status = %v, want %q", m["status"], "ok")
	}
}

func TestIPC_HealthCommand_ReturnsValidJSON(t *testing.T) {
	dir := shortSockDir(t)
	sockPath := filepath.Join(dir, "test.sock")

	srv := NewIPCServer(sockPath, &testHandler{})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer srv.Stop()

	client := NewIPCClient(sockPath)
	resp, err := client.SendCommand("HEALTH")
	if err != nil {
		t.Fatalf("SendCommand() error: %v", err)
	}

	if !json.Valid([]byte(resp)) {
		t.Errorf("HEALTH response is not valid JSON: %q", resp)
	}
}

func TestIPC_UnknownCommand_ReturnsError(t *testing.T) {
	dir := shortSockDir(t)
	sockPath := filepath.Join(dir, "test.sock")

	srv := NewIPCServer(sockPath, &testHandler{})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer srv.Stop()

	client := NewIPCClient(sockPath)
	resp, err := client.SendCommand("NOTACOMMAND")
	if err != nil {
		t.Fatalf("SendCommand() error: %v", err)
	}

	// The response should be a JSON error.
	var m map[string]string
	if err := json.Unmarshal([]byte(resp), &m); err != nil {
		t.Fatalf("response is not valid JSON: %v\nresponse: %q", err, resp)
	}

	if _, ok := m["error"]; !ok {
		t.Error("error response should contain 'error' key")
	}
}

func TestIPCClient_ConnectFailure(t *testing.T) {
	client := NewIPCClient("/tmp/nonexistent-prompt-pulse-test.sock")
	_, err := client.SendCommand("HEALTH")
	if err == nil {
		t.Fatal("SendCommand() to nonexistent socket should fail")
	}
}

func TestParseIPCCommand_Simple(t *testing.T) {
	tests := []struct {
		input   string
		wantCmd string
		wantLen int
	}{
		{"HEALTH", "HEALTH", 0},
		{"health", "HEALTH", 0},
		{"REFRESH", "REFRESH", 0},
		{"QUIT", "QUIT", 0},
	}

	for _, tc := range tests {
		cmd, args := parseIPCCommand(tc.input)
		if cmd != tc.wantCmd {
			t.Errorf("parseIPCCommand(%q) cmd = %q, want %q", tc.input, cmd, tc.wantCmd)
		}
		// Simple commands should have empty args.
		if tc.wantLen == 0 && len(args) != 0 {
			// BANNER key may be pre-populated; for non-BANNER commands args
			// should be effectively empty.
			for _, v := range args {
				if v != "" {
					t.Errorf("parseIPCCommand(%q) args has non-empty value", tc.input)
				}
			}
		}
	}
}

func TestParseIPCCommand_Banner(t *testing.T) {
	cmd, args := parseIPCCommand("BANNER 80 24 kitty")
	if cmd != "BANNER" {
		t.Errorf("cmd = %q, want BANNER", cmd)
	}
	if args["width"] != "80" {
		t.Errorf("width = %q, want 80", args["width"])
	}
	if args["height"] != "24" {
		t.Errorf("height = %q, want 24", args["height"])
	}
	if args["protocol"] != "kitty" {
		t.Errorf("protocol = %q, want kitty", args["protocol"])
	}
}

// ---------------------------------------------------------------------------
// Daemon config tests
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PIDFile == "" {
		t.Error("PIDFile is empty")
	}
	if cfg.HealthFile == "" {
		t.Error("HealthFile is empty")
	}
	if cfg.SocketPath == "" {
		t.Error("SocketPath is empty")
	}
	if cfg.DataDir == "" {
		t.Error("DataDir is empty")
	}
	if cfg.BannerCacheFile == "" {
		t.Error("BannerCacheFile is empty")
	}
}

func TestNew_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		PIDFile:         filepath.Join(dir, "test.pid"),
		HealthFile:      filepath.Join(dir, "health.json"),
		SocketPath:      filepath.Join(dir, "test.sock"),
		DataDir:         filepath.Join(dir, "data"),
		BannerCacheFile: filepath.Join(dir, "banner.json"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if d == nil {
		t.Fatal("New() returned nil daemon")
	}
}

func TestNew_EmptyPIDFile(t *testing.T) {
	cfg := Config{
		PIDFile:         "",
		HealthFile:      "/tmp/health.json",
		SocketPath:      "/tmp/test.sock",
		DataDir:         "/tmp/data",
		BannerCacheFile: "/tmp/banner.json",
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() with empty PIDFile should fail")
	}
}

func TestNew_EmptyHealthFile(t *testing.T) {
	cfg := Config{
		PIDFile:         "/tmp/test.pid",
		HealthFile:      "",
		SocketPath:      "/tmp/test.sock",
		DataDir:         "/tmp/data",
		BannerCacheFile: "/tmp/banner.json",
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() with empty HealthFile should fail")
	}
}

func TestNew_EmptySocketPath(t *testing.T) {
	cfg := Config{
		PIDFile:         "/tmp/test.pid",
		HealthFile:      "/tmp/health.json",
		SocketPath:      "",
		DataDir:         "/tmp/data",
		BannerCacheFile: "/tmp/banner.json",
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() with empty SocketPath should fail")
	}
}

func TestNew_EmptyDataDir(t *testing.T) {
	cfg := Config{
		PIDFile:         "/tmp/test.pid",
		HealthFile:      "/tmp/health.json",
		SocketPath:      "/tmp/test.sock",
		DataDir:         "",
		BannerCacheFile: "/tmp/banner.json",
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() with empty DataDir should fail")
	}
}

func TestNew_EmptyBannerCacheFile(t *testing.T) {
	cfg := Config{
		PIDFile:         "/tmp/test.pid",
		HealthFile:      "/tmp/health.json",
		SocketPath:      "/tmp/test.sock",
		DataDir:         "/tmp/data",
		BannerCacheFile: "",
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() with empty BannerCacheFile should fail")
	}
}

func TestDaemon_IsRunning_NoPIDFile(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		PIDFile:         filepath.Join(dir, "test.pid"),
		HealthFile:      filepath.Join(dir, "health.json"),
		SocketPath:      filepath.Join(dir, "test.sock"),
		DataDir:         filepath.Join(dir, "data"),
		BannerCacheFile: filepath.Join(dir, "banner.json"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if d.IsRunning() {
		t.Error("IsRunning() = true, want false (no PID file)")
	}
}

func TestDaemon_IsRunning_WithPIDFile(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	cfg := Config{
		PIDFile:         pidPath,
		HealthFile:      filepath.Join(dir, "health.json"),
		SocketPath:      filepath.Join(dir, "test.sock"),
		DataDir:         filepath.Join(dir, "data"),
		BannerCacheFile: filepath.Join(dir, "banner.json"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Write current PID.
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	defer os.Remove(pidPath)

	if !d.IsRunning() {
		t.Error("IsRunning() = false, want true (current process PID in file)")
	}
}

func TestDaemon_Running_InitiallyFalse(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		PIDFile:         filepath.Join(dir, "test.pid"),
		HealthFile:      filepath.Join(dir, "health.json"),
		SocketPath:      filepath.Join(dir, "test.sock"),
		DataDir:         filepath.Join(dir, "data"),
		BannerCacheFile: filepath.Join(dir, "banner.json"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if d.Running() {
		t.Error("Running() = true before Start(), want false")
	}
}

func TestDaemon_UpdateCollector(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		PIDFile:         filepath.Join(dir, "test.pid"),
		HealthFile:      filepath.Join(dir, "health.json"),
		SocketPath:      filepath.Join(dir, "test.sock"),
		DataDir:         filepath.Join(dir, "data"),
		BannerCacheFile: filepath.Join(dir, "banner.json"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	d.UpdateCollector("sysmetrics", true, 0)
	d.UpdateCollector("claude", false, 5)

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.collectors) != 2 {
		t.Fatalf("len(collectors) = %d, want 2", len(d.collectors))
	}

	sys := d.collectors["sysmetrics"]
	if !sys.Healthy {
		t.Error("sysmetrics.Healthy = false, want true")
	}
	if sys.ErrorCount != 0 {
		t.Errorf("sysmetrics.ErrorCount = %d, want 0", sys.ErrorCount)
	}

	cl := d.collectors["claude"]
	if cl.Healthy {
		t.Error("claude.Healthy = true, want false")
	}
	if cl.ErrorCount != 5 {
		t.Errorf("claude.ErrorCount = %d, want 5", cl.ErrorCount)
	}
}

func TestDaemon_HandleCommand_Health(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		PIDFile:         filepath.Join(dir, "test.pid"),
		HealthFile:      filepath.Join(dir, "health.json"),
		SocketPath:      filepath.Join(dir, "test.sock"),
		DataDir:         filepath.Join(dir, "data"),
		BannerCacheFile: filepath.Join(dir, "banner.json"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	d.startedAt = time.Now()
	d.UpdateCollector("test", true, 0)

	resp, err := d.HandleCommand("HEALTH", nil)
	if err != nil {
		t.Fatalf("HandleCommand(HEALTH) error: %v", err)
	}

	if !json.Valid([]byte(resp)) {
		t.Errorf("HEALTH response is not valid JSON: %q", resp)
	}

	var status HealthStatus
	if err := json.Unmarshal([]byte(resp), &status); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if status.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", status.PID, os.Getpid())
	}
}

func TestDaemon_HandleCommand_Refresh(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		PIDFile:         filepath.Join(dir, "test.pid"),
		HealthFile:      filepath.Join(dir, "health.json"),
		SocketPath:      filepath.Join(dir, "test.sock"),
		DataDir:         filepath.Join(dir, "data"),
		BannerCacheFile: filepath.Join(dir, "banner.json"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	resp, err := d.HandleCommand("REFRESH", nil)
	if err != nil {
		t.Fatalf("HandleCommand(REFRESH) error: %v", err)
	}

	if !json.Valid([]byte(resp)) {
		t.Errorf("REFRESH response is not valid JSON: %q", resp)
	}
}

func TestDaemon_HandleCommand_Unknown(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		PIDFile:         filepath.Join(dir, "test.pid"),
		HealthFile:      filepath.Join(dir, "health.json"),
		SocketPath:      filepath.Join(dir, "test.sock"),
		DataDir:         filepath.Join(dir, "data"),
		BannerCacheFile: filepath.Join(dir, "banner.json"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	_, err = d.HandleCommand("FOOBAR", nil)
	if err == nil {
		t.Fatal("HandleCommand(FOOBAR) should return error")
	}
}

func TestDaemon_HandleCommand_Banner_NoCachedBanner(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		PIDFile:         filepath.Join(dir, "test.pid"),
		HealthFile:      filepath.Join(dir, "health.json"),
		SocketPath:      filepath.Join(dir, "test.sock"),
		DataDir:         filepath.Join(dir, "data"),
		BannerCacheFile: filepath.Join(dir, "banner.json"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	args := map[string]string{"width": "80", "height": "24", "protocol": "kitty"}
	_, err = d.HandleCommand("BANNER", args)
	if err == nil {
		t.Fatal("HandleCommand(BANNER) with no cached banner should return error")
	}
}

// ---------------------------------------------------------------------------
// Integration: IPC with Daemon handler
// ---------------------------------------------------------------------------

func TestIPC_WithDaemonHandler(t *testing.T) {
	dir := shortSockDir(t)
	sockPath := filepath.Join(dir, "test.sock")
	cfg := Config{
		PIDFile:         filepath.Join(dir, "test.pid"),
		HealthFile:      filepath.Join(dir, "health.json"),
		SocketPath:      sockPath,
		DataDir:         filepath.Join(dir, "data"),
		BannerCacheFile: filepath.Join(dir, "banner.json"),
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	d.startedAt = time.Now()

	srv := NewIPCServer(sockPath, d)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer srv.Stop()

	client := NewIPCClient(sockPath)

	// Test HEALTH.
	resp, err := client.SendCommand("HEALTH")
	if err != nil {
		t.Fatalf("SendCommand(HEALTH) error: %v", err)
	}
	if !json.Valid([]byte(resp)) {
		t.Errorf("HEALTH response is not valid JSON: %q", resp)
	}

	// Test REFRESH.
	resp, err = client.SendCommand("REFRESH")
	if err != nil {
		t.Fatalf("SendCommand(REFRESH) error: %v", err)
	}
	if !json.Valid([]byte(resp)) {
		t.Errorf("REFRESH response is not valid JSON: %q", resp)
	}

	// Test unknown command.
	resp, err = client.SendCommand("BOGUS")
	if err != nil {
		t.Fatalf("SendCommand(BOGUS) error: %v", err)
	}
	var errResp map[string]string
	if err := json.Unmarshal([]byte(resp), &errResp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if _, ok := errResp["error"]; !ok {
		t.Error("BOGUS response should contain 'error' key")
	}
}

func TestComputeHash(t *testing.T) {
	h1 := computeHash("hello")
	h2 := computeHash("hello")
	h3 := computeHash("world")

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("SHA-256 hex hash should be 64 chars, got %d", len(h1))
	}
}

// ---------------------------------------------------------------------------
// BuildRegistry tests
// ---------------------------------------------------------------------------

func TestBuildRegistry_AllEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collectors.SysMetrics.Enabled = true
	cfg.Collectors.Tailscale.Enabled = true
	cfg.Collectors.Kubernetes.Enabled = true
	cfg.Collectors.Claude.Enabled = true
	cfg.Collectors.Billing.Enabled = true

	reg := BuildRegistry(cfg)
	names := reg.List()
	if len(names) != 5 {
		t.Errorf("BuildRegistry(all enabled) registered %d collectors, want 5: %v", len(names), names)
	}
}

func TestBuildRegistry_DisabledCollectors(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collectors.SysMetrics.Enabled = true
	cfg.Collectors.Tailscale.Enabled = false
	cfg.Collectors.Kubernetes.Enabled = false
	cfg.Collectors.Claude.Enabled = false
	cfg.Collectors.Billing.Enabled = false

	reg := BuildRegistry(cfg)
	names := reg.List()
	if len(names) != 1 {
		t.Errorf("BuildRegistry(only sysmetrics) registered %d collectors, want 1: %v", len(names), names)
	}
}

func TestBuildRegistry_NoneEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Collectors.SysMetrics.Enabled = false
	cfg.Collectors.Tailscale.Enabled = false
	cfg.Collectors.Kubernetes.Enabled = false
	cfg.Collectors.Claude.Enabled = false
	cfg.Collectors.Billing.Enabled = false

	reg := BuildRegistry(cfg)
	names := reg.List()
	if len(names) != 0 {
		t.Errorf("BuildRegistry(none enabled) registered %d collectors, want 0: %v", len(names), names)
	}
}
