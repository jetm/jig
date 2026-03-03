// Package components provides reusable TUI widgets for the jig interface.
package components

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/diff"
)

// DiffView wraps bubbles/v2/viewport for scrollable diff content.
type DiffView struct {
	vp              viewport.Model
	rawContent      string
	softWrap        bool
	searching       bool
	matchCount      int
	lineMap         []diff.LineInfo
	showLineNumbers bool
	gutterInstalled bool
}

// NewDiffView creates a DiffView with the given dimensions.
// When showLineNumbers is true, the gutter displays source file line numbers
// parsed from unified diff hunk headers. When false, no gutter is shown.
func NewDiffView(width, height int, showLineNumbers bool) DiffView {
	vp := viewport.New(
		viewport.WithWidth(width),
		viewport.WithHeight(height),
	)
	vp.FillHeight = true
	return DiffView{vp: vp, showLineNumbers: showLineNumbers}
}

// initGutter installs the gutter function on the viewport.
// Must be called after the DiffView is stored at its final address (not from the constructor,
// since the constructor returns by value and the method value would bind to a stale copy).
func (d *DiffView) initGutter() {
	if d.gutterInstalled || !d.showLineNumbers {
		return
	}
	d.gutterInstalled = true
	d.vp.LeftGutterFunc = d.gutterFunc
}

// gutterFunc renders source line numbers in the viewport gutter.
func (d *DiffView) gutterFunc(info viewport.GutterContext) string {
	if info.Soft {
		return "     \u2502 "
	}
	if info.Index >= info.TotalLines {
		return "   ~ \u2502 "
	}
	if info.Index < len(d.lineMap) {
		li := d.lineMap[info.Index]
		if li.Num > 0 {
			return fmt.Sprintf("%4d \u2502 ", li.Num)
		}
	}
	return "     \u2502 "
}

// SetContent stores non-diff content (e.g. error messages) and clears the line map.
func (d *DiffView) SetContent(s string) {
	d.initGutter()
	d.lineMap = nil
	d.rawContent = s
	d.applyContent()
}

// SetDiffContent stores rendered diff content and builds the line-number map from raw diff.
// Use this instead of SetContent when displaying actual diff output.
func (d *DiffView) SetDiffContent(raw, rendered string) {
	d.initGutter()
	if d.showLineNumbers {
		d.lineMap = diff.ParseLineNumbers(raw)
	}
	d.rawContent = rendered
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

// Search highlights all matches of query in the viewport content.
// If query is not valid regex, it is treated as a literal string.
func (d *DiffView) Search(query string) {
	d.searching = true
	re, err := regexp.Compile(query)
	if err != nil {
		re = regexp.MustCompile(regexp.QuoteMeta(query))
	}
	content := d.vp.GetContent()
	indices := re.FindAllStringIndex(content, -1)
	d.matchCount = len(indices)
	if len(indices) > 0 {
		d.vp.SetHighlights(indices)
	}
}

// SearchNext moves to the next search match.
func (d *DiffView) SearchNext() { d.vp.HighlightNext() }

// SearchPrev moves to the previous search match.
func (d *DiffView) SearchPrev() { d.vp.HighlightPrevious() }

// ClearSearch removes all search highlights.
func (d *DiffView) ClearSearch() {
	d.searching = false
	d.matchCount = 0
	d.vp.ClearHighlights()
}

// HasSearch reports whether a search is active.
func (d *DiffView) HasSearch() bool { return d.searching }

// MatchCount returns the number of matches from the last search.
func (d *DiffView) MatchCount() int { return d.matchCount }
