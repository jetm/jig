//go:build integration

package integration_test

import (
	"testing"

	"github.com/jetm/gti/internal/testhelper"
	"github.com/stretchr/testify/assert"
)

func TestLog_ExitsCleanly(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "content\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	stderr, _ := runTUI(t, repoDir, "log")
	assert.Empty(t, stderr, "should start without errors")
}

func TestLog_AllFlag_UnknownFlag(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "content\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// --all is not implemented; gti log should reject it
	stderr, err := runTUI(t, repoDir, "log", "--all")
	assert.Error(t, err, "unknown flag should exit non-zero")
	assert.Contains(t, stderr, "unknown flag", "should report unknown flag")
}
