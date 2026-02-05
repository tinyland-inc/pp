// Package banner orchestrates the full banner generation pipeline.
//
// The banner combines cached collector data (Claude usage, billing, infrastructure),
// system status evaluation, optional waifu image selection, and layout rendering
// into a single terminal-ready string. It is designed to complete in under 100ms
// when all data is cached.
package banner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"gitlab.com/tinyland/lab/prompt-pulse/cache"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/layout"
	"gitlab.com/tinyland/lab/prompt-pulse/display/widgets"
	"gitlab.com/tinyland/lab/prompt-pulse/status"
	"gitlab.com/tinyland/lab/prompt-pulse/waifu"
)

// BannerConfig controls banner generation behavior.
type BannerConfig struct {
	// CacheDir is the prompt-pulse cache directory.
	CacheDir string
	// CacheTTL is how long cached collector data is considered fresh.
	CacheTTL time.Duration
	// WaifuEnabled enables waifu image display.
	WaifuEnabled bool
	// WaifuCategory overrides automatic category selection.
	WaifuCategory string
	// WaifuCacheDir is the directory for cached waifu images.
	WaifuCacheDir string
	// WaifuCacheTTL is how long cached images are valid.
	WaifuCacheTTL time.Duration
	// WaifuMaxCacheMB is the max image cache size.
	WaifuMaxCacheMB int
	// WaifuSessionID is the shell session ID for per-session waifu caching.
	// If empty, falls back to GetSessionKey() or category-based caching.
	WaifuSessionID string
	// WaifuMaxSessions is the max number of session images to keep (LRU eviction).
	WaifuMaxSessions int
	// FastfetchEnabled enables fastfetch system info in the center column.
	FastfetchEnabled bool
	// Hostname overrides os.Hostname().
	Hostname string
	// TermWidth overrides terminal width detection.
	TermWidth int
	// TermHeight overrides terminal height detection.
	TermHeight int
	// Logger for banner operations.
	Logger *slog.Logger
}

// DefaultBannerConfig returns sensible defaults for banner generation.
func DefaultBannerConfig() BannerConfig {
	home, _ := os.UserHomeDir()
	cacheBase := home + "/.cache/prompt-pulse"
	return BannerConfig{
		CacheDir:         cacheBase,
		CacheTTL:         15 * time.Minute,
		WaifuEnabled:     false,
		WaifuCacheDir:    cacheBase + "/waifu",
		WaifuCacheTTL:    24 * time.Hour,
		WaifuMaxCacheMB:  50,
		WaifuMaxSessions: waifu.DefaultMaxSessions,
		TermWidth:        80,
		TermHeight:       24,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// Banner orchestrates the full banner generation pipeline:
//  1. Load cached collector data
//  2. Evaluate system status
//  3. Select waifu category based on status
//  4. Fetch/cache waifu image
//  5. Render banner layout
type Banner struct {
	config BannerConfig
}

// NewBanner creates a Banner with the given configuration.
// If Logger is nil, a no-op logger is used.
func NewBanner(cfg BannerConfig) *Banner {
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Banner{config: cfg}
}

// Generate produces the complete banner string.
// It reads cached collector data, evaluates status, optionally fetches a waifu
// image, and composes the final layout. Designed to complete in <100ms with
// cached data. Returns the banner string or an error if rendering fails.
func (b *Banner) Generate(ctx context.Context) (string, error) {
	// Check for context cancellation early.
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Step 1: Open cache store. If it fails, continue with nil data.
	var claude *collectors.ClaudeUsage
	var billing *collectors.BillingData
	var infra *collectors.InfraStatus
	var fastfetch *collectors.FastfetchData

	store, err := cache.NewStore(b.config.CacheDir, b.config.Logger)
	if err != nil {
		b.config.Logger.Warn("banner: failed to open cache store", "error", err)
	} else {
		claude, billing, infra, fastfetch = b.loadCachedDataWithFastfetch(store)
	}

	// Fastfetch data will be integrated into buildSections()

	// Step 2: Evaluate system status.
	evaluator := status.NewEvaluator(status.DefaultEvaluatorConfig())
	systemStatus := evaluator.Evaluate(claude, billing, infra)

	// Step 3-4: Optionally fetch waifu image with responsive sizing.
	var imageContent string
	if b.config.WaifuEnabled {
		// Determine layout mode and waifu size based on terminal width.
		layoutMode := DetermineLayoutMode(b.config.TermWidth)
		waifuSize := GetWaifuSize(layoutMode)

		selectorCfg := status.DefaultSelectorConfig()
		if b.config.WaifuCategory != "" {
			selectorCfg.OverrideCategory = b.config.WaifuCategory
		}
		selector := status.NewSelector(selectorCfg)
		category := selector.SelectCategory(systemStatus.Overall)

		imageContent = b.fetchWaifuImage(ctx, category, waifuSize.Cols, waifuSize.Rows)
	}

	// Step 5: Determine hostname.
	hostname := b.config.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
		if hostname == "" {
			hostname = "unknown"
		}
	}

	// Step 6: Compute uptime string.
	uptime := computeUptime()

	// Step 7: Build responsive layout configuration.
	width := b.config.TermWidth
	height := b.config.TermHeight
	if width == 0 || height == 0 {
		width, height = layout.DetectTerminalSize()
	}

	responsiveCfg := layout.NewResponsiveConfig(width, height)
	responsiveCfg.ColorEnabled = true

	// Step 8: Build sections from collector data.
	sections := b.buildSections(claude, billing, infra, fastfetch, hostname, systemStatus.Overall.String(), uptime, responsiveCfg.Features)

	// Step 9: Render using responsive layout.
	responsiveLayout := layout.NewResponsiveLayout(responsiveCfg)
	result := responsiveLayout.Render(imageContent, sections, billing)

	return result.Output, nil
}

// loadCachedData reads collector data from the cache store.
// Returns nil pointers for any data that cannot be loaded.
func (b *Banner) loadCachedData(store *cache.Store) (claude *collectors.ClaudeUsage, billing *collectors.BillingData, infra *collectors.InfraStatus) {
	ttl := b.config.CacheTTL

	var err error
	claude, _, err = cache.GetTyped[collectors.ClaudeUsage](store, "claude", ttl)
	if err != nil {
		b.config.Logger.Warn("banner: failed to load claude cache", "error", err)
		claude = nil
	}

	billing, _, err = cache.GetTyped[collectors.BillingData](store, "billing", ttl)
	if err != nil {
		b.config.Logger.Warn("banner: failed to load billing cache", "error", err)
		billing = nil
	}

	infra, _, err = cache.GetTyped[collectors.InfraStatus](store, "infra", ttl)
	if err != nil {
		b.config.Logger.Warn("banner: failed to load infra cache", "error", err)
		infra = nil
	}

	return claude, billing, infra
}

// loadCachedDataWithFastfetch reads collector data including fastfetch from the cache store.
// Returns nil pointers for any data that cannot be loaded.
func (b *Banner) loadCachedDataWithFastfetch(store *cache.Store) (claude *collectors.ClaudeUsage, billing *collectors.BillingData, infra *collectors.InfraStatus, fastfetch *collectors.FastfetchData) {
	// Load standard collectors.
	claude, billing, infra = b.loadCachedData(store)

	// Load fastfetch data if enabled.
	if b.config.FastfetchEnabled {
		var err error
		fastfetch, _, err = cache.GetTyped[collectors.FastfetchData](store, "fastfetch", b.config.CacheTTL)
		if err != nil {
			b.config.Logger.Warn("banner: failed to load fastfetch cache", "error", err)
			fastfetch = nil
		}
	}

	return claude, billing, infra, fastfetch
}

// fetchWaifuImage retrieves a waifu image for the given category with the specified size.
// If WaifuSessionID is explicitly set (via --session-id flag or PPULSE_SESSION_ID env var),
// uses per-session caching where each shell session gets its own unique image
// with LRU eviction when MaxSessions is exceeded. This enables fetching new images
// from the API if none is cached for the session.
// Otherwise falls back to category-based caching (all sessions share same image),
// which only reads from existing cache without network calls.
// The maxCols and maxRows parameters control the rendered image dimensions.
// Returns the rendered image string, or empty string on any error (non-fatal).
func (b *Banner) fetchWaifuImage(ctx context.Context, category string, maxCols, maxRows int) string {
	select {
	case <-ctx.Done():
		return ""
	default:
	}

	var data []byte
	var sessionErr error

	// Only use session-based caching if explicitly configured (via flag) or
	// if PPULSE_SESSION_ID environment variable is set. This avoids making
	// network calls when the user hasn't opted into session-based waifu.
	sessionID := b.config.WaifuSessionID
	if sessionID == "" {
		// Only check environment, don't auto-generate from PID.
		// This ensures we don't make network calls without explicit opt-in.
		if envID := os.Getenv("PPULSE_SESSION_ID"); envID != "" {
			sessionID = envID
		}
	}

	// Use session-aware caching if we have an explicit session ID.
	if sessionID != "" {
		sessionMgr, err := waifu.NewSessionManager(waifu.SessionManagerConfig{
			CacheDir:        b.config.WaifuCacheDir,
			MaxSessions:     b.config.WaifuMaxSessions,
			ImageCacheTTL:   b.config.WaifuCacheTTL,
			ImageMaxCacheMB: b.config.WaifuMaxCacheMB,
			Logger:          b.config.Logger,
		})
		if err != nil {
			b.config.Logger.Warn("banner: failed to create session manager", "error", err)
			// Fall back to category-based caching below.
		} else {
			// Create API client for potential fetch.
			api := waifu.NewAPIClient(b.config.Logger)
			processCfg := waifu.DefaultProcessConfig()

			data, sessionErr = sessionMgr.GetOrFetch(ctx, sessionID, category, api, processCfg)
			if sessionErr != nil {
				b.config.Logger.Warn("banner: session fetch error", "error", sessionErr, "session", sessionID)
				// Fall back to category-based caching below.
				data = nil
			}
		}
	}

	// Fall back to category-based caching (original behavior, read-only).
	if data == nil {
		imgCache, err := waifu.NewImageCache(waifu.ImageCacheConfig{
			Dir:       b.config.WaifuCacheDir,
			TTL:       b.config.WaifuCacheTTL,
			MaxSizeMB: b.config.WaifuMaxCacheMB,
			Logger:    b.config.Logger,
		})
		if err != nil {
			b.config.Logger.Warn("banner: failed to create image cache", "error", err)
			return ""
		}

		cacheKey := "banner-" + category
		data, fresh, err := imgCache.Get(cacheKey)
		if err != nil {
			b.config.Logger.Warn("banner: image cache read error", "error", err, "key", cacheKey)
			return ""
		}
		if data == nil || !fresh {
			b.config.Logger.Debug("banner: no cached image for category", "category", category, "key", cacheKey)
			return ""
		}
	}

	rendered, err := waifu.RenderImage(data, waifu.RenderConfig{
		Protocol: waifu.DetectProtocol(),
		MaxCols:  maxCols,
		MaxRows:  maxRows,
	})
	if err != nil {
		b.config.Logger.Warn("banner: image render error", "error", err)
		return ""
	}

	return rendered
}

// computeUptime returns a human-readable system uptime string.
// Returns "unknown" if the uptime cannot be determined.
func computeUptime() string {
	d := getSystemUptime()
	if d == 0 {
		return "unknown"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	if days > 0 {
		return formatDuration(days, hours, mins, true)
	}
	if hours > 0 {
		return formatDuration(days, hours, mins, false)
	}
	return formatMinutes(mins)
}

// parseUptimeSeconds parses the seconds value from /proc/uptime content.
func parseUptimeSeconds(data []byte) (float64, error) {
	var uptime, idle float64
	_, err := parseFloatPair(string(data), &uptime, &idle)
	if err != nil {
		return 0, err
	}
	return uptime, nil
}

// parseFloatPair reads two space-separated floats from a string.
func parseFloatPair(s string, a, b *float64) (int, error) {
	var n int
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\n' {
			n = i
			break
		}
	}
	if n == 0 {
		n = len(s)
	}

	val, err := parseFloat(s[:n])
	if err != nil {
		return 0, err
	}
	*a = val
	return 1, nil
}

// parseFloat is a simple float parser for uptime values.
func parseFloat(s string) (float64, error) {
	var result float64
	var frac float64
	var fracDiv float64 = 1
	inFrac := false
	for _, ch := range s {
		if ch == '.' {
			inFrac = true
			continue
		}
		if ch < '0' || ch > '9' {
			continue
		}
		if inFrac {
			fracDiv *= 10
			frac += float64(ch-'0') / fracDiv
		} else {
			result = result*10 + float64(ch-'0')
		}
	}
	return result + frac, nil
}

// formatDuration formats days, hours, minutes into a human-readable string.
func formatDuration(days, hours, mins int, showDays bool) string {
	if showDays {
		return intToStr(days) + "d " + intToStr(hours) + "h " + intToStr(mins) + "m"
	}
	return intToStr(hours) + "h " + intToStr(mins) + "m"
}

// formatMinutes formats minutes-only uptime.
func formatMinutes(mins int) string {
	return intToStr(mins) + "m"
}

// intToStr converts a non-negative integer to a string without importing strconv.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// Reverse.
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

// GenerateResponsive produces the banner using the responsive layout system.
// It auto-detects terminal size and selects the appropriate layout mode:
//   - Compact (80x24): Vertical stack, no images
//   - Standard (120x40): Side-by-side with 22-column image
//   - Wide (160x60): 3-column with full metrics
//   - UltraWide (200x80): 4-column with sparklines
//
// Pass 0, 0 for width/height to auto-detect terminal size.
func (b *Banner) GenerateResponsive(ctx context.Context) (string, error) {
	// Check for context cancellation early.
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Step 1: Open cache store. If it fails, continue with nil data.
	var claude *collectors.ClaudeUsage
	var billing *collectors.BillingData
	var infra *collectors.InfraStatus

	store, err := cache.NewStore(b.config.CacheDir, b.config.Logger)
	if err != nil {
		b.config.Logger.Warn("banner: failed to open cache store", "error", err)
	} else {
		claude, billing, infra = b.loadCachedData(store)
	}

	// Step 2: Evaluate system status.
	evaluator := status.NewEvaluator(status.DefaultEvaluatorConfig())
	systemStatus := evaluator.Evaluate(claude, billing, infra)

	// Step 3: Optionally fetch waifu image with responsive sizing.
	var imageContent string
	if b.config.WaifuEnabled {
		// Determine layout mode and waifu size based on terminal width.
		layoutMode := DetermineLayoutMode(b.config.TermWidth)
		waifuSize := GetWaifuSize(layoutMode)

		selectorCfg := status.DefaultSelectorConfig()
		if b.config.WaifuCategory != "" {
			selectorCfg.OverrideCategory = b.config.WaifuCategory
		}
		selector := status.NewSelector(selectorCfg)
		category := selector.SelectCategory(systemStatus.Overall)
		imageContent = b.fetchWaifuImage(ctx, category, waifuSize.Cols, waifuSize.Rows)
	}

	// Step 4: Determine hostname.
	hostname := b.config.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
		if hostname == "" {
			hostname = "unknown"
		}
	}

	// Step 5: Compute uptime string.
	uptime := computeUptime()

	// Step 6: Build responsive layout configuration.
	width := b.config.TermWidth
	height := b.config.TermHeight
	if width == 0 || height == 0 {
		width, height = layout.DetectTerminalSize()
	}

	responsiveCfg := layout.NewResponsiveConfig(width, height)
	responsiveCfg.ColorEnabled = true

	// Only show image if enabled in the layout mode.
	if !responsiveCfg.Features.ShowImage {
		imageContent = ""
	}

	// Step 7: Build sections from data (no fastfetch in this code path).
	sections := b.buildSections(claude, billing, infra, nil, hostname, systemStatus.Overall.String(), uptime, responsiveCfg.Features)

	// Step 8: Render using responsive layout (no billing data in this code path).
	responsiveLayout := layout.NewResponsiveLayout(responsiveCfg)
	result := responsiveLayout.Render(imageContent, sections, nil)

	return result.Output, nil
}

// buildSections converts collector data into layout sections for the responsive layout.
func (b *Banner) buildSections(
	claude *collectors.ClaudeUsage,
	billing *collectors.BillingData,
	infra *collectors.InfraStatus,
	fastfetch *collectors.FastfetchData,
	hostname, statusLevel, uptime string,
	features layout.LayoutFeatures,
) []layout.Section {
	var sections []layout.Section

	// Header section with status.
	headerContent := []string{
		hostname + " :: " + statusLevel,
	}
	if uptime != "" && uptime != "unknown" {
		headerContent = append(headerContent, "uptime: "+uptime)
	}
	sections = append(sections, layout.Section{
		Title:   "Status",
		Content: headerContent,
	})

	// Claude section.
	claudeContent := b.formatClaudeForSection(claude, features.ShowFullMetrics)
	sections = append(sections, layout.Section{
		Title:   "Claude",
		Content: claudeContent,
	})

	// Billing section.
	billingContent := b.formatBillingForSection(billing)
	sections = append(sections, layout.Section{
		Title:   "Billing",
		Content: billingContent,
	})

	// Infrastructure section.
	infraContent := b.formatInfraForSection(infra, features.ShowNodeMetrics)
	sections = append(sections, layout.Section{
		Title:   "Infrastructure",
		Content: infraContent,
	})

	// System section (fastfetch).
	if fastfetch != nil && !fastfetch.IsEmpty() {
		fastfetchContent := b.formatFastfetchForSection(fastfetch)
		sections = append(sections, layout.Section{
			Title:   "System",
			Content: fastfetchContent,
		})
	}

	return sections
}

// formatClaudeForSection formats Claude usage data for a layout section.
func (b *Banner) formatClaudeForSection(data *collectors.ClaudeUsage, showFull bool) []string {
	if data == nil || len(data.Accounts) == 0 {
		return []string{"(no data)"}
	}

	var lines []string
	for _, acct := range data.Accounts {
		if acct.Status != "ok" {
			lines = append(lines, acct.Name+": ERR")
			continue
		}

		switch acct.Type {
		case "subscription":
			var parts []string
			if acct.FiveHour != nil {
				part := intToStr(int(acct.FiveHour.Utilization)) + "% (5h)"
				parts = append(parts, part)
			}
			if acct.SevenDay != nil && showFull {
				part := intToStr(int(acct.SevenDay.Utilization)) + "% (7d)"
				parts = append(parts, part)
			}
			if len(parts) > 0 {
				lines = append(lines, acct.Name+": "+join(parts, " | "))
			} else {
				lines = append(lines, acct.Name+": 0% (5h)")
			}
		case "api":
			if acct.RateLimits != nil {
				used := acct.RateLimits.RequestsLimit - acct.RateLimits.RequestsRemaining
				lines = append(lines, acct.Name+": "+intToStr(int(used))+"/"+intToStr(int(acct.RateLimits.RequestsLimit))+" req")
			} else {
				lines = append(lines, acct.Name+": 0/0 req")
			}
		default:
			lines = append(lines, acct.Name+": ERR")
		}
	}

	return lines
}

// formatBillingForSection formats billing data for a layout section.
func (b *Banner) formatBillingForSection(data *collectors.BillingData) []string {
	if data == nil {
		return []string{"(no data)"}
	}

	line := "$" + intToStr(int(data.Total.CurrentMonthUSD)) + " this month"

	if data.Total.ForecastUSD != nil {
		line += " ($" + intToStr(int(*data.Total.ForecastUSD)) + " forecast)"
	}

	if data.Total.BudgetUSD != nil && data.Total.CurrentMonthUSD > *data.Total.BudgetUSD {
		line += " OVER BUDGET"
	}

	return []string{line}
}

// formatInfraForSection formats infrastructure data for a layout section.
func (b *Banner) formatInfraForSection(data *collectors.InfraStatus, showNodeMetrics bool) []string {
	if data == nil {
		return []string{"(no data)"}
	}

	var lines []string

	if data.Tailscale != nil {
		lines = append(lines, "ts: "+intToStr(data.Tailscale.OnlineCount)+"/"+intToStr(data.Tailscale.TotalCount)+" online")

		// Show per-node metrics if enabled.
		if showNodeMetrics {
			for _, node := range data.Tailscale.Nodes {
				if !node.Online {
					continue
				}
				if node.CPUPercent == nil && node.RAMPercent == nil && node.DiskPercent == nil {
					continue
				}

				var metrics []string
				if node.CPUPercent != nil {
					gauge := widgets.RenderMiniGauge(*node.CPUPercent, 6)
					metrics = append(metrics, "CPU "+gauge+" "+intToStr(int(*node.CPUPercent))+"%")
				}
				if node.RAMPercent != nil {
					gauge := widgets.RenderMiniGauge(*node.RAMPercent, 6)
					metrics = append(metrics, "RAM "+gauge+" "+intToStr(int(*node.RAMPercent))+"%")
				}
				if node.DiskPercent != nil {
					gauge := widgets.RenderMiniGauge(*node.DiskPercent, 6)
					metrics = append(metrics, "Disk "+gauge+" "+intToStr(int(*node.DiskPercent))+"%")
				}

				if len(metrics) > 0 {
					lines = append(lines, "  "+node.Hostname+": "+join(metrics, " | "))
				}
			}
		}
	}

	for _, cluster := range data.Kubernetes {
		lines = append(lines, "k8s: "+cluster.Name+" ("+cluster.Status+")")
	}

	if len(lines) == 0 {
		return []string{"(no data)"}
	}

	return lines
}

// formatFastfetchForSection formats fastfetch system info for a layout section.
func (b *Banner) formatFastfetchForSection(data *collectors.FastfetchData) []string {
	if data == nil || data.IsEmpty() {
		return []string{"(no data)"}
	}
	return data.FormatCompact()
}

// join concatenates strings with a separator.
func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// GetLayoutMode returns the layout mode that would be used for the current configuration.
// Pass 0, 0 to auto-detect terminal size.
func (b *Banner) GetLayoutMode(width, height int) layout.LayoutMode {
	if width == 0 || height == 0 {
		width, height = layout.DetectTerminalSize()
	}
	return layout.DetectLayoutMode(width, height)
}
