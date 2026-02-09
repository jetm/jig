package components

import (
	"strings"
	"testing"
)

func newTestDiffView(content string) *DiffView {
	dv := NewDiffView(80, 5)
	dv.SetContent(content)
	return &dv
}

func TestDiffViewSetContentRendersInView(t *testing.T) {
	dv := newTestDiffView("diff --git a/foo b/foo")
	if !strings.Contains(dv.View(), "diff --git a/foo b/foo") {
		t.Error("View() should contain the set content")
	}
}

func TestDiffViewSetContentReplacesPrevious(t *testing.T) {
	dv := newTestDiffView("old")
	dv.SetContent("new")
	view := dv.View()
	if !strings.Contains(view, "new") {
		t.Error("View() should contain new content")
	}
	if strings.Contains(view, "old") {
		t.Error("View() should not contain old content")
	}
}

func TestDiffViewJIncreasesScrollOffset(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = strings.Repeat("x", 10)
	}
	dv := newTestDiffView(strings.Join(lines, "\n"))
	before := dv.ScrollOffset()
	sendKey(dv, 'j')
	after := dv.ScrollOffset()
	if after <= before {
		t.Errorf("scroll offset should increase: before=%d, after=%d", before, after)
	}
}

func TestDiffViewKDecreasesScrollOffset(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = strings.Repeat("x", 10)
	}
	dv := newTestDiffView(strings.Join(lines, "\n"))
	// scroll down first
	for range 5 {
		sendKey(dv, 'j')
	}
	before := dv.ScrollOffset()
	sendKey(dv, 'k')
	after := dv.ScrollOffset()
	if after >= before {
		t.Errorf("scroll offset should decrease: before=%d, after=%d", before, after)
	}
}

func TestDiffViewANSIContentRendersWithoutCorruption(t *testing.T) {
	ansiContent := "\033[31m+added line\033[0m\n\033[32m-removed line\033[0m"
	dv := newTestDiffView(ansiContent)
	view := dv.View()
	if !strings.Contains(view, "added line") {
		t.Error("ANSI content should render without corruption")
	}
}

func TestDiffViewXOffsetStartsAtZero(t *testing.T) {
	dv := newTestDiffView("short line")
	if dv.XOffset() != 0 {
		t.Errorf("XOffset should start at 0, got %d", dv.XOffset())
	}
}

func TestDiffViewSetWidthAndHeight(t *testing.T) {
	dv := newTestDiffView("content")
	dv.SetWidth(100)
	dv.SetHeight(20)
	if dv.View() == "" {
		t.Error("View() should not be empty after resize")
	}
}

func TestDiffViewSoftWrapOffStoresContentUnchanged(t *testing.T) {
	dv := NewDiffView(20, 5)
	long := strings.Repeat("x", 40) // longer than viewport width
	dv.SetSoftWrap(false)
	dv.SetContent(long)
	if dv.rawContent != long {
		t.Errorf("rawContent should equal input, got %q", dv.rawContent)
	}
	if dv.SoftWrap() {
		t.Error("softWrap should be false after explicit disable")
	}
}

func TestDiffViewSoftWrapOnWrapsLongLines(t *testing.T) {
	dv := NewDiffView(10, 20)
	dv.SetSoftWrap(true)
	// A 25-char line should be broken into multiple physical lines of <=10 chars.
	long := strings.Repeat("a", 25)
	dv.SetContent(long)
	view := dv.View()
	for line := range strings.SplitSeq(view, "\n") {
		// viewport may pad lines but logical content lines should not exceed width
		stripped := strings.TrimRight(line, " ")
		if len(stripped) > 10 {
			t.Errorf("line too long after soft-wrap: %q (len %d)", stripped, len(stripped))
		}
	}
}

func TestDiffViewSoftWrapToggleReappliesContent(t *testing.T) {
	dv := NewDiffView(10, 20)
	long := "+" + strings.Repeat("b", 30)
	dv.SetContent(long)

	// Enable soft-wrap — view should show multiple lines.
	dv.SetSoftWrap(true)
	wrappedView := dv.View()

	// Disable soft-wrap — view should be a single long line again.
	dv.SetSoftWrap(false)
	unwrappedView := dv.View()

	if wrappedView == unwrappedView {
		t.Error("view should differ between wrapped and unwrapped states")
	}
}

func TestDiffViewSoftWrapPreservesDiffPrefix(t *testing.T) {
	dv := NewDiffView(10, 20)
	dv.SetSoftWrap(true)
	// diff prefix + 20 chars should wrap but keep + on first line
	line := "+" + strings.Repeat("c", 20)
	dv.SetContent(line)
	view := dv.View()
	lines := strings.SplitSeq(strings.TrimRight(view, "\n "), "\n")
	// First non-empty line should start with +
	for l := range lines {
		if strings.TrimSpace(l) != "" {
			if !strings.HasPrefix(l, "+") {
				t.Errorf("first content line should start with '+', got %q", l)
			}
			break
		}
	}
}

func TestWrapContent_NarrowWidth(t *testing.T) {
	result := wrapContent("abcdefghij", 5)
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Errorf("expected wrapping into multiple lines, got %d: %v", len(lines), lines)
	}
}

func TestWrapContent_ZeroWidth_NoChange(t *testing.T) {
	input := "some content"
	result := wrapContent(input, 0)
	if result != input {
		t.Errorf("wrapContent with width=0 should return input unchanged, got %q", result)
	}
}

func TestWrapContent_ShortLine_NoWrap(t *testing.T) {
	input := "short"
	result := wrapContent(input, 80)
	if result != input {
		t.Errorf("short line should not be wrapped, got %q", result)
	}
}

func TestWrapContent_WordBoundaryBreak(t *testing.T) {
	// "hello world" at width 8 should break at the space, not mid-word.
	result := wrapContent("hello world", 8)
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapping, got %d lines: %v", len(lines), lines)
	}
	if lines[0] != "hello" {
		t.Errorf("first line should be %q, got %q", "hello", lines[0])
	}
	if strings.TrimRight(lines[1], " ") != " world" {
		t.Errorf("continuation line should be %q, got %q", " world", strings.TrimRight(lines[1], " "))
	}
}

func TestWrapContent_WordNotSplitMidWord(t *testing.T) {
	// "aaa bbb ccc" at width 7 should keep words intact.
	result := wrapContent("aaa bbb ccc", 7)
	lines := strings.SplitSeq(result, "\n")
	for line := range lines {
		trimmed := strings.TrimRight(line, " ")
		if strings.Contains(trimmed, "bb ") || strings.HasSuffix(trimmed, "bb") {
			// check the word "bbb" wasn't split
			if trimmed != " bbb" && trimmed != "aaa bbb" && trimmed != "aaa" {
				t.Errorf("word 'bbb' appears to be split in line %q", trimmed)
			}
		}
	}
}

func TestWrapContent_OverlongTokenForceBreaks(t *testing.T) {
	// A single 20-char token at width 8 should be force-broken.
	token := strings.Repeat("x", 20)
	result := wrapContent(token, 8)
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected force-break of overlong token, got %d lines", len(lines))
	}
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		if len(trimmed) > 8 {
			t.Errorf("line %q exceeds width 8 (len %d)", trimmed, len(trimmed))
		}
	}
}

func TestWrapContent_ANSIWrapsAtVisibleWidth(t *testing.T) {
	// ANSI codes should not count toward visible width.
	// "\033[31m" is 5 bytes but 0 visible chars.
	// "hello world" is 11 visible chars. With ANSI prefix, byte length is 16+4=20.
	line := "\033[31mhello world\033[0m"
	result := wrapContent(line, 8)
	// Should wrap at word boundary based on visible width (8), not byte length.
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapping of ANSI content, got %d lines: %v", len(lines), lines)
	}
}

func TestWrapContent_ANSISequenceNotSplit(t *testing.T) {
	// Construct a line where an ANSI sequence would fall exactly at the wrap point
	// if we were counting bytes. The wrapper must not split inside it.
	line := strings.Repeat("a", 7) + "\033[31m" + "bbb"
	result := wrapContent(line, 8)
	// Verify no partial ANSI sequences (no lone \033 or incomplete [...m).
	if strings.Contains(result, "\033[") {
		// Check each occurrence has a terminator.
		parts := strings.Split(result, "\033[")
		for i, p := range parts {
			if i == 0 {
				continue
			}
			terminated := false
			for _, c := range p {
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					terminated = true
					break
				}
			}
			if !terminated {
				t.Errorf("found unterminated ANSI sequence in wrapped output")
			}
		}
	}
}

func TestWrapContent_ContinuationIndent1Space(t *testing.T) {
	// A long line should produce continuation lines with exactly 1 space indent.
	result := wrapContent("aaa bbb ccc ddd eee", 7)
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapping, got %d lines", len(lines))
	}
	for i, line := range lines[1:] {
		trimmed := strings.TrimRight(line, " ")
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			t.Errorf("continuation line %d should start with space, got %q", i+1, line)
		}
		if strings.HasPrefix(line, "  ") {
			t.Errorf("continuation line %d has 2+ space indent, should be 1: %q", i+1, line)
		}
	}
}

func TestWrapContent_ContinuationEffectiveWidth(t *testing.T) {
	// With width=10, continuation lines have 1-space indent, so effective width is 9.
	// A continuation line of visible content should not exceed 9 chars + 1 space = 10.
	long := strings.Repeat("x ", 20) // "x x x x ..." - many short words
	result := wrapContent(long, 10)
	lines := strings.SplitSeq(result, "\n")
	for line := range lines {
		trimmed := strings.TrimRight(line, " ")
		if len(trimmed) > 10 {
			t.Errorf("line exceeds width 10: %q (len %d)", trimmed, len(trimmed))
		}
	}
}
