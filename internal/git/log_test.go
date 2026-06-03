package git_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/jetm/jig/internal/git"
	"github.com/jetm/jig/internal/testhelper"
)

func TestParseCommitLog_Empty(t *testing.T) {
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	commits, err := git.RecentCommits(context.Background(), runner, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("expected 0 commits, got %d", len(commits))
	}
}

func TestParseCommitLog_SingleCommit(t *testing.T) {
	// Simulate git log output: hash\x1fsubject\x1fauthor\x1freldate\x1e
	raw := "abc1234\x1ffeat: add fixup command\x1fJane Doe\x1f2 hours ago\x1e"
	runner := &testhelper.FakeRunner{Outputs: []string{raw}}
	commits, err := git.RecentCommits(context.Background(), runner, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	c := commits[0]
	if c.Hash != "abc1234" {
		t.Errorf("Hash = %q, want %q", c.Hash, "abc1234")
	}
	if c.Subject != "feat: add fixup command" {
		t.Errorf("Subject = %q, want %q", c.Subject, "feat: add fixup command")
	}
	if c.Author != "Jane Doe" {
		t.Errorf("Author = %q, want %q", c.Author, "Jane Doe")
	}
	if c.Date != "2 hours ago" {
		t.Errorf("Date = %q, want %q", c.Date, "2 hours ago")
	}
}

func TestParseCommitLog_MultipleCommits(t *testing.T) {
	raw := "aaa0001\x1ffirst commit\x1fAlice\x1f1 day ago\x1e" +
		"bbb0002\x1fsecond commit\x1fBob\x1f3 hours ago\x1e"
	runner := &testhelper.FakeRunner{Outputs: []string{raw}}
	commits, err := git.RecentCommits(context.Background(), runner, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].Hash != "aaa0001" {
		t.Errorf("commits[0].Hash = %q, want %q", commits[0].Hash, "aaa0001")
	}
	if commits[1].Hash != "bbb0002" {
		t.Errorf("commits[1].Hash = %q, want %q", commits[1].Hash, "bbb0002")
	}
}

func TestParseCommitLog_SkipsMalformedRecords(t *testing.T) {
	// A record without the correct number of fields is silently skipped.
	raw := "aaa0001\x1ffirst commit\x1fAlice\x1f1 day ago\x1e" +
		"malformed\x1e" +
		"bbb0002\x1fsecond commit\x1fBob\x1f3 hours ago\x1e"
	runner := &testhelper.FakeRunner{Outputs: []string{raw}}
	commits, err := git.RecentCommits(context.Background(), runner, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits (malformed skipped), got %d", len(commits))
	}
}

func TestRecentCommits_UsesLogCommand(t *testing.T) {
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	_, _ = git.RecentCommits(context.Background(), runner, 10)
	testhelper.MustHaveCall(t, runner, "log")
}

func TestRecentCommits_RespectsLimit(t *testing.T) {
	runner := &testhelper.FakeRunner{Outputs: []string{""}}
	_, _ = git.RecentCommits(context.Background(), runner, 5)
	// Check that the call contains "-n5"
	if n := testhelper.CallCount(runner); n != 1 {
		t.Fatalf("expected 1 call, got %d", n)
	}
	call := testhelper.NthCall(runner, 0)
	found := false
	for _, a := range call.Args {
		if strings.Contains(a, "5") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -n5 in args, got %v", call.Args)
	}
}

func TestRecentCommits_ReturnsErrorOnFailure(t *testing.T) {
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{&git.ExecError{Args: []string{"log"}, ExitCode: 128, Stderr: "not a repo"}},
	}
	_, err := git.RecentCommits(context.Background(), runner, 20)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCommitDiff_UsesShowCommand(t *testing.T) {
	gitShowOutput := "commit abc1234def\nAuthor: Dev <dev@example.com>\nDate:   Mon Jan 1 00:00:00 2024 +0000\n\n    add x\n\ndiff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-old\n+new\n"
	runner := &testhelper.FakeRunner{Outputs: []string{gitShowOutput}}
	out, err := git.CommitDiff(context.Background(), runner, "abc1234", -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(out, "diff --git") {
		t.Errorf("CommitDiff should return stripped diff starting with 'diff --git', got: %q", out)
	}
	testhelper.MustHaveCall(t, runner, "show", "abc1234")
}

func TestCommitDiff_StripsCommitMetadataFromGitShow(t *testing.T) {
	gitShowOutput := "commit abc1234def5678\nAuthor: Dev <dev@example.com>\nDate:   Mon Jan 1 00:00:00 2024 +0000\n\n    feat: add feature\n\ndiff --git a/foo.go b/foo.go\nindex abc..def 100644\n--- a/foo.go\n+++ b/foo.go\n@@ -1 +1 @@\n-old\n+new\n"
	runner := &testhelper.FakeRunner{Outputs: []string{gitShowOutput}}
	out, err := git.CommitDiff(context.Background(), runner, "abc1234", -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "Author:") {
		t.Error("CommitDiff should strip commit metadata header, found 'Author:' in output")
	}
	if strings.Contains(out, "commit abc1234") {
		t.Error("CommitDiff should strip commit hash line from output")
	}
	if !strings.HasPrefix(out, "diff --git") {
		t.Errorf("CommitDiff output should begin with 'diff --git', got: %q", out[:min(60, len(out))])
	}
}

func TestCommitNumstat_ParsesPerFileStats(t *testing.T) {
	numstatOut := "10\t5\tfoo.go\n0\t200\tbar.go\n-\t-\timage.png\n"
	runner := &testhelper.FakeRunner{Outputs: []string{numstatOut}}
	stats, err := git.CommitNumstat(context.Background(), runner, "abc1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 3 {
		t.Fatalf("expected 3 file stats, got %d", len(stats))
	}
	if stats[0].Name != "foo.go" || stats[0].Additions != 10 || stats[0].Deletions != 5 {
		t.Errorf("stats[0] = %+v, want {foo.go 10 5 false}", stats[0])
	}
	if stats[1].Name != "bar.go" || stats[1].Additions != 0 || stats[1].Deletions != 200 {
		t.Errorf("stats[1] = %+v, want {bar.go 0 200 false}", stats[1])
	}
	if !stats[2].Binary || stats[2].Name != "image.png" {
		t.Errorf("stats[2] = %+v, want binary image.png", stats[2])
	}
	testhelper.MustHaveCall(t, runner, "show", "--numstat", "--format=", "abc1234")
}

func TestCommitDiffSafe_ExcludesLargeFiles(t *testing.T) {
	// Numstat call returns one large file (30000 lines) and one normal file.
	numstatOut := "25000\t5000\tlarge.go\n10\t5\tsmall.go\n"
	// The second call is the actual diff without large.go.
	diffOut := "commit abc\nAuthor: Dev <d@e.com>\nDate: Mon\n\n    msg\n\ndiff --git a/small.go b/small.go\n--- a/small.go\n+++ b/small.go\n@@ -1 +1 @@\n-old\n+new\n"
	runner := &testhelper.FakeRunner{Outputs: []string{numstatOut, diffOut}}
	out, err := git.CommitDiffSafe(context.Background(), runner, "abc1234", -1, 20000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "large.go") {
		t.Error("output should mention excluded large.go")
	}
	if !strings.Contains(out, "diff --git") {
		t.Error("output should contain the remaining diff")
	}
	// Second call must exclude large.go via :(exclude).
	calls := runner.Calls
	if len(calls) < 2 {
		t.Fatalf("expected 2 runner calls, got %d", len(calls))
	}
	hasExclude := false
	for _, arg := range calls[1].Args {
		if strings.Contains(arg, ":(exclude)large.go") {
			hasExclude = true
		}
	}
	if !hasExclude {
		t.Errorf("second git call should exclude large.go, args: %v", calls[1].Args)
	}
}

func TestCommitDiffSafe_NoLargeFiles_BehavesLikeCommitDiff(t *testing.T) {
	numstatOut := "5\t3\tsmall.go\n"
	diffOut := "commit abc\nAuthor: x\nDate: y\n\n    msg\n\ndiff --git a/small.go b/small.go\n--- a/small.go\n+++ b/small.go\n@@ -1 +1 @@\n-old\n+new\n"
	runner := &testhelper.FakeRunner{Outputs: []string{numstatOut, diffOut}}
	out, err := git.CommitDiffSafe(context.Background(), runner, "abc1234", -1, 20000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(out, "diff --git") {
		t.Errorf("output should start with 'diff --git', got: %q", out[:min(40, len(out))])
	}
}

func TestCommitDiff_InputStartingWithDiff_Unchanged(t *testing.T) {
	// git show can produce output that already begins with "diff --git" when the
	// commit metadata is absent (e.g. --no-commit-id mode or output piped in).
	directDiff := "diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-old\n+new\n"
	runner := &testhelper.FakeRunner{Outputs: []string{directDiff}}
	out, err := git.CommitDiff(context.Background(), runner, "abc1234", -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(out, "diff --git") {
		t.Errorf("diff-only input should be preserved, got %q", out[:min(40, len(out))])
	}
}

func TestCommitDiff_EmptyCommit_ReturnsRawOutput(t *testing.T) {
	// An empty commit has no diff section; the raw git show output is returned.
	emptyOut := "commit abc1234\nAuthor: Dev <d@e.com>\nDate: Mon\n\n    empty commit\n"
	runner := &testhelper.FakeRunner{Outputs: []string{emptyOut}}
	out, err := git.CommitDiff(context.Background(), runner, "abc1234", -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != emptyOut {
		t.Errorf("empty commit should return raw output unchanged, got %q", out)
	}
}

func TestCommitNumstat_ReturnsErrorOnFailure(t *testing.T) {
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{&git.ExecError{Args: []string{"show"}, ExitCode: 128, Stderr: "bad object"}},
	}
	_, err := git.CommitNumstat(context.Background(), runner, "badhash")
	if err == nil {
		t.Error("expected error from CommitNumstat, got nil")
	}
}

func TestCommitDiffSafe_FallsBackWhenNumstatFails(t *testing.T) {
	// When numstat itself fails, CommitDiffSafe falls back to CommitDiff.
	numstatErr := &git.ExecError{Args: []string{"show"}, ExitCode: 128, Stderr: "bad object"}
	diffOut := "commit abc\nAuthor: x\nDate: y\n\n    msg\n\ndiff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-old\n+new\n"
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", diffOut},
		Errors:  []error{numstatErr, nil},
	}
	out, err := git.CommitDiffSafe(context.Background(), runner, "abc1234", -1, 20000)
	if err != nil {
		t.Fatalf("unexpected error on numstat fallback: %v", err)
	}
	if !strings.HasPrefix(out, "diff --git") {
		t.Errorf("fallback output should start with 'diff --git', got: %q", out[:min(40, len(out))])
	}
}

func TestCommitDiff_ReturnsErrorOnFailure(t *testing.T) {
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{&git.ExecError{Args: []string{"show"}, ExitCode: 128, Stderr: "bad object"}},
	}
	_, err := git.CommitDiff(context.Background(), runner, "badhash", -1)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCreateFixupCommit_UsesFixupFlag(t *testing.T) {
	runner := &testhelper.FakeRunner{Outputs: []string{"[main abc1234] fixup! feat: something"}}
	err := git.CreateFixupCommit(context.Background(), runner, "abc1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testhelper.MustHaveCall(t, runner, "commit", "--fixup=abc1234")
}

func TestRecentCommitsFrom_NoRef(t *testing.T) {
	raw := "abc1234\x1ffeat: add log command\x1fJane Doe\x1f1 hour ago\x1e"
	runner := &testhelper.FakeRunner{Outputs: []string{raw}}
	commits, err := git.RecentCommitsFrom(context.Background(), runner, 20, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
}

func TestRecentCommitsFrom_WithRef(t *testing.T) {
	raw := "def5678\x1ffeat: old commit\x1fBob\x1f5 days ago\x1e"
	runner := &testhelper.FakeRunner{Outputs: []string{raw}}
	commits, err := git.RecentCommitsFrom(context.Background(), runner, 20, "v1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	// Verify the ref was passed as an arg
	call := testhelper.NthCall(runner, 0)
	found := slices.Contains(call.Args, "v1.0")
	if !found {
		t.Errorf("expected ref %q in args, got %v", "v1.0", call.Args)
	}
}

func TestRecentCommitsFrom_ReturnsErrorOnFailure(t *testing.T) {
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{&git.ExecError{Args: []string{"log"}, ExitCode: 128, Stderr: "bad revision"}},
	}
	_, err := git.RecentCommitsFrom(context.Background(), runner, 20, "badref")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestParseCommitLog_WhitespaceOnly(t *testing.T) {
	runner := &testhelper.FakeRunner{Outputs: []string{"   \n\t\n  "}}
	commits, err := git.RecentCommits(context.Background(), runner, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("expected 0 commits for whitespace input, got %d", len(commits))
	}
}

func TestParseCommitLog_TrailingRecordSeparators(t *testing.T) {
	// Extra record separators should be treated as empty records and skipped.
	raw := "abc1234\x1ffeat: test\x1fAlice\x1f1 day ago\x1e\x1e\x1e"
	runner := &testhelper.FakeRunner{Outputs: []string{raw}}
	commits, err := git.RecentCommits(context.Background(), runner, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
}

func TestCreateFixupCommit_ReturnsErrorOnFailure(t *testing.T) {
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{&git.ExecError{Args: []string{"commit"}, ExitCode: 1, Stderr: "nothing to commit"}},
	}
	err := git.CreateFixupCommit(context.Background(), runner, "abc1234")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestAutosquashRebase_SuccessPath(t *testing.T) {
	runner := &testhelper.FakeRunner{
		Outputs: []string{"Successfully rebased"},
	}
	err := git.AutosquashRebase(context.Background(), runner, "abc1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the call used RunWithEnv with --autosquash, --interactive, and hash^
	testhelper.MustHaveCall(t, runner, "rebase", "--interactive", "--autosquash", "abc1234^")
	testhelper.MustHaveEnv(t, runner, "GIT_SEQUENCE_EDITOR=true")
}

func TestAutosquashRebase_ErrorPath(t *testing.T) {
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{&git.ExecError{Args: []string{"rebase"}, ExitCode: 1, Stderr: "conflict"}},
	}
	err := git.AutosquashRebase(context.Background(), runner, "abc1234")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "rebase") {
		t.Errorf("error should mention rebase, got: %v", err)
	}
}

func TestAutosquashRebase_RootCommitFallback(t *testing.T) {
	// First call (hash^) fails, second call (--root) succeeds.
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "Successfully rebased"},
		Errors:  []error{&git.ExecError{Args: []string{"rebase"}, ExitCode: 128, Stderr: "unknown revision abc1234^"}, nil},
	}
	err := git.AutosquashRebase(context.Background(), runner, "abc1234")
	if err != nil {
		t.Fatalf("unexpected error on root-commit fallback: %v", err)
	}
	// Second call should use --root
	testhelper.MustHaveCall(t, runner, "rebase", "--interactive", "--autosquash", "--root")
}
