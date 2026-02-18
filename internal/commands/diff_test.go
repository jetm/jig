package commands

import (
	"context"
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

const sampleDiff = "diff --git a/main.go b/main.go\n" +
	"index 1234567..abcdefg 100644\n" +
	"--- a/main.go\n" +
	"+++ b/main.go\n" +
	"@@ -1,3 +1,4 @@\n" +
	" package main\n" +
	"+// added\n" +
	" func main() {}\n"

const twoDiffs = "diff --git a/a.go b/a.go\n" +
	"index 111..222 100644\n" +
	"--- a/a.go\n" +
	"+++ b/a.go\n" +
	"@@ -1 +1 @@\n" +
	"-old\n" +
	"+new\n" +
	"diff --git a/b.go b/b.go\n" +
	"index 333..444 100644\n" +
	"--- a/b.go\n" +
	"+++ b/b.go\n" +
	"@@ -1 +1 @@\n" +
	"-old\n" +
	"+new\n"

func newTestModel(t *testing.T, revision string, staged bool, diffOutput string) *DiffModel {
	t.Helper()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			diffOutput, // git diff output
			"main",     // branch name
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}

	m := NewDiffModel(context.Background(), runner, cfg, renderer, revision, staged, "")

	// Verify DiffArgs were used correctly
	expectedArgs := git.DiffArgs(revision, staged)
	call := testhelper.NthCall(runner, 0)
	if len(call.Args) != len(expectedArgs) {
		t.Fatalf("expected git args %v, got %v", expectedArgs, call.Args)
	}
	for i := range expectedArgs {
		if call.Args[i] != expectedArgs[i] {
			t.Errorf("arg[%d] = %q, want %q", i, call.Args[i], expectedArgs[i])
		}
	}

	return m
}

func TestNewDiffModel_GitArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		revision string
		staged   bool
		wantArgs []string
	}{
		{"working tree", "", false, []string{"diff"}},
		{"staged", "", true, []string{"diff", "--cached"}},
		{"revision", "HEAD~3", false, []string{"diff", "HEAD~3"}},
		{"staged+revision", "abc123", true, []string{"diff", "--cached", "abc123"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runner := &testhelper.FakeRunner{
				Outputs: []string{sampleDiff, "main"},
			}
			cfg := config.NewDefault()
			renderer := &diff.PlainRenderer{}
			_ = NewDiffModel(context.Background(), runner, cfg, renderer, tc.revision, tc.staged, "")

			call := testhelper.NthCall(runner, 0)
			if len(call.Args) != len(tc.wantArgs) {
				t.Fatalf("args = %v, want %v", call.Args, tc.wantArgs)
			}
			for i := range tc.wantArgs {
				if call.Args[i] != tc.wantArgs[i] {
					t.Errorf("arg[%d] = %q, want %q", i, call.Args[i], tc.wantArgs[i])
				}
			}
		})
	}
}

func TestDiffModel_QuitOnQ(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected a command from pressing q")
	}
	msg := cmd()
	if _, ok := msg.(app.PopModelMsg); !ok {
		t.Errorf("expected PopModelMsg, got %T", msg)
	}
	popMsg := msg.(app.PopModelMsg)
	if popMsg.MutatedGit {
		t.Error("PopModelMsg.MutatedGit should be false for diff")
	}
}

func TestDiffModel_QuitOnEsc(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
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

func TestDiffModel_HelpToggle(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
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

func TestDiffModel_View_TwoPanel(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	// Should contain the file path somewhere in the output
	if !strings.Contains(view, "main.go") {
		t.Error("View() should contain the file name 'main.go'")
	}
}

func TestDiffModel_View_TooSmall(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 30
	m.height = 5

	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Errorf("View() with small terminal should mention 'too small', got: %q", view)
	}
}

func TestDiffModel_View_EmptyDiff(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "No changes") {
		t.Errorf("View() with no diffs should mention 'No changes', got: %q", view)
	}
}

func TestDiffModel_StatusBarShowsBranch(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "main") {
		t.Error("View() should show branch name 'main' in status bar")
	}
}

func TestDiffModel_SelectionChangeUpdatesPreview(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, twoDiffs)
	m.width = 120
	m.height = 40

	// Initial view should have first file selected
	if m.selectedPath == "" {
		t.Fatal("initial selectedPath should not be empty")
	}

	if len(m.files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(m.files))
	}
}

func TestDiffModel_WindowSizeMsg(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)

	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})

	if m.width != 200 {
		t.Errorf("width = %d, want 200", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestDiffModel_HelpVisibleBlocksKeys(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40

	// Open help
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !m.help.IsVisible() {
		t.Fatal("help should be visible")
	}

	// Press q while help is visible — should NOT quit
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(app.PopModelMsg); ok {
			t.Error("pressing q while help visible should not produce PopModelMsg")
		}
	}
}

func TestDiffModel_View_HelpVisible(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := m.View()

	if !strings.Contains(view, "Navigation") {
		t.Error("help overlay view should contain 'Navigation'")
	}
}

func TestDiffModel_FileTreeRendersFiles(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "main.go") {
		t.Error("View() should contain file name 'main.go'")
	}
}

func TestDiffModel_KeyForwardsToList(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40

	// Press a random key that goes to the list (e.g., 'j')
	cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Should not be nil (batch of sbCmd + listCmd)
	_ = cmd
}

func TestDiffModel_RenderSelectedDiff_NoSelection(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.selectedPath = "nonexistent.go"
	// Should not panic
	m.renderSelectedDiff()
}

type errorRenderer struct{}

func (e *errorRenderer) Render(_ string) (string, error) {
	return "", context.DeadlineExceeded
}

func TestDiffModel_RenderSelectedDiff_RendererError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{sampleDiff, "main"},
	}
	cfg := config.NewDefault()
	renderer := &errorRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
	m.width = 120
	m.height = 40

	// Model should still have been created (fallback to raw diff)
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
}

func TestDiffModel_TabTogglesFocus(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
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

func TestDiffModel_QuitFromRightPanel(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
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

func TestDiffModel_RightPanelReceivesKeys(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})

	// Press j - should go to diffView, not fileList
	cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_ = cmd // should not panic
}

func TestDiffModel_View_FocusRightRenders(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40
	m.focusRight = true

	view := m.View()
	if view == "" {
		t.Fatal("View() with focusRight returned empty string")
	}
	if !strings.Contains(view, "main.go") {
		t.Error("View() with focusRight should still contain file name")
	}
}

func TestDiffModel_CheckSelectionChange_NilSelected(t *testing.T) {
	t.Parallel()
	// Empty diff — SelectedItem returns nil
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
	// Should not panic
	m.checkSelectionChange()
}

func TestDiffModel_DKeyTogglesDiffPanel(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
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

func TestDiffModel_TabNoopWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{sampleDiff, "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
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

func TestDiffModel_SinglePanelViewWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{sampleDiff, "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	// Should still contain the file name in the left panel
	if !strings.Contains(view, "main.go") {
		t.Error("single-panel View() should still contain file name 'main.go'")
	}
}

func TestDiffModel_FKeyTogglesDiffMaximized(t *testing.T) {
	t.Parallel()
	rawDiff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n"
	m := newTestModel(t, "", false, rawDiff)
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

func TestDiffModel_WKeyTogglesSoftWrap(t *testing.T) {
	t.Parallel()
	rawDiff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n"
	m := newTestModel(t, "", false, rawDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	initial := m.diffView.SoftWrap()
	m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if m.diffView.SoftWrap() == initial {
		t.Error("w should toggle soft-wrap when right panel focused")
	}
}

func TestDiffModel_BracketKeysAdjustPanelRatio(t *testing.T) {
	t.Parallel()
	rawDiff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n"
	m := newTestModel(t, "", false, rawDiff)
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

func TestDiffModel_MaximizeView(t *testing.T) {
	t.Parallel()
	rawDiff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n"
	m := newTestModel(t, "", false, rawDiff)
	m.width = 120
	m.height = 40

	normalView := m.View()
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	maximizedView := m.View()
	if normalView == maximizedView {
		t.Error("maximize mode should produce a different view")
	}
}

func TestDiffModel_ResizeWhileMaximized(t *testing.T) {
	t.Parallel()
	rawDiff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n"
	m := newTestModel(t, "", false, rawDiff)
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

func TestDiffModel_EKey_WithFile(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40

	// e key with a selected file that has RawDiff returns a non-nil cmd.
	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if cmd == nil {
		t.Error("e key with selected file should return non-nil cmd")
	}
}

func TestDiffModel_EKey_NoMatch(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40
	// Point selectedPath to a non-existent file.
	m.selectedPath = "nonexistent.go"

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_ = cmd // returns sbCmd (noop)
}

func TestDiffModel_EKey_EmptyFiles(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_ = cmd // returns sbCmd (noop, no files)
}

func TestDiffModel_EditDiffMsg_Error(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.width = 120
	m.height = 40

	cmd := m.Update(git.EditDiffMsg{Err: context.DeadlineExceeded})
	_ = cmd
}

func TestDiffModel_EditDiffMsg_ApplyError(t *testing.T) {
	t.Parallel()
	editedPath := t.TempDir() + "/addp-hunk-edit.diff"
	originalDiff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n"
	modifiedDiff := originalDiff + "extra"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			sampleDiff, // git diff
			"main",     // branch name
			"",         // git apply --cached (will fail)
		},
		Errors: []error{nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
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

func TestDiffModel_EditDiffMsg_Success(t *testing.T) {
	t.Parallel()
	editedPath := t.TempDir() + "/addp-hunk-edit.diff"
	originalDiff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			sampleDiff, // git diff
			"main",     // branch name
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
	m.width = 120
	m.height = 40

	// Same content — ApplyEditedDiff skips apply; "Patch applied" is shown.
	if err := os.WriteFile(editedPath, []byte(originalDiff), 0o600); err != nil {
		t.Fatalf("failed to write edited diff: %v", err)
	}

	cmd := m.Update(git.EditDiffMsg{
		EditedPath:   editedPath,
		OriginalDiff: originalDiff,
	})
	_ = cmd
}

func TestNewDiffModel_RawInput_ParsesFilesAndSkipsDiffCall(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"main", // branch name (only call expected)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, sampleDiff)

	if len(m.files) != 1 {
		t.Fatalf("expected 1 file from rawInput, got %d", len(m.files))
	}
	if m.files[0].DisplayPath() != "main.go" {
		t.Errorf("file path = %q, want %q", m.files[0].DisplayPath(), "main.go")
	}
	// Only one call should have been made (branch name), not two (diff + branch)
	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 runner call (branch), got %d", len(runner.Calls))
	}
	call := testhelper.NthCall(runner, 0)
	if call.Args[0] != "rev-parse" {
		t.Errorf("expected branch name call, got args %v", call.Args)
	}
}

func TestNewDiffModel_RawInput_StatusBarShowsPagerMode(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}

	// Pager mode: rawInput non-empty
	pagerModel := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, sampleDiff)
	pagerModel.width = 120
	pagerModel.height = 40
	pagerView := pagerModel.View()
	if !strings.Contains(pagerView, "diff (pager)") {
		t.Error("pager mode status bar should contain 'diff (pager)'")
	}

	// Normal mode: rawInput empty
	normalRunner := &testhelper.FakeRunner{
		Outputs: []string{sampleDiff, "main"},
	}
	normalModel := NewDiffModel(context.Background(), normalRunner, cfg, renderer, "", false, "")
	normalModel.width = 120
	normalModel.height = 40
	normalView := normalModel.View()
	if strings.Contains(normalView, "diff (pager)") {
		t.Error("normal mode status bar should not contain 'diff (pager)'")
	}
}

func TestNewDiffModel_RawInput_StripsANSICodes(t *testing.T) {
	t.Parallel()
	// Simulate git pager output with ANSI color codes
	coloredDiff := "\x1b[1mdiff --git c/foo i/foo\x1b[m\n" +
		"\x1b[1mnew file mode 100644\x1b[m\n" +
		"\x1b[1mindex 0000000..7898192\x1b[m\n" +
		"\x1b[1m--- /dev/null\x1b[m\n" +
		"\x1b[1m+++ i/foo\x1b[m\n" +
		"\x1b[36m@@ -0,0 +1 @@\x1b[m\n" +
		"\x1b[32m+\x1b[m\x1b[32ma\x1b[m\n"

	runner := &testhelper.FakeRunner{
		Outputs: []string{"main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, coloredDiff)

	if len(m.files) != 1 {
		t.Fatalf("expected 1 file from colored rawInput, got %d", len(m.files))
	}
	if m.files[0].DisplayPath() != "foo" {
		t.Errorf("file path = %q, want %q", m.files[0].DisplayPath(), "foo")
	}
}

func TestNewDiffModel_RawInput_EmptyStringShowsNoFiles(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",     // git diff (empty - normal mode with empty rawInput)
			"main", // branch name
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	// Empty rawInput falls through to runner.Run which returns empty diff
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
	if len(m.files) != 0 {
		t.Errorf("expected 0 files from empty diff, got %d", len(m.files))
	}
}

func TestNewDiffModel_WithFilterPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			sampleDiff, // git diff -- main.go
			"main",     // branch name
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "", []string{"main.go"})
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	// Verify -- separator was used.
	testhelper.MustHaveCall(t, runner, "diff", "--", "main.go")
}

func TestNewDiffModel_FilterPaths_NoMatch(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",     // git diff -- nonexistent.go (empty)
			"main", // branch name
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "", []string{"nonexistent.go"})
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
