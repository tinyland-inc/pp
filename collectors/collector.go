// Package collectors provides the data collection interface and registration
// for prompt-pulse metrics gathering. Each collector implements a plugin
// pattern for fetching structured data from external services.
package collectors

import (
	"context"
	"time"
)

// Collector is the interface that all data collectors must implement.
// Collectors are responsible for gathering metrics from a single source
// (e.g., Claude API, Civo billing, Tailscale status) and returning
// structured, JSON-serializable results.
type Collector interface {
	// Name returns the collector's unique identifier (e.g., "claude", "civo", "tailscale").
	// Names must be unique within a Registry.
	Name() string

	// Description returns a human-readable description of what this collector gathers.
	Description() string

	// Interval returns the recommended polling interval for this collector.
	// The daemon uses this to schedule collection runs. Collectors that hit
	// rate-limited APIs should return longer intervals.
	Interval() time.Duration

	// Collect gathers metrics and returns structured data.
	// The Data field in CollectResult must be JSON-serializable for caching.
	// Non-fatal issues should be reported as Warnings rather than errors.
	// The context should be respected for cancellation of long-running operations.
	Collect(ctx context.Context) (*CollectResult, error)
}

// CollectResult holds the output of a collection run.
type CollectResult struct {
	// Collector is the name of the collector that produced this result.
	Collector string `json:"collector"`

	// Timestamp records when the collection completed.
	Timestamp time.Time `json:"timestamp"`

	// Data is the collector-specific structured data.
	// Must be JSON-serializable for caching to disk.
	Data interface{} `json:"data"`

	// Warnings contains non-fatal issues encountered during collection.
	// For example, one account failing auth while others succeed.
	Warnings []string `json:"warnings,omitempty"`
}

// Registry holds registered collectors and provides lookup by name.
type Registry struct {
	collectors []Collector
}

// NewRegistry creates a new empty collector registry.
func NewRegistry() *Registry {
	return &Registry{
		collectors: make([]Collector, 0),
	}
}

// Register adds a collector to the registry.
// If a collector with the same name already exists, it is replaced.
func (r *Registry) Register(c Collector) {
	// Replace existing collector with same name
	for i, existing := range r.collectors {
		if existing.Name() == c.Name() {
			r.collectors[i] = c
			return
		}
	}
	r.collectors = append(r.collectors, c)
}

// Get returns a collector by name. The second return value indicates
// whether the collector was found.
func (r *Registry) Get(name string) (Collector, bool) {
	for _, c := range r.collectors {
		if c.Name() == name {
			return c, true
		}
	}
	return nil, false
}

// All returns all registered collectors.
func (r *Registry) All() []Collector {
	result := make([]Collector, len(r.collectors))
	copy(result, r.collectors)
	return result
}
