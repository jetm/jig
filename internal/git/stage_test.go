package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestPathBeyondSymlink_DirectSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create real/file.txt
	realDir := filepath.Join(dir, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "file.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create link -> real (symlink)
	if err := os.Symlink(realDir, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	if !pathBeyondSymlink(dir, "link/file.txt") {
		t.Error("expected link/file.txt to be beyond a symlink")
	}
	if pathBeyondSymlink(dir, "real/file.txt") {
		t.Error("expected real/file.txt to NOT be beyond a symlink")
	}
}

func TestPathBeyondSymlink_NestedSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a/b/c/file.txt
	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Replace a/b with a symlink: a/sym -> a/b/c
	if err := os.Symlink(filepath.Join(dir, "a", "b", "c"), filepath.Join(dir, "a", "sym")); err != nil {
		t.Fatal(err)
	}

	if !pathBeyondSymlink(dir, "a/sym/file.txt") {
		t.Error("expected a/sym/file.txt to be beyond a symlink")
	}
	if pathBeyondSymlink(dir, "a/b/c/file.txt") {
		t.Error("expected a/b/c/file.txt to NOT be beyond a symlink")
	}
}

func TestPathBeyondSymlink_TopLevelFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if pathBeyondSymlink(dir, "file.txt") {
		t.Error("top-level file should not be beyond a symlink")
	}
}

func TestFilterBeyondSymlinks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create real/file.txt and link -> real
	realDir := filepath.Join(dir, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "file.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDir, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	files := []StatusFile{
		{Path: "real/file.txt", Status: Added},
		{Path: "link/file.txt", Status: Added},
		{Path: "top.txt", Status: Added},
	}

	got := filterBeyondSymlinks(dir, files)

	if len(got) != 2 {
		t.Fatalf("expected 2 files after filtering, got %d: %v", len(got), got)
	}
	if got[0].Path != "real/file.txt" {
		t.Errorf("expected first file to be real/file.txt, got %q", got[0].Path)
	}
	if got[1].Path != "top.txt" {
		t.Errorf("expected second file to be top.txt, got %q", got[1].Path)
	}
}

func TestListUnstagedFilesFiltered_ExcludesSymlinkedUntracked(t *testing.T) {
	t.Parallel()

	// Simulate: git diff returns nothing, ls-files returns files including one behind a symlink,
	// rev-parse returns a tmpdir with a symlink setup.
	dir := t.TempDir()

	// Create real/file.go and link -> real
	realDir := filepath.Join(dir, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "file.go"), []byte("package x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDir, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",                             // git diff --name-status
			"real/file.go\nlink/file.go\n", // ls-files --others --exclude-standard
			dir,                            // rev-parse --show-toplevel
		},
	}

	files, err := ListUnstagedFilesFiltered(context.Background(), runner, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only include real/file.go, not link/file.go
	if len(files) != 1 {
		t.Fatalf("expected 1 file after symlink filtering, got %d: %v", len(files), files)
	}
	if files[0].Path != "real/file.go" {
		t.Errorf("expected real/file.go, got %q", files[0].Path)
	}
}
