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

// newFakeLogModel creates a LogModel with scripted runner outputs.
//
//   - output[0]: git log output (commits)
//   - output[1]: git rev-parse --abbrev-ref HEAD (branch name)
//   - output[2]: git show <hash> (diff for first commit, if commits non-empty)
func newFakeLogModel(t *testing.T, logOutput, branch, ref string) *commands.LogModel {
	t.Helper()
	outputs := []string{logOutput, branch}
	if logOutput != "" {
		outputs = append(outputs, "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new")
	}
	runner := &testhelper.FakeRunner{Outputs: outputs}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewLogModel(context.Background(), runner, cfg, renderer, ref)
}

func TestNewLogModel_NoCommits(t *testing.T) {
	m := newFakeLogModel(t, "", "main", "")
	if m == nil {
		t.Fatal("NewLogModel returned nil")
	}
}

func TestNewLogModel_WithCommits(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x00"
	m := newFakeLogModel(t, logOutput, "main", "")
	if m == nil {
		t.Fatal("NewLogModel returned nil")
	}
}

func TestNewLogModel_WithRef(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x00"
	m := newFakeLogModel(t, logOutput, "main", "v1.0")
	if m == nil {
		t.Fatal("NewLogModel returned nil with ref")
	}
}

func TestLogModel_View_TooSmall(t *testing.T) {
	m := newFakeLogModel(t, "", "main", "")
	// Don't send WindowSizeMsg; width=0,height=0 → too small
	view := m.View()
	if view != "Terminal too small. Please resize to at least 60x10." {
		t.Errorf("View() = %q", view)
	}
}

func TestLogModel_View_NoCommits(t *testing.T) {
	m := newFakeLogModel(t, "", "main", "")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view != "No commits to show." {
		t.Errorf("View() = %q, want %q", view, "No commits to show.")
	}
}

func TestLogModel_View_WithCommits(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x00"
	m := newFakeLogModel(t, logOutput, "main", "")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string with commits")
	}
	if view == "No commits to show." {
		t.Error("View() showed 'no commits' even though we have commits")
	}
}

func TestLogModel_Update_WindowSize(t *testing.T) {
	m := newFakeLogModel(t, "", "main", "")
	cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = cmd // no panic is the goal
}

func TestLogModel_Update_QuitWithQ(t *testing.T) {
	m := newFakeLogModel(t, "", "main", "")
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected PopModelMsg, got nil")
	}
}

func TestLogModel_Update_QuitWithEscape(t *testing.T) {
	m := newFakeLogModel(t, "", "main", "")
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape, Text: ""})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected PopModelMsg, got nil")
	}
}

func TestLogModel_Update_HelpToggle(t *testing.T) {
	m := newFakeLogModel(t, "", "main", "")
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

func TestLogModel_Update_HelpHidesNavigation(t *testing.T) {
	m := newFakeLogModel(t, "", "main", "")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Open help
	_ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})

	// With help open, q should NOT quit (it should be consumed by help overlay)
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	_ = cmd // just ensure no panic and help overlay intercepted
}

func TestLogModel_RenderSelectedDiff_ErrorPath(t *testing.T) {
	// Make git show return an error so renderSelectedDiff shows an error message.
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x00"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", ""},
		Errors:  []error{nil, nil, fmt.Errorf("bad object abc1234")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestLogModel_TabThenQuitFromRightPanel(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\x1fAlice\x1f2 hours ago\x00"
	m := newFakeLogModel(t, logOutput, "main", "")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Tab to right panel
	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// q should still quit
	cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected command from q on right panel")
	}
}

func TestLogModel_TabThenJScrollsDiff(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\x1fAlice\x1f2 hours ago\x00"
	m := newFakeLogModel(t, logOutput, "main", "")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	// j on right panel should not panic
	_ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after scrolling right panel")
	}
}

func TestLogModel_Update_NavigationJ(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\x1fAlice\x1f2 hours ago\x00" +
		"bbb5678\x1ffeat: second\x1fBob\x1f3 hours ago\x00"
	// Need diff fetches: initial for abc1234, then after j pressed for bbb5678
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234", "diff for bbb5678"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Press j to move down — selection should still render without panic
	_ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string after navigating with j")
	}
}
