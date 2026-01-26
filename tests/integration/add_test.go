//go:build integration

package integration_test

import (
	"os/exec"
	"testing"

	"github.com/jetm/gti/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdd_DirectMode_StagesFiles(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// Modify the file so it has unstaged changes
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	cmd := exec.Command(gtiBinary, "add", "file1.txt")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "gti add direct mode should exit zero: %s", out)

	// Verify file is now staged
	cached, err := exec.Command("git", "-C", repoDir, "diff", "--name-only", "--cached").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(cached), "file1.txt", "file should be staged after gti add")
}

func TestAdd_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Create an unstaged change
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	stderr, _ := runTUI(t, repoDir, "add")
	assert.Empty(t, stderr, "should start without errors")
}

func TestAdd_NoChanges_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// Working tree is clean - just the initial commit from NewTempRepo

	stderr, _ := runTUI(t, repoDir, "add")
	assert.Empty(t, stderr, "should start without errors")
}

func TestAdd_TUI_StageSingleFile(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	tm := newAddTestModel(t, repoDir)

	// Wait for the TUI to render the file name
	tm.waitFor(t, containsOutput("file1.txt"))

	// Space to select the file, Enter to stage
	sendSpace(tm)
	sendEnter(tm)

	// The program should quit after staging (PopModelMsg -> tea.Quit)
	tm.waitDone(t)

	// Verify file is staged
	cached := gitRun(t, repoDir, "diff", "--name-only", "--cached")
	assert.Contains(t, cached, "file1.txt", "file should be staged after TUI interaction")
}

func TestAdd_TUI_StageAllFiles(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "a.txt", "content a\n")
	testhelper.WriteFile(t, repoDir, "b.txt", "content b\n")
	testhelper.AddCommit(t, repoDir, "add files")
	testhelper.WriteFile(t, repoDir, "a.txt", "modified a\n")
	testhelper.WriteFile(t, repoDir, "b.txt", "modified b\n")

	tm := newAddTestModel(t, repoDir)

	// Wait for the TUI to render
	tm.waitFor(t, containsOutput("a.txt"))

	// 'a' to select all, Enter to stage
	sendKey(tm, 'a')
	sendEnter(tm)

	tm.waitDone(t)

	cached := gitRun(t, repoDir, "diff", "--name-only", "--cached")
	assert.Contains(t, cached, "a.txt", "a.txt should be staged")
	assert.Contains(t, cached, "b.txt", "b.txt should be staged")
}
