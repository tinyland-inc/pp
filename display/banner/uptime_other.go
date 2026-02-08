//go:build !linux && !darwin

package banner

import "time"

// getSystemUptime returns 0 on unsupported platforms.
func getSystemUptime() time.Duration {
	return 0
}
