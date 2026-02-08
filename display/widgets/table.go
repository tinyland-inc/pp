package widgets

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Alignment controls text alignment within a table column.
type Alignment int

const (
	// AlignLeft aligns text to the left (default).
	AlignLeft Alignment = iota
	// AlignRight aligns text to the right.
	AlignRight
	// AlignCenter centers text within the column.
	AlignCenter
)

// Column defines a single table column.
type Column struct {
	// Title is the header text.
	Title string
	// Width is the fixed character width. If 0, auto-calculated from content.
	Width int
	// Align controls text alignment within the column.
	Align Alignment
}

// TableConfig holds the configuration for rendering a table.
type TableConfig struct {
	// Columns defines the table structure.
	Columns []Column
	// Rows is the table data. Each row is a slice of cell strings.
	Rows [][]string
	// MaxWidth is the maximum total table width. Columns are truncated if needed.
	MaxWidth int
	// ShowHeader controls whether the header row is displayed.
	ShowHeader bool
	// HeaderStyle is the lipgloss style for the header row.
	HeaderStyle lipgloss.Style
	// RowStyle is the lipgloss style for data rows.
	RowStyle lipgloss.Style
	// AltRowStyle is the lipgloss style for alternating rows. If zero value, RowStyle is used.
	AltRowStyle lipgloss.Style
	// Separator is the column separator string (default: " | ").
	Separator string
}

// DefaultTableConfig returns a TableConfig with sensible defaults.
func DefaultTableConfig() TableConfig {
	return TableConfig{
		ShowHeader:  true,
		Separator:   " | ",
		HeaderStyle: lipgloss.NewStyle().Bold(true),
		RowStyle:    lipgloss.NewStyle(),
		AltRowStyle: lipgloss.NewStyle().Background(lipgloss.Color("#1A1A2E")),
	}
}

// RenderTable renders a formatted text table from the given configuration.
func RenderTable(cfg TableConfig) string {
	if len(cfg.Columns) == 0 {
		return ""
	}

	if cfg.Separator == "" {
		cfg.Separator = " | "
	}

	widths := calculateColumnWidths(cfg.Columns, cfg.Rows, cfg.MaxWidth)

	var lines []string

	// Header row.
	if cfg.ShowHeader {
		headerCells := make([]string, len(cfg.Columns))
		for i, col := range cfg.Columns {
			headerCells[i] = padOrTruncate(col.Title, widths[i], AlignLeft)
		}
		headerLine := strings.Join(headerCells, cfg.Separator)
		lines = append(lines, cfg.HeaderStyle.Render(headerLine))

		// Separator line under header.
		sepParts := make([]string, len(cfg.Columns))
		for i := range cfg.Columns {
			sepParts[i] = repeatChar("\u2500", widths[i])
		}
		sepLine := strings.Join(sepParts, cfg.Separator)
		lines = append(lines, sepLine)
	}

	// Data rows.
	for rowIdx, row := range cfg.Rows {
		cells := make([]string, len(cfg.Columns))
		for i := range cfg.Columns {
			cellText := ""
			if i < len(row) {
				cellText = row[i]
			}
			cells[i] = padOrTruncate(cellText, widths[i], cfg.Columns[i].Align)
		}
		rowLine := strings.Join(cells, cfg.Separator)

		style := cfg.RowStyle
		if rowIdx%2 == 1 && cfg.AltRowStyle.Value() != "" {
			style = cfg.AltRowStyle
		}
		lines = append(lines, style.Render(rowLine))
	}

	return strings.Join(lines, "\n")
}

// padOrTruncate pads or truncates a string to the given width with the specified alignment.
func padOrTruncate(s string, width int, align Alignment) string {
	if width <= 0 {
		return ""
	}

	runeLen := len([]rune(s))

	// Truncate if too long.
	if runeLen > width {
		if width <= 1 {
			return string([]rune(s)[:width])
		}
		return string([]rune(s)[:width-1]) + "\u2026"
	}

	padding := width - runeLen

	switch align {
	case AlignRight:
		return strings.Repeat(" ", padding) + s
	case AlignCenter:
		leftPad := padding / 2
		rightPad := padding - leftPad
		return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
	default: // AlignLeft
		return s + strings.Repeat(" ", padding)
	}
}

// calculateColumnWidths determines the width for each column.
// If a column has Width > 0, that value is used. Otherwise, the width is
// auto-calculated as the maximum of the header length and all cell lengths.
// If maxWidth > 0, widths are proportionally reduced to fit.
func calculateColumnWidths(cols []Column, rows [][]string, maxWidth int) []int {
	widths := make([]int, len(cols))

	for i, col := range cols {
		if col.Width > 0 {
			widths[i] = col.Width
			continue
		}
		// Auto-calculate: start with header length.
		w := len([]rune(col.Title))
		for _, row := range rows {
			if i < len(row) {
				cellLen := len([]rune(row[i]))
				if cellLen > w {
					w = cellLen
				}
			}
		}
		if w == 0 {
			w = 1
		}
		widths[i] = w
	}

	// Cap total width if maxWidth is set.
	if maxWidth > 0 {
		separatorWidth := len(cols) - 1
		if separatorWidth < 0 {
			separatorWidth = 0
		}
		// Account for separator characters between columns.
		totalSepWidth := separatorWidth * 3 // default " | " is 3 chars
		totalColWidth := 0
		for _, w := range widths {
			totalColWidth += w
		}
		totalWidth := totalColWidth + totalSepWidth
		if totalWidth > maxWidth {
			available := maxWidth - totalSepWidth
			if available < len(cols) {
				available = len(cols)
			}
			// Proportionally reduce column widths.
			for i, w := range widths {
				widths[i] = w * available / totalColWidth
				if widths[i] < 1 {
					widths[i] = 1
				}
			}
		}
	}

	return widths
}

// repeatChar repeats a string n times.
func repeatChar(c string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(c, n)
}
