package collectors

import (
	"context"
	"testing"
	"time"
)

// stubCollector is a minimal Collector implementation for registry tests.
type stubCollector struct {
	name string
}

func (s *stubCollector) Name() string                                       { return s.name }
func (s *stubCollector) Description() string                                { return "stub " + s.name }
func (s *stubCollector) Interval() time.Duration                            { return time.Minute }
func (s *stubCollector) Collect(_ context.Context) (*CollectResult, error)  { return nil, nil }

// TestRegistry_RegisterAll verifies that multiple collectors can be registered
// and retrieved by name, and that All returns all of them.
func TestRegistry_RegisterAll(t *testing.T) {
	reg := NewRegistry()

	claude := &stubCollector{name: "claude"}
	billing := &stubCollector{name: "billing"}
	infra := &stubCollector{name: "infra"}

	reg.Register(claude)
	reg.Register(billing)
	reg.Register(infra)

	// Verify Get returns each collector.
	for _, want := range []string{"claude", "billing", "infra"} {
		got, ok := reg.Get(want)
		if !ok {
			t.Errorf("Get(%q) returned false, want true", want)
			continue
		}
		if got.Name() != want {
			t.Errorf("Get(%q).Name() = %q, want %q", want, got.Name(), want)
		}
	}

	// Verify All returns exactly 3 collectors.
	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d collectors, want 3", len(all))
	}

	// Verify All returns a copy (modifying the slice does not affect the registry).
	all[0] = &stubCollector{name: "mutated"}
	original, ok := reg.Get("claude")
	if !ok {
		t.Fatal("Get(claude) returned false after mutating All() slice")
	}
	if original.Name() != "claude" {
		t.Errorf("registry was mutated via All() slice: got %q, want %q", original.Name(), "claude")
	}
}

// TestRegistry_DuplicateRegistration verifies that registering a collector with
// the same name as an existing one replaces the existing collector.
func TestRegistry_DuplicateRegistration(t *testing.T) {
	reg := NewRegistry()

	first := &stubCollector{name: "claude"}
	second := &stubCollector{name: "claude"}

	reg.Register(first)
	reg.Register(second)

	// Should still have only one collector.
	all := reg.All()
	if len(all) != 1 {
		t.Fatalf("All() returned %d collectors after duplicate registration, want 1", len(all))
	}

	// The retrieved collector should be the second one (replacement).
	got, ok := reg.Get("claude")
	if !ok {
		t.Fatal("Get(claude) returned false after duplicate registration")
	}
	if got != second {
		t.Error("Get(claude) did not return the replacement collector")
	}
}

// TestRegistry_GetMissing verifies that Get returns false for a non-existent collector.
func TestRegistry_GetMissing(t *testing.T) {
	reg := NewRegistry()

	got, ok := reg.Get("nonexistent")
	if ok {
		t.Errorf("Get(nonexistent) returned true, want false")
	}
	if got != nil {
		t.Errorf("Get(nonexistent) returned non-nil collector: %v", got)
	}
}

// TestRegistry_Empty verifies that a new registry starts empty.
func TestRegistry_Empty(t *testing.T) {
	reg := NewRegistry()

	all := reg.All()
	if len(all) != 0 {
		t.Errorf("All() on empty registry returned %d collectors, want 0", len(all))
	}
}

// TestRegistry_AllPreservesOrder verifies that All returns collectors in registration order.
func TestRegistry_AllPreservesOrder(t *testing.T) {
	reg := NewRegistry()

	names := []string{"zebra", "alpha", "middle"}
	for _, name := range names {
		reg.Register(&stubCollector{name: name})
	}

	all := reg.All()
	if len(all) != len(names) {
		t.Fatalf("All() returned %d collectors, want %d", len(all), len(names))
	}

	for i, want := range names {
		if all[i].Name() != want {
			t.Errorf("All()[%d].Name() = %q, want %q", i, all[i].Name(), want)
		}
	}
}
