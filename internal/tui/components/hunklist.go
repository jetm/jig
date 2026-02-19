package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/tui"
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

// CurrentHunk returns the hunk under the cursor, or false if no hunk is selected.
func (hl *HunkList) CurrentHunk() (git.Hunk, bool) {
	if hl.cursor >= len(hl.rows) || hl.rows[hl.cursor].kind != rowKindHunk {
		return git.Hunk{}, false
	}
	r := hl.rows[hl.cursor]
	fileHunks := hl.hunksForFile(r.hunk.fileIdx, hl.files[r.hunk.fileIdx])
	if r.hunk.hunkIdx >= len(fileHunks) {
		return git.Hunk{}, false
	}
	return fileHunks[r.hunk.hunkIdx], true
}

// FileHunks returns the hunk slice for the given file index.
func (hl *HunkList) FileHunks(fileIdx int) []git.Hunk {
	if fileIdx >= len(hl.files) {
		return nil
	}
	return hl.hunksForFile(fileIdx, hl.files[fileIdx])
}

// ScrollToFile moves the cursor to the first hunk of the given file.
func (hl *HunkList) ScrollToFile(fileIdx int) {
	for i, r := range hl.rows {
		if r.kind == rowKindHunk && r.hunk.fileIdx == fileIdx {
			hl.cursor = i
			hl.clampOffset()
			return
		}
	}
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

// clampOffset adjusts the scroll offset so the cursor row is always visible,
// accounting for blank separator lines between file groups.
func (hl *HunkList) clampOffset() {
	if hl.height <= 0 {
		return
	}
	if hl.cursor < hl.offset {
		hl.offset = hl.cursor
	}
	// Count blank separator lines between offset and cursor.
	blanks := 0
	for i := hl.offset + 1; i <= hl.cursor && i < len(hl.rows); i++ {
		if hl.rows[i].kind == rowKindFileHeader {
			blanks++
		}
	}
	if hl.cursor-hl.offset+1+blanks > hl.height {
		hl.offset = max(0, hl.cursor-hl.height+1+blanks)
	}
}

// View renders the visible slice of rows with blank line separators between file groups.
func (hl *HunkList) View() string {
	if len(hl.rows) == 0 {
		return ""
	}

	lineNumWidth := hl.computeLineNumWidth()

	headerStyle := lipgloss.NewStyle().Foreground(tui.ColorFgSubtle)
	cursorStyle := lipgloss.NewStyle().Background(tui.ColorBgSel)
	normalStyle := lipgloss.NewStyle()

	var lines []string
	for i := hl.offset; i < len(hl.rows); i++ {
		if hl.height > 0 && len(lines) >= hl.height {
			break
		}

		r := hl.rows[i]

		// Blank line before file headers (except the first visible row).
		if r.kind == rowKindFileHeader && len(lines) > 0 {
			if hl.height > 0 && len(lines)+1 >= hl.height {
				break
			}
			lines = append(lines, "")
		}

		var line string
		switch r.kind {
		case rowKindFileHeader:
			line = hl.renderFileHeader(r.fileIdx, headerStyle)
		case rowKindHunk:
			style := normalStyle
			if i == hl.cursor {
				style = cursorStyle
			}
			line = hl.renderHunkRow(i, r, style, lineNumWidth)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
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

// renderHunkRow renders a single hunk row with checkbox, line number, change counts,
// and optional context snippet.
func (hl *HunkList) renderHunkRow(rowIdx int, r row, style lipgloss.Style, lineNumWidth int) string {
	fi := r.hunk.fileIdx
	hi := r.hunk.hunkIdx
	fileHunks := hl.hunksForFile(fi, hl.files[fi])

	var checkbox string
	if hl.staged[rowIdx] {
		checkbox = tui.IconChecked
	} else {
		checkbox = tui.IconUnchecked
	}

	header := "@@"
	body := ""
	if hi < len(fileHunks) {
		header = fileHunks[hi].Header
		body = fileHunks[hi].Body()
	}

	lineNum := parseHunkLineNumber(header)
	added, removed := countHunkChanges(body)
	counts := formatChangeCounts(added, removed)
	snippet := hunkContextSnippet(header)

	lineStr := fmt.Sprintf("L%d", lineNum)
	paddedLine := fmt.Sprintf("%*s", lineNumWidth, lineStr)

	var text string
	if snippet != "" {
		text = fmt.Sprintf("%s %s  %-7s %s", checkbox, paddedLine, counts, snippet)
	} else {
		text = fmt.Sprintf("%s %s  %s", checkbox, paddedLine, counts)
	}

	if hl.width > 0 {
		return style.Width(hl.width).MaxWidth(hl.width).Render(text)
	}
	return style.Render(text)
}

// computeLineNumWidth returns the character width needed for the widest L{number}
// string across all hunk rows, for right-alignment.
func (hl *HunkList) computeLineNumWidth() int {
	maxWidth := 2 // minimum "L0"
	for _, r := range hl.rows {
		if r.kind != rowKindHunk {
			continue
		}
		fi := r.hunk.fileIdx
		hi := r.hunk.hunkIdx
		if fi >= len(hl.files) {
			continue
		}
		fileHunks := hl.hunksForFile(fi, hl.files[fi])
		if hi >= len(fileHunks) {
			continue
		}
		lineNum := parseHunkLineNumber(fileHunks[hi].Header)
		w := len(fmt.Sprintf("L%d", lineNum))
		if w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

// parseHunkLineNumber extracts the new-file start line number from a @@ header.
// For "@@ -72,6 +72,10 @@ func main()", it returns 72.
func parseHunkLineNumber(header string) int {
	_, after, ok := strings.Cut(header, " +")
	if !ok {
		return 0
	}
	num := 0
	for _, ch := range after {
		if ch >= '0' && ch <= '9' {
			num = num*10 + int(ch-'0')
		} else {
			break
		}
	}
	return num
}

// countHunkChanges counts added (+) and removed (-) lines in a hunk body.
func countHunkChanges(body string) (added, removed int) {
	for line := range strings.SplitSeq(body, "\n") {
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '+':
			added++
		case '-':
			removed++
		}
	}
	return
}

// formatChangeCounts formats added/removed counts as "+N", "-N", or "+N,-M".
func formatChangeCounts(added, removed int) string {
	switch {
	case added > 0 && removed > 0:
		return fmt.Sprintf("+%d,-%d", added, removed)
	case added > 0:
		return fmt.Sprintf("+%d", added)
	case removed > 0:
		return fmt.Sprintf("-%d", removed)
	default:
		return "+0"
	}
}

// hunkContextSnippet extracts the function/scope text after the closing @@ in a header.
func hunkContextSnippet(header string) string {
	_, afterFirst, ok := strings.Cut(header, "@@")
	if !ok {
		return ""
	}
	_, afterSecond, ok := strings.Cut(afterFirst, "@@")
	if !ok {
		return ""
	}
	return strings.TrimSpace(afterSecond)
}
