package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
