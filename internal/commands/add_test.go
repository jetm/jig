package commands

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/testhelper"
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
	return NewAddModel(context.Background(), runner, cfg, renderer)
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
	m := NewAddModel(context.Background(), runner, cfg, renderer)
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
	m := NewAddModel(context.Background(), runner, cfg, renderer)
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
	m := NewAddModel(context.Background(), runner, cfg, renderer)
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
	testhelper.MustHaveCall(t, runner, "diff", "--no-index", "--", "/dev/null", "newfile.go")
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
	m := NewAddModel(context.Background(), runner, cfg, renderer)
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
		t.Error("MutatedGit should be false when staging fails")
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
	m := NewAddModel(context.Background(), runner, cfg, renderer)
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
	m := NewAddModel(context.Background(), runner, cfg, renderer)
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
	m := NewAddModel(context.Background(), runner, cfg, renderer)

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
	m := NewAddModel(context.Background(), runner, cfg, renderer)
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
	m := NewAddModel(context.Background(), runner, cfg, renderer)
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
	m := NewAddModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// j key forwards to list; no panic expected
	cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_ = cmd
}
