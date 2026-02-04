package waifu

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"
)

// PrefetchConfig controls the image prefetch behavior.
type PrefetchConfig struct {
	// Category is the waifu.pics category to prefetch.
	Category string
	// CacheDir is the directory for cached images.
	CacheDir string
	// CacheTTL is how long cached images remain valid.
	CacheTTL time.Duration
	// MaxCacheMB is the max image cache size in megabytes.
	MaxCacheMB int
	// ProcessCfg controls image processing (resize, sharpen).
	ProcessCfg ProcessConfig
	// Logger for prefetch operations.
	Logger *slog.Logger
	// Timeout is the max duration for the prefetch operation.
	Timeout time.Duration
}

// DefaultPrefetchConfig returns sensible defaults.
func DefaultPrefetchConfig() PrefetchConfig {
	defaults := DefaultImageCacheConfig()
	return PrefetchConfig{
		Category:   "neko",
		CacheDir:   defaults.Dir,
		CacheTTL:   24 * time.Hour,
		MaxCacheMB: 50,
		ProcessCfg: DefaultProcessConfig(),
		Timeout:    30 * time.Second,
	}
}

// Prefetcher handles background image prefetching.
type Prefetcher struct {
	config PrefetchConfig
	api    *APIClient
	cache  *ImageCache
}

// NewPrefetcher creates a Prefetcher and initializes the image cache.
func NewPrefetcher(cfg PrefetchConfig) (*Prefetcher, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	cache, err := NewImageCache(ImageCacheConfig{
		Dir:       cfg.CacheDir,
		TTL:       cfg.CacheTTL,
		MaxSizeMB: cfg.MaxCacheMB,
		Logger:    cfg.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("prefetcher: init cache: %w", err)
	}

	api := NewAPIClient(cfg.Logger)

	return &Prefetcher{
		config: cfg,
		api:    api,
		cache:  cache,
	}, nil
}

// EnsureCached checks if an image for the configured category is cached.
// If not cached (or expired), it fetches a new image from waifu.pics,
// processes it, and stores it in the cache.
// Returns (wasCached bool, error).
func (p *Prefetcher) EnsureCached(ctx context.Context) (bool, error) {
	key := CacheKey(p.config.Category)

	if p.cache.Has(key) {
		p.config.Logger.Debug("prefetch: image already cached",
			slog.String("category", p.config.Category),
			slog.String("key", key),
		)
		return true, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	if err := p.fetchAndCache(timeoutCtx, p.config.Category); err != nil {
		return false, err
	}

	return false, nil
}

// PrefetchCategory fetches and caches an image for a specific category,
// overriding the configured category. Used when the daemon evaluates
// status and wants to cache a specific mood's image.
func (p *Prefetcher) PrefetchCategory(ctx context.Context, category string) error {
	if !IsValidCategory(category) {
		return fmt.Errorf("prefetch: invalid category: %q", category)
	}

	key := CacheKey(category)

	if p.cache.Has(key) {
		p.config.Logger.Debug("prefetch: image already cached for category",
			slog.String("category", category),
			slog.String("key", key),
		)
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	return p.fetchAndCache(timeoutCtx, category)
}

// fetchAndCache is the core logic: fetch URL, download image, process it,
// write to cache.
func (p *Prefetcher) fetchAndCache(ctx context.Context, category string) error {
	p.config.Logger.Debug("prefetch: fetching image URL",
		slog.String("category", category),
	)

	imageURL, err := p.api.FetchImageURL(ctx, category)
	if err != nil {
		return fmt.Errorf("prefetch: fetch URL for %q: %w", category, err)
	}

	p.config.Logger.Debug("prefetch: downloading image",
		slog.String("category", category),
		slog.String("url", imageURL),
	)

	data, _, err := p.api.DownloadImage(ctx, imageURL)
	if err != nil {
		return fmt.Errorf("prefetch: download image for %q: %w", category, err)
	}

	p.config.Logger.Debug("prefetch: processing image",
		slog.String("category", category),
		slog.Int("raw_bytes", len(data)),
	)

	processed, err := ProcessImage(data, p.config.ProcessCfg)
	if err != nil {
		return fmt.Errorf("prefetch: process image for %q: %w", category, err)
	}

	key := CacheKey(category)

	p.config.Logger.Debug("prefetch: caching processed image",
		slog.String("category", category),
		slog.String("key", key),
		slog.Int("processed_bytes", len(processed)),
	)

	if err := p.cache.Put(key, processed); err != nil {
		return fmt.Errorf("prefetch: cache image for %q: %w", category, err)
	}

	return nil
}

// CacheKey returns the cache key used for a given category.
// Format: "banner-{category}" so it's distinguishable from URL-based keys.
func CacheKey(category string) string {
	return "banner-" + category
}
