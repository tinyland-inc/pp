//go:build darwin

package banner

import (
	"time"

	"golang.org/x/sys/unix"
)

// getSystemUptime returns the system uptime on macOS by reading kern.boottime.
func getSystemUptime() time.Duration {
	tv, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return 0
	}
	bootTime := time.Unix(tv.Sec, int64(tv.Usec)*1000)
	return time.Since(bootTime)
}
