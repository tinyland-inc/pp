package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HealthStatus represents the daemon health check output.
type HealthStatus struct {
	Status     string            `json:"status"`
	LastPoll   time.Time         `json:"last_poll"`
	Collectors map[string]string `json:"collectors"`
}

// healthFile is the filename for the daemon health check within the cache directory.
const healthFile = "health.json"

// writeHealthFile writes the health status to the cache directory.
func writeHealthFile(cacheDir string, collectorNames []string) error {
	status := HealthStatus{
		Status:     "ok",
		LastPoll:   time.Now(),
		Collectors: make(map[string]string, len(collectorNames)),
	}
	for _, name := range collectorNames {
		status.Collectors[name] = "ok"
	}

	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal health status: %w", err)
	}

	path := filepath.Join(cacheDir, healthFile)
	return os.WriteFile(path, data, 0644)
}

// readHealthFile reads the health status from the cache directory.
func readHealthFile(cacheDir string) (*HealthStatus, error) {
	path := filepath.Join(cacheDir, healthFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read health file: %w", err)
	}

	var status HealthStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("unmarshal health file: %w", err)
	}

	return &status, nil
}

// checkHealth reads the health file and reports whether the daemon is healthy.
// The daemon is considered healthy if the health file exists and the last poll
// was within 2x the poll interval. Returns exit code 0 for healthy, 1 for
// unhealthy/missing.
func checkHealth(cacheDir string, pollInterval time.Duration, jsonOutput bool) int {
	status, err := readHealthFile(cacheDir)
	if err != nil {
		if jsonOutput {
			fmt.Println(`{"status":"missing","error":"no health file found"}`)
		} else {
			fmt.Fprintln(os.Stderr, "daemon not running (no health file)")
		}
		return 1
	}

	staleThreshold := 2 * pollInterval
	age := time.Since(status.LastPoll)
	isStale := age > staleThreshold

	if jsonOutput {
		output := map[string]interface{}{
			"status":     status.Status,
			"last_poll":  status.LastPoll.Format(time.RFC3339),
			"age":        age.String(),
			"stale":      isStale,
			"collectors": status.Collectors,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
	} else {
		if isStale {
			fmt.Fprintf(os.Stderr, "daemon stale (last poll %s ago, threshold %s)\n", age.Round(time.Second), staleThreshold)
		} else {
			fmt.Printf("daemon healthy (last poll %s ago)\n", age.Round(time.Second))
			for name, s := range status.Collectors {
				fmt.Printf("  %s: %s\n", name, s)
			}
		}
	}

	if isStale {
		return 1
	}
	return 0
}
