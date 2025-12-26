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

func TestDiffViewSetWidthAndHeight(t *testing.T) {
	dv := newTestDiffView("content")
	dv.SetWidth(100)
	dv.SetHeight(20)
	if dv.View() == "" {
		t.Error("View() should not be empty after resize")
	}
}
