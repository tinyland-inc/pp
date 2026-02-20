package waifu

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockClient implements Client for testing.
type mockClient struct {
	meta *ImageMeta
	data []byte
	err  error
}

func (m *mockClient) RandomImage(_ context.Context, _ string) (*ImageMeta, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.meta, nil
}

func (m *mockClient) DownloadImage(_ context.Context, _ string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.data, nil
}

func TestCollect_CachesImage(t *testing.T) {
	dir := t.TempDir()
	imageData := []byte("fake-png-data-12345")

	client := &mockClient{
		meta: &ImageMeta{
			URL:  "http://example.com/test.webp",
			ID:   "test.webp",
			Hash: "abc123",
		},
		data: imageData,
	}

	c := New(Config{
		Interval:  time.Second,
		Endpoint:  "http://example.com",
		Category:  "sfw",
		CacheDir:  dir,
		MaxImages: 5,
	}, client)

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	m, ok := result.(*Manifest)
	if !ok {
		t.Fatalf("Collect() returned %T, want *Manifest", result)
	}

	if len(m.Images) != 1 {
		t.Fatalf("got %d images, want 1", len(m.Images))
	}
	if m.Current == "" {
		t.Fatal("Current hash is empty")
	}

	// Verify file exists on disk.
	if _, err := os.Stat(m.Images[0].Path); err != nil {
		t.Fatalf("cached image file missing: %v", err)
	}

	// Verify manifest file was written.
	manifestPath := filepath.Join(dir, ManifestFile)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest file missing: %v", err)
	}
}

func TestCollect_DeduplicatesImages(t *testing.T) {
	dir := t.TempDir()
	imageData := []byte("same-image-data")

	client := &mockClient{
		meta: &ImageMeta{URL: "http://example.com/a.webp", ID: "a.webp"},
		data: imageData,
	}

	c := New(Config{
		Interval:  time.Second,
		CacheDir:  dir,
		MaxImages: 5,
	}, client)

	// Collect twice with the same data.
	if _, err := c.Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	m := result.(*Manifest)
	if len(m.Images) != 1 {
		t.Fatalf("got %d images after dedup, want 1", len(m.Images))
	}
}

func TestCollect_PrunesOldImages(t *testing.T) {
	dir := t.TempDir()

	callCount := 0
	client := &mockClient{
		meta: &ImageMeta{URL: "http://example.com/img.webp", ID: "img.webp"},
	}

	c := New(Config{
		Interval:  time.Second,
		CacheDir:  dir,
		MaxImages: 2,
	}, client)

	// Collect 3 unique images.
	for i := 0; i < 3; i++ {
		client.data = []byte{byte(i), byte(i + 1), byte(i + 2), byte(callCount)}
		callCount++
		if _, err := c.Collect(context.Background()); err != nil {
			t.Fatalf("Collect #%d: %v", i, err)
		}
	}

	m := c.loadManifest(dir)
	if len(m.Images) != 2 {
		t.Fatalf("got %d images after pruning, want 2", len(m.Images))
	}
}

func TestCollect_NoEndpoint(t *testing.T) {
	c := New(Config{CacheDir: t.TempDir()}, nil)

	_, err := c.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error with nil client")
	}
}
