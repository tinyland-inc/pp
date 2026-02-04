package waifu

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// createTestImage generates a gradient test image of the given dimensions.
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	return img
}

// encodeTestPNG encodes an image to PNG bytes.
func encodeTestPNG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// encodeTestJPEG encodes an image to JPEG bytes.
func encodeTestJPEG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

func TestProcessImage_Resize(t *testing.T) {
	img := createTestImage(800, 600)
	data := encodeTestPNG(img)

	cfg := DefaultProcessConfig()
	cfg.MaxWidth = 400
	cfg.MaxHeight = 300

	out, err := ProcessImage(data, cfg)
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}

	decoded, _, err := DecodeImage(out)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	bounds := decoded.Bounds()
	if bounds.Dx() > cfg.MaxWidth || bounds.Dy() > cfg.MaxHeight {
		t.Errorf("output %dx%d exceeds max %dx%d", bounds.Dx(), bounds.Dy(), cfg.MaxWidth, cfg.MaxHeight)
	}
}

func TestProcessImage_NoResize(t *testing.T) {
	img := createTestImage(100, 80)
	data := encodeTestPNG(img)

	cfg := DefaultProcessConfig()
	cfg.MaxWidth = 400
	cfg.MaxHeight = 300
	cfg.Sharpen = false

	out, err := ProcessImage(data, cfg)
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}

	decoded, _, err := DecodeImage(out)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	bounds := decoded.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 80 {
		t.Errorf("expected 100x80, got %dx%d (image should not be upscaled)", bounds.Dx(), bounds.Dy())
	}
}

func TestProcessImage_AspectRatio(t *testing.T) {
	// 1000x500 image (2:1 aspect ratio) into 400x300 box.
	img := createTestImage(1000, 500)
	data := encodeTestPNG(img)

	cfg := DefaultProcessConfig()
	cfg.MaxWidth = 400
	cfg.MaxHeight = 300
	cfg.Sharpen = false

	out, err := ProcessImage(data, cfg)
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}

	decoded, _, err := DecodeImage(out)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}

	bounds := decoded.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// The aspect ratio should be approximately 2:1.
	ratio := float64(w) / float64(h)
	if ratio < 1.8 || ratio > 2.2 {
		t.Errorf("aspect ratio %dx%d = %.2f, expected ~2.0", w, h, ratio)
	}
}

func TestProcessImage_Sharpen(t *testing.T) {
	img := createTestImage(800, 600)
	data := encodeTestPNG(img)

	cfgSharp := DefaultProcessConfig()
	cfgSharp.Sharpen = true

	cfgNoSharp := DefaultProcessConfig()
	cfgNoSharp.Sharpen = false

	outSharp, err := ProcessImage(data, cfgSharp)
	if err != nil {
		t.Fatalf("ProcessImage (sharp): %v", err)
	}

	outNoSharp, err := ProcessImage(data, cfgNoSharp)
	if err != nil {
		t.Fatalf("ProcessImage (no sharp): %v", err)
	}

	if bytes.Equal(outSharp, outNoSharp) {
		t.Error("sharpened and non-sharpened output should differ")
	}
}

func TestProcessImage_PNG(t *testing.T) {
	img := createTestImage(200, 150)
	data := encodeTestPNG(img)

	cfg := DefaultProcessConfig()
	cfg.OutputFormat = "png"

	out, err := ProcessImage(data, cfg)
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}

	// PNG files start with the 8-byte PNG signature.
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(out, pngSig) {
		t.Error("output does not have PNG signature")
	}
}

func TestProcessImage_JPEG(t *testing.T) {
	img := createTestImage(200, 150)
	data := encodeTestPNG(img)

	cfg := DefaultProcessConfig()
	cfg.OutputFormat = "jpeg"

	out, err := ProcessImage(data, cfg)
	if err != nil {
		t.Fatalf("ProcessImage: %v", err)
	}

	// JPEG files start with FFD8.
	if len(out) < 2 || out[0] != 0xFF || out[1] != 0xD8 {
		t.Error("output does not have JPEG signature")
	}
}

func TestProcessImage_InvalidData(t *testing.T) {
	garbage := []byte("not an image at all")
	cfg := DefaultProcessConfig()

	_, err := ProcessImage(garbage, cfg)
	if err == nil {
		t.Error("expected error for invalid image data")
	}
}

func TestCalculateDimensions_FitWidth(t *testing.T) {
	// 1000x400 into 500x500 -> width is the limiting factor.
	w, h := CalculateDimensions(1000, 400, 500, 500)
	if w != 500 {
		t.Errorf("expected width 500, got %d", w)
	}
	if h != 200 {
		t.Errorf("expected height 200, got %d", h)
	}
}

func TestCalculateDimensions_FitHeight(t *testing.T) {
	// 400x1000 into 500x500 -> height is the limiting factor.
	w, h := CalculateDimensions(400, 1000, 500, 500)
	if w != 200 {
		t.Errorf("expected width 200, got %d", w)
	}
	if h != 500 {
		t.Errorf("expected height 500, got %d", h)
	}
}

func TestCalculateDimensions_AlreadyFits(t *testing.T) {
	w, h := CalculateDimensions(100, 80, 400, 300)
	if w != 100 || h != 80 {
		t.Errorf("expected 100x80, got %dx%d", w, h)
	}
}

func TestCalculateDimensions_Square(t *testing.T) {
	w, h := CalculateDimensions(1000, 1000, 400, 300)
	// Height is the limiting factor: 1000 -> 300, so width also -> 300.
	if w != 300 || h != 300 {
		t.Errorf("expected 300x300, got %dx%d", w, h)
	}
}

func TestCalculateDimensions_ZeroDimensions(t *testing.T) {
	// Zero max values should return original dimensions.
	w, h := CalculateDimensions(800, 600, 0, 0)
	if w != 800 || h != 600 {
		t.Errorf("expected 800x600, got %dx%d", w, h)
	}
}

func TestDecodeImage_PNG(t *testing.T) {
	img := createTestImage(50, 50)
	data := encodeTestPNG(img)

	decoded, format, err := DecodeImage(data)
	if err != nil {
		t.Fatalf("DecodeImage: %v", err)
	}
	if format != "png" {
		t.Errorf("expected format png, got %q", format)
	}
	if decoded.Bounds().Dx() != 50 || decoded.Bounds().Dy() != 50 {
		t.Errorf("expected 50x50, got %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}

func TestDecodeImage_JPEG(t *testing.T) {
	img := createTestImage(50, 50)
	data := encodeTestJPEG(img)

	decoded, format, err := DecodeImage(data)
	if err != nil {
		t.Fatalf("DecodeImage: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("expected format jpeg, got %q", format)
	}
	if decoded.Bounds().Dx() != 50 || decoded.Bounds().Dy() != 50 {
		t.Errorf("expected 50x50, got %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}

func TestDecodeImage_Invalid(t *testing.T) {
	_, _, err := DecodeImage([]byte("garbage"))
	if err == nil {
		t.Error("expected error for invalid data")
	}
}

func TestEncodeImage_PNG(t *testing.T) {
	img := createTestImage(30, 30)

	data, err := EncodeImage(img, "png", 0)
	if err != nil {
		t.Fatalf("EncodeImage: %v", err)
	}

	pngSig := []byte{0x89, 0x50, 0x4E, 0x47}
	if !bytes.HasPrefix(data, pngSig) {
		t.Error("output is not valid PNG")
	}
}

func TestEncodeImage_JPEG(t *testing.T) {
	img := createTestImage(30, 30)

	data, err := EncodeImage(img, "jpeg", 85)
	if err != nil {
		t.Fatalf("EncodeImage: %v", err)
	}

	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		t.Error("output is not valid JPEG")
	}
}

func TestTerminalPixelDimensions(t *testing.T) {
	tests := []struct {
		name                     string
		cols, rows, cw, ch       int
		expectWidth, expectHeight int
	}{
		{"standard 80x24", 80, 24, 8, 16, 640, 384},
		{"wide terminal", 120, 40, 8, 16, 960, 640},
		{"custom cell size", 80, 24, 10, 20, 800, 480},
		{"zero cell width defaults", 80, 24, 0, 0, 640, 384},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h := TerminalPixelDimensions(tt.cols, tt.rows, tt.cw, tt.ch)
			if w != tt.expectWidth || h != tt.expectHeight {
				t.Errorf("got %dx%d, want %dx%d", w, h, tt.expectWidth, tt.expectHeight)
			}
		})
	}
}
