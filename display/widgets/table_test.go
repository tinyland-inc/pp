package widgets

import (
	"strings"
	"testing"
)

func TestRenderTable_Basic(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.Columns = []Column{
		{Title: "Name"},
		{Title: "Age"},
		{Title: "City"},
	}
	cfg.Rows = [][]string{
		{"Alice", "30", "New York"},
		{"Bob", "25", "London"},
	}

	result := RenderTable(cfg)
	if result == "" {
		t.Fatal("expected non-empty table output")
	}
	if !strings.Contains(result, "Alice") {
		t.Error("expected table to contain 'Alice'")
	}
	if !strings.Contains(result, "Bob") {
		t.Error("expected table to contain 'Bob'")
	}
	if !strings.Contains(result, "New York") {
		t.Error("expected table to contain 'New York'")
	}

	lines := strings.Split(result, "\n")
	// Header + separator + 2 data rows = 4 lines.
	if len(lines) != 4 {
		t.Errorf("expected 4 lines, got %d", len(lines))
	}
}

func TestRenderTable_Empty(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.Columns = []Column{
		{Title: "Name"},
		{Title: "Value"},
	}
	cfg.Rows = [][]string{}

	result := RenderTable(cfg)
	if result == "" {
		t.Fatal("expected non-empty output with header")
	}

	lines := strings.Split(result, "\n")
	// Header + separator = 2 lines, no data rows.
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (header + separator), got %d", len(lines))
	}
}

func TestRenderTable_NoColumns(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.Columns = nil
	cfg.Rows = [][]string{{"a", "b"}}

	result := RenderTable(cfg)
	if result != "" {
		t.Errorf("expected empty string for no columns, got %q", result)
	}
}

func TestRenderTable_NoHeader(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.ShowHeader = false
	cfg.Columns = []Column{
		{Title: "Name"},
		{Title: "Value"},
	}
	cfg.Rows = [][]string{
		{"foo", "bar"},
		{"baz", "qux"},
	}

	result := RenderTable(cfg)
	lines := strings.Split(result, "\n")
	// No header, no separator, just 2 data rows.
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (data only), got %d", len(lines))
	}
	// Should not contain separator line with horizontal rules.
	if strings.Contains(result, "\u2500") {
		t.Error("expected no separator line when ShowHeader is false")
	}
}

func TestRenderTable_AutoWidth(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.Columns = []Column{
		{Title: "X"},
		{Title: "LongColumnName"},
	}
	cfg.Rows = [][]string{
		{"a", "b"},
	}

	widths := calculateColumnWidths(cfg.Columns, cfg.Rows, 0)
	// First column: max("X"=1, "a"=1) = 1.
	if widths[0] != 1 {
		t.Errorf("expected first column width 1, got %d", widths[0])
	}
	// Second column: max("LongColumnName"=14, "b"=1) = 14.
	if widths[1] != 14 {
		t.Errorf("expected second column width 14, got %d", widths[1])
	}
}

func TestRenderTable_FixedWidth(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.Columns = []Column{
		{Title: "Name", Width: 10},
		{Title: "Value", Width: 20},
	}
	cfg.Rows = [][]string{
		{"short", "also short"},
	}

	widths := calculateColumnWidths(cfg.Columns, cfg.Rows, 0)
	if widths[0] != 10 {
		t.Errorf("expected fixed width 10, got %d", widths[0])
	}
	if widths[1] != 20 {
		t.Errorf("expected fixed width 20, got %d", widths[1])
	}
}

func TestRenderTable_Truncation(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.Columns = []Column{
		{Title: "Col", Width: 5},
	}
	cfg.Rows = [][]string{
		{"Hello World This Is Long"},
	}

	result := RenderTable(cfg)
	// The cell should be truncated to 5 chars (4 + ellipsis).
	if !strings.Contains(result, "\u2026") {
		t.Error("expected truncated cell to contain ellipsis character")
	}
}

func TestRenderTable_RightAlign(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.ShowHeader = false
	cfg.Columns = []Column{
		{Title: "Num", Width: 8, Align: AlignRight},
	}
	cfg.Rows = [][]string{
		{"42"},
	}

	result := RenderTable(cfg)
	// "42" right-aligned in 8 chars = "      42".
	if !strings.Contains(result, "      42") {
		t.Errorf("expected right-aligned '42' with leading spaces, got %q", result)
	}
}

func TestRenderTable_CenterAlign(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.ShowHeader = false
	cfg.Columns = []Column{
		{Title: "Val", Width: 10, Align: AlignCenter},
	}
	cfg.Rows = [][]string{
		{"hi"},
	}

	result := RenderTable(cfg)
	// "hi" centered in 10 chars = "    hi    ".
	if !strings.Contains(result, "    hi    ") {
		t.Errorf("expected centered 'hi', got %q", result)
	}
}

func TestRenderTable_CustomSeparator(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.Separator = " :: "
	cfg.Columns = []Column{
		{Title: "A"},
		{Title: "B"},
	}
	cfg.Rows = [][]string{
		{"1", "2"},
	}

	result := RenderTable(cfg)
	if !strings.Contains(result, " :: ") {
		t.Error("expected custom separator ' :: ' in output")
	}
}

func TestRenderTable_UnevenRows(t *testing.T) {
	cfg := DefaultTableConfig()
	cfg.Columns = []Column{
		{Title: "A"},
		{Title: "B"},
		{Title: "C"},
	}
	cfg.Rows = [][]string{
		{"only one"},
	}

	result := RenderTable(cfg)
	if result == "" {
		t.Fatal("expected non-empty result for uneven rows")
	}
	// Should not panic and should render the row.
	if !strings.Contains(result, "only one") {
		t.Error("expected table to contain 'only one'")
	}
}

func TestPadOrTruncate_Left(t *testing.T) {
	result := padOrTruncate("hi", 6, AlignLeft)
	if result != "hi    " {
		t.Errorf("expected 'hi    ', got %q", result)
	}
}

func TestPadOrTruncate_Right(t *testing.T) {
	result := padOrTruncate("hi", 6, AlignRight)
	if result != "    hi" {
		t.Errorf("expected '    hi', got %q", result)
	}
}

func TestPadOrTruncate_Center(t *testing.T) {
	result := padOrTruncate("hi", 6, AlignCenter)
	if result != "  hi  " {
		t.Errorf("expected '  hi  ', got %q", result)
	}
}

func TestPadOrTruncate_Truncate(t *testing.T) {
	result := padOrTruncate("Hello World", 5, AlignLeft)
	if result != "Hell\u2026" {
		t.Errorf("expected 'Hell\u2026', got %q", result)
	}
}

func TestCalculateColumnWidths(t *testing.T) {
	cols := []Column{
		{Title: "Short"},
		{Title: "A", Width: 15},
		{Title: "LongerHeader"},
	}
	rows := [][]string{
		{"VeryLongContent", "x", "y"},
		{"a", "b", "LongestCellValue"},
	}

	widths := calculateColumnWidths(cols, rows, 0)

	// First column: auto = max("Short"=5, "VeryLongContent"=15, "a"=1) = 15.
	if widths[0] != 15 {
		t.Errorf("expected first column width 15, got %d", widths[0])
	}
	// Second column: fixed = 15.
	if widths[1] != 15 {
		t.Errorf("expected second column width 15, got %d", widths[1])
	}
	// Third column: auto = max("LongerHeader"=12, "y"=1, "LongestCellValue"=16) = 16.
	if widths[2] != 16 {
		t.Errorf("expected third column width 16, got %d", widths[2])
	}
}
