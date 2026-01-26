//go:build integration

package integration_test

import (
	"testing"
	"time"

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

func TestLog_TUI_RendersCommitMessages(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "a.txt", "content a\n")
	testhelper.AddCommit(t, repoDir, "first real commit")
	testhelper.WriteFile(t, repoDir, "b.txt", "content b\n")
	testhelper.AddCommit(t, repoDir, "second real commit")

	tm := newLogTestModel(t, repoDir, "")

	// Wait for commits to render
	tm.waitFor(t, containsOutput("first real commit"))

	// Both commits should be visible
	output := tm.out.String()
	assert.Contains(t, output, "second real commit", "second commit should be visible")

	tm.quit(t)
}

func TestLog_TUI_TabSwitchesPanel(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "a.txt", "content a\n")
	testhelper.AddCommit(t, repoDir, "add a.txt")

	tm := newLogTestModel(t, repoDir, "")

	// Wait for initial render
	tm.waitFor(t, containsOutput("add a.txt"))

	outputBefore := tm.out.String()

	// Tab switches focus between left/right panels
	sendTab(tm)

	// Give time for re-render with different focus styling
	time.Sleep(200 * time.Millisecond)

	outputAfter := tm.out.String()

	// Output should change (different panel border styling)
	assert.NotEqual(t, outputBefore, outputAfter, "Tab should change output (panel focus)")

	tm.quit(t)
}
