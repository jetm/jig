package commands

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/testhelper"
)

// singleHunkStagedDiff is a staged diff with one file and one hunk.
const singleHunkStagedDiff = "diff --git a/foo.go b/foo.go\n" +
	"index 111..222 100644\n" +
	"--- a/foo.go\n" +
	"+++ b/foo.go\n" +
	"@@ -1,3 +1,4 @@\n" +
	" package main\n" +
	"+// staged line\n" +
	" func foo() {}\n"

// newHunkResetTestModel creates a HunkResetModel using scripted outputs.
// outputs[0] = git diff --cached output, outputs[1] = branch name, rest for git apply calls.
func newHunkResetTestModel(t *testing.T, diffOutput string, extraOutputs ...string) (*HunkResetModel, *testhelper.FakeRunner) {
	t.Helper()
	outputs := []string{diffOutput, "main"}
	outputs = append(outputs, extraOutputs...)
	runner := &testhelper.FakeRunner{Outputs: outputs}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkResetModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewHunkResetModel unexpectedly returned error: %v", err)
	}
	return m, runner
}

func TestNewHunkResetModel_NoStagedChanges(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, "")
	if len(m.files) != 0 {
		t.Errorf("expected 0 files, got %d", len(m.files))
	}
}

func TestNewHunkResetModel_SingleFile(t *testing.T) {
	t.Parallel()
	m, runner := newHunkResetTestModel(t, singleHunkStagedDiff)
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	// Verify it used --cached for the diff
	testhelper.MustHaveCall(t, runner, "diff", "--cached")
}

func TestHunkResetModel_QuitOnQ(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
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

func TestHunkResetModel_ViewEmpty(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "No staged changes") {
		t.Errorf("View() with no files should mention 'No staged changes', got: %q", view)
	}
}

func TestHunkResetModel_EnterWithNothingSelectedDoesNothing(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(app.PopModelMsg); ok {
			t.Error("Enter with nothing selected should not pop")
		}
	}
}

func TestHunkResetModel_EnterAppliesSelectedHunks(t *testing.T) {
	t.Parallel()
	m, runner := newHunkResetTestModel(t, singleHunkStagedDiff, "" /* git apply output */)
	m.width = 120
	m.height = 40

	// Toggle hunk selected
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	// Press Enter to unstage
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Verify git apply --cached --reverse was called
	testhelper.MustHaveCall(t, runner, "apply", "--cached", "--reverse")

	if cmd == nil {
		t.Fatal("expected a command after Enter")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after unstaging")
	}
}

func TestHunkResetModel_ApplyError(t *testing.T) {
	t.Parallel()
	applyErr := fmt.Errorf("apply failed")
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkStagedDiff, "main", ""},
		Errors:  []error{nil, nil, applyErr},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkResetModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	cmd := m.applySelected()
	if cmd == nil {
		t.Error("applySelected on runner error should return a status bar error cmd, not nil")
	}
	// Must not pop the model on error
	msg := cmd()
	if _, ok := msg.(app.PopModelMsg); ok {
		t.Error("should not pop model on unstage error — model must stay visible with error in status bar")
	}
}

func TestHunkResetModel_UsesSelectedLabel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	// File header should show "0/1 selected" not "0/1 staged"
	if !strings.Contains(view, "0/1 selected") {
		t.Error("View() should contain '0/1 selected' label from HunkList")
	}
	if strings.Contains(view, "0/1 staged") {
		t.Error("View() should not contain '0/1 staged' label")
	}
}

func TestHunkResetModel_HelpToggle(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 120
	m.height = 40

	if m.help.IsVisible() {
		t.Fatal("help should start hidden")
	}
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !m.help.IsVisible() {
		t.Error("help should be visible after pressing ?")
	}
}

func TestHunkResetModel_StatusBarMode(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "hunk-reset") {
		t.Error("status bar should display 'hunk-reset' mode")
	}
}

func TestHunkResetModel_TabTogglesFocus(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
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

func TestHunkResetModel_DKeyTogglesDiff(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 120
	m.height = 40

	viewWith := m.View()
	m.Update(tea.KeyPressMsg{Code: 'D', ShiftedCode: 'D', Mod: tea.ModShift, Text: "D"})
	viewWithout := m.View()
	if viewWith == viewWithout {
		t.Error("D should toggle diff panel")
	}
}

func TestHunkResetModel_FKeyMaximizes(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 120
	m.height = 40

	normalView := m.View()
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	maximizedView := m.View()
	if normalView == maximizedView {
		t.Error("F should maximize diff panel")
	}
}

func TestHunkResetModel_WKeySoftWrap(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	initial := m.diffView.SoftWrap()
	m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if m.diffView.SoftWrap() == initial {
		t.Error("w should toggle soft-wrap when right panel focused")
	}
}

func TestHunkResetModel_BracketKeysAdjustRatio(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
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

func TestHunkResetModel_WindowSize(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)

	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	if m.width != 200 || m.height != 50 {
		t.Errorf("width=%d, height=%d, want 200, 50", m.width, m.height)
	}
}

func TestHunkResetModel_ResizeWhileMaximized(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after resize while maximized")
	}
}

func TestHunkResetModel_ViewTooSmall(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 30
	m.height = 5
	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Errorf("View() with small terminal should mention 'too small', got: %q", view)
	}
}

func TestHunkResetModel_NoMatchFilter(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkResetModel(context.Background(), runner, cfg, renderer, []string{"nonexistent.go"})
	require.NoError(t, err)
	m.width = 120
	m.height = 40
	view := m.View()
	if !strings.Contains(view, "No matching") {
		t.Errorf("View() should show no-match message, got: %q", view)
	}
}

func TestHunkResetModel_JKNavigation(t *testing.T) {
	t.Parallel()
	twoHunkStagedDiff := "diff --git a/bar.go b/bar.go\n" +
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

	m, _ := newHunkResetTestModel(t, twoHunkStagedDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if m.hunkList.CurrentHunkIdx() != 1 {
		t.Errorf("expected cursor on hunk 1, got %d", m.hunkList.CurrentHunkIdx())
	}
}

func TestHunkResetModel_RightPanelNavigation(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// Right panel scroll keys shouldn't panic
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
}

func TestHunkResetModel_DiffHiddenSinglePanel(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkStagedDiff, "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkResetModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
}

func TestHunkResetModel_ResizeDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkStagedDiff, "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkResetModel(context.Background(), runner, cfg, renderer)
	require.NoError(t, err)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view == "" {
		t.Fatal("View() should not be empty after resize with diff hidden")
	}
}

func TestHunkResetModel_QuitFromRightPanel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected command from q on right panel")
	}
}

func TestHunkResetModel_EscQuits(t *testing.T) {
	t.Parallel()
	m, _ := newHunkResetTestModel(t, singleHunkStagedDiff)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected command from Esc")
	}
}
