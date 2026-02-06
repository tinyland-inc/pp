package widgets

import (
	"strings"
	"testing"
)

func TestRenderHyperlink(t *testing.T) {
	result := RenderHyperlink("https://example.com", "click here")

	// Should contain the OSC 8 open sequence.
	if !strings.Contains(result, "\x1b]8;;https://example.com\x1b\\") {
		t.Error("missing OSC 8 open sequence")
	}

	// Should contain the visible text.
	if !strings.Contains(result, "click here") {
		t.Error("missing visible text")
	}

	// Should contain the OSC 8 close sequence.
	if !strings.HasSuffix(result, "\x1b]8;;\x1b\\") {
		t.Error("missing OSC 8 close sequence")
	}
}

func TestRenderHyperlink_EmptyURL(t *testing.T) {
	result := RenderHyperlink("", "plain text")
	if result != "plain text" {
		t.Errorf("empty URL should return plain text, got %q", result)
	}
}

func TestRenderHyperlink_Structure(t *testing.T) {
	result := RenderHyperlink("https://dashboard.example.com", "Dashboard")
	expected := "\x1b]8;;https://dashboard.example.com\x1b\\Dashboard\x1b]8;;\x1b\\"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}
