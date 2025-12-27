package diff_test

import (
	"os"
	"testing"

	"github.com/jetm/gti/internal/diff"
)

func TestPlainRenderer_SampleDiff(t *testing.T) {
	fixture, err := os.ReadFile("testdata/sample.diff")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	r := &diff.PlainRenderer{}
	got, err := r.Render(string(fixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != string(fixture) {
		t.Errorf("PlainRenderer output differs from input\ngot:\n%s\nwant:\n%s", got, string(fixture))
	}
}

func TestPlainRenderer_EmptyInput(t *testing.T) {
	r := &diff.PlainRenderer{}
	got, err := r.Render("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("PlainRenderer empty input: got %q, want %q", got, "")
	}
}

func TestPlainRenderer_NeverReturnsError(t *testing.T) {
	r := &diff.PlainRenderer{}

	inputs := []string{
		"",
		"some random text",
		"--- a/file\n+++ b/file\n@@ -1 +1 @@\n-old\n+new\n",
	}

	for _, input := range inputs {
		_, err := r.Render(input)
		if err != nil {
			t.Errorf("PlainRenderer returned error for input %q: %v", input, err)
		}
	}
}
