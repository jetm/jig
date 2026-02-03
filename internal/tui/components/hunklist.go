package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/tui"
)

// rowKind distinguishes the two kinds of rows in a HunkList.
type rowKind int

const (
	rowKindFileHeader rowKind = iota
	rowKindHunk
)

// hunkRow holds the position metadata for a hunk row.
type hunkRow struct {
	fileIdx int
	hunkIdx int // index within the file's hunk slice
}

// row is a single entry in the flat row slice.
type row struct {
	kind rowKind

	// populated when kind == rowKindFileHeader
	fileIdx int

	// populated when kind == rowKindHunk
	hunk hunkRow
}

// StagedHunk pairs a staged hunk with its file index.
type StagedHunk struct {
	FileIdx int
	Hunk    git.Hunk
}

// HunkList renders a flat two-zone list: file header rows (non-selectable) followed
// by hunk rows (selectable, with checkboxes). Navigation with j/k skips file headers.
type HunkList struct {
	files  []git.FileDiff
	hunks  [][]git.Hunk // per-file hunk slices
	rows   []row
	staged map[int]bool // keyed by row index
	cursor int
	offset int
	width  int
	height int
}

// NewHunkList creates a HunkList from a slice of file diffs and their parsed hunks.
// len(hunks) must equal len(files); each hunks[i] contains the hunks for files[i].
func NewHunkList(files []git.FileDiff, hunks [][]git.Hunk) HunkList {
	hl := HunkList{
		files:  files,
		hunks:  hunks,
		staged: make(map[int]bool),
	}
	hl.buildRows()
	// Position cursor on first hunk row, if any.
	for i, r := range hl.rows {
		if r.kind == rowKindHunk {
			hl.cursor = i
			break
		}
	}
	return hl
}

// buildRows rebuilds the flat rows slice from files/hunks. Staged state is preserved
// by row index only for rows that still exist after rebuild.
func (hl *HunkList) buildRows() {
	hl.rows = hl.rows[:0]
	for fi, fd := range hl.files {
		hl.rows = append(hl.rows, row{kind: rowKindFileHeader, fileIdx: fi})
		fileHunks := hl.hunksForFile(fi, fd)
		for hi := range fileHunks {
			hl.rows = append(hl.rows, row{kind: rowKindHunk, hunk: hunkRow{fileIdx: fi, hunkIdx: hi}})
		}
	}
}

// hunksForFile returns the hunk slice for file fi. If hl.hunks is populated (post-parse),
// it uses that directly; otherwise falls back to parsing fd.RawDiff.
func (hl *HunkList) hunksForFile(fi int, fd git.FileDiff) []git.Hunk {
	if fi < len(hl.hunks) && hl.hunks[fi] != nil {
		return hl.hunks[fi]
	}
	return git.ParseHunks(fd.RawDiff)
}

// StagedHunks returns all hunks that are currently checked (staged).
func (hl *HunkList) StagedHunks() []StagedHunk {
	var result []StagedHunk
	for i, r := range hl.rows {
		if r.kind != rowKindHunk {
			continue
		}
		if !hl.staged[i] {
			continue
		}
		fi := r.hunk.fileIdx
		hi := r.hunk.hunkIdx
		fileHunks := hl.hunksForFile(fi, hl.files[fi])
		if hi < len(fileHunks) {
			result = append(result, StagedHunk{FileIdx: fi, Hunk: fileHunks[hi]})
		}
	}
	return result
}

// ReplaceHunks swaps out the hunks for a single file and rebuilds the affected rows.
// Staged state for rows within the file is cleared; other files' state is preserved.
func (hl *HunkList) ReplaceHunks(fileIdx int, hunks []git.Hunk) {
	if fileIdx >= len(hl.hunks) {
		return
	}
	// Clear staged state for rows belonging to fileIdx.
	for i, r := range hl.rows {
		if r.kind == rowKindHunk && r.hunk.fileIdx == fileIdx {
			delete(hl.staged, i)
		}
	}
	hl.hunks[fileIdx] = hunks
	// Remap staged keys: the row indices will shift after rebuild; simplest to clear
	// affected entries (already done above) then rebuild.
	hl.buildRows()
	// Clamp cursor to a valid hunk row.
	hl.clampCursor()
}

// SetWidth sets the viewport width.
func (hl *HunkList) SetWidth(w int) { hl.width = w }

// SetHeight sets the viewport height (number of visible rows).
func (hl *HunkList) SetHeight(h int) { hl.height = h }

// CurrentFileIdx returns the file index of the hunk row under the cursor, or 0.
func (hl *HunkList) CurrentFileIdx() int {
	if hl.cursor < len(hl.rows) && hl.rows[hl.cursor].kind == rowKindHunk {
		return hl.rows[hl.cursor].hunk.fileIdx
	}
	return 0
}

// CurrentHunkIdx returns the within-file hunk index of the hunk row under the cursor, or 0.
func (hl *HunkList) CurrentHunkIdx() int {
	if hl.cursor < len(hl.rows) && hl.rows[hl.cursor].kind == rowKindHunk {
		return hl.rows[hl.cursor].hunk.hunkIdx
	}
	return 0
}

// Update handles keyboard navigation and Space toggling.
func (hl *HunkList) Update(msg tea.Msg) tea.Cmd {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch kp.Code {
	case 'j', tea.KeyDown:
		hl.moveDown()
	case 'k', tea.KeyUp:
		hl.moveUp()
	case tea.KeySpace:
		if hl.cursor < len(hl.rows) && hl.rows[hl.cursor].kind == rowKindHunk {
			hl.staged[hl.cursor] = !hl.staged[hl.cursor]
		}
	}
	return nil
}

// moveDown advances the cursor to the next hunk row, skipping file headers.
func (hl *HunkList) moveDown() {
	for i := hl.cursor + 1; i < len(hl.rows); i++ {
		if hl.rows[i].kind == rowKindHunk {
			hl.cursor = i
			hl.clampOffset()
			return
		}
	}
}

// moveUp retreats the cursor to the previous hunk row, skipping file headers.
func (hl *HunkList) moveUp() {
	for i := hl.cursor - 1; i >= 0; i-- {
		if hl.rows[i].kind == rowKindHunk {
			hl.cursor = i
			hl.clampOffset()
			return
		}
	}
}

// clampCursor ensures the cursor is on a valid hunk row.
func (hl *HunkList) clampCursor() {
	if len(hl.rows) == 0 {
		hl.cursor = 0
		return
	}
	if hl.cursor >= len(hl.rows) {
		hl.cursor = len(hl.rows) - 1
	}
	// Walk backward to find a hunk row.
	for hl.cursor >= 0 && hl.rows[hl.cursor].kind != rowKindHunk {
		hl.cursor--
	}
	// If none found behind, walk forward.
	if hl.cursor < 0 {
		hl.cursor = 0
		for hl.cursor < len(hl.rows) && hl.rows[hl.cursor].kind != rowKindHunk {
			hl.cursor++
		}
		if hl.cursor >= len(hl.rows) {
			hl.cursor = 0
		}
	}
	hl.clampOffset()
}

// clampOffset adjusts the scroll offset so the cursor row is always visible.
func (hl *HunkList) clampOffset() {
	if hl.height <= 0 {
		return
	}
	if hl.cursor < hl.offset {
		hl.offset = hl.cursor
	}
	if hl.cursor >= hl.offset+hl.height {
		hl.offset = hl.cursor - hl.height + 1
	}
}

// View renders the visible slice of rows.
func (hl *HunkList) View() string {
	if len(hl.rows) == 0 {
		return ""
	}

	start := hl.offset
	end := len(hl.rows)
	if hl.height > 0 && start+hl.height < end {
		end = start + hl.height
	}
	if start > len(hl.rows) {
		start = len(hl.rows)
	}

	headerStyle := lipgloss.NewStyle().Foreground(tui.ColorFgSubtle)
	cursorStyle := lipgloss.NewStyle().Background(tui.ColorBgSel)
	normalStyle := lipgloss.NewStyle()

	var sb strings.Builder
	for i := start; i < end; i++ {
		r := hl.rows[i]
		var line string
		switch r.kind {
		case rowKindFileHeader:
			line = hl.renderFileHeader(r.fileIdx, headerStyle)
		case rowKindHunk:
			style := normalStyle
			if i == hl.cursor {
				style = cursorStyle
			}
			line = hl.renderHunkRow(i, r, style)
		}
		sb.WriteString(line)
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// renderFileHeader renders a file header row showing filename and staged count.
func (hl *HunkList) renderFileHeader(fi int, style lipgloss.Style) string {
	if fi >= len(hl.files) {
		return ""
	}
	name := hl.files[fi].DisplayPath()
	total := 0
	staged := 0
	for i, r := range hl.rows {
		if r.kind == rowKindHunk && r.hunk.fileIdx == fi {
			total++
			if hl.staged[i] {
				staged++
			}
		}
	}
	text := fmt.Sprintf("  %s (%d/%d staged)", name, staged, total)
	if hl.width > 0 {
		return style.Width(hl.width).MaxWidth(hl.width).Render(text)
	}
	return style.Render(text)
}

// renderHunkRow renders a single hunk row with checkbox and header line.
func (hl *HunkList) renderHunkRow(rowIdx int, r row, style lipgloss.Style) string {
	fi := r.hunk.fileIdx
	hi := r.hunk.hunkIdx
	fileHunks := hl.hunksForFile(fi, hl.files[fi])
	header := "@@"
	if hi < len(fileHunks) {
		header = fileHunks[hi].Header
	}

	var checkbox string
	if hl.staged[rowIdx] {
		checkbox = tui.IconChecked
	} else {
		checkbox = tui.IconUnchecked
	}

	text := checkbox + " " + header
	if hl.width > 0 {
		return style.Width(hl.width).MaxWidth(hl.width).Render(text)
	}
	return style.Render(text)
}
