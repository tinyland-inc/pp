package waifu

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestCache creates an ImageCache using a temporary directory and the
// given TTL and max size. It registers cleanup with t.Cleanup.
func newTestCache(t *testing.T, ttl time.Duration, maxSizeMB int) *ImageCache {
	t.Helper()
	dir := t.TempDir()
	cache, err := NewImageCache(ImageCacheConfig{
		Dir:       dir,
		TTL:       ttl,
		MaxSizeMB: maxSizeMB,
	})
	if err != nil {
		t.Fatalf("NewImageCache: %v", err)
	}
	return cache
}

func TestImageCache_PutGet(t *testing.T) {
	cache := newTestCache(t, time.Hour, 50)

	data := []byte("fake-png-image-data-1234567890")
	key := "testimage"

	if err := cache.Put(key, data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, fresh, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !fresh {
		t.Error("expected fresh=true for just-written entry")
	}
	if string(got) != string(data) {
		t.Errorf("Get data = %q, want %q", got, data)
	}
}

func TestImageCache_Get_Missing(t *testing.T) {
	cache := newTestCache(t, time.Hour, 50)

	got, fresh, err := cache.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fresh {
		t.Error("expected fresh=false for missing entry")
	}
	if got != nil {
		t.Errorf("expected nil data for missing entry, got %d bytes", len(got))
	}
}

func TestImageCache_Get_Expired(t *testing.T) {
	cache := newTestCache(t, 1*time.Millisecond, 50)

	data := []byte("expiring-image")
	key := "expiring"

	if err := cache.Put(key, data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Wait for expiration.
	time.Sleep(10 * time.Millisecond)

	got, fresh, err := cache.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fresh {
		t.Error("expected fresh=false for expired entry")
	}
	if got != nil {
		t.Errorf("expected nil data for expired entry, got %d bytes", len(got))
	}

	// Verify the file was removed.
	path := cache.keyPath(key)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected expired file to be removed from disk")
	}
}

func TestImageCache_Has_Fresh(t *testing.T) {
	cache := newTestCache(t, time.Hour, 50)

	key := "freshkey"
	if err := cache.Put(key, []byte("data")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if !cache.Has(key) {
		t.Error("Has should return true for a fresh entry")
	}
}

func TestImageCache_Has_Expired(t *testing.T) {
	cache := newTestCache(t, 1*time.Millisecond, 50)

	key := "expkey"
	if err := cache.Put(key, []byte("data")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if cache.Has(key) {
		t.Error("Has should return false for an expired entry")
	}
}

func TestImageCache_Evict(t *testing.T) {
	// MaxSizeMB = 1 byte effectively (we use a tiny limit via direct byte check).
	// Create a cache with 1MB max and put files that exceed it.
	dir := t.TempDir()
	cache, err := NewImageCache(ImageCacheConfig{
		Dir:       dir,
		TTL:       time.Hour,
		MaxSizeMB: 1, // 1MB limit
	})
	if err != nil {
		t.Fatalf("NewImageCache: %v", err)
	}

	// Write files that collectively exceed 1MB.
	// Each file is ~600KB, so two should trigger eviction.
	bigData := make([]byte, 600*1024)
	for i := range bigData {
		bigData[i] = byte(i % 256)
	}

	// Write first file with an old mtime.
	if err := cache.Put("old", bigData); err != nil {
		t.Fatalf("Put old: %v", err)
	}
	oldPath := cache.keyPath("old")
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// Write second file (newer). This Put should trigger eviction of "old".
	if err := cache.Put("new", bigData); err != nil {
		t.Fatalf("Put new: %v", err)
	}

	// The old file should have been evicted.
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("expected old file to be evicted")
	}

	// The new file should still exist.
	newPath := cache.keyPath("new")
	if _, err := os.Stat(newPath); err != nil {
		t.Error("expected new file to survive eviction")
	}
}

func TestImageCache_Evict_PreservesNewest(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewImageCache(ImageCacheConfig{
		Dir:       dir,
		TTL:       time.Hour,
		MaxSizeMB: 1, // 1MB limit
	})
	if err != nil {
		t.Fatalf("NewImageCache: %v", err)
	}

	data := make([]byte, 400*1024) // 400KB each

	// Write three files with decreasing ages.
	keys := []string{"oldest", "middle", "newest"}
	for i, key := range keys {
		if err := cache.Put(key, data); err != nil {
			t.Fatalf("Put %s: %v", key, err)
		}
		// Set mtime: oldest = -3h, middle = -2h, newest = -1h
		age := time.Duration(3-i) * time.Hour
		mtime := time.Now().Add(-age)
		if err := os.Chtimes(cache.keyPath(key), mtime, mtime); err != nil {
			t.Fatalf("Chtimes %s: %v", key, err)
		}
	}

	// Trigger eviction explicitly.
	if err := cache.Evict(); err != nil {
		t.Fatalf("Evict: %v", err)
	}

	// Newest should survive; at least one older file should be evicted.
	if _, err := os.Stat(cache.keyPath("newest")); err != nil {
		t.Error("expected newest file to survive eviction")
	}

	// Oldest should be gone (it was the first to be evicted).
	if _, err := os.Stat(cache.keyPath("oldest")); !os.IsNotExist(err) {
		t.Error("expected oldest file to be evicted")
	}
}

func TestImageCache_Clean(t *testing.T) {
	cache := newTestCache(t, 50*time.Millisecond, 50)

	// Write a fresh file and an expired file.
	if err := cache.Put("fresh", []byte("fresh-data")); err != nil {
		t.Fatalf("Put fresh: %v", err)
	}

	if err := cache.Put("stale", []byte("stale-data")); err != nil {
		t.Fatalf("Put stale: %v", err)
	}
	// Make "stale" old.
	stalePath := cache.keyPath("stale")
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(stalePath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	if err := cache.Clean(); err != nil {
		t.Fatalf("Clean: %v", err)
	}

	// Fresh file should remain.
	if _, err := os.Stat(cache.keyPath("fresh")); err != nil {
		t.Error("expected fresh file to survive Clean")
	}

	// Stale file should be removed.
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Error("expected stale file to be removed by Clean")
	}
}

func TestImageCache_Clear(t *testing.T) {
	cache := newTestCache(t, time.Hour, 50)

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("img%d", i)
		if err := cache.Put(key, []byte("data")); err != nil {
			t.Fatalf("Put %s: %v", key, err)
		}
	}

	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	// Verify all files are gone.
	entries, err := os.ReadDir(cache.config.Dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 files after Clear, got %d", len(entries))
	}
}

func TestImageCache_Stats(t *testing.T) {
	cache := newTestCache(t, time.Hour, 50)

	data1 := []byte("image-data-one")
	data2 := []byte("image-data-two-longer")

	if err := cache.Put("img1", data1); err != nil {
		t.Fatalf("Put img1: %v", err)
	}

	// Give the second file a distinct mtime.
	time.Sleep(10 * time.Millisecond)

	if err := cache.Put("img2", data2); err != nil {
		t.Fatalf("Put img2: %v", err)
	}

	stats, err := cache.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	if stats.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", stats.FileCount)
	}

	expectedSize := int64(len(data1) + len(data2))
	if stats.TotalBytes != expectedSize {
		t.Errorf("TotalBytes = %d, want %d", stats.TotalBytes, expectedSize)
	}

	if stats.TotalMB <= 0 {
		t.Error("TotalMB should be positive")
	}

	if stats.OldestFile.IsZero() {
		t.Error("OldestFile should not be zero")
	}

	if stats.NewestFile.IsZero() {
		t.Error("NewestFile should not be zero")
	}

	if stats.NewestFile.Before(stats.OldestFile) {
		t.Error("NewestFile should not be before OldestFile")
	}
}

func TestImageCache_Stats_Empty(t *testing.T) {
	cache := newTestCache(t, time.Hour, 50)

	stats, err := cache.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	if stats.FileCount != 0 {
		t.Errorf("FileCount = %d, want 0", stats.FileCount)
	}
	if stats.TotalBytes != 0 {
		t.Errorf("TotalBytes = %d, want 0", stats.TotalBytes)
	}
	if stats.TotalMB != 0 {
		t.Errorf("TotalMB = %f, want 0", stats.TotalMB)
	}
	if !stats.OldestFile.IsZero() {
		t.Error("OldestFile should be zero for empty cache")
	}
	if !stats.NewestFile.IsZero() {
		t.Error("NewestFile should be zero for empty cache")
	}
}

func TestImageCache_AtomicWrite(t *testing.T) {
	cache := newTestCache(t, time.Hour, 50)

	data := []byte("atomic-write-test-data")
	key := "atomickey"

	if err := cache.Put(key, data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Verify the final file exists.
	path := cache.keyPath(key)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}

	// Verify no temp files remain.
	entries, err := os.ReadDir(cache.config.Dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}

	// Verify the file has 0600 permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestImageCache_ConcurrentAccess(t *testing.T) {
	cache := newTestCache(t, time.Hour, 50)

	const goroutines = 10
	var wg sync.WaitGroup

	// Concurrent writes.
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent-%d", idx)
			data := []byte(fmt.Sprintf("data-%d", idx))
			if err := cache.Put(key, data); err != nil {
				t.Errorf("concurrent Put %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent reads.
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent-%d", idx)
			data, fresh, err := cache.Get(key)
			if err != nil {
				t.Errorf("concurrent Get %d: %v", idx, err)
				return
			}
			if !fresh {
				t.Errorf("concurrent Get %d: expected fresh", idx)
			}
			expected := fmt.Sprintf("data-%d", idx)
			if string(data) != expected {
				t.Errorf("concurrent Get %d = %q, want %q", idx, data, expected)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent Has checks.
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent-%d", idx)
			if !cache.Has(key) {
				t.Errorf("concurrent Has %d: expected true", idx)
			}
		}(i)
	}
	wg.Wait()
}

func TestKeyFromURL_Deterministic(t *testing.T) {
	url := "https://i.waifu.pics/example-image.png"
	key1 := KeyFromURL(url)
	key2 := KeyFromURL(url)

	if key1 != key2 {
		t.Errorf("KeyFromURL not deterministic: %q != %q", key1, key2)
	}
}

func TestKeyFromURL_Different(t *testing.T) {
	url1 := "https://i.waifu.pics/image-a.png"
	url2 := "https://i.waifu.pics/image-b.png"

	key1 := KeyFromURL(url1)
	key2 := KeyFromURL(url2)

	if key1 == key2 {
		t.Errorf("different URLs produced same key: %q", key1)
	}
}

func TestKeyFromURL_SafeChars(t *testing.T) {
	urls := []string{
		"https://i.waifu.pics/some-image.png",
		"https://example.com/path/to/file?query=1&other=2",
		"https://i.waifu.pics/../../../etc/passwd",
		"https://example.com/image with spaces.png",
	}

	hexPattern := regexp.MustCompile(`^[0-9a-f]{16}$`)

	for _, url := range urls {
		key := KeyFromURL(url)
		if !hexPattern.MatchString(key) {
			t.Errorf("KeyFromURL(%q) = %q, want 16 hex chars", url, key)
		}
	}
}

func TestNewImageCache_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "cache", "dir")

	_, err := NewImageCache(ImageCacheConfig{
		Dir:       dir,
		TTL:       time.Hour,
		MaxSizeMB: 50,
	})
	if err != nil {
		t.Fatalf("NewImageCache: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("expected directory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected path to be a directory")
	}
}

func TestNewImageCache_InvalidDir(t *testing.T) {
	// Use /dev/null as a file that cannot be a directory.
	impossiblePath := "/dev/null/impossible/path"

	_, err := NewImageCache(ImageCacheConfig{
		Dir:       impossiblePath,
		TTL:       time.Hour,
		MaxSizeMB: 50,
	})
	if err == nil {
		t.Fatal("expected error for impossible directory path")
	}
}

func TestDefaultImageCacheConfig(t *testing.T) {
	cfg := DefaultImageCacheConfig()

	if cfg.TTL != 24*time.Hour {
		t.Errorf("TTL = %v, want 24h", cfg.TTL)
	}

	if cfg.MaxSizeMB != 50 {
		t.Errorf("MaxSizeMB = %d, want 50", cfg.MaxSizeMB)
	}

	if cfg.Dir == "" {
		t.Error("Dir should not be empty")
	}

	if !strings.Contains(cfg.Dir, "prompt-pulse") {
		t.Errorf("Dir = %q, expected to contain 'prompt-pulse'", cfg.Dir)
	}

	if !strings.HasSuffix(cfg.Dir, "waifu") {
		t.Errorf("Dir = %q, expected to end with 'waifu'", cfg.Dir)
	}

	if cfg.Logger == nil {
		t.Error("Logger should not be nil")
	}
}

// --- RenderedCache tests ---

func TestRenderedCache_PutGet(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	sessionID := "test-session"
	protocol := "kitty"
	cols, rows := 40, 20
	output := "\033_Gf=100,a=T,t=d,c=40,r=20,m=0;base64data\033\\"

	cache.Put(sessionID, protocol, cols, rows, output)

	entry, exists := cache.Get(sessionID, protocol, cols, rows)
	if !exists {
		t.Fatal("expected entry to exist")
	}
	if entry.Output != output {
		t.Errorf("Output = %q, want %q", entry.Output, output)
	}
	if entry.Protocol != protocol {
		t.Errorf("Protocol = %q, want %q", entry.Protocol, protocol)
	}
	if entry.Cols != cols {
		t.Errorf("Cols = %d, want %d", entry.Cols, cols)
	}
	if entry.Rows != rows {
		t.Errorf("Rows = %d, want %d", entry.Rows, rows)
	}
}

func TestRenderedCache_Get_Missing(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	entry, exists := cache.Get("nonexistent", "kitty", 40, 20)
	if exists {
		t.Error("expected entry not to exist")
	}
	if entry != nil {
		t.Error("expected nil entry for missing key")
	}
}

func TestRenderedCache_Has(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	if cache.Has("test", "kitty", 40, 20) {
		t.Error("Has should return false for non-existent entry")
	}

	cache.Put("test", "kitty", 40, 20, "output")

	if !cache.Has("test", "kitty", 40, 20) {
		t.Error("Has should return true for existing entry")
	}
}

func TestRenderedCache_DifferentDimensions(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	// Same session and protocol, different dimensions should be different entries
	cache.Put("session", "kitty", 40, 20, "output-40x20")
	cache.Put("session", "kitty", 80, 40, "output-80x40")

	entry1, exists1 := cache.Get("session", "kitty", 40, 20)
	if !exists1 || entry1.Output != "output-40x20" {
		t.Error("40x20 entry not found or incorrect")
	}

	entry2, exists2 := cache.Get("session", "kitty", 80, 40)
	if !exists2 || entry2.Output != "output-80x40" {
		t.Error("80x40 entry not found or incorrect")
	}
}

func TestRenderedCache_DifferentProtocols(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	// Same session and dimensions, different protocols should be different entries
	cache.Put("session", "kitty", 40, 20, "kitty-output")
	cache.Put("session", "unicode", 40, 20, "unicode-output")

	entry1, exists1 := cache.Get("session", "kitty", 40, 20)
	if !exists1 || entry1.Output != "kitty-output" {
		t.Error("kitty entry not found or incorrect")
	}

	entry2, exists2 := cache.Get("session", "unicode", 40, 20)
	if !exists2 || entry2.Output != "unicode-output" {
		t.Error("unicode entry not found or incorrect")
	}
}

func TestRenderedCache_LRUEviction(t *testing.T) {
	cfg := DefaultRenderedCacheConfig()
	cfg.MaxEntries = 3
	cache := NewRenderedCache(cfg)

	// Add 3 entries
	cache.Put("s1", "kitty", 40, 20, "output1")
	cache.Put("s2", "kitty", 40, 20, "output2")
	cache.Put("s3", "kitty", 40, 20, "output3")

	// Access s1 to make it recently used
	cache.Get("s1", "kitty", 40, 20)

	// Add 4th entry, should evict s2 (oldest non-accessed)
	cache.Put("s4", "kitty", 40, 20, "output4")

	if !cache.Has("s1", "kitty", 40, 20) {
		t.Error("s1 should survive (was accessed)")
	}
	if cache.Has("s2", "kitty", 40, 20) {
		t.Error("s2 should be evicted (oldest)")
	}
	if !cache.Has("s3", "kitty", 40, 20) {
		t.Error("s3 should survive")
	}
	if !cache.Has("s4", "kitty", 40, 20) {
		t.Error("s4 should exist (just added)")
	}
}

func TestRenderedCache_Invalidate(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	cache.Put("session", "kitty", 40, 20, "output")
	if !cache.Has("session", "kitty", 40, 20) {
		t.Fatal("entry should exist before invalidation")
	}

	cache.Invalidate("session", "kitty", 40, 20)
	if cache.Has("session", "kitty", 40, 20) {
		t.Error("entry should not exist after invalidation")
	}
}

func TestRenderedCache_InvalidateSession(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	// Add entries for session1 with different configs
	cache.Put("session1", "kitty", 40, 20, "output1")
	cache.Put("session1", "unicode", 40, 20, "output2")
	cache.Put("session1", "kitty", 80, 40, "output3")

	// Add entry for session2
	cache.Put("session2", "kitty", 40, 20, "output4")

	// Invalidate session1
	cache.InvalidateSession("session1")

	// session1 entries should be gone
	if cache.Has("session1", "kitty", 40, 20) {
		t.Error("session1 kitty 40x20 should be invalidated")
	}
	if cache.Has("session1", "unicode", 40, 20) {
		t.Error("session1 unicode 40x20 should be invalidated")
	}
	if cache.Has("session1", "kitty", 80, 40) {
		t.Error("session1 kitty 80x40 should be invalidated")
	}

	// session2 should survive
	if !cache.Has("session2", "kitty", 40, 20) {
		t.Error("session2 should survive invalidation of session1")
	}
}

func TestRenderedCache_Clear(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	cache.Put("s1", "kitty", 40, 20, "output1")
	cache.Put("s2", "unicode", 40, 20, "output2")

	cache.Clear()

	if cache.Has("s1", "kitty", 40, 20) {
		t.Error("s1 should be cleared")
	}
	if cache.Has("s2", "unicode", 40, 20) {
		t.Error("s2 should be cleared")
	}

	count, _ := cache.Stats()
	if count != 0 {
		t.Errorf("entry count = %d, want 0 after clear", count)
	}
}

func TestRenderedCache_Stats(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	output1 := "short"
	output2 := "much longer output string for testing"

	cache.Put("s1", "kitty", 40, 20, output1)
	cache.Put("s2", "unicode", 40, 20, output2)

	count, totalBytes := cache.Stats()
	if count != 2 {
		t.Errorf("entry count = %d, want 2", count)
	}

	expectedBytes := int64(len(output1) + len(output2))
	if totalBytes != expectedBytes {
		t.Errorf("total bytes = %d, want %d", totalBytes, expectedBytes)
	}
}

func TestRenderedCache_UpdateExisting(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	cache.Put("session", "kitty", 40, 20, "original")
	cache.Put("session", "kitty", 40, 20, "updated")

	entry, exists := cache.Get("session", "kitty", 40, 20)
	if !exists {
		t.Fatal("entry should exist")
	}
	if entry.Output != "updated" {
		t.Errorf("Output = %q, want 'updated'", entry.Output)
	}

	// Should still be only 1 entry
	count, _ := cache.Stats()
	if count != 1 {
		t.Errorf("entry count = %d, want 1 (no duplicates)", count)
	}
}

func TestRenderedCache_ConcurrentAccess(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())

	const goroutines = 10
	var wg sync.WaitGroup

	// Concurrent writes
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("session-%d", idx)
			cache.Put(sessionID, "kitty", 40, 20, fmt.Sprintf("output-%d", idx))
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("session-%d", idx)
			entry, exists := cache.Get(sessionID, "kitty", 40, 20)
			if !exists {
				t.Errorf("session-%d should exist", idx)
				return
			}
			expected := fmt.Sprintf("output-%d", idx)
			if entry.Output != expected {
				t.Errorf("session-%d output = %q, want %q", idx, entry.Output, expected)
			}
		}(i)
	}
	wg.Wait()
}

func TestDefaultRenderedCacheConfig(t *testing.T) {
	cfg := DefaultRenderedCacheConfig()

	if cfg.MaxEntries != 50 {
		t.Errorf("MaxEntries = %d, want 50", cfg.MaxEntries)
	}

	if cfg.Logger == nil {
		t.Error("Logger should not be nil")
	}
}
