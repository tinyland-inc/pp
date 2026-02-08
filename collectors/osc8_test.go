package collectors

import (
	"strings"
	"testing"
)

func TestLink_BasicURL(t *testing.T) {
	got := Link("https://example.com", "Example")
	want := "\033]8;;https://example.com\033\\Example\033]8;;\033\\"
	if got != want {
		t.Errorf("Link() = %q, want %q", got, want)
	}
}

func TestLink_EmptyURL(t *testing.T) {
	got := Link("", "No Link")
	want := "\033]8;;\033\\No Link\033]8;;\033\\"
	if got != want {
		t.Errorf("Link() = %q, want %q", got, want)
	}
}

func TestLink_EmptyText(t *testing.T) {
	got := Link("https://example.com", "")
	want := "\033]8;;https://example.com\033\\\033]8;;\033\\"
	if got != want {
		t.Errorf("Link() = %q, want %q", got, want)
	}
}

func TestLink_SpecialCharacters(t *testing.T) {
	url := "https://example.com/search?q=hello+world&lang=en"
	text := "Search Results (Hello World)"
	got := Link(url, text)

	// Verify the URL is embedded verbatim.
	if !strings.Contains(got, url) {
		t.Errorf("Link() does not contain URL %q", url)
	}

	// Verify the text is embedded verbatim.
	if !strings.Contains(got, text) {
		t.Errorf("Link() does not contain text %q", text)
	}
}

func TestLink_EscapeSequences(t *testing.T) {
	got := Link("https://test.dev", "Test")

	// OSC 8 format: ESC ] 8 ; ; URL ST text ESC ] 8 ; ; ST
	// Where ESC = \033 and ST = \033\\

	// Verify opening escape: \033]8;;<url>\033\\
	if !strings.HasPrefix(got, "\033]8;;") {
		t.Error("Link() missing opening OSC 8 escape sequence")
	}

	// Verify the URL is between the opening and the first ST.
	parts := strings.Split(got, "\033\\")
	// Expected parts after splitting on ST:
	// parts[0] = "\033]8;;https://test.dev"  (opening + URL)
	// parts[1] = "Test\033]8;;"              (text + closing OSC 8)
	// parts[2] = ""                           (after final ST)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts after splitting on ST, got %d: %v", len(parts), parts)
	}

	if parts[0] != "\033]8;;https://test.dev" {
		t.Errorf("opening sequence = %q, want %q", parts[0], "\033]8;;https://test.dev")
	}
	if parts[1] != "Test\033]8;;" {
		t.Errorf("text + closing sequence = %q, want %q", parts[1], "Test\033]8;;")
	}
}

func TestLink_LongURL(t *testing.T) {
	url := "https://dashboard.civo.com/kubernetes/clusters/bitter-darkness-16657317/nodes?region=nyc1&tab=overview"
	text := "Civo K8s"
	got := Link(url, text)

	if !strings.Contains(got, url) {
		t.Errorf("Link() does not contain long URL")
	}
	if !strings.Contains(got, text) {
		t.Errorf("Link() does not contain text")
	}
}
