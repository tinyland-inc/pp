package cache

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	s, err := NewStore(dir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestSetGetRoundTrip(t *testing.T) {
	s := newTestStore(t)

	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	original := payload{Name: "test", Count: 42}

	if err := s.Set("mykey", original); err != nil {
		t.Fatalf("Set: %v", err)
	}

	raw, fresh, err := s.Get("mykey", 1*time.Hour)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !fresh {
		t.Error("expected fresh=true for recently written entry")
	}
	if raw == nil {
		t.Fatal("expected non-nil data")
	}

	var got payload
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != original {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, original)
	}
}

func TestTypedRoundTrip(t *testing.T) {
	s := newTestStore(t)

	type entry struct {
		Status string  `json:"status"`
		Score  float64 `json:"score"`
	}

	original := &entry{Status: "ok", Score: 99.5}

	if err := SetTyped(s, "typed", original); err != nil {
		t.Fatalf("SetTyped: %v", err)
	}

	got, fresh, err := GetTyped[entry](s, "typed", 1*time.Hour)
	if err != nil {
		t.Fatalf("GetTyped: %v", err)
	}
	if !fresh {
		t.Error("expected fresh=true")
	}
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if *got != *original {
		t.Errorf("typed round-trip mismatch: got %+v, want %+v", *got, *original)
	}
}

func TestTTLExpiry(t *testing.T) {
	s := newTestStore(t)

	if err := s.Set("expiring", map[string]string{"v": "data"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Backdate the file modification time to simulate age.
	path := filepath.Join(s.dir, "expiring.json")
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	raw, fresh, err := s.Get("expiring", 1*time.Hour)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fresh {
		t.Error("expected fresh=false for stale entry")
	}
	if raw == nil {
		t.Error("expected stale data to still be returned")
	}
}

func TestMissingKeyReturnsNil(t *testing.T) {
	s := newTestStore(t)

	raw, fresh, err := s.Get("nonexistent", 1*time.Hour)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fresh {
		t.Error("expected fresh=false for missing key")
	}
	if raw != nil {
		t.Errorf("expected nil data for missing key, got %s", string(raw))
	}
}

func TestCorruptedFileHandling(t *testing.T) {
	s := newTestStore(t)

	// Write invalid JSON directly to the cache file.
	path := filepath.Join(s.dir, "broken.json")
	if err := os.WriteFile(path, []byte("{invalid json!!!"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	raw, fresh, err := s.Get("broken", 1*time.Hour)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fresh {
		t.Error("expected fresh=false for corrupted entry")
	}
	if raw != nil {
		t.Error("expected nil data for corrupted entry")
	}

	// Verify the corrupted file was removed.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected corrupted file to be removed")
	}
}

func TestCorruptedFileTypedHandling(t *testing.T) {
	s := newTestStore(t)

	// Write JSON that is valid but does not match the target type.
	// json.Unmarshal into a struct is lenient, so write truly invalid JSON instead.
	path := filepath.Join(s.dir, "badtype.json")
	if err := os.WriteFile(path, []byte(`not json`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	type target struct {
		Field string `json:"field"`
	}

	got, fresh, err := GetTyped[target](s, "badtype", 1*time.Hour)
	if err != nil {
		t.Fatalf("GetTyped: %v", err)
	}
	if fresh {
		t.Error("expected fresh=false")
	}
	if got != nil {
		t.Error("expected nil result for corrupted typed entry")
	}
}

func TestAtomicWriteConcurrency(t *testing.T) {
	s := newTestStore(t)

	const goroutines = 20
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				data := map[string]int{"writer": id, "iteration": i}
				if err := s.Set("concurrent", data); err != nil {
					t.Errorf("goroutine %d iteration %d: Set: %v", id, i, err)
					return
				}
			}
		}(g)
	}

	wg.Wait()

	// After all writes complete, the file must contain valid JSON.
	raw, fresh, err := s.Get("concurrent", 1*time.Hour)
	if err != nil {
		t.Fatalf("Get after concurrent writes: %v", err)
	}
	if !fresh {
		t.Error("expected fresh=true")
	}
	if raw == nil {
		t.Fatal("expected non-nil data after concurrent writes")
	}

	var result map[string]int
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("final value is not valid JSON: %v", err)
	}
}

func TestAge(t *testing.T) {
	s := newTestStore(t)

	// Missing key returns 0.
	if age := s.Age("missing"); age != 0 {
		t.Errorf("expected age=0 for missing key, got %v", age)
	}

	if err := s.Set("aged", "value"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	age := s.Age("aged")
	if age < 0 || age > 2*time.Second {
		t.Errorf("unexpected age for freshly written entry: %v", age)
	}

	// Backdate and recheck.
	path := filepath.Join(s.dir, "aged.json")
	past := time.Now().Add(-30 * time.Minute)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	age = s.Age("aged")
	if age < 29*time.Minute || age > 31*time.Minute {
		t.Errorf("expected age ~30m, got %v", age)
	}
}

func TestKeys(t *testing.T) {
	s := newTestStore(t)

	if keys := s.Keys(); len(keys) != 0 {
		t.Errorf("expected empty keys, got %v", keys)
	}

	for _, k := range []string{"alpha", "beta", "gamma"} {
		if err := s.Set(k, k); err != nil {
			t.Fatalf("Set %s: %v", k, err)
		}
	}

	keys := s.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(keys), keys)
	}

	want := map[string]bool{"alpha": true, "beta": true, "gamma": true}
	for _, k := range keys {
		if !want[k] {
			t.Errorf("unexpected key: %s", k)
		}
	}
}

func TestClear(t *testing.T) {
	s := newTestStore(t)

	for _, k := range []string{"a", "b", "c"} {
		if err := s.Set(k, k); err != nil {
			t.Fatalf("Set %s: %v", k, err)
		}
	}

	if err := s.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if keys := s.Keys(); len(keys) != 0 {
		t.Errorf("expected no keys after clear, got %v", keys)
	}
}

func TestMeta(t *testing.T) {
	s := newTestStore(t)

	for _, k := range []string{"one", "two"} {
		if err := s.Set(k, map[string]string{"key": k}); err != nil {
			t.Fatalf("Set %s: %v", k, err)
		}
	}

	m, err := s.Meta()
	if err != nil {
		t.Fatalf("Meta: %v", err)
	}

	if len(m.LastUpdate) != 2 {
		t.Errorf("expected 2 entries in LastUpdate, got %d", len(m.LastUpdate))
	}
	if len(m.Sizes) != 2 {
		t.Errorf("expected 2 entries in Sizes, got %d", len(m.Sizes))
	}

	for _, k := range []string{"one", "two"} {
		if _, ok := m.LastUpdate[k]; !ok {
			t.Errorf("missing LastUpdate for key %s", k)
		}
		if size, ok := m.Sizes[k]; !ok || size == 0 {
			t.Errorf("missing or zero Size for key %s", k)
		}
	}
}

func TestFilePermissions(t *testing.T) {
	s := newTestStore(t)

	if err := s.Set("perms", "secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	path := filepath.Join(s.dir, "perms.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected file permissions 0600, got %04o", perm)
	}
}

func TestDirectoryPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir")
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	_, err := NewStore(dir, logger)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0700 {
		t.Errorf("expected directory permissions 0700, got %04o", perm)
	}
}
