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

// Columns computes left and right panel widths from the total terminal width.
// Left panel gets 40% (minimum 28 columns), right panel gets the remainder.
func Columns(totalWidth int) (left, right int) {
	return ColumnsFromConfig(totalWidth, 40)
}

// ColumnsWide computes left and right panel widths from the total terminal width
// using a wider 45/55 split. Left panel gets 45% (minimum 28 columns), right
// panel gets the remainder. Use this for commands where the left panel content
// is wider than typical file names (e.g. commit subjects in rebase-interactive).
func ColumnsWide(totalWidth int) (left, right int) {
	return ColumnsFromConfig(totalWidth, 45)
}

// IsTerminalTooSmall reports whether the terminal dimensions are below the
// minimum required for the two-panel layout (60 columns, 10 rows).
func IsTerminalTooSmall(width, height int) bool {
	return width < minTermWidth || height < minTermHeight
}
