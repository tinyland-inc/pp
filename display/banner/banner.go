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
		CacheDir:        cacheBase,
		CacheTTL:        15 * time.Minute,
		WaifuEnabled:    false,
		WaifuCacheDir:   cacheBase + "/waifu",
		WaifuCacheTTL:   24 * time.Hour,
		WaifuMaxCacheMB: 50,
		TermWidth:       80,
		TermHeight:      24,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
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

	store, err := cache.NewStore(b.config.CacheDir, b.config.Logger)
	if err != nil {
		b.config.Logger.Warn("banner: failed to open cache store", "error", err)
	} else {
		claude, billing, infra = b.loadCachedData(store)
	}

	// Step 2: Evaluate system status.
	evaluator := status.NewEvaluator(status.DefaultEvaluatorConfig())
	systemStatus := evaluator.Evaluate(claude, billing, infra)

	// Step 3-4: Optionally fetch waifu image.
	var imageContent string
	if b.config.WaifuEnabled {
		selectorCfg := status.DefaultSelectorConfig()
		if b.config.WaifuCategory != "" {
			selectorCfg.OverrideCategory = b.config.WaifuCategory
		}
		selector := status.NewSelector(selectorCfg)
		category := selector.SelectCategory(systemStatus.Overall)

		imageContent = b.fetchWaifuImage(ctx, category)
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

	// Step 7: Render the layout.
	layoutCfg := DefaultLayoutConfig()
	layoutCfg.TermWidth = b.config.TermWidth
	layoutCfg.TermHeight = b.config.TermHeight
	layoutCfg.ShowImage = imageContent != ""
	layoutCfg.Hostname = hostname
	layoutCfg.ColorEnabled = true

	layout := NewLayout(layoutCfg)
	data := InfoData{
		Claude:      claude,
		Billing:     billing,
		Infra:       infra,
		StatusLevel: systemStatus.Overall.String(),
		Uptime:      uptime,
	}

	return layout.Render(imageContent, data), nil
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

// fetchWaifuImage retrieves a cached waifu image for the given category.
// Returns the rendered image string, or empty string on any error (non-fatal).
// It only reads from the cache -- if no cached image exists, it returns empty
// because fetching from the network would be too slow for the <100ms target.
// The daemon is responsible for pre-fetching images.
func (b *Banner) fetchWaifuImage(ctx context.Context, category string) string {
	select {
	case <-ctx.Done():
		return ""
	default:
	}

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

	rendered, err := waifu.RenderImage(data, waifu.RenderConfig{
		Protocol: waifu.DetectProtocol(),
		MaxCols:  20,
		MaxRows:  10,
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
	// Read /proc/uptime on Linux; return "unknown" on other platforms.
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "unknown"
	}

	var seconds float64
	n, err := parseUptimeSeconds(data)
	if err != nil {
		return "unknown"
	}
	seconds = n

	d := time.Duration(seconds) * time.Second
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
