package render

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ChafaConfig controls how chafa renders images.
type ChafaConfig struct {
	// Cols is the output width in terminal columns.
	Cols int
	// Rows is the output height in terminal rows.
	Rows int
	// Format overrides auto-detection: "kitty", "iterm", "sixel", "symbols", or "auto".
	Format string
	// Colors is the color mode: 256, "truecolor", or "none".
	Colors string
	// Symbols specifies which character sets to use: "block", "all", "ascii", etc.
	Symbols string
	// Dither enables dithering for better color approximation.
	Dither bool
	// WorkDir is the directory for temp files (defaults to OS temp).
	WorkDir string
}

// DefaultChafaConfig returns sensible defaults for terminal rendering.
func DefaultChafaConfig() ChafaConfig {
	return ChafaConfig{
		Cols:    40,
		Rows:    20,
		Format:  "auto",
		Colors:  "truecolor",
		Symbols: "all",
		Dither:  true,
		WorkDir: "",
	}
}

// RenderWithChafa uses the chafa CLI to render image data to terminal output.
// Chafa automatically detects the best output protocol for the current terminal.
//
// The function:
//  1. Creates a temporary file with the image data
//  2. Invokes chafa with the specified configuration
//  3. Returns the terminal escape sequences for display
//  4. Cleans up the temporary file
func RenderWithChafa(imageData []byte, cfg ChafaConfig) (string, error) {
	// Verify chafa is available
	chafaPath, err := exec.LookPath("chafa")
	if err != nil {
		return "", fmt.Errorf("chafa not found in PATH: %w", err)
	}

	// Create temp file for the image
	tmpDir := cfg.WorkDir
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}

	tmpFile, err := os.CreateTemp(tmpDir, "waifu-*.png")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write image data
	if _, err := tmpFile.Write(imageData); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	// Build chafa command arguments
	args := buildChafaArgs(cfg, tmpPath)

	// Execute chafa
	cmd := exec.Command(chafaPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			return "", fmt.Errorf("chafa failed: %s", errMsg)
		}
		return "", fmt.Errorf("chafa failed: %w", err)
	}

	return stdout.String(), nil
}

// buildChafaArgs constructs the chafa command line arguments.
func buildChafaArgs(cfg ChafaConfig, imagePath string) []string {
	args := []string{}

	// Size specification
	if cfg.Cols > 0 && cfg.Rows > 0 {
		args = append(args, "--size", fmt.Sprintf("%dx%d", cfg.Cols, cfg.Rows))
	} else if cfg.Cols > 0 {
		args = append(args, "--size", fmt.Sprintf("%d", cfg.Cols))
	}

	// Output format
	switch strings.ToLower(cfg.Format) {
	case "kitty":
		args = append(args, "--format", "kitty")
	case "iterm", "iterm2":
		args = append(args, "--format", "iterm")
	case "sixel":
		args = append(args, "--format", "sixel")
	case "symbols":
		args = append(args, "--format", "symbols")
	default:
		// Auto-detection is the default, no flag needed
	}

	// Color mode
	switch strings.ToLower(cfg.Colors) {
	case "truecolor", "24bit", "true":
		args = append(args, "--colors", "full")
	case "256":
		args = append(args, "--colors", "256")
	case "16":
		args = append(args, "--colors", "16")
	case "none", "mono":
		args = append(args, "--colors", "2")
	// default: let chafa auto-detect
	}

	// Symbol selection
	if cfg.Symbols != "" {
		args = append(args, "--symbols", cfg.Symbols)
	}

	// Dithering
	if cfg.Dither {
		args = append(args, "--dither", "ordered")
	}

	// Add the image path last
	args = append(args, imagePath)

	return args
}

// GetChafaVersion returns the installed chafa version, or empty string if not found.
func GetChafaVersion() string {
	chafaPath, err := exec.LookPath("chafa")
	if err != nil {
		return ""
	}

	cmd := exec.Command(chafaPath, "--version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return ""
	}

	// Parse version from output (first line typically)
	output := stdout.String()
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		// Extract version number from "chafa version X.Y.Z"
		parts := strings.Fields(lines[0])
		for i, p := range parts {
			if p == "version" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
		// If no "version" keyword, return first line
		return strings.TrimSpace(lines[0])
	}
	return ""
}

// ChafaCapabilities returns what the installed chafa supports.
type ChafaCapabilities struct {
	// Version is the chafa version string.
	Version string
	// SupportsKitty indicates if chafa supports Kitty output.
	SupportsKitty bool
	// SupportsITerm indicates if chafa supports iTerm2 output.
	SupportsITerm bool
	// SupportsSixel indicates if chafa supports Sixel output.
	SupportsSixel bool
}

// DetectChafaCapabilities probes the installed chafa for its capabilities.
func DetectChafaCapabilities() ChafaCapabilities {
	caps := ChafaCapabilities{
		Version: GetChafaVersion(),
	}

	if caps.Version == "" {
		return caps
	}

	// Query chafa's supported formats
	chafaPath, err := exec.LookPath("chafa")
	if err != nil {
		return caps
	}

	cmd := exec.Command(chafaPath, "--help")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run() // Ignore error, just check output

	helpOutput := stdout.String()

	// Check for format support in help output
	caps.SupportsKitty = strings.Contains(helpOutput, "kitty")
	caps.SupportsITerm = strings.Contains(helpOutput, "iterm")
	caps.SupportsSixel = strings.Contains(helpOutput, "sixel")

	return caps
}
