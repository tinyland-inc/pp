// Package waifu implements a collector that fetches optimized waifu images
// from a mirror API and caches them locally for the TUI dashboard.
package waifu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	DefaultInterval  = 1 * time.Hour
	DefaultMaxImages = 20
	DefaultCategory  = "sfw"
	ManifestFile     = "waifu.json"
)

// Config holds the configuration for the waifu collector.
type Config struct {
	Interval  time.Duration
	Endpoint  string
	Category  string
	CacheDir  string
	MaxImages int
}

// CachedImage records a single cached image in the manifest.
type CachedImage struct {
	Path      string    `json:"path"`
	Hash      string    `json:"hash"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Manifest is the JSON file written to the waifu cache directory. The TUI
// reads this to discover available images and the currently selected one.
type Manifest struct {
	Images  []CachedImage `json:"images"`
	Current string        `json:"current"`
}

// Collector fetches waifu images from the mirror API and manages the
// local image cache.
type Collector struct {
	cfg      Config
	client   Client
	interval time.Duration

	mu      sync.Mutex
	healthy bool
}

// New creates a waifu collector. If client is nil, a default HTTP client
// is created for the configured endpoint.
func New(cfg Config, client Client) *Collector {
	interval := cfg.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	if cfg.MaxImages <= 0 {
		cfg.MaxImages = DefaultMaxImages
	}
	if cfg.Category == "" {
		cfg.Category = DefaultCategory
	}
	if client == nil && cfg.Endpoint != "" {
		client = NewClient(cfg.Endpoint)
	}

	return &Collector{
		cfg:      cfg,
		client:   client,
		interval: interval,
		healthy:  true,
	}
}

// Name returns the collector identifier.
func (c *Collector) Name() string { return "waifu" }

// Interval returns how often this collector should run.
func (c *Collector) Interval() time.Duration { return c.interval }

// Healthy returns whether the last collection succeeded.
func (c *Collector) Healthy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.healthy
}

func (c *Collector) setHealthy(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.healthy = v
}

// Collect fetches one new image from the mirror API, caches it locally,
// prunes old images beyond MaxImages, and returns the current Manifest.
func (c *Collector) Collect(ctx context.Context) (interface{}, error) {
	if c.client == nil {
		c.setHealthy(false)
		return nil, fmt.Errorf("waifu: no endpoint configured")
	}

	cacheDir := c.cfg.CacheDir
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		c.setHealthy(false)
		return nil, fmt.Errorf("waifu: create cache dir: %w", err)
	}

	// Load existing manifest.
	manifest := c.loadManifest(cacheDir)

	// Fetch a new random image.
	meta, err := c.client.RandomImage(ctx, c.cfg.Category)
	if err != nil {
		c.setHealthy(false)
		// Return existing manifest on fetch failure (stale but usable).
		if len(manifest.Images) > 0 {
			return manifest, nil
		}
		return nil, fmt.Errorf("waifu: fetch random: %w", err)
	}

	// Download the image bytes.
	data, err := c.client.DownloadImage(ctx, meta.URL)
	if err != nil {
		c.setHealthy(false)
		if len(manifest.Images) > 0 {
			return manifest, nil
		}
		return nil, fmt.Errorf("waifu: download: %w", err)
	}

	// Compute content hash for dedup.
	hash := contentHash(data)

	// Check if we already have this image.
	for _, img := range manifest.Images {
		if img.Hash == hash {
			// Already cached — just update current selection.
			manifest.Current = hash
			c.writeManifest(cacheDir, manifest)
			c.setHealthy(true)
			return manifest, nil
		}
	}

	// Determine file extension from meta or default to .webp.
	ext := ".webp"
	if meta.ID != "" {
		ext = filepath.Ext(meta.ID)
		if ext == "" {
			ext = ".webp"
		}
	}

	// Write image file.
	imgPath := filepath.Join(cacheDir, hash+ext)
	if err := atomicWrite(imgPath, data); err != nil {
		c.setHealthy(false)
		return nil, fmt.Errorf("waifu: write image: %w", err)
	}

	// Update manifest.
	manifest.Images = append(manifest.Images, CachedImage{
		Path:      imgPath,
		Hash:      hash,
		FetchedAt: time.Now(),
	})
	manifest.Current = hash

	// Prune oldest images if over limit.
	c.pruneImages(cacheDir, manifest)

	// Persist manifest.
	c.writeManifest(cacheDir, manifest)

	c.setHealthy(true)
	return manifest, nil
}

// loadManifest reads the waifu.json manifest from the cache directory.
func (c *Collector) loadManifest(cacheDir string) *Manifest {
	path := filepath.Join(cacheDir, ManifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return &Manifest{}
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return &Manifest{}
	}
	return &m
}

// writeManifest atomically writes the manifest to waifu.json.
func (c *Collector) writeManifest(cacheDir string, m *Manifest) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(cacheDir, ManifestFile)
	_ = atomicWrite(path, data)
}

// pruneImages removes the oldest cached images when len exceeds MaxImages.
func (c *Collector) pruneImages(cacheDir string, m *Manifest) {
	if len(m.Images) <= c.cfg.MaxImages {
		return
	}

	// Sort by FetchedAt ascending (oldest first).
	sort.Slice(m.Images, func(i, j int) bool {
		return m.Images[i].FetchedAt.Before(m.Images[j].FetchedAt)
	})

	// Remove oldest entries.
	toRemove := len(m.Images) - c.cfg.MaxImages
	for i := 0; i < toRemove; i++ {
		os.Remove(m.Images[i].Path)
	}
	m.Images = m.Images[toRemove:]
}

// contentHash returns a hex-encoded SHA-256 of the first 64KB.
func contentHash(data []byte) string {
	limit := 64 * 1024
	if len(data) < limit {
		limit = len(data)
	}
	h := sha256.Sum256(data[:limit])
	return hex.EncodeToString(h[:16]) // 128-bit prefix is enough for dedup
}

// atomicWrite writes data to a temp file then renames to dest.
func atomicWrite(dest string, data []byte) error {
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
