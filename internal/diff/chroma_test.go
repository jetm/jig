package diff_test

import (
	"os"
	"strings"
	"testing"

	"github.com/jetm/jig/internal/diff"
)

func TestChromaRenderer_ANSIEscapeCodes(t *testing.T) {
	fixture, err := os.ReadFile("testdata/sample.diff")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	r, err := diff.NewChromaRenderer()
	if err != nil {
		t.Fatalf("failed to create ChromaRenderer: %v", err)
	}

	got, err := r.Render(string(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, "\x1b[") {
		t.Error("ChromaRenderer output does not contain ANSI escape codes")
	}
}

func TestChromaRenderer_PreservesPrefixes(t *testing.T) {
	fixture, err := os.ReadFile("testdata/sample.diff")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	r, err := diff.NewChromaRenderer()
	if err != nil {
		t.Fatalf("failed to create ChromaRenderer: %v", err)
	}

	got, err := r.Render(string(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasPlus := false
	hasMinus := false
	for line := range strings.SplitSeq(got, "\n") {
		// Strip ANSI escape codes to find the actual content prefix.
		stripped := stripANSI(line)
		if strings.HasPrefix(stripped, "+") {
			hasPlus = true
		}
		if strings.HasPrefix(stripped, "-") {
			hasMinus = true
		}
	}

	if !hasPlus {
		t.Error("ChromaRenderer output missing + line prefixes")
	}
	if !hasMinus {
		t.Error("ChromaRenderer output missing - line prefixes")
	}
}

func TestChromaRenderer_EmptyInput(t *testing.T) {
	r, err := diff.NewChromaRenderer()
	if err != nil {
		t.Fatalf("failed to create ChromaRenderer: %v", err)
	}

	got, renderErr := r.Render("")
	if renderErr != nil {
		t.Fatalf("unexpected error: %v", renderErr)
	}
	if got != "" {
		t.Errorf("ChromaRenderer empty input: got %q, want %q", got, "")
	}
}

func TestChromaRenderer_GoldenOutput(t *testing.T) {
	t.Parallel()
	fixture, err := os.ReadFile("testdata/sample.diff")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	r, err := diff.NewChromaRenderer()
	if err != nil {
		t.Fatalf("failed to create ChromaRenderer: %v", err)
	}

	got, err := r.Render(string(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	golden, err := os.ReadFile("testdata/golden_chroma_sample.ansi")
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}

	if got != string(golden) {
		// Regenerate with: UPDATE_GOLDEN=1 go test ./internal/diff/ -run TestChromaRenderer_GoldenOutput
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			_ = os.WriteFile("testdata/golden_chroma_sample.ansi", []byte(got), 0644)
			t.Log("golden file updated")
			return
		}
		t.Errorf("ChromaRenderer output does not match golden file.\nRun UPDATE_GOLDEN=1 go test ./internal/diff/ to refresh.\ngot  (len=%d)\nwant (len=%d)", len(got), len(golden))
	}
}

func TestChromaRenderer_WordDiffHighlightsAdjacentPairs(t *testing.T) {
	t.Parallel()
	// A diff with one paired -/+ line: only "greet" → "greet(name string)"
	// The changed portion must have the word-diff background injected.
	input := `--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
-func greet() string {
+func greet(name string) string {
 }
`
	r, err := diff.NewChromaRenderer()
	if err != nil {
		t.Fatalf("failed to create ChromaRenderer: %v", err)
	}
	got, err := r.Render(input)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	// The removal background must appear in the - line and the addition
	// background in the + line.
	lines := strings.Split(got, "\n")
	foundRemBg, foundAddBg := false, false
	for _, l := range lines {
		stripped := stripANSI(l)
		if len(stripped) > 0 && stripped[0] == '-' && strings.Contains(l, "\x1b[48;2;80;30;30m") {
			foundRemBg = true
		}
		if len(stripped) > 0 && stripped[0] == '+' && strings.Contains(l, "\x1b[48;2;20;60;20m") {
			foundAddBg = true
		}
	}
	if !foundRemBg {
		t.Error("removal line should contain word-diff background colour")
	}
	if !foundAddBg {
		t.Error("addition line should contain word-diff background colour")
	}
}

func TestChromaRenderer_BinaryDiff(t *testing.T) {
	input := "Binary files a/image.png and b/image.png differ\n"

	r, err := diff.NewChromaRenderer()
	if err != nil {
		t.Fatalf("failed to create ChromaRenderer: %v", err)
	}

	_, err = r.Render(input)
	if err != nil {
		t.Fatalf("ChromaRenderer returned error on binary diff: %v", err)
	}
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until the terminating letter.
			j := i + 2
			for j < len(s) && (s[j] < 'A' || s[j] > 'Z') && (s[j] < 'a' || s[j] > 'z') {
				j++
			}
			if j < len(s) {
				j++ // skip the terminating letter
			}
			i = j
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}
