package waifu

import (
	"encoding/base64"
	"fmt"
	"image/color"
	"io"
	"os"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"gitlab.com/tinyland/lab/prompt-pulse/display/render"
)

// timeNow returns the current time. Wrapped for testing.
func timeNow() time.Time {
	return time.Now()
}

// timeSinceMs returns milliseconds since the given time.
func timeSinceMs(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

// RenderProtocol identifies which image rendering protocol to use.
type RenderProtocol int

const (
	// ProtocolKitty uses the Kitty Graphics Protocol (supported by Ghostty, Kitty, WezTerm).
	ProtocolKitty RenderProtocol = iota
	// ProtocolUnicode uses half-block unicode characters with ANSI 24-bit color.
	ProtocolUnicode
	// ProtocolNone indicates no image rendering support.
	ProtocolNone
)

// kittyChunkSize is the maximum number of base64 bytes per Kitty protocol chunk.
const kittyChunkSize = 4096

// RenderConfig controls how images are rendered to the terminal.
type RenderConfig struct {
	// Protocol is the image rendering protocol to use.
	Protocol RenderProtocol
	// MaxCols is the maximum terminal columns for the image.
	MaxCols int
	// MaxRows is the maximum terminal rows for the image.
	MaxRows int
	// Writer is where to write the escape sequences (default: os.Stdout).
	Writer io.Writer
}

// DefaultRenderConfig returns sensible defaults for terminal image rendering.
func DefaultRenderConfig() RenderConfig {
	return RenderConfig{
		Protocol: ProtocolKitty,
		MaxCols:  40,
		MaxRows:  20,
		Writer:   os.Stdout,
	}
}

// DetectProtocol inspects environment variables to determine which image
// rendering protocol the current terminal supports.
// It is SSH-aware: over SSH sessions, Kitty protocol is downgraded to Unicode
// because the Kitty Graphics Protocol produces garbled output when TERM_PROGRAM
// is inherited but the SSH transport cannot carry the binary escape sequences.
func DetectProtocol() RenderProtocol {
	// Check SSH first: if we're in an SSH session, never return Kitty.
	// TERM_PROGRAM and KITTY_WINDOW_ID are often inherited over SSH
	// but the Kitty Graphics Protocol doesn't work over SSH transport.
	isSSH := render.IsSSHSession()

	termProgram := os.Getenv("TERM_PROGRAM")
	switch strings.ToLower(termProgram) {
	case "ghostty", "kitty", "wezterm":
		if isSSH {
			return ProtocolUnicode
		}
		return ProtocolKitty
	}

	if os.Getenv("TERM") == "xterm-kitty" {
		if isSSH {
			return ProtocolUnicode
		}
		return ProtocolKitty
	}

	if os.Getenv("KITTY_WINDOW_ID") != "" {
		if isSSH {
			return ProtocolUnicode
		}
		return ProtocolKitty
	}

	return ProtocolUnicode
}

// RenderImage renders imageData using the protocol specified in cfg. It returns
// the string containing the appropriate escape sequences (or unicode art).
func RenderImage(imageData []byte, cfg RenderConfig) (string, error) {
	// Try chafa first (handles all protocols automatically)
	if result, err := renderChafa(imageData, cfg.MaxCols, cfg.MaxRows); err == nil {
		return result, nil
	}

	// Fall back to native implementation
	switch cfg.Protocol {
	case ProtocolKitty:
		return renderKitty(imageData, cfg.MaxCols, cfg.MaxRows)
	case ProtocolUnicode:
		return renderUnicode(imageData, cfg.MaxCols, cfg.MaxRows)
	case ProtocolNone:
		return renderNone()
	default:
		return renderNone()
	}
}

// renderKitty encodes image data using the Kitty Graphics Protocol.
// The data is base64-encoded and chunked into segments of at most kittyChunkSize bytes.
func renderKitty(imageData []byte, cols, rows int) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("empty image data")
	}

	encoded := base64.StdEncoding.EncodeToString(imageData)

	var b strings.Builder

	if len(encoded) <= kittyChunkSize {
		// Single chunk: m=0 means this is the only (and last) chunk.
		fmt.Fprintf(&b, "\033_Gf=100,a=T,t=d,c=%d,r=%d,m=0;%s\033\\", cols, rows, encoded)
		return b.String(), nil
	}

	// Multiple chunks needed.
	for i := 0; i < len(encoded); i += kittyChunkSize {
		end := i + kittyChunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]
		isLast := end >= len(encoded)

		if i == 0 {
			// First chunk includes all metadata.
			fmt.Fprintf(&b, "\033_Gf=100,a=T,t=d,c=%d,r=%d,m=1;%s\033\\", cols, rows, chunk)
		} else if isLast {
			fmt.Fprintf(&b, "\033_Gm=0;%s\033\\", chunk)
		} else {
			fmt.Fprintf(&b, "\033_Gm=1;%s\033\\", chunk)
		}
	}

	return b.String(), nil
}

// renderUnicode decodes the image, resizes it, and renders using half-block
// unicode characters with ANSI 24-bit color. Each character cell represents
// two vertical pixels using the upper half-block character.
func renderUnicode(imageData []byte, cols, rows int) (string, error) {
	img, _, err := DecodeImage(imageData)
	if err != nil {
		return "", fmt.Errorf("unicode render decode: %w", err)
	}

	// Each row of text represents 2 pixel rows via half-block characters.
	pixelWidth := cols
	pixelHeight := rows * 2

	resized := imaging.Fit(img, pixelWidth, pixelHeight, imaging.Lanczos)
	bounds := resized.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var b strings.Builder

	// Process pixel rows in pairs.
	for y := 0; y < h; y += 2 {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x := 0; x < w; x++ {
			topR, topG, topB, _ := colorToRGBA(resized.At(bounds.Min.X+x, bounds.Min.Y+y))

			var botR, botG, botB uint8
			if y+1 < h {
				botR, botG, botB, _ = colorToRGBA(resized.At(bounds.Min.X+x, bounds.Min.Y+y+1))
			}

			// Foreground = top pixel, background = bottom pixel, character = upper half-block.
			fmt.Fprintf(&b, "\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm\u2580\033[0m",
				topR, topG, topB, botR, botG, botB)
		}
	}

	return b.String(), nil
}

// renderNone returns a placeholder when no image protocol is available.
func renderNone() (string, error) {
	return "(image: protocol not supported)", nil
}

// renderChafa uses the chafa CLI tool for auto-detected terminal graphics.
// Delegates to the high-quality render.RenderWithChafa() with full ChafaConfig support
// (truecolor, all symbols, dithering) instead of the simple --size/--format flags.
//
// Over SSH sessions, the format is forced to "symbols" to prevent chafa from
// auto-detecting Kitty protocol via inherited TERM_PROGRAM, which would produce
// garbled binary escape sequences that the SSH transport cannot carry.
func renderChafa(imageData []byte, cols, rows int) (string, error) {
	cfg := render.DefaultChafaConfig()
	cfg.Cols = cols
	cfg.Rows = rows
	// Force symbols format over SSH to prevent chafa from auto-detecting
	// Kitty/iTerm2/Sixel protocols that don't work over SSH transport.
	if render.IsSSHSession() {
		cfg.Format = "symbols"
	}
	return render.RenderWithChafa(imageData, cfg)
}

// colorToRGBA converts a color.Color to uint8 RGBA components.
func colorToRGBA(c color.Color) (r, g, b, a uint8) {
	r32, g32, b32, a32 := c.RGBA()
	return uint8(r32 >> 8), uint8(g32 >> 8), uint8(b32 >> 8), uint8(a32 >> 8)
}

// --- Modular Render Package Integration ---
// The following functions bridge to the new display/render package which provides
// enhanced protocol detection including iTerm2 and Sixel support.

// DetectImageProtocol returns the auto-detected image rendering protocol.
// This uses the new modular render package with enhanced detection.
func DetectImageProtocol() string {
	return render.DetectProtocol().String()
}

// DetectImageProtocolWithContext returns the protocol considering SSH/tmux degradation.
func DetectImageProtocolWithContext() string {
	return render.DetectProtocolWithContext().String()
}

// GetRenderDiagnostics returns diagnostic information about the rendering environment.
func GetRenderDiagnostics() string {
	return render.FormatDiagnostics()
}

// RenderImageModular renders image data using the new modular render package.
// This provides access to all supported protocols: Kitty, iTerm2, Sixel, Chafa, Unicode.
func RenderImageModular(imageData []byte, cols, rows int) (string, error) {
	return render.DetectAndRender(imageData, cols, rows)
}

// RenderImageWithProtocol renders using a specific protocol from the modular package.
// Supported protocols: "kitty", "iterm2", "sixel", "chafa", "unicode", "none".
func RenderImageWithProtocol(imageData []byte, protocol string, cols, rows int) (string, error) {
	var p render.ImageProtocol
	switch strings.ToLower(protocol) {
	case "kitty":
		p = render.ProtocolKitty
	case "iterm2", "iterm":
		p = render.ProtocolITerm2
	case "sixel":
		p = render.ProtocolSixel
	case "chafa":
		p = render.ProtocolChafa
	case "unicode":
		p = render.ProtocolUnicode
	default:
		p = render.ProtocolNone
	}

	cfg := render.Config{
		Protocol:        p,
		MaxCols:         cols,
		MaxRows:         rows,
		FallbackEnabled: false, // Don't fallback when specific protocol is requested
	}
	return render.Render(imageData, cfg)
}

// IsChafaAvailable returns true if the chafa CLI is installed.
func IsChafaAvailable() bool {
	return render.IsChafaAvailable()
}

// GetChafaVersion returns the installed chafa version, or empty string if not found.
func GetChafaVersion() string {
	return render.GetChafaVersion()
}

// ErrCorruptImage is returned when image data cannot be decoded.
var ErrCorruptImage = fmt.Errorf("corrupt or unreadable image data")

// OptimizedRenderConfig controls the optimized renderer behavior.
type OptimizedRenderConfig struct {
	// MaxCols is the maximum terminal columns for the image.
	MaxCols int
	// MaxRows is the maximum terminal rows for the image.
	MaxRows int
	// FallbackEnabled enables the protocol fallback chain.
	FallbackEnabled bool
	// MaxCacheEntries is the maximum number of rendered outputs to cache.
	MaxCacheEntries int
}

// DefaultOptimizedRenderConfig returns sensible defaults.
func DefaultOptimizedRenderConfig() OptimizedRenderConfig {
	return OptimizedRenderConfig{
		MaxCols:         40,
		MaxRows:         20,
		FallbackEnabled: true,
		MaxCacheEntries: 50,
	}
}

// OptimizedRenderer provides high-performance image rendering with caching.
// It combines the fallback chain pattern with rendered output caching
// to achieve <100ms render times from cache.
type OptimizedRenderer struct {
	// renderedCache caches terminal escape sequences
	renderedCache *RenderedCache
	// config holds rendering configuration
	config OptimizedRenderConfig
}

// NewOptimizedRenderer creates an OptimizedRenderer with the given configuration.
func NewOptimizedRenderer(cfg OptimizedRenderConfig) *OptimizedRenderer {
	renderedCacheCfg := DefaultRenderedCacheConfig()
	renderedCacheCfg.MaxEntries = cfg.MaxCacheEntries

	return &OptimizedRenderer{
		renderedCache: NewRenderedCache(renderedCacheCfg),
		config:        cfg,
	}
}

// RenderOutput holds the result of an optimized render operation.
type RenderOutput struct {
	// Output is the terminal escape sequence string.
	Output string
	// Protocol is the protocol that was used.
	Protocol string
	// FromCache indicates if the result was served from the rendered cache.
	FromCache bool
	// RenderTimeMs is the rendering time in milliseconds.
	RenderTimeMs int64
}

// Render renders image data using the optimized fallback chain with caching.
// It first checks the rendered cache for a matching entry, then falls back
// to actual rendering if not cached.
func (r *OptimizedRenderer) Render(sessionID string, imageData []byte, cols, rows int) (RenderOutput, error) {
	start := timeNow()

	// Detect protocol for cache key
	protocol := DetectProtocol()
	protocolStr := protocolToString(protocol)

	// Check rendered cache first - try detected protocol and chafa (renderWithFallback may use either)
	for _, proto := range []string{protocolStr, "chafa"} {
		if cached, exists := r.renderedCache.Get(sessionID, proto, cols, rows); exists {
			return RenderOutput{
				Output:       cached.Output,
				Protocol:     cached.Protocol,
				FromCache:    true,
				RenderTimeMs: timeSinceMs(start),
			}, nil
		}
	}

	// Not in cache, perform actual rendering with fallback chain
	output, usedProtocol, err := r.renderWithFallback(imageData, cols, rows)
	if err != nil {
		return RenderOutput{}, fmt.Errorf("optimized render failed: %w", err)
	}

	// Cache the result
	r.renderedCache.Put(sessionID, usedProtocol, cols, rows, output)

	return RenderOutput{
		Output:       output,
		Protocol:     usedProtocol,
		FromCache:    false,
		RenderTimeMs: timeSinceMs(start),
	}, nil
}

// renderWithFallback implements the fallback chain pattern.
// Fallback order:
//  1. chafa CLI (if available) - handles all protocols automatically
//  2. Detected native protocol (Kitty or Unicode)
//  3. Unicode half-blocks (always works)
func (r *OptimizedRenderer) renderWithFallback(imageData []byte, cols, rows int) (string, string, error) {
	if len(imageData) == 0 {
		return "", "", ErrCorruptImage
	}

	// Validate image can be decoded
	if _, _, err := DecodeImage(imageData); err != nil {
		return "", "", ErrCorruptImage
	}

	// Try 1: chafa (handles multiple protocols automatically)
	if output, err := renderChafa(imageData, cols, rows); err == nil {
		return output, "chafa", nil
	}

	// Try 2: Native protocol based on detection
	protocol := DetectProtocol()
	switch protocol {
	case ProtocolKitty:
		if output, err := renderKitty(imageData, cols, rows); err == nil {
			return output, "kitty", nil
		}
	}

	// Try 3: Unicode half-blocks (always works)
	output, err := renderUnicode(imageData, cols, rows)
	if err != nil {
		return "", "", fmt.Errorf("all render protocols failed: %w", err)
	}

	return output, "unicode", nil
}

// RenderWithoutCache renders image data without using the cache.
// Useful for one-off renders or testing.
func (r *OptimizedRenderer) RenderWithoutCache(imageData []byte, cols, rows int) (RenderOutput, error) {
	start := timeNow()

	output, protocol, err := r.renderWithFallback(imageData, cols, rows)
	if err != nil {
		return RenderOutput{}, err
	}

	return RenderOutput{
		Output:       output,
		Protocol:     protocol,
		FromCache:    false,
		RenderTimeMs: timeSinceMs(start),
	}, nil
}

// InvalidateCache clears the rendered cache for a specific session.
func (r *OptimizedRenderer) InvalidateCache(sessionID string) {
	r.renderedCache.InvalidateSession(sessionID)
}

// ClearCache clears all cached rendered outputs.
func (r *OptimizedRenderer) ClearCache() {
	r.renderedCache.Clear()
}

// CacheStats returns statistics about the rendered cache.
func (r *OptimizedRenderer) CacheStats() (entryCount int, totalBytes int64) {
	return r.renderedCache.Stats()
}

// protocolToString converts a RenderProtocol to a string.
func protocolToString(p RenderProtocol) string {
	switch p {
	case ProtocolKitty:
		return "kitty"
	case ProtocolUnicode:
		return "unicode"
	case ProtocolNone:
		return "none"
	default:
		return "unknown"
	}
}

// RenderImageWithFallback renders an image using the fallback chain.
// This is a convenience function for one-off renders.
//
// Fallback order:
//  1. chafa CLI (if available)
//  2. Detected native protocol
//  3. Unicode half-blocks (always works)
func RenderImageWithFallback(imageData []byte, cols, rows int) (string, error) {
	r := NewOptimizedRenderer(OptimizedRenderConfig{
		MaxCols:         cols,
		MaxRows:         rows,
		FallbackEnabled: true,
		MaxCacheEntries: 1, // Minimal cache for one-off
	})

	result, err := r.RenderWithoutCache(imageData, cols, rows)
	if err != nil {
		return "", err
	}

	return result.Output, nil
}

// RenderImageCached renders an image using the specified cache for rendered outputs.
// It first checks the cache, then falls back to actual rendering.
//
// Parameters:
//   - cache: The rendered output cache
//   - sessionID: Unique identifier for the current session/image
//   - imageData: Raw image bytes (PNG, JPEG, GIF)
//   - cols, rows: Terminal dimensions for rendering
//
// Returns the rendered output string and any error.
func RenderImageCached(cache *RenderedCache, sessionID string, imageData []byte, cols, rows int) (string, error) {
	if len(imageData) == 0 {
		return "", ErrCorruptImage
	}

	// Detect protocol for cache key
	protocol := DetectProtocol()
	protocolStr := protocolToString(protocol)

	// Check cache first
	if cached, exists := cache.Get(sessionID, protocolStr, cols, rows); exists {
		return cached.Output, nil
	}

	// Render with fallback
	output, err := RenderImageWithFallback(imageData, cols, rows)
	if err != nil {
		return "", err
	}

	// Cache the result
	cache.Put(sessionID, protocolStr, cols, rows, output)

	return output, nil
}
