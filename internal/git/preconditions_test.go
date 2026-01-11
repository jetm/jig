package git_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/testhelper"
)

func TestHasStagedChanges_True(t *testing.T) {
	r := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("exit 1")},
	}
	if !git.HasStagedChanges(context.Background(), r) {
		t.Fatal("expected HasStagedChanges to return true when diff exits non-zero")
	}
}

func TestHasStagedChanges_False(t *testing.T) {
	r := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{nil},
	}
	if git.HasStagedChanges(context.Background(), r) {
		t.Fatal("expected HasStagedChanges to return false when diff exits zero")
	}
}

func TestHasCommits_True(t *testing.T) {
	r := &testhelper.FakeRunner{
		Outputs: []string{"abc1234"},
		Errors:  []error{nil},
	}
	if !git.HasCommits(context.Background(), r) {
		t.Fatal("expected HasCommits to return true when rev-parse HEAD succeeds")
	}
}

func TestHasCommits_False(t *testing.T) {
	r := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errors.New("exit 128")},
	}
	if git.HasCommits(context.Background(), r) {
		t.Fatal("expected HasCommits to return false when rev-parse HEAD fails")
	}
}

// fakeRunnerWithRoot is a FakeRunner that always returns repoPath for rev-parse --show-toplevel.
type fakeRunnerWithRoot struct {
	repoPath string
}

func (f *fakeRunnerWithRoot) Run(_ context.Context, args ...string) (string, error) {
	if len(args) >= 2 && args[0] == "rev-parse" && args[1] == "--show-toplevel" {
		return f.repoPath, nil
	}
	return "", nil
}

func (f *fakeRunnerWithRoot) RunAllowExitCode(_ context.Context, _ int, _ ...string) (string, error) {
	return "", nil
}

func (f *fakeRunnerWithRoot) RunWithEnv(_ context.Context, _ []string, _ ...string) (string, error) {
	return "", nil
}

func (f *fakeRunnerWithRoot) RunWithStdin(_ context.Context, _ string, _ ...string) (string, error) {
	return "", nil
}

func TestIsRebaseInProgress_True(t *testing.T) {
	repoPath := testhelper.NewTempRepo(t)
	r := &fakeRunnerWithRoot{repoPath: repoPath}

	// Create .git/rebase-merge directory to simulate in-progress rebase
	if err := os.MkdirAll(filepath.Join(repoPath, ".git", "rebase-merge"), 0o755); err != nil {
		t.Fatalf("failed to create rebase-merge dir: %v", err)
	}

	if !git.IsRebaseInProgress(context.Background(), r) {
		t.Fatal("expected IsRebaseInProgress to return true when .git/rebase-merge exists")
	}
}

func TestIsRebaseInProgress_False(t *testing.T) {
	repoPath := testhelper.NewTempRepo(t)
	r := &fakeRunnerWithRoot{repoPath: repoPath}

	if git.IsRebaseInProgress(context.Background(), r) {
		t.Fatal("expected IsRebaseInProgress to return false in a normal repo")
	}
}

func TestIsMergeInProgress_True(t *testing.T) {
	repoPath := testhelper.NewTempRepo(t)
	r := &fakeRunnerWithRoot{repoPath: repoPath}

	// Create .git/MERGE_HEAD file to simulate in-progress merge
	mergeHead := filepath.Join(repoPath, ".git", "MERGE_HEAD")
	if err := os.WriteFile(mergeHead, []byte("abc1234\n"), 0o644); err != nil {
		t.Fatalf("failed to create MERGE_HEAD: %v", err)
	}

	if !git.IsMergeInProgress(context.Background(), r) {
		t.Fatal("expected IsMergeInProgress to return true when .git/MERGE_HEAD exists")
	}
}

func TestIsMergeInProgress_False(t *testing.T) {
	repoPath := testhelper.NewTempRepo(t)
	r := &fakeRunnerWithRoot{repoPath: repoPath}

	if git.IsMergeInProgress(context.Background(), r) {
		t.Fatal("expected IsMergeInProgress to return false in a normal repo")
	}
}
