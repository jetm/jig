package git_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/testhelper"
)

func TestRepoRoot(t *testing.T) {
	f := &testhelper.FakeRunner{Outputs: []string{"/home/user/repo"}}
	ctx := context.Background()
	got, err := git.RepoRoot(ctx, f)
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	if got != "/home/user/repo" {
		t.Errorf("got %q, want %q", got, "/home/user/repo")
	}
	testhelper.MustHaveCall(t, f, "rev-parse", "--show-toplevel")
}

func TestBranchName(t *testing.T) {
	f := &testhelper.FakeRunner{Outputs: []string{"main\n"}}
	ctx := context.Background()
	got, err := git.BranchName(ctx, f)
	if err != nil {
		t.Fatalf("BranchName: %v", err)
	}
	if got != "main" {
		t.Errorf("got %q, want %q", got, "main")
	}
	testhelper.MustHaveCall(t, f, "rev-parse", "--abbrev-ref", "HEAD")
}

func TestBranchName_Detached(t *testing.T) {
	f := &testhelper.FakeRunner{Outputs: []string{"HEAD\n"}}
	ctx := context.Background()
	got, err := git.BranchName(ctx, f)
	if err != nil {
		t.Fatalf("BranchName detached: %v", err)
	}
	if got != "HEAD" {
		t.Errorf("got %q, want %q", got, "HEAD")
	}
	testhelper.MustHaveCall(t, f, "rev-parse", "--abbrev-ref", "HEAD")
}

func TestRepoRoot_Error(t *testing.T) {
	f := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("not a git repo")},
	}
	_, err := git.RepoRoot(context.Background(), f)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBranchName_Error(t *testing.T) {
	f := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("no HEAD")},
	}
	_, err := git.BranchName(context.Background(), f)
	if err == nil {
		t.Fatal("expected error")
	}
}
