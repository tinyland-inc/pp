// Package waifu provides session-aware waifu image caching with LRU eviction.
package waifu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	// DefaultMaxSessions is the default number of sessions to track before LRU eviction.
	DefaultMaxSessions = 10
	// SessionIndexFile is the name of the file that stores session metadata.
	SessionIndexFile = "sessions.json"
)

// SessionInfo holds metadata about a shell session and its associated waifu image.
type SessionInfo struct {
	// SessionID is the unique identifier for this shell session.
	SessionID string `json:"session_id"`
	// Category is the waifu category used for this session.
	Category string `json:"category"`
	// ImageKey is the cache key for the image file.
	ImageKey string `json:"image_key"`
	// LastAccess is when this session last displayed the banner.
	LastAccess time.Time `json:"last_access"`
	// CreatedAt is when this session was first created.
	CreatedAt time.Time `json:"created_at"`
}

// SessionManagerConfig holds settings for the session manager.
type SessionManagerConfig struct {
	// CacheDir is the base cache directory (sessions stored in CacheDir/sessions/).
	CacheDir string
	// MaxSessions is the maximum number of sessions to keep.
	// Older sessions are evicted when this limit is exceeded.
	MaxSessions int
	// ImageCacheTTL is how long session images remain valid.
	ImageCacheTTL time.Duration
	// ImageMaxCacheMB is the max size of the image cache.
	ImageMaxCacheMB int
	// Logger for session operations.
	Logger *slog.Logger
}

// DefaultSessionManagerConfig returns sensible defaults.
func DefaultSessionManagerConfig() SessionManagerConfig {
	home, _ := os.UserHomeDir()
	return SessionManagerConfig{
		CacheDir:        filepath.Join(home, ".cache", "prompt-pulse", "waifu"),
		MaxSessions:     DefaultMaxSessions,
		ImageCacheTTL:   24 * time.Hour,
		ImageMaxCacheMB: 50,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// SessionManager manages per-shell-session waifu images with LRU eviction.
// Each shell session gets its own unique image that persists for the
// lifetime of the session. When new sessions exceed MaxSessions,
// the least recently accessed sessions are evicted.
type SessionManager struct {
	config     SessionManagerConfig
	sessions   map[string]*SessionInfo
	imageCache *ImageCache
	mu         sync.Mutex
}

// NewSessionManager creates a SessionManager and loads any existing session state.
func NewSessionManager(cfg SessionManagerConfig) (*SessionManager, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	sessionsDir := filepath.Join(cfg.CacheDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0700); err != nil {
		return nil, fmt.Errorf("session manager: create sessions dir: %w", err)
	}

	imageCache, err := NewImageCache(ImageCacheConfig{
		Dir:       sessionsDir,
		TTL:       cfg.ImageCacheTTL,
		MaxSizeMB: cfg.ImageMaxCacheMB,
		Logger:    cfg.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("session manager: create image cache: %w", err)
	}

	m := &SessionManager{
		config:     cfg,
		sessions:   make(map[string]*SessionInfo),
		imageCache: imageCache,
	}

	// Load existing session state.
	if err := m.loadIndex(); err != nil {
		cfg.Logger.Warn("session manager: failed to load index, starting fresh",
			slog.String("error", err.Error()))
		m.sessions = make(map[string]*SessionInfo)
	}

	return m, nil
}

// GetSessionKey generates a unique session key from environment variables.
// The key is based on PPULSE_SESSION_ID if set, otherwise falls back to
// combining PID and timestamp. This allows the shell integration to provide
// a stable session ID that survives subshell invocations.
func GetSessionKey() string {
	// Check for explicit session ID from shell integration.
	if sid := os.Getenv("PPULSE_SESSION_ID"); sid != "" {
		return sid
	}

	// Fallback: generate from parent PID and time.
	// This won't be stable across invocations but provides uniqueness.
	ppid := os.Getppid()
	return fmt.Sprintf("%d-%d", ppid, time.Now().Unix())
}

// GetOrFetch retrieves the cached image for this session, or fetches a new one.
// If the session already has an image, it's returned directly (with updated access time).
// If not, a new image is fetched from the API and cached for this session.
// LRU eviction is performed if the session count exceeds MaxSessions.
func (m *SessionManager) GetOrFetch(ctx context.Context, sessionID, category string, api *APIClient, processCfg ProcessConfig) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if session exists and has valid cached image.
	if info, exists := m.sessions[sessionID]; exists {
		// Update last access time.
		info.LastAccess = time.Now()

		// Check if we have cached image data.
		if data, fresh, err := m.imageCache.Get(info.ImageKey); err == nil && fresh && data != nil {
			m.config.Logger.Debug("session: returning cached image",
				slog.String("session_id", sessionID),
				slog.String("category", info.Category),
			)
			// Save updated access time.
			if err := m.saveIndex(); err != nil {
				m.config.Logger.Warn("session: failed to save index", slog.String("error", err.Error()))
			}
			return data, nil
		}
	}

	// Need to fetch a new image for this session.
	m.config.Logger.Debug("session: fetching new image",
		slog.String("session_id", sessionID),
		slog.String("category", category),
	)

	// Fetch image URL.
	imageURL, err := api.FetchImageURL(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("session: fetch URL: %w", err)
	}

	// Download image.
	data, _, err := api.DownloadImage(ctx, imageURL)
	if err != nil {
		return nil, fmt.Errorf("session: download image: %w", err)
	}

	// Process image.
	processed, err := ProcessImage(data, processCfg)
	if err != nil {
		return nil, fmt.Errorf("session: process image: %w", err)
	}

	// Generate unique cache key for this session's image.
	imageKey := fmt.Sprintf("session-%s-%s", sessionID, category)

	// Store in cache.
	if err := m.imageCache.Put(imageKey, processed); err != nil {
		return nil, fmt.Errorf("session: cache image: %w", err)
	}

	// Update or create session info.
	now := time.Now()
	m.sessions[sessionID] = &SessionInfo{
		SessionID:  sessionID,
		Category:   category,
		ImageKey:   imageKey,
		LastAccess: now,
		CreatedAt:  now,
	}

	// Perform LRU eviction if needed.
	if err := m.evictOldSessions(); err != nil {
		m.config.Logger.Warn("session: eviction error", slog.String("error", err.Error()))
	}

	// Save session state.
	if err := m.saveIndex(); err != nil {
		m.config.Logger.Warn("session: failed to save index", slog.String("error", err.Error()))
	}

	return processed, nil
}

// HasSession reports whether a session with the given ID exists.
func (m *SessionManager) HasSession(sessionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.sessions[sessionID]
	return exists
}

// GetSession returns the SessionInfo for a given session ID, if it exists.
func (m *SessionManager) GetSession(sessionID string) (*SessionInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	info, exists := m.sessions[sessionID]
	if !exists {
		return nil, false
	}
	// Return a copy to avoid data races.
	infoCopy := *info
	return &infoCopy, true
}

// SessionCount returns the number of tracked sessions.
func (m *SessionManager) SessionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// ListSessions returns a slice of all session infos, sorted by last access time (newest first).
func (m *SessionManager) ListSessions() []SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions := make([]SessionInfo, 0, len(m.sessions))
	for _, info := range m.sessions {
		sessions = append(sessions, *info)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastAccess.After(sessions[j].LastAccess)
	})

	return sessions
}

// evictOldSessions removes the oldest sessions (by LastAccess) when the
// session count exceeds MaxSessions. Must be called with m.mu held.
func (m *SessionManager) evictOldSessions() error {
	if m.config.MaxSessions <= 0 || len(m.sessions) <= m.config.MaxSessions {
		return nil
	}

	// Build sorted list by last access (oldest first).
	type sessionEntry struct {
		id   string
		info *SessionInfo
	}
	entries := make([]sessionEntry, 0, len(m.sessions))
	for id, info := range m.sessions {
		entries = append(entries, sessionEntry{id: id, info: info})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].info.LastAccess.Before(entries[j].info.LastAccess)
	})

	// Evict oldest until we're at the limit.
	toEvict := len(m.sessions) - m.config.MaxSessions
	for i := 0; i < toEvict && i < len(entries); i++ {
		entry := entries[i]
		m.config.Logger.Debug("session: evicting old session",
			slog.String("session_id", entry.id),
			slog.Time("last_access", entry.info.LastAccess),
		)

		// Remove cached image file.
		// Note: we don't return an error here, just log it.
		imagePath := filepath.Join(m.config.CacheDir, "sessions", entry.info.ImageKey+".img")
		if err := os.Remove(imagePath); err != nil && !os.IsNotExist(err) {
			m.config.Logger.Warn("session: failed to remove evicted image",
				slog.String("session_id", entry.id),
				slog.String("error", err.Error()),
			)
		}

		delete(m.sessions, entry.id)
	}

	return nil
}

// indexPath returns the path to the session index file.
func (m *SessionManager) indexPath() string {
	return filepath.Join(m.config.CacheDir, "sessions", SessionIndexFile)
}

// loadIndex reads the session index from disk.
func (m *SessionManager) loadIndex() error {
	data, err := os.ReadFile(m.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No index yet, that's fine.
		}
		return fmt.Errorf("read index: %w", err)
	}

	var sessions map[string]*SessionInfo
	if err := json.Unmarshal(data, &sessions); err != nil {
		return fmt.Errorf("parse index: %w", err)
	}

	m.sessions = sessions
	m.config.Logger.Debug("session: loaded index", slog.Int("count", len(sessions)))
	return nil
}

// saveIndex writes the session index to disk atomically.
func (m *SessionManager) saveIndex() error {
	data, err := json.MarshalIndent(m.sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	indexPath := m.indexPath()
	tmpPath := indexPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write temp index: %w", err)
	}

	if err := os.Rename(tmpPath, indexPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename index: %w", err)
	}

	return nil
}

// Clean removes expired sessions and their images.
func (m *SessionManager) Clean() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Use image cache's Clean method to remove expired images.
	if err := m.imageCache.Clean(); err != nil {
		return fmt.Errorf("clean image cache: %w", err)
	}

	// Remove sessions whose images no longer exist.
	for sessionID, info := range m.sessions {
		if !m.imageCache.Has(info.ImageKey) {
			m.config.Logger.Debug("session: removing session with missing image",
				slog.String("session_id", sessionID),
			)
			delete(m.sessions, sessionID)
		}
	}

	if err := m.saveIndex(); err != nil {
		return fmt.Errorf("save index after clean: %w", err)
	}

	return nil
}

// Clear removes all sessions and their images.
func (m *SessionManager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.imageCache.Clear(); err != nil {
		return fmt.Errorf("clear image cache: %w", err)
	}

	m.sessions = make(map[string]*SessionInfo)

	// Remove the index file.
	if err := os.Remove(m.indexPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove index: %w", err)
	}

	return nil
}
