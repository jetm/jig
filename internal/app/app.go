// Package app provides the root tea.Model managing a navigation stack.
package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/git"
)

// PushModelMsg tells Model to push a new command model onto the stack.
type PushModelMsg struct{ Model tea.Model }

// PopModelMsg tells Model to pop the current model and return to the parent.
type PopModelMsg struct{ MutatedGit bool }

// RefreshMsg is sent to the parent model after a mutating child returns.
type RefreshMsg struct{}

// AbortMsg signals the program should exit with a non-zero code.
// Used by editor mode when the user aborts.
type AbortMsg struct{}

// Model is the root model managing a navigation stack of tea.Model instances.
type Model struct {
	stack   []tea.Model
	Runner  git.Runner
	Config  config.Config
	Aborted bool // true when an abort was requested (non-zero exit)
}

// New creates a Model with the initial model on the stack.
func New(initial tea.Model, runner git.Runner, cfg config.Config) *Model {
	return &Model{
		stack:  []tea.Model{initial},
		Runner: runner,
		Config: cfg,
	}
}

// Push adds a model to the top of the stack.
func (a *Model) Push(m tea.Model) { a.stack = append(a.stack, m) }

// Pop removes the top model and returns the new top. Returns nil if stack has only one model.
func (a *Model) Pop() tea.Model {
	if len(a.stack) <= 1 {
		return nil
	}
	a.stack[len(a.stack)-1] = nil // allow GC
	a.stack = a.stack[:len(a.stack)-1]
	return a.stack[len(a.stack)-1]
}

// Active returns the model at the top of the stack.
func (a *Model) Active() tea.Model { return a.stack[len(a.stack)-1] }

// Init implements tea.Model.
func (a *Model) Init() tea.Cmd { return a.Active().Init() }

// Update implements tea.Model.
func (a *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case AbortMsg:
		a.Aborted = true
		return a, tea.Quit
	case PushModelMsg:
		a.Push(msg.Model)
		return a, msg.Model.Init()
	case PopModelMsg:
		parent := a.Pop()
		if parent == nil {
			return a, tea.Quit
		}
		if msg.MutatedGit {
			return a, func() tea.Msg { return RefreshMsg{} }
		}
		return a, nil
	default:
		active, cmd := a.Active().Update(msg)
		a.stack[len(a.stack)-1] = active
		return a, cmd
	}
}

// View implements tea.Model. Returns tea.View with AltScreen enabled.
func (a *Model) View() tea.View {
	v := a.Active().View()
	v.AltScreen = true
	return v
}
