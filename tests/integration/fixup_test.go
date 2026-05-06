//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"strings"
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

func TestFixup_TUI_MultipleCommits_NavigateAndFixupSecond(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// Two separate files so the autosquash rebase has no conflicts.
	testhelper.WriteFile(t, repoDir, "first.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "first commit on first.txt")
	testhelper.WriteFile(t, repoDir, "second.txt", "content\n")
	testhelper.AddCommit(t, repoDir, "second commit on second.txt")

	beforeCount := testhelper.CommitCount(t, repoDir)

	// Stage a fixup destined for "first commit on first.txt".
	testhelper.WriteFile(t, repoDir, "first.txt", "amended content\n")
	testhelper.StageFile(t, repoDir, "first.txt")

	tm, err := newFixupTestModel(t, repoDir)
	require.NoError(t, err)

	// The most recent commit ("second commit on second.txt") appears at the top.
	tm.waitFor(t, containsOutput("second commit on second.txt"))

	// 'j' moves the cursor down one position to "first commit on first.txt".
	sendKey(tm, 'j')

	// Enter fixups the staged change into the now-selected commit.
	sendEnter(tm)

	tm.waitDone(t)

	afterCount := testhelper.CommitCount(t, repoDir)
	assert.Equal(t, beforeCount, afterCount, "commit count should not change after fixup")

	cached := gitRun(t, repoDir, "diff", "--name-only", "--cached")
	assert.Empty(t, cached, "no files should remain staged after fixup")

	// Confirm the fixup was absorbed into "first commit on first.txt", not the second.
	hash := strings.TrimSpace(gitRun(t, repoDir, "log", "--format=%H", "--fixed-strings",
		"--grep=first commit on first.txt", "-n1"))
	require.NotEmpty(t, hash, "first commit should still exist after autosquash")
	show := gitRun(t, repoDir, "show", hash)
	assert.Contains(t, show, "amended content", "fixup should be absorbed into the first commit")
}

func TestFixup_TUI_CancelLeavesChangesStaged(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "first commit on file.txt")

	beforeCount := testhelper.CommitCount(t, repoDir)

	testhelper.WriteFile(t, repoDir, "file.txt", "staged change\n")
	testhelper.StageFile(t, repoDir, "file.txt")

	tm, err := newFixupTestModel(t, repoDir)
	require.NoError(t, err)

	tm.waitFor(t, containsOutput("first commit on file.txt"))

	// 'q' cancels without applying the fixup.
	sendKey(tm, 'q')

	tm.waitDone(t)

	// The staged change must remain in the index.
	assertGitStaged(t, repoDir, "file.txt")

	afterCount := testhelper.CommitCount(t, repoDir)
	assert.Equal(t, beforeCount, afterCount, "commit count should not change on cancel")
}
