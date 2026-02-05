package commands

import (
	"context"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/testhelper"
)

// newTestResetModel builds a ResetModel backed by a FakeRunner.
// nameStatus is returned for git diff --cached --name-status.
func newTestResetModel(t *testing.T, nameStatus string) *ResetModel {
	t.Helper()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			nameStatus, // git diff --cached --name-status
			"main",     // git rev-parse --abbrev-ref HEAD (BranchName)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return NewResetModel(context.Background(), runner, cfg, renderer)
}

func TestNewResetModel_EmptyFiles(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "")
	if len(m.files) != 0 {
		t.Errorf("expected 0 files, got %d", len(m.files))
	}
}

func TestNewResetModel_WithFiles(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	if m.files[0].Path != "foo.go" {
		t.Errorf("expected foo.go, got %q", m.files[0].Path)
	}
}

func TestResetModel_QuitOnQ(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected a command from pressing q")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if pop.MutatedGit {
		t.Error("MutatedGit should be false when quitting without unstaging")
	}
}

func TestResetModel_QuitOnEsc(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected a command from pressing Esc")
	}
	msg := cmd()
	if _, ok := msg.(app.PopModelMsg); !ok {
		t.Errorf("expected PopModelMsg, got %T", msg)
	}
}

func TestResetModel_HelpToggle(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	if m.help.IsVisible() {
		t.Fatal("help should start hidden")
	}
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !m.help.IsVisible() {
		t.Error("help should be visible after pressing ?")
	}
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if m.help.IsVisible() {
		t.Error("help should be hidden after pressing ? again")
	}
}

func TestResetModel_HelpVisibleBlocksKeys(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !m.help.IsVisible() {
		t.Fatal("help should be visible")
	}

	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(app.PopModelMsg); ok {
			t.Error("pressing q while help is visible should not produce PopModelMsg")
		}
	}
}

func TestResetModel_SpaceTogglesSelection(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	if len(m.fileList.CheckedPaths()) != 0 {
		t.Fatal("no files should be checked initially")
	}

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if len(m.fileList.CheckedPaths()) != 1 {
		t.Error("foo.go should be checked after pressing Space")
	}

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if len(m.fileList.CheckedPaths()) != 0 {
		t.Error("foo.go should be unchecked after pressing Space again")
	}
}

func TestResetModel_SelectAll(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\nM\tbar.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	checked := m.fileList.CheckedPaths()
	if len(checked) != 2 {
		t.Errorf("expected 2 checked files after pressing a, got %d", len(checked))
	}
}

func TestResetModel_DeselectAll(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\nM\tbar.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	checked := m.fileList.CheckedPaths()
	if len(checked) != 0 {
		t.Errorf("expected 0 checked files after pressing d, got %d", len(checked))
	}
}

func TestResetModel_EnterUnstagesSelected(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch name
			"",            // git reset HEAD (unstage call)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Select the file
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})

	// Press Enter
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command from pressing Enter")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after unstaging")
	}
	testhelper.MustHaveCall(t, runner, "reset", "HEAD", "--", "foo.go")
}

func TestResetModel_EnterUnstagesFocusedWhenNoneSelected(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch
			"",            // git reset HEAD
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Press Enter with no selection — should unstage focused file
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after unstaging focused file")
	}
}

func TestResetModel_View_TwoPanel(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	if !strings.Contains(view, "foo.go") {
		t.Error("View() should contain 'foo.go'")
	}
}

func TestResetModel_View_EmptyState(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "Nothing to unstage") {
		t.Errorf("View() with no files should say 'Nothing to unstage', got: %q", view)
	}
}

func TestResetModel_View_TooSmall(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 30
	m.height = 5

	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Errorf("View() with small terminal should mention 'too small', got: %q", view)
	}
}

func TestResetModel_View_HelpVisible(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := m.View()
	if !strings.Contains(view, "Navigation") {
		t.Error("help overlay should contain 'Navigation'")
	}
}

func TestResetModel_WindowSizeMsg(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")

	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})

	if m.width != 200 {
		t.Errorf("width = %d, want 200", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestResetModel_StatusBarShowsBranch(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "main") {
		t.Error("View() should contain branch name 'main'")
	}
}

func TestResetModel_FileTreeRendersFiles(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\ttest.go\n")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "test.go") {
		t.Errorf("View() should contain 'test.go', got %q", view)
	}
}

func TestResetModel_EnterWithNoFiles(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "")
	m.width = 120
	m.height = 40

	// No files, Enter should return nil (nothing to unstage)
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = cmd // no panic expected
}

func TestResetModel_UnstageError(t *testing.T) {
	t.Parallel()
	// Call order in NewResetModel:
	//   1. ListStagedFiles: diff --cached --name-status
	//   2. BranchName: rev-parse --abbrev-ref HEAD
	//   3. renderSelectedDiff: diff --cached -- foo.go
	// Then after Space+Enter:
	//   4. UnstageFiles: reset HEAD -- foo.go (this should fail)
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // 1. diff --cached --name-status
			"main",        // 2. branch name
			"",            // 3. diff preview
			"",            // 4. git reset HEAD (error)
		},
		Errors: []error{nil, nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command even on error")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if pop.MutatedGit {
		t.Error("MutatedGit should be false when unstaging fails")
	}
}

func TestResetModel_RenderSelectedDiff_WithDiff(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch
			// renderSelectedDiff called in NewResetModel:
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "foo.go") {
		t.Error("View() should contain file name")
	}
}

func TestResetModel_RenderSelectedDiff_DiffError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch
			"",            // renderSelectedDiff: diff call
		},
		Errors: []error{nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after diff error")
	}
}

func TestResetModel_TabTogglesFocus(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	if m.focusRight {
		t.Fatal("focus should start on left panel")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.focusRight {
		t.Error("Tab should switch focus to right panel")
	}
}

func TestResetModel_QuitFromRightPanel(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected command from q on right panel")
	}
	msg := cmd()
	if _, ok := msg.(app.PopModelMsg); !ok {
		t.Errorf("expected PopModelMsg, got %T", msg)
	}
}

func TestResetModel_DKeyTogglesDiffPanel(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	// showDiff starts true (from config default)
	viewWith := m.View()

	// Press D to hide
	m.Update(tea.KeyPressMsg{Code: 'D', ShiftedCode: 'D', Mod: tea.ModShift, Text: "D"})
	viewWithout := m.View()

	if viewWith == viewWithout {
		t.Error("View() should differ after D toggles diff off")
	}

	// Press D again to show
	m.Update(tea.KeyPressMsg{Code: 'D', ShiftedCode: 'D', Mod: tea.ModShift, Text: "D"})
	viewAgain := m.View()

	if viewAgain == viewWithout {
		t.Error("View() should differ after D toggles diff back on")
	}
}

func TestResetModel_TabNoopWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch name
		},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	if m.focusRight {
		t.Fatal("focusRight should start false")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusRight {
		t.Error("Tab should not toggle focus when diff is hidden")
	}
}

func TestResetModel_SinglePanelViewWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch name
		},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	// Should still contain the file name in the left panel
	if !strings.Contains(view, "foo.go") {
		t.Error("single-panel View() should still contain file name 'foo.go'")
	}
}

func TestResetModel_KeyJForwardsToList(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n",
			"main",
			// renderSelectedDiff after keypress:
			"",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// j key forwards to list; no panic expected
	cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_ = cmd
}

func TestResetModel_FKeyTogglesDiffMaximized(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	if m.diffMaximized {
		t.Fatal("diffMaximized should start false")
	}
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	if !m.diffMaximized {
		t.Error("F should set diffMaximized")
	}
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	if m.diffMaximized {
		t.Error("second F should clear diffMaximized")
	}
}

func TestResetModel_WKeyTogglesSoftWrap(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	initial := m.diffView.SoftWrap()
	m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if m.diffView.SoftWrap() == initial {
		t.Error("w should toggle soft-wrap when right panel focused")
	}
}

func TestResetModel_BracketKeysAdjustPanelRatio(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	start := m.panelRatio
	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if m.panelRatio != start+5 {
		t.Errorf("] should increase panelRatio by 5: got %d want %d", m.panelRatio, start+5)
	}
	m.Update(tea.KeyPressMsg{Code: '[', Text: "["})
	if m.panelRatio != start {
		t.Errorf("[ should decrease panelRatio by 5: got %d want %d", m.panelRatio, start)
	}
}

func TestResetModel_MaximizeView(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	normalView := m.View()
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	maximizedView := m.View()
	if normalView == maximizedView {
		t.Error("maximize mode should produce a different view")
	}
}

func TestResetModel_ResizeWhileMaximized(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	// Resize while maximized exercises diffMaximized branch in resize()
	cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	_ = cmd
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after resize while maximized")
	}
}

func TestResetModel_EKey_NoDiff(t *testing.T) {
	t.Parallel()
	// When the runner returns empty string for diff --cached, e key shows "No diff to edit".
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch name
			"",            // renderSelectedDiff
			"",            // e key: diff --cached -- foo.go (empty)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_ = cmd
}

func TestResetModel_EKey_NoSelection(t *testing.T) {
	t.Parallel()
	// When there are no files, e key is a noop.
	m := newTestResetModel(t, "")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_ = cmd
}

func TestResetModel_EditDiffMsg_Error(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	cmd := m.Update(git.EditDiffMsg{Err: context.DeadlineExceeded})
	_ = cmd
}

func TestResetModel_EditDiffMsg_ApplyError(t *testing.T) {
	t.Parallel()
	editedPath := t.TempDir() + "/addp-hunk-edit.diff"
	originalDiff := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n"
	modifiedDiff := originalDiff + "extra"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch name
			"",            // git apply --cached (will fail)
		},
		Errors: []error{nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	if err := os.WriteFile(editedPath, []byte(modifiedDiff), 0o600); err != nil {
		t.Fatalf("failed to write edited diff: %v", err)
	}

	cmd := m.Update(git.EditDiffMsg{
		EditedPath:   editedPath,
		OriginalDiff: originalDiff,
	})
	_ = cmd
}

func TestResetModel_EditDiffMsg_Success(t *testing.T) {
	t.Parallel()
	editedPath := t.TempDir() + "/addp-hunk-edit.diff"
	originalDiff := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch name
			"",            // renderSelectedDiff after success
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	if err := os.WriteFile(editedPath, []byte(originalDiff), 0o600); err != nil {
		t.Fatalf("failed to write edited diff: %v", err)
	}

	cmd := m.Update(git.EditDiffMsg{
		EditedPath:   editedPath,
		OriginalDiff: originalDiff,
	})
	_ = cmd
}

func TestNewResetModel_WithFilterPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // git diff --cached --name-status -- foo.go
			"main",        // branch name
			"",            // renderSelectedDiff
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer, []string{"foo.go"})
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	testhelper.MustHaveCall(t, runner, "diff", "--cached", "--name-status", "--", "foo.go")
}

func TestNewResetModel_FilterPaths_NoMatch(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",     // git diff --cached --name-status -- nonexistent.go
			"main", // branch name
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer, []string{"nonexistent.go"})
	if !m.noMatchFilter {
		t.Error("noMatchFilter should be true when filter paths match no files")
	}
	m.width = 120
	m.height = 40
	view := m.View()
	if !strings.Contains(view, "No matching") {
		t.Errorf("View() should show no-match message, got: %q", view)
	}
}
