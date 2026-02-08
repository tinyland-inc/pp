package waifu

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// createMinimalPNG creates a tiny valid PNG for testing.
func createMinimalPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 128, B: 64, A: 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// createLargePNG creates a larger PNG (800x600) that will be resized by ProcessImage.
func createLargePNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 800; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x + y) % 256),
				G: uint8((x * 2) % 256),
				B: uint8((y * 3) % 256),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// newMockWaifuServer creates a test HTTP server that mimics the waifu.pics API.
func newMockWaifuServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/sfw/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"url": "http://" + r.Host + "/test-image.png",
		})
	})
	mux.HandleFunc("/test-image.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(createMinimalPNG())
	})
	return httptest.NewServer(mux)
}

// newMockWaifuServerLargeImage creates a test server that returns a large PNG.
func newMockWaifuServerLargeImage(t *testing.T) *httptest.Server {
	t.Helper()
	largeImg := createLargePNG()
	mux := http.NewServeMux()
	mux.HandleFunc("/sfw/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"url": "http://" + r.Host + "/large-image.png",
		})
	})
	mux.HandleFunc("/large-image.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(largeImg)
	})
	return httptest.NewServer(mux)
}

// newSlowMockServer creates a test server that delays before responding.
func newSlowMockServer(t *testing.T, delay time.Duration) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/sfw/", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(delay):
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"url": "http://" + r.Host + "/test-image.png",
			})
		case <-r.Context().Done():
			return
		}
	})
	mux.HandleFunc("/test-image.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(createMinimalPNG())
	})
	return httptest.NewServer(mux)
}

// newErrorMockServer creates a test server that returns HTTP 500.
func newErrorMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	})
	return httptest.NewServer(mux)
}

// withMockBaseURL sets the package-level baseURL to the given server URL for the
// duration of the test, restoring the original value on cleanup.
func withMockBaseURL(t *testing.T, serverURL string) {
	t.Helper()
	original := baseURL
	baseURL = serverURL
	t.Cleanup(func() {
		baseURL = original
	})
}

// newTestPrefetcher creates a Prefetcher with a temp cache dir pointed at the
// given mock server. It overrides baseURL for the test.
func newTestPrefetcher(t *testing.T, serverURL string, category string) *Prefetcher {
	t.Helper()
	withMockBaseURL(t, serverURL)

	dir := t.TempDir()
	cfg := PrefetchConfig{
		Category:   category,
		CacheDir:   dir,
		CacheTTL:   time.Hour,
		MaxCacheMB: 50,
		ProcessCfg: DefaultProcessConfig(),
		Logger:     testLogger(),
		Timeout:    10 * time.Second,
	}

	p, err := NewPrefetcher(cfg)
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}
	return p
}

func TestDefaultPrefetchConfig_Valid(t *testing.T) {
	cfg := DefaultPrefetchConfig()

	if cfg.Category != "neko" {
		t.Errorf("Category = %q, want %q", cfg.Category, "neko")
	}
	if !IsValidCategory(cfg.Category) {
		t.Errorf("default category %q is not valid", cfg.Category)
	}
	if cfg.CacheDir == "" {
		t.Error("CacheDir should not be empty")
	}
	if cfg.CacheTTL != 24*time.Hour {
		t.Errorf("CacheTTL = %v, want 24h", cfg.CacheTTL)
	}
	if cfg.MaxCacheMB != 50 {
		t.Errorf("MaxCacheMB = %d, want 50", cfg.MaxCacheMB)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
	if cfg.ProcessCfg.MaxWidth <= 0 || cfg.ProcessCfg.MaxHeight <= 0 {
		t.Error("ProcessCfg dimensions should be positive")
	}
}

func TestCacheKey_Format(t *testing.T) {
	key := CacheKey("neko")
	if key != "banner-neko" {
		t.Errorf("CacheKey(\"neko\") = %q, want %q", key, "banner-neko")
	}

	key2 := CacheKey("waifu")
	if key2 != "banner-waifu" {
		t.Errorf("CacheKey(\"waifu\") = %q, want %q", key2, "banner-waifu")
	}
}

func TestNewPrefetcher_Success(t *testing.T) {
	dir := t.TempDir()
	cfg := PrefetchConfig{
		Category:   "neko",
		CacheDir:   dir,
		CacheTTL:   time.Hour,
		MaxCacheMB: 50,
		ProcessCfg: DefaultProcessConfig(),
		Logger:     testLogger(),
		Timeout:    10 * time.Second,
	}

	p, err := NewPrefetcher(cfg)
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil Prefetcher")
	}
	if p.api == nil {
		t.Fatal("expected non-nil api client")
	}
	if p.cache == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestNewPrefetcher_InvalidCacheDir(t *testing.T) {
	cfg := PrefetchConfig{
		Category:   "neko",
		CacheDir:   "/dev/null/impossible/path",
		CacheTTL:   time.Hour,
		MaxCacheMB: 50,
		ProcessCfg: DefaultProcessConfig(),
		Logger:     testLogger(),
		Timeout:    10 * time.Second,
	}

	_, err := NewPrefetcher(cfg)
	if err == nil {
		t.Fatal("expected error for invalid cache directory")
	}
	if !strings.Contains(err.Error(), "prefetcher: init cache") {
		t.Errorf("expected prefetcher init cache error, got: %v", err)
	}
}

func TestEnsureCached_EmptyCache_FetchesAndCaches(t *testing.T) {
	server := newMockWaifuServer(t)
	defer server.Close()

	p := newTestPrefetcher(t, server.URL, "neko")

	wasCached, err := p.EnsureCached(context.Background())
	if err != nil {
		t.Fatalf("EnsureCached: %v", err)
	}
	if wasCached {
		t.Error("expected wasCached=false for empty cache")
	}

	// Verify the image is now cached.
	key := CacheKey("neko")
	if !p.cache.Has(key) {
		t.Error("expected image to be cached after EnsureCached")
	}
}

func TestEnsureCached_AlreadyCached_ReturnsTrueWithoutFetch(t *testing.T) {
	server := newMockWaifuServer(t)
	defer server.Close()

	p := newTestPrefetcher(t, server.URL, "neko")

	// Pre-populate the cache.
	key := CacheKey("neko")
	if err := p.cache.Put(key, []byte("pre-cached-image-data")); err != nil {
		t.Fatalf("cache.Put: %v", err)
	}

	wasCached, err := p.EnsureCached(context.Background())
	if err != nil {
		t.Fatalf("EnsureCached: %v", err)
	}
	if !wasCached {
		t.Error("expected wasCached=true for pre-cached image")
	}
}

func TestEnsureCached_ExpiredCache_Refetches(t *testing.T) {
	server := newMockWaifuServer(t)
	defer server.Close()

	withMockBaseURL(t, server.URL)

	dir := t.TempDir()
	cfg := PrefetchConfig{
		Category:   "neko",
		CacheDir:   dir,
		CacheTTL:   50 * time.Millisecond, // Short TTL but with margin for reliability.
		MaxCacheMB: 50,
		ProcessCfg: DefaultProcessConfig(),
		Logger:     testLogger(),
		Timeout:    10 * time.Second,
	}

	p, err := NewPrefetcher(cfg)
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}

	// Pre-populate the cache with valid PNG data so Get() succeeds initially.
	key := CacheKey("neko")
	if err := p.cache.Put(key, createMinimalPNG()); err != nil {
		t.Fatalf("cache.Put: %v", err)
	}

	// Wait for expiration (2x TTL ensures reliable expiration across systems).
	time.Sleep(120 * time.Millisecond)

	wasCached, err := p.EnsureCached(context.Background())
	if err != nil {
		t.Fatalf("EnsureCached: %v", err)
	}
	if wasCached {
		t.Error("expected wasCached=false for expired cache")
	}

	// Verify the cache was refreshed with new data.
	if !p.cache.Has(key) {
		t.Error("expected image to be re-cached after expiration")
	}
}

func TestPrefetchCategory_ValidCategory(t *testing.T) {
	server := newMockWaifuServer(t)
	defer server.Close()

	p := newTestPrefetcher(t, server.URL, "neko")

	err := p.PrefetchCategory(context.Background(), "waifu")
	if err != nil {
		t.Fatalf("PrefetchCategory: %v", err)
	}

	// Verify cached under the correct key.
	key := CacheKey("waifu")
	if !p.cache.Has(key) {
		t.Error("expected image cached under banner-waifu")
	}
}

func TestPrefetchCategory_InvalidCategory(t *testing.T) {
	server := newMockWaifuServer(t)
	defer server.Close()

	p := newTestPrefetcher(t, server.URL, "neko")

	err := p.PrefetchCategory(context.Background(), "invalid-category")
	if err == nil {
		t.Fatal("expected error for invalid category")
	}
	if !strings.Contains(err.Error(), "invalid category") {
		t.Errorf("expected invalid category error, got: %v", err)
	}
}

func TestPrefetchCategory_AlreadyCached_SkipsFetch(t *testing.T) {
	server := newMockWaifuServer(t)
	defer server.Close()

	p := newTestPrefetcher(t, server.URL, "neko")

	// Pre-populate cache for "smile".
	key := CacheKey("smile")
	if err := p.cache.Put(key, []byte("cached-smile-data")); err != nil {
		t.Fatalf("cache.Put: %v", err)
	}

	err := p.PrefetchCategory(context.Background(), "smile")
	if err != nil {
		t.Fatalf("PrefetchCategory: %v", err)
	}

	// Verify the original cached data is unchanged (no re-fetch happened).
	data, fresh, err := p.cache.Get(key)
	if err != nil {
		t.Fatalf("cache.Get: %v", err)
	}
	if !fresh {
		t.Error("expected cached data to still be fresh")
	}
	if string(data) != "cached-smile-data" {
		t.Errorf("expected original cached data, got %d bytes", len(data))
	}
}

func TestFetchAndCache_Success(t *testing.T) {
	server := newMockWaifuServer(t)
	defer server.Close()

	p := newTestPrefetcher(t, server.URL, "neko")

	err := p.fetchAndCache(context.Background(), "neko")
	if err != nil {
		t.Fatalf("fetchAndCache: %v", err)
	}

	key := CacheKey("neko")
	data, fresh, err := p.cache.Get(key)
	if err != nil {
		t.Fatalf("cache.Get: %v", err)
	}
	if !fresh {
		t.Error("expected cached data to be fresh")
	}
	if len(data) == 0 {
		t.Error("expected non-empty cached data")
	}
}

func TestFetchAndCache_ContextCancellation(t *testing.T) {
	server := newMockWaifuServer(t)
	defer server.Close()

	p := newTestPrefetcher(t, server.URL, "neko")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	err := p.fetchAndCache(ctx, "neko")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFetchAndCache_APIError(t *testing.T) {
	server := newErrorMockServer(t)
	defer server.Close()

	p := newTestPrefetcher(t, server.URL, "neko")

	err := p.fetchAndCache(context.Background(), "neko")
	if err == nil {
		t.Fatal("expected error for API returning 500")
	}
	if !strings.Contains(err.Error(), "fetch URL") {
		t.Errorf("expected fetch URL error, got: %v", err)
	}
}

func TestEnsureCached_Timeout(t *testing.T) {
	// Server delays 5 seconds; prefetcher timeout is 100ms.
	server := newSlowMockServer(t, 5*time.Second)
	defer server.Close()

	withMockBaseURL(t, server.URL)

	dir := t.TempDir()
	cfg := PrefetchConfig{
		Category:   "neko",
		CacheDir:   dir,
		CacheTTL:   time.Hour,
		MaxCacheMB: 50,
		ProcessCfg: DefaultProcessConfig(),
		Logger:     testLogger(),
		Timeout:    100 * time.Millisecond, // Very short timeout.
	}

	p, err := NewPrefetcher(cfg)
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}

	_, err = p.EnsureCached(context.Background())
	if err == nil {
		t.Fatal("expected error due to timeout")
	}
}

func TestCacheKey_DifferentCategories_DifferentKeys(t *testing.T) {
	categories := []string{"neko", "waifu", "smile", "hug", "pat"}
	seen := make(map[string]string)

	for _, cat := range categories {
		key := CacheKey(cat)
		if prev, exists := seen[key]; exists {
			t.Errorf("cache key collision: %q and %q both produce %q", prev, cat, key)
		}
		seen[key] = cat
	}
}

func TestProcessedImage_ResizedDimensions(t *testing.T) {
	server := newMockWaifuServerLargeImage(t)
	defer server.Close()

	p := newTestPrefetcher(t, server.URL, "neko")

	err := p.fetchAndCache(context.Background(), "neko")
	if err != nil {
		t.Fatalf("fetchAndCache: %v", err)
	}

	key := CacheKey("neko")
	processed, fresh, err := p.cache.Get(key)
	if err != nil {
		t.Fatalf("cache.Get: %v", err)
	}
	if !fresh {
		t.Error("expected fresh cached data")
	}

	// The large image (800x600) should be resized to fit within
	// DefaultProcessConfig's MaxWidth=400, MaxHeight=300.
	// Decode the processed image and verify its dimensions are within bounds.
	img, _, err := DecodeImage(processed)
	if err != nil {
		t.Fatalf("failed to decode processed image: %v", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	cfg := DefaultProcessConfig()
	if w > cfg.MaxWidth {
		t.Errorf("processed width %d exceeds MaxWidth %d", w, cfg.MaxWidth)
	}
	if h > cfg.MaxHeight {
		t.Errorf("processed height %d exceeds MaxHeight %d", h, cfg.MaxHeight)
	}
	if w <= 0 || h <= 0 {
		t.Errorf("processed dimensions should be positive, got %dx%d", w, h)
	}
}

func TestNewPrefetcher_NilLogger(t *testing.T) {
	dir := t.TempDir()
	cfg := PrefetchConfig{
		Category:   "neko",
		CacheDir:   dir,
		CacheTTL:   time.Hour,
		MaxCacheMB: 50,
		ProcessCfg: DefaultProcessConfig(),
		Logger:     nil, // Nil logger should be handled gracefully.
		Timeout:    10 * time.Second,
	}

	p, err := NewPrefetcher(cfg)
	if err != nil {
		t.Fatalf("NewPrefetcher with nil logger: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil Prefetcher")
	}
	if p.config.Logger == nil {
		t.Fatal("expected non-nil logger (should use no-op fallback)")
	}
}

func TestPrefetchCategory_CachesUnderCorrectKey(t *testing.T) {
	server := newMockWaifuServer(t)
	defer server.Close()

	p := newTestPrefetcher(t, server.URL, "neko")

	// Prefetch for "happy" category.
	err := p.PrefetchCategory(context.Background(), "happy")
	if err != nil {
		t.Fatalf("PrefetchCategory: %v", err)
	}

	// Verify it is cached under "banner-happy".
	happyKey := CacheKey("happy")
	if !p.cache.Has(happyKey) {
		t.Error("expected image cached under banner-happy key")
	}

	// Verify it is NOT cached under the default "banner-neko" key.
	nekoKey := CacheKey("neko")
	if p.cache.Has(nekoKey) {
		t.Error("did not expect image under banner-neko when prefetching happy")
	}

	// Verify the cache file actually exists on disk.
	data, fresh, err := p.cache.Get(happyKey)
	if err != nil {
		t.Fatalf("cache.Get: %v", err)
	}
	if !fresh {
		t.Error("expected fresh cached data")
	}
	if len(data) == 0 {
		t.Error("expected non-empty cached data for happy category")
	}
}

func TestEnsureCached_CacheDirCreated(t *testing.T) {
	server := newMockWaifuServer(t)
	defer server.Close()

	withMockBaseURL(t, server.URL)

	// Use a nested directory that does not yet exist.
	baseDir := t.TempDir()
	nestedDir := baseDir + "/deep/nested/cache"

	cfg := PrefetchConfig{
		Category:   "neko",
		CacheDir:   nestedDir,
		CacheTTL:   time.Hour,
		MaxCacheMB: 50,
		ProcessCfg: DefaultProcessConfig(),
		Logger:     testLogger(),
		Timeout:    10 * time.Second,
	}

	p, err := NewPrefetcher(cfg)
	if err != nil {
		t.Fatalf("NewPrefetcher: %v", err)
	}

	// Verify directory was created.
	info, err := os.Stat(nestedDir)
	if err != nil {
		t.Fatalf("expected cache dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected cache path to be a directory")
	}

	wasCached, err := p.EnsureCached(context.Background())
	if err != nil {
		t.Fatalf("EnsureCached: %v", err)
	}
	if wasCached {
		t.Error("expected wasCached=false for fresh nested dir cache")
	}
}
