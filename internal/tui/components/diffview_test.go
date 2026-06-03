package components

import (
	"strings"
	"testing"
)

func newTestDiffView(content string) *DiffView {
	dv := NewDiffView(80, 5, true)
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
	dv := NewDiffView(20, 5, true)
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
	dv := NewDiffView(40, 20, true)
	dv.SetSoftWrap(true)
	// A 50-char line should be broken into multiple physical lines.
	long := strings.Repeat("a", 50)
	dv.SetContent(long)
	view := dv.View()
	contentLines := 0
	for line := range strings.SplitSeq(view, "\n") {
		stripped := strings.TrimRight(line, " ")
		if strings.ContainsAny(stripped, "a") {
			contentLines++
		}
	}
	if contentLines < 2 {
		t.Errorf("expected long line to wrap into multiple lines, got %d content lines", contentLines)
	}
}

func TestDiffViewSoftWrapToggleReappliesContent(t *testing.T) {
	dv := NewDiffView(10, 20, true)
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
	dv := NewDiffView(40, 20, true)
	dv.SetSoftWrap(true)
	// diff prefix + 40 chars should wrap but keep + on first content line
	line := "+" + strings.Repeat("c", 40)
	dv.SetContent(line)
	view := dv.View()
	lines := strings.SplitSeq(strings.TrimRight(view, "\n "), "\n")
	// First non-empty line should contain the + prefix after the gutter
	for l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			if !strings.Contains(l, "+") {
				t.Errorf("first content line should contain '+', got %q", l)
			}
			break
		}
	}
}

func TestDiffViewSoftWrapAccountsForGutterWidth(t *testing.T) {
	t.Parallel()
	// Width=40, gutter="   ~ │ "=7 chars, effective content width=33.
	// A 40-char line must wrap because 40 > 33.
	dv := NewDiffView(40, 20, true)
	dv.SetSoftWrap(true)
	line := strings.Repeat("x", 40)
	dv.SetContent(line)
	view := dv.View()
	contentLines := 0
	for l := range strings.SplitSeq(view, "\n") {
		if strings.ContainsAny(l, "x") {
			contentLines++
		}
	}
	if contentLines < 2 {
		t.Errorf("40-char line should wrap in 40-wide view with 7-char gutter (content=33), got %d content line(s)", contentLines)
	}
}

func TestNewDiffView_FillHeightEnabled(t *testing.T) {
	t.Parallel()
	dv := NewDiffView(80, 20, true)
	dv.SetContent("line1\nline2")

	view := dv.View()
	lines := strings.Split(view, "\n")
	// With FillHeight, the viewport should fill to its full height (20 lines)
	if len(lines) != 20 {
		t.Errorf("expected 20 lines with FillHeight, got %d", len(lines))
	}
}

func TestNewDiffView_GutterShowsSourceLineNumbers(t *testing.T) {
	t.Parallel()
	raw := `diff --git a/foo.go b/foo.go
index abc..def 100644
--- a/foo.go
+++ b/foo.go
@@ -10,3 +10,4 @@ func main() {
 unchanged
-removed
+added1
+added2
 context`
	dv := NewDiffView(80, 20, true)
	dv.SetDiffContent(raw, raw)

	view := dv.View()
	lines := strings.Split(view, "\n")

	// Line 0-3: headers -> blank gutter
	if strings.Contains(lines[0], "1 ") {
		t.Errorf("header line should not have line number, got %q", lines[0])
	}
	// Line 4: hunk header -> blank gutter
	if !strings.Contains(lines[4], "│") {
		t.Errorf("hunk header should have gutter separator, got %q", lines[4])
	}
	// Line 5: " unchanged" -> new-file line 10
	if !strings.Contains(lines[5], "10 │") {
		t.Errorf("context line should show line 10, got %q", lines[5])
	}
	// Line 6: "-removed" -> old-file line 11
	if !strings.Contains(lines[6], "11 │") {
		t.Errorf("removed line should show old line 11, got %q", lines[6])
	}
	// Line 7: "+added1" -> new-file line 11
	if !strings.Contains(lines[7], "11 │") {
		t.Errorf("added line should show new line 11, got %q", lines[7])
	}
	// Line 8: "+added2" -> new-file line 12
	if !strings.Contains(lines[8], "12 │") {
		t.Errorf("added line should show new line 12, got %q", lines[8])
	}
	// Line 9: " context" -> new-file line 13
	if !strings.Contains(lines[9], "13 │") {
		t.Errorf("context line should show new line 13, got %q", lines[9])
	}
}

func TestNewDiffView_GutterBlankForNonDiffContent(t *testing.T) {
	t.Parallel()
	dv := NewDiffView(80, 10, true)
	dv.SetContent("just some text")

	view := dv.View()
	// Should have the separator but no line numbers
	if !strings.Contains(view, "│") {
		t.Error("gutter should still show separator")
	}
	if strings.Contains(view, "1 │") {
		t.Error("non-diff content should not show sequential line numbers")
	}
}

func TestNewDiffView_NoGutterWhenDisabled(t *testing.T) {
	t.Parallel()
	dv := NewDiffView(80, 10, false)
	dv.SetContent("some content")

	view := dv.View()
	if strings.Contains(view, "│") {
		t.Error("gutter should not appear when showLineNumbers is false")
	}
}

func TestNewDiffView_GutterFillLinesShowTilde(t *testing.T) {
	t.Parallel()
	dv := NewDiffView(80, 10, true)
	dv.SetContent("only one line")

	view := dv.View()
	lines := strings.Split(view, "\n")

	// Lines beyond content should show tilde marker
	if len(lines) >= 3 {
		if !strings.Contains(lines[2], "~") {
			t.Errorf("fill line should contain tilde marker, got %q", lines[2])
		}
	}
}

func TestDiffView_SearchFindsMatches(t *testing.T) {
	t.Parallel()
	dv := NewDiffView(80, 10, true)
	dv.SetContent("foo bar\nbaz foo\nqux")

	dv.Search("foo")
	if !dv.HasSearch() {
		t.Error("HasSearch should be true after Search")
	}
	if dv.MatchCount() != 2 {
		t.Errorf("expected 2 matches, got %d", dv.MatchCount())
	}
}

func TestDiffView_SearchNoMatches(t *testing.T) {
	t.Parallel()
	dv := NewDiffView(80, 10, true)
	dv.SetContent("foo bar baz")

	dv.Search("xyz")
	if dv.MatchCount() != 0 {
		t.Errorf("expected 0 matches, got %d", dv.MatchCount())
	}
	if !dv.HasSearch() {
		t.Error("HasSearch should be true even with no matches")
	}
}

func TestDiffView_ClearSearch(t *testing.T) {
	t.Parallel()
	dv := NewDiffView(80, 10, true)
	dv.SetContent("foo bar foo")

	dv.Search("foo")
	if dv.MatchCount() != 2 {
		t.Errorf("expected 2 matches before clear, got %d", dv.MatchCount())
	}

	dv.ClearSearch()
	if dv.HasSearch() {
		t.Error("HasSearch should be false after ClearSearch")
	}
	if dv.MatchCount() != 0 {
		t.Errorf("expected 0 matches after clear, got %d", dv.MatchCount())
	}
}

func TestDiffView_SearchNextDoesNotPanic(t *testing.T) {
	t.Parallel()
	dv := NewDiffView(80, 5, true)
	// Make content tall enough that matches span multiple viewable pages.
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "foo line"
	}
	dv.SetContent(strings.Join(lines, "\n"))
	dv.Search("foo")
	if dv.MatchCount() == 0 {
		t.Fatal("expected matches for 'foo'")
	}
	dv.SearchNext()
	dv.SearchNext()
}

func TestDiffView_SearchPrevDoesNotPanic(t *testing.T) {
	t.Parallel()
	dv := NewDiffView(80, 5, true)
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "bar line"
	}
	dv.SetContent(strings.Join(lines, "\n"))
	dv.Search("bar")
	dv.SearchNext()
	dv.SearchPrev()
}

func TestDiffView_SearchInvalidRegexFallsBackToLiteral(t *testing.T) {
	t.Parallel()
	dv := NewDiffView(80, 10, true)
	dv.SetContent("foo (bar) baz (bar)")

	dv.Search("(bar")
	// Invalid regex should be treated as literal string via QuoteMeta
	if dv.MatchCount() != 2 {
		t.Errorf("expected 2 literal matches for invalid regex, got %d", dv.MatchCount())
	}
}
