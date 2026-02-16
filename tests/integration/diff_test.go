//go:build integration

package integration_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/jetm/jig/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiff_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Create an unstaged change
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")

	stderr, _ := runTUI(t, repoDir, "diff")
	assert.Empty(t, stderr, "should start without errors")
}

func TestDiff_NoChanges_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// Working tree is clean

	stderr, _ := runTUI(t, repoDir, "diff")
	assert.Empty(t, stderr, "should start without errors")
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

	stderr, _ := runTUI(t, repoDir, "diff", "HEAD~1")
	assert.Empty(t, stderr, "should start without errors")
}

func TestDiff_TUI_ShowsModifiedFiles(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "changed.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add changed.txt")
	testhelper.WriteFile(t, repoDir, "changed.txt", "modified\n")

	tm := newDiffTestModel(t, repoDir, "", false)

	// Wait for the TUI to render the modified file name
	tm.waitFor(t, containsOutput("changed.txt"))

	tm.quit(t)
}

func TestDiff_TUI_StagedFlag_ShowsStagedFiles(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "staged.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add staged.txt")
	testhelper.WriteFile(t, repoDir, "staged.txt", "modified\n")
	testhelper.StageFile(t, repoDir, "staged.txt")

	tm := newDiffTestModel(t, repoDir, "", true)

	// Wait for the TUI to render the staged file name
	tm.waitFor(t, containsOutput("staged.txt"))

	tm.quit(t)
}
