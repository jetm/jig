package tui

import tea "charm.land/bubbletea/v2"

// LeftPanel is the interface that all left-panel component types must satisfy.
// FileList, ItemList, and HunkList each implement this interface, allowing
// TwoPanelModel to treat them uniformly for layout, resize, and key forwarding.
type LeftPanel interface {
	View() string
	SetWidth(int)
	SetHeight(int)
	Update(tea.Msg) tea.Cmd
}
