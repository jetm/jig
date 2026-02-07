package components

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func newTestOverlay() *HelpOverlay {
	h := NewHelpOverlay([]KeyGroup{
		{Name: "Navigation", Bindings: []KeyBinding{
			{Key: "j", Desc: "down"},
			{Key: "k", Desc: "up"},
		}},
		{Name: "Actions", Bindings: []KeyBinding{
			{Key: "space", Desc: "stage"},
		}},
	})
	return &h
}

func TestHelpOverlayToggleShowsOverlay(t *testing.T) {
	h := newTestOverlay()
	if h.IsVisible() {
		t.Error("overlay should start hidden")
	}
	h.Toggle()
	if !h.IsVisible() {
		t.Error("overlay should be visible after Toggle()")
	}
	view := h.View("", 80, 24)
	if view == "" {
		t.Error("View() should return content when visible")
	}
}

func TestHelpOverlayToggleDismisses(t *testing.T) {
	h := newTestOverlay()
	h.Toggle() // show
	h.Toggle() // hide
	if h.IsVisible() {
		t.Error("overlay should be hidden after second Toggle()")
	}
}

func TestHelpOverlayContentReflectsBindings(t *testing.T) {
	h := newTestOverlay()
	h.Toggle()
	view := h.View("", 80, 24)
	expected := []string{"Navigation", "j", "down", "k", "up", "Actions", "space", "stage"}
	for _, s := range expected {
		if !strings.Contains(view, s) {
			t.Errorf("View() should contain %q", s)
		}
	}
}

func TestHelpOverlayHiddenReturnsBackground(t *testing.T) {
	h := newTestOverlay()
	bg := "background content"
	view := h.View(bg, 80, 24)
	if view != bg {
		t.Errorf("hidden overlay should return background unchanged, got %q", view)
	}
}

// TestHelpOverlayCompositesOverBackground verifies the background is visible
// in lines not covered by the overlay box.
func TestHelpOverlayCompositesOverBackground(t *testing.T) {
	h := newTestOverlay()
	h.Toggle()

	// Build a background that spans 80x30 - put distinctive text on the first line
	// so it will be outside the overlay box (which is centered).
	var bgLines []string
	bgLines = append(bgLines, "BACKGROUND_FIRST_LINE_MARKER")
	for i := 1; i < 30; i++ {
		bgLines = append(bgLines, strings.Repeat(" ", 80))
	}
	bg := strings.Join(bgLines, "\n")

	result := h.View(bg, 80, 30)

	if !strings.Contains(result, "BACKGROUND_FIRST_LINE_MARKER") {
		t.Error("composited view should contain background content outside overlay box")
	}
	if !strings.Contains(result, "Navigation") {
		t.Error("composited view should contain overlay content")
	}
}

// TestHelpOverlayHandleKey_QuestionMark verifies ? toggles the overlay on/off.
func TestHelpOverlayHandleKey_QuestionMark(t *testing.T) {
	h := newTestOverlay()

	// ? while hidden: show overlay, return true
	consumed := h.HandleKey(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !consumed {
		t.Error("HandleKey(?) while hidden should return true")
	}
	if !h.IsVisible() {
		t.Error("overlay should be visible after HandleKey(?)")
	}

	// ? while visible: hide overlay, return true
	consumed = h.HandleKey(tea.KeyPressMsg{Code: '?', Text: "?"})
	if !consumed {
		t.Error("HandleKey(?) while visible should return true")
	}
	if h.IsVisible() {
		t.Error("overlay should be hidden after second HandleKey(?)")
	}
}

// TestHelpOverlayHandleKey_QDismisses verifies q dismisses the overlay.
func TestHelpOverlayHandleKey_QDismisses(t *testing.T) {
	h := newTestOverlay()
	h.Toggle() // show

	consumed := h.HandleKey(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if !consumed {
		t.Error("HandleKey(q) while visible should return true")
	}
	if h.IsVisible() {
		t.Error("overlay should be hidden after HandleKey(q)")
	}
}

// TestHelpOverlayHandleKey_EscDismisses verifies Esc dismisses the overlay.
func TestHelpOverlayHandleKey_EscDismisses(t *testing.T) {
	h := newTestOverlay()
	h.Toggle() // show

	consumed := h.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !consumed {
		t.Error("HandleKey(Esc) while visible should return true")
	}
	if h.IsVisible() {
		t.Error("overlay should be hidden after HandleKey(Esc)")
	}
}

// TestHelpOverlayHandleKey_SwallowsKeysWhenVisible verifies all keys are
// consumed when the overlay is visible.
func TestHelpOverlayHandleKey_SwallowsKeysWhenVisible(t *testing.T) {
	h := newTestOverlay()
	h.Toggle() // show

	// Some random key that is not ?, q, or Esc
	consumed := h.HandleKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if !consumed {
		t.Error("HandleKey(j) while visible should return true (swallowed)")
	}
	if !h.IsVisible() {
		t.Error("overlay should remain visible after non-dismiss key")
	}
}

// TestHelpOverlayHandleKey_PassesThroughWhenHidden verifies keys are not
// consumed when the overlay is hidden (except ?).
func TestHelpOverlayHandleKey_PassesThroughWhenHidden(t *testing.T) {
	h := newTestOverlay()

	consumed := h.HandleKey(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if consumed {
		t.Error("HandleKey(j) while hidden should return false")
	}
}

// TestHelpOverlayANSIStyledBackgroundCentering verifies that the overlay box is
// correctly centered over a background containing ANSI escape sequences. The old
// runeColumns implementation was ANSI-naive and would shift the overlay because
// it counted escape sequence bytes as display columns.
func TestHelpOverlayANSIStyledBackgroundCentering(t *testing.T) {
	h := newTestOverlay()
	h.Toggle()

	// Build a 40x20 background where every line is styled with a lipgloss color.
	// The ANSI escape sequences add bytes that the old rune-counting logic would
	// miscount as visible columns, pushing the overlay off-center.
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	var bgLines []string
	for i := range 20 {
		line := lineStyle.Render(strings.Repeat(fmt.Sprintf("%d", i%10), 40))
		bgLines = append(bgLines, line)
	}
	bg := strings.Join(bgLines, "\n")

	result := h.View(bg, 40, 20)

	// The overlay content must appear in the result.
	if !strings.Contains(result, "Navigation") {
		t.Error("composited view should contain overlay content")
	}
	// The background content must appear (lines above/below the overlay box).
	if !strings.Contains(result, "0") {
		t.Error("composited view should contain background content")
	}
}

// TestHelpOverlayHiddenReturnsBackgroundExact verifies that a hidden overlay
// returns the exact background string, including any ANSI escape sequences.
func TestHelpOverlayHiddenReturnsBackgroundExact(t *testing.T) {
	h := newTestOverlay()

	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	bg := lineStyle.Render("styled background")

	result := h.View(bg, 80, 24)
	if result != bg {
		t.Errorf("hidden overlay should return background unchanged\ngot:  %q\nwant: %q", result, bg)
	}
}

// boxDimensions returns the visual width and height of the overlay box and the
// number of leading spaces before the top border line in the rendered output.
// It locates the top (╭) and bottom (╰) border lines to measure box extent.
func boxDimensions(lines []string) (boxVisualWidth, boxHeight, leadingSpaces int, ok bool) {
	firstBoxLine, lastBoxLine := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "╭") {
			firstBoxLine = i
		}
		if strings.Contains(line, "╰") {
			lastBoxLine = i
		}
	}
	if firstBoxLine < 0 || lastBoxLine < 0 {
		return 0, 0, 0, false
	}
	topLine := lines[firstBoxLine]
	totalW := lipgloss.Width(topLine)
	leading := 0
	for _, r := range topLine {
		if r == ' ' {
			leading++
		} else {
			break
		}
	}
	return totalW - leading, lastBoxLine - firstBoxLine + 1, leading, true
}

// TestHelpOverlayVisualCentering verifies that the overlay box is horizontally
// and vertically centered using ANSI-aware width calculation. It builds an
// ANSI-styled background (which would confuse a rune-counting implementation)
// and checks that leading-space count and first-box-line index both equal
// (termDim - boxDim) / 2.
func TestHelpOverlayVisualCentering(t *testing.T) {
	h := newTestOverlay()
	h.Toggle()

	termWidth, termHeight := 80, 30

	// ANSI-styled background: every line has a color escape so a naive
	// rune-counter would overcount visual columns and shift the box.
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	var bgLines []string
	for i := range termHeight {
		bgLines = append(bgLines, lineStyle.Render(strings.Repeat(fmt.Sprintf("%d", i%10), termWidth)))
	}
	bg := strings.Join(bgLines, "\n")

	result := h.View(bg, termWidth, termHeight)
	lines := strings.Split(result, "\n")

	boxW, boxH, leading, ok := boxDimensions(lines)
	if !ok {
		t.Fatal("could not find box border lines in rendered output")
	}

	wantLeading := (termWidth - boxW) / 2
	if leading != wantLeading {
		t.Errorf("horizontal centering: got %d leading spaces, want %d (termWidth=%d boxWidth=%d)",
			leading, wantLeading, termWidth, boxW)
	}

	// Locate first box line index to check vertical centering.
	firstBoxLine := -1
	for i, line := range lines {
		if strings.Contains(line, "╭") {
			firstBoxLine = i
			break
		}
	}
	wantFirstLine := (termHeight - boxH) / 2
	if firstBoxLine != wantFirstLine {
		t.Errorf("vertical centering: box starts at line %d, want %d (termHeight=%d boxHeight=%d)",
			firstBoxLine, wantFirstLine, termHeight, boxH)
	}
}

// TestHelpOverlayNarrowTerminal verifies that when the terminal is barely wider
// than the overlay box, the box is left-aligned at column 0 rather than
// overflowing or panicking. The overlay content must still be rendered.
func TestHelpOverlayNarrowTerminal(t *testing.T) {
	h := newTestOverlay()
	h.Toggle()

	// Determine the natural box width from a wide render.
	wide := h.View("", 80, 24)
	wideLines := strings.Split(wide, "\n")
	boxW, _, _, ok := boxDimensions(wideLines)
	if !ok {
		t.Fatal("could not determine box visual width from wide render")
	}

	// Render at narrowWidth = boxW + 1 so centering gives 0 leading spaces:
	// (boxW+1 - boxW) / 2 = 0.
	narrowWidth := boxW + 1
	result := h.View("", narrowWidth, 20)
	lines := strings.Split(result, "\n")

	if !strings.Contains(result, "Navigation") {
		t.Error("narrow terminal: overlay content should still be rendered")
	}

	for _, line := range lines {
		if strings.Contains(line, "╭") {
			leading := 0
			for _, r := range line {
				if r == ' ' {
					leading++
				} else {
					break
				}
			}
			wantLeading := (narrowWidth - boxW) / 2 // = 0
			if leading != wantLeading {
				t.Errorf("narrow terminal: got %d leading spaces before box, want %d (narrowWidth=%d boxWidth=%d)",
					leading, wantLeading, narrowWidth, boxW)
			}
			break
		}
	}
}
