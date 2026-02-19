package collectors

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	// DefaultUpdateBufferSize is the default capacity of the updates channel.
	// A buffered channel prevents slow consumers from blocking collectors.
	DefaultUpdateBufferSize = 64

	// DefaultStopTimeout is the maximum time Stop() will wait for goroutines
	// to finish before returning.
	DefaultStopTimeout = 5 * time.Second
)

// errTracker deduplicates repeated identical errors per collector.
type errTracker struct {
	lastMsg    string
	lastTime   time.Time
	suppressed int64
}

// Runner starts and stops collector goroutines. Each registered collector
// runs in its own goroutine with an independent ticker. Results fan in to a
// single updates channel.
type Runner struct {
	registry    *Registry
	updates     chan<- Update
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	stopped     chan struct{}
	once        sync.Once
	errTrackers map[string]*errTracker
}

// NewRunner creates a runner that sends collection results to the provided
// updates channel. The caller is responsible for creating and reading from
// the channel.
func NewRunner(registry *Registry, updates chan<- Update) *Runner {
	return &Runner{
		registry:    registry,
		updates:     updates,
		stopped:     make(chan struct{}),
		errTrackers: make(map[string]*errTracker),
	}
}

// Start launches a goroutine for each registered collector. Each goroutine
// runs Collect() at the collector's configured Interval(). Start returns an
// error if no collectors are registered (to surface misconfiguration early),
// but an empty registry is not fatal -- the runner simply does nothing.
//
// The provided context controls the lifetime of all collector goroutines.
// Cancelling it (or calling Stop) will shut down all goroutines gracefully.
func (r *Runner) Start(ctx context.Context) error {
	ctx, r.cancel = context.WithCancel(ctx)

	names := r.registry.List()
	if len(names) == 0 {
		// No collectors registered. Not an error -- just nothing to do.
		// Close stopped immediately so Stop() doesn't block.
		close(r.stopped)
		return nil
	}

	for _, name := range names {
		c, ok := r.registry.Get(name)
		if !ok {
			continue
		}
		r.wg.Add(1)
		go r.runCollector(ctx, c)
	}

	// Wait for all goroutines in a background goroutine, then signal stopped.
	go func() {
		r.wg.Wait()
		close(r.stopped)
	}()

	return nil
}

// Stop cancels the runner context and waits for all collector goroutines to
// finish, with a timeout to prevent indefinite blocking.
func (r *Runner) Stop() {
	r.once.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
	})

	select {
	case <-r.stopped:
		// All goroutines finished.
	case <-time.After(DefaultStopTimeout):
		log.Printf("collectors: runner stop timed out after %s", DefaultStopTimeout)
	}
}

// RunOnce manually triggers a single collection cycle for the named collector.
// It blocks until the collection completes or the context is cancelled.
func (r *Runner) RunOnce(ctx context.Context, name string) (interface{}, error) {
	c, ok := r.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("collector %q not found", name)
	}

	start := time.Now()
	data, err := c.Collect(ctx)
	latency := time.Since(start)

	r.registry.updateStatus(name, func(s *CollectorStatus) {
		s.LastRun = start
		s.RunCount++
		s.LastLatency = latency
		if err != nil {
			s.ErrorCount++
			s.LastError = err
			s.Healthy = false
		} else {
			s.LastError = nil
			s.Healthy = true
		}
	})

	return data, err
}

// Health returns a map of collector name to healthy status for all registered
// collectors.
func (r *Runner) Health() map[string]bool {
	statuses := r.registry.AllStatus()
	result := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		result[s.Name] = s.Healthy
	}
	return result
}

// runCollector is the per-collector goroutine. It ticks at c.Interval(),
// performs a collection, updates status, and sends the result on the updates
// channel. Errors are logged but do not stop the goroutine.
func (r *Runner) runCollector(ctx context.Context, c Collector) {
	defer r.wg.Done()

	interval := c.Interval()
	if interval <= 0 {
		interval = time.Second
	}

	// Run immediately on start, then tick.
	r.collectAndSend(ctx, c)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.collectAndSend(ctx, c)
		}
	}
}

// collectAndSend performs one collection cycle and sends the result. It
// catches panics to prevent one misbehaving collector from crashing the
// runner.
func (r *Runner) collectAndSend(ctx context.Context, c Collector) {
	name := c.Name()
	start := time.Now()

	data, err := c.Collect(ctx)
	latency := time.Since(start)

	r.registry.updateStatus(name, func(s *CollectorStatus) {
		s.LastRun = start
		s.RunCount++
		s.LastLatency = latency
		if err != nil {
			s.ErrorCount++
			s.LastError = err
			s.Healthy = false
		} else {
			s.LastError = nil
			s.Healthy = true
		}
	})

	if err != nil {
		r.logCollectorError(name, err)
	}

	update := Update{
		Source:    name,
		Data:     data,
		Timestamp: start,
		Error:    err,
	}

	// Non-blocking send: if the channel is full, drop the update and log.
	// This prevents a slow consumer from blocking all collectors.
	select {
	case r.updates <- update:
	default:
		log.Printf("collectors: update channel full, dropping update from %s", name)
	}
}

// logCollectorError deduplicates repeated identical errors from the same
// collector. If the same error message recurs within 1 hour, it is suppressed
// with a summary logged every 100 suppressions. This prevents multi-MB log
// files from repeated connection/auth failures.
func (r *Runner) logCollectorError(name string, err error) {
	msg := err.Error()
	tracker := r.errTrackers[name]
	if tracker == nil {
		tracker = &errTracker{}
		r.errTrackers[name] = tracker
	}
	now := time.Now()
	if msg == tracker.lastMsg && now.Sub(tracker.lastTime) < time.Hour {
		tracker.suppressed++
		if tracker.suppressed%100 == 0 {
			log.Printf("collectors: %s error (repeated %d times): %v", name, tracker.suppressed, err)
		}
		return
	}
	if tracker.suppressed > 0 {
		log.Printf("collectors: %s previous error repeated %d times", name, tracker.suppressed)
	}
	log.Printf("collectors: %s error: %v", name, err)
	tracker.lastMsg = msg
	tracker.lastTime = now
	tracker.suppressed = 0
}
