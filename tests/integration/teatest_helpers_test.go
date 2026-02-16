//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jetm/jig/internal/app"
	"github.com/jetm/jig/internal/commands"
	"github.com/jetm/jig/internal/config"
	"github.com/jetm/jig/internal/diff"
	"github.com/jetm/jig/internal/git"
)

// cwdMu serializes os.Chdir calls during runner construction.
var cwdMu sync.Mutex

// testModel wraps a tea.Program with virtual I/O for in-process TUI testing.
// This is a minimal reimplementation of teatest v2's API, needed because
// the upstream teatest module uses github.com/charmbracelet/bubbletea/v2
// while this project uses charm.land/bubbletea/v2 (incompatible types).
type testModel struct {
	program *tea.Program
	out     *safeBuffer
	modelCh chan tea.Model
	doneCh  chan struct{}
	done    sync.Once
}

// newTestModel creates a tea.Program with virtual I/O and starts it in a goroutine.
func newTestModel(tb testing.TB, m tea.Model) *testModel {
	tb.Helper()

	tm := &testModel{
		out:     &safeBuffer{},
		modelCh: make(chan tea.Model, 1),
		doneCh:  make(chan struct{}, 1),
	}

	tm.program = tea.NewProgram(m,
		tea.WithInput(bytes.NewReader(nil)),
		tea.WithOutput(tm.out),
		tea.WithoutSignals(),
		tea.WithWindowSize(120, 40),
	)

	go func() {
		finalModel, err := tm.program.Run()
		if err != nil {
			tb.Errorf("tea.Program exited with error: %v", err)
		}
		tm.modelCh <- finalModel
		close(tm.doneCh)
	}()

	// Send initial window size so models can render.
	tm.program.Send(tea.WindowSizeMsg{Width: 120, Height: 40})

	return tm
}

// send sends a message to the running program.
func (tm *testModel) send(msg tea.Msg) {
	tm.program.Send(msg)
}

// waitFor reads from the output buffer until condition returns true or timeout.
func (tm *testModel) waitFor(tb testing.TB, condition func(string) bool, timeout ...time.Duration) {
	tb.Helper()
	dur := 5 * time.Second
	if len(timeout) > 0 {
		dur = timeout[0]
	}

	deadline := time.Now().Add(dur)
	for time.Now().Before(deadline) {
		if condition(tm.out.String()) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	tb.Fatalf("waitFor: condition not met after %s.\nLast output (%d bytes):\n%s",
		dur, tm.out.Len(), tm.out.String())
}

// quit sends a quit message and waits for the program to finish.
func (tm *testModel) quit(tb testing.TB) {
	tb.Helper()
	tm.program.Quit()
	tm.waitDone(tb)
}

// waitDone waits for the program to finish with a timeout.
func (tm *testModel) waitDone(tb testing.TB) {
	tb.Helper()
	select {
	case <-tm.doneCh:
	case <-time.After(5 * time.Second):
		tb.Fatal("testModel: program did not finish within timeout")
	}
}

// safeBuffer is a concurrency-safe bytes.Buffer.
type safeBuffer struct {
	mu  sync.RWMutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buf.String()
}

func (s *safeBuffer) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buf.Len()
}

// --- Keystroke helpers ---

func keyPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

func specialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func sendSpace(tm *testModel)       { tm.send(specialKey(tea.KeySpace)) }
func sendEnter(tm *testModel)       { tm.send(specialKey(tea.KeyEnter)) }
func sendTab(tm *testModel)         { tm.send(specialKey(tea.KeyTab)) }
func sendKey(tm *testModel, c rune) { tm.send(keyPress(c)) }

// --- tea.Model adapters ---
// These duplicate the unexported adapters in cmd/jig/main.go.

type addTeaModelAdapter struct{ inner *commands.AddModel }

func (a *addTeaModelAdapter) Init() tea.Cmd                           { return nil }
func (a *addTeaModelAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return a, a.inner.Update(msg) }
func (a *addTeaModelAdapter) View() tea.View                          { return tea.NewView(a.inner.View()) }

type checkoutTeaModelAdapter struct{ inner *commands.CheckoutModel }

func (c *checkoutTeaModelAdapter) Init() tea.Cmd { return nil }
func (c *checkoutTeaModelAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return c, c.inner.Update(msg)
}
func (c *checkoutTeaModelAdapter) View() tea.View { return tea.NewView(c.inner.View()) }

type diffTeaModelAdapter struct{ inner *commands.DiffModel }

func (d *diffTeaModelAdapter) Init() tea.Cmd                           { return nil }
func (d *diffTeaModelAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return d, d.inner.Update(msg) }
func (d *diffTeaModelAdapter) View() tea.View                          { return tea.NewView(d.inner.View()) }

type fixupTeaModelAdapter struct{ inner *commands.FixupModel }

func (f *fixupTeaModelAdapter) Init() tea.Cmd { return nil }
func (f *fixupTeaModelAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return f, f.inner.Update(msg)
}
func (f *fixupTeaModelAdapter) View() tea.View { return tea.NewView(f.inner.View()) }

type hunkAddTeaModelAdapter struct{ inner *commands.HunkAddModel }

func (h *hunkAddTeaModelAdapter) Init() tea.Cmd { return nil }
func (h *hunkAddTeaModelAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return h, h.inner.Update(msg)
}
func (h *hunkAddTeaModelAdapter) View() tea.View { return tea.NewView(h.inner.View()) }

type logTeaModelAdapter struct{ inner *commands.LogModel }

func (l *logTeaModelAdapter) Init() tea.Cmd                           { return nil }
func (l *logTeaModelAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return l, l.inner.Update(msg) }
func (l *logTeaModelAdapter) View() tea.View                          { return tea.NewView(l.inner.View()) }

type rebaseInteractiveTeaModelAdapter struct {
	inner *commands.RebaseInteractiveModel
}

func (r *rebaseInteractiveTeaModelAdapter) Init() tea.Cmd { return nil }
func (r *rebaseInteractiveTeaModelAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return r, r.inner.Update(msg)
}
func (r *rebaseInteractiveTeaModelAdapter) View() tea.View {
	return tea.NewView(r.inner.View())
}

type resetTeaModelAdapter struct{ inner *commands.ResetModel }

func (r *resetTeaModelAdapter) Init() tea.Cmd { return nil }
func (r *resetTeaModelAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return r, r.inner.Update(msg)
}
func (r *resetTeaModelAdapter) View() tea.View { return tea.NewView(r.inner.View()) }

// --- Model construction helpers ---

// newRunnerInDir creates an ExecRunner rooted at the given directory.
// It holds cwdMu during os.Chdir to avoid races with parallel tests.
func newRunnerInDir(tb testing.TB, dir string) git.Runner {
	tb.Helper()

	cwdMu.Lock()
	defer cwdMu.Unlock()

	orig, err := os.Getwd()
	if err != nil {
		tb.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		tb.Fatalf("chdir to %s: %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			tb.Fatalf("chdir restore: %v", err)
		}
	}()

	runner, err := git.NewExecRunner(context.Background())
	if err != nil {
		tb.Fatalf("NewExecRunner: %v", err)
	}
	return runner
}

func defaultConfig() config.Config {
	cfg, _ := config.Load()
	return cfg
}

func plainRenderer() diff.Renderer {
	return &diff.PlainRenderer{}
}

// newAddTestModel creates an in-process add TUI model for a temp repo.
func newAddTestModel(tb testing.TB, repoDir string) *testModel {
	tb.Helper()
	runner := newRunnerInDir(tb, repoDir)
	cfg := defaultConfig()
	m := commands.NewAddModel(context.Background(), runner, cfg, plainRenderer())
	appModel := app.New(&addTeaModelAdapter{inner: m}, runner, cfg)
	return newTestModel(tb, appModel)
}

// newAddTestModelFiltered creates an in-process add TUI model with path filters.
func newAddTestModelFiltered(tb testing.TB, repoDir string, filterPaths []string) *testModel {
	tb.Helper()
	runner := newRunnerInDir(tb, repoDir)
	cfg := defaultConfig()
	m := commands.NewAddModel(context.Background(), runner, cfg, plainRenderer(), filterPaths)
	appModel := app.New(&addTeaModelAdapter{inner: m}, runner, cfg)
	return newTestModel(tb, appModel)
}

// newCheckoutTestModel creates an in-process checkout TUI model for a temp repo.
func newCheckoutTestModel(tb testing.TB, repoDir string) *testModel {
	tb.Helper()
	runner := newRunnerInDir(tb, repoDir)
	cfg := defaultConfig()
	m := commands.NewCheckoutModel(context.Background(), runner, cfg, plainRenderer())
	appModel := app.New(&checkoutTeaModelAdapter{inner: m}, runner, cfg)
	return newTestModel(tb, appModel)
}

// newDiffTestModel creates an in-process diff TUI model for a temp repo.
func newDiffTestModel(tb testing.TB, repoDir string, revision string, staged bool) *testModel {
	tb.Helper()
	runner := newRunnerInDir(tb, repoDir)
	cfg := defaultConfig()
	m := commands.NewDiffModel(context.Background(), runner, cfg, plainRenderer(), revision, staged)
	appModel := app.New(&diffTeaModelAdapter{inner: m}, runner, cfg)
	return newTestModel(tb, appModel)
}

// newFixupTestModel creates an in-process fixup TUI model for a temp repo.
func newFixupTestModel(tb testing.TB, repoDir string) (*testModel, error) {
	tb.Helper()
	runner := newRunnerInDir(tb, repoDir)
	cfg := defaultConfig()
	m, err := commands.NewFixupModel(context.Background(), runner, cfg, plainRenderer())
	if err != nil {
		return nil, fmt.Errorf("NewFixupModel: %w", err)
	}
	appModel := app.New(&fixupTeaModelAdapter{inner: m}, runner, cfg)
	return newTestModel(tb, appModel), nil
}

// newHunkAddTestModel creates an in-process hunk-add TUI model for a temp repo.
func newHunkAddTestModel(tb testing.TB, repoDir string) *testModel {
	tb.Helper()
	runner := newRunnerInDir(tb, repoDir)
	cfg := defaultConfig()
	m := commands.NewHunkAddModel(context.Background(), runner, cfg, plainRenderer())
	appModel := app.New(&hunkAddTeaModelAdapter{inner: m}, runner, cfg)
	return newTestModel(tb, appModel)
}

// newLogTestModel creates an in-process log TUI model for a temp repo.
func newLogTestModel(tb testing.TB, repoDir string, ref string) *testModel {
	tb.Helper()
	runner := newRunnerInDir(tb, repoDir)
	cfg := defaultConfig()
	m := commands.NewLogModel(context.Background(), runner, cfg, plainRenderer(), ref)
	appModel := app.New(&logTeaModelAdapter{inner: m}, runner, cfg)
	return newTestModel(tb, appModel)
}

// newRebaseInteractiveTestModel creates an in-process rebase-interactive TUI model.
func newRebaseInteractiveTestModel(tb testing.TB, repoDir string, base string) *testModel {
	tb.Helper()
	runner := newRunnerInDir(tb, repoDir)
	cfg := defaultConfig()
	m := commands.NewRebaseInteractiveModel(context.Background(), runner, cfg, plainRenderer(), base, "")
	appModel := app.New(&rebaseInteractiveTeaModelAdapter{inner: m}, runner, cfg)
	return newTestModel(tb, appModel)
}

// newResetTestModel creates an in-process reset TUI model for a temp repo.
func newResetTestModel(tb testing.TB, repoDir string) *testModel {
	tb.Helper()
	runner := newRunnerInDir(tb, repoDir)
	cfg := defaultConfig()
	m := commands.NewResetModel(context.Background(), runner, cfg, plainRenderer())
	appModel := app.New(&resetTeaModelAdapter{inner: m}, runner, cfg)
	return newTestModel(tb, appModel)
}

// containsOutput returns a waitFor condition that checks for a substring.
func containsOutput(substr string) func(string) bool {
	return func(output string) bool {
		return strings.Contains(output, substr)
	}
}

// gitRun executes a git command in the given repo and returns trimmed output.
func gitRun(tb testing.TB, repoDir string, args ...string) string {
	tb.Helper()
	ctx := context.Background()
	runner := newRunnerInDir(tb, repoDir)
	out, err := runner.Run(ctx, args...)
	if err != nil {
		tb.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(out)
}

// assertGitStaged fatally fails if any of the given files is absent from
// the git index (git diff --cached --name-only).
func assertGitStaged(tb testing.TB, repoDir string, files ...string) {
	tb.Helper()
	cached := gitRun(tb, repoDir, "diff", "--cached", "--name-only")
	for _, f := range files {
		if !strings.Contains(cached, f) {
			tb.Fatalf("assertGitStaged: %q not found in staged files.\nStaged:\n%s", f, cached)
		}
	}
}

// assertGitNotStaged fatally fails if any of the given files appears in
// the git index (git diff --cached --name-only).
func assertGitNotStaged(tb testing.TB, repoDir string, files ...string) {
	tb.Helper()
	cached := gitRun(tb, repoDir, "diff", "--cached", "--name-only")
	for _, f := range files {
		if strings.Contains(cached, f) {
			tb.Fatalf("assertGitNotStaged: %q found in staged files but should not be.\nStaged:\n%s", f, cached)
		}
	}
}

// assertOutputContains fatally fails if text is not present in the current
// TUI output buffer.
func assertOutputContains(tb testing.TB, tm *testModel, text string) {
	tb.Helper()
	out := tm.out.String()
	if !strings.Contains(out, text) {
		tb.Fatalf("assertOutputContains: %q not found in TUI output.\nOutput (%d bytes):\n%s", text, len(out), out)
	}
}
