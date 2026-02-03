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
	if len(m.allHunks[0]) != 1 {
		t.Fatalf("expected 1 hunk in file 0, got %d", len(m.allHunks[0]))
	}
}

func TestNewHunkAddModel_TwoHunks(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, twoHunkDiff)
	if len(m.allHunks[0]) != 2 {
		t.Fatalf("expected 2 hunks in file 0, got %d", len(m.allHunks[0]))
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

	// Press Enter while help is visible — should NOT apply staged hunks
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(app.PopModelMsg); ok {
			t.Error("pressing Enter while help visible should not produce PopModelMsg")
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

func TestHunkAddModel_SpaceTogglesStagedInList(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	// Before: no hunks staged.
	if len(m.hunkList.StagedHunks()) != 0 {
		t.Fatal("no hunks should be staged initially")
	}

	// Press Space to stage the first hunk.
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	if len(m.hunkList.StagedHunks()) != 1 {
		t.Errorf("expected 1 staged hunk after Space, got %d", len(m.hunkList.StagedHunks()))
	}

	// Press Space again to unstage.
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	if len(m.hunkList.StagedHunks()) != 0 {
		t.Errorf("expected 0 staged hunks after second Space, got %d", len(m.hunkList.StagedHunks()))
	}
}

func TestHunkAddModel_EnterWithNoStagedHunks_DoesNotPop(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	// Nothing staged — Enter should not produce PopModelMsg.
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(app.PopModelMsg); ok {
			t.Error("Enter with no staged hunks should not produce PopModelMsg")
		}
	}
}

func TestHunkAddModel_SpaceThenEnter_StagesAndPops(t *testing.T) {
	t.Parallel()
	// Extra "" output for git apply --cached call.
	m, runner := newHunkAddTestModel(t, singleHunkDiff, "")
	m.width = 120
	m.height = 40

	// Stage the hunk.
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	// Apply via Enter.
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	testhelper.MustHaveCall(t, runner, "apply", "--cached")

	if cmd == nil {
		t.Fatal("expected a command after Enter with staged hunk")
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

func TestHunkAddModel_EnterWithNoFiles_DoesNotPop(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(app.PopModelMsg); ok {
			t.Error("Enter with no files should not produce PopModelMsg")
		}
	}
}

func TestHunkAddModel_SplitHunk(t *testing.T) {
	t.Parallel()
	// Diff with a single multi-group hunk.
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

	// Press 's' to split — should not panic.
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})

	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after split attempt")
	}
}

func TestHunkAddModel_SplitHunk_CannotSplit(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	before := len(m.allHunks[0])
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	after := len(m.allHunks[0])

	// Hunk count should be same (cannot split single-change hunk).
	if after != before {
		t.Errorf("hunk count changed from %d to %d, but hunk should not be splittable", before, after)
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

func TestHunkAddModel_View_HelpHasSpaceAndEnterBindings(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := m.View()
	if !strings.Contains(view, "Space") {
		t.Error("help overlay should contain 'Space' binding")
	}
	if !strings.Contains(view, "Enter") {
		t.Error("help overlay should contain 'Enter' binding")
	}
}

func TestHunkAddModel_HintsWithProgress_LeftPanel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	hints := m.hintsWithProgress()

	if !strings.Contains(hints, "Space: toggle") {
		t.Errorf("left panel hints should contain 'Space: toggle', got: %q", hints)
	}
	if !strings.Contains(hints, "Enter: apply") {
		t.Errorf("left panel hints should contain 'Enter: apply', got: %q", hints)
	}
	if strings.Contains(hints, "y: stage") {
		t.Errorf("left panel hints should NOT contain 'y: stage', got: %q", hints)
	}
	if strings.Contains(hints, "n: skip") {
		t.Errorf("left panel hints should NOT contain 'n: skip', got: %q", hints)
	}
	if strings.Contains(hints, "a: stage all") {
		t.Errorf("left panel hints should NOT contain 'a: stage all', got: %q", hints)
	}
}

func TestHunkAddModel_HintsWithProgress_RightPanel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.focusRight = true
	hints := m.hintsWithProgress()

	if !strings.Contains(hints, "Tab: panel") {
		t.Errorf("right panel hints should contain 'Tab: panel', got: %q", hints)
	}
}

func TestHunkAddModel_HintsWithProgress_Maximized(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.diffMaximized = true
	hints := m.hintsWithProgress()

	if !strings.Contains(hints, "F: restore") {
		t.Errorf("maximize hints should include 'F: restore', got %q", hints)
	}
}

func TestHunkAddModel_DKeyTogglesDiffPanel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	viewWith := m.View()

	// Press D to hide.
	m.Update(tea.KeyPressMsg{Code: 'D', ShiftedCode: 'D', Mod: tea.ModShift, Text: "D"})
	viewWithout := m.View()

	if viewWith == viewWithout {
		t.Error("View() should differ after D toggles diff off")
	}

	// Press D again to show.
	m.Update(tea.KeyPressMsg{Code: 'D', ShiftedCode: 'D', Mod: tea.ModShift, Text: "D"})
	viewAgain := m.View()

	if viewAgain == viewWithout {
		t.Error("View() should differ after D toggles diff back on")
	}
}

func TestHunkAddModel_TabNoopWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkDiff, "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
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

func TestHunkAddModel_SinglePanelViewWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkDiff, "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	if !strings.Contains(view, "foo.go") {
		t.Error("single-panel View() should still contain file name 'foo.go'")
	}
}

func TestHunkAddModel_MnemonicPrefix_FileTreeShowsCorrectNames(t *testing.T) {
	t.Parallel()
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
	if m.files[0].DisplayPath() != ".gitconfig" {
		t.Errorf("file DisplayPath() = %q, want %q", m.files[0].DisplayPath(), ".gitconfig")
	}

	view := m.View()
	if !strings.Contains(view, ".gitconfig") {
		t.Errorf("View() should contain '.gitconfig', got:\n%s", view)
	}
}

func TestHunkAddModel_NoPrefixFormat_FileTreeShowsCorrectNames(t *testing.T) {
	t.Parallel()
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

func TestHunkAddModel_FKeyTogglesDiffMaximized(t *testing.T) {
	t.Parallel()
	hunkBody := "@@ -1,3 +1,3 @@\n context\n-old\n+new\n context\n"
	diffOutput := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n" + hunkBody
	m, _ := newHunkAddTestModel(t, diffOutput)
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

func TestHunkAddModel_WKeyTogglesSoftWrap(t *testing.T) {
	t.Parallel()
	hunkBody := "@@ -1,3 +1,3 @@\n context\n-old\n+new\n context\n"
	diffOutput := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n" + hunkBody
	m, _ := newHunkAddTestModel(t, diffOutput)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	initial := m.diffView.SoftWrap()
	m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if m.diffView.SoftWrap() == initial {
		t.Error("w should toggle soft-wrap when right panel focused")
	}
}

func TestHunkAddModel_BracketKeysAdjustPanelRatio(t *testing.T) {
	t.Parallel()
	hunkBody := "@@ -1,3 +1,3 @@\n context\n-old\n+new\n context\n"
	diffOutput := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n" + hunkBody
	m, _ := newHunkAddTestModel(t, diffOutput)
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

func TestHunkAddModel_MaximizeView(t *testing.T) {
	t.Parallel()
	hunkBody := "@@ -1,3 +1,3 @@\n context\n-old\n+new\n context\n"
	diffOutput := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n" + hunkBody
	m, _ := newHunkAddTestModel(t, diffOutput)
	m.width = 120
	m.height = 40

	normalView := m.View()
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	maximizedView := m.View()
	if normalView == maximizedView {
		t.Error("maximize mode should produce a different view")
	}
}

func TestHunkAddModel_ResizeWhileMaximized(t *testing.T) {
	t.Parallel()
	hunkBody := "@@ -1,3 +1,3 @@\n context\n-old\n+new\n context\n"
	diffOutput := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n" + hunkBody
	m, _ := newHunkAddTestModel(t, diffOutput)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	_ = cmd
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after resize while maximized")
	}
}

func TestHunkAddModel_SpaceNoopOnRightPanel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40
	m.focusRight = true

	before := len(m.hunkList.StagedHunks())
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	after := len(m.hunkList.StagedHunks())

	// Space on right panel should not toggle hunk.
	if after != before {
		t.Errorf("Space on right panel should not change staged count: before=%d after=%d", before, after)
	}
}

func TestHunkAddModel_ApplyAllStaged_ErrorPath(t *testing.T) {
	t.Parallel()
	applyErr := fmt.Errorf("apply failed")
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkDiff, "main", ""},
		Errors:  []error{nil, nil, applyErr},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Stage the hunk.
	m.hunkList.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	// Apply should fail but still pop (with MutatedGit=false since none succeeded).
	cmd := m.applyAllStaged()
	if cmd == nil {
		t.Fatal("expected cmd from applyAllStaged even on error")
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

func TestHunkAddModel_TwoFiles_SpaceOnBothThenEnter(t *testing.T) {
	t.Parallel()
	m, runner := newHunkAddTestModel(t, twoFileDiff, "", "")
	m.width = 120
	m.height = 40

	if len(m.files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(m.files))
	}

	// Stage hunk in file 0.
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	// Move to file 1's hunk.
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Stage hunk in file 1.
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	if len(m.hunkList.StagedHunks()) != 2 {
		t.Fatalf("expected 2 staged hunks, got %d", len(m.hunkList.StagedHunks()))
	}

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	testhelper.MustHaveCall(t, runner, "apply", "--cached")

	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after staging both files")
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

func TestHunkAddModel_RenderCurrentHunk_NoHunks(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	// allHunks is empty; renderCurrentHunk should not panic
	m.renderCurrentHunk()
	view := m.View()
	_ = view // just ensure no panic
}

func TestHunkAddModel_RenderCurrentHunk_Renderer_FallbackOnError(t *testing.T) {
	t.Parallel()
	// Use a renderer that always fails to cover the err != nil branch.
	runner := &testhelper.FakeRunner{Outputs: []string{singleHunkDiff, "main"}}
	cfg := config.NewDefault()
	m := NewHunkAddModel(context.Background(), runner, cfg, &errRenderer{})
	m.width = 120
	m.height = 40
	// renderCurrentHunk was called in constructor; should have fallen back to raw
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty even when renderer fails")
	}
}

// errRenderer always returns an error from Render.
type errRenderer struct{}

func (e *errRenderer) Render(_ string) (string, error) {
	return "", fmt.Errorf("render error")
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
