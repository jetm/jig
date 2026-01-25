//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"testing"

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

	stderr, _ := runTUI(t, repoDir, "fixup")
	assert.Empty(t, stderr, "should start without errors")
}
