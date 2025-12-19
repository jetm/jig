package main

import (
	"bytes"
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
	cmd.SetArgs([]string{"add"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := errBuf.String()
	if !strings.Contains(output, "not implemented") {
		t.Errorf("stderr = %q, want containing %q", output, "not implemented")
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
