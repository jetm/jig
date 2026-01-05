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

// newTestResetModel builds a ResetModel backed by a FakeRunner.
// nameStatus is returned for git diff --cached --name-status.
func newTestResetModel(t *testing.T, nameStatus string) *ResetModel {
	t.Helper()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			nameStatus, // git diff --cached --name-status
			"main",     // git rev-parse --abbrev-ref HEAD (BranchName)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return NewResetModel(context.Background(), runner, cfg, renderer)
}

func TestNewResetModel_EmptyFiles(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "")
	if len(m.files) != 0 {
		t.Errorf("expected 0 files, got %d", len(m.files))
	}
}

func TestNewResetModel_WithFiles(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	if len(m.files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(m.files))
	}
	if m.files[0].Path != "foo.go" {
		t.Errorf("expected foo.go, got %q", m.files[0].Path)
	}
}

func TestResetModel_QuitOnQ(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
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
		t.Error("MutatedGit should be false when quitting without unstaging")
	}
}

func TestResetModel_QuitOnEsc(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
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

func TestResetModel_HelpToggle(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
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

func TestResetModel_HelpVisibleBlocksKeys(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
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

func TestResetModel_SpaceTogglesSelection(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	if m.selected["foo.go"] {
		t.Fatal("foo.go should not be selected initially")
	}

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if !m.selected["foo.go"] {
		t.Error("foo.go should be selected after pressing Space")
	}

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if m.selected["foo.go"] {
		t.Error("foo.go should be deselected after pressing Space again")
	}
}

func TestResetModel_SelectAll(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\nM\tbar.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})

	for _, f := range m.files {
		if !m.selected[f.Path] {
			t.Errorf("file %q should be selected after pressing a", f.Path)
		}
	}
}

func TestResetModel_DeselectAll(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\nM\tbar.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})

	for _, f := range m.files {
		if m.selected[f.Path] {
			t.Errorf("file %q should not be selected after pressing d", f.Path)
		}
	}
}

func TestResetModel_EnterUnstagesSelected(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch name
			"",            // git reset HEAD (unstage call)
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Select the file
	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})

	// Press Enter
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command from pressing Enter")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after unstaging")
	}
	testhelper.MustHaveCall(t, runner, "reset", "HEAD", "--", "foo.go")
}

func TestResetModel_EnterUnstagesFocusedWhenNoneSelected(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch
			"",            // git reset HEAD
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// Press Enter with no selection — should unstage focused file
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from Enter")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if !pop.MutatedGit {
		t.Error("MutatedGit should be true after unstaging focused file")
	}
}

func TestResetModel_View_TwoPanel(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
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

func TestResetModel_View_EmptyState(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "Nothing to unstage") {
		t.Errorf("View() with no files should say 'Nothing to unstage', got: %q", view)
	}
}

func TestResetModel_View_TooSmall(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 30
	m.height = 5

	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Errorf("View() with small terminal should mention 'too small', got: %q", view)
	}
}

func TestResetModel_View_HelpVisible(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	view := m.View()
	if !strings.Contains(view, "Navigation") {
		t.Error("help overlay should contain 'Navigation'")
	}
}

func TestResetModel_WindowSizeMsg(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")

	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})

	if m.width != 200 {
		t.Errorf("width = %d, want 200", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestResetModel_StatusBarShowsBranch(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "M\tfoo.go\n")
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "main") {
		t.Error("View() should contain branch name 'main'")
	}
}

func TestResetItem_Methods(t *testing.T) {
	t.Parallel()
	item := resetItem{sf: git.StatusFile{Path: "test.go", Status: git.Modified}, selected: false}
	if !strings.Contains(item.Title(), "test.go") {
		t.Errorf("Title() should contain 'test.go', got %q", item.Title())
	}
	if item.FilterValue() != "test.go" {
		t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), "test.go")
	}
	if !strings.Contains(item.Description(), "modified") {
		t.Errorf("Description() should contain 'modified', got %q", item.Description())
	}

	itemSelected := resetItem{sf: git.StatusFile{Path: "test.go", Status: git.Modified}, selected: true}
	if !strings.Contains(itemSelected.Title(), "test.go") {
		t.Errorf("selected Title() should contain 'test.go', got %q", itemSelected.Title())
	}
}

func TestResetModel_EnterWithNoFiles(t *testing.T) {
	t.Parallel()
	m := newTestResetModel(t, "")
	m.width = 120
	m.height = 40

	// No files, Enter should return nil (nothing to unstage)
	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	_ = cmd // no panic expected
}

func TestResetModel_UnstageError(t *testing.T) {
	t.Parallel()
	// Call order in NewResetModel:
	//   1. ListStagedFiles: diff --cached --name-status
	//   2. BranchName: rev-parse --abbrev-ref HEAD
	//   3. renderSelectedDiff: diff --cached -- foo.go
	// Then after Space+Enter:
	//   4. UnstageFiles: reset HEAD -- foo.go (this should fail)
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // 1. diff --cached --name-status
			"main",        // 2. branch name
			"",            // 3. diff preview
			"",            // 4. git reset HEAD (error)
		},
		Errors: []error{nil, nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})

	cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a command even on error")
	}
	msg := cmd()
	pop, ok := msg.(app.PopModelMsg)
	if !ok {
		t.Fatalf("expected PopModelMsg, got %T", msg)
	}
	if pop.MutatedGit {
		t.Error("MutatedGit should be false when unstaging fails")
	}
}

func TestResetModel_RenderSelectedDiff_WithDiff(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch
			// renderSelectedDiff called in NewResetModel:
			"diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	view := m.View()
	if !strings.Contains(view, "foo.go") {
		t.Error("View() should contain file name")
	}
}

func TestResetModel_RenderSelectedDiff_DiffError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // diff --cached --name-status
			"main",        // branch
			"",            // renderSelectedDiff: diff call
		},
		Errors: []error{nil, nil, context.DeadlineExceeded},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after diff error")
	}
}

func TestResetModel_KeyJForwardsToList(t *testing.T) {
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
	m := NewResetModel(context.Background(), runner, cfg, renderer)
	m.width = 120
	m.height = 40

	// j key forwards to list; no panic expected
	cmd := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_ = cmd
}
