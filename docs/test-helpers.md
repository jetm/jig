# Test Helpers and Coverage

jig-specific test infrastructure, helper packages, and per-package coverage requirements.

---

## testhelper package (`internal/testhelper/`)

### FakeRunner (`fakerunner.go`)

Records all calls and returns scripted responses in FIFO order. Safe for concurrent use from parallel subtests.

```go
type FakeRunner struct {
    mu      sync.Mutex
    Calls   []Call
    Outputs []string // FIFO: first call gets Outputs[0]
    Errors  []error  // FIFO: parallel to Outputs
}

type Call struct {
    Args  []string
    Env   []string // populated only for RunWithEnv calls
    Stdin string   // populated only for RunWithStdin calls
}

func (f *FakeRunner) Run(_ context.Context, args ...string) (string, error)
func (f *FakeRunner) RunWithEnv(_ context.Context, env []string, args ...string) (string, error)
func (f *FakeRunner) RunWithStdin(_ context.Context, stdin string, args ...string) (string, error)
```

### FakeRunner assertion helpers

```go
// MustHaveCall fails tb if no Run* call matched all of args (subset match).
func MustHaveCall(tb testing.TB, f *FakeRunner, args ...string)

// MustHaveEnv fails tb if key=value was not present in any RunWithEnv call.
func MustHaveEnv(tb testing.TB, f *FakeRunner, keyvalue string)

// MustHaveStdin fails tb if no RunWithStdin call contained substr.
func MustHaveStdin(tb testing.TB, f *FakeRunner, substr string)

// MustHaveNoCall fails tb if any Run* call was made. Use to assert destructive
// operations did not fire before the required confirmation keypress.
func MustHaveNoCall(tb testing.TB, f *FakeRunner)

// CallCount returns the total number of Run* calls. Use to verify exact call
// count (e.g. batch staging made exactly 2 apply calls, not 3).
func CallCount(f *FakeRunner) int

// NthCall returns the Nth call (0-indexed). Use to verify call ordering:
//   first := NthCall(r, 0)
//   require.Equal(t, []string{"commit", "--fixup=abc"}, first.Args)
//   second := NthCall(r, 1)
//   require.Equal(t, []string{"rebase", "--autosquash", "--interactive", "abc^"}, second.Args)
func NthCall(f *FakeRunner, n int) Call
```

**Assertion best practices:**

```go
// WRONG — proves git add was called, not that the right file was staged:
MustHaveCall(tb, r, "add")

// RIGHT — proves the exact file was passed:
MustHaveCall(tb, r, "add", "driver.c")

// For RunWithStdin, parse the patch content structurally:
call := r.LastStdinCall()
hunks, err := ParseHunks(call.Stdin)
require.NoError(t, err)
require.Len(t, hunks, 1)
require.Equal(t, 2, countSelected(hunks[0], '+'))
```

### Git repo helpers (`gitrepo.go`)

Uses `os.Root` (Go 1.24+) for all file I/O inside the temp repo.

```go
// NewTempRepo inits a real git repo in tb.TempDir() with one initial commit.
// Configures user.email and user.name locally so commits work in CI.
func NewTempRepo(tb testing.TB) string

// AddCommit stages all changes and creates a commit. Returns the short hash.
func AddCommit(tb testing.TB, repoPath, msg string) string

// WriteFile creates or overwrites repoPath/name using os.Root.
func WriteFile(tb testing.TB, repoPath, name, content string)

// StageFile runs git add <name> inside repoPath.
func StageFile(tb testing.TB, repoPath, name string)

// CommitCount returns the number of commits reachable from HEAD.
func CommitCount(tb testing.TB, repoPath string) int
```

### UI test helpers (`components_test_helpers_test.go`)

Drive `Update()` programmatically. Never start a real `tea.Program`.

```go
func sendKey(m tea.Model, key string) tea.Model {
    msg := tea.KeyPressMsg{Code: []rune(key)[0]}
    next, _ := m.Update(msg)
    return next
}

func sendSpecialKey(m tea.Model, code rune) tea.Model {
    msg := tea.KeyPressMsg{Code: code}
    next, _ := m.Update(msg)
    return next
}
// Usage: sendSpecialKey(m, tea.KeyUp), sendSpecialKey(m, tea.KeyDown),
//        sendSpecialKey(m, tea.KeyHome), sendSpecialKey(m, tea.KeyEnd)
```

**Bubbletea v2 key handling in tests:**
- Match on `tea.KeyPressMsg`, not `tea.KeyMsg`
- Use `msg.Code` (a `rune`), not `msg.Type`
- Space bar: `msg.String()` returns `"space"`, not `" "`. `msg.Code == ' '` still works
- Modifiers: use `msg.Mod` field (e.g. `msg.Mod == tea.ModCtrl`)

---

## Coverage requirements by package

### `internal/git`

| Function | Min cases | Key scenarios |
|----------|-----------|---------------|
| `ParseStatus` | 6+ | empty, modified-only, untracked-only, mixed, renames, files with spaces |
| `ParseFileDiff` | 5+ | modified, added, deleted, renamed, binary |
| `ParseHunks` | 6+ | empty diff, single hunk, multi-hunk, added-only, deleted-only, renamed-with-changes |
| `SplitHunks` | 2+ | multi-hunk → N individual hunks; single-hunk → unchanged |
| `SplitHunkAt` | 4+ | splittable hunk, all-changed (no split), context-only (no split), single-line |
| `RecalculateHeader` | 4+ | standard hunk, after editor deletes `+`, after editor removes `-`, trailing context preserved |
| `BuildPatch` | 3+ | selected lines → valid diff, unselected `-` → demoted to context, all unselected → `ErrNothingSelected` |
| `ParseLog` | 5+ | empty, single, multiple, decorated refs, unicode subjects |
| `ParseTodo` | 6+ | empty, single pick, all six actions, comment lines skipped, unicode, exec lines |
| `WriteTodo` | - | assert each action serializes correctly; round-trip property with `ParseTodo` |

### `internal/diff`

All renderers share one fixture: `testdata/sample.diff`.

- `PlainRenderer`: output equals input
- `ChromaRenderer`: output contains ANSI escapes; `+`/`-` prefixes preserved
- `DeltaRenderer`: `t.Skip("delta not in PATH")` if `exec.LookPath` fails
- `Chain()`: table-driven over `Config` combinations → assert correct concrete type

### `internal/tui/components`

- `ItemList`: cursor movement, fuzzy filter, `SelectedItem()` changes
- `HunkView`: all toggle/split/undo/edit interactions (see detailed test specs in project doc)
- `DiffView`: `SetContent`, scroll behavior
- `TodoList`: action changes, reorder, visual mode, undo, break insertion/removal
- `LogView`: detail level cycling, search prefix routing, cross-command messages, lazy loading
- `StatusBar`: hints and branch display

### `internal/commands`

Each command follows the TDD sequence:
1. Write `_test.go` with FakeRunner assertions (RED)
2. Write command stub so tests compile but fail (still RED)
3. Implement until tests pass (GREEN)
4. Write integration `_test.go` with build tag `integration` (RED)
5. Implement missing runner calls (GREEN)
6. Refactor

### `internal/config`

- `JIG_UI_THEME=light` overrides `ui.theme: "dark"` from yaml file
- Use `t.Setenv` (auto-reverts) and `t.TempDir()` for config file
- Env vars must take precedence over file values across all fields

---

## Integration tests

### Build tag

All integration tests use `//go:build integration` and live in `tests/integration/`. Run with:

```bash
make test-integration  # go test -race -tags integration ./tests/integration/...
```

### Git state verification patterns

After every git-mutating action, verify the **exact** git state:

| After... | Verify |
|----------|--------|
| Staging a file | `git diff --cached --name-only` contains the file |
| Reset (mixed) | `CommitCount` matches expected; `git status --porcelain` lists files as unstaged; `git diff --cached` is empty |
| Reset (hard) | `CommitCount` matches; `git status --porcelain` is empty; deleted files absent from disk |
| Fixup | Commit count unchanged; `git show <target>` includes fixup'd content; working tree clean |
| Restore | `git diff --name-only` no longer contains file; file content matches index |
| Hunk staging | `git diff --cached` contains staged hunks; `git diff` contains unstaged hunks |

### Cross-command workflow tests (`tests/integration/workflow_test.go`)

These simulate real user sessions crossing command boundaries via the `app.Model` stack. They use real `ExecRunner` (not FakeRunner) and real temp git repos. 8 workflows defined:

1. **Selective hunk staging from add** — add → hunk-add → partial stage → verify split staging
2. **Log → fixup round-trip** — log → F key → fixup → verify commit absorbed
3. **Log → rebase squash via visual mode** — log → R → visual select → squash → verify count
4. **Hunk split then selective staging** — split hunk → stage first half only
5. **Reset then re-stage** — reset mixed → verify unstaged → re-stage via add
6. **Log → diff (read-only, no refresh)** — log → D → diff → q → verify no re-fetch
7. **Partial staging visibility in add** — hunk stage → add shows file in both sections
8. **Checkout bulk restore** — double-A confirm pattern → verify all restored

### TUI startup assertion pattern

Integration tests for TUI commands use `runTUI` (in `helpers_test.go`) which launches the binary with `TERM=dumb`, pipes `"q\n"` to stdin, and captures stderr. The key assertion is:

```go
stderr, _ := runTUI(t, repoDir, "fixup")
assert.Empty(t, stderr, "should start without errors")
```

**Why stderr, not exit code?** Bubbletea may exit non-zero in dumb terminals (no TTY). That's fine. But pre-TUI failures - git command errors, precondition checks - cause cobra to print `Error: <message>` to stderr before the TUI ever starts. Asserting stderr is empty catches these startup bugs.

**TTY error filtering:** In environments without `/dev/tty` (CI, sandboxes), bubbletea itself produces a TTY error on stderr. `filterTTYError()` strips this known-benign output so it doesn't cause false failures. Real errors (e.g. `fork/exec: invalid argument` from a bad git format string) never contain "could not open TTY" and pass through unfiltered.

**Editor-mode tests** (rebase-interactive with a todo file path) can't use `runTUI` because they need custom stdin. These capture stderr manually and call `filterTTYError` directly:

```go
var stderrBuf bytes.Buffer
cmd.Stderr = &stderrBuf
_ = cmd.Run()
assert.Empty(t, filterTTYError(stderrBuf.String()), "should start without errors")
```

### In-process TUI interaction tests (`teatest_helpers_test.go`)

The third test layer exercises the full `tea.Program` event loop with real git state mutations. Unlike the startup tests above (which launch the compiled binary), these construct models in-process with virtual I/O - no real TTY needed.

**Why not upstream teatest?** The `github.com/charmbracelet/x/exp/teatest/v2` module imports `github.com/charmbracelet/bubbletea/v2`, but jig uses `charm.land/bubbletea/v2`. These are different Go module paths with incompatible types. The helpers in `teatest_helpers_test.go` are a ~150-line equivalent using our bubbletea directly.

**Architecture:**

```text
testModel
  ├── tea.Program (WithInput=empty, WithOutput=safeBuffer, WithoutSignals)
  ├── safeBuffer (concurrency-safe output capture)
  ├── modelCh (receives final model after program exits)
  └── doneCh (signals program completion)
```

The `testModel` wraps a `tea.Program` with:
- `bytes.NewReader(nil)` as input (no stdin - all input via `Send()`)
- `safeBuffer` as output (concurrency-safe for reads during program execution)
- `WithoutSignals()` to prevent signal handling interference
- `WithWindowSize(120, 40)` for consistent rendering

**Core helpers:**

```go
// testModel methods
func (tm *testModel) send(msg tea.Msg)           // inject messages into the event loop
func (tm *testModel) waitFor(tb, condition, ...)  // poll output until condition or 5s timeout
func (tm *testModel) quit(tb)                     // send quit + wait for program exit
func (tm *testModel) waitDone(tb)                 // wait for program to finish (5s timeout)

// Keystroke helpers
func sendSpace(tm *testModel)          // tea.KeySpace
func sendEnter(tm *testModel)          // tea.KeyEnter
func sendTab(tm *testModel)            // tea.KeyTab
func sendKey(tm *testModel, c rune)    // printable character (e.g. 'a', 's', 'y')

// Output condition helpers
func containsOutput(substr string) func(string) bool
```

**Model construction:** Each command has a `newXxxTestModel` helper that handles the full construction chain:

1. `newRunnerInDir(tb, dir)` - holds `cwdMu` during `os.Chdir` + `git.NewExecRunner` + restore
2. `config.Load()` + `diff.PlainRenderer{}` (no delta/chroma dependency)
3. `commands.NewXxxModel(...)` with command-specific parameters
4. `xxxTeaModelAdapter` wrapping (duplicates unexported adapters from `cmd/jig/main.go`)
5. `app.New(adapter, runner, cfg)` as the root model
6. `newTestModel(tb, appModel)` starts the program in a goroutine

```go
// Available constructors
func newAddTestModel(tb, repoDir string) *testModel
func newCheckoutTestModel(tb, repoDir string) *testModel
func newDiffTestModel(tb, repoDir string, revision string, staged bool) *testModel
func newFixupTestModel(tb, repoDir string) (*testModel, error)  // fixup can fail if nothing staged
func newHunkAddTestModel(tb, repoDir string) *testModel
func newLogTestModel(tb, repoDir string, ref string) *testModel
func newRebaseInteractiveTestModel(tb, repoDir string, base string) *testModel
func newResetTestModel(tb, repoDir string) *testModel
```

**CWD safety:** `git.NewExecRunner` resolves the repo root via `git rev-parse --show-toplevel` from CWD. A package-level `cwdMu sync.Mutex` serializes all `os.Chdir` calls during runner construction. Each test gets its own temp repo and runner instance - the mutex is only held during construction, not during test execution.

**Test pattern:** Wait for render, send keystrokes, assert git state.

```go
func TestAdd_TUI_StageSingleFile(t *testing.T) {
    repoDir := testhelper.NewTempRepo(t)
    testhelper.WriteFile(t, repoDir, "file1.txt", "hello\n")
    testhelper.AddCommit(t, repoDir, "add file1.txt")
    testhelper.WriteFile(t, repoDir, "file1.txt", "hello modified\n")

    tm := newAddTestModel(t, repoDir)

    tm.waitFor(t, containsOutput("file1.txt"))  // wait for TUI to render
    sendSpace(tm)                                 // select file
    sendEnter(tm)                                 // confirm staging
    tm.waitDone(t)                                // wait for program exit

    // Assert git state, not rendered output
    cached := gitRun(t, repoDir, "diff", "--name-only", "--cached")
    assert.Contains(t, cached, "file1.txt")
}
```

**Test coverage by command:**

| Command | Test | Interaction | Git assertion |
|---------|------|-------------|---------------|
| add | Stage single file | Space + Enter | `diff --cached --name-only` contains file |
| add | Stage all files | 'a' + Enter | `diff --cached --name-only` contains all files |
| checkout | Restore file | Space + Enter + 'y' | File content reverts to committed version |
| diff | Show modified files | (render only) | Output contains file name |
| diff | Staged flag | (render only) | Output contains staged file name |
| fixup | Fixup into commit | Enter | `CommitCount` unchanged, no staged files remain |
| hunk-add | Stage hunk | 'a' (stage all) | `diff --cached --name-only` contains file |
| log | Render commits | (render only) | Output contains commit messages |
| log | Tab switches panel | Tab | Output changes (panel focus styling) |
| rebase | Set squash action | 's' | Output contains "squash" |
| rebase | Reorder commits | 'j' + 'K' | Commit order changes in output |
| reset | Unstage file | Space + Enter | `diff --cached --name-only` no longer contains file |
