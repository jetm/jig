package commands

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/testhelper"
)

// singleHunkWorkingDiff is a working tree diff with one file and one hunk.
const singleHunkWorkingDiff = "diff --git a/bar.go b/bar.go\n" +
	"index 111..222 100644\n" +
	"--- a/bar.go\n" +
	"+++ b/bar.go\n" +
	"@@ -1,3 +1,4 @@\n" +
	" package main\n" +
	"+// working tree change\n" +
	" func bar() {}\n"

// newHunkCheckoutTestModel creates a HunkCheckoutModel using scripted outputs.
// outputs[0] = git diff output, outputs[1] = branch name, rest for git apply calls.
func newHunkCheckoutTestModel(t *testing.T, diffOutput string, extraOutputs ...string) (*HunkCheckoutModel, *testhelper.FakeRunner) {
	t.Helper()
	outputs := []string{diffOutput, "main"}
	outputs = append(outputs, extraOutputs...)
	runner := &testhelper.FakeRunner{Outputs: outputs}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	return m, runner
}

func TestNewHunkCheckoutModel_NoChanges(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, "")
	if len(m.files) != 0 {
		t.Errorf("expected 0 files, got %d", len(m.files))
	}
}

func TestNewHunkCheckoutModel_SingleFile(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
}

func TestHunkCheckoutModel_QuitOnQ(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
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

func TestHunkCheckoutModel_ViewEmpty(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "No working tree changes") {
		t.Errorf("View() with no files should mention 'No working tree changes', got: %q", view)
	}
}

func TestHunkCheckoutModel_EnterWithNothingSelectedDoesNothing(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(app.PopModelMsg); ok {
			t.Error("Enter with nothing selected should not pop")
		}
	}
	// Should not enter confirmation mode
	if m.confirming {
		t.Error("should not enter confirming mode with nothing selected")
	}
}

func TestHunkCheckoutModel_EnterShowsConfirmation(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	// Toggle hunk
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})

	// Press Enter - should enter confirmation mode
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.confirming {
		t.Fatal("should enter confirming mode after Enter with selected hunks")
	}

	// View should show confirmation prompt
	view := m.View()
	if !strings.Contains(view, "Discard") {
		t.Error("View() in confirmation should contain 'Discard'")
	}
	if !strings.Contains(view, "[y/N]") {
		t.Error("View() in confirmation should contain '[y/N]'")
	}
}

func TestHunkCheckoutModel_ConfirmationCancelOnNonY(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !m.confirming {
		t.Fatal("should be in confirming mode")
	}

	// Press 'n' to cancel
	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.confirming {
		t.Error("pressing non-y key should cancel confirmation")
	}
}

func TestHunkCheckoutModel_ConfirmationAppliesOnY(t *testing.T) {
	t.Parallel()
	m, runner := newHunkCheckoutTestModel(t, singleHunkWorkingDiff, "" /* git apply output */)
	m.width = 120
	m.height = 40

	// Toggle and enter confirmation
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Confirm with 'y'
	cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})

	// Verify git apply --reverse was called (not --cached)
	testhelper.MustHaveCall(t, runner, "apply", "--reverse")

	if cmd == nil {
		t.Fatal("expected command after confirmation")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after discard")
	}
}

func TestHunkCheckoutModel_ApplyError(t *testing.T) {
	t.Parallel()
	applyErr := fmt.Errorf("apply failed")
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkWorkingDiff, "main", ""},
		Errors:  []error{nil, nil, applyErr},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	cmd := m.applySelected()
	if cmd != nil {
		t.Error("applySelected on runner error should return nil when no hunks applied")
	}
}

func TestHunkCheckoutModel_UsesSelectedLabel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "selected") {
		t.Error("View() should contain 'selected' label from HunkList")
	}
}

func TestHunkCheckoutModel_HelpToggle(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
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

func TestHunkCheckoutModel_StatusBarMode(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "hunk-checkout") {
		t.Error("status bar should display 'hunk-checkout' mode")
	}
}

func TestHunkCheckoutModel_TabTogglesFocus(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
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

func TestHunkCheckoutModel_DKeyTogglesDiff(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	viewWith := m.View()
	m.Update(tea.KeyPressMsg{Code: 'D', ShiftedCode: 'D', Mod: tea.ModShift, Text: "D"})
	viewWithout := m.View()
	if viewWith == viewWithout {
		t.Error("D should toggle diff panel")
	}
}

func TestHunkCheckoutModel_FKeyMaximizes(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	normalView := m.View()
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	maximizedView := m.View()
	if normalView == maximizedView {
		t.Error("F should maximize diff panel")
	}
}

func TestHunkCheckoutModel_WKeySoftWrap(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	initial := m.diffView.SoftWrap()
	m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if m.diffView.SoftWrap() == initial {
		t.Error("w should toggle soft-wrap when right panel focused")
	}
}

func TestHunkCheckoutModel_BracketKeysAdjustRatio(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
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

func TestHunkCheckoutModel_WindowSize(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)

	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	if m.width != 200 || m.height != 50 {
		t.Errorf("width=%d, height=%d, want 200, 50", m.width, m.height)
	}
}

func TestHunkCheckoutModel_ResizeWhileMaximized(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after resize while maximized")
	}
}

func TestHunkCheckoutModel_ViewTooSmall(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 30
	m.height = 5
	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Errorf("View() with small terminal should mention 'too small', got: %q", view)
	}
}

func TestHunkCheckoutModel_NoMatchFilter(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "main"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer, []string{"nonexistent.go"})
	m.width = 120
	m.height = 40
	view := m.View()
	if !strings.Contains(view, "No matching") {
		t.Errorf("View() should show no-match message, got: %q", view)
	}
}

func TestHunkCheckoutModel_ConfirmationViewMaximized(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	// Maximize, then enter confirmation
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Discard") {
		t.Error("maximized confirmation view should show 'Discard'")
	}
}

func TestHunkCheckoutModel_ConfirmationViewTwoPanel(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Discard") {
		t.Error("two-panel confirmation view should show 'Discard'")
	}
}

func TestHunkCheckoutModel_ResizeDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkWorkingDiff, "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view == "" {
		t.Fatal("View() should not be empty after resize with diff hidden")
	}
}

func TestHunkCheckoutModel_DiffHiddenSinglePanel(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkWorkingDiff, "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
}

func TestHunkCheckoutModel_ConfirmationViewDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkWorkingDiff, "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	view := m.View()
	if !strings.Contains(view, "Discard") {
		t.Error("single-panel confirmation view should show 'Discard'")
	}
}

func TestHunkCheckoutModel_RightPanelNavigation(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
}

func TestHunkCheckoutModel_EscQuits(t *testing.T) {
	t.Parallel()
	m, _ := newHunkCheckoutTestModel(t, singleHunkWorkingDiff)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected command from Esc")
	}
	msg := cmd()
	if _, ok := msg.(app.PopModelMsg); !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
}

func TestHunkCheckoutModel_DiscardDoesNotUseCached(t *testing.T) {
	t.Parallel()
	m, runner := newHunkCheckoutTestModel(t, singleHunkWorkingDiff, "" /* git apply output */)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})

	// Verify --cached is NOT in the apply args
	for _, c := range runner.Calls {
		for i, arg := range c.Args {
			if arg == "apply" {
				for _, remaining := range c.Args[i:] {
					if remaining == "--cached" {
						t.Error("hunk-checkout should not use --cached flag")
					}
				}
			}
		}
	}
}
