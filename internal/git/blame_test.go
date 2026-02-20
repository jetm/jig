package git

import (
	"context"
	"testing"

	"github.com/jetm/jig/internal/testhelper"
)

func TestParseHunkHeader_Standard(t *testing.T) {
	t.Parallel()
	oldRange, _ := parseHunkHeader("@@ -10,7 +10,9 @@ func main()")
	if oldRange.Start != 10 || oldRange.Count != 7 {
		t.Errorf("expected start=10, count=7, got start=%d, count=%d", oldRange.Start, oldRange.Count)
	}
}

func TestParseHunkHeader_NoCount(t *testing.T) {
	t.Parallel()
	oldRange, _ := parseHunkHeader("@@ -5 +5,2 @@")
	if oldRange.Start != 5 || oldRange.Count != 1 {
		t.Errorf("expected start=5, count=1, got start=%d, count=%d", oldRange.Start, oldRange.Count)
	}
}

func TestParseHunkHeader_InvalidHeader(t *testing.T) {
	t.Parallel()
	oldRange, _ := parseHunkHeader("not a header")
	if oldRange.Start != 0 || oldRange.Count != 0 {
		t.Errorf("expected start=0, count=0, got start=%d, count=%d", oldRange.Start, oldRange.Count)
	}
}

func TestParseHunkHeader_SingleLine(t *testing.T) {
	t.Parallel()
	oldRange, _ := parseHunkHeader("@@ -1,1 +1,1 @@")
	if oldRange.Start != 1 || oldRange.Count != 1 {
		t.Errorf("expected start=1, count=1, got start=%d, count=%d", oldRange.Start, oldRange.Count)
	}
}

func TestParseBlameOutput_SingleCommit(t *testing.T) {
	t.Parallel()
	output := "abc123def456789012345678901234567890abcd 10 10 1\n" +
		"author Test User\n" +
		"author-mail <test@example.com>\n" +
		"\tline content\n" +
		"abc123def456789012345678901234567890abcd 11 11\n" +
		"author Test User\n" +
		"\tanother line\n"

	counts := ParseBlameOutput(output)
	if len(counts) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(counts))
	}
	hash := "abc123def456789012345678901234567890abcd"
	if counts[hash] != 2 {
		t.Errorf("expected 2 lines for commit, got %d", counts[hash])
	}
}

func TestParseBlameOutput_MultipleCommits(t *testing.T) {
	t.Parallel()
	output := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 1 1 1\n" +
		"\tline 1\n" +
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb 2 2 1\n" +
		"\tline 2\n" +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 3 3\n" +
		"\tline 3\n"

	counts := ParseBlameOutput(output)
	if len(counts) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(counts))
	}
	if counts["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] != 2 {
		t.Errorf("expected 2 lines for commit a, got %d", counts["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"])
	}
	if counts["bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"] != 1 {
		t.Errorf("expected 1 line for commit b, got %d", counts["bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"])
	}
}

func TestParseBlameOutput_SkipsUncommitted(t *testing.T) {
	t.Parallel()
	output := "0000000000000000000000000000000000000000 1 1 1\n" +
		"\tuncommitted line\n"

	counts := ParseBlameOutput(output)
	if len(counts) != 0 {
		t.Errorf("expected 0 commits (all-zeros skipped), got %d", len(counts))
	}
}

func TestParseBlameOutput_Empty(t *testing.T) {
	t.Parallel()
	counts := ParseBlameOutput("")
	if len(counts) != 0 {
		t.Errorf("expected 0 commits for empty output, got %d", len(counts))
	}
}

func TestFindFixupTarget_SingleFileClearTarget(t *testing.T) {
	t.Parallel()
	stagedDiff := "diff --git a/foo.go b/foo.go\n" +
		"index 111..222 100644\n" +
		"--- a/foo.go\n" +
		"+++ b/foo.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" package main\n" +
		"+// new line\n" +
		" func foo() {}\n"

	blameOutput := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 1 1 1\n" +
		"\tpackage main\n" +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 2 2\n" +
		"\t// comment\n" +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 3 3\n" +
		"\tfunc foo() {}\n"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			blameOutput, // git blame --porcelain
			"aaaaaaa",   // git rev-parse --short
		},
	}

	result := FindFixupTarget(context.Background(), runner, stagedDiff)
	if result.Hash != "aaaaaaa" {
		t.Errorf("expected hash=aaaaaaa, got %q", result.Hash)
	}
	if result.Confidence != 100 {
		t.Errorf("expected confidence=100, got %d", result.Confidence)
	}
}

func TestFindFixupTarget_MultipleFilesDominantTarget(t *testing.T) {
	t.Parallel()
	stagedDiff := "diff --git a/a.go b/a.go\n" +
		"--- a/a.go\n" +
		"+++ b/a.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" package main\n" +
		"+// new\n" +
		" func a() {}\n" +
		"diff --git a/b.go b/b.go\n" +
		"--- a/b.go\n" +
		"+++ b/b.go\n" +
		"@@ -1,2 +1,3 @@\n" +
		" package main\n" +
		"+// new\n"

	commitA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	// File a: 3 lines all blamed to commitA
	blameA := commitA + " 1 1 1\n\tline\n" +
		commitA + " 2 2\n\tline\n" +
		commitA + " 3 3\n\tline\n"

	// File b: 2 lines - 1 to commitA, 1 to commitB
	blameB := commitA + " 1 1 1\n\tline\n" +
		commitB + " 2 2 1\n\tline\n"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			blameA,    // blame for a.go
			blameB,    // blame for b.go
			"aaaaaaa", // rev-parse --short
		},
	}

	result := FindFixupTarget(context.Background(), runner, stagedDiff)
	if result.Hash != "aaaaaaa" {
		t.Errorf("expected hash=aaaaaaa (dominant), got %q", result.Hash)
	}
	// commitA has 4 out of 5 lines = 80%
	if result.Confidence != 80 {
		t.Errorf("expected confidence=80, got %d", result.Confidence)
	}
}

func TestFindFixupTarget_AllNewFiles(t *testing.T) {
	t.Parallel()
	stagedDiff := "diff --git a/new.go b/new.go\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/new.go\n" +
		"@@ -0,0 +1,3 @@\n" +
		"+package main\n" +
		"+func new() {}\n"

	runner := &testhelper.FakeRunner{}

	result := FindFixupTarget(context.Background(), runner, stagedDiff)
	if result.Hash != "" {
		t.Errorf("expected empty hash for all-new files, got %q", result.Hash)
	}
}

func TestFindFixupTarget_EmptyDiff(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{}
	result := FindFixupTarget(context.Background(), runner, "")
	if result.Hash != "" {
		t.Errorf("expected empty hash for empty diff, got %q", result.Hash)
	}
}

func TestFindFixupTarget_BlameFailure(t *testing.T) {
	t.Parallel()
	stagedDiff := "diff --git a/foo.go b/foo.go\n" +
		"--- a/foo.go\n" +
		"+++ b/foo.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" package main\n" +
		"+// new\n" +
		" func foo() {}\n"

	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{errForTest},
	}

	result := FindFixupTarget(context.Background(), runner, stagedDiff)
	if result.Hash != "" {
		t.Errorf("expected empty hash on blame failure, got %q", result.Hash)
	}
}

func TestFindFixupTarget_AmbiguousTarget(t *testing.T) {
	t.Parallel()
	stagedDiff := "diff --git a/foo.go b/foo.go\n" +
		"--- a/foo.go\n" +
		"+++ b/foo.go\n" +
		"@@ -1,4 +1,5 @@\n" +
		" line1\n" +
		" line2\n" +
		"+// new\n" +
		" line3\n" +
		" line4\n"

	commitA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	// 2 lines to A, 2 lines to B = 50% each
	blameOutput := commitA + " 1 1 1\n\tline\n" +
		commitA + " 2 2\n\tline\n" +
		commitB + " 3 3 1\n\tline\n" +
		commitB + " 4 4\n\tline\n"

	runner := &testhelper.FakeRunner{
		Outputs: []string{
			blameOutput, // git blame
			"aaaaaaa",   // rev-parse --short (or bbbbbbb, depends on map iteration)
		},
	}

	result := FindFixupTarget(context.Background(), runner, stagedDiff)
	// With 50/50 split, still returns a result with 50% confidence
	if result.Hash == "" {
		t.Error("expected a hash even with ambiguous target")
	}
	if result.Confidence != 50 {
		t.Errorf("expected confidence=50, got %d", result.Confidence)
	}
}

// errForTest is a sentinel error for test purposes.
var errForTest = errorString("test error")

type errorString string

func (e errorString) Error() string { return string(e) }
