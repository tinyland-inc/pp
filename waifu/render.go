package waifu

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/color"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/disintegration/imaging"
)

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
func DetectProtocol() RenderProtocol {
	termProgram := os.Getenv("TERM_PROGRAM")
	switch strings.ToLower(termProgram) {
	case "ghostty", "kitty", "wezterm":
		return ProtocolKitty
	}

	if os.Getenv("TERM") == "xterm-kitty" {
		return ProtocolKitty
	}

	if os.Getenv("KITTY_WINDOW_ID") != "" {
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
func renderChafa(imageData []byte, cols, rows int) (string, error) {
	// Check if chafa is available
	chafaPath, err := exec.LookPath("chafa")
	if err != nil {
		return "", fmt.Errorf("chafa not found: %w", err)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "waifu-*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(imageData); err != nil {
		tmpFile.Close()
		return "", err
	}
	tmpFile.Close()

	// Run chafa with auto-detection
	cmd := exec.Command(chafaPath,
		"--size", fmt.Sprintf("%dx%d", cols, rows),
		"--format", "auto", // Auto-detect Kitty/iTerm2/Sixel/symbols
		tmpFile.Name())

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("chafa failed: %w", err)
	}

	return stdout.String(), nil
}

// colorToRGBA converts a color.Color to uint8 RGBA components.
func colorToRGBA(c color.Color) (r, g, b, a uint8) {
	r32, g32, b32, a32 := c.RGBA()
	return uint8(r32 >> 8), uint8(g32 >> 8), uint8(b32 >> 8), uint8(a32 >> 8)
}
