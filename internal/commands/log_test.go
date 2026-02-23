package commands_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/commands"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/testhelper"
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
	m, err := commands.NewLogModel(context.Background(), runner, cfg, renderer, ref)
	if err != nil {
		t.Fatalf("NewLogModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestNewLogModel_NoCommits(t *testing.T) {
	m := newFakeLogModel(t, "", "main", "")
	if m == nil {
		t.Fatal("NewLogModel returned nil")
	}
}

func TestNewLogModel_WithCommits(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	m := newFakeLogModel(t, logOutput, "main", "")
	if m == nil {
		t.Fatal("NewLogModel returned nil")
	}
}

func TestNewLogModel_WithRef(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
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
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
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
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", ""},
		Errors:  []error{nil, nil, fmt.Errorf("bad object abc1234")},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")
	require.NoError(t, err)

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestLogModel_TabThenQuitFromRightPanel(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\x1fAlice\x1f2 hours ago\x1e"
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
	logOutput := "abc1234\x1ffeat: first\x1fAlice\x1f2 hours ago\x1e"
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

func TestLogModel_DKeyTogglesDiffPanel(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	// Need extra diff output for when D re-shows the panel
	runner := &testhelper.FakeRunner{
		// log, branch, show (init), show (D re-show)
		Outputs: []string{logOutput, "main",
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new",
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")
	require.NoError(t, err)
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

func TestLogModel_TabNoopWhenDiffHidden(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")
	require.NoError(t, err)
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	view1 := m.View()
	_ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	view2 := m.View()

	// View should be unchanged - Tab had no effect
	if view1 != view2 {
		t.Error("Tab should be a no-op when diff panel is hidden")
	}
}

func TestLogModel_SinglePanelViewWhenDiffHidden(t *testing.T) {
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{
		// log, branch (no diff on init since showDiff=false),
		// then diff fetched when D pressed to show
		Outputs: []string{logOutput, "main", "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new"},
	}
	cfg := config.NewDefault()
	cfg.ShowDiffPanel = false
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")
	require.NoError(t, err)
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

func TestLogModel_Update_NavigationJ(t *testing.T) {
	logOutput := "abc1234\x1ffeat: first\x1fAlice\x1f2 hours ago\x1e" +
		"bbb5678\x1ffeat: second\x1fBob\x1f3 hours ago\x1e"
	// Need diff fetches: initial for abc1234, then after j pressed for bbb5678
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main", "diff for abc1234", "diff for bbb5678"},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")
	require.NoError(t, err)

	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Press j to move down — selection should still render without panic
	_ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	view := m.View()
	if view == "" {
		t.Error("View() returned empty string after navigating with j")
	}
}

func TestLogModel_FKeyTogglesMaximizeView(t *testing.T) {
	t.Parallel()
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	m := newFakeLogModel(t, logOutput, "main", "")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	normalView := m.View()

	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	maximizedView := m.View()
	if normalView == maximizedView {
		t.Error("F should change the layout to maximize diff panel")
	}

	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	restoredView := m.View()
	if restoredView == maximizedView {
		t.Error("second F should restore the two-panel layout")
	}
}

func TestLogModel_MaximizeHintsContainRestore(t *testing.T) {
	t.Parallel()
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	m := newFakeLogModel(t, logOutput, "main", "")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	view := m.View()
	if !strings.Contains(view, "F: restore") {
		t.Errorf("maximize hints must include 'F: restore', got: %q", view)
	}
}

func TestLogModel_BracketKeyChangesLayout(t *testing.T) {
	t.Parallel()
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	m := newFakeLogModel(t, logOutput, "main", "")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	view := m.View()
	if view == "" {
		t.Error("] key handler should not produce empty view")
	}
}

func TestLogModel_BracketLeftKeyChangesLayout(t *testing.T) {
	t.Parallel()
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	// Extra diff output for [ key which may re-trigger resize
	runner := &testhelper.FakeRunner{
		Outputs: []string{logOutput, "main",
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new"},
	}
	cfg := config.NewDefault()
	cfg.PanelRatio = 60 // above 20 so [ takes effect
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")
	require.NoError(t, err)
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	m.Update(tea.KeyPressMsg{Code: '[', Text: "["})
	view := m.View()
	if view == "" {
		t.Error("[ key handler should not produce empty view")
	}
}

func TestLogModel_ResizeWhileMaximized(t *testing.T) {
	t.Parallel()
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	m := newFakeLogModel(t, logOutput, "main", "")
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.Update(tea.KeyPressMsg{Code: 'F', ShiftedCode: 'F', Mod: tea.ModShift, Text: "F"})
	// Resize while maximized exercises diffMaximized branch in resize()
	_ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after resize while maximized")
	}
}

func TestLogModel_BraceKeysAdjustContextLines(t *testing.T) {
	t.Parallel()
	logOutput := "abc1234\x1ffeat: something\x1fAlice\x1f2 hours ago\x1e"
	showDiff := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new"
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			logOutput, "main", // init
			showDiff, // renderSelectedDiff on init
			showDiff, // renderSelectedDiff after }
			showDiff, // renderSelectedDiff after {
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")
	require.NoError(t, err)
	_ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	m.Update(tea.KeyPressMsg{Code: '}', ShiftedCode: '}', Mod: tea.ModShift, Text: "}"})
	m.Update(tea.KeyPressMsg{Code: '{', ShiftedCode: '{', Mod: tea.ModShift, Text: "{"})
}
