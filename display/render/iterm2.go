package render

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// iTerm2Config controls iTerm2 inline image rendering.
type ITerm2Config struct {
	// Width specifies the display width. Can be:
	// - "auto" or empty: use image dimensions
	// - "N": N cells wide
	// - "Npx": N pixels wide
	// - "N%": N percent of terminal width
	Width string
	// Height specifies the display height (same format as Width).
	Height string
	// PreserveAspect maintains aspect ratio when Width or Height is specified.
	PreserveAspect bool
	// Inline displays the image inline (true) or downloads it (false).
	Inline bool
	// Name is an optional filename for the image (used for downloads).
	Name string
}

// DefaultITerm2Config returns sensible defaults for iTerm2 rendering.
func DefaultITerm2Config() ITerm2Config {
	return ITerm2Config{
		Width:          "auto",
		Height:         "auto",
		PreserveAspect: true,
		Inline:         true,
		Name:           "",
	}
}

// RenderITerm2 encodes image data using the iTerm2 inline images protocol.
//
// The iTerm2 inline images protocol uses OSC (Operating System Command):
//   - OSC 1337 ; File= parameters : base64data BEL
//   - \033]1337;File=...:<base64>\007
//
// Parameters are semicolon-separated key=value pairs.
//
// Protocol documentation: https://iterm2.com/documentation-images.html
func RenderITerm2(imageData []byte, cfg ITerm2Config) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("empty image data")
	}

	encoded := base64.StdEncoding.EncodeToString(imageData)
	params := buildITerm2Params(cfg, len(imageData))

	var b strings.Builder

	// OSC 1337 ; File= params : base64 BEL
	// Using \033]...\007 format (ESC ] ... BEL)
	fmt.Fprintf(&b, "\033]1337;File=%s:%s\007", params, encoded)

	return b.String(), nil
}

// buildITerm2Params constructs the iTerm2 protocol parameters.
func buildITerm2Params(cfg ITerm2Config, dataSize int) string {
	var params []string

	// Inline parameter: 1 = display inline, 0 = download
	if cfg.Inline {
		params = append(params, "inline=1")
	} else {
		params = append(params, "inline=0")
	}

	// Size parameter (original data size before base64)
	params = append(params, fmt.Sprintf("size=%d", dataSize))

	// Width specification
	if cfg.Width != "" && cfg.Width != "auto" {
		params = append(params, fmt.Sprintf("width=%s", cfg.Width))
	}

	// Height specification
	if cfg.Height != "" && cfg.Height != "auto" {
		params = append(params, fmt.Sprintf("height=%s", cfg.Height))
	}

	// Preserve aspect ratio
	if cfg.PreserveAspect {
		params = append(params, "preserveAspectRatio=1")
	} else {
		params = append(params, "preserveAspectRatio=0")
	}

	// Optional filename
	if cfg.Name != "" {
		// Base64 encode the filename
		encodedName := base64.StdEncoding.EncodeToString([]byte(cfg.Name))
		params = append(params, fmt.Sprintf("name=%s", encodedName))
	}

	return strings.Join(params, ";")
}

// RenderITerm2WithCells renders an image sized to specific terminal cell dimensions.
func RenderITerm2WithCells(imageData []byte, cols, rows int) (string, error) {
	cfg := ITerm2Config{
		Width:          fmt.Sprintf("%d", cols),
		Height:         fmt.Sprintf("%d", rows),
		PreserveAspect: true,
		Inline:         true,
	}
	return RenderITerm2(imageData, cfg)
}

// RenderITerm2WithPixels renders an image at specific pixel dimensions.
func RenderITerm2WithPixels(imageData []byte, widthPx, heightPx int) (string, error) {
	cfg := ITerm2Config{
		Width:          fmt.Sprintf("%dpx", widthPx),
		Height:         fmt.Sprintf("%dpx", heightPx),
		PreserveAspect: true,
		Inline:         true,
	}
	return RenderITerm2(imageData, cfg)
}

// RenderITerm2AutoSize renders an image at its native size.
func RenderITerm2AutoSize(imageData []byte) (string, error) {
	cfg := DefaultITerm2Config()
	return RenderITerm2(imageData, cfg)
}

// ITerm2ImageLink creates a clickable image that opens a URL when clicked.
// This combines the inline image with an OSC 8 hyperlink.
func ITerm2ImageLink(imageData []byte, url string, cfg ITerm2Config) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("empty image data")
	}

	imageOutput, err := RenderITerm2(imageData, cfg)
	if err != nil {
		return "", err
	}

	// Wrap in OSC 8 hyperlink
	// Format: OSC 8 ; params ; URI ST text OSC 8 ; ; ST
	// Using \033]8;;url\033\\ text \033]8;;\033\\
	var b strings.Builder
	fmt.Fprintf(&b, "\033]8;;%s\033\\%s\033]8;;\033\\", url, imageOutput)

	return b.String(), nil
}

// ClearITerm2Image attempts to clear inline images.
// Note: iTerm2 doesn't have a native clear command like Kitty.
// This outputs enough newlines to scroll the image off screen.
func ClearITerm2Image(rows int) string {
	if rows <= 0 {
		rows = 24
	}
	return strings.Repeat("\n", rows)
}

// IsITerm2Available checks if we're likely running in iTerm2.
// This is a convenience function for protocol selection.
func IsITerm2Available() bool {
	return DetectProtocol() == ProtocolITerm2
}
