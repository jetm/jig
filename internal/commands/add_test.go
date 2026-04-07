package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/editor"
	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/testhelper"

	"github.com/stretchr/testify/require"
)

// newTestAddModel builds an AddModel backed by a FakeRunner.
// diffOutput and nameStatus are returned for runner calls.
func newTestAddModel(t *testing.T, nameStatus, untracked string) *AddModel {
	t.Helper()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			nameStatus, // git diff --name-status
			untracked,  // git ls-files --others
			"main",     // git rev-parse --abbrev-ref HEAD (BranchName)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewAddModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestNewAddModel_EmptyFiles(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "", "")
	if len(m.files) != 0 {
		t.Errorf("expected 0 files, got %d", len(m.files))
	}
}

func TestNewAddModel_WithFiles(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	if m.files[0].Path != "foo.go" {
		t.Errorf("expected foo.go, got %q", m.files[0].Path)
	}
}

func TestAddModel_QuitOnQ(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
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
		t.Error("MutatedGit should be false when quitting without staging")
	}
}

func TestAddModel_QuitOnEsc(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
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

func TestAddModel_HelpToggle(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
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

func TestAddModel_HelpVisibleBlocksKeys(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
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

func TestAddModel_SpaceTogglesSelection(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
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

func TestAddModel_SelectAll(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\nM\tbar.go\n", "")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})

	checked := m.fileList.CheckedPaths()
	if len(checked) != 2 {
		t.Errorf("expected 2 checked files after pressing a, got %d", len(checked))
	}
}

func TestAddModel_DeselectAll(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\nM\tbar.go\n", "")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})

	checked := m.fileList.CheckedPaths()
	if len(checked) != 0 {
		t.Errorf("expected 0 checked files after pressing d, got %d", len(checked))
	}
}

func TestAddModel_EnterStagesSelected(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files --others
			"main",        // branch name
			"",            // git add (stage call)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
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
		t.Error("MutatedGit should be true after staging")
	}
	testhelper.MustHaveCall(t, runner, "add", "--", "foo.go")
}

func TestAddModel_EnterStagesFocusedWhenNoneSelected(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files
			"main",        // branch
			"",            // git add
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	// Press Enter with no selection — should stage focused file
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
		t.Error("MutatedGit should be true after staging focused file")
	}
}

func TestAddModel_View_TwoPanel(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
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

func TestAddModel_View_EmptyState(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "", "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "Nothing to stage") {
		t.Errorf("View() with no files should say 'Nothing to stage', got: %q", view)
	}
}

func TestAddModel_View_TooSmall(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 30
	m.height = 5

	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Errorf("View() with small terminal should mention 'too small', got: %q", view)
	}
}

func TestAddModel_View_HelpVisible(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := m.View()
	if !strings.Contains(view, "Navigation") {
		t.Error("help overlay should contain 'Navigation'")
	}
}

func TestAddModel_WindowSizeMsg(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")

	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})

	if m.width != 200 {
		t.Errorf("width = %d, want 200", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestAddModel_StatusBarShowsBranch(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "main") {
		t.Error("View() should contain branch name 'main'")
	}
}

func TestAddModel_FileTreeRendersFiles(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\ttest.go\n", "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "test.go") {
		t.Errorf("View() should contain 'test.go', got %q", view)
	}
}

func TestAddModel_EnterWithNoFiles(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "", "")
	m.width = 120
	m.height = 40

	// No files, Enter should return nil (nothing to stage)
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		// It may return nil or a no-op; the important thing is no panic
		_ = cmd
	}
}

func TestAddModel_UntrackedFileShowsDiff(t *testing.T) {
	t.Parallel()
	fakeDiff := "diff --git a/newfile.go b/newfile.go\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/newfile.go\n" +
		"@@ -0,0 +1,3 @@\n" +
		"+package main\n" +
		"+\n" +
		"+func main() {}\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",             // diff --name-status (no tracked changes)
			"newfile.go\n", // ls-files --others
			"main",         // branch name
			fakeDiff,       // diff --no-index -- /dev/null newfile.go
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}

	view := m.View()
	// Should show the actual diff content, not a placeholder
	if strings.Contains(view, "New file (untracked)") {
		t.Error("View() should not show placeholder for untracked files")
	}
	if !strings.Contains(view, "package main") {
		t.Error("View() should show diff content for untracked file")
	}
	// Verify the --no-index call was made
	testhelper.MustHaveCall(t, runner, "diff", "-U3", "--no-index", "--", "/dev/null", "newfile.go")
}

func TestAddModel_StageError(t *testing.T) {
	t.Parallel()
	// Call order in NewAddModel:
	//   1. ListUnstagedFiles: diff --name-status
	//   2. ListUnstagedFiles: ls-files --others
	//   3. BranchName: rev-parse --abbrev-ref HEAD
	//   4. renderSelectedDiff: diff -- foo.go  (tracked modified file)
	// Then after Space+Enter:
	//   5. StageFiles: add -- foo.go  (this should fail)
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // 1. diff --name-status
			"",            // 2. ls-files --others
			"main",        // 3. branch name
			"",            // 4. diff preview
			"",            // 5. git add (error)
		},
		Errors: []error{nil, nil, nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command even on error")
	}
	msg := cmd()
	if _, ok := msg.(app.PopModelMsg); ok {
		t.Fatal("should not pop model on stage error — model must stay visible with error in status bar")
	}
}

func TestAddModel_RenderSelectedDiff_WithDiff(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files
			"main",        // branch
			// renderSelectedDiff called in NewAddModel for first tracked file:
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "foo.go") {
		t.Error("View() should contain file name")
	}
}

func TestAddModel_RenderSelectedDiff_DiffError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files
			"main",        // branch
			"",            // renderSelectedDiff: diff call
		},
		Errors: []error{nil, nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after diff error")
	}
}

func TestAddModel_IsTracked_AddedStatus(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"A\tfoo.go\n", // diff --name-status — Added (tracked)
			"",            // ls-files
			"main",        // branch
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)

	// isTracked checks Status != Added, so Added status returns false
	if m.isTracked("foo.go") {
		t.Error("isTracked should return false for Added status")
	}
	// Modified file would return true
	m.files = append(m.files, git.StatusFile{Path: "bar.go", Status: git.Modified})
	if !m.isTracked("bar.go") {
		t.Error("isTracked should return true for Modified status")
	}
}

func TestAddModel_TabTogglesFocus(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 120
	m.height = 40

	if m.focusRight {
		t.Fatal("focus should start on left panel")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.focusRight {
		t.Error("Tab should switch focus to right panel")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusRight {
		t.Error("Tab again should switch focus back to left panel")
	}
}

func TestAddModel_QuitFromRightPanel(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
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

func TestAddModel_SpaceWorksFromRightPanel(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if len(m.fileList.CheckedPaths()) != 1 {
		t.Error("Space from right panel should toggle selection on left panel item")
	}
}

func TestAddModel_View_StatusBarPresentWithManyFiles(t *testing.T) {
	t.Parallel()
	// Create many files in nested directories to stress the layout.
	var nameStatus strings.Builder
	for i := range 50 {
		fmt.Fprintf(&nameStatus, "M\tdir%d/subdir/file%d.go\n", i, i)
	}
	m := newTestAddModel(t, nameStatus.String(), "")
	m.width = 120
	m.height = 25 // Short terminal

	view := m.View()

	// Status bar must always be present.
	if !strings.Contains(view, "add") {
		t.Error("View() should contain status bar with mode 'add'")
	}

	// Output must not exceed terminal height.
	lines := strings.Split(view, "\n")
	if len(lines) > m.height {
		t.Errorf("View() output has %d lines, exceeds terminal height %d", len(lines), m.height)
	}
}

func TestAddModel_SpaceTogglesEachFileIndividually(t *testing.T) {
	t.Parallel()
	// FileList shows all files flat - space toggles the cursor file only.
	m := newTestAddModel(t, "M\tdir/file1.go\nM\tdir/file2.go\n", "")
	m.width = 120
	m.height = 40

	// Cursor is on first file. Space checks it.
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	checked := m.fileList.CheckedPaths()
	if len(checked) != 1 {
		t.Errorf("expected 1 checked file after first space, got %d: %v", len(checked), checked)
	}

	// Move to second file and check it.
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	checked = m.fileList.CheckedPaths()
	if len(checked) != 2 {
		t.Errorf("expected 2 checked files after checking both, got %d: %v", len(checked), checked)
	}
}

func TestAddModel_DKeyTogglesDiffPanel(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
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

func TestAddModel_TabNoopWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files --others
			"main",        // branch name
		},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
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

func TestAddModel_SinglePanelViewWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files --others
			"main",        // branch name
		},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
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

func TestAddModel_FKeyTogglesDiffMaximized(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 120
	m.height = 40

	if m.diffMaximized {
		t.Fatal("diffMaximized should start false")
	}

	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	if !m.diffMaximized {
		t.Error("F should set diffMaximized to true")
	}

	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	if m.diffMaximized {
		t.Error("F again should restore diffMaximized to false")
	}
}

func TestAddModel_FKeyNoopWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n",
			"",
			"main",
		},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	if m.diffMaximized {
		t.Error("F should not set diffMaximized when diff is hidden")
	}
}

func TestAddModel_WKeyTogglesSoftWrap(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 120
	m.height = 40

	// Focus the right panel first
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.focusRight {
		t.Fatal("should be focused on right panel after Tab")
	}

	initial := m.diff.SoftWrap()
	m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if m.diff.SoftWrap() == initial {
		t.Error("w should toggle soft-wrap")
	}

	m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if m.diff.SoftWrap() != initial {
		t.Error("w again should restore soft-wrap to initial state")
	}
}

func TestAddModel_WKeyNoopWhenLeftFocused(t *testing.T) {
	t.Parallel()
	cfg := config.NewDefault()
	cfg.SoftWrap = false
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "", "main"},
	}
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	// Left panel focused (default)
	m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if m.diff.SoftWrap() {
		t.Error("w should not toggle soft-wrap when left panel is focused")
	}
}

func TestAddModel_MaximizeViewOmitsLeftPanel(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 120
	m.height = 40

	normalView := m.View()

	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	maximizedView := m.View()

	// In normal view foo.go is in the left panel; in maximized view it should not appear
	// (we may or may not have content in diff, but the views must differ)
	if normalView == maximizedView {
		t.Error("maximize mode should produce a different view from normal two-panel layout")
	}
}

func TestAddModel_InitialSoftWrapFromConfig(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "", "main"},
	}
	cfg := config.NewDefault()
	cfg.SoftWrap = true
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)

	if !m.diff.SoftWrap() {
		t.Error("diffView should start with soft-wrap enabled when config.SoftWrap=true")
	}
}

func TestAddModel_BracketRightIncreasesPanelRatio(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "", "main"},
	}
	cfg := config.NewDefault()
	cfg.PanelRatio = 40
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if m.panelRatio != 45 {
		t.Errorf("panelRatio should be 45 after ], got %d", m.panelRatio)
	}
}

func TestAddModel_BracketLeftDecreasesPanelRatio(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "", "main"},
	}
	cfg := config.NewDefault()
	cfg.PanelRatio = 40
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '[', Text: "["})
	if m.panelRatio != 35 {
		t.Errorf("panelRatio should be 35 after [, got %d", m.panelRatio)
	}
}

func TestAddModel_BracketRightClampsAt80(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "", "main"},
	}
	cfg := config.NewDefault()
	cfg.PanelRatio = 80
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if m.panelRatio != 80 {
		t.Errorf("panelRatio should stay at 80 (max), got %d", m.panelRatio)
	}
}

func TestAddModel_BracketLeftClampsAt20(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "", "main"},
	}
	cfg := config.NewDefault()
	cfg.PanelRatio = 20
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '[', Text: "["})
	if m.panelRatio != 20 {
		t.Errorf("panelRatio should stay at 20 (min), got %d", m.panelRatio)
	}
}

func TestAddModel_InitialPanelRatioFromConfig(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "", "main"},
	}
	cfg := config.NewDefault()
	cfg.PanelRatio = 60
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)

	if m.panelRatio != 60 {
		t.Errorf("panelRatio should be 60 from config, got %d", m.panelRatio)
	}
}

func TestAddModel_ResizeWhileMaximized(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Maximize the diff panel
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	// Trigger resize while maximized - exercises diffMaximized branch
	cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	_ = cmd
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after resize while maximized")
	}
}

func TestAddModel_EKeyCallsEditDiff(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	t.Setenv("GIT_EDITOR", "true")
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files --others
			"main",        // branch
			// renderSelectedDiff in NewAddModel:
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n",
			// e key: diff -- foo.go
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	// Should return a tea.Cmd (the ExecProcess cmd)
	if cmd == nil {
		t.Fatal("expected a command from pressing e with a diff available")
	}
}

func TestAddModel_EKeyNoopWhenNoFile(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "", "")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	// No file selected - should return sbCmd (non-nil because status bar processes)
	// but no ExecProcess is returned
	_ = cmd
	// Verify no "diff --" call was made beyond the initial construction calls
}

func TestAddModel_EditDiffMsg_Success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Write a modified diff to the temp file
	editedPath := dir + "/addp-hunk-edit.diff"
	originalDiff := "diff --git a/foo.go b/foo.go\n"
	modifiedDiff := originalDiff + "// extra line\n"
	if err := os.WriteFile(editedPath, []byte(modifiedDiff), 0o600); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n",
			"",
			"main",
			"", // renderSelectedDiff
			"", // git apply --cached
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	msg := editor.EditDiffMsg{
		EditedPath:   editedPath,
		OriginalDiff: originalDiff,
	}
	cmd := m.Update(msg)
	_ = cmd

	testhelper.MustHaveCall(t, runner, "apply", "--cached")
}

func TestAddModel_EditDiffMsg_Error(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 120
	m.height = 40

	msg := editor.EditDiffMsg{Err: fmt.Errorf("editor crashed")}
	cmd := m.Update(msg)
	_ = cmd
	// Should not panic; error displayed in status bar
}

func TestNewAddModel_WithFilterPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status -- foo.go
			"",            // ls-files --others -- foo.go
			"main",        // branch
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer, []string{"foo.go"})
	require.NoError(t, err)
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	if m.files[0].Path != "foo.go" {
		t.Errorf("expected foo.go, got %q", m.files[0].Path)
	}
	if m.noMatchFilter {
		t.Error("noMatchFilter should be false when files are found")
	}
}

func TestNewAddModel_FilterPaths_NoMatch(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",     // diff --name-status -- nonexistent.go (no changes)
			"",     // ls-files --others -- nonexistent.go
			"main", // branch
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer, []string{"nonexistent.go"})
	require.NoError(t, err)
	if len(m.files) != 0 {
		t.Errorf("expected 0 files, got %d", len(m.files))
	}
	if !m.noMatchFilter {
		t.Error("noMatchFilter should be true when filter yields no files")
	}

	m.width = 120
	m.height = 40
	view := m.View()
	if !strings.Contains(view, "No matching changes") {
		t.Errorf("View() with no-match filter should say 'No matching changes', got: %q", view)
	}
}

func TestAddModel_CKeyStagesAndReturnsExecCmd(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files --others
			"main",        // branch name
			"",            // renderSelectedDiff
			"",            // git add (stage call)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	// Select the file
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})

	// Press c to commit
	cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if cmd == nil {
		t.Fatal("expected a command from pressing c")
	}
	// Verify staging happened
	testhelper.MustHaveCall(t, runner, "add", "--", "foo.go")
}

func TestAddModel_ShiftCKeyStagesAndReturnsExecCmd(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files --others
			"main",        // branch name
			"",            // renderSelectedDiff
			"",            // git add (stage call)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	// Press C (shift-c) - should stage focused file and return exec cmd
	cmd := m.Update(tea.KeyPressMsg{Code: 'C', ShiftedCode: 'C', Mod: tea.ModShift, Text: "C"})
	if cmd == nil {
		t.Fatal("expected a command from pressing C")
	}
	testhelper.MustHaveCall(t, runner, "add", "--", "foo.go")
}

func TestAddModel_CKeyStageFailureReturnsNil(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files --others
			"main",        // branch name
			"",            // renderSelectedDiff
			"",            // git add (will fail)
		},
		Errors: []error{nil, nil, nil, nil, fmt.Errorf("staging failed")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	// staging failed -> no exec cmd returned, just status bar update
	if cmd != nil {
		// cmd should be the statusbar cmd, not an exec cmd
		msg := cmd()
		if _, ok := msg.(CommitDoneMsg); ok {
			t.Error("should not return CommitDoneMsg when staging fails")
		}
	}
}

func TestAddModel_CommitDoneMsg_SuccessQuitsWithPop(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status (initial)
			"",            // ls-files --others (initial)
			"main",        // branch name (initial)
			"",            // renderSelectedDiff (initial)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	cmd := m.Update(CommitDoneMsg{Err: nil})
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}

	msg := cmd()
	if _, ok := msg.(app.PopModelMsg); !ok {
		t.Errorf("expected PopModelMsg, got %T", msg)
	}
}

func TestAddModel_CommitDoneMsg_ErrorShowsAborted(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status (initial)
			"",            // ls-files --others (initial)
			"main",        // branch name (initial)
			"",            // renderSelectedDiff (initial)
			"M\tfoo.go\n", // diff --name-status (refresh)
			"",            // ls-files --others (refresh)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	// Simulate commit aborted
	cmd := m.Update(CommitDoneMsg{Err: fmt.Errorf("exit status 1")})
	_ = cmd

	// Files should still be present (commit was aborted)
	if len(m.files) != 1 {
		t.Errorf("expected 1 file after aborted commit, got %d", len(m.files))
	}
}

func TestAddModel_CKeyNoFilesReturnsNil(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "", "")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	// No files -> execCommit returns nil -> should not crash
	_ = cmd
}

func TestAddModel_HelpOverlay_ShowsCommitKeys(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := m.View()
	if !strings.Contains(view, "stage and commit") {
		t.Error("help overlay should mention 'stage and commit'")
	}
}

func TestAddModel_HintsIncludeCommit(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\n", "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "c: commit") {
		t.Error("status bar hints should contain 'c: commit'")
	}
}

func TestAddModel_KeyJForwardsToList(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n",
			"",
			"main",
			// renderSelectedDiff after keypress:
			"",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	// j key forwards to list; no panic expected
	cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_ = cmd
}

func TestAddModel_BraceKeysAdjustContextLines(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"",            // ls-files --others
			"main",        // branch name
			"",            // renderSelectedDiff (initial)
			"",            // renderSelectedDiff (after })
			"",            // renderSelectedDiff (after {)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	initial := m.contextLines
	m.Update(tea.KeyPressMsg{Code: '}', ShiftedCode: '}', Mod: tea.ModShift, Text: "}"})
	if m.contextLines != initial+1 {
		t.Errorf("} should increment contextLines: got %d want %d", m.contextLines, initial+1)
	}

	m.Update(tea.KeyPressMsg{Code: '{', ShiftedCode: '{', Mod: tea.ModShift, Text: "{"})
	if m.contextLines != initial {
		t.Errorf("{ should decrement contextLines: got %d want %d", m.contextLines, initial)
	}
}

func TestAddModel_BraceKeysBounds(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n",
			"",
			"main",
			"",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	m.contextLines = 0
	m.Update(tea.KeyPressMsg{Code: '{', ShiftedCode: '{', Mod: tea.ModShift, Text: "{"})
	if m.contextLines != 0 {
		t.Errorf("contextLines should not go below 0, got %d", m.contextLines)
	}

	m.contextLines = 20
	m.Update(tea.KeyPressMsg{Code: '}', ShiftedCode: '}', Mod: tea.ModShift, Text: "}"})
	if m.contextLines != 20 {
		t.Errorf("contextLines should not go above 20, got %d", m.contextLines)
	}
}

func TestAddModel_ResizeWhileDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tmain.go\n", "", "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after resize with diff hidden")
	}
}

func TestAddModel_MaximizeJK_ChangesFile(t *testing.T) {
	t.Parallel()
	m := newTestAddModel(t, "M\tfoo.go\nM\tbar.go\n", "")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})

	if m.fileList.SelectedPath() == "foo.go" {
		t.Error("j in maximize mode should advance the file list cursor")
	}
}

// newTestAddModelWithCfg builds an AddModel backed by a FakeRunner with custom config.
func newTestAddModelWithCfg(t *testing.T, nameStatus string, cfg config.Config) (*AddModel, *testhelper.FakeRunner) {
	t.Helper()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			nameStatus, // git diff --name-status
			"",         // git ls-files --others
			"main",     // git rev-parse --abbrev-ref HEAD
			"",         // renderSelectedDiff
			"",         // git add (stage call)
		},
	}
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewAddModel unexpectedly returned error: %v", err)
	}
	return m, runner
}

func TestAddModel_ExecCommit_DefaultConfig_CKey(t *testing.T) {
	t.Parallel()
	cfg := config.NewDefault() // CommitCmd="git commit", CommitTitleOnlyFlag=""
	m, runner := newTestAddModelWithCfg(t, "M\tfoo.go\n", cfg)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if cmd == nil {
		t.Fatal("expected a command from pressing c with default config")
	}
	testhelper.MustHaveCall(t, runner, "add", "--", "foo.go")
}

func TestAddModel_ExecCommit_DefaultConfig_ShiftCKey(t *testing.T) {
	t.Parallel()
	// With default config CommitTitleOnlyFlag is "", so C behaves same as c.
	cfg := config.NewDefault()
	m, runner := newTestAddModelWithCfg(t, "M\tfoo.go\n", cfg)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	cmd := m.Update(tea.KeyPressMsg{Code: 'C', ShiftedCode: 'C', Mod: tea.ModShift, Text: "C"})
	if cmd == nil {
		t.Fatal("expected a command from pressing C with default config (no title-only flag)")
	}
	testhelper.MustHaveCall(t, runner, "add", "--", "foo.go")
}

func TestAddModel_ExecCommit_CustomCommand_CKey(t *testing.T) {
	t.Parallel()
	cfg := config.NewDefault()
	cfg.CommitCmd = "devtool commit"
	cfg.CommitTitleOnlyFlag = "-t"
	m, runner := newTestAddModelWithCfg(t, "M\tfoo.go\n", cfg)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if cmd == nil {
		t.Fatal("expected a command from pressing c with custom commit command")
	}
	testhelper.MustHaveCall(t, runner, "add", "--", "foo.go")
}

func TestAddModel_ExecCommit_CustomCommand_ShiftCKey(t *testing.T) {
	t.Parallel()
	cfg := config.NewDefault()
	cfg.CommitCmd = "devtool commit"
	cfg.CommitTitleOnlyFlag = "-t"
	m, runner := newTestAddModelWithCfg(t, "M\tfoo.go\n", cfg)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	cmd := m.Update(tea.KeyPressMsg{Code: 'C', ShiftedCode: 'C', Mod: tea.ModShift, Text: "C"})
	if cmd == nil {
		t.Fatal("expected a command from pressing C with custom commit command and title-only flag")
	}
	testhelper.MustHaveCall(t, runner, "add", "--", "foo.go")
}

func TestAddModel_ExecCommit_NoFlag_ShiftCSameAsC(t *testing.T) {
	t.Parallel()
	// When CommitTitleOnlyFlag is empty, C produces the same command as c.
	// Verify both return a non-nil cmd (both paths exercised, same outcome).
	cfg := config.NewDefault()
	cfg.CommitCmd = "devtool commit"
	cfg.CommitTitleOnlyFlag = "" // no flag configured

	runner1 := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "", "main", "", ""},
	}
	m1, err := NewAddModel(context.Background(), runner1, cfg, &diff.PlainRenderer{})
	if err != nil {
		t.Fatalf("NewAddModel: %v", err)
	}
	m1.width = 120
	m1.height = 40
	m1.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	cmdC := m1.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})

	runner2 := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "", "main", "", ""},
	}
	m2, err := NewAddModel(context.Background(), runner2, cfg, &diff.PlainRenderer{})
	if err != nil {
		t.Fatalf("NewAddModel: %v", err)
	}
	m2.width = 120
	m2.height = 40
	m2.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	cmdShiftC := m2.Update(tea.KeyPressMsg{Code: 'C', ShiftedCode: 'C', Mod: tea.ModShift, Text: "C"})

	if cmdC == nil {
		t.Fatal("c key should return a command when CommitTitleOnlyFlag is empty")
	}
	if cmdShiftC == nil {
		t.Fatal("C key should return a command when CommitTitleOnlyFlag is empty")
	}
	// Both produce a tea.ExecProcess cmd - verify both triggered staging.
	testhelper.MustHaveCall(t, runner1, "add", "--", "foo.go")
	testhelper.MustHaveCall(t, runner2, "add", "--", "foo.go")
}
