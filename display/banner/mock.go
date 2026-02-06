// Package banner provides mock banner generation for testing.
package banner

import (
	"context"
	"os"

	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/layout"
	"gitlab.com/tinyland/lab/prompt-pulse/status"
)

// MockBanner is a Banner that uses injected mock data instead of reading from cache.
type MockBanner struct {
	config  BannerConfig
	claude  *collectors.ClaudeUsage
	billing *collectors.BillingData
	infra   *collectors.InfraStatus
}

// NewMockBanner creates a MockBanner with injected data.
func NewMockBanner(cfg BannerConfig, claude *collectors.ClaudeUsage, billing *collectors.BillingData, infra *collectors.InfraStatus) *MockBanner {
	return &MockBanner{
		config:  cfg,
		claude:  claude,
		billing: billing,
		infra:   infra,
	}
}

// Generate produces the banner string using injected mock data.
func (m *MockBanner) Generate(ctx context.Context) (string, error) {
	// Check for context cancellation early.
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Step 1: Use injected data instead of loading from cache.
	claude := m.claude
	billing := m.billing
	infra := m.infra

	// Step 2: Evaluate system status.
	evaluator := status.NewEvaluator(status.DefaultEvaluatorConfig())
	systemStatus := evaluator.Evaluate(claude, billing, infra)

	// Step 3-4: Optionally fetch waifu image (skip for mocks - don't want network calls).
	var imageContent string
	// For mock mode, skip waifu fetching to avoid network calls

	// Step 5: Determine hostname.
	hostname := m.config.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
		if hostname == "" {
			hostname = "mock-host"
		}
	}

	// Step 6: Compute uptime string.
	uptime := computeUptime()

	// Step 7: Build responsive layout configuration.
	width := m.config.TermWidth
	height := m.config.TermHeight
	if width == 0 || height == 0 {
		width, height = layout.DetectTerminalSize()
	}

	responsiveCfg := layout.NewResponsiveConfig(width, height)
	responsiveCfg.ColorEnabled = m.config.ColorEnabled

	// Step 8: Build sections from mock data (no fastfetch or sysmetrics in mock).
	b := &Banner{config: m.config}
	sections := b.buildSections(claude, billing, infra, nil, nil, hostname, systemStatus.Overall.String(), uptime, responsiveCfg.Features)

	// Step 9: Render using responsive layout.
	responsiveLayout := layout.NewResponsiveLayout(responsiveCfg)
	result := responsiveLayout.Render(imageContent, sections, billing)

	return result.Output, nil
}

// GenerateWithData is an alternative signature that accepts data directly.
// This is useful for testing different data scenarios.
func GenerateWithData(cfg BannerConfig, claude *collectors.ClaudeUsage, billing *collectors.BillingData, infra *collectors.InfraStatus) (string, error) {
	m := NewMockBanner(cfg, claude, billing, infra)
	return m.Generate(context.Background())
}
