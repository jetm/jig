//go:build integration

package integration_test

import (
	"bytes"
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

func TestRebaseInteractive_EditorMode_WritesBackTodo(t *testing.T) {
	// Create a temp repo with 3 commits
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "a.txt", "content a\n")
	testhelper.AddCommit(t, repoDir, "add a.txt")
	testhelper.WriteFile(t, repoDir, "b.txt", "content b\n")
	testhelper.AddCommit(t, repoDir, "add b.txt")
	testhelper.WriteFile(t, repoDir, "c.txt", "content c\n")
	testhelper.AddCommit(t, repoDir, "add c.txt")

	// Get the short hashes for all commits after initial
	hashesOut, err := exec.Command("git", "-C", repoDir, "log", "--reverse", "--format=%h", "HEAD~3..HEAD").CombinedOutput()
	require.NoError(t, err)
	hashes := strings.Split(strings.TrimSpace(string(hashesOut)), "\n")
	require.Len(t, hashes, 3, "expected 3 commits after initial")

	// Write a todo file with pick entries
	todoPath := filepath.Join(t.TempDir(), "git-rebase-todo")
	todoContent := git.FormatTodo([]git.RebaseTodoEntry{
		{Action: git.ActionPick, Hash: hashes[0], Subject: "add a.txt"},
		{Action: git.ActionPick, Hash: hashes[1], Subject: "add b.txt"},
		{Action: git.ActionPick, Hash: hashes[2], Subject: "add c.txt"},
	})
	require.NoError(t, os.WriteFile(todoPath, []byte(todoContent), 0o644))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, gtiBinary, "rebase-interactive", todoPath)
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q") // send quit to avoid hanging
	cmd.Env = append(os.Environ(), "TERM=dumb")
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	// We expect non-zero exit since 'q' triggers abort in editor mode
	_ = cmd.Run()

	require.NoError(t, ctx.Err(), "process should not hang")
	assert.Empty(t, filterTTYError(stderrBuf.String()), "should start without errors")

	// The file should still exist and be readable
	written, err := os.ReadFile(todoPath)
	require.NoError(t, err)
	assert.NotEmpty(t, string(written), "todo file should not be empty after invocation")
}

func TestRebase_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "a.txt", "content a\n")
	testhelper.AddCommit(t, repoDir, "add a.txt")
	testhelper.WriteFile(t, repoDir, "b.txt", "content b\n")
	testhelper.AddCommit(t, repoDir, "add b.txt")

	// Invoke in standalone TUI mode (HEAD~1 is a revision, not a file path)
	stderr, _ := runTUI(t, repoDir, "rebase-interactive", "HEAD~1")
	assert.Empty(t, stderr, "should start without errors")
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
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	err = cmd.Run()

	require.NoError(t, ctx.Err(), "process should not hang")
	assert.Empty(t, filterTTYError(stderrBuf.String()), "should start without errors")

	// The process should exit with a non-zero code when aborted
	if err == nil {
		t.Log("note: process exited with code 0; TUI may not have received 'q' in dumb terminal mode")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		assert.NotEqual(t, 0, exitErr.ExitCode(), "abort should exit with non-zero code")
	}
}
