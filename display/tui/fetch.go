package tui

import (
	"log/slog"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/tinyland/lab/prompt-pulse/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
)

// fetchDataCmd returns a tea.Cmd that reads all collector data from the cache.
// This runs as a non-blocking command to avoid freezing the TUI during I/O.
func fetchDataCmd(cacheDir string, ttl time.Duration) tea.Cmd {
	return func() tea.Msg {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		}))
		store, err := cache.NewStore(cacheDir, logger)
		if err != nil {
			return dataRefreshMsg{err: err}
		}

		claude, _, _ := cache.GetTyped[collectors.ClaudeUsage](store, "claude", ttl)
		billing, _, _ := cache.GetTyped[collectors.BillingData](store, "billing", ttl)
		infra, _, _ := cache.GetTyped[collectors.InfraStatus](store, "infra", ttl)
		fastfetch, _, _ := cache.GetTyped[collectors.FastfetchData](store, "fastfetch", ttl)
		sysmetrics, _, _ := cache.GetTyped[collectors.SysMetricsData](store, "sysmetrics", ttl)

		return dataRefreshMsg{
			claude:     claude,
			billing:    billing,
			infra:      infra,
			fastfetch:  fastfetch,
			sysmetrics: sysmetrics,
		}
	}
}
