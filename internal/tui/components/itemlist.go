package components

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

// SimpleItem implements list.Item and list.DefaultItem for use with ItemList.
type SimpleItem struct {
	title string
	desc  string
}

// NewSimpleItem creates a SimpleItem with the given title and description.
func NewSimpleItem(title, desc string) SimpleItem {
	return SimpleItem{title: title, desc: desc}
}

// Title returns the item's display title.
func (i SimpleItem) Title() string { return i.title }

// Description returns the item's description text.
func (i SimpleItem) Description() string { return i.desc }

// FilterValue returns the string used for filtering (same as title).
func (i SimpleItem) FilterValue() string { return i.title }

// ItemList wraps bubbles/v2/list with a simplified API.
type ItemList struct {
	list list.Model
}

// NewItemList creates an ItemList with the given items and dimensions.
func NewItemList(items []list.Item, width, height int) ItemList {
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, width, height)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.DisableQuitKeybindings()
	return ItemList{list: l}
}

// SelectedItem returns the currently highlighted item, or nil if the list is empty.
func (il *ItemList) SelectedItem() list.Item {
	return il.list.SelectedItem()
}

// SetItems replaces the list items.
func (il *ItemList) SetItems(items []list.Item) tea.Cmd {
	return il.list.SetItems(items)
}

// SetWidth sets the list width.
func (il *ItemList) SetWidth(w int) { il.list.SetWidth(w) }

// SetHeight sets the list height.
func (il *ItemList) SetHeight(h int) { il.list.SetHeight(h) }

// View renders the list as a string.
func (il *ItemList) View() string { return il.list.View() }

// Select moves the cursor to the given index.
func (il *ItemList) Select(index int) { il.list.Select(index) }

// Update forwards messages to the inner list model and returns any command.
func (il *ItemList) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	il.list, cmd = il.list.Update(msg)
	return cmd
}
