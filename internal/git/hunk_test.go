package git

import (
	"context"
	"strings"
	"testing"
)

func TestParseHunks_Empty(t *testing.T) {
	t.Parallel()
	hunks := ParseHunks("")
	if hunks != nil {
		t.Errorf("ParseHunks(\"\") = %v, want nil", hunks)
	}
}

func TestParseHunks_NoHunkMarker(t *testing.T) {
	t.Parallel()
	// A diff header with no @@ lines should return no hunks.
	raw := "diff --git a/foo.go b/foo.go\nindex 111..222 100644\n--- a/foo.go\n+++ b/foo.go\n"
	hunks := ParseHunks(raw)
	if len(hunks) != 0 {
		t.Errorf("expected 0 hunks, got %d", len(hunks))
	}
}

func TestParseHunks_SingleHunk(t *testing.T) {
	t.Parallel()
	raw := "diff --git a/foo.go b/foo.go\n" +
		"index 111..222 100644\n" +
		"--- a/foo.go\n" +
		"+++ b/foo.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" package main\n" +
		"+// added\n" +
		" func foo() {}\n"

	hunks := ParseHunks(raw)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if !strings.HasPrefix(hunks[0].Header, "@@ ") {
		t.Errorf("hunk header = %q, want prefix '@@ '", hunks[0].Header)
	}
	if !strings.Contains(hunks[0].Body, "@@ -1,3 +1,4 @@") {
		t.Errorf("hunk body should contain header line, got: %q", hunks[0].Body)
	}
	if !strings.Contains(hunks[0].Body, "+// added") {
		t.Errorf("hunk body should contain added line, got: %q", hunks[0].Body)
	}
}

func TestParseHunks_TwoHunks(t *testing.T) {
	t.Parallel()
	raw := "diff --git a/bar.go b/bar.go\n" +
		"index aaa..bbb 100644\n" +
		"--- a/bar.go\n" +
		"+++ b/bar.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" package main\n" +
		"+// hunk 1\n" +
		" func a() {}\n" +
		"@@ -10,3 +11,4 @@\n" +
		" // section 2\n" +
		"+// hunk 2\n" +
		" func b() {}\n"

	hunks := ParseHunks(raw)
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	if !strings.Contains(hunks[0].Header, "@@ -1,3 +1,4 @@") {
		t.Errorf("first hunk header = %q", hunks[0].Header)
	}
	if !strings.Contains(hunks[1].Header, "@@ -10,3 +11,4 @@") {
		t.Errorf("second hunk header = %q", hunks[1].Header)
	}
}

func TestParseHunks_HunkBodyDoesNotContainNextHunkHeader(t *testing.T) {
	t.Parallel()
	raw := "@@ -1 +1 @@\n+first\n@@ -5 +5 @@\n+second\n"
	hunks := ParseHunks(raw)
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	// First hunk body should not contain the second hunk's @@ line
	if strings.Contains(hunks[0].Body, "@@ -5") {
		t.Errorf("first hunk body should not contain second hunk header, got: %q", hunks[0].Body)
	}
}

func TestParseHunks_TrailingEmptyLinesStripped(t *testing.T) {
	t.Parallel()
	raw := "@@ -1,2 +1,3 @@\n context\n+added\n context\n"
	hunks := ParseHunks(raw)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	// Body should not end with a blank line introduced by Split
	if strings.HasSuffix(hunks[0].Body, "\n\n") {
		t.Errorf("hunk body has trailing empty lines: %q", hunks[0].Body)
	}
}

func TestStageHunk_CallsApplyCached(t *testing.T) {
	t.Parallel()
	runner := &fakeStdinRunner{}
	err := StageHunk(context.Background(), runner, "diff --git a/f b/f\n--- a/f\n+++ b/f", "@@ -1 +1 @@\n+x\n")
	if err != nil {
		t.Fatalf("StageHunk returned error: %v", err)
	}
	if runner.lastArgs[0] != "apply" || runner.lastArgs[1] != "--cached" {
		t.Errorf("expected args [apply --cached], got %v", runner.lastArgs)
	}
	if !strings.Contains(runner.lastStdin, "diff --git") {
		t.Errorf("stdin should contain diff header, got: %q", runner.lastStdin)
	}
	if !strings.Contains(runner.lastStdin, "@@ -1 +1 @@") {
		t.Errorf("stdin should contain hunk body, got: %q", runner.lastStdin)
	}
}

func TestStageHunk_PropagatesError(t *testing.T) {
	t.Parallel()
	runner := &fakeStdinRunner{err: &ExecError{Args: []string{"apply"}, ExitCode: 1, Stderr: "patch failed"}}
	err := StageHunk(context.Background(), runner, "header", "body")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// fakeStdinRunner records the last RunWithStdin call.
type fakeStdinRunner struct {
	lastStdin string
	lastArgs  []string
	err       error
}

func (f *fakeStdinRunner) Run(_ context.Context, _ ...string) (string, error) {
	return "", nil
}

func (f *fakeStdinRunner) RunWithEnv(_ context.Context, _ []string, _ ...string) (string, error) {
	return "", nil
}

func (f *fakeStdinRunner) RunWithStdin(_ context.Context, stdin string, args ...string) (string, error) {
	f.lastStdin = stdin
	f.lastArgs = args
	return "", f.err
}
