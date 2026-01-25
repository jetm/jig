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
