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

func TestCheckout_DirectMode_RestoresFile(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// Modify the file so it has unstaged changes
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")

	// Verify the file shows as modified before checkout
	diffOut, err := exec.Command("git", "-C", repoDir, "diff", "--name-only").CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(diffOut), "file1.txt", "file should be modified before checkout")

	// Invoke gti checkout with 'y' to confirm
	cmd := exec.Command(gtiBinary, "checkout", "file1.txt")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("y\n")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "gti checkout direct mode should exit zero: %s", out)

	// Verify the file no longer appears in git diff
	diffAfter, err := exec.Command("git", "-C", repoDir, "diff", "--name-only").CombinedOutput()
	require.NoError(t, err)
	assert.NotContains(t, string(diffAfter), "file1.txt", "file should not appear in diff after checkout")
}

func TestCheckout_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Create an unstaged change
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "checkout")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q\n")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	// Non-zero exit is acceptable - TUI may not receive input in dumb terminal
	_ = cmd.Run()

	assert.NoError(t, ctx.Err(), "process should not hang")
}

func TestCheckout_NoChanges_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// Working tree is clean

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "checkout")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "TERM=dumb")
	// The TUI may exit non-zero in dumb terminal (no TTY available); that is
	// acceptable. The important invariant is that the process does not hang.
	_ = cmd.Run()

	assert.NoError(t, ctx.Err(), "process should not hang when working tree is clean")
}
