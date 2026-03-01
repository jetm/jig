package commands

import tea "charm.land/bubbletea/v2"

// ChildModel is the interface that all command models satisfy.
// It captures the Update/View contract without requiring full tea.Model.
type ChildModel interface {
	Update(tea.Msg) tea.Cmd
	View() string
}

// teaModelAdapter wraps a ChildModel as a tea.Model.
type teaModelAdapter struct {
	inner ChildModel
}

// NewTeaModelAdapter creates a tea.Model that delegates to a ChildModel.
func NewTeaModelAdapter(m ChildModel) tea.Model {
	return &teaModelAdapter{inner: m}
}

func (a *teaModelAdapter) Init() tea.Cmd { return nil }

func (a *teaModelAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := a.inner.Update(msg)
	return a, cmd
}

func (a *teaModelAdapter) View() tea.View {
	return tea.NewView(a.inner.View())
}
