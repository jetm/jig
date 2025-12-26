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
	view := h.View(80, 24)
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
	view := h.View(80, 24)
	expected := []string{"Navigation", "j", "down", "k", "up", "Actions", "space", "stage"}
	for _, s := range expected {
		if !strings.Contains(view, s) {
			t.Errorf("View() should contain %q", s)
		}
	}
}

func TestHelpOverlayHiddenReturnsEmpty(t *testing.T) {
	h := newTestOverlay()
	view := h.View(80, 24)
	if view != "" {
		t.Error("hidden overlay should return empty string")
	}
}

func TestHelpOverlayComposableWithContent(t *testing.T) {
	h := newTestOverlay()
	h.Toggle()
	underlying := "underlying content here"
	overlay := h.View(80, 24)
	// The overlay and underlying content should be independently renderable
	composed := underlying + "\n" + overlay
	if !strings.Contains(composed, "underlying content here") {
		t.Error("composed view should contain underlying content")
	}
	if !strings.Contains(composed, "Navigation") {
		t.Error("composed view should contain overlay content")
	}
}

// TestHelpOverlayKeyHandling verifies ? and esc toggle/dismiss behavior
// as it would be handled by a parent model.
func TestHelpOverlayKeyHandling(t *testing.T) {
	h := newTestOverlay()

	// Simulate parent model handling ? to toggle
	msg := tea.KeyPressMsg{Code: '?', Text: "?"}
	if msg.String() == "?" {
		h.Toggle()
	}
	if !h.IsVisible() {
		t.Error("overlay should be visible after ?")
	}

	// Simulate parent model handling esc to dismiss
	escMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	if escMsg.String() == "escape" || escMsg.String() == "esc" {
		h.Toggle()
	}
	if h.IsVisible() {
		t.Error("overlay should be hidden after esc")
	}
}
