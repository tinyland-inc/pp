//go:build linux

package banner

import (
	"os"
	"time"
)

// getSystemUptime returns the system uptime on Linux by reading /proc/uptime.
func getSystemUptime() time.Duration {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	seconds, err := parseUptimeSeconds(data)
	if err != nil {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
