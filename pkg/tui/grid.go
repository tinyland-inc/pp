package tui

import (
	"gitlab.com/tinyland/lab/prompt-pulse/pkg/app"
)

// tuiCell represents a single widget's position and dimensions within
// the computed grid layout.
type tuiCell struct {
	Widget  app.Widget
	Index   int // original index in the widgets slice
	X       int
	Y       int
	W       int
	H       int
	Focused bool
}

// tuiComputeGrid computes a 2-column grid layout for the visible widgets.
// It reserves 1 row at the bottom for the status bar and respects each
// widget's MinSize. Widgets fill top-to-bottom, left-to-right.
//
// Parameters:
//   - widgets: all registered widgets
//   - width, height: terminal dimensions
//   - visible: indices of widgets to display
//   - focusedIdx: index of the focused widget in the widgets slice
func tuiComputeGrid(widgets []app.Widget, width, height int, visible []int, focusedIdx int) []tuiCell {
	if len(visible) == 0 || width <= 0 || height <= 0 {
		return nil
	}

	// Reserve 1 row for the status bar.
	availHeight := height - 1
	if availHeight < 1 {
		availHeight = 1
	}

	// Single widget: give it the full area.
	if len(visible) == 1 {
		idx := visible[0]
		return []tuiCell{{
			Widget:  widgets[idx],
			Index:   idx,
			X:       0,
			Y:       0,
			W:       width,
			H:       availHeight,
			Focused: idx == focusedIdx,
		}}
	}

	// Adaptive column count based on terminal width.
	cols := 2
	if width < 80 {
		cols = 1
	} else if width >= 160 && len(visible) >= 4 {
		cols = 3
	}
	colWidth := width / cols

	// Calculate rows needed.
	rows := (len(visible) + cols - 1) / cols
	rowHeight := availHeight / rows
	if rowHeight < 3 {
		rowHeight = 3
	}

	cells := make([]tuiCell, 0, len(visible))
	for i, idx := range visible {
		col := i % cols
		row := i / cols

		cellW := colWidth
		// Last column gets remaining width to avoid gaps.
		if col == cols-1 {
			cellW = width - col*colWidth
		}

		cellH := rowHeight
		// Last row gets remaining height.
		if row == rows-1 {
			cellH = availHeight - row*rowHeight
		}

		// Enforce minimum size from the widget.
		minW, minH := widgets[idx].MinSize()
		if cellW < minW {
			cellW = minW
		}
		if cellH < minH {
			cellH = minH
		}

		cells = append(cells, tuiCell{
			Widget:  widgets[idx],
			Index:   idx,
			X:       col * colWidth,
			Y:       row * rowHeight,
			W:       cellW,
			H:       cellH,
			Focused: idx == focusedIdx,
		})
	}

	return cells
}
