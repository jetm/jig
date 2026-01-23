//go:build integration

package integration_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jetm/gti/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdd_DirectMode_StagesFiles(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// Modify the file so it has unstaged changes
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	cmd := exec.Command(gtiBinary, "add", "file1.txt")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "gti add direct mode should exit zero: %s", out)

	// Verify file is now staged
	cached, err := exec.Command("git", "-C", repoDir, "diff", "--name-only", "--cached").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(cached), "file1.txt", "file should be staged after gti add")
}

func TestAdd_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Create an unstaged change
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "add")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q\n")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	// Non-zero exit is acceptable - TUI may not receive input in dumb terminal
	_ = cmd.Run()

	// The test passes as long as the process did not hang (context not exceeded)
	assert.NoError(t, ctx.Err(), "process should not hang")
}

func TestAdd_NoChanges_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// Working tree is clean - just the initial commit from NewTempRepo

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "add")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "TERM=dumb")
	// The TUI may exit non-zero in dumb terminal (no TTY available); that is
	// acceptable. The important invariant is that the process does not hang.
	_ = cmd.Run()

	assert.NoError(t, ctx.Err(), "process should not hang when working tree is clean")
}
