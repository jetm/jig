package diff_test

import (
	"os"
	"strings"
	"testing"

	"github.com/jetm/gti/internal/diff"
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
