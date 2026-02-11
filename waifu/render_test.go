package waifu

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

// makePNG creates a minimal PNG image of the given dimensions filled with the
// specified color. This avoids external test fixtures.
func makePNG(w, h int, c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// makeLargePNG creates a PNG large enough that its base64 encoding exceeds
// kittyChunkSize bytes, forcing multi-chunk transmission. It uses
// pseudo-random pixel values to defeat PNG compression.
func makeLargePNG() []byte {
	w, h := 200, 200
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Fill with varying colors that PNG cannot compress efficiently.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := (x*7 + y*13 + x*y) % 256
			img.Set(x, y, color.RGBA{R: uint8(v), G: uint8(255 - v), B: uint8((v * 3) % 256), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// withEnv sets an environment variable for the duration of a test and restores
// the original value (or unsets it) when the test completes.
func withEnv(t *testing.T, key, value string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

// clearEnv unsets an environment variable for the duration of a test and
// restores it afterward.
func clearEnv(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		}
	})
}

// clearSSHEnv clears SSH-related environment variables so tests can simulate
// a non-SSH environment. Without this, tests running on an SSH session would
// see SSH_CLIENT/SSH_CONNECTION/SSH_TTY and downgrade Kitty to Unicode.
func clearSSHEnv(t *testing.T) {
	t.Helper()
	clearEnv(t, "SSH_CLIENT")
	clearEnv(t, "SSH_CONNECTION")
	clearEnv(t, "SSH_TTY")
}

// --- DetectProtocol tests ---

func TestDetectProtocol_Ghostty(t *testing.T) {
	clearSSHEnv(t)
	clearEnv(t, "TERM")
	clearEnv(t, "KITTY_WINDOW_ID")
	withEnv(t, "TERM_PROGRAM", "ghostty")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %d, want ProtocolKitty (%d)", got, ProtocolKitty)
	}
}

func TestDetectProtocol_Kitty(t *testing.T) {
	clearSSHEnv(t)
	clearEnv(t, "TERM")
	clearEnv(t, "KITTY_WINDOW_ID")
	withEnv(t, "TERM_PROGRAM", "kitty")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %d, want ProtocolKitty (%d)", got, ProtocolKitty)
	}
}

func TestDetectProtocol_WezTerm(t *testing.T) {
	clearSSHEnv(t)
	clearEnv(t, "TERM")
	clearEnv(t, "KITTY_WINDOW_ID")
	withEnv(t, "TERM_PROGRAM", "WezTerm")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %d, want ProtocolKitty (%d)", got, ProtocolKitty)
	}
}

func TestDetectProtocol_XtermKitty(t *testing.T) {
	clearSSHEnv(t)
	clearEnv(t, "TERM_PROGRAM")
	clearEnv(t, "KITTY_WINDOW_ID")
	withEnv(t, "TERM", "xterm-kitty")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %d, want ProtocolKitty (%d)", got, ProtocolKitty)
	}
}

func TestDetectProtocol_KittyWindowID(t *testing.T) {
	clearSSHEnv(t)
	clearEnv(t, "TERM_PROGRAM")
	clearEnv(t, "TERM")
	withEnv(t, "KITTY_WINDOW_ID", "42")

	if got := DetectProtocol(); got != ProtocolKitty {
		t.Errorf("DetectProtocol() = %d, want ProtocolKitty (%d)", got, ProtocolKitty)
	}
}

func TestDetectProtocol_Unknown(t *testing.T) {
	clearSSHEnv(t)
	clearEnv(t, "TERM_PROGRAM")
	clearEnv(t, "TERM")
	clearEnv(t, "KITTY_WINDOW_ID")

	if got := DetectProtocol(); got != ProtocolUnicode {
		t.Errorf("DetectProtocol() = %d, want ProtocolUnicode (%d)", got, ProtocolUnicode)
	}
}

// --- SSH downgrade tests ---
// When SSH_CLIENT, SSH_CONNECTION, or SSH_TTY is set, Kitty protocol
// should be downgraded to Unicode to prevent garbled output.

func TestDetectProtocol_SSH_DowngradesGhostty(t *testing.T) {
	withEnv(t, "SSH_CLIENT", "192.168.1.1 12345 22")
	clearEnv(t, "TERM")
	clearEnv(t, "KITTY_WINDOW_ID")
	withEnv(t, "TERM_PROGRAM", "ghostty")

	if got := DetectProtocol(); got != ProtocolUnicode {
		t.Errorf("DetectProtocol() over SSH = %d, want ProtocolUnicode (%d)", got, ProtocolUnicode)
	}
}

func TestDetectProtocol_SSH_DowngradesKitty(t *testing.T) {
	withEnv(t, "SSH_CONNECTION", "192.168.1.1 12345 10.0.0.1 22")
	clearEnv(t, "TERM")
	clearEnv(t, "KITTY_WINDOW_ID")
	withEnv(t, "TERM_PROGRAM", "kitty")

	if got := DetectProtocol(); got != ProtocolUnicode {
		t.Errorf("DetectProtocol() over SSH = %d, want ProtocolUnicode (%d)", got, ProtocolUnicode)
	}
}

func TestDetectProtocol_SSH_DowngradesXtermKitty(t *testing.T) {
	withEnv(t, "SSH_TTY", "/dev/pts/0")
	clearEnv(t, "TERM_PROGRAM")
	clearEnv(t, "KITTY_WINDOW_ID")
	withEnv(t, "TERM", "xterm-kitty")

	if got := DetectProtocol(); got != ProtocolUnicode {
		t.Errorf("DetectProtocol() over SSH = %d, want ProtocolUnicode (%d)", got, ProtocolUnicode)
	}
}

func TestDetectProtocol_SSH_DowngradesKittyWindowID(t *testing.T) {
	withEnv(t, "SSH_CLIENT", "192.168.1.1 12345 22")
	clearEnv(t, "TERM_PROGRAM")
	clearEnv(t, "TERM")
	withEnv(t, "KITTY_WINDOW_ID", "42")

	if got := DetectProtocol(); got != ProtocolUnicode {
		t.Errorf("DetectProtocol() over SSH = %d, want ProtocolUnicode (%d)", got, ProtocolUnicode)
	}
}

func TestDetectProtocol_SSH_UnknownStaysUnicode(t *testing.T) {
	withEnv(t, "SSH_CLIENT", "192.168.1.1 12345 22")
	clearEnv(t, "TERM_PROGRAM")
	clearEnv(t, "TERM")
	clearEnv(t, "KITTY_WINDOW_ID")

	if got := DetectProtocol(); got != ProtocolUnicode {
		t.Errorf("DetectProtocol() over SSH = %d, want ProtocolUnicode (%d)", got, ProtocolUnicode)
	}
}

// --- renderKitty tests ---

func TestRenderKitty_SmallImage(t *testing.T) {
	data := makePNG(1, 1, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	result, err := renderKitty(data, 10, 5)
	if err != nil {
		t.Fatalf("renderKitty() error: %v", err)
	}

	// Small image should fit in a single chunk (m=0, no m=1).
	if strings.Contains(result, "m=1") {
		t.Error("small image should not produce multi-chunk output (found m=1)")
	}
	if !strings.Contains(result, "m=0") {
		t.Error("single-chunk output should contain m=0")
	}
}

func TestRenderKitty_LargeImage(t *testing.T) {
	data := makeLargePNG()
	result, err := renderKitty(data, 40, 20)
	if err != nil {
		t.Fatalf("renderKitty() error: %v", err)
	}

	// Large image should produce multiple chunks.
	if !strings.Contains(result, "m=1") {
		t.Error("large image should produce multi-chunk output (missing m=1)")
	}
	if !strings.Contains(result, "m=0") {
		t.Error("multi-chunk output should end with m=0")
	}

	// The m=0 chunk should be the last escape sequence.
	lastM0 := strings.LastIndex(result, "m=0")
	lastM1 := strings.LastIndex(result, "m=1")
	if lastM1 > lastM0 {
		t.Error("m=1 should not appear after the final m=0 chunk")
	}
}

func TestRenderKitty_EscapeSequences(t *testing.T) {
	data := makePNG(2, 2, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	result, err := renderKitty(data, 20, 10)
	if err != nil {
		t.Fatalf("renderKitty() error: %v", err)
	}

	// Verify escape sequence framing.
	if !strings.Contains(result, "\033_G") {
		t.Error("missing Kitty escape sequence start (\\033_G)")
	}
	if !strings.Contains(result, "\033\\") {
		t.Error("missing Kitty escape sequence terminator (\\033\\\\)")
	}

	// Verify key parameters in first chunk.
	if !strings.Contains(result, "f=100") {
		t.Error("missing format parameter f=100 (PNG)")
	}
	if !strings.Contains(result, "a=T") {
		t.Error("missing action parameter a=T")
	}
	if !strings.Contains(result, "t=d") {
		t.Error("missing transmission type t=d")
	}
	if !strings.Contains(result, "c=20") {
		t.Errorf("missing or incorrect column parameter c=20 in: %s", result)
	}
	if !strings.Contains(result, "r=10") {
		t.Errorf("missing or incorrect row parameter r=10 in: %s", result)
	}
}

// --- renderUnicode tests ---

func TestRenderUnicode_BasicImage(t *testing.T) {
	data := makePNG(2, 2, color.RGBA{R: 255, G: 128, B: 0, A: 255})
	result, err := renderUnicode(data, 10, 5)
	if err != nil {
		t.Fatalf("renderUnicode() error: %v", err)
	}

	// Should contain ANSI color codes.
	if !strings.Contains(result, "\033[38;2;") {
		t.Error("output should contain ANSI 24-bit foreground color sequences")
	}
	if !strings.Contains(result, "\033[48;2;") {
		t.Error("output should contain ANSI 24-bit background color sequences")
	}
	// Should contain reset.
	if !strings.Contains(result, "\033[0m") {
		t.Error("output should contain ANSI reset sequence")
	}
}

func TestRenderUnicode_ContainsHalfBlock(t *testing.T) {
	data := makePNG(2, 2, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	result, err := renderUnicode(data, 10, 5)
	if err != nil {
		t.Fatalf("renderUnicode() error: %v", err)
	}

	if !strings.Contains(result, "\u2580") {
		t.Error("output should contain upper half-block character U+2580")
	}
}

func TestRenderUnicode_Dimensions(t *testing.T) {
	// Create a 4x4 image. With cols=4 and rows=2, the resized image will be
	// at most 4 pixels wide and 4 pixels tall (rows*2). Each pair of pixel
	// rows produces one line of text.
	data := makePNG(4, 4, color.RGBA{R: 50, G: 50, B: 50, A: 255})
	result, err := renderUnicode(data, 4, 2)
	if err != nil {
		t.Fatalf("renderUnicode() error: %v", err)
	}

	lines := strings.Split(result, "\n")
	// A 4x4 image fitting in 4 cols x 4 pixel-rows (2 text rows) should produce
	// at most 2 lines.
	if len(lines) > 2 {
		t.Errorf("expected at most 2 lines, got %d", len(lines))
	}
	if len(lines) == 0 {
		t.Error("expected at least 1 line of output")
	}
}

// --- RenderImage dispatch tests ---

func TestRenderImage_Dispatch_Kitty(t *testing.T) {
	data := makePNG(1, 1, color.White)
	cfg := RenderConfig{Protocol: ProtocolKitty, MaxCols: 10, MaxRows: 5}
	result, err := RenderImage(data, cfg)
	if err != nil {
		t.Fatalf("RenderImage(Kitty) error: %v", err)
	}

	// Chafa-first: if chafa is available, it handles rendering automatically.
	// Accept either Kitty escape sequences or chafa ANSI output.
	if !strings.Contains(result, "\033_G") && !strings.Contains(result, "\033[") {
		t.Error("ProtocolKitty should produce Kitty or chafa escape sequences")
	}
}

func TestRenderImage_Dispatch_Unicode(t *testing.T) {
	data := makePNG(2, 2, color.White)
	cfg := RenderConfig{Protocol: ProtocolUnicode, MaxCols: 10, MaxRows: 5}
	result, err := RenderImage(data, cfg)
	if err != nil {
		t.Fatalf("RenderImage(Unicode) error: %v", err)
	}

	// Chafa-first: if chafa is available, it handles rendering automatically.
	// Accept either half-block characters or chafa ANSI output.
	if !strings.Contains(result, "\u2580") && !strings.Contains(result, "\033[") {
		t.Error("ProtocolUnicode should produce half-block or chafa output")
	}
}

func TestRenderImage_Dispatch_None(t *testing.T) {
	data := makePNG(1, 1, color.White)
	cfg := RenderConfig{Protocol: ProtocolNone, MaxCols: 10, MaxRows: 5}
	result, err := RenderImage(data, cfg)
	if err != nil {
		t.Fatalf("RenderImage(None) error: %v", err)
	}

	// Chafa-first: if chafa is available, it renders even for ProtocolNone.
	// Accept either fallback text or chafa ANSI output.
	if result != "(image: protocol not supported)" && !strings.Contains(result, "\033[") {
		t.Errorf("ProtocolNone should return fallback text or chafa output, got: %q", result)
	}
}

func TestRenderImage_InvalidImage(t *testing.T) {
	cfg := RenderConfig{Protocol: ProtocolUnicode, MaxCols: 10, MaxRows: 5}
	_, err := RenderImage([]byte("not an image"), cfg)
	if err == nil {
		t.Error("RenderImage with invalid data should return an error")
	}
}

func TestDefaultRenderConfig(t *testing.T) {
	cfg := DefaultRenderConfig()

	if cfg.Protocol != ProtocolKitty {
		t.Errorf("Protocol = %d, want ProtocolKitty (%d)", cfg.Protocol, ProtocolKitty)
	}
	if cfg.MaxCols != 40 {
		t.Errorf("MaxCols = %d, want 40", cfg.MaxCols)
	}
	if cfg.MaxRows != 20 {
		t.Errorf("MaxRows = %d, want 20", cfg.MaxRows)
	}
	if cfg.Writer == nil {
		t.Error("Writer should default to os.Stdout, got nil")
	}
}

// --- OptimizedRenderer tests ---

func TestOptimizedRenderer_Render_CacheHit(t *testing.T) {
	cfg := DefaultOptimizedRenderConfig()
	r := NewOptimizedRenderer(cfg)

	data := makePNG(4, 4, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	sessionID := "test-session"

	// First render - cache miss
	result1, err := r.Render(sessionID, data, 20, 10)
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}
	if result1.FromCache {
		t.Error("First render should not be from cache")
	}
	if result1.Output == "" {
		t.Error("Expected non-empty output")
	}

	// Second render - cache hit
	result2, err := r.Render(sessionID, data, 20, 10)
	if err != nil {
		t.Fatalf("Second render failed: %v", err)
	}
	if !result2.FromCache {
		t.Error("Second render should be from cache")
	}
	if result2.Output != result1.Output {
		t.Error("Cached output should match original")
	}
}

func TestOptimizedRenderer_Render_DifferentDimensions(t *testing.T) {
	cfg := DefaultOptimizedRenderConfig()
	r := NewOptimizedRenderer(cfg)

	data := makePNG(4, 4, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	sessionID := "test-session"

	// Render at 20x10
	result1, err := r.Render(sessionID, data, 20, 10)
	if err != nil {
		t.Fatalf("20x10 render failed: %v", err)
	}

	// Render at 40x20 - should NOT be from cache
	result2, err := r.Render(sessionID, data, 40, 20)
	if err != nil {
		t.Fatalf("40x20 render failed: %v", err)
	}
	if result2.FromCache {
		t.Error("Different dimensions should not hit cache")
	}

	// Output may or may not differ depending on protocol and scaling
	// The key point is that different dimensions don't share cache entries
	// If both render successfully and cache doesn't share entries, we're good
	if result1.Output == "" || result2.Output == "" {
		t.Error("Both renders should produce non-empty output")
	}
}

func TestOptimizedRenderer_RenderWithoutCache(t *testing.T) {
	cfg := DefaultOptimizedRenderConfig()
	r := NewOptimizedRenderer(cfg)

	data := makePNG(4, 4, color.RGBA{R: 100, G: 100, B: 100, A: 255})

	result, err := r.RenderWithoutCache(data, 20, 10)
	if err != nil {
		t.Fatalf("RenderWithoutCache failed: %v", err)
	}
	if result.FromCache {
		t.Error("RenderWithoutCache should never return cached result")
	}
	if result.Output == "" {
		t.Error("Expected non-empty output")
	}

	// Verify nothing was cached
	count, _ := r.CacheStats()
	if count != 0 {
		t.Errorf("Cache should be empty, has %d entries", count)
	}
}

func TestOptimizedRenderer_InvalidateCache(t *testing.T) {
	cfg := DefaultOptimizedRenderConfig()
	r := NewOptimizedRenderer(cfg)

	data := makePNG(4, 4, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	sessionID := "test-session"

	// Populate cache
	_, err := r.Render(sessionID, data, 20, 10)
	if err != nil {
		t.Fatalf("Initial render failed: %v", err)
	}

	// Verify cache has entry
	count, _ := r.CacheStats()
	if count == 0 {
		t.Fatal("Cache should have entry")
	}

	// Invalidate
	r.InvalidateCache(sessionID)

	// Render again - should not be from cache
	result, err := r.Render(sessionID, data, 20, 10)
	if err != nil {
		t.Fatalf("Render after invalidation failed: %v", err)
	}
	if result.FromCache {
		t.Error("Render after invalidation should not be from cache")
	}
}

func TestOptimizedRenderer_ClearCache(t *testing.T) {
	cfg := DefaultOptimizedRenderConfig()
	r := NewOptimizedRenderer(cfg)

	data := makePNG(4, 4, color.RGBA{R: 100, G: 100, B: 100, A: 255})

	// Populate cache with multiple sessions
	r.Render("session1", data, 20, 10)
	r.Render("session2", data, 20, 10)

	count, _ := r.CacheStats()
	if count < 2 {
		t.Fatalf("Expected at least 2 cache entries, got %d", count)
	}

	// Clear
	r.ClearCache()

	count, _ = r.CacheStats()
	if count != 0 {
		t.Errorf("Cache should be empty after clear, has %d entries", count)
	}
}

func TestOptimizedRenderer_CorruptImage(t *testing.T) {
	cfg := DefaultOptimizedRenderConfig()
	r := NewOptimizedRenderer(cfg)

	_, err := r.Render("session", []byte("not an image"), 20, 10)
	if err == nil {
		t.Error("Expected error for corrupt image data")
	}
}

func TestOptimizedRenderer_EmptyImage(t *testing.T) {
	cfg := DefaultOptimizedRenderConfig()
	r := NewOptimizedRenderer(cfg)

	_, err := r.Render("session", []byte{}, 20, 10)
	if err == nil {
		t.Error("Expected error for empty image data")
	}
}

func TestRenderImageWithFallback(t *testing.T) {
	data := makePNG(4, 4, color.RGBA{R: 100, G: 100, B: 100, A: 255})

	output, err := RenderImageWithFallback(data, 20, 10)
	if err != nil {
		t.Fatalf("RenderImageWithFallback failed: %v", err)
	}
	if output == "" {
		t.Error("Expected non-empty output")
	}
}

func TestRenderImageWithFallback_CorruptData(t *testing.T) {
	_, err := RenderImageWithFallback([]byte("not an image"), 20, 10)
	if err == nil {
		t.Error("Expected error for corrupt data")
	}
}

func TestRenderImageWithFallback_EmptyData(t *testing.T) {
	_, err := RenderImageWithFallback([]byte{}, 20, 10)
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

func TestRenderImageCached(t *testing.T) {
	cache := NewRenderedCache(DefaultRenderedCacheConfig())
	data := makePNG(4, 4, color.RGBA{R: 100, G: 100, B: 100, A: 255})
	sessionID := "test-session"

	// First call - cache miss
	output1, err := RenderImageCached(cache, sessionID, data, 20, 10)
	if err != nil {
		t.Fatalf("First RenderImageCached failed: %v", err)
	}
	if output1 == "" {
		t.Error("Expected non-empty output")
	}

	// Verify entry was cached
	count, _ := cache.Stats()
	if count == 0 {
		t.Error("Expected cache entry")
	}

	// Second call - cache hit
	output2, err := RenderImageCached(cache, sessionID, data, 20, 10)
	if err != nil {
		t.Fatalf("Second RenderImageCached failed: %v", err)
	}
	if output2 != output1 {
		t.Error("Cached output should match original")
	}
}

func TestProtocolToString(t *testing.T) {
	tests := []struct {
		protocol RenderProtocol
		want     string
	}{
		{ProtocolKitty, "kitty"},
		{ProtocolUnicode, "unicode"},
		{ProtocolNone, "none"},
		{RenderProtocol(99), "unknown"},
	}

	for _, tt := range tests {
		got := protocolToString(tt.protocol)
		if got != tt.want {
			t.Errorf("protocolToString(%d) = %q, want %q", tt.protocol, got, tt.want)
		}
	}
}

func TestDefaultOptimizedRenderConfig(t *testing.T) {
	cfg := DefaultOptimizedRenderConfig()

	if cfg.MaxCols != 40 {
		t.Errorf("MaxCols = %d, want 40", cfg.MaxCols)
	}
	if cfg.MaxRows != 20 {
		t.Errorf("MaxRows = %d, want 20", cfg.MaxRows)
	}
	if !cfg.FallbackEnabled {
		t.Error("FallbackEnabled should default to true")
	}
	if cfg.MaxCacheEntries != 50 {
		t.Errorf("MaxCacheEntries = %d, want 50", cfg.MaxCacheEntries)
	}
}
