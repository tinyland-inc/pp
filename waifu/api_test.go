package waifu

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testLogger returns a logger that only shows errors for clean test output.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testPNG generates a minimal valid 1x1 PNG image.
func testPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// testJPEG generates a minimal valid 1x1 JPEG image.
func testJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

// newTestClient creates an APIClient pointed at the given test server URL.
func newTestClient(serverURL string) *APIClient {
	return &APIClient{
		baseURL:    serverURL,
		httpClient: &http.Client{},
		logger:     testLogger(),
	}
}

func TestFetchImageURL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/sfw/neko") {
			t.Errorf("expected path ending in /sfw/neko, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"url": "https://i.waifu.pics/test-neko.png"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	url, err := client.FetchImageURL(context.Background(), "neko")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://i.waifu.pics/test-neko.png" {
		t.Errorf("expected URL=https://i.waifu.pics/test-neko.png, got %s", url)
	}
}

func TestFetchImageURL_InvalidCategory(t *testing.T) {
	client := newTestClient("http://unused")
	_, err := client.FetchImageURL(context.Background(), "invalid-category")
	if err == nil {
		t.Fatal("expected error for invalid category")
	}
	if !strings.Contains(err.Error(), "invalid waifu category") {
		t.Errorf("expected invalid category error, got: %v", err)
	}
}

func TestFetchImageURL_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message": "internal server error"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.FetchImageURL(context.Background(), "neko")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected StatusCode=500, got %d", apiErr.StatusCode)
	}
}

func TestFetchImageURL_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.FetchImageURL(context.Background(), "neko")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parsing response JSON") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}

func TestFetchImageURL_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"url": "https://i.waifu.pics/test.png"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	client := newTestClient(server.URL)
	_, err := client.FetchImageURL(ctx, "neko")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestFetchMultipleURLs_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/many/sfw/waifu") {
			t.Errorf("expected path ending in /many/sfw/waifu, got %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type=application/json, got %s", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"files": ["https://i.waifu.pics/a.png", "https://i.waifu.pics/b.png", "https://i.waifu.pics/c.png"]}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	urls, err := client.FetchMultipleURLs(context.Background(), "waifu", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 3 {
		t.Fatalf("expected 3 URLs, got %d", len(urls))
	}
	if urls[0] != "https://i.waifu.pics/a.png" {
		t.Errorf("expected first URL=https://i.waifu.pics/a.png, got %s", urls[0])
	}
}

func TestFetchMultipleURLs_LimitCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"files": ["https://i.waifu.pics/a.png", "https://i.waifu.pics/b.png", "https://i.waifu.pics/c.png", "https://i.waifu.pics/d.png", "https://i.waifu.pics/e.png"]}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	urls, err := client.FetchMultipleURLs(context.Background(), "waifu", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 2 {
		t.Errorf("expected 2 URLs (limited by count), got %d", len(urls))
	}
}

func TestDownloadImage_PNG(t *testing.T) {
	pngData := testPNG()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write(pngData)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	data, contentType, err := client.DownloadImage(context.Background(), server.URL+"/test.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/png" {
		t.Errorf("expected content-type=image/png, got %s", contentType)
	}
	if !bytes.Equal(data, pngData) {
		t.Errorf("downloaded data does not match original PNG (%d vs %d bytes)", len(data), len(pngData))
	}
}

func TestDownloadImage_JPEG(t *testing.T) {
	jpegData := testJPEG()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		w.Write(jpegData)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	data, contentType, err := client.DownloadImage(context.Background(), server.URL+"/test.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/jpeg" {
		t.Errorf("expected content-type=image/jpeg, got %s", contentType)
	}
	if !bytes.Equal(data, jpegData) {
		t.Errorf("downloaded data does not match original JPEG (%d vs %d bytes)", len(data), len(jpegData))
	}
}

func TestDownloadImage_TooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		// Write more than maxBodySize bytes.
		data := make([]byte, maxBodySize+100)
		w.Write(data)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, _, err := client.DownloadImage(context.Background(), server.URL+"/huge.png")
	if err == nil {
		t.Fatal("expected error for oversized image")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' error, got: %v", err)
	}
}

func TestDownloadImage_NotImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html>not an image</html>"))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, _, err := client.DownloadImage(context.Background(), server.URL+"/page.html")
	if err == nil {
		t.Fatal("expected error for non-image content type")
	}
	if !strings.Contains(err.Error(), "unexpected content type") {
		t.Errorf("expected 'unexpected content type' error, got: %v", err)
	}
}

func TestIsValidCategory(t *testing.T) {
	for _, cat := range ValidCategories {
		if !IsValidCategory(cat) {
			t.Errorf("expected %q to be valid", cat)
		}
	}
}

func TestIsValidCategory_Invalid(t *testing.T) {
	invalidCategories := []string{"", "invalid", "nsfw", "WAIFU", "Neko", "test123"}
	for _, cat := range invalidCategories {
		if IsValidCategory(cat) {
			t.Errorf("expected %q to be invalid", cat)
		}
	}
}

func TestNewAPIClient_NilLogger(t *testing.T) {
	// Must not panic with nil logger.
	client := NewAPIClient(nil)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.logger == nil {
		t.Fatal("expected non-nil logger (should use no-op)")
	}
	if client.baseURL == "" {
		t.Fatal("expected non-empty baseURL")
	}
	if client.httpClient == nil {
		t.Fatal("expected non-nil httpClient")
	}
}
