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

func TestReset_DirectMode_UnstagesFile(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// Modify and stage the file
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")
	out, err := exec.Command("git", "-C", repoDir, "add", "file1.txt").CombinedOutput()
	require.NoError(t, err, "staging file: %s", out)

	// Verify file is staged
	cached, err := exec.Command("git", "-C", repoDir, "diff", "--name-only", "--cached").CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(cached), "file1.txt", "file should be staged before reset")

	// Invoke gti reset to unstage
	cmd := exec.Command(gtiBinary, "reset", "file1.txt")
	cmd.Dir = repoDir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "gti reset direct mode should exit zero: %s", out)

	// Verify file is no longer staged
	cachedAfter, err := exec.Command("git", "-C", repoDir, "diff", "--name-only", "--cached").CombinedOutput()
	require.NoError(t, err)
	assert.NotContains(t, string(cachedAfter), "file1.txt", "file should not be staged after gti reset")
}

func TestReset_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// Stage a change
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")
	out, err := exec.Command("git", "-C", repoDir, "add", "file1.txt").CombinedOutput()
	require.NoError(t, err, "staging file: %s", out)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "reset")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q\n")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	_ = cmd.Run()

	assert.NoError(t, ctx.Err(), "process should not hang")
}
