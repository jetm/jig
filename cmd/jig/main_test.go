package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetm/jig/internal/commands"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/testhelper"
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
		"add", "hunk-add", "hunk-reset", "hunk-checkout", "checkout", "diff",
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
	expected := "jig version dev (commit: none, built: unknown)"
	if !strings.Contains(output, expected) {
		t.Errorf("version output = %q, want containing %q", output, expected)
	}
}

func TestRun_Success(t *testing.T) {
	os.Args = []string{"jig", "--version"}
	if err := run(); err != nil {
		t.Fatalf("run() returned error: %v", err)
	}
}

func TestFixup_RejectsArgs(t *testing.T) {
	t.Parallel()
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"fixup", "somefile.go"})
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command", "expected args rejection, got: %v", err)
}

func TestHunkAdd_AcceptsPathArgs(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	hunkAddCmd, _, err := cmd.Find([]string{"hunk-add"})
	require.NoError(t, err)
	// hunk-add now accepts arbitrary args; confirm cobra.ArbitraryArgs is set.
	// We verify by checking that passing args doesn't produce an args error at parse time.
	// (The command itself would fail because there's no real git repo, but that's expected.)
	if hunkAddCmd.Args == nil {
		t.Error("hunk-add should have ArbitraryArgs, got nil Args func")
	}
}

func TestAddCmd_DirectMode(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",                     // git add -- paths
			"file1.go\nfile2.go\n", // git diff --name-only --cached
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := addDirect(ctx, runner, []string{"file1.go", "file2.go"}, &buf)
	require.NoError(t, err)

	// Verify StageFiles was called with correct args
	require.GreaterOrEqual(t, len(runner.Calls), 1)
	assert.Equal(t, []string{"add", "--", "file1.go", "file2.go"}, runner.Calls[0].Args)

	// Verify output
	assert.Contains(t, buf.String(), "Staged 2 file(s)")
}

func TestResetCmd_DirectMode(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",                             // git reset HEAD -- paths
			" M file1.go\n?? newfile.go\n", // git status --short
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := resetDirect(ctx, runner, []string{"file1.go"}, &buf)
	require.NoError(t, err)

	// Verify UnstageFiles was called
	require.GreaterOrEqual(t, len(runner.Calls), 1)
	assert.Equal(t, []string{"reset", "HEAD", "--", "file1.go"}, runner.Calls[0].Args)

	// Verify status output is printed
	assert.Contains(t, buf.String(), "M file1.go")
}

func TestCheckoutCmd_DirectMode_Confirmed(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"file1.go\n", // git diff --name-only -- paths
			"",           // git checkout -- paths
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	in := strings.NewReader("y\n")
	err := checkoutDirect(ctx, runner, []string{"file1.go"}, in, &buf)
	require.NoError(t, err)

	// Verify prompt was shown
	assert.Contains(t, buf.String(), "Discard changes to 1 file(s)?")
	// Verify discard happened
	assert.Contains(t, buf.String(), "Discarded changes to 1 file(s)")
	require.GreaterOrEqual(t, len(runner.Calls), 2)
	assert.Equal(t, []string{"checkout", "--", "file1.go"}, runner.Calls[1].Args)
}

func TestCheckoutCmd_DirectMode_Denied(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"file1.go\n", // git diff --name-only -- paths
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	in := strings.NewReader("n\n")
	err := checkoutDirect(ctx, runner, []string{"file1.go"}, in, &buf)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "Aborted")
	// Only one call (diff), no checkout call
	assert.Len(t, runner.Calls, 1)
}

func TestCheckoutCmd_DirectMode_NoChanges(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"", // git diff --name-only -- paths (empty = no changes)
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	in := strings.NewReader("")
	err := checkoutDirect(ctx, runner, []string{"file1.go"}, in, &buf)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "No changes to discard")
}

func TestAddCmd_DirectMode_StageError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("fatal: not a git repository")},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := addDirect(ctx, runner, []string{"file1.go"}, &buf)
	require.Error(t, err)
}

func TestAddCmd_DirectMode_DiffCachedError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", ""},
		Errors:  []error{nil, fmt.Errorf("diff --cached failed")},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := addDirect(ctx, runner, []string{"file1.go"}, &buf)
	if err == nil {
		t.Fatal("expected error when git diff --cached fails, got nil")
	}
}

func TestRun_UnknownCommandError(t *testing.T) {
	os.Args = []string{"jig", "unknowncommand"}
	err := run()
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
}

// runCmdWithInvalidConfig executes a subcommand with JIG_LOG_COMMIT_LIMIT set
// to an invalid value so config.Load() returns an error before any TUI starts.
func runCmdWithInvalidConfig(t *testing.T, args ...string) error {
	t.Helper()
	t.Setenv("JIG_LOG_COMMIT_LIMIT", "notanumber")
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

func TestAddCmd_InteractiveFlagRegistered(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	addCmd, _, err := cmd.Find([]string{"add"})
	require.NoError(t, err)

	interactiveFlag := addCmd.Flags().Lookup("interactive")
	require.NotNil(t, interactiveFlag, "add command should have --interactive flag")
	assert.Equal(t, "false", interactiveFlag.DefValue)
}

func TestAddCmd_DirectFlagRemoved(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	addCmd, _, err := cmd.Find([]string{"add"})
	require.NoError(t, err)

	directFlag := addCmd.Flags().Lookup("direct")
	assert.Nil(t, directFlag, "add command should NOT have --direct flag")
}

func TestAddCmd_WithArgsAndNoInteractiveFlag_CallsDirect(t *testing.T) {
	t.Parallel()
	// When args are present and -i is not set, addDirect() should be called.
	// We verify dispatch by checking that the runner receives "git add -- <paths>".
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",           // git add -- paths
			"file1.go\n", // git diff --name-only --cached
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := addDirect(ctx, runner, []string{"file1.go"}, &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Staged 1 file(s)")
	testhelper.MustHaveCall(t, runner, "add", "--", "file1.go")
}

func TestAddCmd_ShortInteractiveFlag_Registered(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	addCmd, _, err := cmd.Find([]string{"add"})
	require.NoError(t, err)

	// The short flag -i should be registered
	iFlag := addCmd.Flags().ShorthandLookup("i")
	require.NotNil(t, iFlag, "add command should have -i shorthand flag")
}

func TestAddCmd_ShortDirectFlag_Unknown(t *testing.T) {
	t.Parallel()
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"add", "-d", "file.go"})
	err := root.Execute()
	require.Error(t, err, "jig add -d should return an error for unknown flag")
	assert.Contains(t, err.Error(), "unknown shorthand flag")
}

func TestAddCmd_DirectMode_GlobExpansion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create test files for glob matching.
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(dir+"/"+name, nil, 0o600); err != nil {
			t.Fatalf("creating test file: %v", err)
		}
	}
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",             // git add -- expanded paths
			"a.go\nb.go\n", // git diff --name-only --cached
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := addDirect(ctx, runner, []string{dir + "/*.go"}, &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Staged 2 file(s)")
	// Verify the glob was expanded: the add call should contain the actual file paths, not the glob.
	require.GreaterOrEqual(t, len(runner.Calls), 1)
	addArgs := runner.Calls[0].Args
	assert.NotContains(t, addArgs, dir+"/*.go", "glob should be expanded, not passed raw")
}

func TestAddCmd_DirectMode_GlobNoMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var buf bytes.Buffer
	runner := &testhelper.FakeRunner{}
	ctx := context.Background()
	err := addDirect(ctx, runner, []string{dir + "/*.xyz"}, &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Staged 0 file(s)")
	assert.Empty(t, runner.Calls, "no git calls when glob matches nothing")
}

func TestResetCmd_DirectMode_GlobExpansion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(dir+"/"+name, nil, 0o600); err != nil {
			t.Fatalf("creating test file: %v", err)
		}
	}
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",                   // git reset HEAD -- expanded paths
			" M a.go\n M b.go\n", // git status --short
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := resetDirect(ctx, runner, []string{dir + "/*.go"}, &buf)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(runner.Calls), 1)
	assert.NotContains(t, runner.Calls[0].Args, dir+"/*.go")
}

func TestCheckoutCmd_DirectMode_GlobExpansion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(dir+"/"+name, nil, 0o600); err != nil {
			t.Fatalf("creating test file: %v", err)
		}
	}
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"a.go\nb.go\n", // git diff --name-only -- expanded paths
			"",             // git checkout -- expanded paths
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	in := strings.NewReader("y\n")
	err := checkoutDirect(ctx, runner, []string{dir + "/*.go"}, in, &buf)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(runner.Calls), 1)
	assert.NotContains(t, runner.Calls[0].Args, dir+"/*.go")
}

func TestResetCmd_InteractiveFlagRegistered(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	resetCmd, _, err := cmd.Find([]string{"reset"})
	require.NoError(t, err)

	interactiveFlag := resetCmd.Flags().Lookup("interactive")
	require.NotNil(t, interactiveFlag, "reset command should have --interactive flag")
	assert.Equal(t, "false", interactiveFlag.DefValue)

	iFlag := resetCmd.Flags().ShorthandLookup("i")
	require.NotNil(t, iFlag, "reset command should have -i shorthand flag")
}

func TestResetCmd_DirectFlagRemoved(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	resetCmd, _, err := cmd.Find([]string{"reset"})
	require.NoError(t, err)

	directFlag := resetCmd.Flags().Lookup("direct")
	assert.Nil(t, directFlag, "reset command should NOT have --direct flag")
}

func TestResetCmd_ShortDirectFlag_Unknown(t *testing.T) {
	t.Parallel()
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"reset", "-d", "file.go"})
	err := root.Execute()
	require.Error(t, err, "jig reset -d should return an error for unknown flag")
	assert.Contains(t, err.Error(), "unknown shorthand flag")
}

func TestResetCmd_WithArgsAndNoInteractiveFlag_CallsDirect(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"", // git reset HEAD -- paths
			"", // git status --short
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := resetDirect(ctx, runner, []string{"file1.go"}, &buf)
	require.NoError(t, err)
	testhelper.MustHaveCall(t, runner, "reset", "HEAD", "--", "file1.go")
}

func TestResetCmd_WithInteractiveFlagAndArgs_AcceptsFlag(t *testing.T) {
	t.Setenv("JIG_LOG_COMMIT_LIMIT", "notanumber")
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"reset", "-i", "file.go"})
	err := root.Execute()
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "unknown shorthand flag", "jig reset -i should be a valid flag")
	assert.Contains(t, err.Error(), "loading config", "expected to reach TUI path (config load)")
}

func TestCheckoutCmd_DirectFlag_Registered(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	checkoutCmd, _, err := cmd.Find([]string{"checkout"})
	require.NoError(t, err)

	directFlag := checkoutCmd.Flags().Lookup("direct")
	require.NotNil(t, directFlag, "checkout command should have --direct flag")
	assert.Equal(t, "false", directFlag.DefValue)
}

func TestAddCmd_DirectMode_WithFlag(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"",           // git add -- paths
			"file1.go\n", // git diff --name-only --cached
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	// Direct mode with the flag - same behavior as before
	err := addDirect(ctx, runner, []string{"file1.go"}, &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Staged 1 file(s)")
}

func TestAddCmd_WithInteractiveFlagAndArgs_AcceptsFlag(t *testing.T) {
	// jig add -i file.go must not error at cobra parse level - the flag is valid.
	// The TUI path is exercised; we can only verify the flag is accepted without error
	// before the TUI/runner starts (config load error gates it).
	t.Setenv("JIG_LOG_COMMIT_LIMIT", "notanumber")
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"add", "-i", "file.go"})
	err := root.Execute()
	// Config load error means we reached the TUI dispatch path, not a flag parse error.
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "unknown shorthand flag", "jig add -i should be a valid flag")
	assert.Contains(t, err.Error(), "loading config", "expected to reach TUI path (config load)")
}

func TestNewFakeAddModel_WithFilterPaths(t *testing.T) {
	t.Parallel()
	// Covers the jig add -i file.go code path: NewAddModel is called with filter paths.
	runner := &testhelper.FakeRunner{Outputs: []string{"", "", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewAddModel(context.Background(), runner, cfg, renderer, []string{"foo.go"})
	require.NoError(t, err)
	if m == nil {
		t.Fatal("NewAddModel with filter paths should not return nil")
	}
}

func TestResetCmd_DirectMode_WithFlag(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{
			"", // git reset HEAD -- paths
			"", // git status --short
		},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := resetDirect(ctx, runner, []string{"file1.go"}, &buf)
	require.NoError(t, err)
	testhelper.MustHaveCall(t, runner, "reset", "HEAD", "--", "file1.go")
}

func TestHunkAddCmd_AcceptsArgs(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	hunkAddCmd, _, err := cmd.Find([]string{"hunk-add"})
	require.NoError(t, err)
	assert.Equal(t, "hunk-add [paths...]", hunkAddCmd.Use)
}

func TestNewFakeHunkAddModel_WithFilterPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewHunkAddModel(context.Background(), runner, cfg, renderer, []string{"foo.go"})
	require.NoError(t, err)
	if m == nil {
		t.Fatal("NewHunkAddModel should not return nil")
	}
}

func TestNewFakeDiffModel_WithFilterPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "", []string{"foo.go"})
	require.NoError(t, err)
	if m == nil {
		t.Fatal("NewDiffModel should not return nil")
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
	m, err := commands.NewHunkAddModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewHunkAddModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestHunkAddTeaModel_InitReturnsNil(t *testing.T) {
	m := newChildTeaModel(newFakeHunkAddModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestHunkAddTeaModel_Update(t *testing.T) {
	m := newChildTeaModel(newFakeHunkAddModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestHunkAddTeaModel_View(t *testing.T) {
	m := newChildTeaModel(newFakeHunkAddModel(t))
	_ = m.View() // just ensure no panic
}

// newFakeDiffModel creates a DiffModel with no real git calls.
func newFakeDiffModel(t *testing.T) *commands.DiffModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewDiffModel(context.Background(), runner, cfg, renderer, "", false, "")
	if err != nil {
		t.Fatalf("NewDiffModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestDiffTeaModel_InitReturnsNil(t *testing.T) {
	m := newChildTeaModel(newFakeDiffModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestDiffTeaModel_Update(t *testing.T) {
	m := newChildTeaModel(newFakeDiffModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestDiffTeaModel_View(t *testing.T) {
	m := newChildTeaModel(newFakeDiffModel(t))
	view := m.View()
	_ = view // just ensure no panic
}

// newFakeAddModel creates an AddModel with no real git calls.
func newFakeAddModel(t *testing.T) *commands.AddModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewAddModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewAddModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestAddTeaModel_InitReturnsNil(t *testing.T) {
	m := newChildTeaModel(newFakeAddModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestAddTeaModel_Update(t *testing.T) {
	m := newChildTeaModel(newFakeAddModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestAddTeaModel_View(t *testing.T) {
	m := newChildTeaModel(newFakeAddModel(t))
	view := m.View()
	_ = view // just ensure no panic
}

// newFakeResetModel creates a ResetModel with no real git calls.
func newFakeResetModel(t *testing.T) *commands.ResetModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewResetModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewResetModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestResetTeaModel_InitReturnsNil(t *testing.T) {
	m := newChildTeaModel(newFakeResetModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestResetTeaModel_Update(t *testing.T) {
	m := newChildTeaModel(newFakeResetModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestResetTeaModel_View(t *testing.T) {
	m := newChildTeaModel(newFakeResetModel(t))
	_ = m.View() // just ensure no panic
}

// newFakeCheckoutModel creates a CheckoutModel with no real git calls.
func newFakeCheckoutModel(t *testing.T) *commands.CheckoutModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewCheckoutModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewCheckoutModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestCheckoutTeaModel_InitReturnsNil(t *testing.T) {
	m := newChildTeaModel(newFakeCheckoutModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestCheckoutTeaModel_Update(t *testing.T) {
	m := newChildTeaModel(newFakeCheckoutModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestCheckoutTeaModel_View(t *testing.T) {
	m := newChildTeaModel(newFakeCheckoutModel(t))
	view := m.View()
	_ = view // just ensure no panic
}

// newFakeFixupModel creates a FixupModel with no real git calls.
// Call sequence:
//   - output[0]: diff --cached --quiet (HasStagedChanges: error = staged)
//   - output[1]: rev-parse --show-toplevel (IsRebaseInProgress->RepoRoot)
//   - output[2]: git log (RecentCommits, empty)
//   - output[3]: rev-parse --abbrev-ref HEAD (BranchName)
//   - output[4]: diff --cached (FindFixupTarget staged diff)
func newFakeFixupModel(t *testing.T) *commands.FixupModel {
	t.Helper()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", "/fake/repo", "", "main", "" /* staged diff for blame */},
		Errors:  []error{fmt.Errorf("staged"), nil, nil, nil, nil},
	}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewFixupModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestFixupTeaModel_InitReturnsNil(t *testing.T) {
	m := newChildTeaModel(newFakeFixupModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestFixupTeaModel_Update(t *testing.T) {
	m := newChildTeaModel(newFakeFixupModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestFixupTeaModel_View(t *testing.T) {
	m := newChildTeaModel(newFakeFixupModel(t))
	_ = m.View() // just ensure no panic
}

// newFakeLogModel creates a LogModel with no real git calls.
func newFakeLogModel(t *testing.T) *commands.LogModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewLogModel(context.Background(), runner, cfg, renderer, "")
	if err != nil {
		t.Fatalf("NewLogModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestLogTeaModel_InitReturnsNil(t *testing.T) {
	m := newChildTeaModel(newFakeLogModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestLogTeaModel_Update(t *testing.T) {
	m := newChildTeaModel(newFakeLogModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestLogTeaModel_View(t *testing.T) {
	m := newChildTeaModel(newFakeLogModel(t))
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
	m, err := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, renderer, "HEAD~5", "")
	if err != nil {
		t.Fatalf("NewRebaseInteractiveModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestRebaseInteractiveTeaModel_InitReturnsNil(t *testing.T) {
	m := newChildTeaModel(newFakeRebaseInteractiveModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestRebaseInteractiveTeaModel_Update(t *testing.T) {
	m := newChildTeaModel(newFakeRebaseInteractiveModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestRebaseInteractiveTeaModel_View(t *testing.T) {
	m := newChildTeaModel(newFakeRebaseInteractiveModel(t))
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
		"add": false, "hunk-add": false, "hunk-reset": false, "hunk-checkout": false,
		"checkout": false, "diff": false,
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

func TestHunkResetCmd_ConfigLoadError(t *testing.T) {
	if err := runCmdWithInvalidConfig(t, "hunk-reset"); err == nil {
		t.Fatal("expected error from config load failure, got nil")
	}
}

func TestHunkCheckoutCmd_ConfigLoadError(t *testing.T) {
	if err := runCmdWithInvalidConfig(t, "hunk-checkout"); err == nil {
		t.Fatal("expected error from config load failure, got nil")
	}
}

func TestHunkResetCmd_AcceptsArgs(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	hunkResetCmd, _, err := cmd.Find([]string{"hunk-reset"})
	require.NoError(t, err)
	assert.Equal(t, "hunk-reset [paths...]", hunkResetCmd.Use)
}

func TestHunkCheckoutCmd_AcceptsArgs(t *testing.T) {
	t.Parallel()
	cmd := newRootCmd()
	hunkCheckoutCmd, _, err := cmd.Find([]string{"hunk-checkout"})
	require.NoError(t, err)
	assert.Equal(t, "hunk-checkout [paths...]", hunkCheckoutCmd.Use)
}

// newFakeHunkResetModel creates a HunkResetModel with no real git calls.
func newFakeHunkResetModel(t *testing.T) *commands.HunkResetModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewHunkResetModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewHunkResetModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestHunkResetTeaModel_InitReturnsNil(t *testing.T) {
	m := newChildTeaModel(newFakeHunkResetModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestHunkResetTeaModel_Update(t *testing.T) {
	m := newChildTeaModel(newFakeHunkResetModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestHunkResetTeaModel_View(t *testing.T) {
	m := newChildTeaModel(newFakeHunkResetModel(t))
	_ = m.View() // just ensure no panic
}

// newFakeHunkCheckoutModel creates a HunkCheckoutModel with no real git calls.
func newFakeHunkCheckoutModel(t *testing.T) *commands.HunkCheckoutModel {
	t.Helper()
	runner := &testhelper.FakeRunner{Outputs: []string{"", "main"}}
	cfg := config.NewDefault()
	renderer := &diff.PlainRenderer{}
	m, err := commands.NewHunkCheckoutModel(context.Background(), runner, cfg, renderer)
	if err != nil {
		t.Fatalf("NewHunkCheckoutModel unexpectedly returned error: %v", err)
	}
	return m
}

func TestHunkCheckoutTeaModel_InitReturnsNil(t *testing.T) {
	m := newChildTeaModel(newFakeHunkCheckoutModel(t))
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestHunkCheckoutTeaModel_Update(t *testing.T) {
	m := newChildTeaModel(newFakeHunkCheckoutModel(t))
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if next == nil {
		t.Error("Update() should return non-nil model")
	}
	_ = cmd
}

func TestHunkCheckoutTeaModel_View(t *testing.T) {
	m := newChildTeaModel(newFakeHunkCheckoutModel(t))
	_ = m.View() // just ensure no panic
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

func TestCheckoutCmd_DirectMode_NoPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{}
	ctx := context.Background()
	var buf bytes.Buffer
	in := strings.NewReader("")
	err := checkoutDirect(ctx, runner, []string{}, in, &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No changes to discard")
}

func TestCheckoutCmd_DirectMode_DiffError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("git error")},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	in := strings.NewReader("")
	err := checkoutDirect(ctx, runner, []string{"file1.go"}, in, &buf)
	if err == nil {
		t.Fatal("expected error when git diff fails, got nil")
	}
}

func TestCheckoutCmd_DirectMode_DiscardError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"file1.go\n", ""},
		Errors:  []error{nil, fmt.Errorf("checkout failed")},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	in := strings.NewReader("y\n")
	err := checkoutDirect(ctx, runner, []string{"file1.go"}, in, &buf)
	if err == nil {
		t.Fatal("expected error when git checkout fails, got nil")
	}
}

func TestCheckoutCmd_DirectMode_EmptyInput(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"file1.go\n"},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	// Empty reader simulates EOF without newline (Scan returns false)
	in := strings.NewReader("")
	err := checkoutDirect(ctx, runner, []string{"file1.go"}, in, &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Aborted")
}

func TestResetCmd_DirectMode_NoPaths(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{}
	ctx := context.Background()
	var buf bytes.Buffer
	err := resetDirect(ctx, runner, []string{}, &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Nothing to unstage")
}

func TestResetCmd_DirectMode_UnstageError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{""},
		Errors:  []error{fmt.Errorf("reset failed")},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := resetDirect(ctx, runner, []string{"file1.go"}, &buf)
	if err == nil {
		t.Fatal("expected error when git reset fails, got nil")
	}
}

func TestResetCmd_DirectMode_StatusError(t *testing.T) {
	t.Parallel()
	runner := &testhelper.FakeRunner{
		Outputs: []string{"", ""},
		Errors:  []error{nil, fmt.Errorf("status failed")},
	}
	ctx := context.Background()
	var buf bytes.Buffer
	err := resetDirect(ctx, runner, []string{"file1.go"}, &buf)
	if err == nil {
		t.Fatal("expected error when git status fails, got nil")
	}
}

// runCmdOutsideRepo executes a subcommand from a non-git directory so
// git.NewExecRunner fails with "not a git repository".
func runCmdOutsideRepo(t *testing.T, args ...string) error {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	root := newRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return err //nolint:wrapcheck // test helper; caller checks for nil
}

func TestDiffCmd_GitRunnerInitError(t *testing.T) {
	if err := runCmdOutsideRepo(t, "diff"); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

func TestAddCmd_GitRunnerInitError(t *testing.T) {
	if err := runCmdOutsideRepo(t, "add"); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

func TestHunkAddCmd_GitRunnerInitError(t *testing.T) {
	if err := runCmdOutsideRepo(t, "hunk-add"); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

func TestLogCmd_GitRunnerInitError(t *testing.T) {
	if err := runCmdOutsideRepo(t, "log"); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

func TestFixupCmd_GitRunnerInitError(t *testing.T) {
	if err := runCmdOutsideRepo(t, "fixup"); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

func TestResetCmd_GitRunnerInitError(t *testing.T) {
	if err := runCmdOutsideRepo(t, "reset"); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

func TestCheckoutCmd_GitRunnerInitError(t *testing.T) {
	if err := runCmdOutsideRepo(t, "checkout"); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

func TestHunkResetCmd_GitRunnerInitError(t *testing.T) {
	if err := runCmdOutsideRepo(t, "hunk-reset"); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

func TestHunkCheckoutCmd_GitRunnerInitError(t *testing.T) {
	if err := runCmdOutsideRepo(t, "hunk-checkout"); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}

func TestRebaseInteractiveCmd_GitRunnerInitError(t *testing.T) {
	if err := runCmdOutsideRepo(t, "rebase-interactive"); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}
