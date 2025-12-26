package components

import tea "charm.land/bubbletea/v2"

// updater is the interface for components that accept Bubbletea messages.
type updater interface {
	Update(tea.Msg) tea.Cmd
}

// sendKey simulates a printable key press through a component's Update method.
func sendKey(u updater, key rune) {
	msg := tea.KeyPressMsg{Code: key, Text: string(key)}
	u.Update(msg)
}

// sendSpecialKey simulates a special key press (e.g., tea.KeyEscape).
func sendSpecialKey(u updater, code rune) {
	msg := tea.KeyPressMsg{Code: code}
	u.Update(msg)
}
