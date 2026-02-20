package components

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
)

func newTestList() *ItemList {
	items := []list.Item{
		NewSimpleItem("alpha", "first"),
		NewSimpleItem("beta", "second"),
		NewSimpleItem("gamma", "third"),
		NewSimpleItem("main-branch", "fourth"),
	}
	il := NewItemList(items, 40, 20)
	return &il
}

func TestItemListInitialSelection(t *testing.T) {
	il := newTestList()
	sel := il.SelectedItem()
	if sel == nil {
		t.Fatal("SelectedItem() returned nil")
	}
	if sel.(SimpleItem).Title() != "alpha" {
		t.Errorf("initial selection = %q, want alpha", sel.(SimpleItem).Title())
	}
}

func TestItemListJMovesDown(t *testing.T) {
	il := newTestList()
	sendKey(il, 'j')
	sel := il.SelectedItem()
	if sel == nil {
		t.Fatal("SelectedItem() returned nil after j")
	}
	if sel.(SimpleItem).Title() != "beta" {
		t.Errorf("after j, selection = %q, want beta", sel.(SimpleItem).Title())
	}
}

func TestItemListKMovesUp(t *testing.T) {
	il := newTestList()
	sendKey(il, 'j') // move to beta
	sendKey(il, 'k') // back to alpha
	sel := il.SelectedItem()
	if sel == nil {
		t.Fatal("SelectedItem() returned nil after k")
	}
	if sel.(SimpleItem).Title() != "alpha" {
		t.Errorf("after j+k, selection = %q, want alpha", sel.(SimpleItem).Title())
	}
}

func TestItemListCursorClampsAtTop(t *testing.T) {
	il := newTestList()
	sendKey(il, 'k') // already at top
	sel := il.SelectedItem()
	if sel.(SimpleItem).Title() != "alpha" {
		t.Errorf("cursor should clamp at top, got %q", sel.(SimpleItem).Title())
	}
}

func TestItemListCursorClampsAtBottom(t *testing.T) {
	il := newTestList()
	for range 10 { // more than item count
		sendKey(il, 'j')
	}
	sel := il.SelectedItem()
	if sel.(SimpleItem).Title() != "main-branch" {
		t.Errorf("cursor should clamp at bottom, got %q", sel.(SimpleItem).Title())
	}
}

func TestItemListFilterNarrows(t *testing.T) {
	il := newTestList()
	sendKey(il, '/')
	for _, ch := range "main" {
		sendKey(il, ch)
	}
	view := il.View()
	if !strings.Contains(view, "main-branch") {
		t.Error("filtered view should contain main-branch")
	}
}

func TestItemListEscClearsFilter(t *testing.T) {
	il := newTestList()
	sendKey(il, '/')
	for _, ch := range "main" {
		sendKey(il, ch)
	}
	sendSpecialKey(il, 0x1b) // escape
	view := il.View()
	if !strings.Contains(view, "alpha") {
		t.Error("after esc, all items should be visible (alpha missing)")
	}
}

func TestItemListSetItemsReplacesItems(t *testing.T) {
	il := newTestList()
	il.SetItems([]list.Item{
		NewSimpleItem("new1", "desc"),
		NewSimpleItem("new2", "desc"),
	})
	sel := il.SelectedItem()
	if sel.(SimpleItem).Title() != "new1" {
		t.Errorf("after SetItems, selection = %q, want new1", sel.(SimpleItem).Title())
	}
}

func TestItemListSetWidthAndHeight(t *testing.T) {
	il := newTestList()
	il.SetWidth(60)
	il.SetHeight(30)
	// No panic means success; verify view still renders
	if il.View() == "" {
		t.Error("View() should not be empty after resize")
	}
}

func TestSimpleItemFilterValue(t *testing.T) {
	item := NewSimpleItem("test-title", "desc")
	if item.FilterValue() != "test-title" {
		t.Errorf("FilterValue() = %q, want test-title", item.FilterValue())
	}
}

func TestCompactDelegateSpacingIsZero(t *testing.T) {
	d := CompactDelegate{}
	if d.Spacing() != 0 {
		t.Errorf("Spacing() = %d, want 0", d.Spacing())
	}
}

func TestCompactDelegateHeightIsOne(t *testing.T) {
	d := CompactDelegate{}
	if d.Height() != 1 {
		t.Errorf("Height() = %d, want 1", d.Height())
	}
}

func TestCompactItemListRendersCompact(t *testing.T) {
	items := []list.Item{
		NewSimpleItem("first", "desc1"),
		NewSimpleItem("second", "desc2"),
		NewSimpleItem("third", "desc3"),
	}
	il := NewCompactItemList(items, 40, 10)

	view := il.View()
	// Each item should be on its own line with no blank lines between them.
	lines := strings.Split(view, "\n")
	// Filter out empty trailing lines from the viewport.
	var nonEmpty []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) != 3 {
		t.Errorf("expected 3 non-empty lines, got %d:\n%s", len(nonEmpty), view)
	}
	if !strings.Contains(view, "first") {
		t.Error("view should contain 'first'")
	}
	if !strings.Contains(view, "second") {
		t.Error("view should contain 'second'")
	}
	if !strings.Contains(view, "third") {
		t.Error("view should contain 'third'")
	}
}

func TestCompactItemListNavigates(t *testing.T) {
	items := []list.Item{
		NewSimpleItem("alpha", ""),
		NewSimpleItem("beta", ""),
	}
	il := NewCompactItemList(items, 40, 10)

	sel := il.SelectedItem()
	if sel.(SimpleItem).Title() != "alpha" {
		t.Errorf("initial selection = %q, want alpha", sel.(SimpleItem).Title())
	}

	sendKey(&il, 'j')
	sel = il.SelectedItem()
	if sel.(SimpleItem).Title() != "beta" {
		t.Errorf("after j, selection = %q, want beta", sel.(SimpleItem).Title())
	}
}

func TestItemListSelect(t *testing.T) {
	t.Parallel()
	il := newTestList()
	il.Select(1)
	sel := il.SelectedItem()
	if sel == nil {
		t.Fatal("SelectedItem() returned nil after Select(1)")
	}
	if sel.(SimpleItem).Title() != "beta" {
		t.Errorf("after Select(1), selection = %q, want beta", sel.(SimpleItem).Title())
	}
}
