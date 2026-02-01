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
// centered over background using line-by-line compositing so background content
// remains visible outside the box.
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

	// Composite the box centered over the background.
	return overlayOnBackground(background, box, width, height)
}

// overlayOnBackground places box centered over background line-by-line.
// Lines of the background outside the overlay region are preserved verbatim.
func overlayOnBackground(background, box string, width, height int) string {
	bgLines := strings.Split(background, "\n")
	boxLines := strings.Split(box, "\n")

	boxH := len(boxLines)
	boxW := min(
		// Clamp to available space.
		lipgloss.Width(box), width)

	topStart := max((height-boxH)/2, 0)
	leftStart := max((width-boxW)/2, 0)

	// Ensure background has enough lines to cover height.
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}

	out := make([]string, height)
	for i := range height {
		bgLine := ""
		if i < len(bgLines) {
			bgLine = bgLines[i]
		}

		boxRow := i - topStart
		if boxRow < 0 || boxRow >= len(boxLines) {
			// Outside overlay region: emit background line as-is.
			out[i] = bgLine
			continue
		}

		// Composite this row: background | overlay | background remainder.
		overlayLine := boxLines[boxRow]
		overlayLineW := lipgloss.Width(overlayLine)

		// Left segment of background (runes before leftStart column).
		leftSeg := runeColumns(bgLine, 0, leftStart)
		// Pad left segment to exactly leftStart columns.
		leftSeg = padRight(leftSeg, leftStart)

		// Right segment of background (runes after leftStart+boxW columns).
		rightSeg := runeColumnsFrom(bgLine, leftStart+overlayLineW)

		out[i] = leftSeg + overlayLine + rightSeg
	}

	return strings.Join(out, "\n")
}

// runeColumns returns the substring of s covering display columns [from, to).
// It is ANSI-naive: treats each byte sequence without escape codes as one cell.
// For plain text backgrounds this is sufficient.
func runeColumns(s string, from, to int) string {
	if from >= to {
		return ""
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if col >= to {
			break
		}
		if col >= from {
			b.WriteRune(r)
		}
		col++
	}
	return b.String()
}

// runeColumnsFrom returns the substring of s starting at display column start.
func runeColumnsFrom(s string, start int) string {
	runes := []rune(s)
	if start >= len(runes) {
		return ""
	}
	if start <= 0 {
		return s
	}
	return string(runes[start:])
}

// padRight pads s with spaces on the right until its rune length equals width.
func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return s + strings.Repeat(" ", width-len(runes))
}
