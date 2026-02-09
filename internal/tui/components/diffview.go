// Package components provides reusable TUI widgets for the gti interface.
package components

import (
	"strings"

	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// DiffView wraps bubbles/v2/viewport for scrollable diff content.
type DiffView struct {
	vp         viewport.Model
	rawContent string
	softWrap   bool
}

// NewDiffView creates a DiffView with the given dimensions.
func NewDiffView(width, height int) DiffView {
	vp := viewport.New(
		viewport.WithWidth(width),
		viewport.WithHeight(height),
	)
	return DiffView{vp: vp}
}

// SetContent stores the raw content and applies it to the viewport, wrapping if enabled.
func (d *DiffView) SetContent(s string) {
	d.rawContent = s
	d.applyContent()
}

// applyContent pushes rawContent into the viewport, wrapping long lines when softWrap is true.
func (d *DiffView) applyContent() {
	if !d.softWrap {
		d.vp.SetContent(d.rawContent)
		return
	}
	d.vp.SetContent(wrapContent(d.rawContent, d.vp.Width()))
}

// wrapContent wraps lines in s that exceed width visible characters.
// It uses ANSI-aware word wrapping (breaking at spaces) with a fallback to
// forced character breaks for overlong tokens. Continuation lines are indented
// with 1 space to align with content after the unified diff prefix (+/-/ ).
func wrapContent(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		// First pass: break at word boundaries (ANSI-aware).
		wrapped := wordwrap.String(line, width)
		// Second pass: force-break any remaining overlong words.
		wrapped = wrap.String(wrapped, width)

		parts := strings.Split(wrapped, "\n")
		out = append(out, parts[0])
		// Indent continuation lines with 1 space, wrapping at width-1
		// to account for the indent. Each sub-line gets the indent.
		for _, cont := range parts[1:] {
			effWidth := max(width-1, 1)
			rewrapped := wordwrap.String(cont, effWidth)
			rewrapped = wrap.String(rewrapped, effWidth)
			for sub := range strings.SplitSeq(rewrapped, "\n") {
				out = append(out, " "+sub)
			}
		}
	}
	return strings.Join(out, "\n")
}

// SetSoftWrap enables or disables soft-wrap and re-applies the stored content.
func (d *DiffView) SetSoftWrap(enabled bool) {
	d.softWrap = enabled
	d.applyContent()
}

// SoftWrap reports whether soft-wrap is currently enabled.
func (d *DiffView) SoftWrap() bool { return d.softWrap }

// ScrollOffset returns the current vertical scroll offset.
func (d *DiffView) ScrollOffset() int { return d.vp.YOffset() }

// XOffset returns the current horizontal scroll offset.
func (d *DiffView) XOffset() int { return d.vp.XOffset() }

// SetWidth sets the viewport width and re-applies content so wrap reflects the new width.
func (d *DiffView) SetWidth(w int) {
	d.vp.SetWidth(w)
	if d.softWrap {
		d.applyContent()
	}
}

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
