package git

import (
	"context"
	"errors"
	"testing"

	"github.com/jetm/jig/internal/testhelper"
)

func TestListUnstagedFilesFiltered_WithPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // git diff --name-status -- foo.go
			"",            // ls-files --others -- foo.go
		},
	}
	files, err := ListUnstagedFilesFiltered(context.Background(), runner, []string{"foo.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
	testhelper.MustHaveCall(t, runner, "diff", "--name-status", "--", "foo.go")
}

func TestListUnstagedFilesFiltered_NoPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\n", // git diff --name-status (no -- separator)
			"",            // ls-files --others
		},
	}
	files, err := ListUnstagedFilesFiltered(context.Background(), runner, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	// Verify no -- separator in the diff call when no paths.
	calls := runner.Calls
	if len(calls) == 0 {
		t.Fatal("expected at least one call")
	}
	for _, arg := range calls[0].Args {
		if arg == "--" {
			t.Error("should not include -- separator when no paths given")
		}
	}
}

func TestListModifiedFilesFiltered_WithPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tbar.go\n"},
	}
	files, err := ListModifiedFilesFiltered(context.Background(), runner, []string{"bar.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0].Path != "bar.go" {
		t.Errorf("expected [{bar.go Modified}], got %v", files)
	}
	testhelper.MustHaveCall(t, runner, "diff", "--name-status", "--", "bar.go")
}

func TestListModifiedFilesFiltered_NoPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tbar.go\n"},
	}
	files, err := ListModifiedFilesFiltered(context.Background(), runner, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestListStagedFilesFiltered_WithPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tbaz.go\n"},
	}
	files, err := ListStagedFilesFiltered(context.Background(), runner, []string{"baz.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0].Path != "baz.go" {
		t.Errorf("expected [{baz.go Modified}], got %v", files)
	}
	testhelper.MustHaveCall(t, runner, "diff", "--cached", "--name-status", "--", "baz.go")
}

func TestListStagedFilesFiltered_NoPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tbaz.go\n"},
	}
	files, err := ListStagedFilesFiltered(context.Background(), runner, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestListModifiedFilesFiltered_DiffError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("diff failed")},
	}
	_, err := ListModifiedFilesFiltered(context.Background(), runner, nil)
	if err == nil {
		t.Fatal("expected error from diff failure")
	}
}

func TestListStagedFilesFiltered_DiffError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("diff failed")},
	}
	_, err := ListStagedFilesFiltered(context.Background(), runner, nil)
	if err == nil {
		t.Fatal("expected error from diff failure")
	}
}

func TestStageFiles_Basic(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	err := StageFiles(context.Background(), runner, []string{"a.go", "b.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testhelper.MustHaveCall(t, runner, "add", "--", "a.go", "b.go")
}

func TestStageFiles_Empty(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{}
	err := StageFiles(context.Background(), runner, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testhelper.MustHaveNoCall(t, runner)
}

func TestStageFiles_Error(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("git add failed")},
	}
	err := StageFiles(context.Background(), runner, []string{"a.go"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDiscardFiles_Basic(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	err := DiscardFiles(context.Background(), runner, []string{"a.go", "b.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testhelper.MustHaveCall(t, runner, "checkout", "--", "a.go", "b.go")
}

func TestDiscardFiles_Empty(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{}
	err := DiscardFiles(context.Background(), runner, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testhelper.MustHaveNoCall(t, runner)
}

func TestDiscardFiles_Error(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("checkout failed")},
	}
	err := DiscardFiles(context.Background(), runner, []string{"a.go"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnstageFiles_Basic(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	err := UnstageFiles(context.Background(), runner, []string{"a.go", "b.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testhelper.MustHaveCall(t, runner, "reset", "HEAD", "--", "a.go", "b.go")
}

func TestUnstageFiles_Empty(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{}
	err := UnstageFiles(context.Background(), runner, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testhelper.MustHaveNoCall(t, runner)
}

func TestUnstageFiles_Error(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("reset failed")},
	}
	err := UnstageFiles(context.Background(), runner, []string{"a.go"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUnstageHunk_Basic(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	patch := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1,3 +1,4 @@\n context\n+added\n context\n"
	err := UnstageHunk(context.Background(), runner, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testhelper.MustHaveCall(t, runner, "apply", "--cached", "--reverse")
	testhelper.MustHaveStdin(t, runner, "@@ -1,3 +1,4 @@")
}

func TestUnstageHunk_Error(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("apply failed")},
	}
	err := UnstageHunk(context.Background(), runner, "patch")
	if err == nil {
		t.Fatal("expected error from apply failure")
	}
}

func TestDiscardHunk_Basic(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	patch := "diff --git a/foo.go b/foo.go\n--- a/foo.go\n+++ b/foo.go\n@@ -1,3 +1,4 @@\n context\n+added\n context\n"
	err := DiscardHunk(context.Background(), runner, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testhelper.MustHaveCall(t, runner, "apply", "--reverse")
	testhelper.MustHaveStdin(t, runner, "@@ -1,3 +1,4 @@")
}

func TestDiscardHunk_Error(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("apply failed")},
	}
	err := DiscardHunk(context.Background(), runner, "patch")
	if err == nil {
		t.Fatal("expected error from apply failure")
	}
}

func TestDiscardHunk_DoesNotUseCached(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	err := DiscardHunk(context.Background(), runner, "patch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify --cached is NOT in the args (only --reverse should be present)
	for _, c := range runner.Calls {
		for _, arg := range c.Args {
			if arg == "--cached" {
				t.Error("DiscardHunk should not use --cached flag")
			}
		}
	}
}

func TestParseNameStatusLine_MalformedLine(t *testing.T) {
	t.Parallel()
	// A line with no tab — should return Modified with path=entire line
	sf := parseNameStatusLine("nodivider")
	if sf.Status != Modified {
		t.Errorf("expected Modified, got %v", sf.Status)
	}
	if sf.Path != "nodivider" {
		t.Errorf("expected path=nodivider, got %q", sf.Path)
	}
}

func TestParseNameStatusLine_AllStatuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line       string
		wantPath   string
		wantStatus FileStatus
	}{
		{"M\tfoo.go", "foo.go", Modified},
		{"A\tnew.go", "new.go", Added},
		{"D\told.go", "old.go", Deleted},
		{"R100\told.go\tnew.go", "new.go", Renamed},
	}
	for _, tt := range tests {
		sf := parseNameStatusLine(tt.line)
		if sf.Path != tt.wantPath {
			t.Errorf("parseNameStatusLine(%q).Path = %q, want %q", tt.line, sf.Path, tt.wantPath)
		}
		if sf.Status != tt.wantStatus {
			t.Errorf("parseNameStatusLine(%q).Status = %v, want %v", tt.line, sf.Status, tt.wantStatus)
		}
	}
}
