package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/git"
)

// PushModelMsg tells AppModel to push a new command model onto the stack.
type PushModelMsg struct{ Model tea.Model }

// PopModelMsg tells AppModel to pop the current model and return to the parent.
type PopModelMsg struct{ MutatedGit bool }

// RefreshMsg is sent to the parent model after a mutating child returns.
type RefreshMsg struct{}

// AppModel is the root model managing a navigation stack of tea.Model instances.
type AppModel struct {
	stack  []tea.Model
	Runner git.Runner
	Config config.Config
}

// NewAppModel creates an AppModel with the initial model on the stack.
func NewAppModel(initial tea.Model, runner git.Runner, cfg config.Config) *AppModel {
	return &AppModel{
		stack:  []tea.Model{initial},
		Runner: runner,
		Config: cfg,
	}
}

// Push adds a model to the top of the stack.
func (a *AppModel) Push(m tea.Model) { a.stack = append(a.stack, m) }

// Pop removes the top model and returns the new top. Returns nil if stack has only one model.
func (a *AppModel) Pop() tea.Model {
	if len(a.stack) <= 1 {
		return nil
	}
	a.stack[len(a.stack)-1] = nil // allow GC
	a.stack = a.stack[:len(a.stack)-1]
	return a.stack[len(a.stack)-1]
}

// Active returns the model at the top of the stack.
func (a *AppModel) Active() tea.Model { return a.stack[len(a.stack)-1] }

// Init implements tea.Model.
func (a *AppModel) Init() tea.Cmd { return a.Active().Init() }

// Update implements tea.Model.
func (a *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
func (a *AppModel) View() tea.View {
	v := a.Active().View()
	v.AltScreen = true
	return v
}
