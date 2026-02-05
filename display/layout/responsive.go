// Package layout provides a responsive terminal layout system for prompt-pulse banners.
//
// The layout engine supports 4 terminal size targets:
//   - Compact (80x24): Vertical stack, no images, minimal SSH-friendly display
//   - Standard (120x40): Side-by-side layout with 22-column image, default for laptops
//   - Wide (160x60): 3-column layout with full metrics, for ultra-wide monitors
//   - UltraWide (200x80): 4-column layout with sparklines, for large displays
//
// The engine auto-detects terminal size via term.GetSize() and gracefully degrades
// for smaller terminals.
package layout

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"gitlab.com/tinyland/lab/prompt-pulse/collectors"
	"gitlab.com/tinyland/lab/prompt-pulse/display/widgets"
)

// LayoutMode represents one of the 4 supported terminal layout modes.
type LayoutMode int

const (
	// LayoutCompact is for minimal terminals (80x24) - vertical stack, no images.
	LayoutCompact LayoutMode = iota
	// LayoutStandard is the default (120x40) - side-by-side with 22-col image.
	LayoutStandard
	// LayoutWide is for ultra-wide monitors (160x60) - 3-column, full metrics.
	LayoutWide
	// LayoutUltraWide is for large displays (200x80) - 4-column, sparklines.
	LayoutUltraWide
)

// String returns the human-readable name of the layout mode.
func (m LayoutMode) String() string {
	switch m {
	case LayoutCompact:
		return "compact"
	case LayoutStandard:
		return "standard"
	case LayoutWide:
		return "wide"
	case LayoutUltraWide:
		return "ultra-wide"
	default:
		return "unknown"
	}
}

// LayoutTarget defines the target dimensions for a layout mode.
type LayoutTarget struct {
	MinWidth  int
	MinHeight int
	Mode      LayoutMode
}

// layoutTargets defines the thresholds for each layout mode (ordered from largest to smallest).
var layoutTargets = []LayoutTarget{
	{MinWidth: 200, MinHeight: 80, Mode: LayoutUltraWide},
	{MinWidth: 160, MinHeight: 60, Mode: LayoutWide},
	{MinWidth: 120, MinHeight: 40, Mode: LayoutStandard},
	{MinWidth: 80, MinHeight: 24, Mode: LayoutCompact},
}

// ColumnConfig defines the column widths for a layout mode.
type ColumnConfig struct {
	// ImageCols is the width allocated to the image column (0 if no image).
	ImageCols int
	// MainCols is the width allocated to the main content column.
	MainCols int
	// InfoCols is the width allocated to the info/status column.
	InfoCols int
	// MetricsCols is the width for metrics/sparklines (Wide/UltraWide only).
	MetricsCols int
	// SparklineCols is the width for sparkline charts (UltraWide only).
	SparklineCols int
}

// LayoutFeatures defines what features are enabled for a layout mode.
type LayoutFeatures struct {
	// ShowImage enables the waifu/banner image.
	ShowImage bool
	// ShowSparklines enables sparkline mini-charts.
	ShowSparklines bool
	// ShowFullMetrics enables detailed metrics display.
	ShowFullMetrics bool
	// ShowNodeMetrics enables per-node resource metrics.
	ShowNodeMetrics bool
	// VerticalStack uses vertical stacking instead of side-by-side.
	VerticalStack bool
	// ShowBorders enables Unicode box drawing borders.
	ShowBorders bool
}

// ResponsiveConfig holds the complete configuration for a responsive layout.
type ResponsiveConfig struct {
	// Mode is the detected or selected layout mode.
	Mode LayoutMode
	// TermWidth is the terminal width in columns.
	TermWidth int
	// TermHeight is the terminal height in rows.
	TermHeight int
	// Columns contains the column width configuration.
	Columns ColumnConfig
	// Features contains the feature flags for this layout.
	Features LayoutFeatures
	// ColorEnabled enables ANSI color output.
	ColorEnabled bool
}

// Color palette matching the TUI theme (display/tui/theme.go).
var (
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple - headers
	colorSecondary = lipgloss.Color("#06B6D4") // Cyan - section titles
	colorSuccess   = lipgloss.Color("#22C55E") // Green - healthy status
	colorWarning   = lipgloss.Color("#EAB308") // Yellow - warning status
	colorDanger    = lipgloss.Color("#EF4444") // Red - critical status
	colorMuted     = lipgloss.Color("#6B7280") // Gray - separators/borders
)

// Unicode box drawing characters.
const (
	boxTopLeft     = '╭'
	boxTopRight    = '╮'
	boxBottomLeft  = '╰'
	boxBottomRight = '╯'
	boxHorizontal  = '─'
	boxVertical    = '│'
)

// DetectTerminalSize returns the current terminal dimensions.
// It tries TTY detection first, then environment variables, then defaults.
func DetectTerminalSize() (width, height int) {
	// Try TTY detection first using stdout file descriptor.
	w, h, err := term.GetSize(os.Stdout.Fd())
	if err == nil && w > 0 && h > 0 {
		return w, h
	}

	// Try environment variables.
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if w, err := strconv.Atoi(cols); err == nil && w > 0 {
			width = w
		}
	}
	if lines := os.Getenv("LINES"); lines != "" {
		if h, err := strconv.Atoi(lines); err == nil && h > 0 {
			height = h
		}
	}

	// Defaults.
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}
	return width, height
}

// DetectLayoutMode determines the appropriate layout mode for the given dimensions.
// It returns the largest mode that fits within the terminal dimensions.
func DetectLayoutMode(width, height int) LayoutMode {
	for _, target := range layoutTargets {
		if width >= target.MinWidth && height >= target.MinHeight {
			return target.Mode
		}
	}
	// Even if smaller than compact threshold, default to compact.
	return LayoutCompact
}

// NewResponsiveConfig creates a ResponsiveConfig for the given terminal dimensions.
// Pass 0, 0 to auto-detect terminal size.
func NewResponsiveConfig(width, height int) ResponsiveConfig {
	if width == 0 || height == 0 {
		width, height = DetectTerminalSize()
	}

	mode := DetectLayoutMode(width, height)
	columns := columnsForMode(mode, width)
	features := featuresForMode(mode)

	return ResponsiveConfig{
		Mode:         mode,
		TermWidth:    width,
		TermHeight:   height,
		Columns:      columns,
		Features:     features,
		ColorEnabled: true,
	}
}

// columnsForMode returns the column configuration for a layout mode.
func columnsForMode(mode LayoutMode, termWidth int) ColumnConfig {
	switch mode {
	case LayoutUltraWide:
		// 4-column: image(24) | main(50) | info(50) | sparklines(remaining)
		imageCols := 24
		mainCols := 50
		infoCols := 50
		separators := 9 // 3 separators * 3 chars each (" | ")
		remaining := termWidth - imageCols - mainCols - infoCols - separators
		sparklineCols := remaining
		if sparklineCols < 20 {
			sparklineCols = 20
		}
		return ColumnConfig{
			ImageCols:     imageCols,
			MainCols:      mainCols,
			InfoCols:      infoCols,
			MetricsCols:   infoCols, // Metrics in info column
			SparklineCols: sparklineCols,
		}

	case LayoutWide:
		// 3-column: image(24) | main(60) | info(remaining)
		imageCols := 24
		mainCols := 60
		separators := 6 // 2 separators * 3 chars each
		remaining := termWidth - imageCols - mainCols - separators
		infoCols := remaining
		if infoCols < 40 {
			infoCols = 40
		}
		return ColumnConfig{
			ImageCols:   imageCols,
			MainCols:    mainCols,
			InfoCols:    infoCols,
			MetricsCols: infoCols,
		}

	case LayoutStandard:
		// 2-column: image(22) | main(remaining)
		imageCols := 22
		separators := 3 // 1 separator
		mainCols := termWidth - imageCols - separators
		if mainCols < 40 {
			mainCols = 40
		}
		return ColumnConfig{
			ImageCols: imageCols,
			MainCols:  mainCols,
		}

	default: // LayoutCompact
		// 1-column: no image, full width
		return ColumnConfig{
			MainCols: termWidth - 4, // Allow for box borders
		}
	}
}

// featuresForMode returns the feature configuration for a layout mode.
func featuresForMode(mode LayoutMode) LayoutFeatures {
	switch mode {
	case LayoutUltraWide:
		return LayoutFeatures{
			ShowImage:       true,
			ShowSparklines:  true,
			ShowFullMetrics: true,
			ShowNodeMetrics: true,
			VerticalStack:   false,
			ShowBorders:     true,
		}

	case LayoutWide:
		return LayoutFeatures{
			ShowImage:       true,
			ShowSparklines:  false,
			ShowFullMetrics: true,
			ShowNodeMetrics: true,
			VerticalStack:   false,
			ShowBorders:     true,
		}

	case LayoutStandard:
		return LayoutFeatures{
			ShowImage:       true,
			ShowSparklines:  false,
			ShowFullMetrics: false,
			ShowNodeMetrics: false,
			VerticalStack:   false,
			ShowBorders:     true,
		}

	default: // LayoutCompact
		return LayoutFeatures{
			ShowImage:       false,
			ShowSparklines:  false,
			ShowFullMetrics: false,
			ShowNodeMetrics: false,
			VerticalStack:   true,
			ShowBorders:     false, // No box borders in compact mode
		}
	}
}

// ResponsiveLayout renders content using the responsive layout system.
type ResponsiveLayout struct {
	config ResponsiveConfig
}

// NewResponsiveLayout creates a new ResponsiveLayout with the given configuration.
func NewResponsiveLayout(config ResponsiveConfig) *ResponsiveLayout {
	return &ResponsiveLayout{config: config}
}

// Config returns the current configuration.
func (l *ResponsiveLayout) Config() ResponsiveConfig {
	return l.config
}

// Section represents a content section for layout rendering.
type Section struct {
	Title   string
	Content []string
}

// RenderResult holds the rendered layout output.
type RenderResult struct {
	// Output is the final rendered string.
	Output string
	// Lines is the number of output lines.
	Lines int
	// Truncated indicates if content was truncated to fit terminal height.
	Truncated bool
}

// Render composes a complete layout from the given sections.
// imageContent is the pre-rendered image string (may be empty).
// sections are the content sections to display.
// billing is optional billing data for sparkline rendering.
func (l *ResponsiveLayout) Render(imageContent string, sections []Section, billing *collectors.BillingData) RenderResult {
	switch l.config.Mode {
	case LayoutUltraWide:
		return l.renderUltraWide(imageContent, sections, billing)
	case LayoutWide:
		return l.renderWide(imageContent, sections)
	case LayoutStandard:
		return l.renderStandard(imageContent, sections)
	default:
		return l.renderCompact(sections)
	}
}

// renderCompact renders content in compact (vertical stack) mode.
func (l *ResponsiveLayout) renderCompact(sections []Section) RenderResult {
	var lines []string

	for _, section := range sections {
		// Section header.
		titleLine := l.sectionTitle(section.Title)
		lines = append(lines, titleLine)

		// Section content.
		for _, line := range section.Content {
			// Truncate lines to fit width.
			if len(line) > l.config.Columns.MainCols {
				line = truncateToWidth(line, l.config.Columns.MainCols)
			}
			lines = append(lines, "  "+line)
		}
		lines = append(lines, "")
	}

	// Truncate to terminal height.
	truncated := false
	if len(lines) > l.config.TermHeight {
		lines = lines[:l.config.TermHeight]
		truncated = true
	}

	return RenderResult{
		Output:    strings.Join(lines, "\n"),
		Lines:     len(lines),
		Truncated: truncated,
	}
}

// renderStandard renders content in standard (2-column) mode.
func (l *ResponsiveLayout) renderStandard(imageContent string, sections []Section) RenderResult {
	// Build info panel.
	infoPanelLines := l.buildInfoPanel(sections)

	// If no image, render info only.
	if imageContent == "" || !l.config.Features.ShowImage {
		return l.wrapInBox(infoPanelLines)
	}

	// Compose side-by-side.
	return l.composeSideBySide(imageContent, infoPanelLines)
}

// renderWide renders content in wide (3-column) mode.
func (l *ResponsiveLayout) renderWide(imageContent string, sections []Section) RenderResult {
	// For wide mode, we split sections between main and info columns.
	mainSections := sections
	var infoSections []Section

	// If we have more than 2 sections, split them.
	if len(sections) > 2 {
		mainSections = sections[:2]
		infoSections = sections[2:]
	}

	// Build panels.
	mainLines := l.buildInfoPanel(mainSections)
	infoLines := l.buildInfoPanel(infoSections)

	// If no image, show main + info side by side.
	if imageContent == "" || !l.config.Features.ShowImage {
		return l.composeTwoColumns(mainLines, infoLines)
	}

	// 3-column: image | main | info.
	return l.composeThreeColumns(imageContent, mainLines, infoLines)
}

// renderUltraWide renders content in ultra-wide (4-column) mode.
func (l *ResponsiveLayout) renderUltraWide(imageContent string, sections []Section, billing *collectors.BillingData) RenderResult {
	// For ultra-wide, split sections into main and info.
	mainSections := sections
	var infoSections []Section
	if len(sections) > 2 {
		mainSections = sections[:2]
		infoSections = sections[2:]
	}

	mainLines := l.buildInfoPanel(mainSections)
	infoLines := l.buildInfoPanel(infoSections)

	// Generate sparklines from billing history.
	sparklineLines := l.buildActualSparklines(billing)

	// 4-column: image | main | info | sparklines.
	if imageContent == "" || !l.config.Features.ShowImage {
		return l.composeThreeColumns(strings.Join(mainLines, "\n"), infoLines, sparklineLines)
	}

	return l.composeFourColumns(imageContent, mainLines, infoLines, sparklineLines)
}

// buildInfoPanel builds the content lines for the info panel from sections.
func (l *ResponsiveLayout) buildInfoPanel(sections []Section) []string {
	var lines []string

	for i, section := range sections {
		if i > 0 {
			lines = append(lines, "")
		}

		// Section title.
		titleLine := l.sectionTitle(section.Title)
		lines = append(lines, titleLine)

		// Content lines.
		for _, line := range section.Content {
			lines = append(lines, "  "+line)
		}
	}

	return lines
}

// buildSparklinePlaceholder builds placeholder sparkline content.
// DEPRECATED: Use buildActualSparklines() instead.
func (l *ResponsiveLayout) buildSparklinePlaceholder() []string {
	// This is a placeholder; actual sparklines would be rendered by the caller.
	return []string{
		l.sectionTitle("Trends"),
		"  CPU: [sparkline]",
		"  MEM: [sparkline]",
		"  NET: [sparkline]",
	}
}

// buildActualSparklines builds sparkline content from billing history data.
func (l *ResponsiveLayout) buildActualSparklines(billing *collectors.BillingData) []string {
	if billing == nil || billing.History == nil {
		return []string{l.sectionTitle("Trends"), "  (no data)"}
	}

	lines := []string{l.sectionTitle("Trends")}

	// Total spend sparkline (30-day history).
	if len(billing.History.TotalHistory) > 0 {
		values := collectors.GetSpendValues(billing.History.TotalHistory)
		if len(values) > 0 {
			sparkline := widgets.RenderSparkline(widgets.SparklineConfig{
				Data:  values,
				Width: 20,
				Label: "Total",
			})
			lines = append(lines, "  "+sparkline)
		}
	}

	// Per-provider sparklines (show top 2 providers by spend).
	if len(billing.History.ProviderHistory) > 0 {
		// Find top 2 providers by most recent spend.
		type providerSpend struct {
			name  string
			spend float64
		}
		var providers []providerSpend
		for name, history := range billing.History.ProviderHistory {
			if len(history) > 0 {
				providers = append(providers, providerSpend{
					name:  name,
					spend: history[len(history)-1].SpendUSD,
				})
			}
		}

		// Sort by spend (simple bubble sort for top 2).
		for i := 0; i < len(providers); i++ {
			for j := i + 1; j < len(providers); j++ {
				if providers[j].spend > providers[i].spend {
					providers[i], providers[j] = providers[j], providers[i]
				}
			}
		}

		// Show top 2 providers.
		for i, p := range providers {
			if i >= 2 {
				break
			}
			history := billing.History.ProviderHistory[p.name]
			values := collectors.GetSpendValues(history)
			if len(values) > 0 {
				sparkline := widgets.RenderSparkline(widgets.SparklineConfig{
					Data:  values,
					Width: 20,
					Label: p.name,
				})
				lines = append(lines, "  "+sparkline)
			}
		}
	}

	if len(lines) == 1 {
		lines = append(lines, "  (no history)")
	}

	return lines
}

// composeSideBySide places image and info side-by-side.
func (l *ResponsiveLayout) composeSideBySide(imageContent string, infoLines []string) RenderResult {
	imageLines := strings.Split(imageContent, "\n")

	maxRows := max(len(imageLines), len(infoLines))
	if maxRows > l.config.TermHeight {
		maxRows = l.config.TermHeight
	}

	separator := l.columnSeparator()
	var result []string

	for i := 0; i < maxRows; i++ {
		imgLine := ""
		if i < len(imageLines) {
			imgLine = imageLines[i]
		}
		infoLine := ""
		if i < len(infoLines) {
			infoLine = infoLines[i]
		}

		// Pad image line to column width.
		imgLine = padToWidth(imgLine, l.config.Columns.ImageCols)

		result = append(result, imgLine+separator+infoLine)
	}

	truncated := len(imageLines) > l.config.TermHeight || len(infoLines) > l.config.TermHeight

	return RenderResult{
		Output:    strings.Join(result, "\n"),
		Lines:     len(result),
		Truncated: truncated,
	}
}

// composeTwoColumns places two content columns side-by-side.
func (l *ResponsiveLayout) composeTwoColumns(leftLines, rightLines []string) RenderResult {
	maxRows := max(len(leftLines), len(rightLines))
	if maxRows > l.config.TermHeight {
		maxRows = l.config.TermHeight
	}

	separator := l.columnSeparator()
	var result []string

	for i := 0; i < maxRows; i++ {
		leftLine := ""
		if i < len(leftLines) {
			leftLine = leftLines[i]
		}
		rightLine := ""
		if i < len(rightLines) {
			rightLine = rightLines[i]
		}

		leftLine = padToWidth(leftLine, l.config.Columns.MainCols)
		result = append(result, leftLine+separator+rightLine)
	}

	truncated := len(leftLines) > l.config.TermHeight || len(rightLines) > l.config.TermHeight

	return RenderResult{
		Output:    strings.Join(result, "\n"),
		Lines:     len(result),
		Truncated: truncated,
	}
}

// composeThreeColumns places image, main, and info in three columns.
func (l *ResponsiveLayout) composeThreeColumns(imageContent string, mainLines, infoLines []string) RenderResult {
	imageLines := strings.Split(imageContent, "\n")

	maxRows := max(len(imageLines), max(len(mainLines), len(infoLines)))
	if maxRows > l.config.TermHeight {
		maxRows = l.config.TermHeight
	}

	separator := l.columnSeparator()
	var result []string

	for i := 0; i < maxRows; i++ {
		imgLine := ""
		if i < len(imageLines) {
			imgLine = imageLines[i]
		}
		mainLine := ""
		if i < len(mainLines) {
			mainLine = mainLines[i]
		}
		infoLine := ""
		if i < len(infoLines) {
			infoLine = infoLines[i]
		}

		imgLine = padToWidth(imgLine, l.config.Columns.ImageCols)
		mainLine = padToWidth(mainLine, l.config.Columns.MainCols)

		result = append(result, imgLine+separator+mainLine+separator+infoLine)
	}

	truncated := maxRows < max(len(imageLines), max(len(mainLines), len(infoLines)))

	return RenderResult{
		Output:    strings.Join(result, "\n"),
		Lines:     len(result),
		Truncated: truncated,
	}
}

// composeFourColumns places image, main, info, and sparklines in four columns.
func (l *ResponsiveLayout) composeFourColumns(imageContent string, mainLines, infoLines, sparkLines []string) RenderResult {
	imageLines := strings.Split(imageContent, "\n")

	maxRows := max(len(imageLines), max(len(mainLines), max(len(infoLines), len(sparkLines))))
	if maxRows > l.config.TermHeight {
		maxRows = l.config.TermHeight
	}

	separator := l.columnSeparator()
	var result []string

	for i := 0; i < maxRows; i++ {
		imgLine := ""
		if i < len(imageLines) {
			imgLine = imageLines[i]
		}
		mainLine := ""
		if i < len(mainLines) {
			mainLine = mainLines[i]
		}
		infoLine := ""
		if i < len(infoLines) {
			infoLine = infoLines[i]
		}
		sparkLine := ""
		if i < len(sparkLines) {
			sparkLine = sparkLines[i]
		}

		imgLine = padToWidth(imgLine, l.config.Columns.ImageCols)
		mainLine = padToWidth(mainLine, l.config.Columns.MainCols)
		infoLine = padToWidth(infoLine, l.config.Columns.InfoCols)

		result = append(result, imgLine+separator+mainLine+separator+infoLine+separator+sparkLine)
	}

	truncated := maxRows < max(len(imageLines), max(len(mainLines), max(len(infoLines), len(sparkLines))))

	return RenderResult{
		Output:    strings.Join(result, "\n"),
		Lines:     len(result),
		Truncated: truncated,
	}
}

// wrapInBox wraps content in a Unicode box border.
func (l *ResponsiveLayout) wrapInBox(lines []string) RenderResult {
	if !l.config.Features.ShowBorders {
		// No borders, return as-is.
		output := strings.Join(lines, "\n")
		return RenderResult{
			Output:    output,
			Lines:     len(lines),
			Truncated: false,
		}
	}

	width := l.config.TermWidth - 2 // Account for side borders.
	var result []string

	// Top border.
	topBorder := string(boxTopLeft) + strings.Repeat(string(boxHorizontal), width) + string(boxTopRight)
	if l.config.ColorEnabled {
		topBorder = lipgloss.NewStyle().Foreground(colorMuted).Render(topBorder)
	}
	result = append(result, topBorder)

	// Content lines.
	for _, line := range lines {
		paddedLine := padToWidth(line, width)
		vertBar := string(boxVertical)
		if l.config.ColorEnabled {
			vertBar = lipgloss.NewStyle().Foreground(colorMuted).Render(vertBar)
		}
		result = append(result, vertBar+paddedLine+vertBar)
	}

	// Bottom border.
	bottomBorder := string(boxBottomLeft) + strings.Repeat(string(boxHorizontal), width) + string(boxBottomRight)
	if l.config.ColorEnabled {
		bottomBorder = lipgloss.NewStyle().Foreground(colorMuted).Render(bottomBorder)
	}
	result = append(result, bottomBorder)

	// Truncate to terminal height.
	truncated := false
	if len(result) > l.config.TermHeight {
		result = result[:l.config.TermHeight]
		truncated = true
	}

	return RenderResult{
		Output:    strings.Join(result, "\n"),
		Lines:     len(result),
		Truncated: truncated,
	}
}

// sectionTitle renders a styled section title.
func (l *ResponsiveLayout) sectionTitle(title string) string {
	if !l.config.ColorEnabled {
		return title
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(colorSecondary).
		Render(title)
}

// columnSeparator returns the separator string between columns.
func (l *ResponsiveLayout) columnSeparator() string {
	sep := " " + string(boxVertical) + " "
	if l.config.ColorEnabled {
		return " " + lipgloss.NewStyle().Foreground(colorMuted).Render(string(boxVertical)) + " "
	}
	return sep
}

// RenderBox creates a Unicode box around content with an optional title.
func (l *ResponsiveLayout) RenderBox(lines []string, width int, title string) string {
	if width < 4 {
		width = l.config.TermWidth
	}
	innerWidth := width - 2

	var result strings.Builder

	// Top border with optional title.
	result.WriteRune(boxTopLeft)
	if title != "" {
		titleStyled := title
		if l.config.ColorEnabled {
			titleStyled = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render(title)
		}
		titleLen := len(title) + 2 // Space padding.
		result.WriteString(strings.Repeat(string(boxHorizontal), 1))
		result.WriteString(" ")
		result.WriteString(titleStyled)
		result.WriteString(" ")
		remaining := innerWidth - titleLen - 2
		if remaining > 0 {
			result.WriteString(strings.Repeat(string(boxHorizontal), remaining))
		}
	} else {
		result.WriteString(strings.Repeat(string(boxHorizontal), innerWidth))
	}
	result.WriteRune(boxTopRight)
	result.WriteString("\n")

	// Content lines.
	for _, line := range lines {
		result.WriteRune(boxVertical)
		result.WriteString(" ")
		paddedLine := padOrTruncate(line, innerWidth-2)
		result.WriteString(paddedLine)
		result.WriteString(" ")
		result.WriteRune(boxVertical)
		result.WriteString("\n")
	}

	// Bottom border.
	result.WriteRune(boxBottomLeft)
	result.WriteString(strings.Repeat(string(boxHorizontal), innerWidth))
	result.WriteRune(boxBottomRight)

	return result.String()
}

// StatusIndicator renders a color-coded status indicator.
func (l *ResponsiveLayout) StatusIndicator(status string) string {
	icons := map[string]string{
		"healthy":  "●",
		"warning":  "●",
		"critical": "●",
		"unknown":  "○",
	}

	icon, ok := icons[status]
	if !ok {
		icon = "○"
	}

	if !l.config.ColorEnabled {
		return icon + " " + status
	}

	color := l.statusColor(status)
	styledIcon := lipgloss.NewStyle().Foreground(color).Render(icon)
	styledStatus := lipgloss.NewStyle().Foreground(color).Bold(true).Render(status)

	return styledIcon + " " + styledStatus
}

// statusColor returns the appropriate color for a status level.
func (l *ResponsiveLayout) statusColor(status string) lipgloss.Color {
	switch status {
	case "healthy":
		return colorSuccess
	case "warning":
		return colorWarning
	case "critical":
		return colorDanger
	default:
		return colorMuted
	}
}

// Helper functions.

// padToWidth pads or truncates a string to exactly the given width.
func padToWidth(s string, width int) string {
	visible := visibleLen(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// padOrTruncate pads or truncates a string to exactly the given width.
func padOrTruncate(s string, width int) string {
	visible := visibleLen(s)
	if visible >= width {
		return truncateToWidth(s, width)
	}
	return s + strings.Repeat(" ", width-visible)
}

// visibleLen returns the visible length of a string, stripping ANSI escape sequences.
func visibleLen(s string) int {
	length := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '~' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			continue
		}
		length++
	}
	return length
}

// truncateToWidth truncates a string to at most width visible characters.
// It preserves ANSI escape sequences but counts only visible characters.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}

	var result strings.Builder
	visibleCount := 0
	inEscape := false

	for _, r := range s {
		if inEscape {
			result.WriteRune(r)
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '~' {
				inEscape = false
			}
			continue
		}
		if r == '\x1b' {
			inEscape = true
			result.WriteRune(r)
			continue
		}
		if visibleCount >= width {
			break
		}
		result.WriteRune(r)
		visibleCount++
	}

	return result.String()
}

// max returns the larger of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
