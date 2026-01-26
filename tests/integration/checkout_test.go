//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jetm/gti/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckout_DirectMode_RestoresFile(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// Modify the file so it has unstaged changes
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")

	// Verify the file shows as modified before checkout
	diffOut, err := exec.Command("git", "-C", repoDir, "diff", "--name-only").CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(diffOut), "file1.txt", "file should be modified before checkout")

	// Invoke gti checkout with 'y' to confirm
	cmd := exec.Command(gtiBinary, "checkout", "file1.txt")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader("y\n")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "gti checkout direct mode should exit zero: %s", out)

	// Verify the file no longer appears in git diff
	diffAfter, err := exec.Command("git", "-C", repoDir, "diff", "--name-only").CombinedOutput()
	require.NoError(t, err)
	assert.NotContains(t, string(diffAfter), "file1.txt", "file should not appear in diff after checkout")
}

func TestCheckout_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	// Create an unstaged change
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")

	stderr, _ := runTUI(t, repoDir, "checkout")
	assert.Empty(t, stderr, "should start without errors")
}

func TestCheckout_NoChanges_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// Working tree is clean

	stderr, _ := runTUI(t, repoDir, "checkout")
	assert.Empty(t, stderr, "should start without errors")
}

func TestCheckout_TUI_RestoreModifiedFile(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")

	tm := newCheckoutTestModel(t, repoDir)

	// Wait for the TUI to render the file
	tm.waitFor(t, containsOutput("file1.txt"))

	// Space to select, Enter to trigger confirmation, 'y' to confirm discard
	sendSpace(tm)
	sendEnter(tm)

	// Wait for confirmation prompt
	tm.waitFor(t, func(out string) bool {
		return strings.Contains(out, "Discard") || strings.Contains(out, "discard") || strings.Contains(out, "y/N")
	})

	sendKey(tm, 'y')
	tm.waitDone(t)

	// Verify file is restored to original content
	content, err := os.ReadFile(filepath.Join(repoDir, "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, "original\n", string(content), "file should be restored to committed version")
}
