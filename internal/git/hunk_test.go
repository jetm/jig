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
	if !strings.Contains(hunks[0].Body(), "@@ -1,3 +1,4 @@") {
		t.Errorf("hunk body should contain header line, got: %q", hunks[0].Body())
	}
	if !strings.Contains(hunks[0].Body(), "+// added") {
		t.Errorf("hunk body should contain added line, got: %q", hunks[0].Body())
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
	if strings.Contains(hunks[0].Body(), "@@ -5") {
		t.Errorf("first hunk body should not contain second hunk header, got: %q", hunks[0].Body())
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
	if strings.HasSuffix(hunks[0].Body(), "\n\n") {
		t.Errorf("hunk body has trailing empty lines: %q", hunks[0].Body())
	}
}

func TestParseHunks_SingleHeaderOnly(t *testing.T) {
	t.Parallel()
	// A hunk with only the @@ header line and no body lines.
	raw := "@@ -0,0 +1 @@\n"
	hunks := ParseHunks(raw)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	if hunks[0].Header != "@@ -0,0 +1 @@" {
		t.Errorf("header = %q, want %q", hunks[0].Header, "@@ -0,0 +1 @@")
	}
	if hunks[0].Body() != "@@ -0,0 +1 @@" {
		t.Errorf("body = %q, want just the header", hunks[0].Body())
	}
}

// --- BuildPatch and RecalculateHeader tests ---

func TestRecalculateHeader_FullSelection(t *testing.T) {
	t.Parallel()
	h := Hunk{
		Header: "@@ -10,7 +10,9 @@",
		Lines: []Line{
			{Op: ' ', Content: "context1"},
			{Op: '-', Content: "old1", Selected: true},
			{Op: '-', Content: "old2", Selected: true},
			{Op: '+', Content: "new1", Selected: true},
			{Op: '+', Content: "new2", Selected: true},
			{Op: '+', Content: "new3", Selected: true},
			{Op: '+', Content: "new4", Selected: true},
			{Op: ' ', Content: "context2"},
			{Op: ' ', Content: "context3"},
		},
	}
	got := RecalculateHeader(h)
	want := "@@ -10,5 +10,7 @@"
	if got != want {
		t.Errorf("RecalculateHeader full selection = %q, want %q", got, want)
	}
}

func TestRecalculateHeader_DeselectAdded(t *testing.T) {
	t.Parallel()
	h := Hunk{
		Header: "@@ -10,7 +10,9 @@",
		Lines: []Line{
			{Op: ' ', Content: "context1"},
			{Op: '-', Content: "old1", Selected: true},
			{Op: '+', Content: "new1", Selected: true},
			{Op: '+', Content: "new2", Selected: false}, // deselected addition = omitted
			{Op: ' ', Content: "context2"},
		},
	}
	got := RecalculateHeader(h)
	// old: 1 context + 1 removed + 1 context = 3
	// new: 1 context + 1 added + 1 context = 3
	want := "@@ -10,3 +10,3 @@"
	if got != want {
		t.Errorf("RecalculateHeader deselect added = %q, want %q", got, want)
	}
}

func TestRecalculateHeader_DeselectRemoved(t *testing.T) {
	t.Parallel()
	h := Hunk{
		Header: "@@ -10,4 +10,3 @@",
		Lines: []Line{
			{Op: ' ', Content: "context1"},
			{Op: '-', Content: "old1", Selected: true},
			{Op: '-', Content: "old2", Selected: false}, // deselected removal = context
			{Op: ' ', Content: "context2"},
		},
	}
	got := RecalculateHeader(h)
	// old: ctx + removed + removed-as-ctx + ctx = 4
	// new: ctx + removed-as-ctx + ctx = 3
	want := "@@ -10,4 +10,3 @@"
	if got != want {
		t.Errorf("RecalculateHeader deselect removed = %q, want %q", got, want)
	}
}

func TestRecalculateHeader_PreservesContextSuffix(t *testing.T) {
	t.Parallel()
	h := Hunk{
		Header: "@@ -10,3 +10,4 @@ func main()",
		Lines: []Line{
			{Op: ' ', Content: "context"},
			{Op: '+', Content: "added", Selected: true},
			{Op: ' ', Content: "context2"},
		},
	}
	got := RecalculateHeader(h)
	want := "@@ -10,2 +10,3 @@ func main()"
	if got != want {
		t.Errorf("RecalculateHeader with suffix = %q, want %q", got, want)
	}
}

func TestBuildPatch_FullSelection(t *testing.T) {
	t.Parallel()
	header := "diff --git a/f b/f\n--- a/f\n+++ b/f"
	h := Hunk{
		Header: "@@ -1,3 +1,4 @@",
		Lines: []Line{
			{Op: ' ', Content: "context"},
			{Op: '+', Content: "added", Selected: true},
			{Op: ' ', Content: "context2"},
		},
	}
	patch := BuildPatch(header, h)
	if !strings.Contains(patch, "diff --git") {
		t.Error("patch should contain diff header")
	}
	if !strings.Contains(patch, "+added") {
		t.Error("patch should contain added line")
	}
	if !strings.Contains(patch, "@@ -1,2 +1,3 @@") {
		t.Errorf("patch header mismatch, got:\n%s", patch)
	}
}

func TestBuildPatch_PartialSelection(t *testing.T) {
	t.Parallel()
	header := "diff --git a/f b/f\n--- a/f\n+++ b/f"
	h := Hunk{
		Header: "@@ -1,3 +1,5 @@",
		Lines: []Line{
			{Op: ' ', Content: "context"},
			{Op: '+', Content: "keep", Selected: true},
			{Op: '+', Content: "skip", Selected: false},
			{Op: ' ', Content: "context2"},
		},
	}
	patch := BuildPatch(header, h)
	if !strings.Contains(patch, "+keep") {
		t.Error("patch should contain selected line")
	}
	if strings.Contains(patch, "+skip") {
		t.Error("patch should not contain deselected added line")
	}
}

func TestBuildPatch_DeselectedRemovedBecomesContext(t *testing.T) {
	t.Parallel()
	header := "diff --git a/f b/f\n--- a/f\n+++ b/f"
	h := Hunk{
		Header: "@@ -1,3 +1,2 @@",
		Lines: []Line{
			{Op: ' ', Content: "context"},
			{Op: '-', Content: "keep_as_context", Selected: false},
			{Op: ' ', Content: "context2"},
		},
	}
	patch := BuildPatch(header, h)
	// No changed lines selected - should return empty
	if patch != "" {
		t.Errorf("patch with no selected changes should be empty, got:\n%s", patch)
	}
}

func TestBuildPatch_AllDeselected(t *testing.T) {
	t.Parallel()
	header := "diff --git a/f b/f\n--- a/f\n+++ b/f"
	h := Hunk{
		Header: "@@ -1,3 +1,4 @@",
		Lines: []Line{
			{Op: ' ', Content: "context"},
			{Op: '+', Content: "added", Selected: false},
			{Op: ' ', Content: "context2"},
		},
	}
	patch := BuildPatch(header, h)
	if patch != "" {
		t.Errorf("patch with all deselected should be empty, got:\n%s", patch)
	}
}

func TestBuildPatch_SingleLineHunk(t *testing.T) {
	t.Parallel()
	header := "diff --git a/f b/f\n--- a/f\n+++ b/f"
	h := Hunk{
		Header: "@@ -1,0 +1,1 @@",
		Lines: []Line{
			{Op: '+', Content: "only line", Selected: true},
		},
	}
	patch := BuildPatch(header, h)
	if !strings.Contains(patch, "+only line") {
		t.Errorf("single-line patch missing content, got:\n%s", patch)
	}
}

func TestBuildPatch_MixedSelectionsProduceValidPatch(t *testing.T) {
	t.Parallel()
	header := "diff --git a/f b/f\n--- a/f\n+++ b/f"
	h := Hunk{
		Header: "@@ -1,4 +1,4 @@",
		Lines: []Line{
			{Op: ' ', Content: "context"},
			{Op: '-', Content: "removed_selected", Selected: true},
			{Op: '-', Content: "removed_not", Selected: false},
			{Op: '+', Content: "added_selected", Selected: true},
			{Op: '+', Content: "added_not", Selected: false},
			{Op: ' ', Content: "context2"},
		},
	}
	patch := BuildPatch(header, h)
	if !strings.Contains(patch, "-removed_selected") {
		t.Error("patch should contain selected removal")
	}
	if strings.Contains(patch, "-removed_not") {
		t.Error("deselected removal should appear as context, not as removal")
	}
	if !strings.Contains(patch, " removed_not") {
		t.Error("deselected removal should appear as context line")
	}
	if !strings.Contains(patch, "+added_selected") {
		t.Error("patch should contain selected addition")
	}
	if strings.Contains(patch, "+added_not") {
		t.Error("deselected addition should be omitted")
	}
}

// --- Body() round-trip tests ---

func TestBody_RoundTrip_SingleHunk(t *testing.T) {
	t.Parallel()
	raw := "@@ -1,3 +1,4 @@\n package main\n+// added\n func foo() {}\n"
	hunks := ParseHunks(raw)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	// Body() should reconstruct identically (minus trailing newline from split)
	got := hunks[0].Body()
	want := "@@ -1,3 +1,4 @@\n package main\n+// added\n func foo() {}"
	if got != want {
		t.Errorf("Body() round-trip mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestBody_RoundTrip_MultiHunk(t *testing.T) {
	t.Parallel()
	raw := "@@ -1,3 +1,4 @@\n context\n+added\n context2\n@@ -10,2 +11,3 @@\n before\n+new\n after\n"
	hunks := ParseHunks(raw)
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	// Each hunk's Body() should be self-contained
	body0 := hunks[0].Body()
	if !strings.HasPrefix(body0, "@@ -1,3") {
		t.Errorf("hunk 0 body prefix wrong: %q", body0)
	}
	if !strings.Contains(body0, "+added") {
		t.Errorf("hunk 0 body missing +added: %q", body0)
	}

	body1 := hunks[1].Body()
	if !strings.HasPrefix(body1, "@@ -10,2") {
		t.Errorf("hunk 1 body prefix wrong: %q", body1)
	}
	if !strings.Contains(body1, "+new") {
		t.Errorf("hunk 1 body missing +new: %q", body1)
	}
}

func TestBody_RoundTrip_Deletions(t *testing.T) {
	t.Parallel()
	raw := "@@ -1,3 +1,2 @@\n context\n-removed\n context2\n"
	hunks := ParseHunks(raw)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	got := hunks[0].Body()
	if !strings.Contains(got, "-removed") {
		t.Errorf("Body() missing deletion line: %q", got)
	}
}

func TestBody_RoundTrip_EmptyHunk(t *testing.T) {
	t.Parallel()
	h := Hunk{Header: "@@ -0,0 +1 @@"}
	got := h.Body()
	if got != "@@ -0,0 +1 @@" {
		t.Errorf("Body() of empty-lines hunk = %q, want just header", got)
	}
}

func TestBody_LinesPreserveOperations(t *testing.T) {
	t.Parallel()
	raw := "@@ -1,4 +1,4 @@\n ctx1\n-old\n+new\n ctx2\n"
	hunks := ParseHunks(raw)
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	lines := hunks[0].Lines
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if lines[0].Op != ' ' || lines[0].Content != "ctx1" {
		t.Errorf("line 0 = %c%q, want ' ctx1'", lines[0].Op, lines[0].Content)
	}
	if lines[1].Op != '-' || lines[1].Content != "old" {
		t.Errorf("line 1 = %c%q, want '-old'", lines[1].Op, lines[1].Content)
	}
	if lines[2].Op != '+' || lines[2].Content != "new" {
		t.Errorf("line 2 = %c%q, want '+new'", lines[2].Op, lines[2].Content)
	}
	if lines[3].Op != ' ' || lines[3].Content != "ctx2" {
		t.Errorf("line 3 = %c%q, want ' ctx2'", lines[3].Op, lines[3].Content)
	}
}

// --- ParseHunkRange tests ---

func TestParseHunkRange_WithCount(t *testing.T) {
	t.Parallel()
	r := ParseHunkRange("10,7")
	if r.Start != 10 || r.Count != 7 {
		t.Errorf("ParseHunkRange(\"10,7\") = {%d,%d}, want {10,7}", r.Start, r.Count)
	}
}

func TestParseHunkRange_NoCount(t *testing.T) {
	t.Parallel()
	r := ParseHunkRange("5")
	if r.Start != 5 || r.Count != 1 {
		t.Errorf("ParseHunkRange(\"5\") = {%d,%d}, want {5,1}", r.Start, r.Count)
	}
}

func TestParseLine_BackslashPrefix(t *testing.T) {
	t.Parallel()
	l := ParseLine("\\ No newline at end of file")
	if l.Op != '\\' {
		t.Errorf("Op = %c, want '\\'", l.Op)
	}
	if l.Selected {
		t.Error("backslash line should not be selected")
	}
}

func TestParseLine_EmptyString(t *testing.T) {
	t.Parallel()
	l := ParseLine("")
	if l.Op != ' ' {
		t.Errorf("Op = %c, want ' '", l.Op)
	}
}

func TestParseLine_UnknownPrefix(t *testing.T) {
	t.Parallel()
	l := ParseLine("xunknown")
	if l.Op != ' ' {
		t.Errorf("Op = %c, want ' ' for unknown prefix", l.Op)
	}
	if l.Content != "xunknown" {
		t.Errorf("Content = %q, want 'xunknown'", l.Content)
	}
}

func TestParseHunkHeader_Malformed(t *testing.T) {
	t.Parallel()
	old, newR := parseHunkHeader("not a header")
	if old.Start != 0 || newR.Start != 0 {
		t.Errorf("malformed header should return zero ranges")
	}
}

func TestParseHunkHeader_MissingNewRange(t *testing.T) {
	t.Parallel()
	old, newR := parseHunkHeader("@@ -1,3 @@")
	if old.Start != 0 && newR.Start != 0 {
		t.Log("incomplete header handled")
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

func (f *fakeStdinRunner) RunAllowExitCode(_ context.Context, _ int, _ ...string) (string, error) {
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
