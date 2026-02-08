package retry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// --- Mock Collector ---

type mockCollector struct {
	name        string
	description string
	interval    time.Duration
	results     []*collectors.CollectResult
	errors      []error
	calls       int
	mu          sync.Mutex
}

func (m *mockCollector) Name() string        { return m.name }
func (m *mockCollector) Description() string  { return m.description }
func (m *mockCollector) Interval() time.Duration { return m.interval }

func (m *mockCollector) Collect(ctx context.Context) (*collectors.CollectResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := m.calls
	m.calls++

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if idx < len(m.errors) && m.errors[idx] != nil {
		return nil, m.errors[idx]
	}
	if idx < len(m.results) {
		return m.results[idx], nil
	}
	return &collectors.CollectResult{
		Collector: m.name,
		Timestamp: time.Now(),
	}, nil
}

func (m *mockCollector) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func newMock(name string) *mockCollector {
	return &mockCollector{
		name:        name,
		description: name + " collector",
		interval:    5 * time.Minute,
	}
}

func newFailingMock(name string, n int) *mockCollector {
	m := newMock(name)
	m.errors = make([]error, n)
	for i := range n {
		m.errors[i] = fmt.Errorf("fail-%d", i)
	}
	return m
}

// --- Tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxFailures != 3 {
		t.Errorf("MaxFailures = %d, want 3", cfg.MaxFailures)
	}
	if cfg.ResetTimeout != 1*time.Minute {
		t.Errorf("ResetTimeout = %v, want 1m", cfg.ResetTimeout)
	}
	if cfg.MaxResetTimeout != 30*time.Minute {
		t.Errorf("MaxResetTimeout = %v, want 30m", cfg.MaxResetTimeout)
	}
	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier = %f, want 2.0", cfg.BackoffMultiplier)
	}
	if cfg.Logger != nil {
		t.Error("Logger should be nil by default")
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half_open"},
		{State(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestNewCircuitBreaker(t *testing.T) {
	m := newMock("test")
	cb := NewCircuitBreaker(m, DefaultConfig())

	if cb.State() != StateClosed {
		t.Errorf("initial state = %v, want StateClosed", cb.State())
	}
	if cb.Name() != "test" {
		t.Errorf("Name() = %q, want %q", cb.Name(), "test")
	}
}

func TestCircuitBreaker_Name(t *testing.T) {
	m := newMock("my-collector")
	cb := NewCircuitBreaker(m, DefaultConfig())

	if got := cb.Name(); got != "my-collector" {
		t.Errorf("Name() = %q, want %q", got, "my-collector")
	}
}

func TestCircuitBreaker_Description(t *testing.T) {
	m := newMock("desc-test")
	m.description = "A test collector"
	cb := NewCircuitBreaker(m, DefaultConfig())

	got := cb.Description()
	if !strings.Contains(got, "A test collector") {
		t.Errorf("Description() = %q, should contain wrapped description", got)
	}
	if !strings.Contains(got, "[circuit: closed]") {
		t.Errorf("Description() = %q, should contain circuit state", got)
	}
}

func TestCircuitBreaker_Interval(t *testing.T) {
	m := newMock("interval-test")
	m.interval = 10 * time.Minute
	cb := NewCircuitBreaker(m, DefaultConfig())

	if got := cb.Interval(); got != 10*time.Minute {
		t.Errorf("Interval() = %v, want 10m", got)
	}
}

func TestCollect_Success_StaysClosed(t *testing.T) {
	m := newMock("success")
	cb := NewCircuitBreaker(m, DefaultConfig())

	result, err := cb.Collect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Collector != "success" {
		t.Errorf("result.Collector = %q, want %q", result.Collector, "success")
	}
	if cb.State() != StateClosed {
		t.Errorf("state = %v, want StateClosed", cb.State())
	}
}

func TestCollect_SingleFailure_StaysClosed(t *testing.T) {
	m := newFailingMock("single-fail", 1)
	cfg := DefaultConfig()
	cfg.MaxFailures = 3
	cb := NewCircuitBreaker(m, cfg)

	_, err := cb.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if cb.State() != StateClosed {
		t.Errorf("state = %v, want StateClosed after 1 failure", cb.State())
	}
}

func TestCollect_MaxFailures_OpensCircuit(t *testing.T) {
	m := newFailingMock("max-fail", 5)
	cfg := DefaultConfig()
	cfg.MaxFailures = 3
	cb := NewCircuitBreaker(m, cfg)

	for i := range 3 {
		_, err := cb.Collect(context.Background())
		if err == nil {
			t.Fatalf("call %d: expected error, got nil", i)
		}
	}

	if cb.State() != StateOpen {
		t.Errorf("state = %v, want StateOpen after %d failures", cb.State(), 3)
	}
}

func TestCollect_OpenCircuit_SkipsCollection(t *testing.T) {
	m := newFailingMock("skip", 5)
	cfg := DefaultConfig()
	cfg.MaxFailures = 2
	cfg.ResetTimeout = 1 * time.Hour // long timeout so circuit stays open
	cb := NewCircuitBreaker(m, cfg)

	// Open the circuit.
	for range 2 {
		cb.Collect(context.Background())
	}
	if cb.State() != StateOpen {
		t.Fatalf("state = %v, want StateOpen", cb.State())
	}

	callsBefore := m.callCount()

	// This should return a synthetic result without calling the collector.
	result, err := cb.Collect(context.Background())
	if err != nil {
		t.Fatalf("open circuit should return nil error, got: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Error("open circuit result should have warnings")
	}
	if m.callCount() != callsBefore {
		t.Errorf("collector should not have been called; calls went from %d to %d",
			callsBefore, m.callCount())
	}
}

func TestCollect_OpenCircuit_TransitionsToHalfOpen(t *testing.T) {
	m := newMock("half-open-transition")
	m.errors = []error{
		errors.New("fail-1"),
		errors.New("fail-2"),
		nil, // half-open probe succeeds
	}
	cfg := DefaultConfig()
	cfg.MaxFailures = 2
	cfg.ResetTimeout = 10 * time.Millisecond
	cb := NewCircuitBreaker(m, cfg)

	// Open the circuit.
	for range 2 {
		cb.Collect(context.Background())
	}

	// Wait for the timeout to elapse.
	time.Sleep(20 * time.Millisecond)

	// Next collect should transition to half-open and probe.
	_, err := cb.Collect(context.Background())
	if err != nil {
		t.Fatalf("half-open probe should succeed, got error: %v", err)
	}

	// After a successful probe, circuit should be closed.
	if cb.State() != StateClosed {
		t.Errorf("state = %v, want StateClosed after successful half-open probe", cb.State())
	}
}

func TestCollect_HalfOpen_SuccessCloses(t *testing.T) {
	// 2 failures to open, then 1 success in half-open.
	m := newFailingMock("ho-success", 2)
	cfg := DefaultConfig()
	cfg.MaxFailures = 2
	cfg.ResetTimeout = 5 * time.Millisecond
	cb := NewCircuitBreaker(m, cfg)

	for range 2 {
		cb.Collect(context.Background())
	}

	time.Sleep(10 * time.Millisecond)

	result, err := cb.Collect(context.Background())
	if err != nil {
		t.Fatalf("expected success in half-open, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if cb.State() != StateClosed {
		t.Errorf("state = %v, want StateClosed", cb.State())
	}
	stats := cb.Stats()
	if stats.ConsecutiveFails != 0 {
		t.Errorf("ConsecutiveFails = %d, want 0 after close", stats.ConsecutiveFails)
	}
}

func TestCollect_HalfOpen_FailureReopens(t *testing.T) {
	// 3 failures to open, then 1 more failure in half-open.
	m := newFailingMock("ho-fail", 10)
	cfg := DefaultConfig()
	cfg.MaxFailures = 3
	cfg.ResetTimeout = 5 * time.Millisecond
	cfg.BackoffMultiplier = 2.0
	cb := NewCircuitBreaker(m, cfg)

	for range 3 {
		cb.Collect(context.Background())
	}

	time.Sleep(10 * time.Millisecond)

	// Half-open probe fails.
	_, err := cb.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error in half-open probe")
	}
	if cb.State() != StateOpen {
		t.Errorf("state = %v, want StateOpen after half-open failure", cb.State())
	}

	stats := cb.Stats()
	expectedTimeout := 10 * time.Millisecond // 5ms * 2.0
	if stats.CurrentTimeout != expectedTimeout {
		t.Errorf("CurrentTimeout = %v, want %v", stats.CurrentTimeout, expectedTimeout)
	}
}

func TestCollect_BackoffMultiplier(t *testing.T) {
	m := newFailingMock("backoff", 100)
	cfg := DefaultConfig()
	cfg.MaxFailures = 1
	cfg.ResetTimeout = 5 * time.Millisecond
	cfg.MaxResetTimeout = 1 * time.Second
	cfg.BackoffMultiplier = 3.0
	cb := NewCircuitBreaker(m, cfg)

	// First failure opens the circuit with 5ms timeout.
	cb.Collect(context.Background())
	if cb.Stats().CurrentTimeout != 5*time.Millisecond {
		t.Fatalf("initial timeout = %v, want 5ms", cb.Stats().CurrentTimeout)
	}

	// Wait and let half-open fail, timeout should become 15ms.
	time.Sleep(10 * time.Millisecond)
	cb.Collect(context.Background())
	if cb.Stats().CurrentTimeout != 15*time.Millisecond {
		t.Fatalf("second timeout = %v, want 15ms", cb.Stats().CurrentTimeout)
	}

	// Wait and let half-open fail again, timeout should become 45ms.
	time.Sleep(20 * time.Millisecond)
	cb.Collect(context.Background())
	if cb.Stats().CurrentTimeout != 45*time.Millisecond {
		t.Fatalf("third timeout = %v, want 45ms", cb.Stats().CurrentTimeout)
	}
}

func TestCollect_MaxResetTimeout(t *testing.T) {
	m := newFailingMock("max-timeout", 100)
	cfg := DefaultConfig()
	cfg.MaxFailures = 1
	cfg.ResetTimeout = 5 * time.Millisecond
	cfg.MaxResetTimeout = 20 * time.Millisecond
	cfg.BackoffMultiplier = 10.0
	cb := NewCircuitBreaker(m, cfg)

	// Open circuit: timeout = 5ms.
	cb.Collect(context.Background())

	// Half-open fail: timeout = min(50ms, 20ms) = 20ms.
	time.Sleep(10 * time.Millisecond)
	cb.Collect(context.Background())

	stats := cb.Stats()
	if stats.CurrentTimeout != 20*time.Millisecond {
		t.Errorf("CurrentTimeout = %v, want 20ms (capped)", stats.CurrentTimeout)
	}

	// Another cycle should still be capped at 20ms.
	time.Sleep(25 * time.Millisecond)
	cb.Collect(context.Background())

	stats = cb.Stats()
	if stats.CurrentTimeout != 20*time.Millisecond {
		t.Errorf("CurrentTimeout = %v, want 20ms (still capped)", stats.CurrentTimeout)
	}
}

func TestCollect_SuccessResetsFailureCount(t *testing.T) {
	m := newMock("reset-count")
	m.errors = []error{
		errors.New("fail-1"),
		errors.New("fail-2"),
		nil, // success
		errors.New("fail-3"),
	}
	cfg := DefaultConfig()
	cfg.MaxFailures = 3
	cb := NewCircuitBreaker(m, cfg)

	// Two failures.
	cb.Collect(context.Background())
	cb.Collect(context.Background())
	if cb.Stats().ConsecutiveFails != 2 {
		t.Fatalf("ConsecutiveFails = %d, want 2", cb.Stats().ConsecutiveFails)
	}

	// One success resets count.
	cb.Collect(context.Background())
	if cb.Stats().ConsecutiveFails != 0 {
		t.Errorf("ConsecutiveFails = %d, want 0 after success", cb.Stats().ConsecutiveFails)
	}
	if cb.State() != StateClosed {
		t.Errorf("state = %v, want StateClosed", cb.State())
	}

	// Next failure starts count from 1 again.
	cb.Collect(context.Background())
	if cb.Stats().ConsecutiveFails != 1 {
		t.Errorf("ConsecutiveFails = %d, want 1", cb.Stats().ConsecutiveFails)
	}
}

func TestCollect_ContextCancelled(t *testing.T) {
	m := newMock("ctx-cancel")
	cb := NewCircuitBreaker(m, DefaultConfig())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := cb.Collect(ctx)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestStats(t *testing.T) {
	m := newMock("stats")
	m.errors = []error{errors.New("e1"), errors.New("e2"), nil}
	cfg := DefaultConfig()
	cfg.MaxFailures = 5
	cb := NewCircuitBreaker(m, cfg)

	// Two failures.
	cb.Collect(context.Background())
	cb.Collect(context.Background())

	stats := cb.Stats()
	if stats.State != StateClosed {
		t.Errorf("State = %v, want StateClosed", stats.State)
	}
	if stats.ConsecutiveFails != 2 {
		t.Errorf("ConsecutiveFails = %d, want 2", stats.ConsecutiveFails)
	}
	if stats.TotalFailures != 2 {
		t.Errorf("TotalFailures = %d, want 2", stats.TotalFailures)
	}
	if stats.TotalSuccesses != 0 {
		t.Errorf("TotalSuccesses = %d, want 0", stats.TotalSuccesses)
	}
	if stats.LastFailure.IsZero() {
		t.Error("LastFailure should be set")
	}

	// One success.
	cb.Collect(context.Background())

	stats = cb.Stats()
	if stats.TotalSuccesses != 1 {
		t.Errorf("TotalSuccesses = %d, want 1", stats.TotalSuccesses)
	}
	if stats.TotalFailures != 2 {
		t.Errorf("TotalFailures = %d, want 2 (unchanged)", stats.TotalFailures)
	}
	if stats.LastSuccess.IsZero() {
		t.Error("LastSuccess should be set")
	}
	if stats.ConsecutiveFails != 0 {
		t.Errorf("ConsecutiveFails = %d, want 0 after success", stats.ConsecutiveFails)
	}
}

func TestReset(t *testing.T) {
	m := newFailingMock("reset", 5)
	cfg := DefaultConfig()
	cfg.MaxFailures = 2
	cfg.ResetTimeout = 1 * time.Hour
	cb := NewCircuitBreaker(m, cfg)

	// Open the circuit.
	cb.Collect(context.Background())
	cb.Collect(context.Background())
	if cb.State() != StateOpen {
		t.Fatalf("state = %v, want StateOpen", cb.State())
	}

	// Manual reset.
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("state = %v, want StateClosed after Reset()", cb.State())
	}
	stats := cb.Stats()
	if stats.ConsecutiveFails != 0 {
		t.Errorf("ConsecutiveFails = %d, want 0 after Reset()", stats.ConsecutiveFails)
	}
	if stats.ConsecutiveSkips != 0 {
		t.Errorf("ConsecutiveSkips = %d, want 0 after Reset()", stats.ConsecutiveSkips)
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := newMock("concurrent")
	cfg := DefaultConfig()
	cfg.MaxFailures = 100 // high threshold so circuit stays closed
	cb := NewCircuitBreaker(m, cfg)

	const goroutines = 50
	const callsPerGoroutine = 20

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*callsPerGoroutine)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range callsPerGoroutine {
				_, err := cb.Collect(context.Background())
				if err != nil {
					errCh <- err
				}
				// Also exercise Stats() and State() concurrently.
				cb.Stats()
				cb.State()
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("unexpected error during concurrent access: %v", err)
	}

	stats := cb.Stats()
	if stats.TotalSuccesses != goroutines*callsPerGoroutine {
		t.Errorf("TotalSuccesses = %d, want %d", stats.TotalSuccesses, goroutines*callsPerGoroutine)
	}
}

func TestCollect_OpenCircuit_WarningMessage(t *testing.T) {
	m := newFailingMock("warning-msg", 5)
	cfg := DefaultConfig()
	cfg.MaxFailures = 2
	cfg.ResetTimeout = 1 * time.Hour
	cb := NewCircuitBreaker(m, cfg)

	// Open the circuit.
	cb.Collect(context.Background())
	cb.Collect(context.Background())

	result, err := cb.Collect(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}

	w := result.Warnings[0]
	if !strings.Contains(w, "circuit breaker open for warning-msg") {
		t.Errorf("warning = %q, should contain collector name", w)
	}
	if !strings.Contains(w, "failures: 2") {
		t.Errorf("warning = %q, should contain failure count", w)
	}
	if !strings.Contains(w, "retry in") {
		t.Errorf("warning = %q, should contain retry info", w)
	}
}

func TestCollect_MultipleResets(t *testing.T) {
	// Sequence: open -> half-open -> fail -> open -> half-open -> succeed -> closed.
	callIdx := 0
	m := newMock("multi-reset")
	m.errors = []error{
		errors.New("f1"), // open circuit
		errors.New("f2"), // half-open probe fails, re-open
		nil,              // half-open probe succeeds, close
	}
	_ = callIdx

	cfg := DefaultConfig()
	cfg.MaxFailures = 1
	cfg.ResetTimeout = 5 * time.Millisecond
	cfg.BackoffMultiplier = 2.0
	cb := NewCircuitBreaker(m, cfg)

	// First failure opens the circuit.
	cb.Collect(context.Background())
	if cb.State() != StateOpen {
		t.Fatalf("step 1: state = %v, want StateOpen", cb.State())
	}

	// Wait for timeout, half-open probe fails, re-opens.
	time.Sleep(10 * time.Millisecond)
	_, err := cb.Collect(context.Background())
	if err == nil {
		t.Fatal("step 2: expected error in half-open probe")
	}
	if cb.State() != StateOpen {
		t.Fatalf("step 2: state = %v, want StateOpen", cb.State())
	}

	// Timeout is now 10ms (5ms * 2.0). Wait and let half-open succeed.
	time.Sleep(15 * time.Millisecond)
	_, err = cb.Collect(context.Background())
	if err != nil {
		t.Fatalf("step 3: expected success, got: %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("step 3: state = %v, want StateClosed", cb.State())
	}
}

func TestCollect_InterfaceCompliance(t *testing.T) {
	m := newMock("iface")
	cb := NewCircuitBreaker(m, DefaultConfig())

	// This is also verified at compile time, but test it explicitly.
	var c collectors.Collector = cb
	if c.Name() != "iface" {
		t.Errorf("interface Name() = %q, want %q", c.Name(), "iface")
	}
}

func TestCollect_NilLogger(t *testing.T) {
	m := newFailingMock("nil-logger", 5)
	cfg := DefaultConfig()
	cfg.MaxFailures = 2
	cfg.Logger = nil // explicitly nil
	cb := NewCircuitBreaker(m, cfg)

	// Should not panic with nil logger.
	cb.Collect(context.Background())
	cb.Collect(context.Background())

	// Circuit is now open, this should also not panic.
	result, err := cb.Collect(context.Background())
	if err != nil {
		t.Fatalf("expected nil error for open circuit, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil synthetic result")
	}

	// Reset should also not panic.
	cb.Reset()
}

func TestCollect_ZeroMaxFailures(t *testing.T) {
	m := newFailingMock("zero-max", 5)
	cfg := DefaultConfig()
	cfg.MaxFailures = 0 // zero means immediate open on any failure
	cfg.ResetTimeout = 1 * time.Hour
	cb := NewCircuitBreaker(m, cfg)

	// A single failure with MaxFailures=0 means failures(1) >= 0, so circuit opens.
	_, err := cb.Collect(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if cb.State() != StateOpen {
		t.Errorf("state = %v, want StateOpen after first failure with MaxFailures=0", cb.State())
	}
}
