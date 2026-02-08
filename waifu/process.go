package waifu

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	"github.com/disintegration/imaging"
)

// Ensure decoders are registered.
var (
	_ = gif.Decode
	_ = jpeg.Decode
	_ = png.Decode
)

// ProcessConfig controls how images are resized and encoded for terminal display.
type ProcessConfig struct {
	// MaxWidth is the maximum pixel width for the output image.
	MaxWidth int
	// MaxHeight is the maximum pixel height for the output image.
	MaxHeight int
	// Quality is the JPEG/PNG compression quality (1-100).
	Quality int
	// Sharpen applies a subtle sharpen filter after resize (improves terminal display).
	Sharpen bool
	// OutputFormat is the desired output format: "png" or "jpeg".
	OutputFormat string
}

// DefaultProcessConfig returns sensible defaults for terminal image display.
func DefaultProcessConfig() ProcessConfig {
	return ProcessConfig{
		MaxWidth:     400,
		MaxHeight:    300,
		Quality:      85,
		Sharpen:      true,
		OutputFormat: "png",
	}
}

// ProcessImage decodes, resizes, optionally sharpens, and re-encodes an image.
// It auto-detects the input format (PNG, JPEG, GIF, WebP) and outputs in the
// format specified by cfg.OutputFormat.
func ProcessImage(imageData []byte, cfg ProcessConfig) ([]byte, error) {
	img, _, err := DecodeImage(imageData)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	newW, newH := CalculateDimensions(origW, origH, cfg.MaxWidth, cfg.MaxHeight)

	// Only resize if dimensions actually changed.
	if newW != origW || newH != origH {
		img = imaging.Fit(img, newW, newH, imaging.Lanczos)
	}

	if cfg.Sharpen {
		img = imaging.Sharpen(img, 0.5)
	}

	return EncodeImage(img, cfg.OutputFormat, cfg.Quality)
}

// CalculateDimensions returns dimensions that fit within maxWidth x maxHeight
// while maintaining the original aspect ratio. It never upscales: if the image
// already fits, the original dimensions are returned.
func CalculateDimensions(origWidth, origHeight, maxWidth, maxHeight int) (int, int) {
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

// DecodeImage auto-detects the image format and decodes the raw bytes.
// Supported formats: PNG, JPEG, GIF (via registered decoders).
func DecodeImage(data []byte) (image.Image, string, error) {
	r := bytes.NewReader(data)
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, "", fmt.Errorf("decode: %w", err)
	}
	return img, format, nil
}

// EncodeImage writes img into the specified format ("png" or "jpeg") and
// returns the encoded bytes. The quality parameter is used for JPEG encoding.
func EncodeImage(img image.Image, format string, quality int) ([]byte, error) {
	var buf bytes.Buffer

	switch format {
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("encode png: %w", err)
		}
	case "jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, fmt.Errorf("encode jpeg: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported output format: %q", format)
	}

	return buf.Bytes(), nil
}

// TerminalPixelDimensions calculates the total pixel dimensions available
// for a given terminal size. Default cell size is 8x16 pixels (standard terminal).
func TerminalPixelDimensions(cols, rows, cellWidth, cellHeight int) (int, int) {
	if cellWidth <= 0 {
		cellWidth = 8
	}
	if cellHeight <= 0 {
		cellHeight = 16
	}
	return cols * cellWidth, rows * cellHeight
}
