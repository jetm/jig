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

// testFileDiff builds a minimal git.FileDiff.
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

func TestHunkList_View_ContainsHunkHeader(t *testing.T) {
	t.Parallel()
	files := []git.FileDiff{testFileDiffH("x.go")}
	hunks := [][]git.Hunk{{testHunk("@@ -3,5 +3,6 @@")}}
	hl := NewHunkList(files, hunks)
	hl.SetWidth(80)
	hl.SetHeight(20)

	view := hl.View()
	if !strings.Contains(view, "@@ -3,5 +3,6 @@") {
		t.Errorf("View() should contain hunk header, got:\n%s", view)
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
