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

// newTestCheckoutModel builds a CheckoutModel backed by a FakeRunner.
func newTestCheckoutModel(t *testing.T, nameStatus string) *CheckoutModel {
	t.Helper()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			nameStatus, // git diff --name-status
			"main",     // branch name
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return NewCheckoutModel(context.Background(), runner, cfg, renderer)
}

func TestNewCheckoutModel_EmptyFiles(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "")
	if len(m.files) != 0 {
		t.Errorf("expected 0 files, got %d", len(m.files))
	}
}

func TestNewCheckoutModel_WithFiles(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	if m.files[0].Path != "foo.go" {
		t.Errorf("expected foo.go, got %q", m.files[0].Path)
	}
}

func TestCheckoutModel_QuitOnQ(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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
		t.Error("MutatedGit should be false when quitting without discarding")
	}
}

func TestCheckoutModel_QuitOnEsc(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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

func TestCheckoutModel_HelpToggle(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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

func TestCheckoutModel_HelpVisibleBlocksKeys(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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

func TestCheckoutModel_ConfirmationAppears(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	if m.confirming {
		t.Fatal("confirming should be false initially")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.confirming {
		t.Error("confirming should be true after pressing Enter")
	}
}

func TestCheckoutModel_ConfirmationInView(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Discard changes") {
		t.Errorf("View() should show confirmation prompt, got: %q", view)
	}
	if !strings.Contains(view, "[y/N]") {
		t.Errorf("View() should contain [y/N], got: %q", view)
	}
}

func TestCheckoutModel_YDiscardsAndPops(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"main",        // branch name
			"",            // git checkout --
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Press Enter to trigger confirmation
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.confirming {
		t.Fatal("should be in confirming state")
	}

	// Press y to confirm
	cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("expected a command after pressing y")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after discarding")
	}
	testhelper.MustHaveCall(t, runner, "checkout", "--", "foo.go")
}

func TestCheckoutModel_CancelConfirmation(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	// Enter confirmation
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !m.confirming {
		t.Fatal("should be confirming")
	}

	// Press n to cancel
	m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.confirming {
		t.Error("confirming should be false after pressing n")
	}
}

func TestCheckoutModel_EscCancelsConfirmation(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.confirming {
		t.Error("confirming should be false after Esc")
	}
	// Should not quit (confirming mode intercepts Esc)
}

func TestCheckoutModel_SpaceTogglesSelection(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	if len(m.fileList.CheckedPaths()) != 0 {
		t.Fatal("no files should be checked initially")
	}
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if len(m.fileList.CheckedPaths()) != 1 {
		t.Error("foo.go should be checked after Space")
	}
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if len(m.fileList.CheckedPaths()) != 0 {
		t.Error("foo.go should be unchecked after second Space")
	}
}

func TestCheckoutModel_SelectAll(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\nM\tbar.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	checked := m.fileList.CheckedPaths()
	if len(checked) != 2 {
		t.Errorf("expected 2 checked files after pressing a, got %d", len(checked))
	}
}

func TestCheckoutModel_DeselectAll(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\nM\tbar.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	checked := m.fileList.CheckedPaths()
	if len(checked) != 0 {
		t.Errorf("expected 0 checked files after pressing d, got %d", len(checked))
	}
}

func TestCheckoutModel_View_EmptyState(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "Nothing to discard") {
		t.Errorf("View() with no files should say 'Nothing to discard', got: %q", view)
	}
}

func TestCheckoutModel_View_TooSmall(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 30
	m.height = 5

	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Errorf("View() with small terminal should mention 'too small', got: %q", view)
	}
}

func TestCheckoutModel_View_TwoPanel(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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

func TestCheckoutModel_View_HelpVisible(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := m.View()
	if !strings.Contains(view, "Navigation") {
		t.Error("help overlay should contain 'Navigation'")
	}
}

func TestCheckoutModel_WindowSizeMsg(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")

	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	if m.width != 200 {
		t.Errorf("width = %d, want 200", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestCheckoutModel_StatusBarShowsBranch(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "main") {
		t.Error("View() should contain branch name 'main'")
	}
}

func TestCheckoutModel_FileListRendersFiles(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\ttest.go\n")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "test.go") {
		t.Errorf("View() should contain 'test.go', got %q", view)
	}
}

func TestCheckoutModel_EnterWithNoFiles(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "")
	m.width = 120
	m.height = 40

	// No files, Enter should not set confirming=true
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.confirming {
		t.Error("confirming should not be set when there are no files")
	}
}

func TestCheckoutModel_MultiSelectDiscard(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\nM\tbar.go\n", // diff --name-status
			"main",                   // branch name
			"",                       // git checkout --
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Select all
	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	// Confirm
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("expected a command after y")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after discarding")
	}
}

func TestCheckoutModel_DiscardError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status (ListModifiedFiles)
			"main",        // branch name (BranchName)
			"",            // diff preview (renderSelectedDiff — no diff)
			"",            // git checkout -- (discard call)
		},
		Errors: []error{nil, nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("expected a command even on discard error")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if pop.MutatedGit {
		t.Error("MutatedGit should be false when discard fails")
	}
}

func TestCheckoutModel_RenderSelectedDiff_WithDiff(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"main",        // branch
			// renderSelectedDiff on construction:
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "foo.go") {
		t.Error("View() should contain file name")
	}
}

func TestCheckoutModel_RenderSelectedDiff_DiffError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"main",        // branch
			"",            // renderSelectedDiff: diff error
		},
		Errors: []error{nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after diff error")
	}
}

func TestCheckoutModel_TabTogglesFocus(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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

func TestCheckoutModel_QuitFromRightPanel(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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

func TestCheckoutModel_DKeyTogglesDiffPanel(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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

func TestCheckoutModel_TabNoopWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"main",        // branch name
		},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
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

func TestCheckoutModel_SinglePanelViewWhenDiffHidden(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"main",        // branch name
		},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
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

func TestCheckoutModel_KeyJForwardsToList(t *testing.T) {
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
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_ = cmd
}

func TestCheckoutModel_FKeyTogglesDiffMaximized(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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

func TestCheckoutModel_WKeyTogglesSoftWrap(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	initial := m.diffView.SoftWrap()
	m.Update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if m.diffView.SoftWrap() == initial {
		t.Error("w should toggle soft-wrap when right panel focused")
	}
}

func TestCheckoutModel_BracketKeysAdjustPanelRatio(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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

func TestCheckoutModel_MaximizeView(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	normalView := m.View()
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	maximizedView := m.View()
	if normalView == maximizedView {
		t.Error("maximize mode should produce a different view")
	}
}

func TestCheckoutModel_TabNoopWhenMaximized(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.focusRight {
		t.Error("Tab should not switch focus when maximized")
	}
}

func TestCheckoutModel_ResizeWhileMaximized(t *testing.T) {
	t.Parallel()
	m := newTestCheckoutModel(t, "M\tfoo.go\n")
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

func TestCheckoutModel_EKey_NoDiff(t *testing.T) {
	t.Parallel()
	// When the runner returns empty string for diff, e key shows "No diff to edit".
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"main",        // branch name
			"",            // renderSelectedDiff
			"",            // e key: diff -- foo.go (empty = no diff)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_ = cmd // sbCmd returned, no editor launched
}

func TestCheckoutModel_EKey_NoSelection(t *testing.T) {
	t.Parallel()
	// When there are no files, e key should be a noop.
	m := newTestCheckoutModel(t, "")
	m.width = 120
	m.height = 40

	cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	_ = cmd // sbCmd returned
}

func TestCheckoutModel_EditDiffMsg_Error(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"main",        // branch name
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Send EditDiffMsg with error directly.
	cmd := m.Update(git.EditDiffMsg{Err: context.DeadlineExceeded})
	_ = cmd // sbCmd returned
}

func TestCheckoutModel_EditDiffMsg_ApplyError(t *testing.T) {
	t.Parallel()
	// Write a temp file with modified content so ApplyEditedDiff tries to apply it.
	editedPath := t.TempDir() + "/addp-hunk-edit.diff"

	originalDiff := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n"
	modifiedDiff := originalDiff + "extra"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"main",        // branch name
			"",            // git apply --cached (will fail)
		},
		Errors: []error{nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Write modified diff to the temp edited path.
	if err := os.WriteFile(editedPath, []byte(modifiedDiff), 0o600); err != nil {
		t.Fatalf("failed to write edited diff: %v", err)
	}

	cmd := m.Update(git.EditDiffMsg{
		EditedPath:   editedPath,
		OriginalDiff: originalDiff,
	})
	_ = cmd // sbCmd with error message
}

func TestCheckoutModel_EditDiffMsg_Success(t *testing.T) {
	t.Parallel()
	// Write a temp file with the SAME content so ApplyEditedDiff skips apply.
	editedPath := t.TempDir() + "/addp-hunk-edit.diff"
	originalDiff := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"main",        // branch name
			"",            // renderSelectedDiff after success
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	if err := os.WriteFile(editedPath, []byte(originalDiff), 0o600); err != nil {
		t.Fatalf("failed to write edited diff: %v", err)
	}

	cmd := m.Update(git.EditDiffMsg{
		EditedPath:   editedPath,
		OriginalDiff: originalDiff,
	})
	_ = cmd // sbCmd returned, renderSelectedDiff called
}

func TestNewCheckoutModel_WithFilterPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // git diff --name-status -- foo.go
			"main",        // branch name
			"",            // renderSelectedDiff
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer, []string{"foo.go"})
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	testhelper.MustHaveCall(t, runner, "diff", "--name-status", "--", "foo.go")
}

func TestNewCheckoutModel_FilterPaths_NoMatch(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"", // git diff --name-status -- nonexistent.go (no files)
			"main",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer, []string{"nonexistent.go"})
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

func TestCheckoutModel_BraceKeysAdjustContextLines(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --name-status
			"main",        // branch name
			"",            // renderSelectedDiff (after })
			"",            // renderSelectedDiff (after {)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewCheckoutModel(context.Background(), runner, cfg, renderer)
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
