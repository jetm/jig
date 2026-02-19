package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/git"
)

func testHunkForView() git.Hunk {
	return git.Hunk{
		Header: "@@ -1,4 +1,5 @@",
		Lines: []git.Line{
			{Op: ' ', Content: "context1"},
			{Op: '-', Content: "removed", Selected: true},
			{Op: '+', Content: "added1", Selected: true},
			{Op: '+', Content: "added2", Selected: true},
			{Op: ' ', Content: "context2"},
		},
	}
}

func TestNewHunkView_CursorOnFirstChangeLine(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	// Line 0 is context, cursor should be on line 1 (first change)
	if hv.cursor != 1 {
		t.Errorf("cursor = %d, want 1", hv.cursor)
	}
}

func TestNewHunkView_EmptyHunk(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(git.Hunk{Header: "@@ -1 +1 @@"})
	if hv.cursor != 0 {
		t.Errorf("cursor = %d, want 0 for empty hunk", hv.cursor)
	}
}

func TestHunkView_JNavigatesDown(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	// cursor starts at 1 (removed)
	hv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Should skip to next change line (2: added1)
	if hv.cursor != 2 {
		t.Errorf("after j cursor = %d, want 2", hv.cursor)
	}
}

func TestHunkView_KNavigatesUp(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	// Move to line 2 first
	hv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	hv.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if hv.cursor != 1 {
		t.Errorf("after j+k cursor = %d, want 1", hv.cursor)
	}
}

func TestHunkView_JAtLastChangeStays(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	// Navigate to last change line
	hv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	hv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	pos := hv.cursor
	// Try to go further
	hv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if hv.cursor != pos {
		t.Errorf("j at last change should stay, cursor = %d, want %d", hv.cursor, pos)
	}
}

func TestHunkView_KAtFirstChangeStays(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	pos := hv.cursor
	hv.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if hv.cursor != pos {
		t.Errorf("k at first change should stay, cursor = %d, want %d", hv.cursor, pos)
	}
}

func TestHunkView_SpaceTogglesLine(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	// cursor at line 1, Selected=true
	if !hv.hunk.Lines[1].Selected {
		t.Fatal("line should start selected")
	}
	hv.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if hv.hunk.Lines[1].Selected {
		t.Error("Space should deselect the line")
	}
	hv.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if !hv.hunk.Lines[1].Selected {
		t.Error("Space again should re-select the line")
	}
}

func TestHunkView_SpaceDoesNotToggleContext(t *testing.T) {
	t.Parallel()
	h := git.Hunk{
		Header: "@@ -1,2 +1,2 @@",
		Lines: []git.Line{
			{Op: ' ', Content: "context"},
			{Op: '+', Content: "added", Selected: true},
		},
	}
	hv := NewHunkView(h)
	// Force cursor to context line
	hv.cursor = 0
	hv.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	// Context line should remain unselected
	if hv.hunk.Lines[0].Selected {
		t.Error("Space should not toggle context line")
	}
}

func TestHunkView_UndoRevertsToggle(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	// Toggle line 1 off
	hv.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if hv.hunk.Lines[1].Selected {
		t.Fatal("line should be deselected after Space")
	}
	// Undo
	hv.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if !hv.hunk.Lines[1].Selected {
		t.Error("undo should re-select the line")
	}
}

func TestHunkView_UndoMultiple(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	// Toggle line 1 off, then line 2 off
	hv.Update(tea.KeyPressMsg{Code: tea.KeySpace}) // toggle 1
	hv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	hv.Update(tea.KeyPressMsg{Code: tea.KeySpace}) // toggle 2

	if hv.hunk.Lines[1].Selected || hv.hunk.Lines[2].Selected {
		t.Fatal("both lines should be deselected")
	}

	// Undo second toggle
	hv.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if !hv.hunk.Lines[2].Selected {
		t.Error("first undo should re-select line 2")
	}
	// Undo first toggle
	hv.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	if !hv.hunk.Lines[1].Selected {
		t.Error("second undo should re-select line 1")
	}
}

func TestHunkView_UndoEmptyStackDoesNothing(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	// Undo with nothing to undo - should not panic
	hv.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
}

func TestHunkView_HunkReturnsModifiedState(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	hv.Update(tea.KeyPressMsg{Code: tea.KeySpace}) // deselect line 1

	result := hv.Hunk()
	if result.Lines[1].Selected {
		t.Error("returned hunk should reflect deselected state")
	}
}

func TestHunkView_View_ContainsHeader(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	hv.SetWidth(80)
	hv.SetHeight(20)
	view := hv.View()
	if !strings.Contains(view, "@@ -1,4 +1,5 @@") {
		t.Errorf("View should contain hunk header, got:\n%s", view)
	}
}

func TestHunkView_View_ContainsLines(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	hv.SetWidth(80)
	hv.SetHeight(20)
	view := hv.View()
	if !strings.Contains(view, "context1") {
		t.Errorf("View should contain context line, got:\n%s", view)
	}
	if !strings.Contains(view, "added1") {
		t.Errorf("View should contain added line, got:\n%s", view)
	}
}

func TestHunkView_View_EmptyHunk(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(git.Hunk{Header: "@@ -1 +1 @@"})
	hv.SetWidth(80)
	hv.SetHeight(20)
	view := hv.View()
	if !strings.Contains(view, "empty") {
		t.Errorf("View of empty hunk should mention 'empty', got: %q", view)
	}
}

func TestHunkView_UpdateNonKeyMsg(t *testing.T) {
	t.Parallel()
	hv := NewHunkView(testHunkForView())
	cmd := hv.Update(nil)
	if cmd != nil {
		t.Error("non-key message should return nil cmd")
	}
}

func TestHunkView_NavigationSkipsContext(t *testing.T) {
	t.Parallel()
	h := git.Hunk{
		Header: "@@ -1,5 +1,5 @@",
		Lines: []git.Line{
			{Op: '+', Content: "first", Selected: true},
			{Op: ' ', Content: "ctx1"},
			{Op: ' ', Content: "ctx2"},
			{Op: '-', Content: "last", Selected: true},
		},
	}
	hv := NewHunkView(h)
	// cursor starts at 0 (first change)
	hv.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Should skip both context lines and land on line 3
	if hv.cursor != 3 {
		t.Errorf("j should skip context lines, cursor = %d, want 3", hv.cursor)
	}
}
