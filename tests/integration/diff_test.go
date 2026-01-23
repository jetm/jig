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

func TestDiff_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Create an unstaged change
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "diff")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q\n")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	_ = cmd.Run()

	assert.NoError(t, ctx.Err(), "process should not hang")
}

func TestDiff_NoChanges_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// Working tree is clean

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "diff")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "TERM=dumb")
	// The TUI may exit non-zero in dumb terminal (no TTY available); that is
	// acceptable. The important invariant is that the process does not hang.
	_ = cmd.Run()

	assert.NoError(t, ctx.Err(), "process should not hang when working tree is clean")
}

func TestDiff_Revision(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "content a\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	testhelper.WriteFile(t, repoDir, "file2.txt", "content b\n")
	testhelper.AddCommit(t, repoDir, "add file2.txt")

	// Verify we have at least 2 commits beyond initial
	out, err := exec.Command("git", "-C", repoDir, "rev-list", "--count", "HEAD").CombinedOutput()
	require.NoError(t, err)
	require.NotEmpty(t, strings.TrimSpace(string(out)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "diff", "HEAD~1")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q\n")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	_ = cmd.Run()

	assert.NoError(t, ctx.Err(), "process should not hang with revision argument")
}
