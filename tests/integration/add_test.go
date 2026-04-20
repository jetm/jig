//go:build integration

package integration_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jetm/jig/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitForAfterResize polls the TUI output for substr while periodically
// re-sending a WindowSizeMsg. Needed when a just-pushed child model hasn't
// seen any WindowSizeMsg yet and View() returns "Terminal too small" - the
// PushModelMsg is emitted by a cmd running in a goroutine, so ordering
// relative to a plain tm.send is non-deterministic. Sending the resize on
// every poll tick guarantees the child eventually observes it.
func waitForAfterResize(tb testing.TB, tm *testModel, substr string) {
	tb.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tm.send(tea.WindowSizeMsg{Width: 120, Height: 40})
		if strings.Contains(tm.out.String(), substr) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	tb.Fatalf("waitForAfterResize: %q not found after 5s.\nLast output (%d bytes):\n%s",
		substr, tm.out.Len(), tm.out.String())
}

func TestAdd_DirectMode_StagesFiles(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")

	// Modify the file so it has unstaged changes
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	cmd := exec.Command(jigBinary, "add", "file1.txt")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "jig add direct mode should exit zero: %s", out)

	// Verify file is now staged
	cached, err := exec.Command("git", "-C", repoDir, "diff", "--name-only", "--cached").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(cached), "file1.txt", "file should be staged after jig add")
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

func TestAdd_TUI_HelpOverlay_OpenAndDismissWithQ(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	tm := newAddTestModel(t, repoDir)

	// Wait for initial render
	tm.waitFor(t, containsOutput("file1.txt"))

	// '?' opens help overlay; wait for overlay content
	sendKey(tm, '?')
	tm.waitFor(t, containsOutput("Navigation"))

	assertOutputContains(t, tm, "Navigation")

	// 'q' should dismiss the overlay (q is consumed by the overlay, not quit)
	outBefore := tm.out.String()
	sendKey(tm, 'q')
	time.Sleep(200 * time.Millisecond)

	// After dismissal the overlay text should not appear in new output
	// (the accumulated buffer will still have old frames, so we check
	// that new output was produced that no longer contains the overlay box border)
	outAfter := tm.out.String()
	assert.NotEqual(t, outBefore, outAfter, "output should change after dismissing overlay with q")

	tm.quit(t)
}

func TestAdd_TUI_HelpOverlay_DismissWithEsc(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	tm := newAddTestModel(t, repoDir)

	// Wait for initial render
	tm.waitFor(t, containsOutput("file1.txt"))

	// '?' opens help overlay
	sendKey(tm, '?')
	tm.waitFor(t, containsOutput("Navigation"))

	outBefore := tm.out.String()

	// Esc should dismiss the overlay
	tm.send(tea.KeyPressMsg{Code: tea.KeyEscape})
	time.Sleep(200 * time.Millisecond)

	outAfter := tm.out.String()
	assert.NotEqual(t, outBefore, outAfter, "output should change after dismissing overlay with Esc")

	tm.quit(t)
}

func TestAdd_TUI_SoftWrapToggle(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	testhelper.WriteFile(t, repoDir, "file1.txt", strings.Repeat("hello world ", 20)+"\n")

	tm := newAddTestModel(t, repoDir)

	// Wait for initial render then switch to diff panel
	tm.waitFor(t, containsOutput("file1.txt"))
	sendTab(tm)
	time.Sleep(100 * time.Millisecond)

	outBefore := tm.out.String()

	// 'w' toggles soft-wrap when diff panel has focus
	sendKey(tm, 'w')
	time.Sleep(200 * time.Millisecond)

	outAfter := tm.out.String()
	assert.NotEqual(t, outBefore, outAfter, "output should change after soft-wrap toggle")

	tm.quit(t)
}

func TestAdd_TUI_MaximizeToggle(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	tm := newAddTestModel(t, repoDir)

	// Wait for initial render
	tm.waitFor(t, containsOutput("file1.txt"))

	outBefore := tm.out.String()

	// 'F' toggles maximize of diff panel
	sendKey(tm, 'F')
	time.Sleep(200 * time.Millisecond)

	outAfter := tm.out.String()
	assert.NotEqual(t, outBefore, outAfter, "output should change after maximize toggle")

	tm.quit(t)
}

func TestAdd_TUI_PanelResizeShrink(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	tm := newAddTestModel(t, repoDir)

	// Wait for initial render
	tm.waitFor(t, containsOutput("file1.txt"))

	outBefore := tm.out.String()

	// '[' shrinks the left panel
	tm.send(keyPress('['))
	time.Sleep(200 * time.Millisecond)

	outAfter := tm.out.String()
	assert.NotEqual(t, outBefore, outAfter, "output should change after panel resize shrink")

	tm.quit(t)
}

func TestAdd_TUI_PanelResizeGrow(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

	tm := newAddTestModel(t, repoDir)

	// Wait for initial render
	tm.waitFor(t, containsOutput("file1.txt"))

	outBefore := tm.out.String()

	// ']' grows the left panel
	tm.send(keyPress(']'))
	time.Sleep(200 * time.Millisecond)

	outAfter := tm.out.String()
	assert.NotEqual(t, outBefore, outAfter, "output should change after panel resize grow")

	tm.quit(t)
}

func TestAdd_TUI_FileFilter_MatchingPath(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "alpha.txt", "content\n")
	testhelper.WriteFile(t, repoDir, "beta.txt", "content\n")
	testhelper.AddCommit(t, repoDir, "add files")
	testhelper.WriteFile(t, repoDir, "alpha.txt", "modified\n")
	testhelper.WriteFile(t, repoDir, "beta.txt", "modified\n")

	tm := newAddTestModelFiltered(t, repoDir, []string{"alpha.txt"})

	// Wait for alpha.txt to appear
	tm.waitFor(t, containsOutput("alpha.txt"))

	assertOutputContains(t, tm, "alpha.txt")

	// beta.txt should not be visible
	out := tm.out.String()
	assert.NotContains(t, out, "beta.txt", "beta.txt should not appear when filtered to alpha.txt")

	tm.quit(t)
}

func TestAdd_TUI_FileFilter_NoMatch(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "file1.txt", "content\n")
	testhelper.AddCommit(t, repoDir, "add file1.txt")
	testhelper.WriteFile(t, repoDir, "file1.txt", "modified\n")

	tm := newAddTestModelFiltered(t, repoDir, []string{"nonexistent.txt"})

	// The TUI should render a no-match message
	tm.waitFor(t, containsOutput("No matching"))

	assertOutputContains(t, tm, "No matching")

	tm.quit(t)
}

func TestAdd_TUI_FixupConfirmPath_AbsorbsStagedChanges(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	// Seed: initial commit (from NewTempRepo) + a second commit on a.txt.
	testhelper.WriteFile(t, repoDir, "a.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "second commit on a.txt")

	// Unstaged modification to a.txt.
	testhelper.WriteFile(t, repoDir, "a.txt", "modified for fixup\n")

	beforeCount := testhelper.CommitCount(t, repoDir)

	tm := newAddTestModel(t, repoDir)

	// Wait for the add file list to render a.txt
	tm.waitFor(t, containsOutput("a.txt"))

	// 'f' stages the cursor file and pushes the fixup picker. The app.Model
	// forwards the push's Init() but does not replay the prior WindowSizeMsg
	// to the newly pushed model, so its width/height start at zero and its
	// View() returns "Terminal too small" until a resize arrives. Re-send
	// the window size repeatedly while waiting so the msg is guaranteed to
	// arrive *after* the PushModelMsg (which is emitted by a cmd running in
	// a goroutine, ordering is non-deterministic relative to tm.send).
	sendKey(tm, 'f')
	waitForAfterResize(t, tm, "second commit on a.txt")

	// Enter selects the top commit (index 0 = most recent) for fixup.
	sendEnter(tm)

	// The fixup path runs CreateFixupCommit + AutosquashRebase, then pops
	// back to AddModel (program stays running). Poll git state until the
	// staged changes have been absorbed (index becomes empty) so we know
	// the rebase has completed before we quit the TUI.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		cached := gitRun(t, repoDir, "diff", "--cached", "--name-only")
		if cached == "" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	tm.quit(t)

	// Nothing should remain staged after the autosquash absorbed the change.
	cached := gitRun(t, repoDir, "diff", "--cached", "--name-only")
	assert.Empty(t, cached, "index should be empty after fixup+autosquash")

	// Commit count is unchanged (fixup amends into an existing commit).
	afterCount := testhelper.CommitCount(t, repoDir)
	assert.Equal(t, beforeCount, afterCount, "commit count should not change after fixup")

	// Resolve the target commit by subject (hash changed because of the rebase).
	// Find the commit whose subject matches our seed and inspect its diff.
	hash := gitRun(t, repoDir, "log", "--format=%H", "--grep=^second commit on a.txt$", "-E")
	require.NotEmpty(t, hash, "target commit should still exist after autosquash")
	// If multiple commits match (shouldn't), take the first.
	if idx := strings.Index(hash, "\n"); idx >= 0 {
		hash = hash[:idx]
	}

	show := gitRun(t, repoDir, "show", hash)
	assert.Contains(t, show, "modified for fixup",
		"the previously unstaged change should now be part of the target commit")
}

func TestAdd_TUI_FixupCancelPath_LeavesFilesStaged(t *testing.T) {
	repoDir := testhelper.NewTempRepo(t)
	testhelper.WriteFile(t, repoDir, "a.txt", "original\n")
	testhelper.AddCommit(t, repoDir, "second commit on a.txt")

	testhelper.WriteFile(t, repoDir, "a.txt", "modified for fixup\n")

	beforeCount := testhelper.CommitCount(t, repoDir)

	tm := newAddTestModel(t, repoDir)

	tm.waitFor(t, containsOutput("a.txt"))

	// 'f' stages the cursor file (a.txt) and pushes the fixup picker. See
	// matching comment in the confirm-path test for why the resize is
	// re-sent while waiting.
	sendKey(tm, 'f')
	waitForAfterResize(t, tm, "second commit on a.txt")

	// 'q' cancels the picker and pops back to AddModel with MutatedGit=false.
	// The file remains staged (no unstage-on-cancel - design decision D3).
	sendKey(tm, 'q')

	// Wait for the picker to pop. After the pop the mode indicator returns
	// to "add"; poll for the staged state so we don't quit before the pop
	// settles.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cached := gitRun(t, repoDir, "diff", "--cached", "--name-only")
		if strings.Contains(cached, "a.txt") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	tm.quit(t)

	cached := gitRun(t, repoDir, "diff", "--cached", "--name-only")
	assert.Contains(t, cached, "a.txt", "a.txt should remain staged after fixup cancel")

	afterCount := testhelper.CommitCount(t, repoDir)
	assert.Equal(t, beforeCount, afterCount, "commit count should not change on fixup cancel")
}
