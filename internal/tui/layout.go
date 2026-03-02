// Package tui provides shared styles, layout functions, and constants for the jig terminal UI.
package tui

const (
	minLeftCols   = 28
	minTermWidth  = 60
	minTermHeight = 10
)

// ColumnsFromConfig computes left and right panel widths from the total terminal
// width and a configurable ratio (percentage for the left panel, e.g. 40 means
// 40%). The left panel is at least minLeftCols columns wide.
func ColumnsFromConfig(totalWidth, ratio int) (left, right int) {
	left = max(totalWidth*ratio/100, minLeftCols)
	right = totalWidth - left
	return left, right
}

// IsTerminalTooSmall reports whether the terminal dimensions are below the
// minimum required for the two-panel layout (60 columns, 10 rows).
func IsTerminalTooSmall(width, height int) bool {
	return width < minTermWidth || height < minTermHeight
}
