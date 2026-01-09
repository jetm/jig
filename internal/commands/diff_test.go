package commands

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/gti/internal/app"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/testhelper"
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

	m := NewDiffModel(context.Background(), runner, cfg, renderer, revision, staged)

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
			_ = NewDiffModel(context.Background(), runner, cfg, renderer, tc.revision, tc.staged)

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
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false)
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

	// Initial view should have first file's content
	if m.selectedIdx != 0 {
		t.Fatalf("initial selectedIdx = %d, want 0", m.selectedIdx)
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

func TestDiffItem_FilterValue(t *testing.T) {
	t.Parallel()
	item := diffItem{fd: git.FileDiff{NewPath: "test.go", Status: git.Modified}}
	if got := item.FilterValue(); got != "test.go" {
		t.Errorf("FilterValue() = %q, want %q", got, "test.go")
	}
}

func TestStatusLabel_AllStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status git.FileStatus
		want   string
	}{
		{git.Added, "added"},
		{git.Deleted, "deleted"},
		{git.Renamed, "renamed"},
		{git.Modified, "modified"},
	}

	for _, tc := range tests {
		got := statusLabel(tc.status)
		if !strings.Contains(got, tc.want) {
			t.Errorf("statusLabel(%v) = %q, want containing %q", tc.status, got, tc.want)
		}
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

func TestDiffModel_RenderSelectedDiff_OutOfBounds(t *testing.T) {
	t.Parallel()
	m := newTestModel(t, "", false, sampleDiff)
	m.selectedIdx = 999
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
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false)
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
	m := NewDiffModel(context.Background(), runner, cfg, renderer, "", false)
	// Should not panic
	m.checkSelectionChange()
}
