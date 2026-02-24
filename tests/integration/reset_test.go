//go:build integration

package integration_test

import (
	"os/exec"
	"testing"

	"github.com/jetm/jig/internal/testhelper"
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

	// Invoke jig reset to unstage
	cmd := exec.Command(jigBinary, "reset", "file1.txt")
	cmd.Dir = repoDir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "jig reset direct mode should exit zero: %s", out)

	// Verify file is no longer staged
	cachedAfter, err := exec.Command("git", "-C", repoDir, "diff", "--name-only", "--cached").CombinedOutput()
	require.NoError(t, err)
	assert.NotContains(t, string(cachedAfter), "file1.txt", "file should not be staged after jig reset")
}

func TestReset_TUI_UnstageFile(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// Modify and stage the file
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")
	testhelper.StageFile(t, repoDir, "file1.txt")

	// Verify file is staged before TUI interaction
	cachedBefore := gitRun(t, repoDir, "diff", "--name-only", "--cached")
	require.Contains(t, cachedBefore, "file1.txt", "file should be staged before reset")

	tm, err := newResetTestModel(t, repoDir)
	require.NoError(t, err)

	// Wait for the TUI to render the staged file
	tm.waitFor(t, containsOutput("file1.txt"))

	// Space to select, Enter to unstage
	sendSpace(tm)
	sendEnter(tm)

	tm.waitDone(t)

	// Verify file is no longer staged
	cachedAfter := gitRun(t, repoDir, "diff", "--name-only", "--cached")
	assert.NotContains(t, cachedAfter, "file1.txt", "file should not be staged after reset TUI")
}

func TestReset_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// Stage a change
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")
	out, err := exec.Command("git", "-C", repoDir, "add", "file1.txt").CombinedOutput()
	require.NoError(t, err, "staging file: %s", out)

	stderr, _ := runTUI(t, repoDir, "reset")
	assert.Empty(t, stderr, "should start without errors")
}
