package cache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store provides a JSON file-based cache with per-key TTL.
// Files are stored in a flat directory structure:
//
//	~/.cache/prompt-pulse/
//	  claude.json
//	  billing.json
//	  tailscale.json
//	  kubernetes.json
//	  waifu.json
//	  meta.json
type Store struct {
	dir    string
	logger *slog.Logger
}

// Meta holds cache metadata including last update times and file sizes.
type Meta struct {
	LastUpdate map[string]time.Time `json:"last_update"`
	Sizes      map[string]int64    `json:"sizes"`
}

// NewStore creates a cache store at the given directory.
// The directory is created with 0700 permissions if it does not exist.
func NewStore(dir string, logger *slog.Logger) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("cache: create directory %s: %w", dir, err)
	}
	return &Store{dir: dir, logger: logger}, nil
}

// keyPath returns the filesystem path for a cache key.
func (s *Store) keyPath(key string) string {
	return filepath.Join(s.dir, key+".json")
}

// Get reads a cached value. Returns the data and whether it is fresh (within TTL).
// If the file does not exist, returns nil, false, nil.
// If the file exists but is stale (past TTL), returns data, false, nil.
// Corrupted or unreadable JSON files are removed and treated as a miss.
func (s *Store) Get(key string, ttl time.Duration) (json.RawMessage, bool, error) {
	path := s.keyPath(key)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("cache: stat %s: %w", key, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("cache: read %s: %w", key, err)
	}

	// Validate that the file contains valid JSON. If not, remove the
	// corrupted file and treat it as a cache miss.
	if !json.Valid(data) {
		s.logger.Warn("cache: removing corrupted entry", slog.String("key", key))
		_ = os.Remove(path)
		return nil, false, nil
	}

	fresh := time.Since(info.ModTime()) < ttl
	return json.RawMessage(data), fresh, nil
}

// Set writes a value to the cache with an atomic write (write to temp file,
// then rename). This prevents corrupted reads from concurrent access.
func (s *Store) Set(key string, data interface{}) error {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("cache: marshal %s: %w", key, err)
	}

	path := s.keyPath(key)

	tmp, err := os.CreateTemp(s.dir, ".tmp-"+key+"-*.json")
	if err != nil {
		return fmt.Errorf("cache: create temp for %s: %w", key, err)
	}
	tmpName := tmp.Name()

	// Clean up the temp file on any failure path.
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	if err := os.Chmod(tmpName, 0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("cache: chmod temp for %s: %w", key, err)
	}

	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("cache: write temp for %s: %w", key, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cache: close temp for %s: %w", key, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("cache: rename temp for %s: %w", key, err)
	}

	success = true
	return nil
}

// GetTyped reads and unmarshals a cached value into the type parameter T.
// Returns nil if the key does not exist. The fresh boolean indicates TTL status.
func GetTyped[T any](s *Store, key string, ttl time.Duration) (*T, bool, error) {
	raw, fresh, err := s.Get(key, ttl)
	if err != nil {
		return nil, false, err
	}
	if raw == nil {
		return nil, false, nil
	}

	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		s.logger.Warn("cache: removing entry with unmarshal error",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		_ = os.Remove(s.keyPath(key))
		return nil, false, nil
	}

	return &result, fresh, nil
}

// SetTyped marshals and caches a value of type T.
func SetTyped[T any](s *Store, key string, data *T) error {
	return s.Set(key, data)
}

// Age returns how old a cache entry is based on file modification time.
// Returns 0 if the entry does not exist.
func (s *Store) Age(key string) time.Duration {
	info, err := os.Stat(s.keyPath(key))
	if err != nil {
		return 0
	}
	return time.Since(info.ModTime())
}

// Keys returns all cached keys (filenames without the .json extension).
func (s *Store) Keys() []string {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}

	var keys []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(name, ".tmp-") {
			continue
		}
		if strings.HasSuffix(name, ".json") {
			keys = append(keys, strings.TrimSuffix(name, ".json"))
		}
	}
	return keys
}

// Clear removes all cache files from the store directory.
func (s *Store) Clear() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cache: clear read dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("cache: clear remove %s: %w", e.Name(), err)
		}
	}
	return nil
}

// Meta returns cache metadata including last update times and file sizes
// for every cached key.
func (s *Store) Meta() (*Meta, error) {
	m := &Meta{
		LastUpdate: make(map[string]time.Time),
		Sizes:      make(map[string]int64),
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, fmt.Errorf("cache: meta read dir: %w", err)
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".tmp-") || !strings.HasSuffix(name, ".json") {
			continue
		}

		key := strings.TrimSuffix(name, ".json")
		info, err := e.Info()
		if err != nil {
			continue
		}

		m.LastUpdate[key] = info.ModTime()
		m.Sizes[key] = info.Size()
	}

	return m, nil
}
