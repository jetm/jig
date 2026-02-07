package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jetm/gti/internal/tui"
)

// KeyBinding describes a single keybinding for the help overlay.
type KeyBinding struct {
	Key  string
	Desc string
}

// KeyGroup is a named group of keybindings.
type KeyGroup struct {
	Name     string
	Bindings []KeyBinding
}

// HelpOverlay renders a modal keybinding reference overlay.
type HelpOverlay struct {
	groups  []KeyGroup
	visible bool
}

// NewHelpOverlay creates a HelpOverlay with the given keybinding groups.
func NewHelpOverlay(groups []KeyGroup) HelpOverlay {
	return HelpOverlay{groups: groups}
}

// Toggle flips the overlay visibility.
func (h *HelpOverlay) Toggle() { h.visible = !h.visible }

// Dismiss hides the overlay.
func (h *HelpOverlay) Dismiss() { h.visible = false }

// IsVisible reports whether the overlay is currently shown.
func (h *HelpOverlay) IsVisible() bool { return h.visible }

// HandleKey processes a key event for the overlay. It returns true if the key
// was consumed (overlay was visible, or ? was pressed to show it). When visible,
// ? toggles off, q/Esc dismiss, and all other keys are swallowed. When hidden,
// only ? is consumed (to show the overlay).
func (h *HelpOverlay) HandleKey(msg tea.KeyPressMsg) bool {
	if msg.Text == "?" {
		h.Toggle()
		return true
	}
	if h.visible {
		if msg.Code == 'q' || msg.Code == tea.KeyEscape {
			h.Dismiss()
		}
		return true
	}
	return false
}

// View renders the overlay composited over background. When the overlay is
// hidden, background is returned unchanged. When visible, the overlay box is
// centered over background using lipgloss Canvas/Layer compositing so that
// ANSI-styled backgrounds are handled correctly.
func (h *HelpOverlay) View(background string, width, height int) string {
	if !h.visible {
		return background
	}

	var sections []string
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(tui.ColorFgEmph)
	keyStyle := lipgloss.NewStyle().
		Foreground(tui.ColorCyan).
		Bold(true)
	descStyle := lipgloss.NewStyle().
		Foreground(tui.ColorFg)

	for _, g := range h.groups {
		lines := []string{headerStyle.Render(g.Name)}
		for _, b := range g.Bindings {
			lines = append(lines, "  "+keyStyle.Render(b.Key)+" "+descStyle.Render(b.Desc))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	content := strings.Join(sections, "\n\n")

	overlayStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(tui.ColorBgSel).
		Background(tui.ColorBgFloat).
		Foreground(tui.ColorFg).
		Padding(1, 2).
		Align(lipgloss.Center)

	box := overlayStyle.Render(content)

	// Center the box over the background using ANSI-aware lipgloss dimensions.
	boxW := lipgloss.Width(box)
	boxH := lipgloss.Height(box)
	x := max((width-boxW)/2, 0)
	y := max((height-boxH)/2, 0)

	bgLayer := lipgloss.NewLayer(background).Z(0)
	boxLayer := lipgloss.NewLayer(box).X(x).Y(y).Z(1)
	comp := lipgloss.NewCompositor(bgLayer, boxLayer)
	canvas := lipgloss.NewCanvas(width, height)
	canvas.Compose(comp)
	return canvas.Render()
}
