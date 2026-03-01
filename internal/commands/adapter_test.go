package commands

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeChild is a minimal ChildModel for testing the adapter.
type fakeChild struct {
	updated bool
	lastMsg tea.Msg
}

func (f *fakeChild) Update(msg tea.Msg) tea.Cmd {
	f.updated = true
	f.lastMsg = msg
	return nil
}

func (f *fakeChild) View() string {
	return "fake view"
}

func TestNewTeaModelAdapter_Init(t *testing.T) {
	t.Parallel()
	child := &fakeChild{}
	adapter := NewTeaModelAdapter(child)

	cmd := adapter.Init()
	assert.Nil(t, cmd)
}

func TestNewTeaModelAdapter_Update(t *testing.T) {
	t.Parallel()
	child := &fakeChild{}
	adapter := NewTeaModelAdapter(child)

	msg := tea.KeyPressMsg{Code: 'a'}
	model, cmd := adapter.Update(msg)

	require.Same(t, adapter, model, "Update must return the same adapter instance")
	assert.Nil(t, cmd)
	assert.True(t, child.updated, "inner model's Update must be called")
	assert.Equal(t, msg, child.lastMsg)
}

func TestNewTeaModelAdapter_View(t *testing.T) {
	t.Parallel()
	child := &fakeChild{}
	adapter := NewTeaModelAdapter(child)

	view := adapter.View()
	assert.Equal(t, "fake view", view.Content)
}
