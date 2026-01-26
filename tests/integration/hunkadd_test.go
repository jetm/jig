//go:build integration

package integration_test

import (
	"testing"

	"github.com/jetm/gti/internal/testhelper"
	"github.com/stretchr/testify/assert"
)

func TestHunkAdd_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Create an unstaged change
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")

	stderr, _ := runTUI(t, repoDir, "hunk-add")
	assert.Empty(t, stderr, "should start without errors")
}

func TestHunkAdd_NoChanges_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// Working tree is clean

	stderr, _ := runTUI(t, repoDir, "hunk-add")
	assert.Empty(t, stderr, "should start without errors")
}

func TestHunkAdd_TUI_StageHunk(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "line1\nline2\nline3\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Modify file to create a hunk
	testhelper.WriteFile(t, repoDir, "file1.txt", "line1\nline2 modified\nline3\n")

	tm := newHunkAddTestModel(t, repoDir)

	// Wait for the TUI to render the file
	tm.waitFor(t, containsOutput("file1.txt"))

	// 'a' to stage all remaining hunks in this file
	sendKey(tm, 'a')

	// The program should finish after all hunks are decided
	tm.waitDone(t)

	// Verify the change was staged
	cached := gitRun(t, repoDir, "diff", "--name-only", "--cached")
	assert.Contains(t, cached, "file1.txt", "file should have staged hunks")
}
