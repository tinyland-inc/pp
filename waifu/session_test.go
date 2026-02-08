package waifu

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetSessionKey(t *testing.T) {
	// Test with explicit session ID.
	os.Setenv("PPULSE_SESSION_ID", "test-session-123")
	defer os.Unsetenv("PPULSE_SESSION_ID")

	key := GetSessionKey()
	if key != "test-session-123" {
		t.Errorf("GetSessionKey() with env = %q, want %q", key, "test-session-123")
	}

	// Test without explicit session ID.
	os.Unsetenv("PPULSE_SESSION_ID")
	key = GetSessionKey()
	if key == "" {
		t.Error("GetSessionKey() without env returned empty string")
	}
}

func TestNewSessionManager(t *testing.T) {
	dir := t.TempDir()

	cfg := SessionManagerConfig{
		CacheDir:        dir,
		MaxSessions:     5,
		ImageCacheTTL:   time.Hour,
		ImageMaxCacheMB: 10,
	}

	mgr, err := NewSessionManager(cfg)
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	if mgr.SessionCount() != 0 {
		t.Errorf("SessionCount() = %d, want 0", mgr.SessionCount())
	}

	// Verify sessions directory was created.
	sessionsDir := filepath.Join(dir, "sessions")
	if _, err := os.Stat(sessionsDir); err != nil {
		t.Errorf("sessions directory not created: %v", err)
	}
}

func TestSessionManager_HasSession(t *testing.T) {
	dir := t.TempDir()
	cfg := SessionManagerConfig{
		CacheDir:        dir,
		MaxSessions:     5,
		ImageCacheTTL:   time.Hour,
		ImageMaxCacheMB: 10,
	}

	mgr, err := NewSessionManager(cfg)
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Initially no sessions.
	if mgr.HasSession("test-session") {
		t.Error("HasSession() = true for non-existent session")
	}

	// Manually add a session (simulating what GetOrFetch does).
	mgr.mu.Lock()
	mgr.sessions["test-session"] = &SessionInfo{
		SessionID:  "test-session",
		Category:   "neko",
		ImageKey:   "session-test-session-neko",
		LastAccess: time.Now(),
		CreatedAt:  time.Now(),
	}
	mgr.mu.Unlock()

	if !mgr.HasSession("test-session") {
		t.Error("HasSession() = false for existing session")
	}
}

func TestSessionManager_LRUEviction(t *testing.T) {
	dir := t.TempDir()
	cfg := SessionManagerConfig{
		CacheDir:        dir,
		MaxSessions:     3, // Small limit for testing.
		ImageCacheTTL:   time.Hour,
		ImageMaxCacheMB: 10,
	}

	mgr, err := NewSessionManager(cfg)
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Add sessions with staggered access times.
	now := time.Now()
	mgr.mu.Lock()
	mgr.sessions["session-1"] = &SessionInfo{
		SessionID:  "session-1",
		Category:   "neko",
		ImageKey:   "session-session-1-neko",
		LastAccess: now.Add(-3 * time.Hour), // Oldest
		CreatedAt:  now.Add(-3 * time.Hour),
	}
	mgr.sessions["session-2"] = &SessionInfo{
		SessionID:  "session-2",
		Category:   "neko",
		ImageKey:   "session-session-2-neko",
		LastAccess: now.Add(-2 * time.Hour),
		CreatedAt:  now.Add(-2 * time.Hour),
	}
	mgr.sessions["session-3"] = &SessionInfo{
		SessionID:  "session-3",
		Category:   "neko",
		ImageKey:   "session-session-3-neko",
		LastAccess: now.Add(-1 * time.Hour),
		CreatedAt:  now.Add(-1 * time.Hour),
	}
	mgr.mu.Unlock()

	if mgr.SessionCount() != 3 {
		t.Errorf("SessionCount() = %d, want 3", mgr.SessionCount())
	}

	// Add a fourth session, should evict the oldest (session-1).
	mgr.mu.Lock()
	mgr.sessions["session-4"] = &SessionInfo{
		SessionID:  "session-4",
		Category:   "neko",
		ImageKey:   "session-session-4-neko",
		LastAccess: now,
		CreatedAt:  now,
	}
	err = mgr.evictOldSessions()
	mgr.mu.Unlock()

	if err != nil {
		t.Fatalf("evictOldSessions() error = %v", err)
	}

	if mgr.SessionCount() != 3 {
		t.Errorf("SessionCount() after eviction = %d, want 3", mgr.SessionCount())
	}

	// Oldest session should be evicted.
	if mgr.HasSession("session-1") {
		t.Error("session-1 should have been evicted")
	}

	// Newer sessions should remain.
	for _, s := range []string{"session-2", "session-3", "session-4"} {
		if !mgr.HasSession(s) {
			t.Errorf("session %s should not have been evicted", s)
		}
	}
}

func TestSessionManager_ListSessions(t *testing.T) {
	dir := t.TempDir()
	cfg := SessionManagerConfig{
		CacheDir:        dir,
		MaxSessions:     5,
		ImageCacheTTL:   time.Hour,
		ImageMaxCacheMB: 10,
	}

	mgr, err := NewSessionManager(cfg)
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	now := time.Now()
	mgr.mu.Lock()
	mgr.sessions["old"] = &SessionInfo{
		SessionID:  "old",
		LastAccess: now.Add(-1 * time.Hour),
	}
	mgr.sessions["new"] = &SessionInfo{
		SessionID:  "new",
		LastAccess: now,
	}
	mgr.mu.Unlock()

	sessions := mgr.ListSessions()
	if len(sessions) != 2 {
		t.Fatalf("ListSessions() returned %d sessions, want 2", len(sessions))
	}

	// Should be sorted by last access, newest first.
	if sessions[0].SessionID != "new" {
		t.Errorf("sessions[0].SessionID = %q, want %q", sessions[0].SessionID, "new")
	}
	if sessions[1].SessionID != "old" {
		t.Errorf("sessions[1].SessionID = %q, want %q", sessions[1].SessionID, "old")
	}
}

func TestSessionManager_Clear(t *testing.T) {
	dir := t.TempDir()
	cfg := SessionManagerConfig{
		CacheDir:        dir,
		MaxSessions:     5,
		ImageCacheTTL:   time.Hour,
		ImageMaxCacheMB: 10,
	}

	mgr, err := NewSessionManager(cfg)
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Add a session.
	mgr.mu.Lock()
	mgr.sessions["test"] = &SessionInfo{
		SessionID:  "test",
		LastAccess: time.Now(),
	}
	mgr.mu.Unlock()

	if mgr.SessionCount() != 1 {
		t.Errorf("SessionCount() = %d, want 1", mgr.SessionCount())
	}

	// Clear all sessions.
	if err := mgr.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if mgr.SessionCount() != 0 {
		t.Errorf("SessionCount() after Clear() = %d, want 0", mgr.SessionCount())
	}
}

func TestSessionManager_IndexPersistence(t *testing.T) {
	dir := t.TempDir()
	cfg := SessionManagerConfig{
		CacheDir:        dir,
		MaxSessions:     5,
		ImageCacheTTL:   time.Hour,
		ImageMaxCacheMB: 10,
	}

	// Create first manager and add a session.
	mgr1, err := NewSessionManager(cfg)
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	now := time.Now()
	mgr1.mu.Lock()
	mgr1.sessions["persistent-session"] = &SessionInfo{
		SessionID:  "persistent-session",
		Category:   "neko",
		ImageKey:   "session-persistent-session-neko",
		LastAccess: now,
		CreatedAt:  now,
	}
	if err := mgr1.saveIndex(); err != nil {
		mgr1.mu.Unlock()
		t.Fatalf("saveIndex() error = %v", err)
	}
	mgr1.mu.Unlock()

	// Create a new manager with the same cache dir.
	mgr2, err := NewSessionManager(cfg)
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	// Session should be loaded from disk.
	if !mgr2.HasSession("persistent-session") {
		t.Error("session was not persisted to disk")
	}

	info, ok := mgr2.GetSession("persistent-session")
	if !ok {
		t.Fatal("GetSession() returned false for persistent session")
	}
	if info.Category != "neko" {
		t.Errorf("info.Category = %q, want %q", info.Category, "neko")
	}
}

// mockAPIClient is a mock for testing GetOrFetch without network calls.
type mockAPIClient struct {
	fetchCount int
}

func (m *mockAPIClient) FetchImageURL(ctx context.Context, category string) (string, error) {
	m.fetchCount++
	return "https://example.com/waifu.png", nil
}

func (m *mockAPIClient) DownloadImage(ctx context.Context, imageURL string) ([]byte, string, error) {
	// Return a minimal valid PNG (1x1 pixel).
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, // IHDR length
		0x49, 0x48, 0x44, 0x52, // IHDR
		0x00, 0x00, 0x00, 0x01, // width = 1
		0x00, 0x00, 0x00, 0x01, // height = 1
		0x08, 0x02, // 8-bit depth, RGB
		0x00, 0x00, 0x00, // no compression, no filter, no interlace
		0x90, 0x77, 0x53, 0xde, // CRC
		0x00, 0x00, 0x00, 0x0c, // IDAT length
		0x49, 0x44, 0x41, 0x54, // IDAT
		0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0xff, 0x00, 0x05, 0xfe, 0x02, 0xfe, // compressed data
		0xa3, 0x6c, 0x81, 0x7a, // CRC
		0x00, 0x00, 0x00, 0x00, // IEND length
		0x49, 0x45, 0x4e, 0x44, // IEND
		0xae, 0x42, 0x60, 0x82, // CRC
	}
	return png, "image/png", nil
}

func TestDefaultMaxSessions(t *testing.T) {
	if DefaultMaxSessions != 10 {
		t.Errorf("DefaultMaxSessions = %d, want 10", DefaultMaxSessions)
	}
}

func TestSessionIndexFile(t *testing.T) {
	if SessionIndexFile != "sessions.json" {
		t.Errorf("SessionIndexFile = %q, want %q", SessionIndexFile, "sessions.json")
	}
}
