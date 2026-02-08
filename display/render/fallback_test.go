package render

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

// makePNG creates a minimal PNG image of the given dimensions filled with the
// specified color.
func makePNG(w, h int, c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// makeLargePNG creates a PNG large enough to force multi-chunk Kitty transmission.
func makeLargePNG() []byte {
	w, h := 200, 200
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := (x*7 + y*13 + x*y) % 256
			img.Set(x, y, color.RGBA{R: uint8(v), G: uint8(255 - v), B: uint8((v * 3) % 256), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// withEnv sets an environment variable for the duration of a test.
func withEnv(t *testing.T, key, value string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	os.Setenv(key, value)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

// clearEnv unsets an environment variable for the duration of a test.
func clearEnv(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, old)
		}
	})
}

func TestFallbackRenderer_RenderImage_EmptyData(t *testing.T) {
	r := NewFallbackRenderer(DefaultRenderConfig())
	_, err := r.RenderImage([]byte{})
	if err == nil {
		t.Error("expected error for empty data")
	}
	if err != ErrCorruptImage {
		t.Errorf("expected ErrCorruptImage, got %v", err)
	}
}

func TestFallbackRenderer_RenderImage_CorruptData(t *testing.T) {
	r := NewFallbackRenderer(DefaultRenderConfig())
	_, err := r.RenderImage([]byte("not an image"))
	if err == nil {
		t.Error("expected error for corrupt data")
	}
	if err != ErrCorruptImage {
		t.Errorf("expected ErrCorruptImage, got %v", err)
	}
}

func TestFallbackRenderer_RenderImage_ValidPNG(t *testing.T) {
	// Clear environment to force unicode fallback
	clearEnv(t, "TERM_PROGRAM")
	clearEnv(t, "TERM")
	clearEnv(t, "KITTY_WINDOW_ID")

	cfg := DefaultRenderConfig()
	cfg.FallbackEnabled = true
	r := NewFallbackRenderer(cfg)

	data := makePNG(10, 10, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	result, err := r.RenderImage(data)
	if err != nil {
		t.Fatalf("RenderImage failed: %v", err)
	}

	if result.Output == "" {
		t.Error("expected non-empty output")
	}
}

func TestFallbackRenderer_RenderKitty_SmallImage(t *testing.T) {
	r := NewFallbackRenderer(RenderConfig{MaxCols: 10, MaxRows: 5})
	data := makePNG(1, 1, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	result, err := r.renderKitty(data)
	if err != nil {
		t.Fatalf("renderKitty failed: %v", err)
	}

	// Small image should fit in a single chunk (m=0, no m=1).
	if strings.Contains(result, "m=1") {
		t.Error("small image should not produce multi-chunk output (found m=1)")
	}
	if !strings.Contains(result, "m=0") {
		t.Error("single-chunk output should contain m=0")
	}
}

func TestFallbackRenderer_RenderKitty_LargeImage(t *testing.T) {
	r := NewFallbackRenderer(RenderConfig{MaxCols: 40, MaxRows: 20})
	data := makeLargePNG()

	result, err := r.renderKitty(data)
	if err != nil {
		t.Fatalf("renderKitty failed: %v", err)
	}

	// Large image should produce multiple chunks.
	if !strings.Contains(result, "m=1") {
		t.Error("large image should produce multi-chunk output (missing m=1)")
	}
	if !strings.Contains(result, "m=0") {
		t.Error("multi-chunk output should end with m=0")
	}

	// The m=0 chunk should be the last escape sequence.
	lastM0 := strings.LastIndex(result, "m=0")
	lastM1 := strings.LastIndex(result, "m=1")
	if lastM1 > lastM0 {
		t.Error("m=1 should not appear after the final m=0 chunk")
	}
}

func TestFallbackRenderer_RenderKitty_EscapeSequences(t *testing.T) {
	r := NewFallbackRenderer(RenderConfig{MaxCols: 20, MaxRows: 10})
	data := makePNG(2, 2, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	result, err := r.renderKitty(data)
	if err != nil {
		t.Fatalf("renderKitty failed: %v", err)
	}

	// Verify escape sequence framing.
	if !strings.Contains(result, "\033_G") {
		t.Error("missing Kitty escape sequence start (\\033_G)")
	}
	if !strings.Contains(result, "\033\\") {
		t.Error("missing Kitty escape sequence terminator (\\033\\\\)")
	}
	if !strings.Contains(result, "f=100") {
		t.Error("missing format parameter f=100 (PNG)")
	}
	if !strings.Contains(result, "a=T") {
		t.Error("missing action parameter a=T")
	}
}

func TestFallbackRenderer_RenderITerm2(t *testing.T) {
	r := NewFallbackRenderer(RenderConfig{MaxCols: 20, MaxRows: 10})
	data := makePNG(2, 2, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	result, err := r.renderITerm2(data)
	if err != nil {
		t.Fatalf("renderITerm2 failed: %v", err)
	}

	// Verify iTerm2 escape sequence format
	if !strings.Contains(result, "\033]1337;File=") {
		t.Error("missing iTerm2 escape sequence start")
	}
	if !strings.Contains(result, "inline=1") {
		t.Error("missing inline=1 parameter")
	}
	if !strings.HasSuffix(result, "\007") {
		t.Error("missing BEL terminator")
	}
}

func TestFallbackRenderer_RenderUnicode_BasicImage(t *testing.T) {
	r := NewFallbackRenderer(RenderConfig{MaxCols: 10, MaxRows: 5})
	data := makePNG(2, 2, color.RGBA{R: 255, G: 128, B: 0, A: 255})

	result, err := r.renderUnicode(data)
	if err != nil {
		t.Fatalf("renderUnicode failed: %v", err)
	}

	// Should contain ANSI color codes.
	if !strings.Contains(result, "\033[38;2;") {
		t.Error("output should contain ANSI 24-bit foreground color sequences")
	}
	if !strings.Contains(result, "\033[48;2;") {
		t.Error("output should contain ANSI 24-bit background color sequences")
	}
	// Should contain reset.
	if !strings.Contains(result, "\033[0m") {
		t.Error("output should contain ANSI reset sequence")
	}
}

func TestFallbackRenderer_RenderUnicode_ContainsHalfBlock(t *testing.T) {
	r := NewFallbackRenderer(RenderConfig{MaxCols: 10, MaxRows: 5})
	data := makePNG(2, 2, color.RGBA{R: 100, G: 100, B: 100, A: 255})

	result, err := r.renderUnicode(data)
	if err != nil {
		t.Fatalf("renderUnicode failed: %v", err)
	}

	if !strings.Contains(result, "\u2580") {
		t.Error("output should contain upper half-block character U+2580")
	}
}

func TestFallbackRenderer_FallbackChain(t *testing.T) {
	// Clear environment to force fallback chain testing
	clearEnv(t, "TERM_PROGRAM")
	clearEnv(t, "TERM")
	clearEnv(t, "KITTY_WINDOW_ID")
	clearEnv(t, "ITERM_SESSION_ID")
	clearEnv(t, "MLTERM")

	cfg := DefaultRenderConfig()
	cfg.FallbackEnabled = true
	r := NewFallbackRenderer(cfg)

	data := makePNG(4, 4, color.RGBA{R: 50, G: 100, B: 150, A: 255})
	result, err := r.RenderImage(data)
	if err != nil {
		t.Fatalf("RenderImage with fallback failed: %v", err)
	}

	// Should have fallen back to either chafa or unicode
	if result.Protocol != ProtocolChafa && result.Protocol != ProtocolUnicode {
		t.Errorf("expected fallback to chafa or unicode, got %s", result.Protocol.String())
	}

	if result.Output == "" {
		t.Error("expected non-empty output from fallback")
	}
}

func TestFallbackRenderer_FallbackDisabled(t *testing.T) {
	cfg := DefaultRenderConfig()
	cfg.FallbackEnabled = false
	cfg.Protocol = ProtocolNone // Will auto-detect
	r := NewFallbackRenderer(cfg)

	// With fallback disabled, if the detected protocol fails, we should get an error
	// This is hard to test without mocking, but we can at least verify the path exists
	data := makePNG(2, 2, color.White)
	result, err := r.RenderImage(data)

	// Should succeed with detected protocol (or its own fallback logic)
	if err != nil {
		// This is acceptable if no protocol works
		t.Logf("Expected: fallback disabled error path: %v", err)
	} else {
		if result.Output == "" {
			t.Error("expected non-empty output when render succeeds")
		}
	}
}

func TestProcessImageForRender(t *testing.T) {
	data := makePNG(100, 100, color.RGBA{R: 128, G: 64, B: 192, A: 255})

	processed, err := ProcessImageForRender(data, 50, 50)
	if err != nil {
		t.Fatalf("ProcessImageForRender failed: %v", err)
	}

	if len(processed) == 0 {
		t.Error("expected non-empty processed image")
	}

	// Verify it's still a valid PNG
	img, _, err := decodeImage(processed)
	if err != nil {
		t.Fatalf("processed image not decodable: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() > 50 || bounds.Dy() > 50 {
		t.Errorf("processed image too large: %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestProcessImageForRender_CorruptData(t *testing.T) {
	_, err := ProcessImageForRender([]byte("not an image"), 50, 50)
	if err == nil {
		t.Error("expected error for corrupt data")
	}
}

func TestCalculateDimensions(t *testing.T) {
	tests := []struct {
		origW, origH int
		maxW, maxH   int
		wantW, wantH int
	}{
		// Already fits
		{50, 50, 100, 100, 50, 50},
		// Needs scaling down by width
		{200, 100, 100, 100, 100, 50},
		// Needs scaling down by height
		{100, 200, 100, 100, 50, 100},
		// Needs scaling down by both (height is limiting factor)
		{200, 200, 100, 50, 50, 50},
		// Zero dimensions
		{0, 100, 100, 100, 0, 100},
		{100, 0, 100, 100, 100, 0},
	}

	for _, tt := range tests {
		w, h := calculateDimensions(tt.origW, tt.origH, tt.maxW, tt.maxH)
		if w != tt.wantW || h != tt.wantH {
			t.Errorf("calculateDimensions(%d, %d, %d, %d) = (%d, %d), want (%d, %d)",
				tt.origW, tt.origH, tt.maxW, tt.maxH, w, h, tt.wantW, tt.wantH)
		}
	}
}

func TestIsValidImage(t *testing.T) {
	valid := makePNG(1, 1, color.White)
	if !isValidImage(valid) {
		t.Error("expected valid PNG to be valid")
	}

	invalid := []byte("not an image")
	if isValidImage(invalid) {
		t.Error("expected invalid data to be invalid")
	}
}

func TestRenderNone(t *testing.T) {
	result := RenderNone()
	if result == "" {
		t.Error("RenderNone should return a non-empty placeholder")
	}
}

func TestDefaultRenderConfig(t *testing.T) {
	cfg := DefaultRenderConfig()

	if cfg.MaxCols != 40 {
		t.Errorf("MaxCols = %d, want 40", cfg.MaxCols)
	}
	if cfg.MaxRows != 20 {
		t.Errorf("MaxRows = %d, want 20", cfg.MaxRows)
	}
	if !cfg.UseContextDetection {
		t.Error("UseContextDetection should default to true")
	}
	if !cfg.FallbackEnabled {
		t.Error("FallbackEnabled should default to true")
	}
}
