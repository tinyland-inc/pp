package banner

import (
	"os"
	"testing"
)

func TestDetectTerminalSize_Defaults(t *testing.T) {
	// Clear environment variables to test defaults
	os.Unsetenv("COLUMNS")
	os.Unsetenv("LINES")

	w, h := DetectTerminalSize()

	// Should return reasonable values (either detected or defaults)
	if w <= 0 {
		t.Errorf("width should be positive, got %d", w)
	}
	if h <= 0 {
		t.Errorf("height should be positive, got %d", h)
	}
}

func TestDetectTerminalSize_EnvVariables(t *testing.T) {
	// Save original values
	origCols := os.Getenv("COLUMNS")
	origLines := os.Getenv("LINES")
	defer func() {
		if origCols != "" {
			os.Setenv("COLUMNS", origCols)
		} else {
			os.Unsetenv("COLUMNS")
		}
		if origLines != "" {
			os.Setenv("LINES", origLines)
		} else {
			os.Unsetenv("LINES")
		}
	}()

	// Test with environment variables (only effective if TTY detection fails)
	os.Setenv("COLUMNS", "120")
	os.Setenv("LINES", "40")

	w, h := DetectTerminalSize()

	// Should return positive values
	if w <= 0 {
		t.Errorf("width should be positive, got %d", w)
	}
	if h <= 0 {
		t.Errorf("height should be positive, got %d", h)
	}
}

func TestDetectTerminalSize_InvalidEnv(t *testing.T) {
	// Save original values
	origCols := os.Getenv("COLUMNS")
	origLines := os.Getenv("LINES")
	defer func() {
		if origCols != "" {
			os.Setenv("COLUMNS", origCols)
		} else {
			os.Unsetenv("COLUMNS")
		}
		if origLines != "" {
			os.Setenv("LINES", origLines)
		} else {
			os.Unsetenv("LINES")
		}
	}()

	// Test with invalid environment variables
	os.Setenv("COLUMNS", "invalid")
	os.Setenv("LINES", "-5")

	w, h := DetectTerminalSize()

	// Should fall back to defaults (80x24) or TTY detection
	if w <= 0 {
		t.Errorf("width should be positive even with invalid env, got %d", w)
	}
	if h <= 0 {
		t.Errorf("height should be positive even with invalid env, got %d", h)
	}
}
