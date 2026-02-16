//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/jetm/jig/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixup_NothingStaged_Error(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// No staged changes - just the initial commit from NewTempRepo

	cmd := exec.Command(jigBinary, "fixup")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "TERM=dumb")
	err := cmd.Run()
	assert.Error(t, err, "jig fixup with nothing staged should exit non-zero")
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

func TestFixup_TUI_FixupIntoRecentCommit(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	beforeCount := testhelper.CommitCount(t, repoDir)

	// Stage a change for fixup
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified for fixup\n")
	testhelper.StageFile(t, repoDir, "file1.txt")

	tm, err := newFixupTestModel(t, repoDir)
	require.NoError(t, err)

	// Wait for the TUI to render commit list
	tm.waitFor(t, containsOutput("add file1.txt"))

	// Enter to fixup into the first (most recent) commit
	sendEnter(tm)

	tm.waitDone(t)

	// Commit count should be unchanged (fixup amends, doesn't add)
	afterCount := testhelper.CommitCount(t, repoDir)
	assert.Equal(t, beforeCount, afterCount, "commit count should not change after fixup")

	// Verify the staged changes were absorbed
	cached := gitRun(t, repoDir, "diff", "--name-only", "--cached")
	assert.Empty(t, cached, "no files should remain staged after fixup")
}
