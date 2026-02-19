package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/tui"
)

// undoEntry records a single toggle action for undo support.
type undoEntry struct {
	lineIdx  int
	wasState bool // Selected state before the toggle
}

// HunkView provides line-level navigation and selection within a single hunk.
// j/k navigate lines, Space toggles a line's selection, u undoes the last toggle.
// Only +/- lines are togglable; context lines are skipped during navigation.
type HunkView struct {
	hunk      git.Hunk
	cursor    int // index into hunk.Lines
	offset    int // scroll offset for viewport
	width     int
	height    int
	undoStack []undoEntry
}

// NewHunkView creates a HunkView for the given hunk.
// The cursor starts on the first changeable line.
func NewHunkView(h git.Hunk) HunkView {
	hv := HunkView{hunk: h}
	// Position cursor on first change line.
	for i, l := range h.Lines {
		if l.Op == '+' || l.Op == '-' {
			hv.cursor = i
			break
		}
	}
	return hv
}

// Hunk returns the current hunk with line selections.
func (hv *HunkView) Hunk() git.Hunk {
	return hv.hunk
}

// SetWidth sets the view width.
func (hv *HunkView) SetWidth(w int) { hv.width = w }

// SetHeight sets the view height.
func (hv *HunkView) SetHeight(h int) { hv.height = h }

// Update handles keyboard input for line-level navigation and toggling.
func (hv *HunkView) Update(msg tea.Msg) tea.Cmd {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch kp.Code {
	case 'j', tea.KeyDown:
		hv.moveDown()
	case 'k', tea.KeyUp:
		hv.moveUp()
	case tea.KeySpace:
		hv.toggleCurrent()
	case 'u':
		hv.undo()
	}
	return nil
}

// moveDown advances the cursor to the next change line, skipping context.
func (hv *HunkView) moveDown() {
	for i := hv.cursor + 1; i < len(hv.hunk.Lines); i++ {
		if hv.hunk.Lines[i].Op == '+' || hv.hunk.Lines[i].Op == '-' {
			hv.cursor = i
			hv.clampOffset()
			return
		}
	}
}

// moveUp retreats the cursor to the previous change line, skipping context.
func (hv *HunkView) moveUp() {
	for i := hv.cursor - 1; i >= 0; i-- {
		if hv.hunk.Lines[i].Op == '+' || hv.hunk.Lines[i].Op == '-' {
			hv.cursor = i
			hv.clampOffset()
			return
		}
	}
}

// toggleCurrent toggles the Selected state of the line under the cursor.
func (hv *HunkView) toggleCurrent() {
	if hv.cursor >= len(hv.hunk.Lines) {
		return
	}
	l := &hv.hunk.Lines[hv.cursor]
	if l.Op != '+' && l.Op != '-' {
		return
	}
	hv.undoStack = append(hv.undoStack, undoEntry{
		lineIdx:  hv.cursor,
		wasState: l.Selected,
	})
	l.Selected = !l.Selected
}

// undo reverts the most recent toggle.
func (hv *HunkView) undo() {
	if len(hv.undoStack) == 0 {
		return
	}
	last := hv.undoStack[len(hv.undoStack)-1]
	hv.undoStack = hv.undoStack[:len(hv.undoStack)-1]
	if last.lineIdx < len(hv.hunk.Lines) {
		hv.hunk.Lines[last.lineIdx].Selected = last.wasState
	}
}

// clampOffset adjusts scroll offset so the cursor line is visible.
func (hv *HunkView) clampOffset() {
	if hv.height <= 0 {
		return
	}
	if hv.cursor < hv.offset {
		hv.offset = hv.cursor
	}
	if hv.cursor >= hv.offset+hv.height {
		hv.offset = hv.cursor - hv.height + 1
	}
}

// View renders the line-level hunk view with selection indicators.
func (hv *HunkView) View() string {
	if len(hv.hunk.Lines) == 0 {
		return "(empty hunk)"
	}

	addedStyle := lipgloss.NewStyle().Foreground(tui.ColorGreen)
	removedStyle := lipgloss.NewStyle().Foreground(tui.ColorRed)
	contextStyle := lipgloss.NewStyle().Foreground(tui.ColorFgSubtle)
	cursorStyle := lipgloss.NewStyle().Background(tui.ColorBgSel)

	var lines []string

	// Show hunk header
	headerLine := contextStyle.Render(hv.hunk.Header)
	lines = append(lines, headerLine)

	end := len(hv.hunk.Lines)
	if hv.height > 0 && hv.offset+hv.height-1 < end {
		end = hv.offset + hv.height - 1 // -1 for header line
	}
	start := min(hv.offset, len(hv.hunk.Lines))

	for i := start; i < end; i++ {
		if hv.height > 0 && len(lines) >= hv.height {
			break
		}
		l := hv.hunk.Lines[i]
		line := hv.renderLine(i, l, addedStyle, removedStyle, contextStyle, cursorStyle)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderLine renders a single diff line with selection indicator and styling.
func (hv *HunkView) renderLine(idx int, l git.Line, addedStyle, removedStyle, contextStyle, cursorStyle lipgloss.Style) string {
	// Selection indicator
	indicator := "  "
	switch {
	case (l.Op == '+' || l.Op == '-') && l.Selected:
		indicator = tui.IconChecked + " "
	case l.Op == '+' || l.Op == '-':
		indicator = tui.IconUnchecked + " "
	}

	content := fmt.Sprintf("%s%c%s", indicator, l.Op, l.Content)

	// Apply line-type styling
	var styled string
	switch l.Op {
	case '+':
		styled = addedStyle.Render(content)
	case '-':
		styled = removedStyle.Render(content)
	default:
		styled = contextStyle.Render(content)
	}

	// Apply cursor highlight
	if idx == hv.cursor {
		if hv.width > 0 {
			return cursorStyle.Width(hv.width).MaxWidth(hv.width).Render(styled)
		}
		return cursorStyle.Render(styled)
	}

	if hv.width > 0 {
		return lipgloss.NewStyle().Width(hv.width).MaxWidth(hv.width).Render(styled)
	}
	return styled
}
