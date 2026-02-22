# Commands

| Command              | Replaces                            |
|----------------------|-------------------------------------|
| `add`                | forgit add                          |
| `hunk-add`           | git add -p / git add --interactive  |
| `checkout`           | git restore (working tree)          |
| `diff`               | diffnav                             |
| `fixup`              | manual git commit --fixup + rebase  |
| `rebase-interactive` | git-interactive-rebase-tool         |
| `reset`              | git reset --soft/--mixed/--hard     |
| `log`                | tig / git log (visual browser)      |

---

# Command Specifications

Detailed command specifications for jig. Each command includes keybindings, UI layout, state transitions, edge cases, and git commands used.

All 8 commands share a single `tea.Program` managed by an `app.Model` root model. Commands are pushed/popped via `PushModelMsg`/`PopModelMsg`. Commands emit `PopModelMsg`, never `tea.Quit` directly.

---

## `add` (alias `ga`) — interactive file staging

Accepts optional path arguments to scope the file list: `jig add src/` shows only files under `src/`. Without arguments, shows all changed files in the repo.

Left panel: files from `git status --short`, grouped as unstaged then untracked. Each entry shows filename + `+N -N` stat from `git diff --stat`.
Right panel: `git diff <file>` (unstaged) or `git diff --cached <file>` (staged), rendered via `diff.Chain(cfg)`.

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

The left panel groups files in three sections: **Staged** (index changes, shown first so the user sees what's already prepared), **Unstaged** (working tree changes), and **Untracked** (new files). Sections with zero items are hidden. `<space>` toggles a file between sections; `<u>` unstages explicitly.

**Partially staged files** (git status shows `MM`, `MD`, etc.): a file with changes in both the index and the working tree appears in **both** Staged and Unstaged sections simultaneously. Each entry is a distinct `StatusEntry` — the Staged entry shows `git diff --cached` in the right panel, the Unstaged entry shows `git diff`. `<space>` on the Staged entry unstages it; `<space>` on the Unstaged entry stages the remaining working tree changes.

Keybindings:
- `j`/`k` or `↑`/`↓`: move file cursor
- `Home` / `End`: jump to first / last file
- `/`: open fuzzy filter input (filters across all sections by filename)
- `<space>`: **toggle staging.** If file is in Unstaged or Untracked section → `git add <file>` (stages it). If file is in Staged section → `git reset HEAD <file>` (unstages it). List re-fetches from `git status` after each toggle.
- `<u>`: unstage selected file → `git reset HEAD <file>`. No-op on untracked/unstaged files.
- `<a>`: stage all → `git add -A`. All files move to "Staged".
- `<enter>`: descend into `hunk-add` for selected file (state machine, not subprocess). Only available on modified files (not untracked).
- `{` / `}`: jump to previous / next section header.
- `y`: copy file path to clipboard.
- `q` / `<esc>`: emit `PopModelMsg`

---

## `hunk-add` (alias `gha`) — line/hunk-level staging

Replaces `git add -p`. Can be launched standalone or entered from `add` via `<enter>`.

**Standalone mode:** left panel shows changed files (same source as `add`). Select a file with `<enter>` to descend into HunkView. `<esc>` from HunkView returns to file list. `q` or `<esc>` from the file list itself emits `PopModelMsg{MutatedGit: staged}`.

**Hunk editing mode:** left panel is replaced by `HunkView` (full left column height). Right panel shows the same diff rendered for context, scrolled to match the left panel's cursor position.

```
+------------------------+-----------------------------------------------+
|  @@ -42,7 +42,10 @@   |  diff --git a/driver.c b/driver.c             |
|   int ret;             |  @@ -42,7 +42,10 @@                          |
| - old_init_sequence(); |   int ret;                                    |
|+ new_init_a();        |  -old_init_sequence();                        |
|+ new_init_b();        |  +new_init_a();                               |
|   return ret;          |  +new_init_b();                               |
|  @@ -88,4 +88,4 @@    |   return ret;                                 |
| - legacy_probe();     |                                               |
| + modern_probe();     |                                               |
+------------------------+-----------------------------------------------+
| ␣ toggle  a hunk  A all  s split  u undo  e edit  ⏎ stage  q back   |
| 3 lines in 2 hunks selected                                          |
```

**Initial selection state:** all togglable lines start unselected. The user opts in to exactly which lines to stage.

**Toggle semantics for `+` and `-` lines:**
- `+` line **selected** (): the addition will be staged
- `+` line **unselected** (): the addition stays in the working tree only
- `-` line **selected** (): the deletion will be staged
- `-` line **unselected** (): the deletion stays in working tree; in the patch, demoted to context (` `)

Keybindings (hunk editing mode):
- `j`/`k` or `↑`/`↓`: move cursor between lines
- `Home` / `End`: jump to first / last line in file
- `n`/`N`: jump to next/previous `@@` hunk header
- `<space>`: toggle selected line. If on non-togglable line, jump to next togglable line.
- `<a>`: toggle all togglable lines in current hunk
- `<A>`: toggle all togglable lines in file
- `s`: **split current hunk** at first run of ≥1 context lines within it. No-op if no internal context boundary.
- `u`: **undo** — restore previous selection state of current hunk (self-inverting toggle)
- `e`: open current hunk in `$EDITOR` (see editor flow below)
- `<enter>`: **stage all hunks that have any selected lines**, applied sequentially. After staging, hunk list is re-fetched from `git diff`.
- `q` / `<esc>`: return to file list (discards unsaved toggle selections, but hunks already staged via `<enter>` are persisted)

**Selection counter:** status bar shows "N lines in M hunks selected" in `ColorFgSubtle` *italic*.

**Line state visual encoding:**
- `` = selected (will be staged); `ColorGreen`, normal weight
- `` = unselected; `ColorFgSubtle`, *italic*
- Context lines: always included in patch; not togglable; *italic* muted
- `@@` hunk headers: **bold** `ColorCyan`; not togglable

### Patch construction (`internal/git/hunk.go`)

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
```

Key functions:
- `ParseHunks(rawDiff string) ([]Hunk, error)` — parses raw unified diff into structured hunks
- `SplitHunks(h []Hunk) []Hunk` — splits multi-hunk diff into individual single-hunk patches
- `SplitHunkAt(h Hunk) (Hunk, Hunk, bool)` — splits single hunk at first context boundary
- `RecalculateHeader(h *Hunk)` — recomputes `@@` header counts from actual lines
- `BuildPatch(h Hunk) (string, error)` — assembles valid unified diff from a single hunk

**BuildPatch algorithm (single-pass O(n)):**
1. Count selected togglable lines. If zero, return `ErrNothingSelected`.
2. Emit `h.FileHeader` (if non-empty).
3. Emit `h.Header`.
4. For each line:
   - Selected `+`: emit as `+<content>`
   - Unselected `+`: **drop** (stays in working tree)
   - Selected `-`: emit as `-<content>` (deletion staged)
   - Unselected `-`: emit as ` <content>` (**demote to context**)
   - Context ` `: emit as ` <content>` (always included)
5. Ensure output ends with newline.

### `$EDITOR` editing flow (the `e` key)

1. Snapshot the current hunk state (for undo on failure).
2. Write current hunk to a temp file with `#` comment annotations.
3. Resolve editor via `git.ResolveEditor(ctx, runner)`. Open with `tea.ExecProcess`.
4. On editor exit:
   - Non-zero exit: discard edit, restore snapshot, show error.
   - Read back, strip `#` comment lines.
   - Empty result: restore snapshot, show "empty edit discarded".
   - Re-parse into Hunk via `ParseHunks`, call `RecalculateHeader`.
   - Replace current hunk in HunkView. User reviews before staging with `<enter>`.

### Batch multi-hunk staging (`<enter>`)

All hunks with selected lines are staged sequentially via `git apply --cached -` (one per hunk). After staging, hunk list is re-fetched from `git diff`. If re-fetch returns empty diff, automatically return to file list.

If any `git apply` fails mid-batch: show error, stop batch, re-fetch hunk list.

---

## `checkout` (alias `gc`) — file checkout (restore working tree files)

Uses `git restore` (Git 2.23+), the modern replacement for `git checkout -- <file>`.

**Precondition:** at least one modified file exists (`git diff --name-only` non-empty). If no files are modified, show "working tree clean" and emit `PopModelMsg{MutatedGit: false}`.

Left panel: `git diff --name-only` (unstaged modified files), each with `+N -N` stat.
Right panel: `git diff -- <file>` via `diff.Chain(cfg)`.

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
- `<D>` (uppercase): `git restore <file>` (destructive — lowercase `d` is a no-op). File disappears from list. If last file restored, show " working tree clean" and quit.
- `<A>` (uppercase): restore ALL files (`git restore .`). Requires double-press within 1 second to confirm.
- `y`: copy file path to clipboard.
- `q` / `<esc>`: emit `PopModelMsg`

---

## `diff` (alias `gd`) — diffnav-style viewer

Read-only. File list on the left, full diff on the right.

Accepts optional revision argument: `jig diff HEAD~3`, `jig diff main..feature`. Default: `git diff` (unstaged changes). `jig diff --staged` shows `git diff --cached`.

Left panel: list of changed files with `+N -N` stats and color-coded Nerd Font icons (=modified, =added, =deleted, 󰕓=renamed). Fuzzy-filterable with `/`.
Right panel: full diff of selected file via `diff.Chain(cfg)`.

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
- `<esc>`: context-dependent — if right panel focused, return to left; if left panel focused, emit `PopModelMsg`.
- `q`: always emit `PopModelMsg{MutatedGit: false}`
- `y`: copy current file path to clipboard

### Data types

```go
type FileDiff struct {
    Name     string // relative file path
    OldName  string // original path if renamed (empty otherwise)
    Status   rune   // 'M' modified, 'A' added, 'D' deleted, 'R' renamed
    Added    int    // lines added (from --stat)
    Deleted  int    // lines deleted (from --stat)
}

func ParseFileDiff(nameStatus, stat string) ([]FileDiff, error) {}
func SplitMultiFileDiff(raw string) ([]StdinFileDiff, error) {}
```

### Piped input (`--stdin`)

```
git diff main..feature | jig diff --stdin
gh pr diff 123 | jig diff --stdin
```

Reads entire unified diff from stdin into `[]StdinFileDiff` (ordered slice, not map — map iteration order is non-deterministic in Go). A `map[string]int` index maps filenames to slice positions for O(1) lookup. Stdin is consumed once and never re-read. `--stdin` mode is read-only.

---

## `fixup` (alias `gfix`) — amend staged changes into a commit

**Preconditions:**
1. Staged changes exist (`git diff --cached --quiet` exits non-zero). Error: "nothing staged".
2. Not mid-rebase/merge.

Left panel: recent commits from `git log --oneline` (default depth: 30).
Right panel: `git show <hash>` via `diff.Chain(cfg)`.

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
- `q` / `<esc>`: emit `PopModelMsg{MutatedGit: false}`

No `/` filter — commit list is short (default 30 entries).

**Execution on `<enter>`:**
1. `git commit --fixup=<hash>`
2. `GIT_SEQUENCE_EDITOR=true git rebase --autosquash --interactive <hash>^`
3. Root commit edge case: if `<hash>^` fails, retry with `--root` flag.
4. Conflict: check `.git/rebase-merge/stopped-sha`. If present, show "conflict at <sha> — resolve then git rebase --continue". Do NOT auto-abort.
5. Success: show " fixup applied", emit `PopModelMsg{MutatedGit: true}`.

---

## `rebase-interactive` (alias `gri`) — visual todo editor

Drop-in replacement for `$GIT_SEQUENCE_EDITOR`. Two usage paths:

**Path A — standalone:** `jig rebase-interactive [base]`
Reads commits from `git log --reverse`, presents todo list, executes via `GIT_SEQUENCE_EDITOR="cp <tempfile>" git rebase -i <base>`.

**Path B — sequence editor:** `git config --global sequence.editor "jig rebase-interactive"`
When called with a file path argument, reads existing todo file, presents editor, writes result back on `<enter>`.

**Mode detection:** inspects first argument via `os.Stat()`. Existing file → Path B; otherwise → Path A.

Left panel: ordered todo list (full terminal width by default).
Right panel: `git show <hash>` of commit under cursor (hidden on startup — press `D` to show).

The diff panel is hidden by default so the command launches instantly without fetching diffs. Press `D` to toggle it on; the layout switches to a 45/55 left/right split to give commit subjects more room.

```
+------------------------------------------------------+
| 5 commits on feature                                 |
|  pick   a1b2c3 fix wifi scan in mt7927               |
|  pick   d4e5f6 add init sequence                     |
|  squash 7g8h9i cleanup headers                       |
|  drop   0j1k2l debug logging                         |
|  pick   3m4n5o add probe callbacks                   |
+------------------------------------------------------+
| p r e s f d  K/J reorder  D: diff  ⏎ go  q          |
```

With diff visible (`D` pressed):

```
+----------------------+-----------------------------------+
| 5 commits on feature |  commit a1b2c3                    |
|  pick   a1b2c3 ...   |  diff --git a/driver.c b/driver.c |
|  pick   d4e5f6 ...   |  @@ -42,7 ...                    |
|  squash 7g8h9i ...   |  ...                              |
+----------------------+-----------------------------------+
| p r e s f d  K/J reorder  D: diff  Tab: panel  ⏎ go  q |
```

### Action keybindings

| Key       | Action  | Description                                                |
|-----------|---------|------------------------------------------------------------|
| `p`       | pick    | use commit as-is                                           |
| `r`       | reword  | use commit; git stops to edit message                      |
| `e`       | edit    | use commit; git stops after applying for amending          |
| `s`       | squash  | meld into previous commit, keep both messages              |
| `f`       | fixup   | meld into previous commit, discard this message            |
| `d`       | drop    | remove commit entirely (rendered dim + strikethrough)      |

### Navigation and editing

- `k` / `j`: move row up / down (reorder, swap with adjacent row)
- `↑` / `↓`: navigate cursor between rows
- `Home` / `End`: jump to first / last entry
- `D`: toggle diff panel. Hidden on startup. When shown, uses a 45/55 split; `Tab` to switch focus between panels.
- `Tab`: switch focus between left and right panels (only active when diff panel is visible)
- `V`: enter visual mode — select range of rows
- `u`: undo last action (self-inverting toggle)
- `b`: insert `break` line below cursor (pause point)
- `B`: remove `break` line under cursor
- `!`: open todo in `$EDITOR` for manual editing
- `y`: copy commit hash to clipboard
- `<enter>`: execute rebase (Path A) or write todo file (Path B)
- `q`/`<esc>`: abort — discard changes and exit

### Visual mode (`V`)

Press `V` to enter visual mode at current row. Navigate with `↑`/`↓` to extend selection. While in visual mode:
- Action keys (`p`, `r`, `e`, `s`, `f`, `d`): apply to **all selected rows**
- `k`/`j`: move entire selected block up/down as a unit
- `<esc>` or `V` again: exit visual mode, keep changes

### Visual rendering by action

| Action  | Icon | Rendering                                       |
|---------|------|-------------------------------------------------|
| pick    |  | `ColorBlue` **bold**, subject `ColorFg`           |
| reword  |  | `ColorYellow` **bold**, subject `ColorFg`         |
| edit    |  | `ColorCyan` **bold**, subject `ColorFg`           |
| squash  |  | `ColorPurple` **bold**, subject indented          |
| fixup   |  | `ColorPurple` *italic* + dim, subject indented    |
| drop    |  | entire row `ColorRed` dim + strikethrough         |

### Undo (`u`)

Single-level undo. Before each undoable action, clone `[]TodoEntry` into `prevEntries`. `u` swaps `entries` and `prevEntries` (self-inverting toggle). O(n) clone, O(1) swap.

### Quit semantics

**Path A:** `q`/`<esc>` emits `PopModelMsg{MutatedGit: false}`. `<enter>` runs rebase, emits `PopModelMsg{MutatedGit: true}`.

**Path B:** `q`/`<esc>` sets `model.aborted = true` and emits `tea.Quit`. `<enter>` writes todo file and emits `tea.Quit`. After `Run()` returns, `main.go` checks `model.aborted` — if true, `os.Exit(1)` so git aborts the rebase.

### Data types (`internal/git/rebase_todo.go`)

```go
type Action string

const (
    ActionPick   Action = "pick"
    ActionReword Action = "reword"
    ActionEdit   Action = "edit"
    ActionSquash Action = "squash"
    ActionFixup  Action = "fixup"
    ActionDrop   Action = "drop"
    ActionBreak  Action = "break"
)

type TodoEntry struct {
    Action  Action
    Hash    string
    Subject string
}

type TodoEntries []TodoEntry

func ParseTodo(raw string) (TodoEntries, error) {}
func WriteTodo(entries TodoEntries) string {}
```

---

## `reset` (alias `gr`) — interactive reset (soft / mixed / hard)

Visual interface for `git reset` with mode selection.

**Preconditions:**
1. Not mid-rebase/merge.
2. At least one commit exists. Error: "no commits yet".

**Modes (toggled by keypress):**

| Mode    | Key  | Git command                  | Effect                                        |
|---------|------|------------------------------|-----------------------------------------------|
| mixed   | `m`  | `git reset --mixed <hash>`   | Unstage changes, keep working tree (default)  |
| soft    | `s`  | `git reset --soft <hash>`    | Keep staged + working tree                    |
| hard    | `H`  | `git reset --hard <hash>`    | Discard everything (destructive — uppercase) |

Left panel: recent commits from `git log --oneline` (depth: `JIG_LOG_COMMIT_LIMIT`).
Right panel: `git diff <selected_hash>..HEAD` — accumulated changes between target and HEAD.

```
+------------------------+-----------------------------------------------+
| Reset to (mode: mixed) |  3 commits, 5 files  +42 -15                 |
|  a1b2c3 fix wifi scan |  diff --git a/driver.c b/driver.c             |
| >  d4e5f6 add mt7927  |  @@ -42,7 +42,10 @@                          |
|  7g8h9i init probe    |  ...                                          |
+------------------------+-----------------------------------------------+
| s soft  m mixed  H HARD  ⏎ reset  q quit          [ main] [reset]   |
```

**Right panel summary line:** First line shows `N commits, M files  +A -D` from `git diff --stat`.

Keybindings:
- `j`/`k` or `↑`/`↓`: move commit cursor
- `Home` / `End`: jump to first / last commit
- `s`: set mode to soft
- `m`: set mode to mixed (default)
- `H` (uppercase): set mode to hard — status bar shows `⚠ HARD RESET` in `ColorRed`
- `<enter>`: execute `git reset --<mode> <hash>`
- `y`: copy commit hash to clipboard
- `q` / `<esc>`: emit `PopModelMsg{MutatedGit: false}`

**Hard reset safety:** keyhint changes to `<enter> RESET (destructive)` in `ColorRed`. If working tree has uncommitted changes, show warning "uncommitted changes will be lost".

**Execution:** On success: " reset to <short_hash> (<mode>)", emit `PopModelMsg{MutatedGit: true}`. On error: show in statusbar, do NOT quit.

**Visual rendering by mode:**

| Mode  | Header text color | Status bar indicator        |
|-------|-------------------|-----------------------------|
| mixed | `ColorFg`         | `[mixed]` in `ColorFgSubtle`|
| soft  | `ColorGreen`      | `[soft]` in `ColorGreen`    |
| hard  | `ColorRed`        | `⚠ HARD RESET` in `ColorRed`|

---

## `log` (alias `gl`) — visual commit browser

Interactive commit log viewer with three detail levels. Replaces `git log` / tig.

**Default scope:** current branch (`git log HEAD`). `jig log --all` for all branches. Accepts revision arguments: `jig log main..feature`.

### Three detail levels (cycle with `<tab>`)

| Level     | Left panel                  | Right panel                            |
|-----------|-----------------------------|----------------------------------------|
| **oneline** (default) | ` hash subject` one line per commit | Commit metadata: full message, author, date, refs |
| **stat**   | Same as oneline | `git diff-tree --stat <hash>` — file list with stats |
| **full**   | Same as oneline | `git show <hash>` via `diff.Chain(cfg)` — full diff |

`<tab>` cycles forward, `<shift+tab>` cycles backward. Current level shown in status bar.

```
+------------------------+-----------------------------------------------+
| Commits [ oneline]    |  commit d4e5f6                                |
|  a1b2c3 fix wifi scan |  Author: Javier <javier@example.com>          |
| >  d4e5f6 add mt7927  |  Date:   2 hours ago                          |
|  7g8h9i init probe    |  Add MT7927 WiFi driver initialization        |
|  0j1k2l add firmware  |  Refs: HEAD -> main, origin/main              |
+------------------------+-----------------------------------------------+
| tab level  / search  F fixup  R rebase  D diff  y yank  q quit        |
```

Keybindings:
- `j`/`k` or `↑`/`↓`: move commit cursor
- `Home` / `End`: jump to first / last loaded commit
- `<tab>` / `<shift+tab>`: cycle detail level
- `<enter>`: move focus to right panel (scroll mode)
- `<esc>`: context-dependent — right panel focused → return to left; left panel focused → emit `PopModelMsg`. Search active → clear search.
- `/`: open search input
- `F`: **fixup** staged changes into selected commit (launches fixup command). Precondition: staged changes must exist.
- `R`: **rebase-interactive** from selected commit (standalone mode with `<hash>^` as base). Root commit: use `--root`.
- `D`: **diff** for selected commit (`<hash>^..<hash>`). Root commit: use `git diff --root <hash>`.
- `y`: copy commit hash to clipboard
- `q`: always emit `PopModelMsg{MutatedGit: false}`

### Infinite scroll / lazy loading

Loads `JIG_LOG_COMMIT_LIMIT` commits initially (default: 50). When cursor reaches last loaded commit, next batch fetched via `git log --skip=<loaded> -<N>` as `tea.Cmd`. `End` loads up to 500 additional commits (capped).

### Search (`/`)

| Prefix     | Behavior                                                     |
|------------|--------------------------------------------------------------|
| (none)     | Grep commit messages AND diffs (`git log -G <query>`)        |
| `@`        | Filter by author (`git log --author=<query>`)                |
| `:`        | Grep commit messages only (`git log --grep=<query>`)         |

Search is asynchronous via `tea.Cmd`. Results replace the commit list. `<esc>` clears search and returns to full list. `n`/`N` jump to next/prev match in right panel.

### Cross-command integration

`F`, `R`, `D` are **state machine transitions** (push model onto stack), not subprocesses. On quit from sub-command, return to log at same cursor position.

### Data types (`internal/git/log.go`, `internal/git/status.go`)

```go
type Commit struct {
    Hash       string
    Subject    string
    Author     string
    AuthorDate time.Time
    Refs       []string
}

type Commits []Commit

func ParseLog(raw string) (Commits, error) {}
```

```go
type StatusEntry struct {
    Path     string
    Index    rune   // ' ', 'M', 'A', 'D', 'R', '?', '!'
    WorkTree rune   // ' ', 'M', 'A', 'D', 'R', '?'
}

type StatusEntries []StatusEntry

func ParseStatus(raw string) (StatusEntries, error) {}
```
