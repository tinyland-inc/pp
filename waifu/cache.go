package waifu

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ImageCacheConfig holds settings for the image cache.
type ImageCacheConfig struct {
	// Dir is the directory for cached images.
	Dir string
	// TTL is how long cached images remain valid.
	TTL time.Duration
	// MaxSizeMB is the maximum total cache size in megabytes.
	MaxSizeMB int
	// Logger for cache operations.
	Logger *slog.Logger
}

// DefaultImageCacheConfig returns an ImageCacheConfig with sensible defaults.
func DefaultImageCacheConfig() ImageCacheConfig {
	home, _ := os.UserHomeDir()
	return ImageCacheConfig{
		Dir:       filepath.Join(home, ".cache", "prompt-pulse", "waifu"),
		TTL:       24 * time.Hour,
		MaxSizeMB: 50,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// CacheStats holds statistics about the image cache.
type CacheStats struct {
	FileCount  int
	TotalBytes int64
	TotalMB    float64
	OldestFile time.Time
	NewestFile time.Time
}

// ImageCache provides a file-based image cache with TTL expiration and
// size-based eviction. It stores images as {Dir}/{key}.img files and uses
// file modification times for TTL checks and LRU-like eviction.
type ImageCache struct {
	config ImageCacheConfig
	mu     sync.Mutex
}

// NewImageCache creates an ImageCache and ensures the cache directory exists.
// Returns an error if the directory cannot be created. If cfg.Logger is nil,
// a no-op logger is used.
func NewImageCache(cfg ImageCacheConfig) (*ImageCache, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if err := os.MkdirAll(cfg.Dir, 0700); err != nil {
		return nil, fmt.Errorf("image cache: create directory %s: %w", cfg.Dir, err)
	}
	return &ImageCache{config: cfg}, nil
}

// keyPath returns the filesystem path for a cache key.
func (c *ImageCache) keyPath(key string) string {
	return filepath.Join(c.config.Dir, key+".img")
}

// Get reads an image from the cache. It returns (data, fresh, error).
// If the key does not exist, it returns (nil, false, nil).
// If the key exists but is expired (past TTL), it removes the file and
// returns (nil, false, nil).
func (c *ImageCache) Get(key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.keyPath(key)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("image cache: stat %s: %w", key, err)
	}

	// Check TTL based on modification time.
	if time.Since(info.ModTime()) >= c.config.TTL {
		c.config.Logger.Debug("image cache: removing expired entry", slog.String("key", key))
		_ = os.Remove(path)
		return nil, false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("image cache: read %s: %w", key, err)
	}

	return data, true, nil
}

// Put writes image data to the cache using an atomic write (temp file + rename).
// After writing, it checks total cache size and evicts oldest files if the
// cache exceeds MaxSizeMB.
func (c *ImageCache) Put(key string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.keyPath(key)

	tmp, err := os.CreateTemp(c.config.Dir, ".tmp-"+key+"-*.img")
	if err != nil {
		return fmt.Errorf("image cache: create temp for %s: %w", key, err)
	}
	tmpName := tmp.Name()

	// Clean up temp file on any failure path.
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	if err := os.Chmod(tmpName, 0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("image cache: chmod temp for %s: %w", key, err)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("image cache: write temp for %s: %w", key, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("image cache: close temp for %s: %w", key, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("image cache: rename temp for %s: %w", key, err)
	}

	success = true

	// Check cache size and evict if necessary.
	if err := c.evictLocked(); err != nil {
		c.config.Logger.Warn("image cache: eviction error", slog.String("error", err.Error()))
	}

	return nil
}

// Has reports whether the given key exists in the cache and is still fresh
// (within TTL).
func (c *ImageCache) Has(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	path := c.keyPath(key)

	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return time.Since(info.ModTime()) < c.config.TTL
}

// Evict removes the oldest files (by modification time) until the total
// cache size is under MaxSizeMB. This provides LRU-like behavior.
func (c *ImageCache) Evict() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictLocked()
}

// cacheFileInfo holds info about a single cached file for sorting.
type cacheFileInfo struct {
	path    string
	size    int64
	modTime time.Time
}

// evictLocked performs eviction without acquiring the mutex. The caller
// must hold c.mu.
func (c *ImageCache) evictLocked() error {
	if c.config.MaxSizeMB <= 0 {
		return nil
	}

	maxBytes := int64(c.config.MaxSizeMB) * 1024 * 1024

	files, totalSize, err := c.scanFiles()
	if err != nil {
		return err
	}

	if totalSize <= maxBytes {
		return nil
	}

	// Sort oldest first (ascending modification time).
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	for _, f := range files {
		if totalSize <= maxBytes {
			break
		}
		c.config.Logger.Debug("image cache: evicting file",
			slog.String("path", filepath.Base(f.path)),
			slog.Int64("size", f.size),
		)
		if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("image cache: evict remove %s: %w", filepath.Base(f.path), err)
		}
		totalSize -= f.size
	}

	return nil
}

// Clean removes all expired files (past TTL) from the cache directory.
func (c *ImageCache) Clean() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.config.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("image cache: clean read dir: %w", err)
	}

	now := time.Now()

	for _, e := range entries {
		if e.IsDir() || !isImageFile(e.Name()) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) >= c.config.TTL {
			path := filepath.Join(c.config.Dir, e.Name())
			c.config.Logger.Debug("image cache: cleaning expired file", slog.String("name", e.Name()))
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("image cache: clean remove %s: %w", e.Name(), err)
			}
		}
	}

	return nil
}

// Stats returns statistics about the current cache contents.
func (c *ImageCache) Stats() (CacheStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var stats CacheStats

	files, totalSize, err := c.scanFiles()
	if err != nil {
		return stats, err
	}

	stats.FileCount = len(files)
	stats.TotalBytes = totalSize
	stats.TotalMB = float64(totalSize) / (1024 * 1024)

	for _, f := range files {
		if stats.OldestFile.IsZero() || f.modTime.Before(stats.OldestFile) {
			stats.OldestFile = f.modTime
		}
		if stats.NewestFile.IsZero() || f.modTime.After(stats.NewestFile) {
			stats.NewestFile = f.modTime
		}
	}

	return stats, nil
}

// Clear removes all cached images from the cache directory.
func (c *ImageCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.config.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("image cache: clear read dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(c.config.Dir, e.Name())
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("image cache: clear remove %s: %w", e.Name(), err)
		}
	}

	return nil
}

// scanFiles reads the cache directory and returns info for all .img files,
// along with the total size. Temp files and directories are skipped.
func (c *ImageCache) scanFiles() ([]cacheFileInfo, int64, error) {
	entries, err := os.ReadDir(c.config.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("image cache: scan read dir: %w", err)
	}

	var files []cacheFileInfo
	var totalSize int64

	for _, e := range entries {
		if e.IsDir() || !isImageFile(e.Name()) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		files = append(files, cacheFileInfo{
			path:    filepath.Join(c.config.Dir, e.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		totalSize += info.Size()
	}

	return files, totalSize, nil
}

// isImageFile reports whether the filename is a cached image file
// (has .img extension and is not a temp file).
func isImageFile(name string) bool {
	return filepath.Ext(name) == ".img" && len(name) > 4 && name[0] != '.'
}

// KeyFromURL generates a safe filesystem key from an image URL.
// It uses a SHA256 hash of the URL, hex-encoded and truncated to 16 characters.
// This prevents path traversal and keeps filenames short.
func KeyFromURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:])[:16]
}

// RenderedCacheConfig holds settings for the rendered output cache.
type RenderedCacheConfig struct {
	// MaxEntries is the maximum number of rendered outputs to cache in memory.
	MaxEntries int
	// Logger for cache operations.
	Logger *slog.Logger
}

// DefaultRenderedCacheConfig returns sensible defaults for rendered cache.
func DefaultRenderedCacheConfig() RenderedCacheConfig {
	return RenderedCacheConfig{
		MaxEntries: 50,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// RenderedEntry holds a cached rendered output with its dimensions.
type RenderedEntry struct {
	// Output is the terminal escape sequence string.
	Output string
	// Protocol is the protocol used for rendering.
	Protocol string
	// Cols is the column count used for rendering.
	Cols int
	// Rows is the row count used for rendering.
	Rows int
	// CreatedAt is when the entry was created.
	CreatedAt time.Time
	// AccessedAt is when the entry was last accessed.
	AccessedAt time.Time
}

// RenderedCache provides an in-memory cache for pre-rendered terminal output.
// It caches rendered escape sequences keyed by session ID, protocol, and dimensions.
// This avoids re-rendering the same image data for the same terminal configuration.
type RenderedCache struct {
	config  RenderedCacheConfig
	entries map[string]*RenderedEntry
	order   []string // LRU order (oldest first)
	mu      sync.Mutex
}

// NewRenderedCache creates a RenderedCache with the given configuration.
func NewRenderedCache(cfg RenderedCacheConfig) *RenderedCache {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 50
	}
	return &RenderedCache{
		config:  cfg,
		entries: make(map[string]*RenderedEntry),
		order:   make([]string, 0, cfg.MaxEntries),
	}
}

// renderedCacheKey generates a cache key for rendered output.
// Format: "{sessionID}-{protocol}-{cols}x{rows}"
func renderedCacheKey(sessionID, protocol string, cols, rows int) string {
	return fmt.Sprintf("%s-%s-%dx%d", sessionID, protocol, cols, rows)
}

// Get retrieves a cached rendered output. Returns (entry, exists).
// If found, the entry's AccessedAt is updated and it's moved to the back of the LRU list.
func (c *RenderedCache) Get(sessionID, protocol string, cols, rows int) (*RenderedEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := renderedCacheKey(sessionID, protocol, cols, rows)
	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Update access time and LRU order
	entry.AccessedAt = time.Now()
	c.moveToBack(key)

	c.config.Logger.Debug("rendered cache hit",
		slog.String("session", sessionID),
		slog.String("protocol", protocol),
		slog.Int("cols", cols),
		slog.Int("rows", rows),
	)

	return entry, true
}

// Put stores a rendered output in the cache.
// If the cache is full, the least recently used entry is evicted.
func (c *RenderedCache) Put(sessionID, protocol string, cols, rows int, output string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := renderedCacheKey(sessionID, protocol, cols, rows)
	now := time.Now()

	// Check if entry already exists
	if existing, exists := c.entries[key]; exists {
		existing.Output = output
		existing.AccessedAt = now
		c.moveToBack(key)
		return
	}

	// Evict oldest if at capacity
	if len(c.entries) >= c.config.MaxEntries && len(c.order) > 0 {
		oldestKey := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldestKey)
		c.config.Logger.Debug("rendered cache eviction", slog.String("key", oldestKey))
	}

	// Add new entry
	c.entries[key] = &RenderedEntry{
		Output:     output,
		Protocol:   protocol,
		Cols:       cols,
		Rows:       rows,
		CreatedAt:  now,
		AccessedAt: now,
	}
	c.order = append(c.order, key)

	c.config.Logger.Debug("rendered cache put",
		slog.String("session", sessionID),
		slog.String("protocol", protocol),
		slog.Int("cols", cols),
		slog.Int("rows", rows),
		slog.Int("output_len", len(output)),
	)
}

// Has reports whether a rendered output exists in the cache.
func (c *RenderedCache) Has(sessionID, protocol string, cols, rows int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := renderedCacheKey(sessionID, protocol, cols, rows)
	_, exists := c.entries[key]
	return exists
}

// Invalidate removes a specific rendered output from the cache.
func (c *RenderedCache) Invalidate(sessionID, protocol string, cols, rows int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := renderedCacheKey(sessionID, protocol, cols, rows)
	c.removeKey(key)
}

// InvalidateSession removes all rendered outputs for a session.
func (c *RenderedCache) InvalidateSession(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := sessionID + "-"
	keysToRemove := make([]string, 0)

	for key := range c.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keysToRemove = append(keysToRemove, key)
		}
	}

	for _, key := range keysToRemove {
		c.removeKey(key)
	}

	c.config.Logger.Debug("rendered cache session invalidated",
		slog.String("session", sessionID),
		slog.Int("removed", len(keysToRemove)),
	)
}

// Clear removes all entries from the cache.
func (c *RenderedCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*RenderedEntry)
	c.order = make([]string, 0, c.config.MaxEntries)
}

// Stats returns statistics about the rendered cache.
func (c *RenderedCache) Stats() (entryCount int, totalBytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entryCount = len(c.entries)
	for _, entry := range c.entries {
		totalBytes += int64(len(entry.Output))
	}
	return entryCount, totalBytes
}

// moveToBack moves a key to the back of the LRU order list.
// Must be called with c.mu held.
func (c *RenderedCache) moveToBack(key string) {
	// Find and remove key from current position
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	// Add to back
	c.order = append(c.order, key)
}

// removeKey removes a key from the cache.
// Must be called with c.mu held.
func (c *RenderedCache) removeKey(key string) {
	delete(c.entries, key)
	// Remove from order list
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}
