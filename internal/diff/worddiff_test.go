package diff_test

import (
	"strings"
	"testing"

	"github.com/jetm/jig/internal/diff"
)

func TestWordDiffRenderer_EmptyInput(t *testing.T) {
	r := diff.NewWordDiffRenderer()
	got, err := r.Render("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("empty input: got %q, want %q", got, "")
	}
}

func TestWordDiffRenderer_SingleWordChange(t *testing.T) {
	input := strings.Join([]string{
		"--- a/file.go",
		"+++ b/file.go",
		"@@ -1,3 +1,3 @@",
		" context line",
		`-	return "hello"`,
		`+	return "world"`,
		" more context",
	}, "\n")

	r := diff.NewWordDiffRenderer()
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The output should contain ANSI escape codes for styling.
	if !strings.Contains(got, "\x1b[") {
		t.Error("expected ANSI escape codes in output")
	}

	// Strip ANSI to verify the text content is preserved.
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "hello") {
		t.Error("output missing 'hello' from removed line")
	}
	if !strings.Contains(stripped, "world") {
		t.Error("output missing 'world' from added line")
	}

	// Context lines should pass through.
	if !strings.Contains(stripped, "context line") {
		t.Error("context line not preserved")
	}

	// Headers should pass through unchanged.
	if !strings.Contains(stripped, "--- a/file.go") {
		t.Error("diff header not preserved")
	}
}

func TestWordDiffRenderer_MultiLineBlockPairing(t *testing.T) {
	input := strings.Join([]string{
		"@@ -1,5 +1,5 @@",
		"-func foo() {",
		`-	x := 1`,
		`-	y := 2`,
		"+func bar() {",
		`+	x := 10`,
		`+	y := 20`,
	}, "\n")

	r := diff.NewWordDiffRenderer()
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stripped := stripANSI(got)
	lines := strings.Split(stripped, "\n")

	// Header + 3 removed + 3 added = 7 lines.
	if len(lines) != 7 {
		t.Fatalf("expected 7 lines, got %d: %v", len(lines), lines)
	}

	// Verify paired lines are present (first removed, then added for each pair).
	if !strings.Contains(lines[1], "foo") {
		t.Errorf("line 1 should contain 'foo', got %q", lines[1])
	}
	if !strings.Contains(lines[2], "bar") {
		t.Errorf("line 2 should contain 'bar', got %q", lines[2])
	}
}

func TestWordDiffRenderer_UnequalBlockSizes(t *testing.T) {
	input := strings.Join([]string{
		"@@ -1,2 +1,3 @@",
		"-line one",
		"-line two",
		"+line ONE",
		"+line TWO",
		"+line three",
	}, "\n")

	r := diff.NewWordDiffRenderer()
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stripped := stripANSI(got)

	// The first two pairs should be word-diffed, the third added line
	// should render as a plain added line.
	if !strings.Contains(stripped, "ONE") {
		t.Error("output missing 'ONE'")
	}
	if !strings.Contains(stripped, "TWO") {
		t.Error("output missing 'TWO'")
	}
	if !strings.Contains(stripped, "three") {
		t.Error("output missing unpaired 'three'")
	}
}

func TestWordDiffRenderer_DissimilarLinesSkipEmphasis(t *testing.T) {
	input := strings.Join([]string{
		"@@ -1,1 +1,1 @@",
		"-func calculateTotalRevenue(items []Item) float64 {",
		"+type ServerConfig struct {",
	}, "\n")

	r := diff.NewWordDiffRenderer()
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When lines are dissimilar, there should be no bold/reverse emphasis.
	// Bold is \x1b[1m, reverse is \x1b[7m.
	if strings.Contains(got, "\x1b[1m") && strings.Contains(got, "\x1b[7m") {
		// Check that bold+reverse aren't appearing together (emphasis style).
		// They could appear separately from the base styles, so check
		// for the combined bold+reverse which is the emphasis marker.
		// Actually, let's check for reverse specifically since the base styles
		// only use foreground color.
		if strings.Contains(got, "\x1b[7m") {
			t.Error("dissimilar lines should not have reverse (emphasis) styling")
		}
	}
}

func TestWordDiffRenderer_SemanticBoundaryCleanup(t *testing.T) {
	// Test that DiffCleanupSemanticLossless aligns diffs to word boundaries.
	input := strings.Join([]string{
		"@@ -1,1 +1,1 @@",
		"-the cat sat on the mat",
		"+the dog sat on the rug",
	}, "\n")

	r := diff.NewWordDiffRenderer()
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stripped := stripANSI(got)

	// "sat on the" should be unchanged in both lines (semantic cleanup
	// should keep word boundaries clean).
	if !strings.Contains(stripped, "sat on the") {
		t.Error("expected 'sat on the' to be preserved as unchanged text")
	}
}

func TestWordDiffRenderer_ContextAndHeadersPassThrough(t *testing.T) {
	input := strings.Join([]string{
		"diff --git a/file.go b/file.go",
		"index abc1234..def5678 100644",
		"--- a/file.go",
		"+++ b/file.go",
		"@@ -1,5 +1,5 @@",
		" package main",
		"",
		"-var x = 1",
		"+var x = 2",
		" ",
		" func main() {}",
	}, "\n")

	r := diff.NewWordDiffRenderer()
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stripped := stripANSI(got)

	// Headers should pass through.
	if !strings.Contains(stripped, "diff --git") {
		t.Error("git diff header not preserved")
	}
	if !strings.Contains(stripped, "--- a/file.go") {
		t.Error("--- header not preserved")
	}
	if !strings.Contains(stripped, "+++ b/file.go") {
		t.Error("+++ header not preserved")
	}
	if !strings.Contains(stripped, "@@ -1,5 +1,5 @@") {
		t.Error("hunk header not preserved")
	}
}

func TestWordDiffRenderer_AddOnlyBlock(t *testing.T) {
	// + lines without preceding - lines should render as plain added.
	input := strings.Join([]string{
		"@@ -1,2 +1,4 @@",
		" existing line",
		"+new line one",
		"+new line two",
		" more context",
	}, "\n")

	r := diff.NewWordDiffRenderer()
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stripped := stripANSI(got)
	if !strings.Contains(stripped, "new line one") {
		t.Error("added line not present")
	}
	if !strings.Contains(stripped, "new line two") {
		t.Error("second added line not present")
	}
}

func TestWordDiffRenderer_RemoveOnlyBlock(t *testing.T) {
	// - lines without following + lines should render as plain removed.
	input := strings.Join([]string{
		"@@ -1,4 +1,2 @@",
		" existing line",
		"-removed line one",
		"-removed line two",
		" more context",
	}, "\n")

	r := diff.NewWordDiffRenderer()
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stripped := stripANSI(got)
	if !strings.Contains(stripped, "removed line one") {
		t.Error("removed line not present")
	}
	if !strings.Contains(stripped, "removed line two") {
		t.Error("second removed line not present")
	}
}

func TestWordDiffRenderer_SampleDiffFixture(t *testing.T) {
	// Use the existing sample.diff fixture to verify no panics or errors
	// on real-world diff input.
	input := strings.Join([]string{
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,7 +1,7 @@",
		" package main",
		"",
		` import "fmt"`,
		"",
		"-func greet() string {",
		`-	return "hello"`,
		"+func greet(name string) string {",
		`+	return fmt.Sprintf("hello, %s", name)`,
		" }",
	}, "\n")

	r := diff.NewWordDiffRenderer()
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stripped := stripANSI(got)
	if !strings.Contains(stripped, "greet") {
		t.Error("function name not preserved")
	}
	if !strings.Contains(stripped, "name string") {
		t.Error("added parameter not in output")
	}
}
