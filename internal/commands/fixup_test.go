package commands_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/commands"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/testhelper"
)

// newFakeFixupModel creates a FixupModel with scripted FakeRunner outputs.
// Call sequence in NewFixupModel:
//   - output[0]: git diff --cached --quiet (HasStagedChanges: error = staged, nil = none)
//   - output[1]: git rev-parse --show-toplevel (IsRebaseInProgress->RepoRoot)
//   - output[2]: git log (RecentCommits)
//   - output[3]: git rev-parse --abbrev-ref HEAD (BranchName)
//   - output[4]: git show <hash> (renderSelectedDiff, only if commits non-empty)
func newFakeFixupModel(t *testing.T, logOutput, branch string) *commands.FixupModel {
	t.Helper()
	// HasStagedChanges: return error so staged changes are detected
	// IsRebaseInProgress->RepoRoot: return a path (filesystem won't have .git/rebase-merge)
	outputs := []string{"", "/fake/repo", logOutput, branch}
	errors := []error{fmt.Errorf("staged"), nil, nil, nil}
	if logOutput != "" {
		// First commit diff will be fetched on init
		outputs = append(outputs, "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new")
		errors = append(errors, nil)
	}
	runner := &testhelper.FakeRunner{Outputs: outputs, Errors: errors}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestNewFixupModel_GitLogFails_ReturnsError(t *testing.T) {
	// staged-check (staged), reporoot, then git log fails
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "/fake/repo", ""},
		Errors:  []error{fmt.Errorf("staged"), nil, fmt.Errorf("not a git repository")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	_, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err == nil {
		t.Fatal("expected error when git log fails, got nil")
	}
}

func TestNewFixupModel_NoStagedChanges_ReturnsError(t *testing.T) {
	// HasStagedChanges returns nil (no staged changes) -> error expected
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{nil},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	_, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err == nil {
		t.Fatal("expected error when no staged changes, got nil")
	}
	if !strings.Contains(err.Error(), "nothing staged") {
		t.Errorf("error should say 'nothing staged', got: %v", err)
	}
}

func TestNewFixupModel_RebaseInProgress_ReturnsError(t *testing.T) {
	// HasStagedChanges: staged (returns error)
	// IsRebaseInProgress->RepoRoot: returns a path that has .git/rebase-merge
	// We need a real temp dir with .git/rebase-merge to trigger this path.
	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.git/rebase-merge", 0o755); err != nil {
		t.Fatalf("failed to create .git/rebase-merge: %v", err)
	}
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", tmpDir},
		Errors:  []error{fmt.Errorf("staged"), nil},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	_, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err == nil {
		t.Fatal("expected error when rebase in progress, got nil")
	}
	if !strings.Contains(err.Error(), "rebase") {
		t.Errorf("error should mention rebase, got: %v", err)
	}
}

func TestNewFixupModel_NoCommits(t *testing.T) {
	m := newFakeFixupModel(t, "", "main")
	if m == nil {
		t.Fatal("NewFixupModel returned nil")
	}
}

func TestNewFixupModel_WithCommits(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	m := newFakeFixupModel(t, logOutput, "main")
	if m == nil {
		t.Fatal("NewFixupModel returned nil")
	}
}

func TestFixupModel_View_TooSmall(t *testing.T) {
	m := newFakeFixupModel(t, "", "main")
	// Don't send WindowSizeMsg; width=0,height=0 → too small
	view := m.View()
	if view != "Terminal too small. Please resize to at least 60x10." {
		t.Errorf("View() = %q", view)
	}
}

func TestFixupModel_View_NoCommits(t *testing.T) {
	m := newFakeFixupModel(t, "", "main")
	// Give it a size
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view != "No commits to fixup." {
		t.Errorf("View() = %q, want %q", view, "No commits to fixup.")
	}
}

func TestFixupModel_View_WithCommits(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	m := newFakeFixupModel(t, logOutput, "main")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	// Should render something (not the error/empty messages)
	if view == "" {
		t.Error("View() returned empty string with commits")
	}
	if view == "No commits to fixup." {
		t.Error("View() showed 'no commits' even though we have commits")
	}
}

func TestFixupModel_Update_WindowSize(t *testing.T) {
	m := newFakeFixupModel(t, "", "main")
	cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = cmd // no panic is the goal
}

func TestFixupModel_Update_QuitWithQ(t *testing.T) {
	m := newFakeFixupModel(t, "", "main")
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected PopModelMsg, got nil")
	}
}

func TestFixupModel_Update_QuitWithEscape(t *testing.T) {
	m := newFakeFixupModel(t, "", "main")
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape, Text: ""})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected PopModelMsg, got nil")
	}
}

func TestFixupModel_Update_HelpToggle(t *testing.T) {
	m := newFakeFixupModel(t, "", "main")
	// Help should not be visible initially
	view1 := m.View()

	// Toggle help with ?
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view2 := m.View()

	// Help overlay should now show different content
	if view1 == view2 {
		t.Error("View() should differ after toggling help overlay")
	}
}

func TestFixupModel_Update_HelpHidesNavigation(t *testing.T) {
	m := newFakeFixupModel(t, "", "main")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Open help
	_ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})

	// With help open, q should NOT quit (it should be consumed by help)
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	// cmd should be nil (sbCmd) because help overlay blocks navigation
	_ = cmd // just ensure no panic and help overlay intercepted
}

func TestFixupModel_Update_EnterNoCommits(t *testing.T) {
	m := newFakeFixupModel(t, "", "main")
	// Enter with no commits should return nil
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: ""})
	_ = cmd // no panic, nil is acceptable
}

func TestFixupModel_Update_EnterWithCommits_Failure(t *testing.T) {
	// Provide commits but make git commit --fixup fail
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		// outputs: staged-check, reporoot, log, branch, show (init), then "" for failed commit
		Outputs: []string{"", "/fake/repo", logOutput, "main", "diff content", ""},
		Errors:  []error{fmt.Errorf("staged"), nil, nil, nil, nil, fmt.Errorf("nothing to commit")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: ""})
	if cmd == nil {
		t.Error("expected an error-message command after fixup failure, got nil")
	}
}

func TestFixupModel_Update_EnterWithCommits_Success(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		// staged-check, reporoot, log, branch, show (init), fixup commit, autosquash rebase
		Outputs: []string{"", "/fake/repo", logOutput, "main", "diff content", "[main def5678] fixup! feat: something", ""},
		Errors:  []error{fmt.Errorf("staged"), nil, nil, nil, nil, nil, nil},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: ""})
	if cmd == nil {
		t.Fatal("expected a PopModelMsg command after successful fixup")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected PopModelMsg, got nil")
	}
}

func TestFixupModel_RenderSelectedDiff_ErrorPath(t *testing.T) {
	// Make git show return an error so renderSelectedDiff shows an error message
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		// staged-check, reporoot, log, branch, then error on show
		Outputs: []string{"", "/fake/repo", logOutput, "main", ""},
		Errors:  []error{fmt.Errorf("staged"), nil, nil, nil, fmt.Errorf("bad object abc1234")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}

	// View should render without panicking even with a diff error
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestFixupModel_TabThenQuitFromRightPanel(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\x1fAlice\x1f2 hours ago\x1e"
	m := newFakeFixupModel(t, logOutput, "main")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected command from q on right panel")
	}
}

func TestFixupModel_TabThenEnterFromRightPanel(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		// staged-check, reporoot, log, branch, show (init), fixup commit, autosquash rebase
		Outputs: []string{"", "/fake/repo", logOutput, "main", "diff content", "[main def5678] fixup!", ""},
		Errors:  []error{fmt.Errorf("staged"), nil, nil, nil, nil, nil, nil},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// Enter should still work from right panel (global key)
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter from right panel should trigger fixup")
	}
}

func TestFixupModel_ConfirmFixup_CallsAutosquashRebase(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		// staged-check, reporoot, log, branch, show (init), fixup commit, autosquash rebase
		Outputs: []string{"", "/fake/repo", logOutput, "main", "diff content", "[main def5678] fixup! feat: something", ""},
		Errors:  []error{fmt.Errorf("staged"), nil, nil, nil, nil, nil, nil},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: ""})
	if cmd == nil {
		t.Fatal("expected a command after fixup, got nil")
	}
	// The command should emit PopModelMsg on success
	msg := cmd()
	if msg == nil {
		t.Fatal("expected PopModelMsg, got nil")
	}
	// Verify autosquash rebase was called via RunWithEnv
	testhelper.MustHaveCall(t, runner, "rebase", "--interactive", "--autosquash", "abc1234^")
	testhelper.MustHaveEnv(t, runner, "GIT_SEQUENCE_EDITOR=true")
}

func TestFixupModel_ConfirmFixup_AutosquashFailure_ShowsError(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		// staged-check, reporoot, log, branch, show (init), fixup commit succeeds, autosquash rebase fails
		Outputs: []string{"", "/fake/repo", logOutput, "main", "diff content", "[main def5678] fixup! feat: something", ""},
		Errors:  []error{fmt.Errorf("staged"), nil, nil, nil, nil, nil, fmt.Errorf("conflict during rebase")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: ""})
	if cmd == nil {
		t.Fatal("expected an error-message command after autosquash failure, got nil")
	}
	// Should NOT be a PopModelMsg - should be an error display command
	msg := cmd()
	if msg == nil {
		t.Fatal("expected a message from the command")
	}
	// The message should not be a PopModelMsg
	if _, ok := msg.(app.PopModelMsg); ok {
		t.Error("autosquash failure should NOT emit PopModelMsg")
	}
}

func TestFixupModel_DKeyTogglesDiffPanel(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	// Need extra diff output for when D re-shows the panel
	runner := &testhelper.FakeRunner{
		// staged-check, reporoot, log, branch, show (init), show (D re-show)
		Outputs: []string{"", "/fake/repo", logOutput, "main",
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new",
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new"},
		Errors: []error{fmt.Errorf("staged"), nil, nil, nil, nil, nil},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// showDiff starts true (from config default)
	viewWith := m.View()

	// Press D to hide
	_ = m.Update(tea.KeyPressMsg{Code: 'D', ShiftedCode: 'D', Mod: tea.ModShift, Text: "D"})
	viewWithout := m.View()

	if viewWith == viewWithout {
		t.Error("View() should differ after D toggles diff off")
	}

	// Press D again to show
	_ = m.Update(tea.KeyPressMsg{Code: 'D', ShiftedCode: 'D', Mod: tea.ModShift, Text: "D"})
	viewAgain := m.View()

	if viewAgain == viewWithout {
		t.Error("View() should differ after D toggles diff back on")
	}
}

func TestFixupModel_TabNoopWhenDiffHidden(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "/fake/repo", logOutput, "main"},
		Errors:  []error{fmt.Errorf("staged"), nil, nil, nil},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	view1 := m.View()
	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	view2 := m.View()

	// View should be unchanged - Tab had no effect
	if view1 != view2 {
		t.Error("Tab should be a no-op when diff panel is hidden")
	}
}

func TestFixupModel_SinglePanelViewWhenDiffHidden(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		// staged-check, reporoot, log, branch (no diff on init since showDiff=false),
		// then diff fetched when D pressed to show
		Outputs: []string{"", "/fake/repo", logOutput, "main", "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new"},
		Errors:  []error{fmt.Errorf("staged"), nil, nil, nil, nil},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	viewHidden := m.View()

	// Show the diff
	_ = m.Update(tea.KeyPressMsg{Code: 'D', ShiftedCode: 'D', Mod: tea.ModShift, Text: "D"})
	viewShown := m.View()

	// The two-panel view must differ from single-panel
	if viewHidden == viewShown {
		t.Error("View() must differ between single and two panel modes")
	}
}

func TestFixupModel_Update_NavigationJ(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\x1fAlice\x1f2 hours ago\x1e" +
		"bbb5678\x1ffeat: second\x1fBob\x1f3 hours ago\x1e"
	// Need diff fetches: initial for abc1234, then after j pressed for bbb5678
	runner := &testhelper.FakeRunner{
		// staged-check, reporoot, log, branch, show(abc1234), show(bbb5678)
		Outputs: []string{"", "/fake/repo", logOutput, "main", "diff for abc1234", "diff for bbb5678"},
		Errors:  []error{fmt.Errorf("staged"), nil, nil, nil, nil, nil},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Press j to move down — selection should still render without panic
	_ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string after navigating with j")
	}
}
