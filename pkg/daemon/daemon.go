// Package daemon implements the background data collection daemon for
// prompt-pulse. It manages PID file locking, health reporting, IPC via Unix
// sockets, and a pre-rendered banner cache inspired by powerlevel10k's instant
// prompt technique.
package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/pkg/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/config"
)

// Config holds all configuration for the daemon process.
type Config struct {
	// PIDFile is the path to the PID file used for singleton enforcement.
	// Default: $XDG_RUNTIME_DIR/prompt-pulse.pid or /tmp/prompt-pulse-{uid}.pid
	PIDFile string

	// HealthFile is the path to the health status JSON file.
	// Default: alongside PID file.
	HealthFile string

	// SocketPath is the Unix socket path for IPC communication.
	// Default: alongside PID file with .sock extension.
	SocketPath string

	// DataDir is the directory for persistent data storage.
	DataDir string

	// BannerCacheFile is the path to the pre-rendered banner cache.
	// Default: alongside PID file with -banner.json suffix.
	BannerCacheFile string
}

// DefaultConfig returns a Config with platform-appropriate default paths.
func DefaultConfig() Config {
	base := defaultBasePath()

	return Config{
		PIDFile:         filepath.Join(base, "prompt-pulse.pid"),
		HealthFile:      filepath.Join(base, "prompt-pulse-health.json"),
		SocketPath:      filepath.Join(base, "prompt-pulse.sock"),
		DataDir:         filepath.Join(base, "data"),
		BannerCacheFile: filepath.Join(base, "prompt-pulse-banner.json"),
	}
}

// defaultBasePath returns XDG_RUNTIME_DIR if set, otherwise /tmp/prompt-pulse-{uid}.
func defaultBasePath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir
	}
	return fmt.Sprintf("/tmp/prompt-pulse-%d", os.Getuid())
}

// HealthStatus represents the current state of the daemon and its collectors.
type HealthStatus struct {
	PID        int                        `json:"pid"`
	Uptime     time.Duration              `json:"uptime_ns"`
	StartedAt  time.Time                  `json:"started_at"`
	Collectors map[string]CollectorHealth `json:"collectors"`
	LastUpdate time.Time                  `json:"last_update"`
}

// CollectorHealth tracks the health of a single collector within the daemon.
type CollectorHealth struct {
	Name       string    `json:"name"`
	Healthy    bool      `json:"healthy"`
	LastRun    time.Time `json:"last_run"`
	ErrorCount int64     `json:"error_count"`
}

// Daemon is the main background process that orchestrates data collection,
// health reporting, and IPC.
type Daemon struct {
	cfg       Config
	appCfg    *config.Config
	startedAt time.Time
	running   bool
	ipc       *IPCServer
	banner    *BannerCache

	// collectors tracks health state for registered collectors.
	collectors map[string]*CollectorHealth

	mu sync.Mutex
}

// SetAppConfig sets the application configuration used to build and start
// data collectors when the daemon starts. Must be called before Start().
func (d *Daemon) SetAppConfig(cfg *config.Config) {
	d.appCfg = cfg
}

// New validates the configuration and returns a Daemon ready to be started.
// It does not start any background processes.
func New(cfg Config) (*Daemon, error) {
	if cfg.PIDFile == "" {
		return nil, fmt.Errorf("daemon: PIDFile must not be empty")
	}
	if cfg.HealthFile == "" {
		return nil, fmt.Errorf("daemon: HealthFile must not be empty")
	}
	if cfg.SocketPath == "" {
		return nil, fmt.Errorf("daemon: SocketPath must not be empty")
	}
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("daemon: DataDir must not be empty")
	}
	if cfg.BannerCacheFile == "" {
		return nil, fmt.Errorf("daemon: BannerCacheFile must not be empty")
	}

	return &Daemon{
		cfg:        cfg,
		collectors: make(map[string]*CollectorHealth),
		banner:     NewBannerCache(cfg.BannerCacheFile),
	}, nil
}

// Start acquires the PID lock, starts the IPC server, and enters the main
// collection loop. It blocks until the context is cancelled or an error occurs.
func (d *Daemon) Start(ctx context.Context) error {
	// Ensure directories exist.
	for _, dir := range []string{
		filepath.Dir(d.cfg.PIDFile),
		filepath.Dir(d.cfg.HealthFile),
		filepath.Dir(d.cfg.SocketPath),
		d.cfg.DataDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("daemon: create directory %s: %w", dir, err)
		}
	}

	// Acquire PID lock.
	if err := AcquirePID(d.cfg.PIDFile); err != nil {
		return fmt.Errorf("daemon: acquire PID: %w", err)
	}

	d.mu.Lock()
	d.startedAt = time.Now()
	d.running = true
	d.mu.Unlock()

	// Start IPC server.
	d.ipc = NewIPCServer(d.cfg.SocketPath, d)
	if err := d.ipc.Start(); err != nil {
		ReleasePID(d.cfg.PIDFile)
		d.mu.Lock()
		d.running = false
		d.mu.Unlock()
		return fmt.Errorf("daemon: start IPC: %w", err)
	}

	// Write initial health.
	if err := d.WriteHealth(); err != nil {
		// Non-fatal: log but continue.
		_ = err
	}

	// Start collectors if app config is available.
	var runner *collectors.Runner
	if d.appCfg != nil {
		reg := BuildRegistry(d.appCfg)
		names := reg.List()
		if len(names) > 0 {
			log.Printf("daemon: starting %d collectors: %v", len(names), names)
			updates := make(chan collectors.Update, collectors.DefaultUpdateBufferSize)
			runner = collectors.NewRunner(reg, updates)
			if err := runner.Start(ctx); err != nil {
				log.Printf("daemon: start collectors: %v", err)
			} else {
				cacheDir := d.appCfg.General.CacheDir
				if cacheDir == "" {
					cacheDir = d.cfg.DataDir
				}
				go ConsumeUpdates(ctx, updates, cacheDir, d)
			}
		} else {
			log.Printf("daemon: no collectors enabled")
		}
	}

	// Main loop: write health periodically until context is cancelled.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if runner != nil {
				runner.Stop()
			}
			return d.Stop()
		case <-ticker.C:
			_ = d.WriteHealth()
		}
	}
}

// Stop performs a graceful shutdown: stops the IPC server, removes the PID
// file, and cleans up the socket.
func (d *Daemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	d.running = false

	// Stop IPC server.
	if d.ipc != nil {
		d.ipc.Stop()
	}

	// Remove PID file.
	if err := ReleasePID(d.cfg.PIDFile); err != nil {
		return fmt.Errorf("daemon: release PID: %w", err)
	}

	return nil
}

// IsRunning checks whether a daemon instance is alive by reading the PID file
// and probing the process.
func (d *Daemon) IsRunning() bool {
	pid, err := ReadPID(d.cfg.PIDFile)
	if err != nil {
		return false
	}
	return IsProcessAlive(pid)
}

// Health reads the current health status from the health file.
func (d *Daemon) Health() (*HealthStatus, error) {
	return ReadHealthFile(d.cfg.HealthFile)
}

// WriteHealth writes the current daemon health to the health file.
func (d *Daemon) WriteHealth() error {
	d.mu.Lock()
	collectors := make(map[string]CollectorHealth, len(d.collectors))
	for k, v := range d.collectors {
		collectors[k] = *v
	}
	startedAt := d.startedAt
	d.mu.Unlock()

	status := &HealthStatus{
		PID:        os.Getpid(),
		Uptime:     time.Since(startedAt),
		StartedAt:  startedAt,
		Collectors: collectors,
		LastUpdate: time.Now(),
	}

	return WriteHealthFile(d.cfg.HealthFile, status)
}

// Running returns whether the daemon is currently in its main loop.
func (d *Daemon) Running() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}

// UpdateCollector updates the health state for a named collector.
func (d *Daemon) UpdateCollector(name string, healthy bool, errCount int64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.collectors[name] = &CollectorHealth{
		Name:       name,
		Healthy:    healthy,
		LastRun:    time.Now(),
		ErrorCount: errCount,
	}
}

// HandleCommand implements the IPCHandler interface, dispatching IPC commands.
func (d *Daemon) HandleCommand(cmd string, args map[string]string) (string, error) {
	switch cmd {
	case "HEALTH":
		status, err := d.Health()
		if err != nil {
			// If health file does not exist yet, build from memory.
			d.mu.Lock()
			collectors := make(map[string]CollectorHealth, len(d.collectors))
			for k, v := range d.collectors {
				collectors[k] = *v
			}
			startedAt := d.startedAt
			d.mu.Unlock()

			status = &HealthStatus{
				PID:        os.Getpid(),
				Uptime:     time.Since(startedAt),
				StartedAt:  startedAt,
				Collectors: collectors,
				LastUpdate: time.Now(),
			}
		}
		return healthStatusToJSON(status)

	case "BANNER":
		width, _ := strconv.Atoi(args["width"])
		height, _ := strconv.Atoi(args["height"])
		protocol := args["protocol"]
		entry, ok := d.banner.Get(width, height, protocol)
		if !ok {
			return "", fmt.Errorf("no cached banner for %dx%d/%s", width, height, protocol)
		}
		return bannerEntryToJSON(entry)

	case "REFRESH":
		// In a full implementation, this would trigger a collection cycle.
		return `{"status":"ok","message":"refresh triggered"}`, nil

	case "QUIT":
		go func() {
			// Allow the response to be sent before stopping.
			time.Sleep(100 * time.Millisecond)
			d.Stop()
		}()
		return `{"status":"ok","message":"shutting down"}`, nil

	default:
		return "", fmt.Errorf("unknown command: %s", cmd)
	}
}
