package git

import (
	"context"
	"errors"
	"testing"

	"github.com/jetm/jig/internal/testhelper"
)

func TestListUnstagedFiles_ModifiedAndUntracked(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"M\tfoo.go\nD\tbar.go\n", // git diff --name-status
			"new.go\n",               // git ls-files --others
		},
	}
	files, err := ListUnstagedFiles(context.Background(), runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(files), files)
	}
	if files[0].Path != "foo.go" || files[0].Status != Modified {
		t.Errorf("files[0] = %+v, want {foo.go, Modified}", files[0])
	}
	if files[1].Path != "bar.go" || files[1].Status != Deleted {
		t.Errorf("files[1] = %+v, want {bar.go, Deleted}", files[1])
	}
	if files[2].Path != "new.go" || files[2].Status != Added {
		t.Errorf("files[2] = %+v, want {new.go, Added}", files[2])
	}
}

func TestListUnstagedFiles_Empty(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", ""},
	}
	files, err := ListUnstagedFiles(context.Background(), runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestListUnstagedFiles_DiffError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("diff failed")},
	}
	_, err := ListUnstagedFiles(context.Background(), runner)
	if err == nil {
		t.Fatal("expected error from diff failure")
	}
}

func TestListUnstagedFiles_UntrackedError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go", ""},
		Errors:  []error{nil, errors.New("ls-files failed")},
	}
	_, err := ListUnstagedFiles(context.Background(), runner)
	if err == nil {
		t.Fatal("expected error from ls-files failure")
	}
}

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

func TestListUnstagedFiles_Renamed(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"R100\told.go\tnew.go\n", ""},
	}
	files, err := ListUnstagedFiles(context.Background(), runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Status != Renamed {
		t.Errorf("expected Renamed, got %v", files[0].Status)
	}
	if files[0].Path != "new.go" {
		t.Errorf("expected new.go, got %q", files[0].Path)
	}
}

func TestListUnstagedFiles_AddedFile(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"A\tnew_staged.go\n", ""},
	}
	files, err := ListUnstagedFiles(context.Background(), runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0].Status != Added {
		t.Errorf("expected Added file, got %+v", files)
	}
}

func TestListModifiedFiles_Basic(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\ta.go\nD\tb.go\n"},
	}
	files, err := ListModifiedFiles(context.Background(), runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Path != "a.go" || files[0].Status != Modified {
		t.Errorf("files[0] = %+v", files[0])
	}
	if files[1].Path != "b.go" || files[1].Status != Deleted {
		t.Errorf("files[1] = %+v", files[1])
	}
}

func TestListModifiedFiles_Empty(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	files, err := ListModifiedFiles(context.Background(), runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestListModifiedFiles_Error(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("diff failed")},
	}
	_, err := ListModifiedFiles(context.Background(), runner)
	if err == nil {
		t.Fatal("expected error")
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

func TestListStagedFiles_MultipleStatuses(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\nA\tnew.go\nD\told.go\nR100\ta.go\tb.go\n"},
	}
	files, err := ListStagedFiles(context.Background(), runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 4 {
		t.Fatalf("expected 4 files, got %d: %v", len(files), files)
	}
	if files[0].Path != "foo.go" || files[0].Status != Modified {
		t.Errorf("files[0] = %+v, want {foo.go, Modified}", files[0])
	}
	if files[1].Path != "new.go" || files[1].Status != Added {
		t.Errorf("files[1] = %+v, want {new.go, Added}", files[1])
	}
	if files[2].Path != "old.go" || files[2].Status != Deleted {
		t.Errorf("files[2] = %+v, want {old.go, Deleted}", files[2])
	}
	if files[3].Path != "b.go" || files[3].Status != Renamed {
		t.Errorf("files[3] = %+v, want {b.go, Renamed}", files[3])
	}
	testhelper.MustHaveCall(t, runner, "diff", "--cached", "--name-status")
}

func TestListStagedFiles_Empty(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	files, err := ListStagedFiles(context.Background(), runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestListStagedFiles_Error(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("diff cached failed")},
	}
	_, err := ListStagedFiles(context.Background(), runner)
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
