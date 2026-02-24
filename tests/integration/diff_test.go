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

	tm, err := newDiffTestModel(t, repoDir, "", false)
	require.NoError(t, err)

	// Wait for the TUI to render the modified file name
	tm.waitFor(t, containsOutput("changed.txt"))

	tm.quit(t)
}

func TestDiff_PagerMode_ShowsPipedDiff(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)

	// Simulate git piping a diff to jig diff (pager mode)
	rawDiff := "diff --git a/piped.go b/piped.go\n" +
		"index 1234567..abcdefg 100644\n" +
		"--- a/piped.go\n" +
		"+++ b/piped.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" package main\n" +
		"+// added via pager\n" +
		" func main() {}\n"

	tm, err := newDiffPagerTestModel(t, repoDir, rawDiff)
	require.NoError(t, err)

	tm.waitFor(t, containsOutput("piped.go"))
	tm.waitFor(t, containsOutput("diff (pager)"))

	tm.quit(t)
}

func TestDiff_PagerMode_EmptyFileWithMnemonicPrefix(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)

	// Empty file addition: no ---/+++ lines, only header with mnemonic prefix
	rawDiff := "diff --git c/empty.txt i/empty.txt\n" +
		"new file mode 100644\n" +
		"index 0000000..e69de29\n"

	tm, err := newDiffPagerTestModel(t, repoDir, rawDiff)
	require.NoError(t, err)

	tm.waitFor(t, containsOutput("empty.txt"))

	tm.quit(t)
}

func TestDiff_PagerMode_ColoredInput(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)

	// Simulate git's colored pager output with ANSI escape codes
	rawDiff := "\x1b[1mdiff --git c/colored.go i/colored.go\x1b[m\n" +
		"\x1b[1mindex 1234567..abcdefg 100644\x1b[m\n" +
		"\x1b[1m--- c/colored.go\x1b[m\n" +
		"\x1b[1m+++ i/colored.go\x1b[m\n" +
		"\x1b[36m@@ -1 +1 @@\x1b[m\n" +
		"\x1b[31m-old\x1b[m\n" +
		"\x1b[32m+new\x1b[m\n"

	tm, err := newDiffPagerTestModel(t, repoDir, rawDiff)
	require.NoError(t, err)

	tm.waitFor(t, containsOutput("colored.go"))

	tm.quit(t)
}

func TestDiff_TUI_StagedFlag_ShowsStagedFiles(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "staged.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add staged.txt")
	testhelper.WriteFile(t, repoDir, "staged.txt", "modified\n")
	testhelper.StageFile(t, repoDir, "staged.txt")

	tm, err := newDiffTestModel(t, repoDir, "", true)
	require.NoError(t, err)

	// Wait for the TUI to render the staged file name
	tm.waitFor(t, containsOutput("staged.txt"))

	tm.quit(t)
}
