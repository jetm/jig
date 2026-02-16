# git-tui — Project Instructions for Claude Code

## What we are building

A focused Go TUI for the eight git workflows Javier uses daily. Not a general-purpose
git client — a sharp tool for exactly these operations:

| Command            | Alias   | Replaces                          |
|--------------------|---------|-----------------------------------|
| `add`              | `ga`    | forgit add                        |
| `hunk-add`         | `gha`   | git add -p / git add --interactive|
| `checkout`         | `gc`    | git restore <file> (restore working tree) |
| `diff`             | `gd`    | diffnav                           |
| `fixup`            | `gfix`  | manual git commit --fixup + rebase|
| `rebase-interactive` | `gri` | git-interactive-rebase-tool       |
| `reset`              | `gr`    | git reset --soft/--mixed/--hard   |
| `log`                | `gl`    | tig / git log (visual commit browser) |

The binary is named `jig` (git-tui). Shell aliases wrap each subcommand.

**Reference tools** (study these for functionality baseline):
- **forgit** — [github.com/wfxr/forgit](https://github.com/wfxr/forgit)
  fzf-powered interactive git. Reference for `add` and `checkout` UX.
- **diffnav** — [github.com/dlvhdr/diffnav](https://github.com/dlvhdr/diffnav)
  Git diff pager with file tree. Reference for `diff` two-panel layout and delta integration.
- **git-interactive-rebase-tool** — [github.com/MitMaro/git-interactive-rebase-tool](https://github.com/MitMaro/git-interactive-rebase-tool)
  Terminal sequence editor for rebase. Reference for `rebase-interactive` action keys,
  visual mode, diff preview, and `$GIT_SEQUENCE_EDITOR` integration.
- **tig** — [github.com/jonas/tig](https://github.com/jonas/tig)
  Ncurses git browser. Reference for `log` commit list, detail levels, and search.

`rebase-interactive` also registers as a `$GIT_SEQUENCE_EDITOR` replacement:
```
git config --global sequence.editor "jig rebase-interactive"
```
When invoked this way, git passes the rebase todo file path as the sole argument and
the command edits it in place — exactly like MitMaro's git-interactive-rebase-tool.

---

## Terminal requirements

**jig targets modern terminals exclusively.** No fallbacks for legacy terminals.

| Requirement           | Minimum                                                    |
|-----------------------|------------------------------------------------------------|
| **Truecolor**         | 24-bit color (`COLORTERM=truecolor`). No 256-color fallback. |
| **Nerd Font**         | Any Nerd Font patched font (v3+). Icons will render as tofu without one. |
| **Unicode**           | Full Unicode support including wide characters.            |
| **Keyboard protocol** | Kitty keyboard protocol preferred (Ghostty, Kitty, WezTerm, Alacritty, iTerm2). Bubbletea v2 auto-enables progressive enhancement and falls back gracefully on terminals without it. |
| **Bold / Italic**     | Terminal must render bold and italic ANSI attributes.      |
| **Strikethrough**     | Required for rebase `drop` action rendering.               |

**Tested terminals:** Ghostty, Kitty, WezTerm, Alacritty, foot, iTerm2.
**Not supported:** bare Linux VT, old xterm without truecolor, Windows Console (conhost).
Windows Terminal and VSCode integrated terminal work if Nerd Font is configured.

Bubbletea v2 auto-detects the color profile and enables synchronized output (Mode 2026)
and Unicode width handling (Mode 2027) on supporting terminals. No manual configuration
needed — declare `v.AltScreen = true` in `View()` and the framework handles the rest.

---

## Universal UI contract — two columns, always

Every command renders the same two-column layout. The left column changes per command;
the right column is always a syntax-highlighted diff viewport.

```
+------------------------+-----------------------------------------------+
| LEFT PANEL             | RIGHT PANEL                                   |
| (files / hunks / todo) | (diff — chroma highlighted, scrollable)       |
|                        |                                               |
| [item 1]  +3 -1        |  diff --git a/driver.c b/driver.c             |
| [item 2]  +12          |  @@ -42,7 +42,10 @@                          |
| > [item 3] selected    |   int ret;                                    |
| [item 4]               |  -old_init_sequence();                        |
|                        |  +new_init_a();                               |
|                        |  +new_init_b();                               |
+------------------------+-----------------------------------------------+
| <keyhints for current command>                     [branch] [mode]     |
+------------------------------------------------------------------------+
```

**Layout rules:**
- Left panel: fixed 30% width, min 28 cols. Right panel: remaining 70%.
- Both panels reflow on every `tea.WindowSizeMsg`.
- Left panel uses `bubbles/list` for navigation. Right panel uses `bubbles/viewport`.
- In bubbles v2, width/height are set via methods: `list.SetWidth(w)`, `viewport.SetWidth(w)`,
  `viewport.SetHeight(h)` — not direct field assignment.
- Diff content in the right panel always uses `lipgloss.Width()` (never `len()`) for
  ANSI-safe width measurement.
- The **top-level model's `View()` returns `tea.View`** (bubbletea v2), not a string.
  Child components can still return strings from their own View methods. The root model
  assembles them and wraps in `tea.NewView(content)`:
  ```go
  // AppModel.View() — the only model that returns tea.View
  func (a *AppModel) View() tea.View {
      content := a.Active().View() // child models return string
      v := tea.NewView(content)
      v.AltScreen = true
      return v
  }

  // Command models assemble their layout and return string:
  func (m AddModel) View() string {
      left := m.itemList.View()
      right := m.diffView.View()
      content := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
      return content + "\n" + m.statusBar.View()
  }
  ```
- **Focus indicator:** the active panel’s border uses `ColorBlue` (`#61afef`);
  the inactive panel’s border uses `ColorFgSubtle` (`#5c6370`). When a command has
  no focus switching (e.g. `add`, `checkout`), only the left panel border is shown.
- The bottom status bar is one line: left-aligned keyhints, right-aligned branch + mode.
- **Mutation feedback:** every action that changes git state (stage, unstage, restore,
  reset, fixup, rebase) shows a confirmation in the status bar using `ColorGreen` +
  `IconSuccess`: e.g. " staged driver.c", " restored probe.c", " reset to d4e5f6 (mixed)".
  Errors use `ColorRed` + `IconError`. Messages auto-clear after 3 seconds or on the
  next keypress, whichever comes first.
- **List refresh after mutation:** after any git-mutating action (stage, unstage, restore,
  stage hunk, reset), the left panel re-fetches its data source (`git status`, `git diff`,
  `git log`) and rebuilds the list. The cursor stays on the same item if it still exists,
  or moves to the nearest item if it was removed. This is non-negotiable — stale lists
  after mutations are the primary UX failure mode in terminal git tools.
- **Clipboard yank:** `y` copies the current item’s primary identifier to the system
  clipboard (via `GT_COPY_CMD`): file path for file lists, commit hash for commit lists. Status bar confirms: " copied <value>". If `GT_COPY_CMD`
  is not available, show "clipboard not available" and skip silently.
- **Help overlay (`?`):** pressing `?` in any command shows a modal keybinding
  reference overlaid on the current view. `<esc>` or `?` again dismisses it.
  The overlay lists all keybindings for the current command, grouped by function.
  This is critical for discoverability — users shouldn't need to leave the tool to
  look up a keybinding. The overlay is a single `components/helpoverlay.go` shared
  by all commands, populated with command-specific content at construction time.
- **Minimum terminal size:** 60 columns × 10 rows. If the terminal is smaller,
  show a centered message: "terminal too small (need 60×10, have WxH)" and
  re-check on every `tea.WindowSizeMsg`. Do not render the two-panel layout.
- No modal confirm dialogs. Destructive actions require a second keypress (`D` not `d`),
  shown in the keyhints bar as `<D> drop`.

---

## Keybinding contract — vim-style navigation everywhere

All commands follow these base vim keybindings unless explicitly overridden:

| Key               | Action                                     | Scope         |
|-------------------|--------------------------------------------|---------------|
| `j` / `↓`        | move cursor down                           | all commands  |
| `k` / `↑`        | move cursor up                             | all commands  |
| `Home`            | jump to first item                         | all commands  |
| `End`             | jump to last item                          | all commands  |
| `/`               | open fuzzy filter                          | file lists    |
| `q`               | emit `PopModelMsg` (quit / return to parent) | all commands  |
| `<esc>`            | context: unfocus panel, clear search, or `PopModelMsg` | all commands  |
| `<enter>`         | confirm / descend                          | all commands  |
| `PageDown` / `PageUp` | half-page scroll down/up              | diff viewport |
| `n` / `N`         | next/prev hunk header                      | diff viewport |
| `<tab>`           | cycle detail level / mode (where applicable) | `log`       |
| `?`               | toggle help overlay (keybinding reference)   | all commands  |

**Exception — `rebase-interactive`:** In the todo editor, the primary action is
reordering commits, so `j`/`k` are repurposed for **move row down/up** (reorder),
and `↑`/`↓` arrows handle cursor navigation. This mirrors the ergonomics of
dedicated rebase editors where reordering is the most frequent physical action.

---

## Color palette — OneDark (canonical)

All colors in `internal/tui/styles.go` **must** use these exact hex values. No other
colors are permitted anywhere in the codebase. If lipgloss needs a color not in this
palette, use the nearest token — never invent a new hex value.

**Lipgloss v2 is pure** — it no longer performs I/O. Bubbletea v2 manages terminal
detection and handles truecolor output natively. Since jig requires truecolor,
downsampling is not a concern. Do not use `lipgloss/v2/compat` (which re-introduces
I/O) — the bubbletea program handles it.

```go
// internal/tui/styles.go

// OneDark palette — matches Atom One Dark / the terminal theme in use.
const (
    // Backgrounds
    ColorBg        = lipgloss.Color("#282c34") // editor background
    ColorBgAlt     = lipgloss.Color("#2c313c") // current-line / panel header bg
    ColorBgSel     = lipgloss.Color("#3e4451") // selection / highlighted item bg
    ColorBgFloat   = lipgloss.Color("#21252b") // status bar background

    // Foregrounds
    ColorFg        = lipgloss.Color("#abb2bf") // default text
    ColorFgSubtle  = lipgloss.Color("#5c6370") // comments, line numbers, muted
    ColorFgEmph    = lipgloss.Color("#ffffff") // emphasized / cursor text

    // Syntax / semantic colors
    ColorRed       = lipgloss.Color("#e06c75") // deleted lines, errors, drop action
    ColorOrange    = lipgloss.Color("#d19a66") // numbers, constants
    ColorYellow    = lipgloss.Color("#e5c07b") // reword action, warnings
    ColorGreen     = lipgloss.Color("#98c379") // added lines, success
    ColorCyan      = lipgloss.Color("#56b6c2") // edit action, types
    ColorBlue      = lipgloss.Color("#61afef") // pick action, functions
    ColorPurple    = lipgloss.Color("#c678dd") // squash/fixup action, keywords
)

// Nerd Font icons — require any Nerd Font v3+ patched font.
// See https://www.nerdfonts.com/cheat-sheet for codepoint reference.
const (
    // File status
    IconModified  = "\uf040"  //  (nf-fa-pencil)
    IconAdded     = "\uf055"  //  (nf-fa-plus_circle)
    IconDeleted   = "\uf056"  //  (nf-fa-minus_circle)
    IconRenamed   = "\uf553"  // 󰕓 (nf-md-rename_box)
    IconUntracked = "\uf128"  //  (nf-fa-question)

    // Git
    IconBranch    = "\ue725"  //  (nf-dev-git_branch)
    IconCommit    = "\uf417"  //  (nf-oct-git_commit)

    // UI
    IconChecked   = "\uf14a"  //  (nf-fa-check_square)
    IconUnchecked = "\uf096"  //  (nf-fa-square_o)
    IconWarning   = "\uf071"  //  (nf-fa-warning)
    IconError     = "\uf06a"  //  (nf-fa-exclamation_circle)
    IconSuccess   = "\uf058"  //  (nf-fa-check_circle)
    IconDiff      = "\uf440"  //  (nf-oct-diff)
    IconFilter    = "\uf002"  //  (nf-fa-search)

    // Rebase actions
    IconPick      = "\uf00c"  //  (nf-fa-check)
    IconReword    = "\uf044"  //  (nf-fa-pencil_square_o)
    IconEdit      = "\uf040"  //  (nf-fa-pencil)
    IconSquash    = "\uf066"  //  (nf-fa-compress)
    IconFixup     = "\uf0e2"  //  (nf-fa-undo)
    IconDrop      = "\uf014"  //  (nf-fa-trash)
)
```

**Typography:** use `lipgloss.Bold(true)` and `lipgloss.Italic(true)` for emphasis:
- **Bold:** panel headers, active/cursor item text, action labels in rebase todo,
  branch name in status bar, error/warning messages.
- **Italic:** muted/secondary text (line numbers, keyhint labels, mode indicators,
  hunk context lines, commit dates). Italic + dim for `fixup` rebase action.
- **Strikethrough:** `drop` rebase action (`lipgloss.Strikethrough(true)`).

**Usage by semantic role:**

| Role                              | Token           | Icon / Style                        |
|-----------------------------------|-----------------|-------------------------------------|
| Panel background                  | `ColorBg`       |                                     |
| Panel header / current-line bg    | `ColorBgAlt`    | **bold** text                       |
| Selected item background          | `ColorBgSel`    |                                     |
| Status bar background             | `ColorBgFloat`  |                                     |
| Normal text                       | `ColorFg`       |                                     |
| Muted text (line nums, comments)  | `ColorFgSubtle` | *italic*                            |
| Cursor / active item text         | `ColorFgEmph`   | **bold**                            |
| Deleted diff lines (`-`)          | `ColorRed`      |                                     |
| Added diff lines (`+`)            | `ColorGreen`    |                                     |
| Diff hunk header (`@@`)           | `ColorCyan`     | **bold**                            |
| `+N` stat (additions)             | `ColorGreen`    |                                     |
| `-N` stat (deletions)             | `ColorRed`      |                                     |
| File status: modified             | `ColorYellow`   | `IconModified` () **bold**         |
| File status: added                | `ColorGreen`    | `IconAdded` ()                    |
| File status: deleted              | `ColorRed`      | `IconDeleted` ()                  |
| File status: renamed              | `ColorCyan`     | `IconRenamed` (󰕓)                  |
| File status: untracked            | `ColorFgSubtle` | `IconUntracked` () *italic*       |
| Branch name in status bar         | `ColorBlue`     | `IconBranch` () **bold**           |
| Mode label in status bar          | `ColorFgSubtle` | *italic*                            |
| Key hint labels (`<space>`)       | `ColorFgSubtle` | *italic*                            |
| Key hint actions (`stage`)        | `ColorFg`       |                                     |
| Error message in status bar       | `ColorRed`      | `IconError` () **bold**            |
| Warning / info in status bar      | `ColorYellow`   | `IconWarning` () **bold**          |
| Success message in status bar     | `ColorGreen`    | `IconSuccess` ()                   |
| Rebase action: pick               | `ColorBlue`     | `IconPick` () **bold**             |
| Rebase action: reword             | `ColorYellow`   | `IconReword` () **bold**           |
| Rebase action: edit               | `ColorCyan`     | `IconEdit` () **bold**             |
| Rebase action: squash             | `ColorPurple`   | `IconSquash` () **bold**           |
| Rebase action: fixup              | `ColorPurple`   | `IconFixup` () *italic* + dim      |
| Rebase action: drop               | `ColorRed`      | `IconDrop` () dim + strikethrough  |
| Hunk toggle: selected             | `ColorGreen`    | `IconChecked` ()                   |
| Hunk toggle: unselected           | `ColorFgSubtle` | `IconUnchecked` ()                 |
| Hunk context lines                | `ColorFg`       | *italic*                            |
| Commit hash in left panel         | `ColorOrange`   | `IconCommit` ()                    |
| Reset mode indicator: mixed       | `ColorFgSubtle` | *italic*                            |
| Rebase break line                 | `ColorOrange`   | *italic*                            |
| Visual mode selection bg          | `ColorBgSel`    | highlighted range                   |
| Visual mode indicator             | `ColorPurple`   | **bold** status bar text             |
| Log: author name                  | `ColorBlue`     | *italic*                            |
| Log: date/age                     | `ColorFgSubtle` | *italic*                            |
| Log: search match highlight       | `ColorYellow`   | **bold** + `ColorBgSel` background  |
| Log: detail level indicator       | `ColorCyan`     | **bold** status bar text             |

**Chroma renderer:** use the `"one-dark"` style name — `chroma` ships it natively.
```go
style, _ := styles.Get("one-dark") // alecthomas/chroma/v2/styles
```
This ensures syntax highlighting in the diff viewport matches the terminal theme exactly.

**Delta:** if delta is the active renderer, set these in `~/.config/jig/config.toml` or
document them in README as recommended delta config:
```toml
[delta]
syntax-theme = "OneHalfDark"   # closest delta built-in to One Dark
```
The user's own delta config takes precedence; `jig` only passes `--color-only --width N`
and never overrides delta's theme.

---

**Right panel diff rendering chain** (selected once at startup, stored in `Config`):
```
delta (if found) → chroma native → plain ANSI fallback
```
`delta` is invoked with `--color-only --width <panel_width>` so it respects the panel
size. `chroma` uses the `diff` lexer with `terminal16m` (truecolor) formatter. Plain is the raw
diff string unchanged (still truecolor-safe — no ANSI stripping).

**Runtime fallback:** the renderer is selected once at startup. If `delta` is found
but fails on a specific diff (non-zero exit), fall back to `chroma` for that diff
and log the error via `slog.Warn`. Do not crash or show raw stderr to the user.

---

## Constraints — read before touching any code

- **Never** write implementation code before a failing test exists for it. See TDD rule.
- **Never** use goroutines inside `Update()`. All async work goes through `tea.Cmd`.
- **Never** use go-git for write operations. All git mutations use `os/exec` via
  `internal/git/runner.go`.
- **Never** call `os.Exit` outside `main.go`.
- **Never** swallow errors. Wrap with `fmt.Errorf("context: %w", err)` and surface as
  `errMsg` displayed in the status bar (not a modal).
- **Never** use `any` (or `interface{}`) where a concrete type or constrained generic is
  possible. `go fix` rewrites `interface{}` → `any` automatically; the constraint here
  is about avoiding empty-interface erasure of type information.
- **Never** use an older stdlib pattern when a Go 1.26 equivalent exists
  (e.g. use `slices.Contains` not a manual loop, `min`/`max` builtins not ternaries,
  `errors.AsType[E]` not `errors.As`, `new(expr)` not the two-statement pointer pattern).
  Run `go fix ./...` to auto-modernize; see Makefile `fix` target.
- **`make test` must pass with ≥90% coverage before any phase is marked done.**

---

## Bubbletea v2 — critical API differences from v1

The Charm stack shipped v2 stable on 2026-02-23. All code in this project uses v2.
Do **not** use v1 patterns — Claude's training data is overwhelmingly v1 and will
produce wrong code by default. Key differences:

**Import paths changed:**
```
v1: github.com/charmbracelet/bubbletea    → v2: charm.land/bubbletea/v2
v1: github.com/charmbracelet/lipgloss     → v2: charm.land/lipgloss/v2
v1: github.com/charmbracelet/bubbles/*    → v2: charm.land/bubbles/v2/*
```

**`View()` returns `tea.View`, not `string`:**
```go
// v1 — WRONG for this project:
func (m Model) View() string { return content }

// v2 — CORRECT:
func (m Model) View() tea.View {
    v := tea.NewView(content)
    v.AltScreen = true  // replaces tea.WithAltScreen() program option
    return v
}
```
Only the top-level model must return `tea.View`. Child components (ItemList, DiffView,
TodoList, StatusBar) can still return `string` from their own View methods.

**`tea.KeyMsg` → `tea.KeyPressMsg`:**
```go
// v1 — WRONG:
case tea.KeyMsg:
    switch msg.Type {
    case tea.KeyEnter: …
    case tea.KeyRune:
        switch msg.Runes[0] { case 'q': … }
    }

// v2 — CORRECT:
case tea.KeyPressMsg:
    switch msg.String() {
    case "enter": …
    case "q": …
    case "space": …      // was " " in v1
    case "ctrl+c": …     // was tea.KeyCtrlC in v1
    case "ctrl+k": …     // modifier+key via msg.String()
    }
```

**Bubbles v2 — width/height are methods, not fields:**
```go
// v1 — WRONG:
m.list.Width = 40
m.viewport.Height = 20

// v2 — CORRECT:
m.list.SetWidth(40)
m.viewport.SetHeight(20)
w := m.list.Width()   // getter is now a method too
```

**AltScreen, mouse mode, keyboard enhancements are declarative:**
Set `v.AltScreen = true`, `v.MouseMode = tea.MouseModeCellMotion`, etc. on the
`tea.View` returned from `View()`. Do **not** pass `tea.WithAltScreen()` or similar
to `tea.NewProgram()`.

**`tea.ExecProcess` is unchanged** — same API in v2 for `$EDITOR` handoff.

---

## Go best practices — enforced throughout

**Minimum Go version: 1.26.** `go.mod` must declare `go 1.26`. Use language and
stdlib features from 1.26 and below freely; do not use build tags or version guards
for features that are standard in 1.26.

---

### Code style
- `gofmt` + `goimports` before every commit.
- Run `go fix ./...` periodically to auto-apply modernizer rewrites (`interface{}` → `any`,
  manual loops → `slices.Contains`, `errors.As` → `errors.AsType`, add `omitzero` tags, etc.).
  The rebuilt `go fix` (1.26) includes ~24 modernizer analyzers. Use `-diff` to preview.
- The `pprof` web UI now defaults to **flame graph view** (1.26). When profiling
  benchmarks, `go tool pprof -http=: cpu.prof` opens directly to the flame graph.
- `golangci-lint` with `.golangci.yml` below — zero warnings required.
- Package names: lowercase single words (`git`, `tui`, `config`, `commands`).
- Every exported identifier has a godoc comment starting with its name.

---

### Standard library — prefer new packages over hand-rolled equivalents

**`slices` (1.21+)** — use for all slice operations instead of manual loops:
```go
// sorting
slices.SortFunc(entries, func(a, b TodoEntry) int {
    return cmp.Compare(a.Subject, b.Subject)
})

// searching / membership
if slices.Contains(args, "--root") { … }
idx := slices.Index(hunks, target)

// filtering (returns new slice)
changed := slices.DeleteFunc(entries, func(e TodoEntry) bool {
    return e.Action == ActionDrop
})
```

**`maps` (1.21+)** — use for map operations:
```go
keys   := maps.Keys(m)          // unordered []K
clone  := maps.Clone(original)  // shallow copy
maps.DeleteFunc(m, func(k string, v int) bool { … })
```

**`cmp` (1.21+)** — use for comparisons and fallback chains:
```go
// three-way comparison (replaces manual if/else)
cmp.Compare(a.Hash, b.Hash)

// first non-zero value (replaces verbose OR chains)
path := cmp.Or(os.Getenv("GT_DELTA_PATH"), "delta")
```

**`iter` (1.23+)** — expose collection iterators using `iter.Seq` and `iter.Seq2`:
```go
// in internal/git/rebase_todo.go
type TodoEntries []TodoEntry

func (entries TodoEntries) All() iter.Seq2[int, TodoEntry] {
    return func(yield func(int, TodoEntry) bool) {
        for i, e := range entries {
            if !yield(i, e) { return }
        }
    }
}
// caller:
for i, entry := range entries.All() { … }
```

**Range over integers (1.22+)**:
```go
for i := range 5 { … }   // replaces for i := 0; i < 5; i++
```

**`min` / `max` builtins (1.21+)**:
```go
leftWidth  := max(28, termWidth * 30 / 100)
rightWidth := min(termWidth - leftWidth, termWidth)
```

**`errors.Join` (1.20+)** — combine multiple errors without a library:
```go
return errors.Join(ErrParseFailed, ErrEmptyInput)
```

**`bytes.Buffer.Peek` (1.26+)** — read ahead without consuming. Useful in diff
parsing where you need to inspect the next line before deciding the parse path:
```go
next := buf.Peek(4) // look ahead at "+++ " / "--- " / "@@ -" without advancing
```

**`io.ReadAll` (1.26+ performance)** — now ~2× faster with ~50% less memory allocation,
returning a minimally-sized slice. Since `Runner.Run` reads git command output via
`io.ReadAll`, this is a free performance win on upgrade — no code changes needed.

**`fmt.Errorf` (1.26+ performance)** — when the format string contains no verbs (e.g.
`fmt.Errorf("nothing staged")`), it now matches `errors.New` in allocation cost. For
wrapping with `%w`, continue using `fmt.Errorf`. For fixed messages, prefer
`errors.New` for clarity; either is acceptable performance-wise.

**`log/slog` (1.21+)** — structured logging; never use `fmt.Println` for diagnostics:
```go
slog.Debug("runner.Run", "args", args, "output", out)
slog.Error("git apply failed", "err", err, "patch", patch)
```
Log level controlled by `GT_LOG_LEVEL` env var (`debug`/`info`/`warn`/`error`).
When multiple log sinks are needed (e.g. file + stderr), use `slog.NewMultiHandler`
(1.26+) instead of writing custom fan-out handlers:
```go
h := slog.NewMultiHandler(stderrHandler, fileHandler)
slog.SetDefault(slog.New(h))
```

**`os.Root` (1.24+)** — use in `testhelper/gitrepo.go` for all temp-repo file I/O to
prevent path traversal bugs in test helpers:
```go
root, err := os.OpenRoot(repoPath)
t.Cleanup(func() { root.Close() })
f, err := root.Create("driver.c")
```

**`os/signal.NotifyContext` (1.26+ improvement)** — the context cancellation error now
indicates which signal was received. Use when the TUI needs signal-aware shutdown:
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()
// ctx.Err() after signal includes the signal name
```

**Generic type aliases (1.24+)** — use when a concrete alias improves readability
without introducing a new type hierarchy:
```go
// in internal/tui/components — alias the bubbletea message type for clarity
type Cmd = tea.Cmd
```

**`new(expr)` pointer initialization (1.26+)** — use to create pointer values in a
single expression. Eliminates the two-statement pattern for optional pointer fields:
```go
// Before Go 1.26 — two statements:
v := int64(300)
ptr := &v

// Go 1.26 — single expression:
ptr := new(int64(300))

// Primary use case: optional pointer fields in config structs
type Config struct {
    DiffContext *int    `toml:"diff_context,omitempty"`
    LogDepth   *int    `toml:"log_depth,omitempty"`
}
cfg := Config{
    DiffContext: new(3),
    LogDepth:    new(30),
}
```
Composite literals (`new([]int{1,2,3})`), function calls (`new(f())`), and booleans
(`new(true)`) all work. Passing `nil` is disallowed. Prefer `new(expr)` over the
two-statement `v := expr; ptr := &v` pattern everywhere in this codebase. The Config
example above is illustrative — the project's actual Config struct is defined in the
Config section; do not change its field types without being asked.

---

### Error handling
- Never discard errors with `_` unless the call is provably infallible.
- Use `errors.Is` for sentinel checks — never string matching.
- **Prefer `errors.AsType[E]` (1.26+) over `errors.As`** — it is type-safe,
  reflection-free, ~3× faster, and catches type mismatches at compile time:
  ```go
  // Go 1.26 — preferred:
  if target, ok := errors.AsType[*AppError](err); ok {
      slog.Error("app error", "code", target.Code)
  }

  // Legacy — avoid in new code:
  // var target *AppError
  // if errors.As(err, &target) { … }
  ```
- Sentinel errors as package-level vars: `var ErrNoRepo = errors.New("not a git repository")`.
- Wrap with `%w` at every package boundary: `fmt.Errorf("ParseHunks: %w", err)`.
- Use `errors.Join` when a function accumulates multiple independent errors.

---

### Interfaces and dependency injection
- Interfaces defined at the consumer (commands), not the producer (git package).
- `Runner` is the only interface crossing the git/commands boundary:

```go
// Runner executes git commands and returns stdout. Stderr is captured
// separately for error reporting (e.g. status bar messages on failure).
type Runner interface {
    // Run executes a git command and returns stdout. On non-zero exit, the error
    // wraps stderr content: errors.As(err, &ExecError{}) to access Stderr field.
    Run(ctx context.Context, args ...string) (string, error)
    // RunWithEnv prepends extra env vars to os.Environ() before exec.
    RunWithEnv(ctx context.Context, env []string, args ...string) (string, error)
    // RunWithStdin pipes data to git's stdin. Used for git apply --cached -.
    RunWithStdin(ctx context.Context, stdin string, args ...string) (string, error)
}

// ExecError wraps a non-zero exit from git with captured stderr.
type ExecError struct {
    Args     []string
    ExitCode int
    Stderr   string
}
func (e *ExecError) Error() string { return fmt.Sprintf("git %s: exit %d: %s", e.Args[0], e.ExitCode, e.Stderr) }
```

- `Renderer` is the only interface in the `diff` package boundary:

```go
// Renderer takes raw unified diff text and returns ANSI-colored output.
type Renderer interface {
    Render(rawDiff string) (string, error)
}
```

- Command models receive `Runner` + `Config` + `Renderer` at construction, never
  via globals. The `AppModel` holds all three and passes them to command constructors.
- Commands emit `PushModelMsg` / `PopModelMsg` to navigate; they never import other
  command packages directly. This keeps the dependency graph acyclic:
  `app` → `commands/*`, `commands/*` → `git`, `tui`, `diff`. No `commands` → `commands`.

---

### Concurrency
- `tea.Cmd` only inside bubbletea models. No raw goroutines, channels, or `sync.Mutex`
  outside test helpers.
- `context.Context` is threaded through every `Runner` call so the caller can cancel
  long-running git operations (e.g. `git log` on a large repo).

### Application model — navigation stack

All 8 commands share a single `tea.Program` managed by an `AppModel` root model.
The `AppModel` owns a **model stack** for command-to-command transitions (e.g.
`log` → `fixup` → return to `log`), plus shared dependencies injected once at
startup.

```go
// cmd/jig/main.go constructs AppModel and runs a single tea.Program.

// internal/app/app.go
type AppModel struct {
    stack    []tea.Model     // push on sub-command entry, pop on quit
    runner   git.Runner      // shared across all commands
    config   config.Config   // loaded once at startup
    renderer diff.Renderer   // diff.Chain(cfg) called once at startup
}

// Push replaces the active model. The previous model is preserved on the stack.
func (a *AppModel) Push(m tea.Model) { a.stack = append(a.stack, m) }

// Pop removes the active model and returns to the previous one.
// If the stack has only one model, Pop quits the program.
// The popped element is nil'd to allow GC of the child model.
func (a *AppModel) Pop() tea.Model {
    if len(a.stack) <= 1 { return nil /* quit */ }
    a.stack[len(a.stack)-1] = nil // allow GC of popped model
    a.stack = a.stack[:len(a.stack)-1]
    return a.stack[len(a.stack)-1]
}

// Active returns the model at the top of the stack.
func (a *AppModel) Active() tea.Model { return a.stack[len(a.stack)-1] }
```

**Transition messages** (emitted by commands, handled by `AppModel.Update`):

```go
// PushModelMsg tells AppModel to push a new command model onto the stack.
type PushModelMsg struct { Model tea.Model }

// PopModelMsg tells AppModel to pop the current model and return to the parent.
// MutatedGit indicates whether the child command changed git state (triggers
// a refresh in the parent model via RefreshMsg).
type PopModelMsg struct { MutatedGit bool }

// RefreshMsg is sent to the parent model after a mutating child returns.
// The parent re-fetches its data source (git status, git log, etc.).
type RefreshMsg struct{}
```

**Transition rules:**
- `add` → `<enter>` on file → `PushModelMsg{hunk-add model}`
- `hunk-add` → `<esc>` → `PopModelMsg{MutatedGit: true}` (staging may have changed)
- `log` → `F` → `PushModelMsg{fixup model}` → fixup quits → `PopModelMsg{MutatedGit: true}`
- `log` → `R` → `PushModelMsg{rebase model}` → rebase quits → `PopModelMsg{MutatedGit: true}`
- `log` → `D` → `PushModelMsg{diff model}` → diff quits → `PopModelMsg{MutatedGit: false}`
- Top-level `q` with stack depth 1 → `tea.Quit`

**Refresh-on-return:** when `AppModel` receives `PopModelMsg{MutatedGit: true}`,
it sends `RefreshMsg` to the newly active parent model. The parent re-fetches its
data source (`git log`, `git status`, etc.) and repositions the cursor to the
nearest surviving item. Read-only children (like `diff`) return `MutatedGit: false`
and the parent skips the refresh.

**Shared dependencies:** `Runner`, `Config`, and `Renderer` are created once in
`main.go` and passed to `AppModel`. Each command model receives them at construction
via a factory function:

```go
// Every command constructor follows this pattern:
func NewAddModel(runner git.Runner, cfg config.Config, renderer diff.Renderer) AddModel
func NewLogModel(runner git.Runner, cfg config.Config, renderer diff.Renderer) LogModel
// etc.
```

**Update dispatch:** `AppModel.Update` routes all messages through the active model
first, then intercepts navigation messages from the result:

```go
func (a *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case PushModelMsg:
        a.Push(msg.Model)
        return a, msg.Model.Init()
    case PopModelMsg:
        parent := a.Pop()
        if parent == nil {
            return a, tea.Quit
        }
        if msg.MutatedGit {
            return a, func() tea.Msg { return RefreshMsg{} }
        }
        return a, nil
    case tea.WindowSizeMsg:
        // Forward to active model (it needs the size for layout)
        active, cmd := a.Active().Update(msg)
        a.stack[len(a.stack)-1] = active
        return a, cmd
    default:
        // All other messages (key presses, custom msgs) go to active model
        active, cmd := a.Active().Update(msg)
        a.stack[len(a.stack)-1] = active
        return a, cmd
    }
}
```

**Quit vs pop:** commands ALWAYS emit `PopModelMsg` on `q`/`<esc>`, never `tea.Quit`
directly. `AppModel` converts `PopModelMsg` at stack depth 1 into `tea.Quit`. This
means `q` in a top-level command quits the app, but `q` in a sub-command (e.g. fixup
launched from log) returns to the parent. Commands don't need to know their stack depth.

Similarly, when fixup succeeds it emits `PopModelMsg{MutatedGit: true}`, not
`tea.Quit`. If fixup was launched standalone (stack depth 1), AppModel quits. If
launched from log (stack depth 2), AppModel pops back to log and sends RefreshMsg.

**View delegation:** `AppModel.View()` delegates to `Active().View()` for content,
then wraps it in `tea.NewView()` with `AltScreen` and `KeyboardEnhancements`:

```go
func (a *AppModel) View() tea.View {
    content := a.Active().View() // child returns string
    v := tea.NewView(content)
    v.AltScreen = true
    return v
}
```

Child command models return `string` from their `View()` methods, not `tea.View`.
Only `AppModel` returns `tea.View`. This keeps child models testable without a
full `tea.Program`.

---

### Diff renderer lifecycle

The diff rendering chain (`delta` → `chroma` → `plain`) is resolved **once at
startup** in `main.go` by calling `diff.Chain(cfg)`. The resulting `diff.Renderer`
is passed to `AppModel` and from there to every command constructor. Commands never
call `diff.Chain()` themselves.

```go
// main.go
renderer := diff.Chain(cfg)  // resolved once
app := app.NewAppModel(runner, cfg, renderer)
```

The `DiffView` component receives rendered content (a `string` with ANSI codes),
not a `Renderer`. The command model calls `renderer.Render(rawDiff)` and passes
the result to `DiffView.SetContent(rendered)`.

---

### Shared utilities (`internal/git/`)

**`preconditions.go`** — reusable git state checks shared across commands:

```go
// IsRebaseInProgress returns true if .git/rebase-merge or .git/rebase-apply exists.
func IsRebaseInProgress(ctx context.Context, r Runner) bool

// HasStagedChanges returns true if git diff --cached --quiet exits non-zero.
func HasStagedChanges(ctx context.Context, r Runner) bool

// HasCommits returns true if git rev-parse HEAD succeeds.
func HasCommits(ctx context.Context, r Runner) bool

// IsMergeInProgress returns true if .git/MERGE_HEAD exists.
func IsMergeInProgress(ctx context.Context, r Runner) bool
```

**`editor.go`** — shared editor resolution chain (used by hunk-add, rebase, log):

```go
// ResolveEditor returns the path to the user's preferred editor by checking:
// $GIT_EDITOR → git config core.editor → $VISUAL → $EDITOR → "vi"
func ResolveEditor(ctx context.Context, r Runner) string
```

---

### Runtime — free performance wins in Go 1.26
- The **Green Tea GC** is now default — 10–40% reduction in GC overhead for
  allocation-heavy programs. Do not set `GOGC` or `GOMEMLIMIT` unless profiling shows
  a specific need. Opt-out escape hatch: `GOEXPERIMENT=nogreenteagc` (removed in 1.27).
- **Cgo call overhead dropped ~30%** and **small-object allocation ≤512 bytes is up to
  30% faster**. Both benefit the frequent `os/exec` calls in `Runner`.
- The compiler **places slice backing stores on the stack** in more situations — avoid
  defeating this by passing slice pointers to non-inlineable functions unnecessarily.

---

## Repository layout

```
cmd/jig/main.go

internal/
  app/
    app.go                 # AppModel: root model, navigation stack, shared deps
    app_test.go

  git/
    runner.go              # Runner interface + ExecRunner implementation
    runner_test.go
    repo.go                # Runner calls: rev-parse for repo root and branch name
    repo_test.go
    status.go              # ParseStatus() -> []StatusEntry; StatusEntry struct
    status_test.go
    diff.go                # ParseFileDiff() -> []FileDiff   (for diff, add preview)
    diff_test.go
    hunk.go                # ParseHunks() -> []Hunk; BuildPatch() -> string
    hunk_test.go
    log.go                 # ParseLog() -> []Commit; Commit struct (for fixup, reset, rebase, log)
    log_test.go
    preconditions.go       # IsRebaseInProgress, HasStagedChanges, HasCommits
    preconditions_test.go
    editor.go              # ResolveEditor: $GIT_EDITOR → core.editor → $VISUAL → $EDITOR → vi
    editor_test.go
    rebase_todo.go         # ParseTodo() -> []TodoEntry; WriteTodo() -> string
    rebase_todo_test.go

  diff/
    renderer.go            # Renderer interface + Chain(cfg) -> Renderer
    renderer_test.go
    delta.go               # DeltaRenderer  (skip test if delta not in PATH)
    delta_test.go
    chroma.go              # ChromaRenderer (always available)
    chroma_test.go
    plain.go               # PlainRenderer  (fallback)
    plain_test.go

  tui/
    styles.go              # Theme struct, lipgloss vars (dark/light)
    styles_test.go
    layout.go              # Columns() helper: computes left/right widths from WindowSizeMsg
    layout_test.go
    components/
      itemlist.go          # ItemList: generic bubbles/list wrapper (files, commits, hunks)
      itemlist_test.go
      hunkview.go          # HunkView: full-width line-level diff with toggle state
      hunkview_test.go
      diffview.go          # DiffView: right-panel viewport with rendered diff content
      diffview_test.go
      todolist.go          # TodoList: reorderable list with action cycling
      todolist_test.go
      logview.go           # LogView: commit list with detail-level toggling
      logview_test.go
      statusbar.go         # StatusBar: keyhints left, branch+mode right
      helpoverlay.go       # HelpOverlay: modal keybinding reference (shared)
      statusbar_test.go

  commands/
    add.go                 # ga  — interactive file staging
    add_test.go
    hunk_add.go            # gha — line/hunk-level staging
    hunk_add_test.go
    checkout.go            # gc  — file restore (git restore <file>)
    checkout_test.go
    diff.go                # gd  — diffnav-style diff viewer
    diff_test.go
    fixup.go               # gfix — stage → fixup → autosquash
    fixup_test.go
    rebase_interactive.go  # gri — visual todo editor / GIT_SEQUENCE_EDITOR
    rebase_interactive_test.go
    reset.go               # gr  — interactive reset (soft/mixed/hard)
    reset_test.go
    log.go                 # gl  — visual commit browser (3 detail levels)
    log_test.go

  config/
    config.go
    config_test.go

internal/testhelper/
  gitrepo.go              # NewTempRepo, AddCommit, WriteFile, StageFile
  fakerunner.go           # FakeRunner: records calls, returns scripted output

tests/
  integration/
    add_test.go            # build tag: integration
    checkout_test.go
    hunk_add_test.go
    fixup_test.go
    rebase_interactive_test.go
    reset_test.go
    log_test.go
    workflow_test.go       # cross-command workflow tests (AppModel stack)

shell/
  jig.plugin.fish

Makefile
.golangci.yml
.pre-commit-config.yaml
.goreleaser.yml
LICENSE                  # MIT
README.md                # written in Phase 9 before tagging v0.1.0
go.mod
go.sum
```

---

## Tech stack

```
charm.land/bubbletea/v2            TUI event loop (v2 — Cursed Renderer, declarative View)
charm.land/lipgloss/v2             Styling and layout (v2 — pure, no I/O, auto color downsampling)
charm.land/bubbles/v2              list, viewport, textinput, spinner (v2)
github.com/sahilm/fuzzy            Fuzzy filtering for file lists
alecthomas/chroma/v2               Native diff syntax highlighting
bluekeyes/go-gitdiff               Unified diff parser (used by hunk.go and chroma renderer)
spf13/cobra                        CLI subcommands
spf13/viper                        Config: env vars + ~/.config/jig/config.toml
```

**Import conventions:**
```go
import (
    tea "charm.land/bubbletea/v2"
    "charm.land/bubbles/v2/list"
    "charm.land/bubbles/v2/viewport"
    "charm.land/bubbles/v2/textinput"
    "charm.land/bubbles/v2/key"
    "charm.land/lipgloss/v2"
)
```
Do **not** use the legacy `github.com/charmbracelet/*` import paths — those are v1.

---

## Command specifications

### `add` (alias `ga`) — interactive file staging ([reference: forgit add](https://github.com/wfxr/forgit))

Accepts optional path arguments to scope the file list: `jig add src/` shows only
files under `src/`. Without arguments, shows all changed files in the repo.

Left panel: files from `git status --short`, grouped as unstaged then untracked.
Each entry shows filename + `+N -N` stat from `git diff --stat`.
Right panel: `git diff <file>` (unstaged) or `git diff --cached <file>` (staged),
rendered via `diff.Chain(cfg)`.

```
+------------------------+-----------------------------------------------+
| Staged (1)             |  diff --git a/driver.c b/driver.c             |
|  probe.c      +2  -1  |  @@ -42,7 +42,10 @@                          |
| Unstaged (2)           |  ...                                          |
|  driver.c     +8  -3  |                                               |
| >  mt7927.c   +12      |                                               |
| Untracked (1)          |                                               |
|  new_feature.c        |                                               |
+------------------------+-----------------------------------------------+
| ␣ toggle  / filter  a all  y yank  ⏎ hunk-add  q quit  [ main]     |
```

The left panel groups files in three sections: **Staged** (index changes, shown
first so the user sees what's already prepared), **Unstaged** (working tree changes),
and **Untracked** (new files). Sections with zero items are hidden. `<space>` toggles
a file between sections; `<u>` unstages explicitly.

**Partially staged files** (git status shows `MM`, `MD`, etc.): a file with changes
in both the index and the working tree appears in **both** Staged and Unstaged
sections simultaneously. Each entry is a distinct `StatusEntry` — the Staged entry
shows `git diff --cached` in the right panel, the Unstaged entry shows `git diff`.
`<space>` on the Staged entry unstages it; `<space>` on the Unstaged entry stages the
remaining working tree changes. The `` icon for the Staged entry gets a small
`·` suffix (e.g. `·`) to indicate partial staging.

Keybindings:
- `j`/`k` or `↑`/`↓`: move file cursor
- `Home` / `End`: jump to first / last file
- `/`: open fuzzy filter input (filters across all sections by filename)
- `<space>`: **toggle staging.** If file is in Unstaged or Untracked section →
  `git add <file>` (stages it). If file is in Staged section → `git reset HEAD <file>`
  (unstages it). The file moves between sections accordingly. This makes `<space>`
  a universal toggle regardless of which section the cursor is in.
  List re-fetches from `git status` after each toggle.
- `<u>`: unstage selected file → `git reset HEAD <file>`. File moves back to "Unstaged".
  No-op on untracked files and files that are not staged.
- `<a>`: stage all → `git add -A`. All files move to "Staged". If nothing remains
  unstaged, show " all files staged" and keep the view open for review.
- `<enter>`: descend into `hunk-add` for selected file (state machine, not subprocess).
  Only available on modified files (not untracked — untracked files have no hunks).
- `{` / `}`: jump to previous / next section header (Unstaged / Untracked / Staged).
- `y`: copy file path to clipboard.
- `q` / `<esc>`: emit `PopModelMsg` (quits app at top level, returns to parent if stacked)

---

### `hunk-add` (alias `gha`) — line/hunk-level staging

Replaces `git add -p`. Can be launched standalone or entered from `add` via `<enter>`.

**Standalone mode:** left panel shows changed files (same source as `add`). Select a
file with `<enter>` to descend into HunkView. `<esc>` from HunkView returns to file list.
`q` or `<esc>` from the file list itself emits `PopModelMsg{MutatedGit: staged}` (where
`staged` is true if any hunks were staged during this session).

**Hunk editing mode:** left panel is replaced by `HunkView` (full left column height).
Right panel shows the same diff rendered for context (the full file diff via the
renderer, scrolled to match the left panel’s cursor position). When the cursor moves
in the left panel, the right panel auto-scrolls to keep the same hunk visible — this
lets the user see the rendered diff alongside the toggle state. The HunkView renders
each line of the selected file’s diff with toggle state:

```
+------------------------+-----------------------------------------------+
|  @@ -42,7 +42,10 @@   |  diff --git a/driver.c b/driver.c             |
|   int ret;             |  @@ -42,7 +42,10 @@                          |
| - old_init_sequence(); |   int ret;                                    |
|+ new_init_a();        |  -old_init_sequence();                        |  <- selected
|+ new_init_b();        |  +new_init_a();                               |  <- selected
|   return ret;          |  +new_init_b();                               |
|  @@ -88,4 +88,4 @@    |   return ret;                                 |
| - legacy_probe();     |                                               |  <- unselected
| + modern_probe();     |                                               |  <- unselected
+------------------------+-----------------------------------------------+
| ␣ toggle  a hunk  A all  s split  u undo  e edit  ⏎ stage  q back   |
| 3 lines in 2 hunks selected                                          |
```

**Initial selection state:** when entering hunk editing mode, **all togglable lines
start unselected** (``). The user opts in to exactly which lines to stage. This is
the opposite of `git add -p`’s "accept whole hunk" default and matches the mental
model of building a patch from scratch.

**Toggle semantics for `+` and `-` lines:**
- `+` line **selected** (): the addition will be staged (line appears in the index).
- `+` line **unselected** (): the addition stays in the working tree only.
- `-` line **selected** (): the deletion will be staged (line is removed from the index).
- `-` line **unselected** (): the deletion stays in the working tree; the line remains
  in the index. In the patch, this `-` line is **demoted to context** (` `).

**Cursor-to-hunk mapping:** the cursor is always on a specific line. The "current hunk"
is the hunk whose `@@` header is the most recent one above (or at) the cursor position.
Actions like `<a>` (toggle hunk), `e` (edit), `s` (split), and `u` (undo) operate on
the current hunk as determined by cursor position.

Keybindings (hunk editing mode):
- `j`/`k` or `↑`/`↓`: move cursor between lines
- `Home` / `End`: jump to first / last line in file
- `n`/`N`: jump to next/previous `@@` hunk header
- `<space>`: toggle selected line. If cursor is on a non-togglable line (context or
  `@@` header), jump to the next togglable line below.
- `<a>`: toggle all togglable lines in current hunk
- `<A>`: toggle all togglable lines in file
- `s`: **split current hunk** at the first run of ≥1 context lines within it.
  If the hunk has no internal context (all contiguous `+`/`-` lines), `s` is a no-op
  and the status bar shows "cannot split — no context boundary". After splitting,
  the hunk list updates in place and the cursor stays on the first line of the new
  second hunk. The user can also press `e` to open the hunk in `$EDITOR` for manual
  splitting with full control over line content.
- `u`: **undo** — restore the previous selection state of the current hunk (one level).
  Captures a snapshot before each `<space>`, `<a>`, `<A>`, or `s` action. Pressing `u`
  again after an undo re-does (toggle between last two states). Only the current hunk’s
  snapshot is affected; other hunks are untouched.

  Undo data structure (per hunk): a `prevSelected []bool` array, one entry per
  `Line` in the hunk. On each undoable action, swap `line[i].Selected` with
  `prevSelected[i]` for all lines in the hunk. This is O(n) where n = lines in
  the hunk (typically <100). The swap operation makes `u` a self-inverting toggle:
  press once to undo, press again to redo, with zero additional bookkeeping.
- `e`: open current hunk in `$EDITOR` (see editor flow below)
- `<enter>`: **stage all hunks that have any selected lines**, applied sequentially
  from top to bottom. Hunks with no selected lines are skipped. After staging, the
  hunk list is re-fetched from `git diff` so line numbers reflect the new state.
  If no lines are selected anywhere, show `ErrNothingSelected` in the status bar.
- `q` / `<esc>`: return to file list (discards unsaved toggle selections, but hunks
  already staged via `<enter>` are persisted in the index). In standalone mode, emits
  `PopModelMsg{MutatedGit: staged}` where `staged` tracks whether any
  `git apply` call succeeded during this session. When entered from `add` via
  `<enter>`, returns to the add file list (pops the stack).

**Selection counter:** the status bar shows a live count of selected lines and
hunks: "N lines in M hunks selected" in `ColorFgSubtle` *italic*. When nothing is
selected, this area is blank. When the user presses `<enter>`, this count tells them
exactly what will be staged without scrolling through every hunk to verify.

Line state visual encoding:
- `` = selected (will be staged); `ColorGreen`, normal weight
- `` = unselected; `ColorFgSubtle`, *italic*
- Context lines (no prefix): always included in patch; not togglable; *italic* muted
- `@@` hunk headers: **bold** `ColorCyan`; not togglable

**Patch construction (`internal/git/hunk.go`):**

```go
type Hunk struct {
    FileHeader string // "diff --git …\nindex …\n--- …\n+++ …\n" (only on Hunk[0])
    Header     string // "@@ -a,b +c,d @@ optional-context" verbatim from source
    OldStart   int    // parsed from Header: the 'a' in @@ -a,b
    NewStart   int    // parsed from Header: the 'c' in @@ +c,d
    Lines      []Line
}

type Line struct {
    Op       rune   // '+', '-', or ' '
    Content  string // text without op prefix, no trailing newline
    Selected bool   // meaningful only for '+' and '-' lines
}

// ParseHunks parses raw unified diff for one file into structured hunks.
// The file header block (diff --git, index, ---, +++) is stored on Hunk[0].FileHeader
// verbatim so BuildPatch can emit a complete, self-contained patch.
// OldStart and NewStart are parsed from each @@ header.
func ParseHunks(rawDiff string) ([]Hunk, error) {}

// SplitHunks splits a multi-hunk diff into individual single-hunk patches, one per
// Hunk returned. Each returned Hunk carries the original FileHeader on index 0.
// The caller always works on slices of individual hunks — multi-hunk files are
// never passed as a unit to BuildPatch.
func SplitHunks(h []Hunk) []Hunk {}

// SplitHunkAt splits a single Hunk into two hunks at the first run of ≥1 context
// lines found after any '+' or '-' line.
//
// Algorithm (single-pass, O(n) where n = len(h.Lines)):
//   1. Scan lines left to right. Track whether we've seen any '+' or '-' line.
//   2. After seeing a changed line, the first ' ' (context) line is the split point.
//   3. Partition h.Lines into first[:splitIdx] and second[splitIdx:].
//   4. Both halves inherit h.FileHeader. The first hunk keeps h.OldStart/NewStart.
//      The second hunk's OldStart = h.OldStart + (context lines + '-' lines in first).
//      The second hunk's NewStart = h.NewStart + (context lines + '+' lines in first).
//   5. Call RecalculateHeader on both halves.
//
// Returns (first, second, true) if a split point was found, or
// (original, Hunk{}, false) if the hunk cannot be split (all changed lines are
// contiguous with no internal context boundary).
// This implements the 's' key in HunkView (equivalent to git add -p's split).
func SplitHunkAt(h Hunk) (Hunk, Hunk, bool) {}

// RecalculateHeader recomputes the @@ header for h based on its actual Lines slice.
// It preserves OldStart and NewStart from the Hunk struct (these are set during
// parsing or splitting and must not change). Only old_count and new_count are
// recomputed from the line content:
//
//   old_count = count of context lines + count of '-' lines
//   new_count = count of context lines + count of '+' lines
//
// The trailing optional context string (e.g. " @@ func foo()") is preserved unchanged.
// Call this after any $EDITOR round-trip that may have added or removed lines.
func RecalculateHeader(h *Hunk) {}

// BuildPatch assembles a valid unified diff from a single Hunk (already split
// by SplitHunks). Single-pass O(n) algorithm:
//   1. Count selected togglable lines. If zero, return ErrNothingSelected.
//   2. Emit h.FileHeader (if non-empty).
//   3. Emit h.Header (the @@ line — must reflect current counts; call
//      RecalculateHeader first if lines were added/removed by $EDITOR).
//   4. For each line:
//      - Selected '+': emit as "+<content>"
//      - Unselected '+': **drop** (line stays in working tree only)
//      - Selected '-': emit as "-<content>" (deletion staged)
//      - Unselected '-': emit as " <content>" (**demote to context** — line
//        stays in index, deletion not staged)
//      - Context ' ': emit as " <content>" (always included)
//   5. Ensure output ends with newline (git apply requires it).
//
// The caller must call RecalculateHeader before BuildPatch whenever lines have
// been added or removed (e.g. after $EDITOR editing), because the @@ counts
// must match the actual emitted lines or git apply will reject the patch.
func BuildPatch(h Hunk) (string, error) {}

var ErrNothingSelected = errors.New("no lines selected for staging")
```

**`$EDITOR` editing flow (the `e` key in HunkView):**

`git add -p` lets the user hand-edit a hunk before staging. `jig hunk-add` replicates
this with better error handling. The `e` key operates on the **current hunk** (determined
by cursor position). When pressed:

```
1. Snapshot the current hunk state (for undo on failure).
2. Write the current hunk (single hunk, with FileHeader) to a temp file.
3. Annotate the temp file with comment lines (lines starting with '#') that explain
   which lines can be removed and which cannot, mirroring git's own helper text.
4. Resolve editor via `git.ResolveEditor(ctx, runner)` (shared utility, see
   `internal/git/editor.go`). Open the temp file with
   `tea.ExecProcess(exec.Command(editor, tempPath), ...)`.
5. On editor exit:
   a. If editor exits non-zero: discard the edit, restore the snapshot from step 1,
      show "editor exited with error" in status bar. Do not stage anything.
   b. Read the file back. Strip all comment lines (prefix '#').
   c. If the result is empty (user deleted everything): restore snapshot, show
      "empty edit discarded" in status bar.
   d. Re-parse into a Hunk via ParseHunks — the user may have added, removed,
      or changed '+'/'-' lines freely.
   e. Call RecalculateHeader on the resulting Hunk to recompute the @@ counts
      (OldStart/NewStart are preserved from the original hunk).
   f. Replace the current hunk in HunkView with the edited version. The user can
      review the result before staging with `<enter>`.
```

Note: the editor flow does **not** auto-stage. The user presses `e` to edit, reviews
the result in HunkView, then presses `<enter>` to stage when satisfied. This is
different from git add -p where `e` edits and immediately applies.

**Batch multi-hunk staging (the `<enter>` key):**

When the user presses `<enter>`, **all hunks with at least one selected line** are
staged sequentially from top to bottom. Each hunk is staged as a separate
`git apply --cached -` call (one per hunk, not batched). This avoids header offset
interdependencies — each call is self-contained because each Hunk carries its own
`OldStart`/`NewStart` from parsing.

After all selected hunks are staged, the hunk list in HunkView is **re-fetched from
`git diff`** so line numbers always reflect the current state. If the re-fetch returns
an empty diff (all hunks were staged), automatically return to the file list — the
file has been fully staged. If launched for a single file from `add`, emit
`PopModelMsg{MutatedGit: true}` to return to the add file list.

If any individual `git apply` fails mid-batch:
- Show the error in the status bar: "apply failed at hunk N: <error>".
- Stop the batch — do not continue with remaining hunks.
- Re-fetch the hunk list so the user sees the partially staged state.
- The user can fix and retry.

Execution: pipe `BuildPatch(hunk)` output to `git apply --cached -` via `RunWithStdin`.

---

### `checkout` (alias `gc`) — file checkout (restore working tree files)

Single-mode command for restoring modified working tree files to their index state.
Uses `git restore` (Git 2.23+), the modern replacement for `git checkout -- <file>`.

**Precondition:** at least one modified file exists (`git diff --name-only` non-empty).
If no files are modified, show status bar message "working tree clean" and emit
`PopModelMsg{MutatedGit: false}` (quits at top level).

Left panel: `git diff --name-only` (unstaged modified files), each with `+N -N`
stat and color-coded status icon. This matches `git restore` semantics: both
operate on the working tree vs index boundary.
Right panel: `git diff -- <file>` via `diff.Chain(cfg)` (unstaged changes for selected file).

```
+------------------------+-----------------------------------------------+
| Modified files (3)     |  diff --git a/driver.c b/driver.c             |
|  driver.c    +8  -3   |  @@ -42,7 +42,10 @@                          |
| >  mt7927.c  +12      |  ...                                          |
|  probe.c     +2  -1   |                                               |
+------------------------+-----------------------------------------------+
| j/k navigate   filter  D restore  q quit           [ main] [checkout] |
```

Keybindings:
- `j`/`k` or `↑`/`↓`: move file cursor
- `Home` / `End`: jump to first / last file
- `/`: open fuzzy filter input
- `<D>` (uppercase): `git restore <file>` (destructive — lowercase `<d>` is a no-op).
  File disappears from list after restore. If last file was restored, show
  " working tree clean" and quit automatically.
- `<A>` (uppercase): restore ALL files in the list (`git restore .`). Since this is
  destructive, requires uppercase key. Status bar shows "⚠ restore all N files?" with
  a 1-second confirmation window — press `<A>` again within 1 second to confirm. If not
  confirmed, show "cancelled" and do nothing. This is the mass-discard shortcut for
  "throw everything away and start clean."
- `y`: copy file path to clipboard.
- `q` / `<esc>`: emit `PopModelMsg` (quits or returns to parent)

---

### `diff` (alias `gd`) — diffnav-style viewer ([reference: diffnav](https://github.com/dlvhdr/diffnav))

Accepts optional path arguments: `jig diff src/` limits the file list to `src/`.

Read-only. Inspired by diffnav: file tree on the left, full diff on the right.
Accepts optional revision argument: `jig diff HEAD~3`, `jig diff main..feature`.
Default: `git diff` (unstaged changes only, matches vanilla git behavior).
`jig diff --staged` shows `git diff --cached` (staged changes only).

Left panel: list of changed files, each with `+N -N` stats and a color-coded status
Nerd Font icon (=modified, =added, =deleted, 󰕓=renamed). Fuzzy-filterable with `/`.

Right panel: full diff of selected file via `diff.Chain(cfg)`. Scrollable.
When delta is available, pass `--width <panel_width>` to avoid the "squished" issue
from forgit (#471).

```
+------------------------+-----------------------------------------------+
| Changed files (4)      |  diff --git a/driver.c b/driver.c             |
|  <filter>             |  index a3f1b2..9c4d5e 100644                  |
|  driver.c    +8  -3   |  --- a/driver.c                               |
| >  mt7927.c  +42 -12  |  +++ b/driver.c                               |
|  new_feat.c  +88      |  @@ -1,6 +1,8 @@                             |
|  legacy.c    -120     |  ...                                          |
+------------------------+-----------------------------------------------+
| j/k navigate   filter  ⏎ focus diff  q quit         [ HEAD]  diff   |
```

Keybindings:
- `j`/`k` or `↑`/`↓`: move file cursor
- `Home` / `End`: jump to first / last file
- `/`: open fuzzy filter input
- `<enter>`: move focus to right panel (scroll mode)
- In right panel: `j`/`k`/`PageDown`/`PageUp` scroll diff; `n`/`N` jump to next/prev hunk header
- `<esc>`: context-dependent — if right panel is focused, return focus to left panel;
  if left panel is focused, emit `PopModelMsg{MutatedGit: false}`.
- `q`: always emit `PopModelMsg{MutatedGit: false}` regardless of panel focus.
- `y`: copy current file path to clipboard

**FileDiff data type** (used by the diff command for non-stdin mode):

```go
// FileDiff represents one changed file from git diff --name-status / --stat.
type FileDiff struct {
    Name     string // relative file path
    OldName  string // original path if renamed (empty otherwise)
    Status   rune   // 'M' modified, 'A' added, 'D' deleted, 'R' renamed
    Added    int    // lines added (from --stat)
    Deleted  int    // lines deleted (from --stat)
}

// ParseFileDiff parses git diff --name-status combined with --stat output
// into a FileDiff slice. Used by the diff command left panel.
func ParseFileDiff(nameStatus, stat string) ([]FileDiff, error) {}

// SplitMultiFileDiff splits a multi-file unified diff (from stdin or git diff)
// at "diff --git" boundaries into per-file chunks. Used by --stdin mode and
// by the right panel when rendering a single file's diff from a combined output.
func SplitMultiFileDiff(raw string) ([]StdinFileDiff, error) {}
```

Each `FileDiff` implements `list.Item` for `ItemList` via `FilterValue()` (returns
`Name`) and `Title()` (returns the formatted line with icon and stats).

**Piped input:** `jig diff` also accepts piped input for reviewing external diffs:
```
git diff main..feature | jig diff --stdin
gh pr diff 123 | jig diff --stdin
```
When `--stdin` is passed, jig reads the **entire unified diff from stdin into
memory at startup**, then parses it into an **ordered** `[]StdinFileDiff` slice:

```go
type StdinFileDiff struct {
    Name    string // filename from "diff --git a/X b/X" header
    RawDiff string // raw unified diff for this file (pre-rendering)
}
```

The file list preserves the order from the diff (typically alphabetical or grouped
by directory, matching git's output). The right panel renders on demand from
`StdinFileDiff.RawDiff` via the shared `Renderer`. A `map[string]int` index maps
filenames to slice positions for O(1) lookup when the user selects a file.
Stdin is consumed once and never re-read.

Note: do **not** use a bare `map[string]string` — map iteration order is
non-deterministic in Go, which would randomize the file list on every startup.

Note: `--stdin` mode is read-only. The `y` key copies the file path, but there
is no staging or restoring since the diff may not correspond to the local repo.

---

### `fixup` (alias `gfix`) — amend staged changes into a commit

**Preconditions (checked at startup, shown as status bar error if violated):**
1. Staged changes exist (`git diff --cached --quiet` exits non-zero). Error: "nothing staged".
2. Not mid-rebase/merge (`.git/rebase-merge`, `.git/rebase-apply`, `.git/MERGE_HEAD` absent).

Left panel: recent commits from `git log --oneline --color=always` (default depth: 30).
Right panel: `git show <hash>` via `diff.Chain(cfg)` — shows what the target commit contains.

```
+------------------------+-----------------------------------------------+
| Recent commits         |  commit d4e5f6...                             |
|  a1b2c3 fix wifi scan |  Author: Javier ...                          |
| >  d4e5f6 add mt7927  |                                               |
|  7g8h9i init probe    |  diff --git a/driver.c b/driver.c             |
|                        |  ...                                          |
+------------------------+-----------------------------------------------+
| ⏎ fixup into  q quit                 [2 files staged] [ main]        |
```

Keybindings:
- `j`/`k` or `↑`/`↓`: move commit cursor
- `Home` / `End`: jump to first / last commit
- `<enter>`: fixup into selected commit
- `y`: copy commit hash to clipboard
- `q` / `<esc>`: emit `PopModelMsg{MutatedGit: false}` (user cancelled)

Note: no `/` filter in fixup — the commit list is short (default 30 entries) and
the user typically knows which commit to target. If needed, scroll with `j`/`k`.

Execution on `<enter>`:
```
1. git commit --fixup=<hash>
2. GIT_SEQUENCE_EDITOR=true git rebase --autosquash --interactive <hash>^
   (GIT_SEQUENCE_EDITOR=true accepts the todo list without opening an editor)
3. Root commit edge case: if <hash>^ fails (no parent), retry with --root flag.
4. Conflict on non-zero exit: check .git/rebase-merge/stopped-sha.
   If present: show statusbar message "conflict at <sha> — resolve then git rebase --continue".
   Do NOT auto-abort. Let the user handle it.
5. Success: show " fixup applied" in statusbar, emit `PopModelMsg{MutatedGit: true}`.
```

---

### `rebase-interactive` (alias `gri`) — visual todo editor ([reference: git-interactive-rebase-tool](https://github.com/MitMaro/git-interactive-rebase-tool))

Acts as a drop-in replacement for `$GIT_SEQUENCE_EDITOR`. Two usage paths:

**Path A — standalone:**
```
jig rebase-interactive [base]
```
Reads commits from `git log --reverse --pretty=format:"pick %h %s" <base>..HEAD`,
presents the todo list, executes via `GIT_SEQUENCE_EDITOR="cp <tempfile>" git rebase -i <base>`.

**Path B — sequence editor (the right way):**
```
git config --global sequence.editor "jig rebase-interactive"
git rebase -i HEAD~5   # git calls `jig rebase-interactive <todo-file-path>`
```
When called with a file path argument, the command reads the existing todo file, presents
the editor, and **writes the result back to that file path** on `<enter>`. Git then
reads the modified file and proceeds. This is identical to how MitMaro's
git-interactive-rebase-tool works.

**Mode detection:** the command inspects its first argument via `os.Stat()`. If the
argument is an existing file, it enters Path B (sequence-editor mode). Otherwise, it
treats the argument as a git ref and enters Path A (standalone mode). No flags needed.

Left panel: the ordered todo list.
Right panel: `git show <hash>` of the commit under cursor via `diff.Chain(cfg)`.

```
+------------------------+-----------------------------------------------+
| 5 commits on feature   |  commit a1b2c3                                |
|                        |  Author: Javier ...                           |
|  pick   a1b2c3 fix wifi|                                               |  <- cursor
|  pick   d4e5f6 add init|  diff --git a/driver.c b/driver.c             |
|  squash 7g8h9i cleanup |  @@ -42,7 ...                                |
|  drop   0j1k2l debug   |  ...                                          |
|  pick   3m4n5o add probe                                               |
+------------------------+-----------------------------------------------+
| p r e s f d  j/k move  V visual  u undo  b break  ! edit  ⏎ go  q   |
```

**Action keybindings** (applied to row under cursor):

| Key       | Action  | Description                                                |
|-----------|---------|------------------------------------------------------------|
| `p`       | pick    | use commit as-is                                           |
| `r`       | reword  | use commit; git stops after rebase starts to edit message  |
| `e`       | edit    | use commit; git stops after applying it for amending       |
| `s`       | squash  | meld into previous commit, keep both messages              |
| `f`       | fixup   | meld into previous commit, discard this message            |
| `d`       | drop    | remove commit entirely (rendered dim + strikethrough)      |
| `k`       | —       | move row up (swap with row above), clamped at index 0      |
| `j`       | —       | move row down (swap with row below), clamped at last index |
| `V`       | —       | enter visual mode: select a range of rows (see below)      |
| `u`       | —       | undo last action (action change, reorder, or visual apply) |
| `b`       | break   | insert a `break` line below cursor (pause point in rebase) |
| `B`       | —       | remove `break` line under cursor                           |
| `!`       | —       | open todo in `$EDITOR` for manual editing (shell out)      |
| `y`       | —       | copy commit hash under cursor to clipboard                 |
| `<enter>` | —       | execute rebase (Path A) or write todo file (Path B)        |
| `q`/`<esc>`| —      | abort: discard changes and exit (see quit semantics below) |

**Visual rendering by action:**

| Action  | Icon | Rendering                                                                  |
|---------|------|----------------------------------------------------------------------------|
| pick    |  | `ColorBlue` **bold**, subject `ColorFg`                                    |
| reword  |  | `ColorYellow` **bold**, subject `ColorFg`                                  |
| edit    |  | `ColorCyan` **bold**, subject `ColorFg`                                    |
| squash  |  | `ColorPurple` **bold**, subject indented two spaces                        |
| fixup   |  | `ColorPurple` *italic* + dim, subject indented                             |
| drop    |  | entire row `ColorRed` dim + strikethrough                                  |

**Reordering:**
- `k`: move row up (swap with row above), clamped at index 0
- `j`: move row down (swap with row below), clamped at last index
- `↑`/`↓` arrows: navigate cursor between rows
- `Home` / `End`: jump to first / last entry in todo list

**Visual mode (`V`):** inspired by git-interactive-rebase-tool’s visual mode.
Press `V` to enter visual mode at the current row. Navigate with `↑`/`↓` to extend
the selection (highlighted rows use `ColorBgSel`). While in visual mode:
- Action keys (`p`, `r`, `e`, `s`, `f`, `d`): apply the action to **all selected rows**.
- `k`/`j`: move the entire selected block up/down as a unit.
- `<esc>` or `V` again: exit visual mode, keep changes.
This eliminates the tedious one-by-one action changes for "squash the last 5 commits"
type operations. Status bar shows "VISUAL (N rows)" in `ColorPurple` while active.

**Undo (`u`):** single-level undo. Before each undoable action (action change,
reorder, visual apply, break insertion), clone the `[]TodoEntry` slice into
`prevEntries`. Pressing `u` swaps `entries` and `prevEntries` in place. This is the
same self-inverting toggle as hunk-add undo: press once to undo, press again to redo.
Cost: O(n) clone where n = number of todo entries (typically <50). The swap is O(1)
(pointer swap of two slice headers).
Status bar shows " undo" briefly when triggered.

**Break insertion (`b` / `B`):** `b` inserts a `break` line below the cursor. A break
causes git to pause the rebase at that point, returning control to the shell. `B`
(uppercase) removes a break line if the cursor is on one. Break lines are rendered in
`ColorOrange` *italic*. This is useful for "rebase up to this point, test, then continue."

**Shell-out (`!`):** opens the raw todo file in `$EDITOR` (using the same editor
resolution chain as hunk-add). On editor exit, re-parse the file and update the
TodoList view. This is the escape hatch for operations jig doesn’t support natively
(e.g. `exec` lines, `label`/`reset` for complex rebases).

**Quit semantics (Path A vs Path B):**

The rebase command's quit behavior differs by invocation mode because Path B runs as
a subprocess of git (not as a stacked sub-command within jig):

Path A (standalone): `q`/`<esc>` emits `PopModelMsg{MutatedGit: false}`. `<enter>`
runs the rebase, then emits `PopModelMsg{MutatedGit: true}` on success.

Path B (sequence editor): the program is invoked directly by git, not through the
AppModel stack. `q`/`<esc>` sets `model.aborted = true` and emits `tea.Quit`. `<enter>`
writes the todo file and emits `tea.Quit`. After `tea.Program.Run()` returns, `main.go`
checks `model.aborted`:
```go
// cmd/jig/main.go — after p.Run() returns for sequence-editor mode:
if rebaseModel, ok := finalModel.(*RebaseModel); ok && rebaseModel.Aborted() {
    os.Exit(1) // git sees non-zero exit, aborts the rebase
}
// Otherwise exit 0 — git reads the modified todo file and proceeds
```
This is the one place `os.Exit` is used outside the normal flow, and it's in `main.go`
as required by the constraints.

**Execution on `<enter>`:**

Path A:
```go
todo := git.WriteTodo(entries)
f := os.CreateTemp("", "jig-rebase-todo-*")
f.WriteString(todo)
runner.RunWithEnv(ctx,
    []string{"GIT_SEQUENCE_EDITOR=cp " + f.Name()},
    "rebase", "--interactive", base,
)
```

Path B (sequence editor mode):
```go
todo := git.WriteTodo(entries)
os.WriteFile(todoFilePath, []byte(todo), 0644)
// exit 0 — git reads the modified file and continues
```

**Conflict handling:** same as `fixup` — detect `.git/rebase-merge/stopped-sha`, show
statusbar message, leave terminal in a usable state.

**`reword` and `edit` mid-rebase — `tea.ExecProcess` hand-off:**

Both actions cause git to pause mid-rebase and hand control back to the user.
The TUI must suspend itself cleanly, yield the terminal, and resume after the
user finishes.

- `reword`: git stops and opens `$GIT_EDITOR` (or `$EDITOR`) on the commit message
  file. Use `tea.ExecProcess(exec.Command(editor, msgFile))` so bubbletea suspends
  the alternate screen, lets the editor own the terminal, then redraws when it exits.
  The rebase then continues automatically.

- `edit`: git stops after applying the commit and prints instructions
  (`git commit --amend`, then `git rebase --continue`). The TUI detects this state
  by checking `.git/rebase-merge/stopped-sha` on non-zero exit from the rebase call.
  Show a statusbar message:
  ```
  edit stopped at <sha> — amend then: git rebase --continue
  ```
  Leave the terminal in a usable state (exit the TUI). The user runs their amend
  in the shell and continues manually. Do **not** try to wrap `--continue` inside
  the TUI; the user may need multiple shell commands before continuing.

**Data types (`internal/git/rebase_todo.go`):**

```go
type Action string

const (
    ActionPick   Action = "pick"
    ActionReword Action = "reword"
    ActionEdit   Action = "edit"   // stop after apply; user amends then continues
    ActionSquash Action = "squash"
    ActionFixup  Action = "fixup"
    ActionDrop   Action = "drop"
    ActionBreak  Action = "break"  // pause rebase at this point
)

type TodoEntry struct {
    Action  Action
    Hash    string
    Subject string
}

// TodoEntries is a named slice type enabling iterator methods (see iter section).
type TodoEntries []TodoEntry

// ParseTodo parses a git-rebase-todo file or log output into TodoEntries.
// Skips comment lines and blank lines. Empty input returns nil, nil.
func ParseTodo(raw string) (TodoEntries, error) {}

// WriteTodo serialises TodoEntries to a valid git-rebase-todo string.
func WriteTodo(entries TodoEntries) string {}
```

**Commit data type (`internal/git/log.go`):**

```go
// Commit represents a parsed git log entry. Used by fixup, reset, rebase, and log.
type Commit struct {
    Hash       string    // short hash (7+ chars)
    Subject    string    // first line of commit message
    Author     string    // author name
    AuthorDate time.Time // author date (for relative display: "2 hours ago")
    Refs       []string  // parsed from --decorate: ["HEAD -> main", "origin/main"]
}

// Commits is a named slice type enabling iterator methods.
type Commits []Commit

// ParseLog parses git log output into Commits. The git command must use
// --pretty=format with NUL-delimited fields:
//   git log --pretty=format:"%h%x00%s%x00%an%x00%at%x00%D" [args...]
// where %at is the author date as a unix timestamp (unambiguous, locale-independent).
// ParseLog converts %at to time.Time via time.Unix(n, 0).
// Refs (%D) are split on ", " into []string; empty string yields nil slice.
func ParseLog(raw string) (Commits, error) {}
```

The `log` command fetches stat and full diff data **on demand** when the user
changes detail levels or moves the cursor — not at parse time. Only `ParseLog`
runs during initial load. This keeps startup fast even for repos with thousands
of commits.

**StatusEntry data type (`internal/git/status.go`):**

```go
// StatusEntry represents one line from git status --short output.
type StatusEntry struct {
    Path     string // relative file path
    Index    rune   // index status: ' ', 'M', 'A', 'D', 'R', '?', '!'
    WorkTree rune   // working tree status: ' ', 'M', 'A', 'D', 'R', '?'
}

// StatusEntries is a named slice type.
type StatusEntries []StatusEntry

// IsUntracked returns true if the entry is untracked (?? in git status).
func (e StatusEntry) IsUntracked() bool { return e.Index == '?' && e.WorkTree == '?' }

// IsStaged returns true if the entry has changes in the index.
func (e StatusEntry) IsStaged() bool { return e.Index != ' ' && e.Index != '?' }

// Section returns "Staged", "Unstaged", or "Untracked" for grouping in add.
func (e StatusEntry) Section() string

// ParseStatus parses git status --short --porcelain=v1 output.
// The porcelain format guarantees machine-readable, locale-independent output.
func ParseStatus(raw string) (StatusEntries, error) {}
```

Each `StatusEntry` implements `list.Item` for `ItemList` via `FilterValue()` (returns
`Path`) and `Title()` (returns the formatted line with icon and stats).
---

### `reset` (alias `gr`) — interactive reset (soft / mixed / hard)

Provides a visual interface for `git reset` with mode selection. Shows exactly
which commits and changes will be affected before executing.

**Preconditions (checked at startup, shown as status bar error if violated):**
1. Not mid-rebase/merge (`.git/rebase-merge`, `.git/rebase-apply`, `.git/MERGE_HEAD` absent).
2. At least one commit exists (`git rev-parse HEAD` succeeds). Error: "no commits yet".

**Mode (shown in left panel header, toggled by keypress):**

| Mode    | Key  | Git command                  | Effect                                        |
|---------|------|------------------------------|-----------------------------------------------|
| mixed   | `m`  | `git reset --mixed <hash>`   | Unstage changes, keep working tree (default)  |
| soft    | `s`  | `git reset --soft <hash>`    | Keep staged + working tree                    |
| hard    | `H`  | `git reset --hard <hash>`    | Discard everything (destructive — uppercase) |

Left panel: recent commits from `git log --oneline` (default depth: `GT_LOG_DEPTH`).
The header shows the current mode: `Reset to (mode: mixed)`.
Right panel: `git diff <selected_hash>..HEAD` via `diff.Chain(cfg)` — shows the
accumulated changes between the target commit and HEAD that will be affected by the
reset. This is the actual content that will move to working tree (mixed), stay staged
(soft), or be discarded (hard).

```
+------------------------+-----------------------------------------------+
| Reset to (mode: mixed) |  3 commits, 5 files  +42 -15                 |
|  a1b2c3 fix wifi scan |  diff --git a/driver.c b/driver.c             |
| >  d4e5f6 add mt7927  |  @@ -42,7 +42,10 @@                          |
|  7g8h9i init probe    |  ...                                          |
|                        |                                               |
+------------------------+-----------------------------------------------+
| s soft  m mixed  H HARD  ⏎ reset  q quit          [ main] [reset]   |
```

**Right panel summary line:** The first line of the right panel shows a summary:
`N commits, M files  +A -D` computed from `git diff --stat <hash>..HEAD`. This gives
the user an at-a-glance understanding of the scope before scrolling the full diff.

Keybindings:
- `j`/`k` or `↑`/`↓`: move commit cursor
- `Home` / `End`: jump to first / last commit
- `s`: set mode to soft
- `m`: set mode to mixed (default)
- `H` (uppercase): set mode to hard — status bar shows `⚠ HARD RESET` in `ColorRed`
- `<enter>`: execute `git reset --<mode> <hash>`
- `y`: copy commit hash to clipboard.
- `q` / `<esc>`: emit `PopModelMsg{MutatedGit: false}` (user cancelled)

**Hard reset safety:**
- When mode is `hard`, the `<enter>` keyhint changes to `<enter> RESET (destructive)` in
  `ColorRed` to match the uppercase-key-for-destructive pattern used by `checkout`.
- If the working tree has uncommitted changes (`git status --porcelain` non-empty) and mode
  is `hard`, show a status bar warning: "uncommitted changes will be lost" in `ColorYellow`.
  The reset still proceeds on `<enter>` — the warning is informational, not blocking.

**Quit semantics:**

`q`/`<esc>` emits `PopModelMsg{MutatedGit: false}` — the user cancelled without
making changes. After a successful `git reset`, the command emits
`PopModelMsg{MutatedGit: true}` so the parent model refreshes its data.

**Execution on `<enter>`:**
```
1. git reset --<mode> <hash>
2. On success: show " reset to <short_hash> (<mode>)" in statusbar, emit `PopModelMsg{MutatedGit: true}`.
3. On error: show error in statusbar. Do NOT quit — let the user try again.
```

**Visual rendering by mode (left panel header + status bar):**

| Mode  | Header text color | Status bar indicator                                     |
|-------|-------------------|----------------------------------------------------------|
| mixed | `ColorFg`         | `[mixed]` in `ColorFgSubtle`                             |
| soft  | `ColorGreen`      | `[soft]` in `ColorGreen`                                 |
| hard  | `ColorRed`        | `⚠ HARD RESET` in `ColorRed`                             |


---

### `log` (alias `gl`) — visual commit browser ([reference: tig](https://github.com/jonas/tig))

Interactive commit log viewer with three detail levels. Replaces `git log` / tig
for day-to-day commit browsing with integrated search and cross-command actions.

**Default scope:** current branch (`git log HEAD`). Use `jig log --all` for all branches.
Accepts optional revision arguments: `jig log main..feature`, `jig log HEAD~20`.

**Three detail levels** (cycle with `<tab>`):

| Level     | Left panel                  | Right panel                            |
|-----------|-----------------------------|----------------------------------------|
| **oneline** (default) | ` hash subject` one line per commit, `ColorOrange` hash, `ColorFg` subject | Commit metadata: full message, author (**bold**), date (*italic*), refs |
| **stat**   | Same as oneline (cursor stays) | `git diff-tree --stat <hash>` — file list with `+N -N` stats |
| **full**   | Same as oneline (cursor stays) | `git show <hash>` via `diff.Chain(cfg)` — full diff of commit |

The left panel is always the commit list. The right panel content changes based on
the current detail level. `<tab>` cycles forward (oneline → stat → full → oneline),
`<shift+tab>` cycles backward. The current level is shown in the status bar as
`[ oneline]`, `[ stat]`, or `[ full]` using `ColorCyan` **bold**.

```
+------------------------+-----------------------------------------------+
| Commits [ oneline]    |  commit d4e5f6                                |
|                        |  Author: Javier <javier@example.com>          |
|  a1b2c3 fix wifi scan |  Date:   2 hours ago                          |
| >  d4e5f6 add mt7927  |                                               |
|  7g8h9i init probe    |  Add MT7927 WiFi driver initialization        |
|  0j1k2l add firmware  |                                               |
|  3m4n5o first commit  |  Refs: HEAD -> main, origin/main              |
+------------------------+-----------------------------------------------+
| tab level  / search  F fixup  R rebase  D diff  y yank  q quit        |
```

**Stat level wireframe:**
```
+------------------------+-----------------------------------------------+
| Commits [ stat]       |  3 files changed, +42, -15                    |
|  a1b2c3 fix wifi scan |                                               |
| >  d4e5f6 add mt7927  |   driver.c   | 11 +++++----                  |
|  7g8h9i init probe    |   mt7927.c   | 42 +++++++++++++++             |
|  0j1k2l add firmware  |   firmware.h |  4 ++--                        |
+------------------------+-----------------------------------------------+
```

**Full diff level wireframe:**
```
+------------------------+-----------------------------------------------+
| Commits [ full]       |  diff --git a/driver.c b/driver.c             |
|  a1b2c3 fix wifi scan |  @@ -42,7 +42,10 @@                          |
| >  d4e5f6 add mt7927  |   int ret;                                    |
|  7g8h9i init probe    |  -old_init_sequence();                        |
|  0j1k2l add firmware  |  +new_init_a();                               |
|                        |  +new_init_b();                               |
+------------------------+-----------------------------------------------+
```

Keybindings:
- `j`/`k` or `↑`/`↓`: move commit cursor
- `Home` / `End`: jump to first / last loaded commit
- `<tab>`: cycle detail level forward (oneline → stat → full)
- `<shift+tab>`: cycle detail level backward
- `<enter>`: move focus to right panel (scroll mode)
- `<esc>`: context-dependent — if right panel is focused, return to left; if left panel
  is focused, emit `PopModelMsg{MutatedGit: false}`. If search input is active, clear
  search and return to full list.
- `/`: open search input (see search below)
- `F`: **fixup** staged changes into selected commit (launches `fixup` command with
  the selected hash pre-filled). Precondition: staged changes must exist.
- `R`: **rebase-interactive** from selected commit (launches `rebase-interactive`
  in standalone mode with `<hash>^` as base). Root commit edge case: use
  `--root` flag instead of `<hash>^`.
- `D`: **diff** for selected commit (launches `diff` with `<hash>^..<hash>`).
  Root commit edge case: if `<hash>^` fails (no parent), use `git diff --root <hash>`
  which diffs the root commit against the empty tree.
- `y`: copy commit hash to clipboard.
- `q`: always emit `PopModelMsg{MutatedGit: false}` regardless of panel focus.

**Infinite scroll / lazy loading:** the commit list loads `GT_LOG_DEPTH` commits
initially (default: 30) via `git log --oneline -<N>`. When the cursor reaches the
last loaded commit, the next batch is fetched via `git log --oneline --skip=<loaded> -<N>`.
This is triggered as a `tea.Cmd` so the UI stays responsive. `Home` always scrolls
to HEAD; `End` loads up to 500 additional commits (capped to prevent OOM on
huge repos like the Linux kernel — show a
`` spinner in the status bar while loading).

**Performance note:** `--skip=N` is O(N) in git (it walks N commits before outputting).
For page 10+ of a very active repo this gets slow. If profiling shows this is a
bottleneck, switch to hash-based pagination: `git log <last_hash>~1 -<N>` which
is O(1) lookup. For v0.1.0, `--skip` is acceptable because most users rarely scroll
past ~100 commits and the async `tea.Cmd` keeps the UI responsive regardless.

**Search pagination:** search (`/`) loads **all matching results at once** (no lazy
loading during search). This is acceptable because search queries naturally narrow
the result set. `<esc>` clears the search and returns to the lazy-loaded full list.

**Search (`/`):**

When `/` is pressed, a search input appears in the status bar. The search syntax:

| Prefix     | Behavior                                                     |
|------------|--------------------------------------------------------------|
| (none)     | Grep commit messages AND diffs (`git log -G <query>`)        |
| `@`        | Filter by author (`git log --author=<query>`)                |
| `:`        | Grep commit messages only (`git log --grep=<query>`)         |

Search respects the current branch scope: if `jig log` was launched without
`--all`, search only queries the current branch. If `jig log --all` was used,
search queries all branches. This prevents confusing results where search returns
commits not visible in the main list.

Search is **asynchronous** — the TUI remains responsive while `git log` runs in
the background via `tea.Cmd`. Results replace the commit list. A status bar indicator
shows " searching..." during the query and " N results" when complete.

Submitting an empty search query is a no-op (status bar clears, list unchanged).
Press `<esc>` to clear the search and return to the full commit list.
`n`/`N` jump to the next/previous match within commit messages in the right panel
(highlights matches using `ColorYellow` **bold** on `ColorBgSel` background).

**Cross-command integration:**

The `F`, `R`, and `D` keys launch other jig commands as sub-programs. Implementation:

```go
// F: fixup into selected commit
// Pre-check: git diff --cached --quiet must exit non-zero.
// Then: launch the fixup command model with the selected hash.
// This is a state machine transition, not a subprocess.

// R: rebase-interactive from selected commit
// Launch rebase-interactive in standalone mode (Path A)
// with base = <selected_hash>^

// D: diff for selected commit
// Launch diff with args ["<hash>^..<hash>"]
// This shows exactly what that commit changed.
```

These are **state machine transitions** within the bubbletea program (replace the
current model), not subprocesses. On quit from the sub-command, return to the log
view at the same cursor position.


---

## Config

`~/.config/jig/config.toml` — all keys also readable as `GT_*` env vars.

| Env var            | Toml key         | Default                        |
|--------------------|------------------|--------------------------------|
| `GT_THEME`         | `theme`          | `dark`                         |
| `GT_COPY_CMD`      | `copy_cmd`       | `wl-copy` (Wayland) / `xclip` |
| `GT_DELTA_PATH`    | `delta_path`     | auto-detected via LookPath     |
| `GT_LOG_DEPTH`     | `log_depth`      | `30`                           |
| `GT_DIFF_CONTEXT`  | `diff_context`   | `3`                            |
| `GT_LOG_LEVEL`     | `log_level`      | `warn`                         |

---

## Shell plugin contract — Fish

```fish
# shell/jig.plugin.fish
# Source from ~/.config/fish/conf.d/jig.fish or via fisher.
# Set JIG_NO_ALIASES=1 before sourcing to disable alias functions.

if not set -q JIG_NO_ALIASES
    function ga;   jig add $argv; end
    function gha;  jig hunk-add $argv; end
    function gc;   jig checkout $argv; end
    function gd;   jig diff $argv; end
    function gfix; jig fixup $argv; end
    function gri;  jig rebase-interactive $argv; end
    function gr;   jig reset $argv; end
    function gl;   jig log $argv; end
end

# GIT_SEQUENCE_EDITOR setup (run once):
# git config --global sequence.editor "jig rebase-interactive"
```

---

## Makefile

```makefile
BINARY    := jig
BUILD_DIR := bin
THRESHOLD := 90
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test test-integration lint fmt vet fix coverage install clean snapshot check-release

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/jig

test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@COV=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | tr -d '%'); \
	 [ $$(echo "$$COV < $(THRESHOLD)" | bc) -eq 1 ] \
	   && { echo "FAIL: $$COV% < $(THRESHOLD)%"; exit 1; } \
	   || echo "OK: $$COV%"

test-integration:
	go test -race -tags integration ./tests/integration/...

fix:    ; go fix ./...
lint:   ; golangci-lint run ./...
fmt:    ; gofmt -w . && goimports -w .
vet:    ; go vet ./...
coverage: ; go tool cover -html=coverage.out -o coverage.html

install: build
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)

snapshot:
	goreleaser release --snapshot --clean

check-release:
	goreleaser check

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html dist/
```

---

## .golangci.yml

```yaml
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - gocritic
    - revive
    - wrapcheck
    - testifylint

linters-settings:
  wrapcheck:
    ignoreSigs: ["fmt.Errorf("]

issues:
  exclude-rules:
    - path: _test\.go
      linters: [wrapcheck]
```

---

## `.pre-commit-config.yaml`

Run linters locally before every commit to catch errors without waiting for CI.
Install: `pip install pre-commit && pre-commit install`.

```yaml
repos:
  - repo: https://github.com/golangci/golangci-lint
    rev: v2.1.6
    hooks:
      - id: golangci-lint
        args: ["--timeout=3m"]

  - repo: https://github.com/dnephin/pre-commit-golang
    rev: v0.5.1
    hooks:
      - id: go-vet
      - id: go-fmt
      - id: go-imports

  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-merge-conflict
      - id: detect-private-key

  - repo: local
    hooks:
      - id: go-fix
        name: go fix
        entry: bash -c 'go fix ./... && git diff --exit-code'
        language: system
        pass_filenames: false
        types: [go]
      - id: go-build
        name: go build
        entry: go build ./cmd/jig
        language: system
        pass_filenames: false
        types: [go]
```

This catches formatting, vet, lint, and build errors **before the commit is
created**, avoiding the CI round-trip. The `golangci-lint` hook uses the same
`.golangci.yml` as CI, so local and remote results are identical.

Add `.pre-commit-config.yaml` to the repo root. It is created in Phase 0.

---

## Test strategy — Test Driven Development

### The TDD rule — non-negotiable

**Every function is written test-first.** The commit sequence within any phase is
always:

```
1. RED   — write _test.go with all cases; it must fail to compile or fail to run.
            Commit message: "git/hunk: add failing tests for ParseHunks"
2. GREEN — write the minimal implementation that makes the tests pass.
            Commit message: "git/hunk: implement ParseHunks"
3. REFACTOR — clean up without breaking tests; run make lint.
            Commit message: "git/hunk: clean up ParseHunks"
```

Never write implementation code before writing a failing test for it. If you find
yourself writing `func ParseHunks` before `func TestParseHunks`, stop and write the
test first.

**Commit message format — Linux kernel style:**

```
subsystem: imperative short description
                                         ← blank line
Optional body paragraph explaining *why*, not *what*. The diff shows what
changed; the body explains the motivation or design rationale. Wrap at 72
columns.
```

Rules:
- **Subsystem** is the package path relative to `internal/`: `git`, `git/hunk`,
  `tui/components`, `commands`, `app`, `config`. For root-level files use `build`,
  `ci`, or `doc`.
- **Imperative mood**, lowercase, no trailing period: "add support for" not
  "Added support for." or "adds support for".
- Subject line ≤72 characters.
- No `feat:`, `fix:`, `refactor:` conventional-commits prefixes.
- No emoji, no icons in commit messages.

Examples:
```
git/hunk: add failing tests for SplitHunkAt
git/hunk: implement SplitHunkAt
git/hunk: clean up SplitHunkAt
commands: wire hunk-add into cobra
tui/components: add undo to HunkView
app: handle RefreshMsg on PopModelMsg
build: add pre-commit hooks
ci: add goreleaser check step
doc: update README with log command
```

The changelog in goreleaser includes every non-merge commit as-is. Since
the subsystem prefix already groups commits naturally, no further filtering
or categorization is needed.

**What "failing" means:**
- A compile error because the function does not exist yet is a valid RED state.
- A test that calls `t.Fatal("not implemented")` is **not** a valid RED state —
  it will always fail regardless of implementation. Write real assertions.

---

### Test integrity — tests must prove correctness, not just pass

This section exists because AI code generators (including Claude Code) have a
documented tendency to write tests that compile and pass without actually verifying
behavior. Every rule below is mandatory and must be followed without exception.

**Rule 1: Never weaken an assertion to make a test pass.** If a test fails, the
implementation is wrong — fix the implementation, not the test. The only exception
is when the test itself has a genuine specification error (wrong expected value in
the spec). In that case, fix the spec first, update the test second, then fix the
implementation.

```go
// WRONG — weakened assertion to make it pass:
require.NotNil(t, result)           // proves nothing about content
require.NoError(t, err)             // only proves it didn't crash
require.True(t, len(result) > 0)    // proves non-empty, not correct

// RIGHT — assert the exact expected state:
require.Equal(t, 3, len(result))
require.Equal(t, "pick", string(result[0].Action))
require.Equal(t, "a1b2c3", result[0].Hash)
require.Equal(t, "fix wifi scan", result[0].Subject)
```

**Rule 2: Assert observable side effects, not just return values.** For functions
that mutate state (staging, resetting, writing files), verify the mutation actually
happened by inspecting the result independently:

```go
// WRONG — only checks that the function returned no error:
_, err := runner.Run(ctx, "add", file)
require.NoError(t, err)

// RIGHT — verify the side effect through a separate query:
_, err := runner.Run(ctx, "add", file)
require.NoError(t, err)
status, _ := runner.Run(ctx, "status", "--porcelain")
require.Contains(t, status, "A  "+file)  // file is now staged
```

For integration tests, ALWAYS verify git state after mutation:
- After staging: `git diff --cached --name-only` contains the file.
- After reset: `git log --oneline | wc -l` matches expected count.
- After fixup: commit count is unchanged AND target commit's diff includes the fixup.
- After restore: `git diff --name-only` no longer contains the file.
- After hunk staging: `git diff --cached` contains the staged hunk AND `git diff`
  still contains the unstaged hunks.

**Rule 3: FakeRunner assertions must verify exact call arguments.** Do not use
`MustHaveCall(tb, r, "add")` when the test is about staging a specific file. Assert
the specific file was passed:

```go
// WRONG — proves git add was called, not that the right file was staged:
MustHaveCall(tb, r, "add")

// RIGHT — proves the exact file was passed:
MustHaveCall(tb, r, "add", "driver.c")
```

For `RunWithStdin`, assert the patch content is structurally valid:

```go
// WRONG — proves something was piped, not that it was a valid patch:
MustHaveStdin(tb, r, "@@ ")

// RIGHT — parse the stdin content and verify patch structure:
call := r.LastStdinCall()
hunks, err := ParseHunks(call.Stdin)
require.NoError(t, err)
require.Len(t, hunks, 1)
require.Equal(t, 2, countSelected(hunks[0], '+'))  // exactly 2 '+' lines staged
```

**Rule 4: Every test must fail when the implementation is wrong.** After writing a
test, mentally (or actually) break the implementation and verify the test catches it:
- `ParseHunks` returns an empty slice → the test must fail (not just "pass with 0 items").
- `BuildPatch` emits an unselected `-` line as `-` instead of ` ` → the test must fail.
- `RecalculateHeader` uses wrong start values → the test must fail.
- `SplitHunkAt` splits at the wrong line → the test must fail.

If a test would still pass with a broken implementation, the test is worthless —
rewrite it with stronger assertions.

**Rule 5: Do not hardcode implementation internals in tests.** Tests assert
*behavior* (inputs → outputs), not *how* the code works internally:

```go
// WRONG — testing internals (index into a private array):
require.Equal(t, 3, m.cursorIndex)

// RIGHT — testing observable behavior:
view := m.View()
require.Contains(t, view, "> driver.c")  // cursor indicator on expected file
```

**Rule 6: Edge cases are not optional.** Every parser must be tested with:
- Empty input
- Single-element input
- Maximum realistic input (e.g. a 500-line diff)
- Malformed input (missing fields, truncated data, unexpected encoding)
- Unicode content (commit messages, filenames, branch names)
- Inputs with special characters (spaces in filenames, `#` in messages)

Every UI component must be tested at boundary conditions:
- Cursor at index 0 + move up → clamped (no crash, no wrap)
- Cursor at last index + move down → clamped
- Empty list + any keypress → no crash, sensible behavior
- Single-item list + toggle/navigate → correct state
- All items selected + toggle all → all deselected (not a no-op)
- Terminal at minimum size (60×10) → layout renders without panic

**Rule 7: Integration tests verify the full pipeline end-to-end.** An integration
test that only checks "command didn't crash" is useless. Each integration test must:

1. Set up a known git state (specific files, specific commits, specific staging).
2. Execute the command (simulate keypresses via the `sendKey`/`sendSpecialKey` helpers).
3. Verify the **exact** git state after execution (not just "no error").

```go
// WRONG — integration test that proves nothing:
func TestHunkAddIntegration(t *testing.T) {
    repo := NewTempRepo(t)
    // ... setup ...
    err := runHunkAdd(repo)
    require.NoError(t, err)  // proves it didn't crash, not that it staged correctly
}

// RIGHT — integration test that verifies exact git state:
func TestHunkAddIntegration(t *testing.T) {
    repo := NewTempRepo(t)
    WriteFile(t, repo, "driver.c", original)
    AddCommit(t, repo, "initial")
    WriteFile(t, repo, "driver.c", modified)  // creates 2 hunks

    // Simulate: select only hunk 1's '+' lines, press enter
    m := newHunkAddModel(repo, "driver.c")
    m = sendKey(m, ' ')     // toggle first '+' line in hunk 1
    m = sendKey(m, '\r')    // stage

    // Verify hunk 1 is staged, hunk 2 is not
    cached, _ := exec("git", "-C", repo, "diff", "--cached")
    require.Contains(t, cached, "new_init_a")      // hunk 1 staged
    require.NotContains(t, cached, "modern_probe")  // hunk 2 NOT staged

    unstaged, _ := exec("git", "-C", repo, "diff")
    require.Contains(t, unstaged, "modern_probe")   // hunk 2 still in working tree
    require.NotContains(t, unstaged, "new_init_a")  // hunk 1 no longer unstaged
}
```

**Rule 8: Do not write pass-through tests.** A test that calls a function and asserts
the return value equals a hardcoded copy of what the function returns is circular —
it passes by definition and tests nothing:

```go
// WRONG — circular test (hardcoded output == function output by construction):
func TestWriteTodo(t *testing.T) {
    entries := []TodoEntry{{ActionPick, "a1b2c3", "fix"}}
    got := WriteTodo(entries)
    require.Equal(t, "pick a1b2c3 fix\n", got)  // how do you know "pick a1b2c3 fix\n" is correct?
}

// RIGHT — round-trip property test (proves parse and write are inverses):
func TestWriteTodoRoundTrip(t *testing.T) {
    original := "pick a1b2c3 fix wifi scan\nreword d4e5f6 add init\ndrop 7g8h9i cleanup\n"
    entries, err := ParseTodo(original)
    require.NoError(t, err)
    roundTripped := WriteTodo(entries)
    require.Equal(t, original, roundTripped)
}

// RIGHT — structural assertion (proves the output is valid git-rebase-todo format):
func TestWriteTodo(t *testing.T) {
    entries := []TodoEntry{{ActionSquash, "a1b2c3", "fix wifi"}}
    got := WriteTodo(entries)
    // Parse it back to prove it's valid
    parsed, err := ParseTodo(got)
    require.NoError(t, err)
    require.Equal(t, entries, parsed)
    // Also verify format (e.g. for git compatibility)
    require.True(t, strings.HasSuffix(got, "\n"), "must end with newline for git")
}
```

**Rule 9: Test the negative path with equal rigor.** For every "happy path" test,
write a corresponding test that verifies the function rejects invalid input correctly:

```go
// Happy path:
{name: "valid hunk", input: validDiff, wantErr: false}

// Negative paths (equally important):
{name: "truncated header", input: "@@ -1,3 +1", wantErr: true}
{name: "missing file header", input: "@@ -1,3 +1,3 @@\n+foo\n", wantErr: true}
{name: "wrong line prefix", input: header + "x invalid line\n", wantErr: true}
{name: "empty after stripping comments", input: "# just a comment\n", wantErr: true}
```

And for UI components, test that invalid actions are properly rejected:
```go
{name: "enter with nothing selected", key: '\r', wantErr: ErrNothingSelected}
{name: "split unsplittable hunk", key: 's', wantStatusBar: "cannot split"}
{name: "fixup without staged changes", key: 'F', wantStatusBar: "nothing staged"}
```

---

### Test patterns — use consistently

**Table-driven tests with `t.Run` subtests** — the default for all pure functions:
```go
func TestParseTodo(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name    string
        input   string
        want    TodoEntries
        wantErr bool
    }{
        {
            name:  "single pick",
            input: "pick a1b2c3 fix wifi scan\n",
            want:  []TodoEntry{{Action: ActionPick, Hash: "a1b2c3", Subject: "fix wifi scan"}},
        },
        {
            name:    "empty input",
            input:   "",
            want:    nil,
            wantErr: false,
        },
        // more cases
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            got, err := ParseTodo(tc.input)
            if tc.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            require.Equal(t, tc.want, got)
        })
    }
}
```

**`t.Parallel()` at every level** — both the top-level `TestXxx` and each `t.Run`
subtest, unless the test touches shared mutable state. Integration tests that use
`testhelper.NewTempRepo` each get their own directory and are safe to parallelize.

**`t.Cleanup` over `defer`** — cleanup runs even if the goroutine is not the one that
created the resource:
```go
func NewTempRepo(tb testing.TB) string {
    tb.Helper()
    dir := tb.TempDir() // cleaned up automatically
    tb.Cleanup(func() { /* anything beyond TempDir */ })
    return dir
}
```

**`T.ArtifactDir()` / `B.ArtifactDir()` (1.26+)** — use for any test output files
(diffs, rendered output, debug snapshots) instead of writing to `os.TempDir()` or
ad-hoc locations. Artifacts persist when `go test -artifacts` is passed:
```go
func TestChromaRenderer(t *testing.T) {
    out := renderer.Render(fixture)
    os.WriteFile(filepath.Join(t.ArtifactDir(), "rendered.diff"), []byte(out), 0644)
}
```

**`B.Loop` (1.24+)** — use instead of `B.N` for all benchmarks. `B.Loop` no longer
prevents inlining in the loop body, giving accurate performance measurements:
```go
// Go 1.26 — preferred:
func BenchmarkParseHunks(b *testing.B) {
    for b.Loop() {
        ParseHunks(fixture)
    }
}

// Legacy — avoid:
// for i := 0; i < b.N; i++ { … }
```

**`testing.TB`** — all helper functions accept `testing.TB`, not `*testing.T`, so they
work in both tests and benchmarks.

**`require` vs `assert` (testify)**:
- `require.X` — preconditions; test stops on failure.
- `assert.X` — independent property checks; test continues to collect all failures.
- Default to `require`; use `assert` when checking multiple properties of one result.

**Always include an error-path subtest** named `"error"` or `"invalid input"` to cover
error branches. Error path coverage is not optional.

---

### Helpers (`internal/testhelper/`)

```go
// fakerunner.go

// FakeRunner records all calls and returns scripted responses in FIFO order.
// Safe for concurrent use from parallel subtests.
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

```go
// gitrepo.go — uses os.Root (Go 1.24+) for all file I/O inside the temp repo

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

---

### Coverage requirements by package

**`internal/git`**

`ParseStatus` — table-driven, >=5 cases: empty, modified-only, untracked-only,
mixed, renames. All subtests parallel.

`ParseFileDiff` — table-driven: modified/added/deleted/renamed/binary. Binary files
return empty slice without panicking.

`ParseHunks` — table-driven, >=6 cases: empty diff, single hunk, multi-hunk,
added-only, deleted-only, renamed-with-changes. Must not panic on any input. Add
`FuzzParseHunks` to `hunk_test.go` as a stretch goal.

`SplitHunks`:
- multi-hunk input → N returned hunks; each has `FileHeader` non-empty; each has
  exactly one `@@` line.
- single-hunk input → one hunk returned unchanged.

`RecalculateHeader` — table-driven:
- Hunk with OldStart=42, NewStart=42, 3 context + 2 `+` + 1 `-` → `@@ -42,4 +42,5 @@`.
  OldStart/NewStart preserved; only counts recomputed.
- After editor deletes a `+` line → new_count decreases by 1; old_count unchanged.
- After editor removes a `-` line → old_count decreases by 1; new_count unchanged.
- Trailing context string after `@@` preserved verbatim.
- Hunk at line 1 (OldStart=1, NewStart=1) → `@@ -1,... +1,... @@`.

`SplitHunkAt` — table-driven:
- Hunk with `+`, `+`, context, `-`, `+` → splits into two hunks at the context line.
  Second hunk’s OldStart/NewStart recomputed from first hunk’s content.
- Hunk with all `+` lines (no internal context) → returns (original, {}, false).
- Hunk with context only → returns (original, {}, false).
- Single-line hunk → returns (original, {}, false).

`BuildPatch` — table-driven (single Hunk input, post-`SplitHunks`):
- selected lines → valid unified diff.
- unselected `-` → demoted to context lines.
- all togglable lines unselected → `ErrNothingSelected`.
- (Integration subtest tagged `integration`) output passes `git apply --check`.

`ParseStatus` — table-driven, >=6 cases: empty, single modified, staged+unstaged,
untracked, renamed (R100), deleted. Verify `IsUntracked()`, `IsStaged()`, `Section()`
for each case. Include files with spaces in names (git quotes these).

`ParseLog` — table-driven, >=5 cases: empty, single, multiple, decorated refs,
unicode subjects. The `Commit` struct must include at minimum: `Hash`, `Subject`,
`Author`, `AuthorDate`, `Refs` (parsed from `--decorate`). These fields are used by
the `log` command for all three detail levels and by `fixup`/`reset` for the commit
list. The `log` command fetches stat and full diff data on demand (not at parse time).

`ParseTodo` — table-driven, >=6 cases: empty, single pick, all six actions, comment
lines skipped, unicode subjects, `exec` lines round-tripped without loss. Property:
`WriteTodo(ParseTodo(raw))` is semantically identical to `raw`.

`WriteTodo` — assert each of drop/squash/fixup/edit/reword serializes correctly.

---

**`internal/diff`**

All renderers share one fixture: `testdata/sample.diff` (10-line unified diff).
Load with `os.ReadFile` in `TestMain` or per-test.

- `PlainRenderer`: output equals input.
- `ChromaRenderer`: output contains `\x1b[`; `+`/`-` prefixes preserved.
- `DeltaRenderer`: `t.Skip("delta not in PATH")` if `exec.LookPath("delta")` fails.
- `Chain()` — table-driven over `Config` combinations → assert correct concrete type
  via `reflect.TypeOf` or a type switch. Note: `reflect.Type.Fields()` and
  `reflect.Type.Methods()` (1.26+) return `iter.Seq` iterators — use these instead of
  manual index loops when inspecting struct fields or interface methods.

---

**`internal/tui/components`**

Drive `Update()` programmatically. Never start a real `tea.Program`.

```go
// shared helper in components_test_helpers_test.go
func sendKey(m tea.Model, key string) tea.Model {
    msg := tea.KeyPressMsg{Code: []rune(key)[0]}
    next, _ := m.Update(msg)
    return next
}

// for special keys (arrow keys, Home/End):
func sendSpecialKey(m tea.Model, code rune) tea.Model {
    msg := tea.KeyPressMsg{Code: code}
    next, _ := m.Update(msg)
    return next
}
// Usage: sendSpecialKey(m, tea.KeyUp)
//        sendSpecialKey(m, tea.KeyDown)
//        sendSpecialKey(m, tea.KeyHome)
//        sendSpecialKey(m, tea.KeyEnd)

// for special keys (PageDown, PageUp, Home, End):
// use sendSpecialKey above with tea.KeyPgDown, tea.KeyPgUp, etc.
```

**Bubbletea v2 key handling rules:**
- Match on `tea.KeyPressMsg`, not `tea.KeyMsg` (which is an interface for both press + release).
- Use `msg.Code` (a `rune`) instead of `msg.Type`. Use `msg.Text` instead of `msg.Runes`.
- Space bar: `msg.String()` returns `"space"`, not `" "`. `msg.Code == ' '` still works.
- Modifiers: use `msg.Mod` field (e.g. `msg.Mod == tea.ModCtrl`), not `tea.KeyCtrlC` constants.
- Use `msg.String()` for matching in switch statements: `"ctrl+c"`, `"ctrl+k"`, `"space"`, etc.

`ItemList` — write tests before implementing:
- `j`/`k` move cursor; `SelectedItem()` changes.
- `/` + chars: `View()` shows only matching items.

`HunkView` — write ALL tests before implementing:
- Load 2-hunk fixture via `ParseHunks` → `SplitHunks`.
- **Initial state:** all togglable lines start unselected (). `SelectedLines()` is empty.
- `<space>` on `+` line: `SelectedLines()` grows by 1; icon changes to .
- `<space>` on context line: cursor jumps to next togglable line below (not a toggle).
- `<space>` on `@@` header: cursor jumps to first togglable line in that hunk.
- `<a>`: all togglable lines in current hunk become selected. Press again: all deselected.
- `<A>`: all togglable lines in entire file become selected. Press again: all deselected.
- `n`: cursor jumps to next `@@` header. `N`: jumps to previous.
- `n` on last hunk: no-op. `N` on first hunk: no-op.
- `u` after `<space>`: selection reverts to state before `<space>`. `u` again: re-does.
- `u` with no prior action: no-op.
- `s` on hunk with internal context: hunk splits into two; hunk list length increases.
  Cursor lands on first line of new second hunk.
- `s` on hunk with no internal context: no-op; status bar shows "cannot split".
- `e`: assert `HunkEditRequestedMsg{Hunk}` emitted with the current hunk (by cursor).
- After simulated editor round-trip via `SetHunk(modifiedHunk)`: assert `@@` header
  has correct OldStart preserved and counts recomputed via `RecalculateHeader`.
- After editor with empty result: hunk restored to pre-editor state.
- `<enter>` with 2 hunks, only hunk 1 has selections → one `git apply --cached -` call
  (hunk 1 only). Hunk 2 is skipped.
- `<enter>` with both hunks having selections → two sequential `git apply --cached -` calls.
- `<enter>` with no selections anywhere → `ErrNothingSelected` in status bar.
- `-` line toggled: assert `BuildPatch` demotes it to context when unselected.

`DiffView`:
- `SetContent(s)` → `View()` contains `s`.
- `j` → `ScrollOffset()` increases.

`TodoList` — write ALL tests before implementing:
- 4 "pick" entries loaded.
- `↓` arrow on row 0 → cursor moves to row 1 (navigation).
- `↑` arrow on row 0 → no-op (clamped at top).
- `r` on row 0 → `View()` contains "reword"; color is `ColorYellow`.
- `e` on row 1 → "edit"; color is `ColorCyan`.
- `s` on row 2 → "squash"; subject indented.
- `f` on row 2 → "fixup"; subject indented + dim.
- `d` on row 3 → "drop"; row dim + strikethrough.
- `p` after `d` on row 3 → reverts to "pick", normal styling.
- `k` on row 1 → rows 0 and 1 swapped in `Entries()` (reorder, not navigate).
- `k` on row 0 → no-op (clamped).
- `j` on last row → no-op (clamped).
- `V` on row 1, `↓` to row 3, `d` → rows 1–3 all become "drop".
- `V` on row 0, `↓` to row 1, `k` → block of 2 rows moves up (row 0 wraps to top).
- `u` after action change → entire todo list restored to previous state.
- `u` with no prior action → no-op.
- `b` on row 1 → break line inserted at index 2; `Entries()` length increases by 1.
- `B` on break line → break removed; length decreases.
- `B` on non-break line → no-op.
- `<enter>` → `TodoConfirmedMsg{Entries}` emitted with correct order and actions.
- `q` → `TodoAbortedMsg{}` emitted.

`LogView` — write ALL tests before implementing:
- Load 5-commit fixture via `ParseLog`.
- Initial detail level is `oneline`. Status bar shows "[ oneline]".
- `<tab>` → level changes to `stat`; right panel content changes to stat output.
- `<tab>` again → level changes to `full`; right panel shows full diff.
- `<tab>` again → cycles back to `oneline`.
- `<shift+tab>` from `oneline` → cycles to `full` (backward).
- `j`/`k` → cursor moves; right panel updates for selected commit.
- `/` + "wifi" → `SearchRequestedMsg{Query: "wifi"}` emitted.
- `/@javier` → `SearchRequestedMsg{Author: "javier"}` emitted.
- `/:fix bug` → `SearchRequestedMsg{Grep: "fix bug"}` emitted.
- `F` with staged changes → `FixupRequestedMsg{Hash}` emitted.
- `F` without staged changes → status bar shows error; no message emitted.
- `R` → `RebaseRequestedMsg{Base: "<hash>^"}` emitted.
- `D` → `DiffRequestedMsg{Rev: "<hash>^..<hash>"}` emitted.
- `y` → `ClipboardMsg{Value: "<hash>"}` emitted.
- Infinite scroll: cursor on last item triggers `LoadMoreMsg`.

`StatusBar`:
- `SetHints(s)` → `View()` left side contains `s`.
- `SetBranch("main")` → `View()` right side contains "main".

---

**`internal/commands`**

TDD sequence for every command:
```
1. Write _test.go with FakeRunner assertions (RED — won't compile yet).
2. Write command stub so tests compile but fail assertions (still RED).
3. Implement command until tests pass (GREEN).
4. Write integration _test.go with build tag `integration` (RED).
5. Implement missing runner calls (GREEN).
6. Refactor.
```

`add`:
- `<space>` → `MustHaveCall(tb, r, "add", "<file>")`. Verify the file moved from
  the Unstaged section to the Staged section in `View()`. Verify cursor position is
  preserved or moves to the nearest remaining unstaged file.
- `<u>` → `MustHaveCall(tb, r, "reset", "HEAD", "<file>")`. Verify the file moved
  from Staged back to Unstaged in `View()`.
- `<a>` → `MustHaveCall(tb, r, "add", "-A")`. Verify the Unstaged section is empty
  in `View()` and status bar shows " all files staged".
- `<enter>` on untracked file → no `PushModelMsg` emitted (untracked files have no hunks).
  Verify status bar shows an explanatory message.
- `<enter>` on modified file → `PushModelMsg{hunk-add model}` emitted with the correct
  filename.
- Integration: stage a real file; assert `git status --porcelain` shows the file with
  index status `A` or `M` in the first column AND working tree status ` ` in the second.
  Unstage it; assert it reverts to working tree status `M` or `?`.

`hunk-add`:
- toggle 2 `+` lines in hunk 1, `<enter>` → `MustHaveCall(tb, r, "apply", "--cached", "-")`.
  Parse the stdin content of the call: verify it contains a valid unified diff header,
  the two selected `+` lines are present, and unselected lines are absent or demoted.
  Only one `apply` call (hunk 2 has no selections).
- toggle lines in both hunks, `<enter>` → two sequential `apply` calls. Verify each
  call's stdin contains only lines from its respective hunk (no cross-contamination).
- `u` after toggle → verify `SelectedLines()` is empty (reverted to initial state).
  `<enter>` after undo → `ErrNothingSelected` in status bar; `MustHaveNoCall(tb, r)`.
- `s` on splittable hunk → hunk count increases by 1; verify both new hunks have valid
  `OldStart`/`NewStart` values; no runner calls (split is UI-only).
- editor path: `HunkEditRequestedMsg` → `tea.ExecProcess`; mock editor writes
  modified hunk; verify `RecalculateHeader` produced correct counts (OldStart preserved,
  counts match actual line content). Stage the result and verify patch is valid.
- editor non-zero exit → hunk restored to pre-editor state; verify `View()` matches
  the snapshot taken before editor launch; `MustHaveNoCall(tb, r)`.
- `git apply` failure → status bar shows error message containing the failure reason;
  batch stops; verify call count is exactly 1 (not 2).
- Integration: 2-hunk file; select lines in hunk 1 only; stage; assert
  `git diff --cached` contains hunk 1 changes AND does not contain hunk 2 changes;
  `git diff` contains hunk 2 changes AND does not contain hunk 1 changes.
- Integration: split a large hunk with `s`; select only the first half; stage; assert
  only the first half appears in `git diff --cached`; second half remains in `git diff`.

`checkout`:
- `<D>` → `MustHaveCall(tb, r, "restore", "<file>")`. After restore, verify the file
  is no longer in the `View()` file list. If it was the last file, verify View shows
  "working tree clean" message.
- `<d>` (lowercase) → `MustHaveNoCall(tb, r)`. Verify file is still in the list
  (no state change).
- no modified files → statusbar shows "working tree clean" (exact string match);
  `MustHaveNoCall(tb, r)`. Verify `tea.Quit` was emitted.
- Integration: modify a file, launch checkout, press `<D>`, verify `git diff --name-only`
  no longer contains the file AND the file content matches the index version.

`diff`:
- `jig diff` → args `["diff"]`. Verify the file list in `View()` matches the output
  of `git diff --name-only`.
- `jig diff --staged` → args `["diff", "--cached"]`. Verify file list matches
  `git diff --cached --name-only`.
- `jig diff HEAD~3` → args `["diff", "HEAD~3"]`.
- `jig diff main..feature` → args `["diff", "main..feature"]`.
- `jig diff --stdin` with piped input → verify file list matches diff headers in the
  piped content; right panel renders the correct file's diff when cursor moves.
  Verify no `Runner` calls are made (stdin mode is fully offline).
- Integration: modify 2 files; launch diff; verify both appear in file list; navigate
  to each file and verify the right panel content matches `git diff -- <file>`.

`fixup`:
- nothing staged → `errMsg` contains "nothing staged" (exact substring); `MustHaveNoCall(tb, r)`.
  Verify `View()` does not show the commit list (precondition failed before rendering).
- mid-rebase → `errMsg` contains "rebase in progress"; `MustHaveNoCall(tb, r)`.
- happy path → `MustHaveCall(tb, r, "commit", "--fixup=<hash>")` then
  `MustHaveEnv(tb, r, "GIT_SEQUENCE_EDITOR=true")`. Verify the calls happen in this
  exact order (commit before rebase).
- root commit → `MustHaveCall(tb, r, "rebase", "--autosquash", "--interactive", "--root")`.
- conflict (non-zero exit) → verify status bar shows "conflict" message AND `tea.Quit`
  was NOT emitted (user stays in the TUI to see the message).
- Integration: 3-commit repo; modify a file that commit 2 touched; stage the change;
  fixup into commit 2; assert commit count is still 3; assert `git show <commit-2-hash>`
  includes the fixup'd change; assert working tree is clean after fixup.

`rebase-interactive`:
- standalone `<enter>` → `MustHaveEnv(tb, r, "GIT_SEQUENCE_EDITOR=cp ")`
  and `MustHaveCall(tb, r, "rebase", "--interactive", base)`. Verify the temp file
  written by cp contains the exact `WriteTodo(entries)` output.
- sequence-editor mode (file-path arg) → after `<enter>` assert
  `os.ReadFile(todoFilePath)` equals `WriteTodo(entries)`. Parse the written file
  back via `ParseTodo` and verify it round-trips to the same entries.
- visual mode: select 3 rows, press `s` → verify all 3 entries have `ActionSquash` in
  `Entries()`. Verify `View()` renders "squash" for all 3 rows with correct indentation.
- undo: change row 0 to reword, press `u` → verify row 0 is `ActionPick` in `Entries()`.
  Verify `View()` no longer contains "reword" for row 0.
- break: press `b` → verify `Entries()` length increased by 1. Verify the inserted
  entry has `ActionBreak`. Verify `WriteTodo` output contains a "break" line at the
  correct position.
- shell-out `!`: assert `tea.ExecProcess` called with editor + todo file path. Simulate
  editor modifying the file (swap two lines); verify `Entries()` reflects the swap.
- `q` (abort) → verify todo file is unchanged (sequence-editor mode) or no rebase
  command was executed (standalone mode).
- Integration: 4-commit repo; drop commit 2, squash 4 into 3; verify exactly 2 commits
  remain; verify the surviving commits have the correct subjects; verify commit 3's diff
  includes content from both original commits 3 and 4.

`reset`:
- default mode is mixed → verify `View()` contains "[mixed]" in the status bar area.
  Verify the header text uses `ColorFg` (not red or green).
- `s` → mode changes to soft; verify `View()` contains "mode: soft" in header AND
  "[soft]" in status bar. Verify header uses `ColorGreen`.
- `m` → mode changes back to mixed; verify header and status bar reverted.
- `H` (uppercase) → mode changes to hard; verify `View()` contains "⚠ HARD RESET"
  in `ColorRed`. Verify keyhint for enter shows destructive styling.
- `h` (lowercase) → verify mode did NOT change (still whatever it was before). This
  is critical — lowercase must be a no-op to prevent accidental hard resets.
- `<enter>` in mixed mode → `MustHaveCall(tb, r, "reset", "--mixed", "<hash>")`.
  Verify the hash matches the commit under cursor, not a hardcoded value.
- `<enter>` in soft mode → `MustHaveCall(tb, r, "reset", "--soft", "<hash>")`.
- `<enter>` in hard mode → `MustHaveCall(tb, r, "reset", "--hard", "<hash>")`.
- dirty tree + hard mode → verify status bar shows "uncommitted changes will be lost"
  warning. Verify the reset still executes on `<enter>` (warning is informational).
- no commits → statusbar shows "no commits yet"; `MustHaveNoCall(tb, r)`.
  Verify `tea.Quit` was emitted (or app stays showing the error).
- right panel content → verify it updates when cursor moves to a different commit.
  Verify it shows `git diff <selected>..HEAD`, not `git show <selected>`.
- Integration: 3-commit repo; reset --mixed to commit 1; assert `CommitCount` returns 1;
  assert `git status --porcelain` lists the previously committed files as unstaged;
  assert `git diff --cached` is empty (nothing staged after mixed reset).
- Integration: 3-commit repo; reset --hard to commit 1; assert `CommitCount` returns 1;
  assert `git status --porcelain` is empty (clean working tree);
  assert the deleted files no longer exist on disk.

`log`:
- `jig log` → args `["log", "--oneline", "--color=always", "-30"]`.
- `jig log --all` → args include `"--all"`.
- `jig log main..feature` → args include `"main..feature"`.
- `<tab>` to stat level → verify right panel fetches `git diff-tree --stat <hash>`
  with the hash matching the commit under cursor. Verify status bar shows "[ stat]".
  Verify left panel commit list is unchanged (same items, same cursor position).
- `<tab>` to full level → verify right panel fetches `git show <hash>`. Verify cursor
  position preserved.
- `<tab>` 3 times → cycles back to oneline. Verify status bar shows "[ oneline]".
- `/wifi` search → `MustHaveCall(tb, r, "log", "-G", "wifi")` (no `--all` unless
  launched with `--all`). Verify commit list is replaced with search results.
  Verify status bar shows " N results" after search completes.
- `/@javier` → `MustHaveCall(tb, r, "log", "--author=javier")`.
- `/:fix bug` → `MustHaveCall(tb, r, "log", "--grep=fix bug")`.
- Empty search (`/` then `<enter>`) → `MustHaveNoCall(tb, r)`. Verify commit list
  unchanged.
- `<esc>` after search → verify commit list reverts to the full (pre-search) list.
- `F` with staged changes → `PushModelMsg` emitted with a fixup model whose target
  hash matches the commit under cursor. Verify fixup precondition check ran first.
- `F` without staged changes → verify status bar shows "nothing staged". Verify no
  `PushModelMsg` emitted.
- `R` → `PushModelMsg` emitted with rebase model whose base is `<cursor_hash>^`.
- `D` → `PushModelMsg` emitted with diff model whose args include `"<hash>^..<hash>"`.
- Return from mutating sub-command → verify `RefreshMsg` was sent to log model;
  verify commit list was re-fetched (Runner was called with fresh `git log` args).
- Return from read-only sub-command (diff) → verify no `RefreshMsg`; verify commit
  list unchanged.
- Lazy loading: cursor on last item → verify `LoadMoreMsg` emitted; verify Runner
  called with `--skip=30 -30`. Verify new commits appended to list (not replaced).
- Integration: 5-commit repo; search for a unique commit message; verify result
  list has exactly 1 entry; verify the entry's hash matches the expected commit.
- Integration: press `D` on commit 3; verify diff view shows changes from commit 3;
  press `q` in diff; verify log view returns with cursor still on commit 3.

---

**`internal/config`**

- RED first: assert `GT_THEME=light` overrides `theme = "dark"` from toml file.
- Use `t.Setenv` (reverts automatically after test); `t.TempDir()` for config file.
- Env vars must take precedence over file values across all fields.

---

### Cross-command workflow integration tests (`tests/integration/workflow_test.go`)

These tests simulate real user sessions that cross command boundaries via the
AppModel stack. They are the highest-value integration tests because they verify
the transitions, RefreshMsg handling, and git state consistency that unit tests
cannot reach. All use build tag `integration`.

**Workflow 1 — Selective hunk staging from add:**

```
Setup: 3-commit repo. Modify driver.c (creates 2 hunks) and probe.c.

1. Launch add model. Verify 2 files in Unstaged section.
2. Navigate to driver.c, press <enter>.
   → Verify PushModelMsg emitted. Stack depth is now 2.
   → Verify HunkView shows 2 hunks, all lines unselected.
3. Press <a> on hunk 1 (select all lines in hunk 1).
   → Verify status bar: "N lines in 1 hunks selected".
4. Press <enter> to stage.
   → Verify git diff --cached -- driver.c contains hunk 1 content.
   → Verify git diff -- driver.c contains only hunk 2.
   → Verify hunk list re-fetched: only hunk 2 remains.
5. Press <esc> to return to add.
   → Verify PopModelMsg{MutatedGit: true} emitted.
   → Verify add refreshed: driver.c appears in BOTH Staged and Unstaged (partial).
   → Verify probe.c still in Unstaged only.
6. Navigate to probe.c, press <space>.
   → Verify git status --porcelain shows probe.c staged.
7. Press q.
   → Verify final git state: driver.c hunk 1 staged, hunk 2 unstaged,
     probe.c fully staged.
```

**Workflow 2 — Log → fixup round-trip:**

```
Setup: 5-commit repo. Modify a file that commit 3 touched. Stage the change.

1. Launch log model. Verify 5 commits in list.
2. Navigate to commit 3, press F.
   → Verify HasStagedChanges checked first (returns true).
   → Verify PushModelMsg emitted with fixup model targeting commit 3.
3. Fixup model opens. Verify commit list shows same 5 commits.
   Verify cursor is on commit 3 (pre-selected from log).
4. Press <enter> to execute fixup.
   → Verify git commit --fixup=<hash3> called.
   → Verify git rebase --autosquash --interactive <hash3>^ called.
   → Verify PopModelMsg{MutatedGit: true} emitted.
5. Back in log. Verify log refreshed (git log re-executed).
   → Verify commit count is still 5 (fixup doesn't change count).
   → Verify git show <hash3> includes the fixup'd content.
   → Verify cursor repositioned to nearest surviving commit.
```

**Workflow 3 — Log → rebase squash via visual mode:**

```
Setup: 6-commit repo (A, B, C, D, E, F).

1. Launch log model. Verify 6 commits.
2. Navigate to commit C, press R.
   → Verify PushModelMsg emitted with rebase model, base = C^.
3. Rebase model opens. Todo list shows: C, D, E, F (4 entries).
4. Navigate ↓ to D. Press V (enter visual mode).
   → Verify status bar shows "VISUAL (1 rows)".
5. Press ↓ to extend selection to E. Status bar: "VISUAL (2 rows)".
6. Press s (squash). Both D and E become ActionSquash.
   → Verify Entries()[1].Action == ActionSquash.
   → Verify Entries()[2].Action == ActionSquash.
7. Press <esc> to exit visual mode. Press <enter> to execute.
   → Verify rebase completes.
   → Verify PopModelMsg{MutatedGit: true} emitted.
8. Back in log. Verify refresh. Commit count is now 4 (C absorbed D+E).
   → Verify git log --oneline shows exactly 4 commits.
   → Verify the squashed commit's message or diff includes content from D and E.
```

**Workflow 4 — Hunk split then selective staging:**

```
Setup: 2-commit repo. Modify file to create 1 large hunk with internal context.

1. Launch hunk-add standalone for the file. Verify 1 hunk.
2. Press s to split.
   → Verify hunk count increases to 2.
   → Verify both hunks have valid OldStart/NewStart.
   → Verify cursor is on first line of new second hunk.
3. Press n to go back to hunk 1 header. Press <a> to select all of hunk 1.
4. Press <enter> to stage.
   → Verify git diff --cached contains only hunk 1 content.
   → Verify git diff contains only hunk 2 content.
5. Verify hunk list re-fetched: only hunk 2 remains.
```

**Workflow 5 — Reset then re-stage:**

```
Setup: 4-commit repo (A, B, C, D). All committed, clean working tree.

1. Launch reset model. Verify 4 commits, mode = mixed.
2. Navigate to commit B. Right panel shows diff B..HEAD (commits C + D changes).
3. Press <enter>.
   → Verify git reset --mixed <hashB> called.
   → Verify PopModelMsg{MutatedGit: true} emitted.
4. After quit, verify git state:
   → CommitCount returns 2 (A, B).
   → git status --porcelain shows files from C and D as unstaged.
   → git diff --cached is empty (mixed reset = nothing staged).
5. Launch add model. Verify the previously committed files appear in Unstaged.
   Press <a> to re-stage all. Press q.
   → Verify all files staged. Ready for a new commit.
```

**Workflow 6 — Log → diff (read-only, no refresh):**

```
Setup: 5-commit repo.

1. Launch log model. Navigate to commit 3. Press D.
   → Verify PushModelMsg with diff model, args ["<hash3>^..<hash3>"].
2. Diff view opens. Verify file list shows files changed in commit 3.
   Navigate through files, scroll the diff.
3. Press q.
   → Verify PopModelMsg{MutatedGit: false} emitted.
4. Back in log. Verify NO refresh (git log was NOT re-executed).
   → Verify cursor is still on commit 3.
   → Verify commit list is identical (same object, no re-fetch).
```

**Workflow 7 — Partial staging visibility in add:**

```
Setup: 2-commit repo. Modify driver.c to create 2 hunks.

1. Launch hunk-add for driver.c. Select only hunk 1. Press <enter>. Press <esc>.
2. Launch add model.
   → Verify driver.c appears in BOTH Staged and Unstaged sections.
   → Verify Staged entry right panel shows git diff --cached -- driver.c (hunk 1).
   → Verify Unstaged entry right panel shows git diff -- driver.c (hunk 2).
3. Navigate to Staged entry. Press <space> (unstage).
   → Verify git reset HEAD driver.c called.
   → Verify driver.c now appears ONLY in Unstaged (back to fully unstaged).
   → Verify git diff --cached is empty.
```

**Workflow 8 — Checkout bulk restore:**

```
Setup: 2-commit repo. Modify 3 files.

1. Launch checkout model. Verify 3 files in list.
2. Press <A>.
   → Verify status bar shows "⚠ restore all 3 files?"
   → Do NOT press <A> again (timeout).
   → Verify status bar shows "cancelled". Verify all 3 files still in list.
3. Press <A> again. Press <A> within 1 second to confirm.
   → Verify git restore . called.
   → Verify all 3 files gone from list.
   → Verify "working tree clean" shown. Verify PopModelMsg emitted.
4. Verify git state: git diff --name-only is empty. All files match index.
```

These workflow tests are written in Phase 10 (after all individual commands work)
and placed in `tests/integration/workflow_test.go` with build tag `integration`.
They use the real `AppModel` stack (not FakeRunner) — each test creates a temp repo
via `NewTempRepo`, constructs a real `AppModel` with `ExecRunner`, and simulates
keystrokes via the `sendKey`/`sendSpecialKey` test helpers.

---

## Phased execution order

**A phase is done when:** `make fix` produces no diff AND `make test` ≥90% AND `make lint` zero warnings.
**Within every phase:** Red → Green → Refactor. Test file committed before or
simultaneously with the implementation file it tests. No exceptions.

---

### Phase 0 — Skeleton

RED first:
1. `go mod init github.com/jetm/jig` — then set `go 1.26` in `go.mod`.
   Note: `go mod init` (1.26) defaults to `go 1.25.0` (one version back); manually
   edit the directive to `go 1.26` immediately after init.
2. Write `internal/testhelper/fakerunner_test.go` + `gitrepo_test.go` asserting the
   helper contracts (MustHaveCall panics on wrong args, NewTempRepo produces a valid
   git repo).
3. Write `internal/git/runner_test.go` with a smoke test: `ExecRunner.Run(ctx, "--version")`
   returns output containing "git version".
4. Write `internal/git/preconditions_test.go`: `HasStagedChanges` returns true/false
   based on FakeRunner output; `IsRebaseInProgress` checks `.git/rebase-merge`.
5. Write `internal/git/editor_test.go`: `ResolveEditor` respects env var priority.
6. Write `internal/app/app_test.go`: `Push` increases stack depth; `Pop` returns
   previous model; `PopModelMsg{MutatedGit: true}` sends `RefreshMsg` to parent.

GREEN:
4. Implement `internal/testhelper/fakerunner.go` + `gitrepo.go`.
5. Implement `internal/git/runner.go`: `Runner` interface + `ExecRunner`.
6. Implement `internal/git/preconditions.go`: `IsRebaseInProgress`, `HasStagedChanges`,
   `HasCommits`, `IsMergeInProgress`.
7. Implement `internal/git/editor.go`: `ResolveEditor`.
8. Implement `internal/app/app.go`: `AppModel` with stack, `Push`/`Pop`/`Active`,
   `PushModelMsg`/`PopModelMsg`/`RefreshMsg` handling.
9. `cmd/jig/main.go`: cobra root + 8 subcommand stubs that print "not implemented".
   `main.go` creates `Runner`, `Config`, `Renderer`, and `AppModel`.
10. `Makefile` + `.golangci.yml` + `.pre-commit-config.yaml`.
    Run `pre-commit install` to activate the hooks.
11. `LICENSE` — MIT license, copyright `Javier`, current year.

**Acceptance:** `make build` OK. `jig add` prints "not implemented". `LICENSE` exists.
`AppModel` stack transitions work. Preconditions and editor resolution tested.
`make test` ≥90%.

---

### Phase 1 — Core TUI components

RED first (write all `_test.go` files before any implementation):
1. `internal/tui/styles_test.go` — assert all 13 palette constants are valid hex strings.
2. `internal/tui/layout_test.go` — assert `Columns(80)` returns left≥28, left+right==80;
   test at widths 80, 120, 200.
3. `internal/tui/components/itemlist_test.go`
   `internal/tui/components/helpoverlay_test.go`
4. `internal/tui/components/diffview_test.go`
5. `internal/tui/components/statusbar_test.go`

GREEN:
6. Implement `styles.go`, `layout.go`, `itemlist.go`, `diffview.go`, `statusbar.go`,
   `helpoverlay.go`.

**Acceptance:** `make test` ≥90%. Layout math correct at all three tested widths.

---

### Phase 2 — Diff rendering

RED first:
1. `internal/diff/plain_test.go`
2. `internal/diff/chroma_test.go`
3. `internal/diff/delta_test.go`
4. `internal/diff/renderer_test.go`
5. Create `internal/diff/testdata/sample.diff` (10-line unified diff fixture).

GREEN:
6. Implement `plain.go`, `chroma.go`, `delta.go`, `renderer.go` + `Chain()`.

**Acceptance:** `make test` ≥90%. `ChromaRenderer` emits ANSI escape codes on the
fixture. `Chain()` returns the right concrete type for each config.

---

### Phase 3 — `diff` command

RED first:
1. `internal/git/diff_test.go` — all `ParseFileDiff` table cases.
2. `internal/commands/diff_test.go` — assert git args built from revision variants.

GREEN:
3. Implement `internal/git/diff.go`.
4. Implement `internal/commands/diff.go`.
5. Wire into cobra.

**Acceptance:** `jig diff` and `jig diff HEAD~3` work in a real repo. File list populates.
Right panel updates on cursor move. Fuzzy filter works. `make test` ≥90%.

---

### Phase 4 — `add` and `checkout`

RED first:
1. `internal/git/status_test.go` — all `ParseStatus` table cases.
2. `internal/commands/add_test.go` — `<space>`/`<u>`/`<a>` runner assertions.
3. `internal/commands/checkout_test.go` — file restore assertions (`<D>` triggers, `<d>` no-op).

GREEN:
4. Implement `internal/git/status.go`.
5. Implement `internal/commands/add.go`.
6. Implement `internal/commands/checkout.go`.
7. Wire both into cobra.

REFACTOR + Integration:
8. Add `add_integration_test.go` + `checkout_integration_test.go` (tag `integration`).

**Acceptance:** `jig add` stages files. `jig checkout` restores files (with `<D>`
confirm). `make test` ≥90%.

---

### Phase 5 — `hunk-add`

RED first:
1. `internal/git/hunk_test.go` — all `ParseHunks`, `SplitHunks`, `SplitHunkAt`,
   `RecalculateHeader`, `BuildPatch` table cases. Add `FuzzParseHunks` stub.
2. `internal/tui/components/hunkview_test.go` — all HunkView key assertions including
   initial-state-unselected, undo (`u`), split (`s`), navigation (`n`/`N`),
   space-on-context jump, batch staging scope, and editor error handling.
3. `internal/commands/hunk_add_test.go` — FakeRunner assertions + editor round-trip +
   batch staging + apply failure mid-batch.

GREEN:
4. Implement `internal/git/hunk.go` (includes `SplitHunkAt`; `RecalculateHeader`
   preserves `OldStart`/`NewStart`, only recomputes counts).
5. Implement `internal/tui/components/hunkview.go` (undo snapshot, split, batch enter).
6. Implement `internal/commands/hunk_add.go` (state machine: `stateItemList` →
   `stateHunkEdit` → `stateItemList`; `<enter>` from `add` lands in `stateHunkEdit`).
7. Wire into cobra; hook `<enter>` in `add`.

REFACTOR + Integration:
8. Add `hunk_add_integration_test.go`: 2-hunk file; stage hunk 1 only; assert
   `git diff --cached` has hunk 1, `git diff` has hunk 2.
9. Integration: split a large hunk with `s`; select first half; stage; assert only
   first half in `git diff --cached`.

**Acceptance:** `make test` ≥90%.

---

### Phase 6 — `fixup`

RED first:
1. `internal/git/log_test.go` — all `ParseLog` table cases.
2. `internal/commands/fixup_test.go` — precondition, happy path, root-commit cases.

GREEN:
3. Implement `internal/git/log.go`.
4. Implement `internal/commands/fixup.go`.
5. Wire into cobra.

REFACTOR + Integration:
6. `fixup_integration_test.go`: 3-commit repo; stage change for commit 2; fixup;
   assert `CommitCount` still returns 3.

**Acceptance:** `make test` ≥90%.

---

---

### Phase 7 — `log`

RED first:
1. `internal/tui/components/logview_test.go` — detail level toggling, search input
   parsing (bare/`@`/`:`), cross-command message emissions (`F`/`R`/`D`), infinite
   scroll trigger, clipboard yank.
2. `internal/commands/log_test.go` — git args for default/--all/revision ranges,
   detail level data fetching (stat vs full), search query translation to git args,
   state transitions to fixup/rebase/diff models.

GREEN:
3. Implement `internal/tui/components/logview.go` (3-level detail, search input,
   cross-command messages, lazy loading).
4. Implement `internal/commands/log.go` (reuses `ParseLog` from `internal/git/log.go`,
   adds stat/full fetching, search via `git log -G`/`--grep`/`--author`).
5. Wire into cobra.

REFACTOR + Integration:
6. `log_integration_test.go`: 5-commit repo; search unique message; assert 1 result.
   Press `D` on commit; assert diff view launches.

**Acceptance:** all 3 detail levels work. Search returns correct results. `F`/`R`/`D`
transitions launch the correct sub-command. `make test` ≥90%.

---

### Phase 8 — `rebase-interactive`

RED first:
1. `internal/git/rebase_todo_test.go` — all `ParseTodo`/`WriteTodo` cases + round-trip
   property test. Include `break` action, `exec` lines, and `label`/`reset` lines.
2. `internal/tui/components/todolist_test.go` — all TodoList key + rendering assertions
   including visual mode (V + range + action apply), undo (u), break insertion (b/B),
   and shell-out (!).
3. `internal/commands/rebase_interactive_test.go` — standalone mode, sequence-editor
   mode, env assertions, visual mode batch, undo, break, shell-out.

GREEN:
4. Implement `internal/git/rebase_todo.go`.
5. Implement `internal/tui/components/todolist.go`.
6. Implement `internal/commands/rebase_interactive.go` (detect mode from arg, standalone
   vs sequence-editor paths).
7. Wire into cobra; add `gri` alias; add gitconfig snippet to README.

REFACTOR + Integration:
8. `rebase_interactive_integration_test.go`: 4-commit repo; drop commit 2, squash 4
   into 3; assert `CommitCount` returns 2.

**Acceptance:** standalone and sequence-editor modes both work end-to-end.
`make test` ≥90%.

---

### Phase 9 — `reset`

RED first:
1. `internal/commands/reset_test.go` — mode toggling, default mode, runner assertions
   for all three modes, hard-mode uppercase guard, precondition checks.

GREEN:
2. Implement `internal/commands/reset.go` (reuses `ParseLog` from `internal/git/log.go`).
3. Wire into cobra.

REFACTOR + Integration:
4. `reset_integration_test.go`: 3-commit repo; reset --mixed to commit 1; assert
   `CommitCount` returns 1 and working tree has unstaged changes. Repeat for --hard;
   assert clean working tree.

**Acceptance:** all three modes work. Hard mode shows warning on dirty tree.
`make test` ≥90%.

---

### Phase 10 — Config, shell integration, distribution

RED first:
1. `internal/config/config_test.go` — env override, file loading, precedence table.
2. `tests/integration/workflow_test.go` — all 8 cross-command workflow tests (see
   "Cross-command workflow integration tests" section above). These require all
   commands to be implemented, which is why they live in Phase 10.

GREEN:
3. Implement `internal/config/config.go`.
4. Implement `tests/integration/workflow_test.go` — the tests themselves are the
   deliverable (they exercise the full AppModel stack with real git repos).
5. `shell/jig.plugin.fish`.
6. `--version` via ldflags.
7. `.goreleaser.yml` with builds, archives, signs, changelog.
8. Validate config: `goreleaser check`. Test locally: `goreleaser release --snapshot --clean`.
9. `.github/workflows/ci.yml`:
   ```yaml
   - run: make fmt && git diff --exit-code
   - run: make fix && git diff --exit-code
   - run: make vet
   - run: make lint
   - run: make test
   - run: make test-integration
   ```
10. `.github/workflows/release.yml` (tag-triggered, see Versioning section below).
11. Write `README.md` (see README spec below). This is the last file before tagging —
    it documents what the tool does, not what we plan to build.
12. Tag `v0.1.0` once acceptance criteria pass.

**Acceptance:** `goreleaser check` passes. `goreleaser release --snapshot --clean`
produces all targets (tarballs, signed checksums). All 8 workflow integration tests
pass (`make test-integration`). CI green. `README.md` exists with installation,
usage, and all 8 commands documented. `make test` ≥90%.

---

## LICENSE — MIT

The project is released under the MIT License. The `LICENSE` file is created in
Phase 0 and must exist in the repo root from the first commit.

```
MIT License

Copyright (c) <year> Javier

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

Replace `<year>` with the current year at time of creation.

---

## CLAUDE.md size and Claude Code strategy

This document is ~2800 lines (~35K tokens). Claude Code's context window is 200K
tokens. The CLAUDE.md itself consumes ~18% of the window, leaving ~164K tokens for
code, conversation, and tool output. This is manageable for single-phase work but
becomes tight when multiple source files are open simultaneously.

**Mitigation strategies (apply when context pressure increases):**

1. **Work one phase at a time.** Each phase touches 3–6 files. Keep only the files
   for the current phase in context. Use `/compact` between phases.
2. **Split CLAUDE.md into linked files.** If the document grows past ~4000 lines,
   split into:
   - `CLAUDE.md` — architecture, constraints, tech stack, repo layout (~800 lines)
   - `CLAUDE-COMMANDS.md` — all 8 command specifications (~1200 lines)
   - `CLAUDE-TESTS.md` — test strategy, assertions, phase checklist (~600 lines)
   - `CLAUDE-RELEASE.md` — release workflow, goreleaser, README spec (~400 lines)
   Add a one-line `# See also: CLAUDE-COMMANDS.md` cross-reference in each file.
   Claude Code reads all files in the project root automatically.
3. **Use `.claudeignore`** to exclude generated files (`coverage.out`, `bin/`,
   `coverage.html`) from context.
4. **Prefer `@file` references** over pasting file content into the conversation.
   Claude Code can read files on demand without consuming context upfront.

For v0.1.0, the single-file CLAUDE.md is the right choice — it keeps all decisions
in one searchable document and avoids cross-file consistency bugs. Split only when
context pressure makes it necessary.

---

## README spec — written in Phase 9

The `README.md` is the **last file created before tagging `v0.1.0`**. It documents
the shipped tool, not aspirational features. Write it only after all commands work
and CI is green.

**Structure (in this order):**

1. **Title + one-line description** — `# jig` followed by a single sentence:
   "A focused TUI for the git workflows that matter."

2. **Screenshot / demo** — a single terminal screenshot or asciinema link showing
   `jig diff` in action (two-column layout, syntax-highlighted diff). No animated GIF
   walls — one image that communicates the tool's visual identity.

3. **Install** — three methods:
   - **Binary** (GitHub Releases): `curl -sL … | tar xz` one-liner for linux/amd64,
     or "download from Releases". Checksums are signed with cosign.
   - **Go**: `go install github.com/jetm/jig/cmd/jig@latest`
   - **AUR**: `yay -S jig-bin` (planned)
   - Note minimum Go version: 1.26.
   - **Requirements note** (after install methods): truecolor terminal + Nerd Font.
     One sentence: "jig requires a truecolor terminal and a [Nerd Font](https://www.nerdfonts.com/)."
     Link to the Terminal requirements section of this CLAUDE.md if useful, or just
     list the tested terminals inline.

4. **Quick start** — 4–5 lines showing the most common flow:
   ```
   jig add          # stage files interactively
   jig hunk-add     # stage individual hunks
   jig diff         # browse diffs
   jig fixup        # amend into a past commit
   jig reset        # undo commits interactively
   ```

5. **Commands** — one subsection per command (`## add`, `## hunk-add`, etc.), each with:
   - One sentence explaining what it does.
   - The alias (`ga`, `gha`, etc.).
   - A keybinding summary table (abbreviated from this CLAUDE.md — not the full spec).
   - No wireframes in the README — the screenshot covers visual layout.

6. **Shell integration** — how to source `jig.plugin.fish` via `conf.d` or
   fisher, and `JIG_NO_ALIASES=1` to opt out.

7. **Configuration** — the config table from this CLAUDE.md (`GT_THEME`, etc.) and
   the config file path `~/.config/jig/config.toml`.

8. **Diff rendering** — one paragraph explaining the delta → chroma → plain chain
   and how to configure delta.

9. **License** — "MIT" with a link to the `LICENSE` file.

**Tone:** direct, scannable, no marketing fluff. Write for someone who already uses
git daily and wants to know what `jig` does and how to install it in under 60 seconds.
Use the minimum number of words. No badges wall — at most: CI status, Go version,
license.

---

## Versioning and release workflow

**SemVer starting at `v0.1.0`.** The project uses [Semantic Versioning](https://semver.org/):
- `0.x.y` — pre-stable. Breaking changes allowed between minor versions.
- `0.1.0` — first tagged release, cut when all phases (0–10) are complete and CI is green.
- `0.x.0` — bump minor for new commands, features, or breaking config changes.
- `0.x.y` — bump patch for bug fixes, performance improvements, dependency updates.
- `1.0.0` — reserved for when the tool has been daily-driven for ≥1 month with no
  significant issues. Do not tag 1.0.0 prematurely.

**Version injection:** `main.version` is set via ldflags at build time:
```go
// cmd/jig/main.go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```
The Makefile `VERSION` variable reads from `git describe --tags --always --dirty`.
`jig --version` prints `jig version <version> (commit: <hash>, built: <date>)`.

**Branching model:** trunk-based development on `main`. No long-lived feature branches.
Tags trigger releases.

**Release process:**

1. Ensure `main` is green: `make fmt && make fix && make vet && make lint && make test && make test-integration`.
2. Verify `README.md` is complete and reflects the shipped tool (see README spec above).
3. Verify `LICENSE` exists in repo root.
4. Tag: `git tag -s v0.1.0 -m "v0.1.0: initial release"` (signed tags required).
5. Push: `git push origin v0.1.0`.
6. GitHub Actions runs the release workflow (below), which calls goreleaser.
7. goreleaser builds all targets, creates GitHub Release with changelog, uploads
   tarballs, and signs checksums.

**`.github/workflows/release.yml`:**
```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write
  id-token: write  # required for cosign keyless signing via OIDC

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"

      - name: Install cosign
        uses: sigstore/cosign-installer@v3

      - name: Validate goreleaser config
        uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: check

      - name: Run tests
        run: |
          make test
          make test-integration

      - name: Release
        uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**`.goreleaser.yml`:**
```yaml
# yaml-language-server: $schema=https://goreleaser.com/static/schema.json
version: 2

project_name: jig

before:
  hooks:
    - go mod tidy
    - go fix ./...
    - go vet ./...

builds:
  - id: jig
    main: ./cmd/jig
    binary: jig
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{ .Version }} -X main.commit={{ .ShortCommit }} -X main.date={{ .Date }}
    flags:
      - -trimpath

archives:
  - id: default
    formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - LICENSE
      - README.md
      - shell/jig.plugin.fish
    format_overrides:
      - goos: darwin
        formats: [zip]

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

# Sign the checksum file with cosign keyless (GitHub OIDC).
signs:
  - cmd: cosign
    artifacts: checksum
    args:
      - "sign-blob"
      - "--yes"
      - "--output-signature=${signature}"
      - "${artifact}"
    signature: "${artifact}.sig"

# Source archive for reproducibility
source:
  enabled: true
  name_template: "{{ .ProjectName }}_{{ .Version }}_source"

changelog:
  sort: asc
  use: github
  filters:
    exclude:
      - "^Merge"

release:
  github:
    owner: jetm
    name: jig
  draft: false
  prerelease: auto
  name_template: "v{{ .Version }}"
  footer: |
    ## Install
    ```
    go install github.com/jetm/jig/cmd/jig@v{{ .Version }}
    ```
    Or download a binary from the assets below.
```

**Changelog:** goreleaser generates a flat changelog from all commits between tags,
sorted ascending. Merge commits are excluded. The `use: github` option enriches
entries with PR links and author handles. Since commit messages use the kernel-style
`subsystem: description` format, the changelog reads naturally without any grouping
or rewriting — each entry is self-descriptive.

**Cosign keyless signing:** the checksum file is signed via GitHub OIDC (no key to
manage). All archives are verified transitively. Users verify with:
```
cosign verify-blob --signature checksums.txt.sig checksums.txt
```

**Future packaging:** `.deb`, Arch Linux nfpms packages, and AUR publishing via
goreleaser's `nfpms` and `aurs` sections will be added after v0.1.0 once the tool
has been daily-driven and the install base justifies the packaging overhead.

**Post-release checklist:**
- [ ] GitHub Release created with tarballs and signed checksums
- [ ] `jig --version` on downloaded binary shows correct version and commit hash
- [ ] `cosign verify-blob` succeeds on checksums.txt
- [ ] README.md renders correctly on the GitHub repo page
- [ ] Blog post drafted (if milestone release)

**Post-v0.1.0 roadmap:**
- [ ] Add `nfpms` section to goreleaser for `.deb` and Arch Linux packages
- [ ] Add `aurs` section to goreleaser for automated AUR publishing (`jig-bin`)
- [ ] Integrate [git-cliff](https://github.com/orhun/git-cliff) for cumulative
      `CHANGELOG.md` in the repo (goreleaser can consume it via `changelog.use: git-cliff`)
- [ ] Add SBOM generation via goreleaser `sboms` section
