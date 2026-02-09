package git

import (
	"testing"
)

func TestSanitizeEditedDiff_RestoresStrippedContextPrefix(t *testing.T) {
	t.Parallel()
	// A context line that lost its leading space (editor stripped trailing whitespace).
	input := "@@ -1,3 +1,3 @@\n some code\nstripped context\n+added\n"
	got := sanitizeEditedDiff(input)
	want := "@@ -1,3 +1,3 @@\n some code\n stripped context\n+added\n"
	if got != want {
		t.Errorf("sanitizeEditedDiff:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestSanitizeEditedDiff_ValidPrefixesUnchanged(t *testing.T) {
	t.Parallel()
	input := "@@ -1,3 +1,3 @@\n context\n+added\n-removed\n\\ No newline at end of file\n"
	got := sanitizeEditedDiff(input)
	if got != input {
		t.Errorf("valid diff lines should not be modified:\ngot:  %q\nwant: %q", got, input)
	}
}

func TestSanitizeEditedDiff_StripsCommentLines(t *testing.T) {
	t.Parallel()
	input := "@@ -1,2 +1,2 @@\n# this is a comment\n context\n+added\n"
	got := sanitizeEditedDiff(input)
	want := "@@ -1,2 +1,2 @@\n context\n+added\n"
	if got != want {
		t.Errorf("comment lines should be stripped:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestSanitizeEditedDiff_PreservesTrailingEmptyLines(t *testing.T) {
	t.Parallel()
	input := "@@ -1,2 +1,2 @@\n context\n+added\n\n"
	got := sanitizeEditedDiff(input)
	if got != input {
		t.Errorf("trailing empty lines should be preserved:\ngot:  %q\nwant: %q", got, input)
	}
}

func TestSanitizeEditedDiff_EmptyContextLineNotModified(t *testing.T) {
	t.Parallel()
	// An empty line between hunk content lines (represents blank line in source
	// whose " " prefix was stripped). Should get space re-added since it's
	// between content lines, not a trailing empty line.
	input := "@@ -1,4 +1,4 @@\n first\n\n last\n"
	got := sanitizeEditedDiff(input)
	want := "@@ -1,4 +1,4 @@\n first\n \n last\n"
	if got != want {
		t.Errorf("empty line between content should get space prefix:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestHasValidDiffPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line string
		want bool
	}{
		{"+added", true},
		{"-removed", true},
		{" context", true},
		{"@@ -1 +1 @@", true},
		{`\ No newline at end of file`, true},
		{"no prefix", false},
		{"", false},
	}
	for _, tt := range tests {
		got := hasValidDiffPrefix(tt.line)
		if got != tt.want {
			t.Errorf("hasValidDiffPrefix(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}
