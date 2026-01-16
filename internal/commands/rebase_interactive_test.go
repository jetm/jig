package commands_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/commands"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/testhelper"
)

// newFakeRebaseModel creates a RebaseInteractiveModel in standalone mode with scripted git outputs.
// The FakeRunner receives calls in order:
//   - output[0]: git log --reverse ... (commits for rebase)
//   - output[1]: git rev-parse --abbrev-ref HEAD (branch name)
//   - output[2+]: git show <hash> (diff for first commit, if any)
func newFakeRebaseModel(t *testing.T, logOutput, branch, base string) *commands.RebaseInteractiveModel {
	t.Helper()
	outputs := []string{logOutput, branch}
	if logOutput != "" {
		outputs = append(outputs, "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new")
	}
	runner := &testhelper.FakeRunner{Outputs: outputs}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, base, "")
}

func TestNewRebaseInteractiveModel_NoCommits(t *testing.T) {
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	if m == nil {
		t.Fatal("NewRebaseInteractiveModel returned nil")
	}
}

func TestNewRebaseInteractiveModel_WithCommits(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	m := newFakeRebaseModel(t, logOutput, "main", "HEAD~2")
	if m == nil {
		t.Fatal("NewRebaseInteractiveModel returned nil")
	}
}

func TestNewRebaseInteractiveModel_DefaultBase(t *testing.T) {
	// Empty base should use default of HEAD~10
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "", "")
	if m == nil {
		t.Fatal("NewRebaseInteractiveModel returned nil")
	}
}

func TestRebaseInteractiveModel_View_TooSmall(t *testing.T) {
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	// No WindowSizeMsg sent → width=0,height=0 → too small
	view := m.View()
	if view != "Terminal too small. Please resize to at least 60x10." {
		t.Errorf("View() = %q", view)
	}
}

func TestRebaseInteractiveModel_View_NoCommits(t *testing.T) {
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view != "No commits to rebase. Specify a valid base revision." {
		t.Errorf("View() = %q, want no-commits message", view)
	}
}

func TestRebaseInteractiveModel_View_WithCommits(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	m := newFakeRebaseModel(t, logOutput, "main", "HEAD~2")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string with commits")
	}
	if view == "No commits to rebase. Specify a valid base revision." {
		t.Error("View() showed no-commits message even though we have commits")
	}
}

func TestRebaseInteractiveModel_Update_WindowSize(t *testing.T) {
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = cmd // no panic is the goal
}

func TestRebaseInteractiveModel_Update_QuitWithQ(t *testing.T) {
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected PopModelMsg, got nil")
	}
}

func TestRebaseInteractiveModel_Update_QuitWithEscape(t *testing.T) {
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected PopModelMsg, got nil")
	}
}

func TestRebaseInteractiveModel_Update_HelpToggle(t *testing.T) {
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view1 := m.View()

	_ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view2 := m.View()

	if view1 == view2 {
		t.Error("View() should differ after toggling help overlay")
	}
}

func TestRebaseInteractiveModel_Update_HelpHidesNavigation(t *testing.T) {
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Open help
	_ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})

	// With help open, q should NOT quit (intercepted by help overlay)
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	_ = cmd // just no panic
}

func TestRebaseInteractiveModel_Update_EnterNoCommits(t *testing.T) {
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = cmd // should be nil or a cmd; no panic
}

func TestRebaseInteractiveModel_Update_EnterWithCommits_Success(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff content", ""},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~1", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a PopModelMsg command after successful rebase, got nil")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected PopModelMsg, got nil")
	}
}

func TestRebaseInteractiveModel_Update_EnterWithCommits_Failure(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff content", ""},
		Errors:  []error{nil, nil, nil, fmt.Errorf("rebase conflict")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~1", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected an error-message command after rebase failure, got nil")
	}
}

func TestRebaseInteractiveModel_Update_SpaceCyclesAction(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff content"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~1", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view1 := m.View()

	// Press space to cycle action (pick → reword)
	_ = m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	view2 := m.View()

	// View should change because the action label changes
	if view1 == view2 {
		t.Error("View() should differ after cycling action with space")
	}
}

func TestRebaseInteractiveModel_Update_SetActionKeys(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\n"

	cases := []struct {
		key  rune
		text string
	}{
		{'p', "p"},
		{'r', "r"},
		{'e', "e"},
		{'s', "s"},
		{'f', "f"},
		{'d', "d"},
	}

	for _, tc := range cases {
		t.Run(string(tc.key), func(t *testing.T) {
			runner := &testhelper.FakeRunner{
				Outputs: []string{logOutput, "main", "diff content"},
			}
			cfg := config.NewDefault()
			renderer := &diff.PlainRenderer{}
			m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~1", "")

			_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			cmd := m.Update(tea.KeyPressMsg{Code: tc.key, Text: tc.text})
			_ = cmd
			view := m.View()
			if view == "" {
				t.Errorf("View() empty after pressing %q", tc.key)
			}
		})
	}
}

func TestRebaseInteractiveModel_Update_MoveUp(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234", "diff for bbb5678", "diff for bbb5678"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~2", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate down to second commit first
	_ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Press K to move commit up
	_ = m.Update(tea.KeyPressMsg{Code: 'k', ShiftedCode: 'K', Mod: tea.ModShift, Text: "K"})

	view := m.View()
	if view == "" {
		t.Error("View() returned empty string after moveUp")
	}
}

func TestRebaseInteractiveModel_Update_MoveDown(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234", "diff for abc1234"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~2", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press J to move first commit down
	_ = m.Update(tea.KeyPressMsg{Code: 'j', ShiftedCode: 'J', Mod: tea.ModShift, Text: "J"})

	view := m.View()
	if view == "" {
		t.Error("View() returned empty string after moveDown")
	}
}

func TestRebaseInteractiveModel_Update_MoveUp_AtTop(_ *testing.T) {
	logOutput := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~2", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// K at top (selectedIdx=0) should be a no-op
	_ = m.Update(tea.KeyPressMsg{Code: 'k', ShiftedCode: 'K', Mod: tea.ModShift, Text: "K"})
	_ = m.View() // no panic
}

func TestRebaseInteractiveModel_Update_MoveDown_AtBottom(_ *testing.T) {
	logOutput := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234", "diff for bbb5678"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~2", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// J at bottom should be a no-op
	_ = m.Update(tea.KeyPressMsg{Code: 'j', ShiftedCode: 'J', Mod: tea.ModShift, Text: "J"})
	_ = m.View() // no panic
}

func TestRebaseInteractiveModel_Update_CtrlUp(_ *testing.T) {
	logOutput := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234", "diff for bbb5678", "diff for bbb5678"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~2", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	// Ctrl+Up should move commit up
	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl})
	_ = m.View() // no panic
}

func TestRebaseInteractiveModel_Update_CtrlDown(_ *testing.T) {
	logOutput := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234", "diff for abc1234"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~2", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Ctrl+Down should move commit down
	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl})
	_ = m.View() // no panic
}

func TestRebaseInteractiveModel_Update_UpArrowWithoutCtrl(_ *testing.T) {
	logOutput := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~2", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Plain up arrow (no Ctrl) should navigate list, not move commit
	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	_ = m.View() // no panic
}

func TestRebaseInteractiveModel_RenderSelectedDiff_ErrorPath(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", ""},
		Errors:  []error{nil, nil, fmt.Errorf("bad object")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~1", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string even with diff error")
	}
}

func TestRebaseInteractiveModel_CycleAction_NoCommits(t *testing.T) {
	// Space with no commits should be a no-op (guard: selectedIdx >= len(entries))
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	cmd := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	_ = cmd // no panic, no effect
}

func TestRebaseInteractiveModel_SetAction_NoCommits(t *testing.T) {
	// Action keys with no commits should be a no-op
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	cmd := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	_ = cmd // no panic, no effect
}

func TestRebaseInteractiveModel_TabThenQuitFromRightPanel(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\n"
	m := newFakeRebaseModel(t, logOutput, "main", "HEAD~1")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected command from q on right panel")
	}
}

func TestRebaseInteractiveModel_ActionKeysWorkFromRightPanel(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\n"
	m := newFakeRebaseModel(t, logOutput, "main", "HEAD~1")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// Action keys (p/r/e/s/f/d) should still work from right panel
	_ = m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after action key from right panel")
	}
}

func TestRebaseInteractiveModel_Update_NavigationJ(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\nbbb5678\x1ffix: second\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234", "diff for bbb5678"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~2", "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string after navigating with j")
	}
}

// --- Editor mode tests (tasks 2.1, 3.1, 3.2) ---

func TestNewRebaseInteractiveModel_EditorMode_ParsesTodoFile(t *testing.T) {
	t.Parallel()
	// Create a temp todo file with native git format
	dir := t.TempDir()
	todoPath := filepath.Join(dir, "git-rebase-todo")
	todoContent := "pick abc1234 feat: first\nreword bbb5678 fix: second\n"
	require.NoError(t, os.WriteFile(todoPath, []byte(todoContent), 0o644))

	// Runner only needs branch name (no git log call in editor mode)
	runner := &testhelper.FakeRunner{Outputs: []string{"main", "diff content"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "", todoPath)

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	// Should NOT show the no-commits message since we parsed the todo file
	assert.NotEqual(t, "No commits to rebase. Specify a valid base revision.", view)
	assert.NotEmpty(t, view)
}

func TestRebaseInteractiveModel_EditorMode_ConfirmWritesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	todoPath := filepath.Join(dir, "git-rebase-todo")
	todoContent := "pick abc1234 feat: first\npick bbb5678 fix: second\n"
	require.NoError(t, os.WriteFile(todoPath, []byte(todoContent), 0o644))

	runner := &testhelper.FakeRunner{Outputs: []string{"main", "diff content"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "", todoPath)

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Change second entry's action to squash
	_ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})

	// Confirm rebase (Enter)
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd, "confirmRebase should return a command")

	// The todo file should be written back with the modified entries
	written, err := os.ReadFile(todoPath)
	require.NoError(t, err)

	expected := git.FormatTodo([]git.RebaseTodoEntry{
		{Action: git.ActionPick, Hash: "abc1234", Subject: "feat: first"},
		{Action: git.ActionSquash, Hash: "bbb5678", Subject: "fix: second"},
	})
	assert.Equal(t, expected, string(written))

	// Should NOT have called ExecuteRebaseInteractive (no rebase -i call)
	for _, call := range runner.Calls {
		for _, arg := range call.Args {
			assert.NotEqual(t, "rebase", arg, "editor mode should not call git rebase")
		}
	}
}

func TestRebaseInteractiveModel_EditorMode_AbortExitsNonZero(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	todoPath := filepath.Join(dir, "git-rebase-todo")
	todoContent := "pick abc1234 feat: first\n"
	require.NoError(t, os.WriteFile(todoPath, []byte(todoContent), 0o644))

	runner := &testhelper.FakeRunner{Outputs: []string{"main", "diff content"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "", todoPath)

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press q to abort
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	require.NotNil(t, cmd)
	msg := cmd()

	// In editor mode, abort should emit AbortEditorMsg (not PopModelMsg)
	_, isAbort := msg.(commands.AbortEditorMsg)
	assert.True(t, isAbort, "expected AbortEditorMsg, got %T", msg)
}

func TestRebaseInteractiveModel_StandaloneMode_QuitEmitsPopModel(t *testing.T) {
	t.Parallel()
	m := newFakeRebaseModel(t, "", "main", "HEAD~5")

	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	require.NotNil(t, cmd)
	msg := cmd()

	// In standalone mode, quit should still emit PopModelMsg
	_, isPop := msg.(app.PopModelMsg)
	assert.True(t, isPop, "expected PopModelMsg in standalone mode, got %T", msg)
}
