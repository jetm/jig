package components

import (
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jetm/jig/internal/tui"
)

// MessageKind indicates the type of status bar feedback message.
type MessageKind int

const (
	// Info indicates a neutral informational message.
	Info MessageKind = iota
	// Success indicates a successful operation.
	Success
	// Error indicates a failed operation.
	Error
)

// ClearMessageMsg is sent when a transient message should be cleared.
type ClearMessageMsg struct{}

// StatusBar renders a bottom bar with keyhints, branch, mode, and messages.
type StatusBar struct {
	hints   string
	branch  string
	mode    string
	message string
	msgKind MessageKind
	width   int
}

// NewStatusBar creates a StatusBar with the given width.
func NewStatusBar(width int) StatusBar {
	return StatusBar{width: width}
}

// SetHints sets the keyhint text displayed on the left.
func (s *StatusBar) SetHints(h string) { s.hints = h }

// SetBranch sets the branch name displayed on the right.
func (s *StatusBar) SetBranch(name string) { s.branch = name }

// SetMode sets the mode label displayed on the right.
func (s *StatusBar) SetMode(mode string) { s.mode = mode }

// SetWidth sets the bar width.
func (s *StatusBar) SetWidth(w int) { s.width = w }

// SetMessage sets a transient feedback message and returns a command that
// will clear it after 3 seconds.
func (s *StatusBar) SetMessage(text string, kind MessageKind) tea.Cmd {
	s.message = text
	s.msgKind = kind
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return ClearMessageMsg{}
	})
}

// Update handles ClearMessageMsg and clears messages on any keypress.
func (s *StatusBar) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case ClearMessageMsg:
		s.message = ""
	case tea.KeyPressMsg:
		s.message = ""
	}
	return nil
}

// View renders the status bar as a string.
func (s *StatusBar) View() string {
	barStyle := lipgloss.NewStyle().
		Background(tui.ColorBgFloat).
		Foreground(tui.ColorFg).
		Width(s.width)

	// Left side: hints or message
	var left string
	if s.message != "" {
		left = s.renderMessage()
	} else {
		left = s.hints
	}

	// Right side: mode + branch
	var rightParts []string
	if s.mode != "" {
		modeStyle := lipgloss.NewStyle().
			Foreground(tui.ColorFgSubtle).
			Italic(true)
		rightParts = append(rightParts, modeStyle.Render(s.mode))
	}
	if s.branch != "" {
		branchStyle := lipgloss.NewStyle().
			Foreground(tui.ColorBlue).
			Bold(true)
		rightParts = append(rightParts, branchStyle.Render(tui.IconBranch+" "+s.branch))
	}
	right := strings.Join(rightParts, " ")

	// Compose: left-aligned hints, right-aligned branch/mode
	gap := max(s.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	content := left + strings.Repeat(" ", gap) + right
	return barStyle.Render(content)
}

func (s *StatusBar) renderMessage() string {
	var icon string
	var fg color.Color
	switch s.msgKind {
	case Success:
		icon = tui.IconSuccess
		fg = tui.ColorGreen
	case Error:
		icon = tui.IconError
		fg = tui.ColorRed
	default:
		icon = ""
		fg = tui.ColorFg
	}
	style := lipgloss.NewStyle().Foreground(fg)
	if icon != "" {
		return style.Render(icon + " " + s.message)
	}
	return style.Render(s.message)
}
