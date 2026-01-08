package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jetm/gti/internal/config"
)

// mockModel is a simple tea.Model for testing that records calls.
type mockModel struct {
	initCalled   bool
	updateCalled bool
	lastMsg      tea.Msg
	viewContent  string
	updateCmd    tea.Cmd
}

func (m *mockModel) Init() tea.Cmd {
	m.initCalled = true
	return nil
}

func (m *mockModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.updateCalled = true
	m.lastMsg = msg
	return m, m.updateCmd
}

func (m *mockModel) View() tea.View {
	return tea.NewView(m.viewContent)
}

func newMock(content string) *mockModel {
	return &mockModel{viewContent: content}
}

func configZero() config.Config {
	return config.Config{}
}

// TestInit delegates to active model's Init.
func TestInit(t *testing.T) {
	root := newMock("root")
	a := New(root, nil, configZero())
	a.Init()
	if !root.initCalled {
		t.Fatal("expected root.Init() to be called")
	}
}

// TestPushIncreasesDepth verifies Push adds to the stack and Active returns the new model.
func TestPushIncreasesDepth(t *testing.T) {
	root := newMock("root")
	a := New(root, nil, configZero())

	if a.Active() != root {
		t.Fatal("expected root to be active before push")
	}

	child := newMock("child")
	a.Push(child)

	if a.Active() != child {
		t.Fatalf("expected child to be active after push, got %v", a.Active())
	}
	if len(a.stack) != 2 {
		t.Fatalf("expected stack depth 2, got %d", len(a.stack))
	}
}

// TestPopReturnsPreviousModel verifies Pop removes the top and returns the parent.
func TestPopReturnsPreviousModel(t *testing.T) {
	root := newMock("root")
	a := New(root, nil, configZero())
	child := newMock("child")
	a.Push(child)

	prev := a.Pop()

	if prev != root {
		t.Fatalf("expected root after pop, got %v", prev)
	}
	if a.Active() != root {
		t.Fatal("expected root to be active after pop")
	}
	if len(a.stack) != 1 {
		t.Fatalf("expected stack depth 1 after pop, got %d", len(a.stack))
	}
}

// TestPopAtDepthOneReturnsNil verifies Pop returns nil when stack has only one model.
func TestPopAtDepthOneReturnsNil(t *testing.T) {
	root := newMock("root")
	a := New(root, nil, configZero())

	result := a.Pop()

	if result != nil {
		t.Fatalf("expected nil when popping at depth 1, got %v", result)
	}
	if a.Active() != root {
		t.Fatal("expected root to remain active")
	}
}

// TestPushModelMsgPushesAndCallsInit verifies PushModelMsg pushes and inits the new model.
func TestPushModelMsgPushesAndCallsInit(t *testing.T) {
	root := newMock("root")
	a := New(root, nil, configZero())
	child := newMock("child")

	_, _ = a.Update(PushModelMsg{Model: child})

	if a.Active() != child {
		t.Fatal("expected child to be active after PushModelMsg")
	}
	if !child.initCalled {
		t.Fatal("expected child.Init() to be called")
	}
}

// TestPopModelMsgMutatedGitSendsRefreshMsg verifies PopModelMsg{MutatedGit:true} sends RefreshMsg.
func TestPopModelMsgMutatedGitSendsRefreshMsg(t *testing.T) {
	root := newMock("root")
	a := New(root, nil, configZero())
	child := newMock("child")
	a.Push(child)

	_, cmd := a.Update(PopModelMsg{MutatedGit: true})

	if a.Active() != root {
		t.Fatal("expected root to be active after pop")
	}
	if cmd == nil {
		t.Fatal("expected a command to be returned for RefreshMsg")
	}
	msg := cmd()
	if _, ok := msg.(RefreshMsg); !ok {
		t.Fatalf("expected RefreshMsg, got %T", msg)
	}
}

// TestPopModelMsgNoMutationNoRefresh verifies PopModelMsg{MutatedGit:false} returns nil cmd.
func TestPopModelMsgNoMutationNoRefresh(t *testing.T) {
	root := newMock("root")
	a := New(root, nil, configZero())
	child := newMock("child")
	a.Push(child)

	_, cmd := a.Update(PopModelMsg{MutatedGit: false})

	if a.Active() != root {
		t.Fatal("expected root to be active after pop")
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd for non-mutating pop, got non-nil")
	}
}

// TestPopModelMsgAtDepthOneEmitsQuit verifies PopModelMsg at depth 1 returns tea.Quit.
func TestPopModelMsgAtDepthOneEmitsQuit(t *testing.T) {
	root := newMock("root")
	a := New(root, nil, configZero())

	_, cmd := a.Update(PopModelMsg{MutatedGit: false})

	if cmd == nil {
		t.Fatal("expected tea.Quit command, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

// TestWindowSizeMsgForwardedToActive verifies WindowSizeMsg is forwarded to the active model.
func TestWindowSizeMsgForwardedToActive(t *testing.T) {
	root := newMock("root")
	a := New(root, nil, configZero())

	sizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	a.Update(sizeMsg) //nolint:errcheck

	if !root.updateCalled {
		t.Fatal("expected active model's Update to be called with WindowSizeMsg")
	}
	got, ok := root.lastMsg.(tea.WindowSizeMsg)
	if !ok {
		t.Fatalf("expected WindowSizeMsg, got %T", root.lastMsg)
	}
	if got.Width != 80 || got.Height != 24 {
		t.Fatalf("expected 80x24, got %dx%d", got.Width, got.Height)
	}
}

// TestViewReturnsAltScreen verifies View() returns tea.View with AltScreen true.
func TestViewReturnsAltScreen(t *testing.T) {
	root := newMock("hello world")
	a := New(root, nil, configZero())

	v := a.View()

	if !v.AltScreen {
		t.Fatal("expected AltScreen to be true")
	}
	if v.Content != "hello world" {
		t.Fatalf("expected content %q, got %q", "hello world", v.Content)
	}
}
