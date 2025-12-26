// Package components provides reusable TUI widgets for the gti interface.
package components

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// DiffView wraps bubbles/v2/viewport for scrollable diff content.
type DiffView struct {
	vp viewport.Model
}

// NewDiffView creates a DiffView with the given dimensions.
func NewDiffView(width, height int) DiffView {
	vp := viewport.New(
		viewport.WithWidth(width),
		viewport.WithHeight(height),
	)
	return DiffView{vp: vp}
}

// SetContent sets the viewport content.
func (d *DiffView) SetContent(s string) { d.vp.SetContent(s) }

// ScrollOffset returns the current vertical scroll offset.
func (d *DiffView) ScrollOffset() int { return d.vp.YOffset() }

// SetWidth sets the viewport width.
func (d *DiffView) SetWidth(w int) { d.vp.SetWidth(w) }

// SetHeight sets the viewport height.
func (d *DiffView) SetHeight(h int) { d.vp.SetHeight(h) }

// View renders the viewport as a string.
func (d *DiffView) View() string { return d.vp.View() }

// Update forwards messages to the inner viewport and returns any command.
func (d *DiffView) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	d.vp, cmd = d.vp.Update(msg)
	return cmd
}
