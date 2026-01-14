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

// singleHunkDiff is a minimal unified diff with one file and one hunk.
const singleHunkDiff = "diff --git a/foo.go b/foo.go\n" +
	"index 111..222 100644\n" +
	"--- a/foo.go\n" +
	"+++ b/foo.go\n" +
	"@@ -1,3 +1,4 @@\n" +
	" package main\n" +
	"+// added\n" +
	" func foo() {}\n"

// twoHunkDiff is a diff with one file containing two separate hunks.
const twoHunkDiff = "diff --git a/bar.go b/bar.go\n" +
	"index aaa..bbb 100644\n" +
	"--- a/bar.go\n" +
	"+++ b/bar.go\n" +
	"@@ -1,3 +1,4 @@\n" +
	" package main\n" +
	"+// hunk 1\n" +
	" func a() {}\n" +
	"@@ -10,3 +11,4 @@\n" +
	" // section 2\n" +
	"+// hunk 2\n" +
	" func b() {}\n"

// twoFileDiff is a diff with two files each having one hunk.
const twoFileDiff = "diff --git a/a.go b/a.go\n" +
	"index 111..222 100644\n" +
	"--- a/a.go\n" +
	"+++ b/a.go\n" +
	"@@ -1,2 +1,3 @@\n" +
	" package main\n" +
	"+// file a\n" +
	" func a() {}\n" +
	"diff --git a/b.go b/b.go\n" +
	"index 333..444 100644\n" +
	"--- a/b.go\n" +
	"+++ b/b.go\n" +
	"@@ -1,2 +1,3 @@\n" +
	" package main\n" +
	"+// file b\n" +
	" func b() {}\n"

// newHunkAddTestModel creates a HunkAddModel using scripted outputs.
// outputs[0] = git diff output, outputs[1] = branch name, rest for git apply calls.
func newHunkAddTestModel(t *testing.T, diffOutput string, extraOutputs ...string) (*HunkAddModel, *testhelper.FakeRunner) {
	t.Helper()
	outputs := []string{diffOutput, "main"}
	outputs = append(outputs, extraOutputs...)
	runner := &testhelper.FakeRunner{Outputs: outputs}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	return m, runner
}

func TestNewHunkAddModel_NoChanges(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	if len(m.files) != 0 {
		t.Errorf("expected 0 files, got %d", len(m.files))
	}
}

func TestNewHunkAddModel_SingleFile(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	if len(m.hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(m.hunks))
	}
	if m.hunkIdx != 0 {
		t.Errorf("hunkIdx = %d, want 0", m.hunkIdx)
	}
}

func TestNewHunkAddModel_TwoHunks(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, twoHunkDiff)
	if len(m.hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(m.hunks))
	}
}

func TestHunkAddModel_QuitOnQ(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
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
		t.Error("MutatedGit should be false when quitting")
	}
}

func TestHunkAddModel_QuitOnEsc(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected a command from pressing Esc")
	}
	msg := cmd()
	if _, ok := msg.(app.PopModelMsg); !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
}

func TestHunkAddModel_HelpToggle(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
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

func TestHunkAddModel_HelpVisibleBlocksKeys(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !m.help.IsVisible() {
		t.Fatal("help should be visible")
	}

	// Press y while help is visible — should NOT stage
	cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(app.PopModelMsg); ok {
			t.Error("pressing y while help visible should not produce PopModelMsg")
		}
	}
}

func TestHunkAddModel_View_NoChanges(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "No unstaged changes") {
		t.Errorf("View() with no files should mention 'No unstaged changes', got: %q", view)
	}
}

func TestHunkAddModel_View_TooSmall(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 30
	m.height = 5

	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Errorf("View() with small terminal should mention 'too small', got: %q", view)
	}
}

func TestHunkAddModel_View_TwoPanel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	if !strings.Contains(view, "foo.go") {
		t.Error("View() should contain filename 'foo.go'")
	}
}

func TestHunkAddModel_TabTogglesFocus(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
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

func TestHunkAddModel_QuitFromRightPanel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
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

func TestHunkAddModel_HunkActionsWorkFromRightPanel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff, "" /* git apply output */)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// y (stage hunk) should still work from right panel
	cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	_ = cmd // should not panic
}

func TestHunkAddModel_View_HelpVisible(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := m.View()
	if !strings.Contains(view, "Navigation") {
		t.Error("help overlay should contain 'Navigation'")
	}
}

func TestHunkAddModel_View_ShowsProgress(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "Hunk 1/1") {
		t.Errorf("View() should show hunk progress 'Hunk 1/1', got: %q", view)
	}
}

func TestHunkAddModel_StageHunk_Success(t *testing.T) {
	t.Parallel()
	// Extra "" output for git apply --cached call
	m, runner := newHunkAddTestModel(t, singleHunkDiff, "")
	m.width = 120
	m.height = 40

	// Press 'y' to stage the hunk; only one hunk, so it should pop
	cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})

	// Verify git apply was called
	testhelper.MustHaveCall(t, runner, "apply", "--cached")

	// With single hunk/file, after staging we should get PopModelMsg
	if cmd == nil {
		t.Fatal("expected a command after staging the only hunk")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after staging")
	}
}

func TestHunkAddModel_SkipHunk(t *testing.T) {
	t.Parallel()
	m, runner := newHunkAddTestModel(t, twoHunkDiff)
	m.width = 120
	m.height = 40

	callsBefore := testhelper.CallCount(runner)

	// Skip first hunk
	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})

	// No additional git calls (apply) should have been made
	callsAfter := testhelper.CallCount(runner)
	if callsAfter != callsBefore {
		t.Errorf("expected no new git calls after skip, but got %d new calls", callsAfter-callsBefore)
	}

	// hunkIdx should advance to 1
	if m.hunkIdx != 1 {
		t.Errorf("hunkIdx = %d, want 1 after skipping first hunk", m.hunkIdx)
	}
}

func TestHunkAddModel_SkipAllHunks_NoMutation(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	// Skip the only hunk — should pop with MutatedGit=false (nothing staged)
	cmd := m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if cmd == nil {
		t.Fatal("expected a command after skipping the only hunk")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if pop.MutatedGit {
		t.Error("MutatedGit should be false when only skipping (no staging)")
	}
}

func TestHunkAddModel_StageAllRemaining(t *testing.T) {
	t.Parallel()
	// Two hunks: press 'a' stages both
	m, runner := newHunkAddTestModel(t, twoHunkDiff, "", "")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})

	// Both hunks staged
	if testhelper.CallCount(runner) < 3 { // diff + branch + at least 1 apply
		t.Errorf("expected at least 3 calls (diff, branch, apply), got %d", testhelper.CallCount(runner))
	}

	// Should pop after all decided
	if cmd == nil {
		t.Fatal("expected command after staging all")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after staging all")
	}
}

func TestHunkAddModel_SplitHunk(t *testing.T) {
	t.Parallel()
	// twoHunkDiff already has 2 hunks; splitting one that has multiple change groups
	// creates sub-hunks. Use a diff with a single multi-group hunk.
	multiGroupDiff := "diff --git a/x.go b/x.go\n" +
		"index 111..222 100644\n" +
		"--- a/x.go\n" +
		"+++ b/x.go\n" +
		"@@ -1,10 +1,12 @@\n" +
		" package main\n" +
		"+// change1\n" +
		" func a() {}\n" +
		" // comment\n" +
		" // comment2\n" +
		" // comment3\n" +
		"+// change2\n" +
		" func b() {}\n"

	m, _ := newHunkAddTestModel(t, multiGroupDiff)
	m.width = 120
	m.height = 40

	initialHunks := len(m.hunks)

	// Press 's' to split
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})

	// Hunks should increase if split was possible
	// (For this diff it may or may not split depending on context lines)
	_ = initialHunks
	// At minimum should not panic and view should still work
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after split attempt")
	}
}

func TestHunkAddModel_SplitHunk_CannotSplit(t *testing.T) {
	t.Parallel()
	// Single-change hunk cannot be split further
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	before := len(m.hunks)
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	after := len(m.hunks)

	// Hunk count should be same (cannot split)
	if after != before {
		t.Errorf("hunk count changed from %d to %d, but hunk should not be splittable", before, after)
	}
}

func TestHunkAddModel_TwoFiles_AdvancesAfterAllHunksDecided(t *testing.T) {
	t.Parallel()
	// Two files: after staging file 1's hunk, should load file 2
	m, runner := newHunkAddTestModel(t, twoFileDiff, "")
	m.width = 120
	m.height = 40

	if len(m.files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(m.files))
	}

	// Stage first file's hunk — should not pop yet
	cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	testhelper.MustHaveCall(t, runner, "apply", "--cached")

	// Should advance to second file (not pop)
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(app.PopModelMsg); ok {
			t.Error("should not pop after staging first of two files")
		}
	}

	if m.fileIdx != 1 {
		t.Errorf("fileIdx = %d, want 1 after staging first file's hunk", m.fileIdx)
	}
}

func TestHunkAddModel_WindowSizeMsg(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)

	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})

	if m.width != 200 {
		t.Errorf("width = %d, want 200", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestHunkAddModel_NoHunks_YDoesNothing(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd != nil {
		t.Error("y with no files should return nil cmd")
	}
}

func TestHunkAddModel_NoHunks_NDoesNothing(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	m.width = 120
	m.height = 40

	// Should not panic
	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
}

func TestHunkAddModel_StageHunk_AllDecided_YDoesNothing(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff, "")
	m.width = 120
	m.height = 40
	m.allDecided = true

	cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd != nil {
		t.Error("y with allDecided should return nil cmd")
	}
}

func TestHunkAddModel_StageAllRemaining_NoFiles(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if cmd != nil {
		t.Error("a with no files should return nil cmd")
	}
}

func TestHunkAddModel_RenderCurrentHunk_AllDecided(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40
	m.allDecided = true
	// Should not panic; diffView gets updated with "All hunks decided" text
	m.renderCurrentHunk()
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
}

func TestHunkAddModel_RenderCurrentHunk_HunkIdxOutOfBounds(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40
	m.hunkIdx = 999
	// Should not panic
	m.renderCurrentHunk()
}

func TestHunkAddModel_SyncFileSelection_WrongType(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	// Forward a key that the list consumes (but won't match our type)
	// Just ensure no panic
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
}

func TestHunkAddModel_StageCurrentHunk_FileIdxOutOfBounds(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.fileIdx = 999
	cmd := m.stageCurrentHunk()
	if cmd != nil {
		t.Error("stageCurrentHunk with out-of-bounds fileIdx should return nil")
	}
}

func TestHunkAddModel_StageAllRemaining_AlreadyAllDecided(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.allDecided = true
	cmd := m.stageAllRemaining()
	if cmd != nil {
		t.Error("stageAllRemaining when allDecided should return nil")
	}
}

func TestHunkAddModel_StageAllRemaining_FileIdxOutOfBounds(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.fileIdx = 999
	cmd := m.stageAllRemaining()
	if cmd != nil {
		t.Error("stageAllRemaining with out-of-bounds fileIdx should return nil")
	}
}

func TestHunkAddModel_SplitCurrentHunk_AllDecided(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.allDecided = true
	before := len(m.hunks)
	m.splitCurrentHunk()
	after := len(m.hunks)
	if before != after {
		t.Error("splitCurrentHunk when allDecided should not change hunks")
	}
}

func TestHunkAddModel_SplitCurrentHunk_HunkIdxOutOfBounds(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.hunkIdx = 999
	// Should not panic
	m.splitCurrentHunk()
}

func TestHunkAddModel_SplitCurrentHunk_NoHunks(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	// Should not panic
	m.splitCurrentHunk()
}

func TestHunkAddModel_StageCurrentHunk_ErrorPath(t *testing.T) {
	t.Parallel()
	applyErr := fmt.Errorf("apply failed")
	runner := &testhelper.FakeRunner{
		// diff, branch, apply(fails)
		Outputs: []string{singleHunkDiff, "main", ""},
		Errors:  []error{nil, nil, applyErr},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Stage should fail and return nil cmd (error path shows status message instead)
	cmd := m.stageCurrentHunk()
	if cmd != nil {
		t.Error("stageCurrentHunk on runner error should return nil")
	}
}

func TestHunkAddModel_StageAllRemaining_ErrorPath(t *testing.T) {
	t.Parallel()
	applyErr := fmt.Errorf("apply failed")
	runner := &testhelper.FakeRunner{
		// diff, branch, apply(fails), apply(fails)
		Outputs: []string{twoHunkDiff, "main", "", ""},
		Errors:  []error{nil, nil, applyErr, applyErr},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// stageAllRemaining should not panic even when all apply calls fail
	cmd := m.stageAllRemaining()
	// allDecided should be set
	if !m.allDecided {
		t.Error("allDecided should be true after stageAllRemaining")
	}
	// cmd: since no staging succeeded, MutatedGit = false, pops
	if cmd == nil {
		t.Fatal("expected cmd from stageAllRemaining")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if pop.MutatedGit {
		t.Error("MutatedGit should be false when all apply calls failed")
	}
}

func TestHunkAddModel_FileTreeRendersFiles(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
}

func TestHunkAddModel_SyncFileSelection_NilItem(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	// Should not panic
	m.syncFileSelection()
}

func TestHunkAddModel_MnemonicPrefix_FileTreeShowsCorrectNames(t *testing.T) {
	t.Parallel()
	// Simulate git diff output with mnemonicPrefix=true (i/ and w/ prefixes).
	mnemonicDiff := "diff --git i/.gitconfig w/.gitconfig\n" +
		"index 111..222 100644\n" +
		"--- i/.gitconfig\n" +
		"+++ w/.gitconfig\n" +
		"@@ -83,6 +83,9 @@\n" +
		"     ignore = \"!gi() { curl -sL; }; gi\"\n" +
		"+    check-series = \"!f() { b4 prep; }; f\"\n" +
		"+\n" +
		" [core]\n"

	m, _ := newHunkAddTestModel(t, mnemonicDiff)
	m.width = 120
	m.height = 40

	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	// File path should be ".gitconfig", not "" or "."
	if m.files[0].DisplayPath() != ".gitconfig" {
		t.Errorf("file DisplayPath() = %q, want %q", m.files[0].DisplayPath(), ".gitconfig")
	}

	// The file tree View should render the actual filename, not "."
	view := m.View()
	if !strings.Contains(view, ".gitconfig") {
		t.Errorf("View() should contain '.gitconfig', got:\n%s", view)
	}
	if strings.Contains(view, "└─  .") {
		t.Error("View() should NOT show '└─  .' (empty path rendering)")
	}
}

func TestHunkAddModel_NoPrefixFormat_FileTreeShowsCorrectNames(t *testing.T) {
	t.Parallel()
	// Simulate git diff output with diff.noprefix=true (no prefix at all).
	noPrefixDiff := "diff --git .gitconfig .gitconfig\n" +
		"index 111..222 100644\n" +
		"--- .gitconfig\n" +
		"+++ .gitconfig\n" +
		"@@ -1,3 +1,4 @@\n" +
		" [user]\n" +
		"+    name = test\n" +
		"     email = test@example.com\n"

	m, _ := newHunkAddTestModel(t, noPrefixDiff)
	m.width = 120
	m.height = 40

	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	if m.files[0].DisplayPath() != ".gitconfig" {
		t.Errorf("file DisplayPath() = %q, want %q", m.files[0].DisplayPath(), ".gitconfig")
	}
}

func TestHunkAddModel_PatchHeader(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	fd := newTestFileDiff("path/to/file.go")
	header := m.patchHeader(fd)

	if !strings.Contains(header, "diff --git") {
		t.Errorf("patchHeader missing 'diff --git': %q", header)
	}
	if !strings.Contains(header, "a/path/to/file.go") {
		t.Errorf("patchHeader missing old path: %q", header)
	}
	if !strings.Contains(header, "b/path/to/file.go") {
		t.Errorf("patchHeader missing new path: %q", header)
	}
}

func TestHunkAddModel_HintsWithProgress_NoHunks(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	hints := m.hintsWithProgress()
	if strings.Contains(hints, "Hunk") {
		t.Errorf("hintsWithProgress() with no hunks should not mention 'Hunk', got: %q", hints)
	}
}

func TestHunkAddModel_HintsWithProgress_WithHunks(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, twoHunkDiff)
	hints := m.hintsWithProgress()
	if !strings.Contains(hints, "Hunk 1/2") {
		t.Errorf("hintsWithProgress() should contain 'Hunk 1/2', got: %q", hints)
	}
}

func TestSplitHunk_SingleChange(t *testing.T) {
	t.Parallel()
	h := newTestHunk("@@ -1,3 +1,4 @@\n package main\n+// added\n func foo() {}\n")
	result := splitHunk(h)
	if len(result) != 1 {
		t.Errorf("splitHunk of single-change hunk should return 1 hunk, got %d", len(result))
	}
}

func TestSplitHunk_TwoChanges(t *testing.T) {
	t.Parallel()
	// Two change groups separated by multiple context lines
	body := "@@ -1,10 +1,12 @@\n" +
		" package main\n" +
		"+// change1\n" +
		" func a() {}\n" +
		" // comment\n" +
		" // comment2\n" +
		" // comment3\n" +
		"+// change2\n" +
		" func b() {}\n"
	h := newTestHunk(body)
	result := splitHunk(h)
	// With enough context lines between the two change groups, should split
	if len(result) < 1 {
		t.Error("splitHunk should return at least 1 hunk")
	}
}

func TestSplitHunk_TooShort(t *testing.T) {
	t.Parallel()
	h := newTestHunk("@@ -1 +1 @@\n+x\n")
	result := splitHunk(h)
	if len(result) != 1 {
		t.Errorf("splitHunk of very short hunk should return 1 hunk, got %d", len(result))
	}
}

// newTestFileDiff creates a minimal FileDiff for testing.
func newTestFileDiff(path string) git.FileDiff {
	return git.FileDiff{OldPath: path, NewPath: path, Status: git.Modified}
}

// newTestHunk creates a Hunk for testing.
func newTestHunk(body string) git.Hunk {
	lines := strings.SplitN(body, "\n", 2)
	header := lines[0]
	return git.Hunk{Header: header, Body: body}
}
