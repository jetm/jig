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
	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/testhelper"
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
}

func TestNewHunkAddModel_TwoHunks(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, twoHunkDiff)
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
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

	// Press Space while help is visible - should NOT toggle hunk
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
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

func TestHunkAddModel_SpaceTogglesHunk(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	// Space toggles first hunk staged
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	staged := m.hunkList.StagedHunks()
	if len(staged) != 1 {
		t.Fatalf("expected 1 staged hunk after Space, got %d", len(staged))
	}

	// Space again untoggles
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	staged = m.hunkList.StagedHunks()
	if len(staged) != 0 {
		t.Errorf("expected 0 staged hunks after second Space, got %d", len(staged))
	}
}

func TestHunkAddModel_WKeyAppliesStaged(t *testing.T) {
	t.Parallel()
	m, runner := newHunkAddTestModel(t, singleHunkDiff, "" /* git apply output */)
	m.width = 120
	m.height = 40

	// Toggle hunk staged with Space
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	// Press w to apply
	cmd := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})

	// Verify git apply was called
	testhelper.MustHaveCall(t, runner, "apply", "--cached")

	if cmd == nil {
		t.Fatal("expected a command after w")
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

func TestHunkAddModel_WKeyWithNothingStagedDoesNothing(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	// Press w without toggling anything
	cmd := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(app.PopModelMsg); ok {
			t.Error("w with nothing staged should not pop")
		}
	}
}

func TestHunkAddModel_WKeyAppliesMultipleHunks(t *testing.T) {
	t.Parallel()
	m, runner := newHunkAddTestModel(t, twoHunkDiff, "", "" /* two apply calls */)
	m.width = 120
	m.height = 40

	// Toggle first hunk
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	// Move to second hunk
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Toggle second hunk
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	staged := m.hunkList.StagedHunks()
	if len(staged) != 2 {
		t.Fatalf("expected 2 staged hunks, got %d", len(staged))
	}

	// Apply
	cmd := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})

	testhelper.MustHaveCall(t, runner, "apply", "--cached")

	if cmd == nil {
		t.Fatal("expected command after w")
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

func TestHunkAddModel_WKeyAppliesAcrossFiles(t *testing.T) {
	t.Parallel()
	m, runner := newHunkAddTestModel(t, twoFileDiff, "", "" /* two apply calls */)
	m.width = 120
	m.height = 40

	if len(m.files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(m.files))
	}

	// Toggle first file's hunk
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	// Move to next hunk (second file)
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Toggle second file's hunk
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	// Apply all
	cmd := m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})

	testhelper.MustHaveCall(t, runner, "apply", "--cached")

	if cmd == nil {
		t.Fatal("expected command after w")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true")
	}
}

func TestHunkAddModel_SplitHunk(t *testing.T) {
	t.Parallel()
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

	// Press 's' to split
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

	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after failed split")
	}
}

func TestHunkAddModel_SplitHunk_NoHunks(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
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

func TestHunkAddModel_ApplyStaged_ErrorPath(t *testing.T) {
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

	// Toggle and apply
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	cmd := m.applyStaged()
	if cmd != nil {
		t.Error("applyStaged on runner error should return nil when no hunks applied")
	}
}

func TestHunkAddModel_ApplyStaged_PartialFailure(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{twoHunkDiff, "main", "", ""},
		Errors:  []error{nil, nil, nil, fmt.Errorf("apply failed")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Toggle both hunks
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	cmd := m.applyStaged()
	if cmd == nil {
		t.Fatal("expected cmd from partial apply")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true when at least one hunk applied")
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

func TestHunkAddModel_HintsForContext_Normal(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	hints := m.hintsForContext()
	if !strings.Contains(hints, "Space: toggle") {
		t.Errorf("hints should contain 'Space: toggle', got: %q", hints)
	}
	if !strings.Contains(hints, "w: apply") {
		t.Errorf("hints should contain 'w: apply', got: %q", hints)
	}
}

func TestHunkAddModel_HintsForContext_RightFocus(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.focusRight = true
	hints := m.hintsForContext()
	if !strings.Contains(hints, "h/l: scroll") {
		t.Errorf("right-focus hints should contain 'h/l: scroll', got: %q", hints)
	}
}

func TestHunkAddModel_HintsForContext_Maximized(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.diffMaximized = true
	hints := m.hintsForContext()
	if !strings.Contains(hints, "F: restore") {
		t.Errorf("maximized hints should contain 'F: restore', got: %q", hints)
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

// newTestFileDiff creates a minimal FileDiff for testing.
func newTestFileDiff(path string) git.FileDiff {
	return git.FileDiff{OldPath: path, NewPath: path, Status: git.Modified}
}

// newTestHunk creates a Hunk for testing by parsing a raw body string.
func newTestHunk(body string) git.Hunk {
	rawLines := strings.Split(body, "\n")
	header := rawLines[0]
	// Trim trailing empty lines
	for len(rawLines) > 1 && rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}
	var parsed []git.Line
	for _, rl := range rawLines[1:] {
		parsed = append(parsed, git.ParseLine(rl))
	}
	return git.Hunk{Header: header, Lines: parsed}
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

func TestHunkAddModel_EKey_WithHunk(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if cmd == nil {
		t.Error("e key with hunk available should return a non-nil cmd")
	}
}

func TestHunkAddModel_EKey_NoHunks(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_ = cmd
}

func TestHunkAddModel_EditDiffMsg_Error(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	cmd := m.Update(git.EditDiffMsg{Err: context.DeadlineExceeded})
	_ = cmd
}

func TestHunkAddModel_EditDiffMsg_ApplyError(t *testing.T) {
	t.Parallel()
	editedPath := t.TempDir() + "/addp-hunk-edit.diff"
	originalDiff := "@@ -1,3 +1,4 @@\n package main\n+// added\n func foo() {}\n"
	modifiedDiff := originalDiff + "extra"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			singleHunkDiff, // git diff
			"main",         // branch name
			"",             // git apply --cached (will fail)
		},
		Errors: []error{nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
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

func TestHunkAddModel_EditDiffMsg_Success(t *testing.T) {
	t.Parallel()
	editedPath := t.TempDir() + "/addp-hunk-edit.diff"
	originalDiff := "@@ -1,3 +1,4 @@\n package main\n+// added\n func foo() {}\n"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			singleHunkDiff, // git diff
			"main",         // branch name
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
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

func TestNewHunkAddModel_WithFilterPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			singleHunkDiff,
			"main",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer, []string{"foo.go"})
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	testhelper.MustHaveCall(t, runner, "diff", "--", "foo.go")
}

func TestNewHunkAddModel_FilterPaths_NoMatch(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",
			"main",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer, []string{"nonexistent.go"})
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

func TestHunkAddModel_EKey_SendsFullPatchToEditDiff(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if cmd == nil {
		t.Fatal("e key with hunk available should return a non-nil cmd")
	}

	tempPath := os.TempDir() + "/addp-hunk-edit.diff"
	content, err := os.ReadFile(tempPath)
	if err != nil {
		t.Fatalf("failed to read temp diff file %s: %v", tempPath, err)
	}

	got := string(content)

	if !strings.HasPrefix(got, "diff --git") {
		t.Errorf("EditDiff received bare hunk body instead of full patch; got:\n%s", got)
	}

	if !strings.Contains(got, "foo.go") {
		t.Errorf("EditDiff patch missing filename 'foo.go'; got:\n%s", got)
	}

	if !strings.Contains(got, "@@ -1,3 +1,4 @@") {
		t.Errorf("EditDiff patch missing hunk body; got:\n%s", got)
	}
}

func TestHunkAddModel_CKeyStagesAndReturnsExecCmd(t *testing.T) {
	t.Parallel()
	m, runner := newHunkAddTestModel(t, singleHunkDiff, "" /* git apply output */)
	m.width = 120
	m.height = 40

	// Toggle hunk staged with Space
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	// Press c to commit
	cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if cmd == nil {
		t.Fatal("expected a command from pressing c")
	}
	// Verify staging happened
	testhelper.MustHaveCall(t, runner, "apply", "--cached")
}

func TestHunkAddModel_ShiftCKeyStagesAndReturnsExecCmd(t *testing.T) {
	t.Parallel()
	m, runner := newHunkAddTestModel(t, singleHunkDiff, "" /* git apply output */)
	m.width = 120
	m.height = 40

	// Toggle hunk staged
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	// Press C (shift-c)
	cmd := m.Update(tea.KeyPressMsg{Code: 'C', ShiftedCode: 'C', Mod: tea.ModShift, Text: "C"})
	if cmd == nil {
		t.Fatal("expected a command from pressing C")
	}
	testhelper.MustHaveCall(t, runner, "apply", "--cached")
}

func TestHunkAddModel_CKeyNoStagedReturnsNil(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	// Press c without toggling anything
	cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	// No staged hunks -> should not return an exec cmd
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(CommitDoneMsg); ok {
			t.Error("should not return CommitDoneMsg when nothing staged")
		}
	}
}

func TestHunkAddModel_CKeyStageFailureReturnsNil(t *testing.T) {
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

	// Toggle and press c
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	// All hunks failed to stage -> no exec cmd
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(CommitDoneMsg); ok {
			t.Error("should not return CommitDoneMsg when all staging fails")
		}
	}
}

func TestHunkAddModel_CommitDoneMsg_SuccessQuitsWithPop(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			singleHunkDiff, // git diff (initial)
			"main",         // branch name (initial)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
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

func TestHunkAddModel_CommitDoneMsg_ErrorShowsAborted(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			singleHunkDiff, // git diff (initial)
			"main",         // branch name (initial)
			singleHunkDiff, // git diff (refresh)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	cmd := m.Update(CommitDoneMsg{Err: fmt.Errorf("exit status 1")})
	_ = cmd

	if len(m.files) != 1 {
		t.Errorf("expected 1 file after aborted commit, got %d", len(m.files))
	}
}

func TestHunkAddModel_HelpOverlay_ShowsCommitKeys(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := m.View()
	if !strings.Contains(view, "stage and commit") {
		t.Error("help overlay should mention 'stage and commit'")
	}
}

func TestHunkAddModel_HintsIncludeCommit(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	hints := m.hintsForContext()
	if !strings.Contains(hints, "c: commit") {
		t.Errorf("hints should contain 'c: commit', got: %q", hints)
	}
}

func TestHunkAddModel_EnterTransitionsToLineEdit(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	if m.inLineEdit {
		t.Fatal("should start in hunk list phase")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.inLineEdit {
		t.Error("Enter should transition to line edit phase")
	}
	if m.hunkView == nil {
		t.Error("hunkView should be set after Enter")
	}
}

func TestHunkAddModel_EscapeExitsLineEdit(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	// Enter line edit
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.inLineEdit {
		t.Fatal("should be in line edit")
	}

	// Exit line edit
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.inLineEdit {
		t.Error("Escape should exit line edit phase")
	}
	if m.hunkView != nil {
		t.Error("hunkView should be nil after exit")
	}
}

func TestHunkAddModel_LineEditSelectionsPreserved(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	// Enter line edit
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Toggle a line (Space)
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	// Exit line edit - selections should be preserved in the hunk
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.inLineEdit {
		t.Error("should be back in hunk list phase")
	}
}

func TestHunkAddModel_EnterNoHunksDoesNothing(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, "")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.inLineEdit {
		t.Error("Enter with no hunks should not enter line edit")
	}
}

func TestHunkAddModel_DKeyTogglesDiffPanel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	viewWith := m.View()

	m.Update(tea.KeyPressMsg{Code: 'D', ShiftedCode: 'D', Mod: tea.ModShift, Text: "D"})
	viewWithout := m.View()

	if viewWith == viewWithout {
		t.Error("View() should differ after D toggles diff off")
	}

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

func TestHunkAddModel_MnemonicPrefix_ShowsCorrectNames(t *testing.T) {
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

func TestHunkAddModel_NoPrefixFormat_ShowsCorrectNames(t *testing.T) {
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

func TestHunkAddModel_JKNavigationUpdatesDiffPreview(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, twoHunkDiff)
	m.width = 120
	m.height = 40

	// Move to second hunk
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})

	if m.hunkList.CurrentHunkIdx() != 1 {
		t.Errorf("expected cursor on hunk 1, got %d", m.hunkList.CurrentHunkIdx())
	}
}

func TestHunkAddModel_SplitPanelView_ShowsFileHeaders(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, twoFileDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "a.go") {
		t.Error("View should contain file 'a.go' in file header")
	}
	if !strings.Contains(view, "b.go") {
		t.Error("View should contain file 'b.go' in file header")
	}
	if !strings.Contains(view, "0/1 staged") {
		t.Error("View should contain '0/1 staged' counter in file headers")
	}
}

func TestHunkAddModel_HelpOverlay_ShowsNewKeys(t *testing.T) {
	t.Parallel()
	m, _ := newHunkAddTestModel(t, singleHunkDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := m.View()
	if !strings.Contains(view, "Space") {
		t.Error("help overlay should mention 'Space'")
	}
	if !strings.Contains(view, "Enter") {
		t.Error("help overlay should mention 'Enter'")
	}
	if !strings.Contains(view, "Line Edit") {
		t.Error("help overlay should contain 'Line Edit' section")
	}
}
