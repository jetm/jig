package commands

// Tests for Task 7.5: config save errors must be displayed in the status bar.
// Each test points XDG_CONFIG_HOME at a nonexistent path so config.Save fails,
// then triggers a panel-ratio key ([) and verifies the status bar shows the error.

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/testhelper"
)

// triggerBracketLeft sends the [ key which decrements panelRatio and calls config.Save.
// Returns the tea.Cmd produced by Update.
func triggerBracketLeft(m interface {
	Update(tea.Msg) tea.Cmd
}) tea.Cmd {
	return m.Update(tea.KeyPressMsg{Code: '[', Text: "["})
}

func TestAddModel_ConfigSaveError_ShowsStatusBarMessage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/no/write")
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "", "main"},
	}
	cfg := config.NewDefault()
	cfg.PanelRatio = 40
	renderer := &diff.PlainRenderer{}
	m, err := NewAddModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewAddModel failed: %v", err)
	}
	m.width = 120
	m.height = 40

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = triggerBracketLeft(m)

	view := m.View()
	if !strings.Contains(view, "Config save failed") {
		t.Errorf("status bar should show 'Config save failed', got: %q", view)
	}
}

func TestCheckoutModel_ConfigSaveError_ShowsStatusBarMessage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/no/write")
	runner := &testhelper.FakeRunner{
		Outputs: []string{"M\tfoo.go\n", "main", ""},
	}
	cfg := config.NewDefault()
	cfg.PanelRatio = 40
	renderer := &diff.PlainRenderer{}
	m, err := NewCheckoutModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewCheckoutModel failed: %v", err)
	}
	m.width = 120
	m.height = 40

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = triggerBracketLeft(m)

	view := m.View()
	if !strings.Contains(view, "Config save failed") {
		t.Errorf("status bar should show 'Config save failed', got: %q", view)
	}
}

func TestHunkAddModel_ConfigSaveError_ShowsStatusBarMessage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/no/write")
	runner := &testhelper.FakeRunner{
		Outputs: []string{singleHunkDiff, "main"},
	}
	cfg := config.NewDefault()
	cfg.PanelRatio = 40
	renderer := &diff.PlainRenderer{}
	m, err := NewHunkAddModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewHunkAddModel failed: %v", err)
	}
	m.width = 120
	m.height = 40

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = triggerBracketLeft(m)

	view := m.View()
	if !strings.Contains(view, "Config save failed") {
		t.Errorf("status bar should show 'Config save failed', got: %q", view)
	}
}
