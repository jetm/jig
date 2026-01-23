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

func TestFixup_NothingStaged_Error(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// No staged changes - just the initial commit from NewTempRepo

	cmd := exec.Command(gtiBinary, "fixup")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "TERM=dumb")
	err := cmd.Run()
	assert.Error(t, err, "gti fixup with nothing staged should exit non-zero")
}

func TestFixup_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// Stage a change
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")
	out, err := exec.Command("git", "-C", repoDir, "add", "file1.txt").CombinedOutput()
	require.NoError(t, err, "staging file: %s", out)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, gtiBinary, "fixup")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("q\n")
	cmd.Env = append(os.Environ(), "TERM=dumb")
	_ = cmd.Run()

	assert.NoError(t, ctx.Err(), "process should not hang")
}
