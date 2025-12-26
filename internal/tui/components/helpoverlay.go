package components

import (
	"strings"

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

// IsVisible reports whether the overlay is currently shown.
func (h *HelpOverlay) IsVisible() bool { return h.visible }

// View renders the help overlay content centered within the given dimensions.
func (h *HelpOverlay) View(width, height int) string {
	if !h.visible {
		return ""
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

	rendered := overlayStyle.Render(content)

	// Center vertically and horizontally
	renderedLines := strings.Split(rendered, "\n")
	renderedWidth := lipgloss.Width(rendered)
	topPad := (height - len(renderedLines)) / 2
	leftPad := (width - renderedWidth) / 2
	if topPad < 0 {
		topPad = 0
	}
	if leftPad < 0 {
		leftPad = 0
	}

	var out strings.Builder
	for range topPad {
		out.WriteString("\n")
	}
	pad := strings.Repeat(" ", leftPad)
	for _, line := range renderedLines {
		out.WriteString(pad)
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}
