package commands

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/testhelper"
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

func TestCheckoutModel_FileTreeRendersFiles(t *testing.T) {
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
