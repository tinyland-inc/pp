// Package retry provides a circuit breaker that wraps collectors to handle
// persistent failures gracefully. When a collector fails repeatedly, the
// circuit breaker "opens" to skip it for increasing intervals, reducing
// wasted API calls and log noise.
package retry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// Compile-time check: CircuitBreaker satisfies the Collector interface.
var _ collectors.Collector = (*CircuitBreaker)(nil)

// State represents the circuit breaker state.
type State int

const (
	// StateClosed is normal operation; requests pass through to the collector.
	StateClosed State = iota
	// StateOpen means failures exceeded the threshold; requests are blocked.
	StateOpen
	// StateHalfOpen is a probe state testing whether the collector has recovered.
	StateHalfOpen
)

// String returns the human-readable state name.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Config configures the circuit breaker behavior.
type Config struct {
	// MaxFailures is the number of consecutive failures before opening the circuit.
	MaxFailures int
	// ResetTimeout is the initial wait duration before transitioning from Open to HalfOpen.
	ResetTimeout time.Duration
	// MaxResetTimeout caps the exponential backoff.
	MaxResetTimeout time.Duration
	// BackoffMultiplier is the factor by which ResetTimeout increases on each re-open.
	BackoffMultiplier float64
	// Logger for circuit breaker events. Nil is safe (a discard logger is used).
	Logger *slog.Logger
}

// DefaultConfig returns sensible defaults for production use.
func DefaultConfig() Config {
	return Config{
		MaxFailures:       3,
		ResetTimeout:      1 * time.Minute,
		MaxResetTimeout:   30 * time.Minute,
		BackoffMultiplier: 2.0,
	}
}

// Stats holds circuit breaker statistics for external inspection.
type Stats struct {
	State            State
	ConsecutiveFails int
	TotalFailures    int
	TotalSuccesses   int
	LastFailure      time.Time
	LastSuccess      time.Time
	CurrentTimeout   time.Duration
	ConsecutiveSkips int
}

// CircuitBreaker wraps a collectors.Collector with failure tracking and
// automatic circuit opening/closing.
type CircuitBreaker struct {
	collector collectors.Collector
	config    Config
	logger    *slog.Logger

	mu               sync.Mutex
	state            State
	failures         int
	lastFailure      time.Time
	lastSuccess      time.Time
	currentTimeout   time.Duration
	totalFailures    int
	totalSuccesses   int
	consecutiveSkips int
}

// NewCircuitBreaker wraps a collector with circuit breaker logic.
// If cfg.Logger is nil, a discard logger is used.
func NewCircuitBreaker(c collectors.Collector, cfg Config) *CircuitBreaker {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &CircuitBreaker{
		collector:      c,
		config:         cfg,
		logger:         logger,
		state:          StateClosed,
		currentTimeout: cfg.ResetTimeout,
	}
}

// Name delegates to the wrapped collector.
func (cb *CircuitBreaker) Name() string {
	return cb.collector.Name()
}

// Description delegates to the wrapped collector, appending the circuit state.
func (cb *CircuitBreaker) Description() string {
	cb.mu.Lock()
	state := cb.state
	cb.mu.Unlock()
	return fmt.Sprintf("%s [circuit: %s]", cb.collector.Description(), state)
}

// Interval delegates to the wrapped collector.
func (cb *CircuitBreaker) Interval() time.Duration {
	return cb.collector.Interval()
}

// Collect checks the circuit state and either executes the wrapped collector
// or returns a synthetic result when the circuit is open.
func (cb *CircuitBreaker) Collect(ctx context.Context) (*collectors.CollectResult, error) {
	cb.mu.Lock()

	switch cb.state {
	case StateClosed:
		cb.mu.Unlock()
		return cb.collectClosed(ctx)

	case StateOpen:
		elapsed := time.Since(cb.lastFailure)
		if elapsed < cb.currentTimeout {
			remaining := cb.currentTimeout - elapsed
			cb.consecutiveSkips++
			skips := cb.consecutiveSkips
			failures := cb.failures
			name := cb.collector.Name()
			cb.mu.Unlock()

			cb.logger.Info("circuit breaker open, skipping collection",
				"collector", name,
				"failures", failures,
				"retry_in", remaining,
				"skips", skips,
			)

			return &collectors.CollectResult{
				Collector: name,
				Timestamp: time.Now(),
				Warnings: []string{fmt.Sprintf(
					"circuit breaker open for %s (failures: %d, retry in %s)",
					name, failures, remaining.Truncate(time.Second),
				)},
			}, nil
		}

		// Timeout elapsed, transition to half-open.
		cb.state = StateHalfOpen
		cb.logger.Info("circuit breaker transitioning to half-open",
			"collector", cb.collector.Name(),
		)
		cb.mu.Unlock()
		return cb.collectHalfOpen(ctx)

	case StateHalfOpen:
		cb.mu.Unlock()
		return cb.collectHalfOpen(ctx)

	default:
		cb.mu.Unlock()
		return nil, fmt.Errorf("circuit breaker in unknown state: %d", cb.state)
	}
}

// collectClosed runs the collector in closed (normal) state.
func (cb *CircuitBreaker) collectClosed(ctx context.Context) (*collectors.CollectResult, error) {
	result, err := cb.collector.Collect(ctx)
	if err != nil {
		cb.recordFailure()
		return result, err
	}

	cb.recordSuccess()
	return result, nil
}

// collectHalfOpen runs the collector as a probe to test recovery.
func (cb *CircuitBreaker) collectHalfOpen(ctx context.Context) (*collectors.CollectResult, error) {
	result, err := cb.collector.Collect(ctx)
	if err != nil {
		cb.mu.Lock()
		cb.failures++
		cb.totalFailures++
		cb.lastFailure = time.Now()

		// Increase timeout with backoff, capped at max.
		cb.currentTimeout = time.Duration(float64(cb.currentTimeout) * cb.config.BackoffMultiplier)
		if cb.currentTimeout > cb.config.MaxResetTimeout {
			cb.currentTimeout = cb.config.MaxResetTimeout
		}

		cb.state = StateOpen
		cb.logger.Warn("circuit breaker re-opened after half-open failure",
			"collector", cb.collector.Name(),
			"failures", cb.failures,
			"next_timeout", cb.currentTimeout,
		)
		cb.mu.Unlock()
		return result, err
	}

	// Success in half-open: close the circuit.
	cb.mu.Lock()
	cb.state = StateClosed
	cb.failures = 0
	cb.consecutiveSkips = 0
	cb.totalSuccesses++
	cb.lastSuccess = time.Now()
	cb.currentTimeout = cb.config.ResetTimeout
	cb.logger.Info("circuit breaker closed after successful probe",
		"collector", cb.collector.Name(),
	)
	cb.mu.Unlock()
	return result, nil
}

// recordFailure increments failure counters and optionally opens the circuit.
func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.totalFailures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.config.MaxFailures {
		cb.state = StateOpen
		cb.currentTimeout = cb.config.ResetTimeout
		cb.logger.Warn("circuit breaker opened",
			"collector", cb.collector.Name(),
			"failures", cb.failures,
			"timeout", cb.currentTimeout,
		)
	}
}

// recordSuccess resets the consecutive failure counter.
func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.consecutiveSkips = 0
	cb.totalSuccesses++
	cb.lastSuccess = time.Now()
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Stats returns a snapshot of the circuit breaker statistics.
func (cb *CircuitBreaker) Stats() Stats {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return Stats{
		State:            cb.state,
		ConsecutiveFails: cb.failures,
		TotalFailures:    cb.totalFailures,
		TotalSuccesses:   cb.totalSuccesses,
		LastFailure:      cb.lastFailure,
		LastSuccess:      cb.lastSuccess,
		CurrentTimeout:   cb.currentTimeout,
		ConsecutiveSkips: cb.consecutiveSkips,
	}
}

// Reset forces the circuit breaker back to the closed state, clearing all
// failure counters and restoring the initial timeout.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.consecutiveSkips = 0
	cb.currentTimeout = cb.config.ResetTimeout
	cb.logger.Info("circuit breaker manually reset",
		"collector", cb.collector.Name(),
	)
}
