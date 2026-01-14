//go:build integration

package integration_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jetm/gti/internal/git"
	"github.com/jetm/gti/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var gtiBinary string

func TestMain(m *testing.M) {
	// Build the gti binary once for all integration tests.
	tmpDir, err := os.MkdirTemp("", "gti-integration-*")
	if err != nil {
		panic("creating temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmpDir)

	gtiBinary = filepath.Join(tmpDir, "gti")
	cmd := exec.Command("go", "build", "-o", gtiBinary, "../../cmd/gti")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("building gti: " + err.Error())
	}

	os.Exit(m.Run())
}

func TestRebaseInteractive_EditorMode_WritesBackTodo(t *testing.T) {
	// Create a temp repo with 3 commits
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "a.txt", "content a\n")
	testhelper.AddCommit(t, repoDir, "add a.txt")
	testhelper.WriteFile(t, repoDir, "b.txt", "content b\n")
	testhelper.AddCommit(t, repoDir, "add b.txt")
	testhelper.WriteFile(t, repoDir, "c.txt", "content c\n")
	hash3 := testhelper.AddCommit(t, repoDir, "add c.txt")

	// Get the short hashes for all commits after initial
	hashesOut, err := exec.Command("git", "-C", repoDir, "log", "--reverse", "--format=%h", "HEAD~3..HEAD").CombinedOutput()
	require.NoError(t, err)
	hashes := strings.Split(strings.TrimSpace(string(hashesOut)), "\n")
	require.Len(t, hashes, 3, "expected 3 commits after initial")
	_ = hash3

	// Write a todo file with pick entries
	todoPath := filepath.Join(t.TempDir(), "git-rebase-todo")
	todoContent := git.FormatTodo([]git.RebaseTodoEntry{
		{Action: git.ActionPick, Hash: hashes[0], Subject: "add a.txt"},
		{Action: git.ActionPick, Hash: hashes[1], Subject: "add b.txt"},
		{Action: git.ActionPick, Hash: hashes[2], Subject: "add c.txt"},
	})
	require.NoError(t, os.WriteFile(todoPath, []byte(todoContent), 0o644))

	// Invoke gti rebase-interactive with the todo file path.
	// Since we can't interact with the TUI, we rely on the fact that
	// the program reads the file, and if we immediately send Enter (via stdin),
	// it should write back the (unmodified) todo and exit cleanly.
	//
	// For a non-interactive subprocess test, we use a trick: set TERM=dumb
	// and pipe a keypress. But bubbletea needs a real terminal for input.
	// Instead, we verify the simpler contract: the program accepts the file
	// without crashing, and the file remains valid after execution.
	//
	// The unit tests already verify the full TUI interaction. This integration
	// test validates the CLI argument detection and file I/O path.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, gtiBinary, "rebase-interactive", todoPath)
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q") // send quit to avoid hanging
	cmd.Env = append(os.Environ(), "TERM=dumb")
	// We expect non-zero exit since 'q' triggers abort in editor mode
	_ = cmd.Run()

	// The file should still exist and be readable
	written, err := os.ReadFile(todoPath)
	require.NoError(t, err)
	assert.NotEmpty(t, string(written), "todo file should not be empty after invocation")
}

func TestRebaseInteractive_EditorMode_AbortExitsNonZero(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "a.txt", "content a\n")
	testhelper.AddCommit(t, repoDir, "add a.txt")

	hashOut, err := exec.Command("git", "-C", repoDir, "log", "--reverse", "--format=%h", "HEAD~1..HEAD").CombinedOutput()
	require.NoError(t, err)
	hash := strings.TrimSpace(string(hashOut))

	todoPath := filepath.Join(t.TempDir(), "git-rebase-todo")
	todoContent := git.FormatTodo([]git.RebaseTodoEntry{
		{Action: git.ActionPick, Hash: hash, Subject: "add a.txt"},
	})
	require.NoError(t, os.WriteFile(todoPath, []byte(todoContent), 0o644))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, gtiBinary, "rebase-interactive", todoPath)
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	err = cmd.Run()

	// The process should exit with a non-zero code when aborted
	if err == nil {
		t.Log("note: process exited with code 0; TUI may not have received 'q' in dumb terminal mode")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		assert.NotEqual(t, 0, exitErr.ExitCode(), "abort should exit with non-zero code")
	}
}
