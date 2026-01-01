package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/gti/internal/commands"
	"github.com/jetm/gti/internal/config"
	"github.com/jetm/gti/internal/diff"
	"github.com/jetm/gti/internal/testhelper"
)

func TestRootCommand_ShowsHelp(t *testing.T) {
	cmd := newRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})
	_ = cmd.Execute()

	output := buf.String()
	// Verify all 8 subcommands appear in help
	for _, sub := range []string{
		"add", "hunk-add", "checkout", "diff",
		"fixup", "rebase-interactive", "reset", "log",
	} {
		if !strings.Contains(output, sub) {
			t.Errorf("help output missing subcommand %q", sub)
		}
	}
}

func TestVersionFlag(t *testing.T) {
	cmd := newRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--version"})
	_ = cmd.Execute()

	output := buf.String()
	expected := "gti version dev (commit: none, built: unknown)"
	if !strings.Contains(output, expected) {
		t.Errorf("version output = %q, want containing %q", output, expected)
	}
}

func TestSubcommandStub_PrintsNotImplemented(t *testing.T) {
	cmd := newRootCmd()
	errBuf := new(bytes.Buffer)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"rebase-interactive"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := errBuf.String()
	if !strings.Contains(output, "not implemented") {
		t.Errorf("stderr = %q, want containing %q", output, "not implemented")
	}
}

func TestRun_Success(t *testing.T) {
	os.Args = []string{"gti", "--version"}
	if err := run(); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
}

func TestRun_EachSubcommand(t *testing.T) {
	// add, checkout, diff, hunk-add, fixup are excluded because they launch a real TUI (requires TTY).
	for _, name := range []string{
		"rebase-interactive", "reset", "log",
	} {
		t.Run(name, func(t *testing.T) {
			os.Args = []string{"gti", name}
			if err := run(); err != nil {
				t.Fatalf("run(%s) returned error: %v", name, err)
			}
		})
	}
}

func TestDiffCmd_RegisteredWithFlags(t *testing.T) {
	cmd := newRootCmd()
	diffCmd, _, err := cmd.Find([]string{"diff"})
	if err != nil {
		t.Fatalf("diff subcommand not found: %v", err)
	}
	if diffCmd.Use != "diff [revision]" {
		t.Errorf("diff Use = %q, want %q", diffCmd.Use, "diff [revision]")
	}

	staged := diffCmd.Flags().Lookup("staged")
	if staged == nil {
		t.Fatal("diff command missing --staged flag")
	}
	if staged.DefValue != "false" {
		t.Errorf("--staged default = %q, want %q", staged.DefValue, "false")
	}
}

// newFakeHunkAddModel creates a HunkAddModel with no real git calls.
func newFakeHunkAddModel(t *testing.T) *commands.HunkAddModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewHunkAddModel(context.Background(), runner, cfg, renderer)
}

func TestHunkAddTeaModel_InitReturnsNil(t *testing.T) {
	m := newHunkAddTeaModel(newFakeHunkAddModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestHunkAddTeaModel_Update(t *testing.T) {
	m := newHunkAddTeaModel(newFakeHunkAddModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestHunkAddTeaModel_View(t *testing.T) {
	m := newHunkAddTeaModel(newFakeHunkAddModel(t))
	_ = m.View() // just ensure no panic
}

// newFakeDiffModel creates a DiffModel with no real git calls.
func newFakeDiffModel(t *testing.T) *commands.DiffModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewDiffModel(context.Background(), runner, cfg, renderer, "", false)
}

func TestDiffTeaModel_InitReturnsNil(t *testing.T) {
	m := newDiffTeaModel(newFakeDiffModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestDiffTeaModel_Update(t *testing.T) {
	m := newDiffTeaModel(newFakeDiffModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestDiffTeaModel_View(t *testing.T) {
	m := newDiffTeaModel(newFakeDiffModel(t))
	view := m.View()
	_ = view // just ensure no panic
}

// newFakeAddModel creates an AddModel with no real git calls.
func newFakeAddModel(t *testing.T) *commands.AddModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewAddModel(context.Background(), runner, cfg, renderer)
}

func TestAddTeaModel_InitReturnsNil(t *testing.T) {
	m := newAddTeaModel(newFakeAddModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestAddTeaModel_Update(t *testing.T) {
	m := newAddTeaModel(newFakeAddModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestAddTeaModel_View(t *testing.T) {
	m := newAddTeaModel(newFakeAddModel(t))
	view := m.View()
	_ = view // just ensure no panic
}

// newFakeCheckoutModel creates a CheckoutModel with no real git calls.
func newFakeCheckoutModel(t *testing.T) *commands.CheckoutModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewCheckoutModel(context.Background(), runner, cfg, renderer)
}

func TestCheckoutTeaModel_InitReturnsNil(t *testing.T) {
	m := newCheckoutTeaModel(newFakeCheckoutModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestCheckoutTeaModel_Update(t *testing.T) {
	m := newCheckoutTeaModel(newFakeCheckoutModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestCheckoutTeaModel_View(t *testing.T) {
	m := newCheckoutTeaModel(newFakeCheckoutModel(t))
	view := m.View()
	_ = view // just ensure no panic
}

// newFakeFixupModel creates a FixupModel with no real git calls.
func newFakeFixupModel(t *testing.T) *commands.FixupModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewFixupModel(context.Background(), runner, cfg, renderer)
}

func TestFixupTeaModel_InitReturnsNil(t *testing.T) {
	m := newFixupTeaModel(newFakeFixupModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestFixupTeaModel_Update(t *testing.T) {
	m := newFixupTeaModel(newFakeFixupModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestFixupTeaModel_View(t *testing.T) {
	m := newFixupTeaModel(newFakeFixupModel(t))
	_ = m.View() // just ensure no panic
}

func TestAllSubcommandsRegistered(t *testing.T) {
	cmd := newRootCmd()
	expected := map[string]bool{
		"add": false, "hunk-add": false, "checkout": false, "diff": false,
		"fixup": false, "rebase-interactive": false, "reset": false, "log": false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("subcommand %q not registered", name)
		}
	}
}
