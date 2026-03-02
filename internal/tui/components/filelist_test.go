package components

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/tui"
)

// --- Task 1.1: construction ---

func TestNewFileList_EmptyList(t *testing.T) {
	t.Parallel()
	fl := NewFileList(nil, false)
	assert.Empty(t, fl.SelectedPath())
	assert.Empty(t, fl.CheckedPaths())
}

func TestNewFileList_CursorOnFirstEntry(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	assert.Equal(t, "a.go", fl.SelectedPath())
}

func TestNewFileList_SingleEntry(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "foo/bar.go", Status: git.Modified},
	}
	fl := NewFileList(entries, false)
	assert.Equal(t, "foo/bar.go", fl.SelectedPath())
}

// --- Task 1.2: cursor navigation ---

func TestFileList_JMovesDown(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	sendKey(&fl, 'j')
	assert.Equal(t, "b.go", fl.SelectedPath())
}

func TestFileList_KMovesUp(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	sendKey(&fl, 'j')
	sendKey(&fl, 'k')
	assert.Equal(t, "a.go", fl.SelectedPath())
}

func TestFileList_CursorClampsAtBottom(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	sendKey(&fl, 'j')
	sendKey(&fl, 'j') // already at bottom
	assert.Equal(t, "b.go", fl.SelectedPath())
}

func TestFileList_CursorClampsAtTop(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	sendKey(&fl, 'k') // already at top
	assert.Equal(t, "a.go", fl.SelectedPath())
}

func TestFileList_ArrowDownMovesDown(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	fl.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, "b.go", fl.SelectedPath())
}

func TestFileList_ArrowUpMovesUp(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	fl.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	fl.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, "a.go", fl.SelectedPath())
}

// --- Task 1.3: rendering ---

func TestFileList_ViewContainsFilePaths(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b/c.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	fl.SetWidth(80)
	fl.SetHeight(24)
	view := fl.View()
	assert.Contains(t, view, "a.go")
	assert.Contains(t, view, "b/c.go")
}

func TestFileList_ViewContainsModifiedIcon(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
	}
	fl := NewFileList(entries, false)
	fl.SetWidth(80)
	fl.SetHeight(24)
	view := fl.View()
	assert.Contains(t, view, "M")
}

func TestFileList_ViewContainsAddedIcon(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	fl.SetWidth(80)
	fl.SetHeight(24)
	view := fl.View()
	assert.Contains(t, view, "A")
}

func TestFileList_ViewContainsDeletedIcon(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Deleted},
	}
	fl := NewFileList(entries, false)
	fl.SetWidth(80)
	fl.SetHeight(24)
	view := fl.View()
	assert.Contains(t, view, "D")
}

func TestFileList_ViewContainsRenamedIcon(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Renamed},
	}
	fl := NewFileList(entries, false)
	fl.SetWidth(80)
	fl.SetHeight(24)
	view := fl.View()
	assert.Contains(t, view, "R")
}

func TestFileList_CheckboxEnabled_UncheckedShowsUncheckedIcon(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
	}
	fl := NewFileList(entries, true)
	fl.SetWidth(80)
	fl.SetHeight(24)
	view := fl.View()
	assert.Contains(t, view, tui.IconUnchecked)
}

func TestFileList_CheckboxEnabled_CheckedShowsCheckedIcon(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
	}
	fl := NewFileList(entries, true)
	fl.SetWidth(80)
	fl.SetHeight(24)
	fl.ToggleChecked()
	view := fl.View()
	assert.Contains(t, view, tui.IconChecked)
}

func TestFileList_CheckboxDisabled_NoCheckboxIcons(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
	}
	fl := NewFileList(entries, false)
	fl.SetWidth(80)
	fl.SetHeight(24)
	view := fl.View()
	assert.NotContains(t, view, tui.IconChecked)
	assert.NotContains(t, view, tui.IconUnchecked)
}

// --- Task 1.4: checkbox operations ---

func TestFileList_ToggleChecked_UncheckedBecomesChecked(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
	}
	fl := NewFileList(entries, true)
	fl.ToggleChecked()
	paths := fl.CheckedPaths()
	assert.Equal(t, []string{"a.go"}, paths)
}

func TestFileList_ToggleChecked_CheckedBecomesUnchecked(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
	}
	fl := NewFileList(entries, true)
	fl.ToggleChecked()
	fl.ToggleChecked()
	paths := fl.CheckedPaths()
	assert.Empty(t, paths)
}

func TestFileList_SetAllChecked_True_ChecksAll(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
		{Path: "c.go", Status: git.Deleted},
	}
	fl := NewFileList(entries, true)
	fl.SetAllChecked(true)
	paths := fl.CheckedPaths()
	assert.ElementsMatch(t, []string{"a.go", "b.go", "c.go"}, paths)
}

func TestFileList_SetAllChecked_False_UnchecksAll(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, true)
	fl.SetAllChecked(true)
	fl.SetAllChecked(false)
	paths := fl.CheckedPaths()
	assert.Empty(t, paths)
}

func TestFileList_CheckedPaths_Empty_WhenNoneChecked(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, true)
	paths := fl.CheckedPaths()
	assert.Empty(t, paths)
}

func TestFileList_CheckedPaths_MixedState(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, true)
	// Only check the second entry
	sendKey(&fl, 'j')
	fl.ToggleChecked()
	paths := fl.CheckedPaths()
	assert.Equal(t, []string{"b.go"}, paths)
}

// --- Task 1.5: scrolling ---

func TestFileList_Scroll_CursorBeyondViewportAdvancesOffset(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
		{Path: "c.go", Status: git.Modified},
		{Path: "d.go", Status: git.Added},
		{Path: "e.go", Status: git.Modified},
	}
	fl := NewFileList(entries, false)
	fl.SetWidth(80)
	fl.SetHeight(3) // only 3 rows visible

	// Move cursor to row 3 (d.go) - beyond viewport
	sendKey(&fl, 'j') // b.go
	sendKey(&fl, 'j') // c.go
	sendKey(&fl, 'j') // d.go

	view := fl.View()
	// a.go should have scrolled out of view
	assert.NotContains(t, view, "a.go")
	assert.Contains(t, view, "d.go")
}

func TestFileList_Scroll_CursorAboveViewportDecrementsOffset(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
		{Path: "c.go", Status: git.Modified},
		{Path: "d.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	fl.SetWidth(80)
	fl.SetHeight(2)

	// Scroll down
	sendKey(&fl, 'j')
	sendKey(&fl, 'j')
	sendKey(&fl, 'j') // d.go

	// Now scroll back up
	sendKey(&fl, 'k')
	sendKey(&fl, 'k')
	sendKey(&fl, 'k') // a.go

	view := fl.View()
	assert.Contains(t, view, "a.go")
}

func TestFileList_SetHeight_LimitsVisibleRows(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
		{Path: "c.go", Status: git.Modified},
		{Path: "d.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	fl.SetWidth(80)
	fl.SetHeight(2)

	view := fl.View()
	// With height=2, only first 2 entries visible
	assert.Contains(t, view, "a.go")
	assert.Contains(t, view, "b.go")
	assert.NotContains(t, view, "c.go")
	assert.NotContains(t, view, "d.go")
}

func TestFileList_SetWidthHeight_DoesNotPanic(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "file.go", Status: git.Modified},
	}
	fl := NewFileList(entries, false)
	fl.SetWidth(80)
	fl.SetHeight(24)
	require.NotEmpty(t, fl.View())
}

func TestFileList_Update_NonKeyMsg_NoChange(t *testing.T) {
	t.Parallel()
	entries := []FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}
	fl := NewFileList(entries, false)
	fl.Update("some non-key message")
	assert.Equal(t, "a.go", fl.SelectedPath())
}

func TestFileList_SelectedOrCheckedPaths_ReturnsCheckedWhenPresent(t *testing.T) {
	t.Parallel()
	fl := NewFileList([]FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
		{Path: "c.go", Status: git.Deleted},
	}, true)
	fl.ToggleChecked() // check a.go
	paths := fl.SelectedOrCheckedPaths()
	if len(paths) != 1 || paths[0] != "a.go" {
		t.Errorf("SelectedOrCheckedPaths() = %v, want [a.go]", paths)
	}
}

func TestFileList_SelectedOrCheckedPaths_FallsBackToFocused(t *testing.T) {
	t.Parallel()
	fl := NewFileList([]FileEntry{
		{Path: "a.go", Status: git.Modified},
		{Path: "b.go", Status: git.Added},
	}, true)
	paths := fl.SelectedOrCheckedPaths()
	if len(paths) != 1 || paths[0] != "a.go" {
		t.Errorf("SelectedOrCheckedPaths() = %v, want [a.go]", paths)
	}
}

func TestFileList_SelectedOrCheckedPaths_EmptyList(t *testing.T) {
	t.Parallel()
	fl := NewFileList(nil, true)
	paths := fl.SelectedOrCheckedPaths()
	if paths != nil {
		t.Errorf("SelectedOrCheckedPaths() = %v, want nil", paths)
	}
}
