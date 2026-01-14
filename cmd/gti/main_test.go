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

func TestRun_Success(t *testing.T) {
	os.Args = []string{"gti", "--version"}
	if err := run(); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
}

func TestRun_UnknownCommandError(t *testing.T) {
	os.Args = []string{"gti", "unknowncommand"}
	err := run()
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
}

// runCmdWithInvalidConfig executes a subcommand with GTI_LOG_COMMIT_LIMIT set
// to an invalid value so config.Load() returns an error before any TUI starts.
func runCmdWithInvalidConfig(t *testing.T, args ...string) error {
	t.Helper()
	t.Setenv("GTI_LOG_COMMIT_LIMIT", "notanumber")
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return err //nolint:wrapcheck // test helper; caller checks for nil
}

func TestDiffCmd_ConfigLoadError(t *testing.T) {
	if err := runCmdWithInvalidConfig(t, "diff"); err == nil {
		t.Fatal("expected error from config load failure, got nil")
	}
}

func TestAddCmd_ConfigLoadError(t *testing.T) {
	if err := runCmdWithInvalidConfig(t, "add"); err == nil {
		t.Fatal("expected error from config load failure, got nil")
	}
}

func TestResetCmd_ConfigLoadError(t *testing.T) {
	if err := runCmdWithInvalidConfig(t, "reset"); err == nil {
		t.Fatal("expected error from config load failure, got nil")
	}
}

func TestCheckoutCmd_ConfigLoadError(t *testing.T) {
	if err := runCmdWithInvalidConfig(t, "checkout"); err == nil {
		t.Fatal("expected error from config load failure, got nil")
	}
}

func TestHunkAddCmd_ConfigLoadError(t *testing.T) {
	if err := runCmdWithInvalidConfig(t, "hunk-add"); err == nil {
		t.Fatal("expected error from config load failure, got nil")
	}
}

func TestFixupCmd_ConfigLoadError(t *testing.T) {
	if err := runCmdWithInvalidConfig(t, "fixup"); err == nil {
		t.Fatal("expected error from config load failure, got nil")
	}
}

func TestLogCmd_ConfigLoadError(t *testing.T) {
	if err := runCmdWithInvalidConfig(t, "log"); err == nil {
		t.Fatal("expected error from config load failure, got nil")
	}
}

func TestRebaseInteractiveCmd_ConfigLoadError(t *testing.T) {
	if err := runCmdWithInvalidConfig(t, "rebase-interactive"); err == nil {
		t.Fatal("expected error from config load failure, got nil")
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

// newFakeResetModel creates a ResetModel with no real git calls.
func newFakeResetModel(t *testing.T) *commands.ResetModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewResetModel(context.Background(), runner, cfg, renderer)
}

func TestResetTeaModel_InitReturnsNil(t *testing.T) {
	m := newResetTeaModel(newFakeResetModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestResetTeaModel_Update(t *testing.T) {
	m := newResetTeaModel(newFakeResetModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestResetTeaModel_View(t *testing.T) {
	m := newResetTeaModel(newFakeResetModel(t))
	_ = m.View() // just ensure no panic
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

// newFakeLogModel creates a LogModel with no real git calls.
func newFakeLogModel(t *testing.T) *commands.LogModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewLogModel(context.Background(), runner, cfg, renderer, "")
}

func TestLogTeaModel_InitReturnsNil(t *testing.T) {
	m := newLogTeaModel(newFakeLogModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestLogTeaModel_Update(t *testing.T) {
	m := newLogTeaModel(newFakeLogModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestLogTeaModel_View(t *testing.T) {
	m := newLogTeaModel(newFakeLogModel(t))
	_ = m.View() // just ensure no panic
}

func TestLogCmd_RegisteredWithArgs(t *testing.T) {
	cmd := newRootCmd()
	logCmd, _, err := cmd.Find([]string{"log"})
	if err != nil {
		t.Fatalf("log subcommand not found: %v", err)
	}
	if logCmd.Use != "log [revision]" {
		t.Errorf("log Use = %q, want %q", logCmd.Use, "log [revision]")
	}
}

// newFakeRebaseInteractiveModel creates a RebaseInteractiveModel with no real git calls.
func newFakeRebaseInteractiveModel(t *testing.T) *commands.RebaseInteractiveModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	return commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~5", "")
}

func TestRebaseInteractiveTeaModel_InitReturnsNil(t *testing.T) {
	m := newRebaseInteractiveTeaModel(newFakeRebaseInteractiveModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestRebaseInteractiveTeaModel_Update(t *testing.T) {
	m := newRebaseInteractiveTeaModel(newFakeRebaseInteractiveModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestRebaseInteractiveTeaModel_View(t *testing.T) {
	m := newRebaseInteractiveTeaModel(newFakeRebaseInteractiveModel(t))
	_ = m.View() // just ensure no panic
}

func TestRebaseInteractiveCmd_RegisteredWithArgs(t *testing.T) {
	cmd := newRootCmd()
	riCmd, _, err := cmd.Find([]string{"rebase-interactive"})
	if err != nil {
		t.Fatalf("rebase-interactive subcommand not found: %v", err)
	}
	if riCmd.Use != "rebase-interactive [revision]" {
		t.Errorf("rebase-interactive Use = %q, want %q", riCmd.Use, "rebase-interactive [revision]")
	}
}

func TestAllSubcommandsRegistered(t *testing.T) {
	cmd := newRootCmd()
	expected := map[string]bool{
		"add": false, "hunk-add": false, "checkout": false, "diff": false,
		"fixup": false, "rebase-interactive": false, "reset": false, "log": false,
		"completion": false,
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

func TestCompletionCmd_BashOutput(t *testing.T) {
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion bash error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty bash completion output")
	}
}

func TestCompletionCmd_ZshOutput(t *testing.T) {
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion", "zsh"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion zsh error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty zsh completion output")
	}
}

func TestCompletionCmd_FishOutput(t *testing.T) {
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion", "fish"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion fish error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty fish completion output")
	}
}

func TestCompletionCmd_PowershellOutput(t *testing.T) {
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion", "powershell"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion powershell error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty powershell completion output")
	}
}

func TestCompletionCmd_UnknownShellErrors(t *testing.T) {
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion", "unknownshell"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown shell, got nil")
	}
}
