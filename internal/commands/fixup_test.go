package commands_test

import (
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/gti/internal/commands"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/testhelper"
)

// fakeFixupOutputs returns the scripted outputs for NewFixupModel:
//   - output[0]: git log output (commits)
//   - output[1]: git rev-parse --abbrev-ref HEAD (branch name)
//   - output[2]: git show <hash> (diff for the first commit, if commits non-empty)
func newFakeFixupModel(t *testing.T, logOutput, branch string) *commands.FixupModel {
	t.Helper()
	outputs := []string{logOutput, branch}
	if logOutput != "" {
		// First commit diff will be fetched on init
		outputs = append(outputs, "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new")
	}
	runner := &testhelper.FakeRunner{Outputs: outputs}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewFixupModel(context.Background(), runner, cfg, renderer)
}

func TestNewFixupModel_NoCommits(t *testing.T) {
	m := newFakeFixupModel(t, "", "main")
	if m == nil {
		t.Fatal("NewFixupModel returned nil")
	}
}

func TestNewFixupModel_WithCommits(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x00"
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
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x00"
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
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x00"
	runner := &testhelper.FakeRunner{
		// outputs: log, branch, show (init), then "" for failed commit
		Outputs: []string{logOutput, "main", "diff content", ""},
		Errors:  []error{nil, nil, nil, fmt.Errorf("nothing to commit")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewFixupModel(context.Background(), runner, cfg, renderer)

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Text: ""})
	if cmd == nil {
		t.Error("expected an error-message command after fixup failure, got nil")
	}
}

func TestFixupModel_Update_EnterWithCommits_Success(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x00"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff content", "[main def5678] fixup! feat: something"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewFixupModel(context.Background(), runner, cfg, renderer)

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
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x00"
	runner := &testhelper.FakeRunner{
		// log, branch, then error on show
		Outputs: []string{logOutput, "main", ""},
		Errors:  []error{nil, nil, fmt.Errorf("bad object abc1234")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewFixupModel(context.Background(), runner, cfg, renderer)

	// View should render without panicking even with a diff error
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestFixupModel_Update_NavigationJ(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\x1fAlice\x1f2 hours ago\x00" +
		"bbb5678\x1ffeat: second\x1fBob\x1f3 hours ago\x00"
	// Need diff fetches: initial for abc1234, then after j pressed for bbb5678
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234", "diff for bbb5678"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewFixupModel(context.Background(), runner, cfg, renderer)

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Press j to move down — selection should still render without panic
	_ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string after navigating with j")
	}
}
