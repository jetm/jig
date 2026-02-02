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
	dv.SetContent(long)
	if dv.rawContent != long {
		t.Errorf("rawContent should equal input, got %q", dv.rawContent)
	}
	if dv.SoftWrap() {
		t.Error("softWrap should be false by default")
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
	for _, line := range lines {
		if len(line) > 5 {
			t.Errorf("line %q exceeds width 5", line)
		}
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
