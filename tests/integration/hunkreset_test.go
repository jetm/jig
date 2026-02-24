//go:build integration

package integration_test

import (
	"testing"

	"github.com/jetm/jig/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHunkReset_ExitsCleanly_NoStagedChanges(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// No staged changes - constructor returns error, runTUI will report it
	stderr, _ := runTUI(t, repoDir, "hunk-reset")
	// With no staged changes, the model constructor returns an error.
	// That's expected - just verify it doesn't panic.
	_ = stderr
}

func TestHunkReset_ExitsCleanly_WithStagedChanges(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Stage a change
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")
	testhelper.StageFile(t, repoDir, "file1.txt")

	stderr, _ := runTUI(t, repoDir, "hunk-reset")
	assert.Empty(t, stderr, "should start without errors")
}

func TestHunkReset_TUI_UnstageHunk(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "line1\nline2\nline3\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Modify and stage the change
	testhelper.WriteFile(t, repoDir, "file1.txt", "line1\nline2 modified\nline3\n")
	testhelper.StageFile(t, repoDir, "file1.txt")

	// Verify file is staged before the test
	assertGitStaged(t, repoDir, "file1.txt")

	tm, err := newHunkResetTestModel(t, repoDir)
	require.NoError(t, err, "model creation should succeed with staged changes")

	// Wait for the TUI to render the file
	tm.waitFor(t, containsOutput("file1.txt"))

	// Space to toggle the hunk, then Enter to apply (unstage)
	sendSpace(tm)
	sendEnter(tm)

	// The program should finish after applying
	tm.waitDone(t)

	// Verify the change was unstaged
	assertGitNotStaged(t, repoDir, "file1.txt")
}
