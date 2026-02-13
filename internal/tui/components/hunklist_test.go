package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/gti/internal/git"
)

// testHunk builds a git.Hunk with the given header.
func testHunk(header string) git.Hunk {
	return git.Hunk{Header: header, Body: header + "\n context\n+added\n"}
}

// testFileDiffH builds a minimal git.FileDiff.
func testFileDiffH(path string) git.FileDiff {
	return git.FileDiff{OldPath: path, NewPath: path, Status: git.Modified}
}

func TestNewHunkList_Empty(t *testing.T) {
	t.Parallel()
	hl := NewHunkList(nil, nil)
	if len(hl.rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(hl.rows))
	}
	if hl.View() != "" {
		t.Error("View() on empty HunkList should return empty string")
	}
	staged := hl.StagedHunks()
	if len(staged) != 0 {
		t.Errorf("StagedHunks() on empty list should return nil, got %d", len(staged))
	}
}

func TestNewHunkList_SingleFileSingleHunk(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("foo.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1,3 +1,4 @@")}}
	hl := NewHunkList(files, hunks)

	// Should have 2 rows: 1 file header + 1 hunk row.
	if len(hl.rows) != 2 {
		t.Fatalf("expected 2 rows (header + hunk), got %d", len(hl.rows))
	}
	if hl.rows[0].kind != rowKindFileHeader {
		t.Error("row 0 should be a file header")
	}
	if hl.rows[1].kind != rowKindHunk {
		t.Error("row 1 should be a hunk row")
	}
	// Cursor should be on the hunk row.
	if hl.cursor != 1 {
		t.Errorf("cursor = %d, want 1", hl.cursor)
	}
}

func TestNewHunkList_TwoFiles(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go"), testFileDiffH("b.go")}
	hunks := [][]git.Hunk{
		{testHunk("@@ -1 +1 @@")},
		{testHunk("@@ -2 +2 @@"), testHunk("@@ -5 +5 @@")},
	}
	hl := NewHunkList(files, hunks)

	// Rows: header_a, hunk_a0, header_b, hunk_b0, hunk_b1 = 5 rows
	if len(hl.rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(hl.rows))
	}
}

func TestHunkList_JNavigation(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go"), testFileDiffH("b.go")}
	hunks := [][]git.Hunk{
		{testHunk("@@ -1 +1 @@")},
		{testHunk("@@ -2 +2 @@")},
	}
	hl := NewHunkList(files, hunks)
	// Rows: [0]=header_a [1]=hunk_a0 [2]=header_b [3]=hunk_b0
	// Cursor starts at 1.
	if hl.cursor != 1 {
		t.Fatalf("initial cursor = %d, want 1", hl.cursor)
	}

	hl.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Should skip header_b and land on hunk_b0 at row 3.
	if hl.cursor != 3 {
		t.Errorf("after j cursor = %d, want 3", hl.cursor)
	}
}

func TestHunkList_KNavigation(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go"), testFileDiffH("b.go")}
	hunks := [][]git.Hunk{
		{testHunk("@@ -1 +1 @@")},
		{testHunk("@@ -2 +2 @@")},
	}
	hl := NewHunkList(files, hunks)
	// Move to last hunk row first.
	hl.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// cursor = 3 (hunk_b0)
	hl.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	// Should skip header_b and land on hunk_a0 at row 1.
	if hl.cursor != 1 {
		t.Errorf("after k cursor = %d, want 1", hl.cursor)
	}
}

func TestHunkList_JAtLastHunkDoesNotAdvance(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1 +1 @@")}}
	hl := NewHunkList(files, hunks)
	// cursor = 1 (only hunk row)
	hl.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Should stay at 1 (no next hunk row).
	if hl.cursor != 1 {
		t.Errorf("cursor should stay at 1, got %d", hl.cursor)
	}
}

func TestHunkList_SpaceTogglesStagedState(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("foo.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1 +1 @@")}}
	hl := NewHunkList(files, hunks)
	// cursor = 1 (hunk row)

	if hl.staged[1] {
		t.Fatal("hunk should start unstaged")
	}

	hl.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if !hl.staged[1] {
		t.Error("hunk should be staged after Space")
	}

	hl.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if hl.staged[1] {
		t.Error("hunk should be unstaged after second Space")
	}
}

func TestHunkList_StagedHunks_Empty(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("foo.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1 +1 @@"), testHunk("@@ -5 +5 @@")}}
	hl := NewHunkList(files, hunks)

	staged := hl.StagedHunks()
	if len(staged) != 0 {
		t.Errorf("StagedHunks with nothing checked should return 0, got %d", len(staged))
	}
}

func TestHunkList_StagedHunks_Partial(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("foo.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1 +1 @@"), testHunk("@@ -5 +5 @@")}}
	hl := NewHunkList(files, hunks)
	// Rows: [0]=header [1]=hunk0 [2]=hunk1
	// Stage only hunk0 (row 1).
	hl.staged[1] = true

	staged := hl.StagedHunks()
	if len(staged) != 1 {
		t.Fatalf("StagedHunks should return 1, got %d", len(staged))
	}
	if staged[0].FileIdx != 0 {
		t.Errorf("StagedHunk FileIdx = %d, want 0", staged[0].FileIdx)
	}
}

func TestHunkList_StagedHunks_AllChecked(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go"), testFileDiffH("b.go")}
	hunks := [][]git.Hunk{
		{testHunk("@@ -1 +1 @@")},
		{testHunk("@@ -2 +2 @@")},
	}
	hl := NewHunkList(files, hunks)
	// Rows: [0]=header_a [1]=hunk_a0 [2]=header_b [3]=hunk_b0
	hl.staged[1] = true
	hl.staged[3] = true

	staged := hl.StagedHunks()
	if len(staged) != 2 {
		t.Fatalf("expected 2 staged hunks, got %d", len(staged))
	}
	if staged[0].FileIdx != 0 || staged[1].FileIdx != 1 {
		t.Errorf("FileIdx mismatch: got %d, %d", staged[0].FileIdx, staged[1].FileIdx)
	}
}

func TestHunkList_ReplaceHunks_RebuildRows(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("foo.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1 +1 @@")}}
	hl := NewHunkList(files, hunks)
	// Stage the hunk.
	hl.staged[1] = true

	// Replace with 2 hunks.
	newHunks := []git.Hunk{testHunk("@@ -1 +1 @@"), testHunk("@@ -10 +10 @@")}
	hl.ReplaceHunks(0, newHunks)

	// Rows: [0]=header [1]=hunk0 [2]=hunk1
	if len(hl.rows) != 3 {
		t.Fatalf("expected 3 rows after replace, got %d", len(hl.rows))
	}
	// Staged state should be cleared for file 0's rows.
	if hl.staged[1] || hl.staged[2] {
		t.Error("staged state should be cleared after ReplaceHunks")
	}
}

func TestHunkList_View_ContainsFileName(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("myfile.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1,3 +1,4 @@")}}
	hl := NewHunkList(files, hunks)
	hl.SetWidth(80)
	hl.SetHeight(20)

	view := hl.View()
	if !strings.Contains(view, "myfile.go") {
		t.Errorf("View() should contain file name, got:\n%s", view)
	}
}

func TestHunkList_View_ContainsStagedCount(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("foo.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1 +1 @@"), testHunk("@@ -5 +5 @@")}}
	hl := NewHunkList(files, hunks)
	hl.SetWidth(80)
	hl.SetHeight(20)
	// Stage 1 of 2.
	hl.staged[1] = true

	view := hl.View()
	if !strings.Contains(view, "1/2 staged") {
		t.Errorf("View() should contain '1/2 staged', got:\n%s", view)
	}
}

func TestHunkList_View_ContainsLineNumberAndCounts(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("x.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -3,5 +3,6 @@")}}
	hl := NewHunkList(files, hunks)
	hl.SetWidth(80)
	hl.SetHeight(20)

	view := hl.View()
	if !strings.Contains(view, "L3") {
		t.Errorf("View() should contain line number 'L3', got:\n%s", view)
	}
	if !strings.Contains(view, "+1") {
		t.Errorf("View() should contain change count '+1', got:\n%s", view)
	}
}

func TestHunkList_CurrentFileIdx_CurrentHunkIdx(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go"), testFileDiffH("b.go")}
	hunks := [][]git.Hunk{
		{testHunk("@@ -1 +1 @@")},
		{testHunk("@@ -2 +2 @@"), testHunk("@@ -5 +5 @@")},
	}
	hl := NewHunkList(files, hunks)
	// cursor starts at row 1 (hunk_a0): fileIdx=0, hunkIdx=0
	if hl.CurrentFileIdx() != 0 {
		t.Errorf("CurrentFileIdx = %d, want 0", hl.CurrentFileIdx())
	}
	if hl.CurrentHunkIdx() != 0 {
		t.Errorf("CurrentHunkIdx = %d, want 0", hl.CurrentHunkIdx())
	}

	// Move to hunk_b1 (row 4).
	hl.Update(tea.KeyPressMsg{Code: 'j'})
	hl.Update(tea.KeyPressMsg{Code: 'j'})
	if hl.CurrentFileIdx() != 1 {
		t.Errorf("after 2xj CurrentFileIdx = %d, want 1", hl.CurrentFileIdx())
	}
	if hl.CurrentHunkIdx() != 1 {
		t.Errorf("after 2xj CurrentHunkIdx = %d, want 1", hl.CurrentHunkIdx())
	}
}

func TestHunkList_SetWidthHeight(t *testing.T) {
	t.Parallel()
	hl := NewHunkList(nil, nil)
	hl.SetWidth(100)
	hl.SetHeight(30)
	if hl.width != 100 {
		t.Errorf("width = %d, want 100", hl.width)
	}
	if hl.height != 30 {
		t.Errorf("height = %d, want 30", hl.height)
	}
}

func TestHunkList_CurrentFileIdx_EmptyList(t *testing.T) {
	t.Parallel()
	hl := NewHunkList(nil, nil)
	// cursor = 0, no rows — should return 0 without panic
	if hl.CurrentFileIdx() != 0 {
		t.Errorf("CurrentFileIdx on empty list = %d, want 0", hl.CurrentFileIdx())
	}
	if hl.CurrentHunkIdx() != 0 {
		t.Errorf("CurrentHunkIdx on empty list = %d, want 0", hl.CurrentHunkIdx())
	}
}

func TestHunkList_HunksForFile_Fallback(t *testing.T) {
	t.Parallel()
	// Construct a HunkList with hunks[fi] == nil to exercise the ParseHunks fallback.
	hunkBody := "@@ -1,3 +1,4 @@\n package main\n+// added\n func foo() {}\n"
	fd := git.FileDiff{
		OldPath: "foo.go",
		NewPath: "foo.go",
		Status:  git.Modified,
		RawDiff: "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n" + hunkBody,
	}
	// Pass nil for hunks so hunksForFile uses RawDiff.
	hl := NewHunkList([]git.FileDiff{fd}, [][]git.Hunk{nil})
	// buildRows will call hunksForFile which calls ParseHunks on RawDiff.
	// Should produce 1 hunk row + 1 header row.
	hunkRowCount := 0
	for _, r := range hl.rows {
		if r.kind == rowKindHunk {
			hunkRowCount++
		}
	}
	if hunkRowCount != 1 {
		t.Errorf("expected 1 hunk row from RawDiff fallback, got %d", hunkRowCount)
	}
}

func TestHunkList_ClampOffset_Scrolling(t *testing.T) {
	t.Parallel()
	// Create a list with many hunks and a small height so scrolling activates.
	files := []git.FileDiff{testFileDiffH("big.go")}
	var hunks []git.Hunk
	for i := range 10 {
		hunks = append(hunks, testHunk("@@ -"+string(rune('1'+i))+" +1 @@"))
	}
	hl := NewHunkList(files, [][]git.Hunk{hunks})
	hl.SetHeight(3) // only 3 rows visible

	// Navigate to the last hunk row.
	for range 9 {
		hl.Update(tea.KeyPressMsg{Code: 'j'})
	}
	// Offset should have been adjusted.
	if hl.offset == 0 {
		t.Error("offset should be non-zero after scrolling past viewport")
	}
}

func TestHunkList_View_NoWidth(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("x.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1 +1 @@")}}
	hl := NewHunkList(files, hunks)
	// width == 0 — exercises the else branch in renderFileHeader and renderHunkRow
	hl.SetWidth(0)
	hl.SetHeight(20)
	view := hl.View()
	if view == "" {
		t.Error("View() with width=0 should not be empty")
	}
}

func TestHunkList_ReplaceHunks_OutOfBounds(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("foo.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1 +1 @@")}}
	hl := NewHunkList(files, hunks)
	// Should not panic for out-of-range fileIdx.
	hl.ReplaceHunks(99, nil)
}

func TestHunkList_Update_NonKeyMsg(t *testing.T) {
	t.Parallel()
	hl := NewHunkList(nil, nil)
	// Non-key messages should be handled gracefully.
	cmd := hl.Update(nil)
	if cmd != nil {
		t.Error("Update(nil) should return nil cmd")
	}
}

func TestHunkList_CurrentHunk_ReturnsHunkUnderCursor(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go")}
	h := testHunk("@@ -1,3 +1,4 @@")
	hunks := [][]git.Hunk{{h}}
	hl := NewHunkList(files, hunks)

	hunk, ok := hl.CurrentHunk()
	if !ok {
		t.Fatal("CurrentHunk should return true for list with hunks")
	}
	if hunk.Header != "@@ -1,3 +1,4 @@" {
		t.Errorf("CurrentHunk header = %q, want %q", hunk.Header, "@@ -1,3 +1,4 @@")
	}
}

func TestHunkList_CurrentHunk_EmptyList(t *testing.T) {
	t.Parallel()
	hl := NewHunkList(nil, nil)
	_, ok := hl.CurrentHunk()
	if ok {
		t.Error("CurrentHunk on empty list should return false")
	}
}

func TestHunkList_FileHunks_ReturnsFileSlice(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go"), testFileDiffH("b.go")}
	hunks := [][]git.Hunk{
		{testHunk("@@ -1 +1 @@")},
		{testHunk("@@ -2 +2 @@"), testHunk("@@ -5 +5 @@")},
	}
	hl := NewHunkList(files, hunks)

	fh := hl.FileHunks(1)
	if len(fh) != 2 {
		t.Errorf("FileHunks(1) should return 2 hunks, got %d", len(fh))
	}
}

func TestHunkList_FileHunks_OutOfBounds(t *testing.T) {
	t.Parallel()
	hl := NewHunkList(nil, nil)
	fh := hl.FileHunks(99)
	if fh != nil {
		t.Error("FileHunks out of bounds should return nil")
	}
}

func TestHunkList_ScrollToFile_MovesToFirstHunk(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go"), testFileDiffH("b.go")}
	hunks := [][]git.Hunk{
		{testHunk("@@ -1 +1 @@")},
		{testHunk("@@ -2 +2 @@")},
	}
	hl := NewHunkList(files, hunks)

	// Starts at file 0
	if hl.CurrentFileIdx() != 0 {
		t.Fatal("should start at file 0")
	}

	// Scroll to file 1
	hl.ScrollToFile(1)
	if hl.CurrentFileIdx() != 1 {
		t.Errorf("after ScrollToFile(1) CurrentFileIdx = %d, want 1", hl.CurrentFileIdx())
	}
}

func TestHunkList_ScrollToFile_NoMatchDoesNothing(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1 +1 @@")}}
	hl := NewHunkList(files, hunks)

	before := hl.cursor
	hl.ScrollToFile(99) // no such file
	if hl.cursor != before {
		t.Error("ScrollToFile with invalid index should not change cursor")
	}
}

// --- Helper function tests ---

func TestParseHunkLineNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		header string
		want   int
	}{
		{"@@ -1,3 +1,4 @@", 1},
		{"@@ -72,6 +72,10 @@ func main()", 72},
		{"@@ -1 +1 @@", 1},
		{"@@ -150,5 +151,6 @@", 151},
		{"@@ -10,3 +10,5 @@", 10},
		{"@@", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseHunkLineNumber(tt.header)
		if got != tt.want {
			t.Errorf("parseHunkLineNumber(%q) = %d, want %d", tt.header, got, tt.want)
		}
	}
}

func TestCountHunkChanges(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		body        string
		wantAdded   int
		wantRemoved int
	}{
		{
			name:        "pure addition",
			body:        "@@ -1 +1 @@\n context\n+line1\n+line2\n+line3\n+line4\n",
			wantAdded:   4,
			wantRemoved: 0,
		},
		{
			name:        "pure deletion",
			body:        "@@ -1 +1 @@\n context\n-line1\n-line2\n-line3\n",
			wantAdded:   0,
			wantRemoved: 3,
		},
		{
			name:        "mixed changes",
			body:        "@@ -1 +1 @@\n context\n+added1\n+added2\n-removed1\n",
			wantAdded:   2,
			wantRemoved: 1,
		},
		{
			name:        "empty body",
			body:        "",
			wantAdded:   0,
			wantRemoved: 0,
		},
	}
	for _, tt := range tests {
		added, removed := countHunkChanges(tt.body)
		if added != tt.wantAdded || removed != tt.wantRemoved {
			t.Errorf("countHunkChanges(%s) = (%d, %d), want (%d, %d)",
				tt.name, added, removed, tt.wantAdded, tt.wantRemoved)
		}
	}
}

func TestFormatChangeCounts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		added   int
		removed int
		want    string
	}{
		{4, 0, "+4"},
		{0, 3, "-3"},
		{2, 1, "+2,-1"},
		{0, 0, "+0"},
	}
	for _, tt := range tests {
		got := formatChangeCounts(tt.added, tt.removed)
		if got != tt.want {
			t.Errorf("formatChangeCounts(%d, %d) = %q, want %q",
				tt.added, tt.removed, got, tt.want)
		}
	}
}

func TestHunkContextSnippet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		header string
		want   string
	}{
		{"@@ -72,6 +72,10 @@ func main()", "func main()"},
		{"@@ -3,5 +3,10 @@", ""},
		{"@@ -1 +1 @@", ""},
		{"@@ -1,3 +1,4 @@ for i := range items {", "for i := range items {"},
	}
	for _, tt := range tests {
		got := hunkContextSnippet(tt.header)
		if got != tt.want {
			t.Errorf("hunkContextSnippet(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestComputeLineNumWidth(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go"), testFileDiffH("b.go"), testFileDiffH("c.go")}
	hunks := [][]git.Hunk{
		{git.Hunk{Header: "@@ -3,5 +3,6 @@", Body: "@@ -3,5 +3,6 @@\n+x\n"}},
		{git.Hunk{Header: "@@ -70,5 +72,6 @@", Body: "@@ -70,5 +72,6 @@\n+x\n"}},
		{git.Hunk{Header: "@@ -150,5 +151,6 @@", Body: "@@ -150,5 +151,6 @@\n+x\n"}},
	}
	hl := NewHunkList(files, hunks)
	width := hl.computeLineNumWidth()
	// L151 = 4 chars
	if width != 4 {
		t.Errorf("computeLineNumWidth = %d, want 4", width)
	}
}

func TestHunkList_View_BlankLineBetweenFiles(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go"), testFileDiffH("b.go")}
	hunks := [][]git.Hunk{
		{testHunk("@@ -1 +1 @@")},
		{testHunk("@@ -2 +2 @@")},
	}
	hl := NewHunkList(files, hunks)
	hl.SetWidth(80)
	hl.SetHeight(20)

	view := hl.View()
	if !strings.Contains(view, "\n\n") {
		t.Errorf("View() should contain a blank line between file groups, got:\n%s", view)
	}
}

func TestHunkList_View_NoBlankLineForSingleFile(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -1 +1 @@")}}
	hl := NewHunkList(files, hunks)
	hl.SetWidth(80)
	hl.SetHeight(20)

	view := hl.View()
	if strings.Contains(view, "\n\n") {
		t.Errorf("View() for single file should not contain blank line separator, got:\n%s", view)
	}
}

func TestHunkList_View_HunkRowFormat(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("main.go")}
	h := git.Hunk{
		Header: "@@ -72,6 +72,10 @@ func main()",
		Body:   "@@ -72,6 +72,10 @@ func main()\n context\n+line1\n+line2\n-removed\n",
	}
	hunks := [][]git.Hunk{{h}}
	hl := NewHunkList(files, hunks)
	hl.SetWidth(80)
	hl.SetHeight(20)

	view := hl.View()
	if !strings.Contains(view, "L72") {
		t.Errorf("View() should contain 'L72', got:\n%s", view)
	}
	if !strings.Contains(view, "+2,-1") {
		t.Errorf("View() should contain '+2,-1' for mixed changes, got:\n%s", view)
	}
	if !strings.Contains(view, "func main()") {
		t.Errorf("View() should contain context snippet 'func main()', got:\n%s", view)
	}
}

func TestHunkList_View_RightAlignedLineNumbers(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("a.go")}
	hunks := [][]git.Hunk{{
		git.Hunk{Header: "@@ -3,5 +3,6 @@", Body: "@@ -3,5 +3,6 @@\n+x\n"},
		git.Hunk{Header: "@@ -150,5 +151,6 @@", Body: "@@ -150,5 +151,6 @@\n+x\n"},
	}}
	hl := NewHunkList(files, hunks)
	hl.SetWidth(0) // no width constraint for easier assertion
	hl.SetHeight(20)

	view := hl.View()
	// L3 should be right-padded to match L151 width (4 chars): "  L3"
	if !strings.Contains(view, "  L3") {
		t.Errorf("View() should contain right-aligned '  L3', got:\n%s", view)
	}
	if !strings.Contains(view, "L151") {
		t.Errorf("View() should contain 'L151', got:\n%s", view)
	}
}
