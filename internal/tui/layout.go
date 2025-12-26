// Package tui provides shared styles, layout functions, and constants for the gti terminal UI.
package tui

const (
	minLeftCols   = 28
	minTermWidth  = 60
	minTermHeight = 10
)

// Columns computes left and right panel widths from the total terminal width.
// Left panel gets 30% (minimum 28 columns), right panel gets the remainder.
func Columns(totalWidth int) (left, right int) {
	left = max(totalWidth*30/100, minLeftCols)
	right = totalWidth - left
	return left, right
}

// IsTerminalTooSmall reports whether the terminal dimensions are below the
// minimum required for the two-panel layout (60 columns, 10 rows).
func IsTerminalTooSmall(width, height int) bool {
	return width < minTermWidth || height < minTermHeight
}
