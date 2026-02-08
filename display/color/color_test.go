package color

import (
	"os"
	"strings"
	"testing"
)

func TestShouldDisableColor_NOCOLORSet(t *testing.T) {
	// Save and restore NO_COLOR.
	orig, hadOrig := os.LookupEnv("NO_COLOR")
	defer func() {
		if hadOrig {
			os.Setenv("NO_COLOR", orig)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	// Set NO_COLOR to various values - all should disable color.
	for _, val := range []string{"", "1", "true", "anything"} {
		os.Setenv("NO_COLOR", val)
		if !ShouldDisableColor() {
			t.Errorf("ShouldDisableColor() = false with NO_COLOR=%q, want true", val)
		}
	}
}

func TestShouldDisableColor_NOCOLORUnset(t *testing.T) {
	// Save and restore NO_COLOR.
	orig, hadOrig := os.LookupEnv("NO_COLOR")
	defer func() {
		if hadOrig {
			os.Setenv("NO_COLOR", orig)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	os.Unsetenv("NO_COLOR")

	// In test environments, stdout is typically not a terminal, so
	// ShouldDisableColor may return true due to pipe detection.
	// We just verify it does not panic and returns a bool.
	_ = ShouldDisableColor()
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "strips color codes",
			input: "\x1b[31mred text\x1b[0m",
			want:  "red text",
		},
		{
			name:  "strips bold",
			input: "\x1b[1mbold\x1b[0m normal",
			want:  "bold normal",
		},
		{
			name:  "strips multiple sequences",
			input: "\x1b[1;31;40mstyle\x1b[0m gap \x1b[32mgreen\x1b[0m",
			want:  "style gap green",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "cursor control stripped",
			input: "\x1b[?25h",
			want:  "",
		},
		{
			name:  "preserves unicode",
			input: "CPU \x1b[32m45%\x1b[0m | RAM 62%",
			want:  "CPU 45% | RAM 62%",
		},
		{
			name:  "preserves sparkline blocks",
			input: "\x1b[36m▁▂▃▄▅▆▇█\x1b[0m",
			want:  "▁▂▃▄▅▆▇█",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.input)
			if got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestApply_NOCOLORSet(t *testing.T) {
	// Save and restore NO_COLOR.
	orig, hadOrig := os.LookupEnv("NO_COLOR")
	defer func() {
		if hadOrig {
			os.Setenv("NO_COLOR", orig)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	os.Setenv("NO_COLOR", "1")
	enabled := Apply()
	if enabled {
		t.Error("Apply() should return false when NO_COLOR is set")
	}
}

func TestApply_InTestEnv(t *testing.T) {
	// Save and restore NO_COLOR.
	orig, hadOrig := os.LookupEnv("NO_COLOR")
	defer func() {
		if hadOrig {
			os.Setenv("NO_COLOR", orig)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	os.Unsetenv("NO_COLOR")

	// In test environments, stdout is typically not a TTY (piped to test runner),
	// so Apply should return false for pipe detection.
	enabled := Apply()
	// We cannot strictly assert true/false here because it depends on the test runner,
	// but we verify it doesn't panic.
	_ = enabled
}

func TestForceDisable(t *testing.T) {
	// ForceDisable should not panic.
	ForceDisable()
}

func TestShouldDisableColor_NOCOLOREmptyString(t *testing.T) {
	// Per the NO_COLOR spec, an empty value still means "disable color".
	orig, hadOrig := os.LookupEnv("NO_COLOR")
	defer func() {
		if hadOrig {
			os.Setenv("NO_COLOR", orig)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	os.Setenv("NO_COLOR", "")
	if !ShouldDisableColor() {
		t.Error("ShouldDisableColor() = false with NO_COLOR=\"\", want true (spec: any value)")
	}
}

func TestStripANSI_NoEscapesInOutput(t *testing.T) {
	// Verify that output never contains the ESC character.
	inputs := []string{
		"\x1b[31mred\x1b[0m",
		"\x1b[1;31;42mcomplex\x1b[0m",
		"plain",
		"\x1b[?25h\x1b[?25l",
	}

	for _, input := range inputs {
		result := StripANSI(input)
		if strings.Contains(result, "\x1b") {
			t.Errorf("StripANSI(%q) still contains ESC: %q", input, result)
		}
	}
}
