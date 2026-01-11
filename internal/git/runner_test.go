package git_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jetm/gti/internal/git"
)

// initTempRepo creates a temporary git repository and changes the working directory to it.
// It returns a cleanup function that restores the original working directory.
func initTempRepo(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	// Initialize the repo
	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	// Configure local user so commits work in CI
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config name: %v: %s", err, out)
	}
	// Create an initial commit so the repo is valid
	if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("git initial commit: %v: %s", err, out)
	}
}

func TestExecRunner_Run_Version(t *testing.T) {
	initTempRepo(t)
	ctx := context.Background()
	runner, err := git.NewExecRunner(ctx)
	if err != nil {
		t.Fatalf("NewExecRunner: %v", err)
	}
	out, err := runner.Run(ctx, "--version")
	if err != nil {
		t.Fatalf("Run --version: %v", err)
	}
	if !strings.Contains(out, "git version") {
		t.Errorf("expected output to contain %q, got %q", "git version", out)
	}
}

func TestExecRunner_Run_ExecError(t *testing.T) {
	initTempRepo(t)
	ctx := context.Background()
	runner, err := git.NewExecRunner(ctx)
	if err != nil {
		t.Fatalf("NewExecRunner: %v", err)
	}
	_, err = runner.Run(ctx, "not-a-real-command")
	if err == nil {
		t.Fatal("expected error for invalid git command, got nil")
	}
	var execErr *git.ExecError
	if !errors.As(err, &execErr) {
		t.Fatalf("expected *git.ExecError, got %T: %v", err, err)
	}
	if execErr.ExitCode == 0 {
		t.Errorf("expected non-zero ExitCode, got 0")
	}
}

func TestExecError_Error(t *testing.T) {
	e := &git.ExecError{
		Args:     []string{"push", "origin"},
		ExitCode: 128,
		Stderr:   "permission denied",
	}
	got := e.Error()
	if got != "git push: exit 128: permission denied" {
		t.Errorf("Error() = %q", got)
	}
}

func TestExecRunner_RunAllowExitCode_ReturnsOutput(t *testing.T) {
	initTempRepo(t)
	ctx := context.Background()
	runner, err := git.NewExecRunner(ctx)
	if err != nil {
		t.Fatalf("NewExecRunner: %v", err)
	}
	// Create a file so diff --no-index has something to compare
	if err := os.WriteFile("testfile.txt", []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write testfile: %v", err)
	}
	// git diff --no-index exits 1 when files differ; RunAllowExitCode should return stdout
	out, err := runner.RunAllowExitCode(ctx, 1, "diff", "--no-index", "--", "/dev/null", "testfile.txt")
	if err != nil {
		t.Fatalf("RunAllowExitCode: %v", err)
	}
	if !strings.Contains(out, "+hello") {
		t.Errorf("expected diff output containing +hello, got %q", out)
	}
}

func TestExecRunner_RunAllowExitCode_PropagatesOtherErrors(t *testing.T) {
	initTempRepo(t)
	ctx := context.Background()
	runner, err := git.NewExecRunner(ctx)
	if err != nil {
		t.Fatalf("NewExecRunner: %v", err)
	}
	// A command that exits with a code != the allowed one should still error
	_, err = runner.RunAllowExitCode(ctx, 42, "not-a-real-command")
	if err == nil {
		t.Fatal("expected error for non-matching exit code, got nil")
	}
}

func TestExecRunner_RunWithEnv(t *testing.T) {
	initTempRepo(t)
	ctx := context.Background()
	runner, err := git.NewExecRunner(ctx)
	if err != nil {
		t.Fatalf("NewExecRunner: %v", err)
	}
	out, err := runner.RunWithEnv(ctx, []string{"GIT_AUTHOR_NAME=TestEnv"}, "--version")
	if err != nil {
		t.Fatalf("RunWithEnv: %v", err)
	}
	if !strings.Contains(out, "git version") {
		t.Errorf("expected git version output, got %q", out)
	}
}

func TestExecRunner_RunWithStdin(t *testing.T) {
	initTempRepo(t)
	ctx := context.Background()
	runner, err := git.NewExecRunner(ctx)
	if err != nil {
		t.Fatalf("NewExecRunner: %v", err)
	}
	// Use hash-object which reads from stdin
	out, err := runner.RunWithStdin(ctx, "test content\n", "hash-object", "--stdin")
	if err != nil {
		t.Fatalf("RunWithStdin: %v", err)
	}
	if len(out) != 40 {
		t.Errorf("expected 40-char SHA, got %q (len %d)", out, len(out))
	}
}

func TestExecRunner_Run_ContextCancel(t *testing.T) {
	initTempRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	runner, err := git.NewExecRunner(context.Background())
	if err != nil {
		t.Fatalf("NewExecRunner: %v", err)
	}
	_, err = runner.Run(ctx, "--version")
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		// The error may be wrapped in an ExecError or context error — check both
		var execErr *git.ExecError
		if !errors.As(err, &execErr) {
			t.Errorf("expected context.Canceled or *git.ExecError, got %T: %v", err, err)
		}
	}
}
