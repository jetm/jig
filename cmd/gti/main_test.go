package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
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
	cmd.SetArgs([]string{"hunk-add"})
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
	// add, checkout, diff are excluded because they launch a real TUI (requires TTY).
	for _, name := range []string{
		"hunk-add",
		"fixup", "rebase-interactive", "reset", "log",
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
