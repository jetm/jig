package diff_test

import (
	"strings"
	"testing"

	"github.com/jetm/jig/internal/diff"
)

func TestWordDiff_IdenticalStrings_NoSpans(t *testing.T) {
	t.Parallel()
	old, new := diff.WordDiff("hello world", "hello world")
	if len(old) != 0 || len(new) != 0 {
		t.Errorf("identical strings: expected no spans, got old=%v new=%v", old, new)
	}
}

func TestWordDiff_CompletelyDifferent_WholeSpan(t *testing.T) {
	t.Parallel()
	oldSpans, newSpans := diff.WordDiff("abc", "xyz")
	if len(oldSpans) == 0 {
		t.Error("completely different old: expected at least one span")
	}
	if len(newSpans) == 0 {
		t.Error("completely different new: expected at least one span")
	}
	// Old span must cover "abc" (3 chars starting at 0).
	if oldSpans[0].Start != 0 || oldSpans[0].End != 3 {
		t.Errorf("old span = %v, want [0, 3)", oldSpans[0])
	}
}

func TestWordDiff_WordReplaced_BothSidesHaveSpans(t *testing.T) {
	t.Parallel()
	// "return old" → "return new" — "old" is replaced by "new".
	// Both sides must have a changed span; "return " is shared and must NOT appear in spans.
	old := "return old"
	new := "return new"
	oldSpans, newSpans := diff.WordDiff(old, new)

	if len(oldSpans) == 0 {
		t.Error("expected changed spans in old when a word is replaced")
	}
	if len(newSpans) == 0 {
		t.Error("expected changed spans in new when a word is replaced")
	}
	// The common prefix "return " (7 chars) must not appear in either span set.
	for _, sp := range oldSpans {
		if sp.Start < 7 {
			t.Errorf("old span starts at %d — unchanged prefix included: %v", sp.Start, oldSpans)
		}
	}
	for _, sp := range newSpans {
		if sp.Start < 7 {
			t.Errorf("new span starts at %d — unchanged prefix included: %v", sp.Start, newSpans)
		}
	}
}

func TestWordDiff_EmptyStrings_NoSpans(t *testing.T) {
	t.Parallel()
	old, new := diff.WordDiff("", "")
	if len(old) != 0 || len(new) != 0 {
		t.Errorf("empty strings: expected no spans, got old=%v new=%v", old, new)
	}
}

func TestWordDiff_AddedContent_OnlyNewSpan(t *testing.T) {
	t.Parallel()
	old := "return x"
	new := "return x + y"
	oldSpans, newSpans := diff.WordDiff(old, new)
	if len(oldSpans) != 0 {
		t.Errorf("old had no changes, expected no spans, got %v", oldSpans)
	}
	if len(newSpans) == 0 {
		t.Error("new has added content, expected spans")
	}
}

// --- ApplyHighlightSpans ---

func TestApplyHighlightSpans_EmptySpans_Unchanged(t *testing.T) {
	t.Parallel()
	input := "\x1b[31m-removed\x1b[0m"
	got := diff.ApplyHighlightSpans(input, nil, "\x1b[48;2;80;30;30m")
	if got != input {
		t.Errorf("empty spans: expected unchanged output, got %q", got)
	}
}

func TestApplyHighlightSpans_WholeString_InjectsBgAndReset(t *testing.T) {
	t.Parallel()
	// Plain (no ANSI) input, highlight the whole thing.
	input := "hello"
	spans := []diff.Span{{0, 5}}
	bg := "\x1b[48;2;80;30;30m"
	got := diff.ApplyHighlightSpans(input, spans, bg)
	if !strings.Contains(got, bg) {
		t.Error("output should contain the background escape sequence")
	}
	if !strings.Contains(got, "\x1b[49m") {
		t.Error("output should contain the background reset \\x1b[49m")
	}
	stripped := diff.StripANSI(got)
	if stripped != "hello" {
		t.Errorf("stripped output should equal original text, got %q", stripped)
	}
}

func TestApplyHighlightSpans_ANSIInput_VisibleTextPreserved(t *testing.T) {
	t.Parallel()
	// ANSI-coloured input: "\x1b[31mfoo\x1b[0m bar"
	// Highlight the word "bar" at visible positions [4, 7).
	input := "\x1b[31mfoo\x1b[0m bar"
	spans := []diff.Span{{4, 7}}
	bg := "\x1b[48;2;0;60;0m"
	got := diff.ApplyHighlightSpans(input, spans, bg)
	stripped := diff.StripANSI(got)
	if stripped != "foo bar" {
		t.Errorf("visible text should be preserved, got %q", stripped)
	}
	if !strings.Contains(got, bg) {
		t.Error("output should contain the injected background")
	}
}

func TestApplyHighlightSpans_SpanAtZero_InjectsAtStart(t *testing.T) {
	t.Parallel()
	input := "abc"
	spans := []diff.Span{{0, 1}}
	bg := "\x1b[48;2;80;30;30m"
	got := diff.ApplyHighlightSpans(input, spans, bg)
	if !strings.HasPrefix(got, bg) {
		t.Errorf("span at position 0 should inject bg at start, got %q", got)
	}
}

// --- StripANSI ---

func TestStripANSI_PlainText_Unchanged(t *testing.T) {
	t.Parallel()
	in := "hello world"
	got := diff.StripANSI(in)
	if got != in {
		t.Errorf("plain text should be unchanged, got %q", got)
	}
}

func TestStripANSI_ANSICodes_Removed(t *testing.T) {
	t.Parallel()
	in := "\x1b[31mred text\x1b[0m and \x1b[32mgreen\x1b[0m"
	got := diff.StripANSI(in)
	want := "red text and green"
	if got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", in, got, want)
	}
}

func TestStripANSI_TruecolorBg_Removed(t *testing.T) {
	t.Parallel()
	in := "\x1b[48;2;80;30;30mhello\x1b[49m"
	got := diff.StripANSI(in)
	if got != "hello" {
		t.Errorf("truecolor bg not stripped, got %q", got)
	}
}
