// Package render provides terminal image rendering protocol detection and rendering.

package render

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"strings"

	"github.com/disintegration/imaging"
)

// kittyChunkSize is the maximum number of base64 bytes per Kitty protocol chunk.
const kittyChunkSize = 4096

// ErrCorruptImage is returned when image data cannot be decoded.
var ErrCorruptImage = errors.New("corrupt or unreadable image data")

// ErrUnsupportedProtocol is returned when no rendering protocol is available.
var ErrUnsupportedProtocol = errors.New("no supported rendering protocol available")

// RenderConfig controls how images are rendered to the terminal.
type RenderConfig struct {
	// Protocol is the preferred image rendering protocol (auto-detect if not set).
	Protocol ImageProtocol
	// MaxCols is the maximum terminal columns for the image.
	MaxCols int
	// MaxRows is the maximum terminal rows for the image.
	MaxRows int
	// UseContextDetection uses SSH/tmux-aware detection.
	UseContextDetection bool
	// FallbackEnabled enables the fallback chain on protocol failure.
	FallbackEnabled bool
}

// DefaultRenderConfig returns sensible defaults for terminal image rendering.
func DefaultRenderConfig() RenderConfig {
	return RenderConfig{
		Protocol:            ProtocolNone, // Auto-detect
		MaxCols:             40,
		MaxRows:             20,
		UseContextDetection: true,
		FallbackEnabled:     true,
	}
}

// RenderResult holds the rendered output and metadata.
type RenderResult struct {
	// Output is the terminal escape sequence string.
	Output string
	// Protocol is the protocol that successfully rendered the image.
	Protocol ImageProtocol
	// FromCache indicates if the result was served from cache.
	FromCache bool
	// RenderTimeMs is the rendering time in milliseconds.
	RenderTimeMs int64
}

// FallbackRenderer implements the fallback chain pattern for image rendering.
// It tries multiple protocols in order of preference, gracefully degrading
// until one succeeds or all fail.
type FallbackRenderer struct {
	config RenderConfig
}

// NewFallbackRenderer creates a FallbackRenderer with the given configuration.
func NewFallbackRenderer(cfg RenderConfig) *FallbackRenderer {
	return &FallbackRenderer{config: cfg}
}

// RenderImage renders image data using the fallback chain.
// The fallback order is:
//  1. Detected/specified protocol
//  2. chafa CLI (handles many protocols automatically)
//  3. Unicode half-blocks (always works)
//
// Returns the rendered output or an error if all protocols fail.
func (r *FallbackRenderer) RenderImage(imageData []byte) (RenderResult, error) {
	if len(imageData) == 0 {
		return RenderResult{}, ErrCorruptImage
	}

	// Validate image can be decoded before attempting render
	if !isValidImage(imageData) {
		return RenderResult{}, ErrCorruptImage
	}

	// Detect protocol if not specified
	protocol := r.config.Protocol
	if protocol == ProtocolNone {
		if r.config.UseContextDetection {
			protocol = DetectProtocolWithContext()
		} else {
			protocol = DetectProtocol()
		}
	}

	// Try the detected/specified protocol first
	output, err := r.tryProtocol(protocol, imageData)
	if err == nil {
		return RenderResult{
			Output:   output,
			Protocol: protocol,
		}, nil
	}

	// Fallback chain (if enabled)
	if !r.config.FallbackEnabled {
		return RenderResult{}, fmt.Errorf("render with %s failed: %w", protocol.String(), err)
	}

	// Try 2: chafa fallback (if not already attempted)
	if protocol != ProtocolChafa && IsChafaAvailable() {
		output, err = r.renderWithChafa(imageData)
		if err == nil {
			return RenderResult{
				Output:   output,
				Protocol: ProtocolChafa,
			}, nil
		}
	}

	// Try 3: Unicode half-blocks (always works)
	output, err = r.renderUnicode(imageData)
	if err == nil {
		return RenderResult{
			Output:   output,
			Protocol: ProtocolUnicode,
		}, nil
	}

	return RenderResult{}, fmt.Errorf("all render protocols failed: %w", err)
}

// tryProtocol attempts to render with a specific protocol.
func (r *FallbackRenderer) tryProtocol(p ImageProtocol, data []byte) (string, error) {
	switch p {
	case ProtocolKitty:
		return r.renderKitty(data)
	case ProtocolSixel:
		return r.renderSixel(data)
	case ProtocolITerm2:
		return r.renderITerm2(data)
	case ProtocolChafa:
		return r.renderWithChafa(data)
	case ProtocolUnicode:
		return r.renderUnicode(data)
	default:
		return "", ErrUnsupportedProtocol
	}
}

// renderKitty encodes image data using the Kitty Graphics Protocol.
func (r *FallbackRenderer) renderKitty(imageData []byte) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("empty image data")
	}

	encoded := base64.StdEncoding.EncodeToString(imageData)
	cols, rows := r.config.MaxCols, r.config.MaxRows

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

// renderSixel encodes image data using the Sixel graphics protocol.
func (r *FallbackRenderer) renderSixel(imageData []byte) (string, error) {
	// Sixel rendering is complex; delegate to chafa which handles it well
	if IsChafaAvailable() {
		return r.renderWithChafaFormat(imageData, "sixel")
	}
	return "", fmt.Errorf("sixel rendering requires chafa CLI")
}

// renderITerm2 encodes image data using the iTerm2 inline images protocol.
func (r *FallbackRenderer) renderITerm2(imageData []byte) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("empty image data")
	}

	encoded := base64.StdEncoding.EncodeToString(imageData)
	cols, rows := r.config.MaxCols, r.config.MaxRows

	// iTerm2 format: OSC 1337 ; File=<args>:<base64 data> ST
	// args: name=<filename>;size=<size>;width=<cols>;height=<rows>;inline=1
	args := fmt.Sprintf("name=image;width=%d;height=%d;inline=1", cols, rows)

	return fmt.Sprintf("\033]1337;File=%s:%s\007", args, encoded), nil
}

// renderWithChafa uses the chafa CLI tool for auto-detected terminal graphics.
func (r *FallbackRenderer) renderWithChafa(imageData []byte) (string, error) {
	return r.renderWithChafaFormat(imageData, "auto")
}

// renderWithChafaFormat uses chafa with a specific output format.
func (r *FallbackRenderer) renderWithChafaFormat(imageData []byte, format string) (string, error) {
	chafaPath, err := exec.LookPath("chafa")
	if err != nil {
		return "", fmt.Errorf("chafa not found: %w", err)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "waifu-render-*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(imageData); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	// Run chafa
	cmd := exec.Command(chafaPath,
		"--size", fmt.Sprintf("%dx%d", r.config.MaxCols, r.config.MaxRows),
		"--format", format,
		tmpFile.Name())

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("chafa failed: %w", err)
	}

	return stdout.String(), nil
}

// renderUnicode decodes the image, resizes it, and renders using half-block
// unicode characters with ANSI 24-bit color.
func (r *FallbackRenderer) renderUnicode(imageData []byte) (string, error) {
	img, _, err := decodeImage(imageData)
	if err != nil {
		return "", fmt.Errorf("unicode render decode: %w", err)
	}

	cols, rows := r.config.MaxCols, r.config.MaxRows
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

// RenderNone returns a placeholder when no image protocol is available.
func RenderNone() string {
	return "(image: protocol not supported)"
}

// Helper functions

// isValidImage checks if the data is a valid decodable image.
func isValidImage(data []byte) bool {
	_, _, err := decodeImage(data)
	return err == nil
}

// decodeImage auto-detects the image format and decodes the raw bytes.
func decodeImage(data []byte) (image.Image, string, error) {
	reader := bytes.NewReader(data)
	img, format, err := image.Decode(reader)
	if err != nil {
		return nil, "", fmt.Errorf("decode: %w", err)
	}
	return img, format, nil
}

// colorToRGBA converts a color.Color to uint8 RGBA components.
func colorToRGBA(c color.Color) (r, g, b, a uint8) {
	r32, g32, b32, a32 := c.RGBA()
	return uint8(r32 >> 8), uint8(g32 >> 8), uint8(b32 >> 8), uint8(a32 >> 8)
}

// ProcessImageForRender resizes and optimizes an image for terminal rendering.
// This is useful for pre-processing images before caching the rendered output.
func ProcessImageForRender(imageData []byte, maxWidth, maxHeight int) ([]byte, error) {
	img, _, err := decodeImage(imageData)
	if err != nil {
		return nil, fmt.Errorf("process image decode: %w", err)
	}

	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	// Calculate dimensions that fit within maxWidth x maxHeight
	newW, newH := calculateDimensions(origW, origH, maxWidth, maxHeight)

	// Only resize if dimensions actually changed
	if newW != origW || newH != origH {
		img = imaging.Fit(img, newW, newH, imaging.Lanczos)
	}

	// Optionally sharpen for better terminal display
	img = imaging.Sharpen(img, 0.5)

	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("process image encode: %w", err)
	}

	return buf.Bytes(), nil
}

// calculateDimensions returns dimensions that fit within maxWidth x maxHeight
// while maintaining the original aspect ratio.
func calculateDimensions(origWidth, origHeight, maxWidth, maxHeight int) (int, int) {
	if origWidth <= 0 || origHeight <= 0 {
		return origWidth, origHeight
	}
	if maxWidth <= 0 || maxHeight <= 0 {
		return origWidth, origHeight
	}

	// Already fits, no resize needed.
	if origWidth <= maxWidth && origHeight <= maxHeight {
		return origWidth, origHeight
	}

	ratioW := float64(maxWidth) / float64(origWidth)
	ratioH := float64(maxHeight) / float64(origHeight)

	ratio := ratioW
	if ratioH < ratioW {
		ratio = ratioH
	}

	newW := int(float64(origWidth) * ratio)
	newH := int(float64(origHeight) * ratio)

	// Ensure at least 1 pixel in each dimension.
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	return newW, newH
}

// Ensure image format decoders are registered
func init() {
	// PNG decoder is registered by default
	// Additional formats are registered through blank imports in the calling code
}

// --- Convenience Functions for Direct Use ---

// Config is an alias for RenderConfig for API clarity.
type Config = RenderConfig

// FormatDiagnostics returns a human-readable string describing the current
// rendering environment and capabilities.
func FormatDiagnostics() string {
	protocol := DetectProtocol()
	contextProtocol := DetectProtocolWithContext()
	caps := GetCapabilities(protocol)

	var b strings.Builder
	b.WriteString("Render Diagnostics:\n")
	fmt.Fprintf(&b, "  Detected Protocol: %s\n", protocol.String())
	fmt.Fprintf(&b, "  Context-Aware: %s\n", contextProtocol.String())
	fmt.Fprintf(&b, "  Protocol Name: %s\n", caps.Name)
	fmt.Fprintf(&b, "  Max Colors: %d\n", caps.MaxColors)
	fmt.Fprintf(&b, "  Transparency: %v\n", caps.SupportsTransparency)
	fmt.Fprintf(&b, "  Animation: %v\n", caps.SupportsAnimation)
	fmt.Fprintf(&b, "  Temp File Required: %v\n", caps.RequiresTempFile)
	fmt.Fprintf(&b, "  SSH Session: %v\n", IsSSHSession())
	fmt.Fprintf(&b, "  Tmux Session: %v\n", IsTmuxSession())
	fmt.Fprintf(&b, "  Chafa Available: %v\n", IsChafaAvailable())

	if IsChafaAvailable() {
		fmt.Fprintf(&b, "  Chafa Version: %s\n", GetChafaVersion())
	}

	return b.String()
}

// DetectAndRender auto-detects the protocol and renders the image.
// This is a convenience function that creates a FallbackRenderer and
// renders the image with context-aware detection.
func DetectAndRender(imageData []byte, cols, rows int) (string, error) {
	cfg := RenderConfig{
		Protocol:            ProtocolNone, // Auto-detect
		MaxCols:             cols,
		MaxRows:             rows,
		UseContextDetection: true,
		FallbackEnabled:     true,
	}
	renderer := NewFallbackRenderer(cfg)

	result, err := renderer.RenderImage(imageData)
	if err != nil {
		return "", err
	}

	return result.Output, nil
}

// Render renders an image with the given configuration.
// This is a convenience function that creates a FallbackRenderer with the
// provided configuration and renders the image.
func Render(imageData []byte, cfg Config) (string, error) {
	renderer := NewFallbackRenderer(cfg)

	result, err := renderer.RenderImage(imageData)
	if err != nil {
		return "", err
	}

	return result.Output, nil
}

// Note: GetChafaVersion is defined in chafa.go
