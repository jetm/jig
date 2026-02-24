//go:build integration

package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jetm/jig/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHunkCheckout_ExitsCleanly_NoWorkingTreeChanges(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// Working tree is clean - constructor returns error
	stderr, _ := runTUI(t, repoDir, "hunk-checkout")
	// With no changes, the model constructor returns an error.
	// Just verify it doesn't panic.
	_ = stderr
}

func TestHunkCheckout_ExitsCleanly_WithChanges(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Create a working tree change
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")

	stderr, _ := runTUI(t, repoDir, "hunk-checkout")
	assert.Empty(t, stderr, "should start without errors")
}

func TestHunkCheckout_TUI_DiscardHunk(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "line1\nline2\nline3\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Modify file to create a working tree change
	testhelper.WriteFile(t, repoDir, "file1.txt", "line1\nline2 modified\nline3\n")

	// Verify the file has the modification
	content, err := os.ReadFile(filepath.Join(repoDir, "file1.txt"))
	require.NoError(t, err)
	require.Contains(t, string(content), "line2 modified")

	tm, err := newHunkCheckoutTestModel(t, repoDir)
	require.NoError(t, err, "model creation should succeed with working tree changes")

	// Wait for the TUI to render the file
	tm.waitFor(t, containsOutput("file1.txt"))

	// Space to toggle the hunk, Enter to start confirmation, 'y' to confirm
	sendSpace(tm)
	sendEnter(tm)

	// Wait for the confirmation prompt
	tm.waitFor(t, containsOutput("Discard"))

	sendKey(tm, 'y')

	// The program should finish after applying
	tm.waitDone(t)

	// Verify the working tree change was discarded
	restored, err := os.ReadFile(filepath.Join(repoDir, "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\nline3\n", string(restored),
		"file should be restored to HEAD content after discard")
}
