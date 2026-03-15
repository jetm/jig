# jig

An interactive TUI for git workflows, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). One tool replacing the features I use most from [forgit](https://github.com/wfxr/forgit), [diffnav](https://github.com/dlvhdr/diffnav), [tig](https://github.com/jonas/tig), and [git-interactive-rebase-tool](https://github.com/MitMaro/git-interactive-rebase-tool).

## Installation

### From source

```sh
git clone https://github.com/jetm/jig
cd jig
make install   # installs to $GOPATH/bin/jig
```

### Pre-built binaries

Download from the [releases page](https://github.com/jetm/jig/releases), or use goreleaser:

```sh
make snapshot   # produces binaries in dist/
```

## Commands

| Command | Description | Replaces |
|---------|-------------|----------|
| `jig add [paths...]` | Interactively stage files | [forgit](https://github.com/wfxr/forgit) add |
| `jig hunk-add [paths...]` | Interactively stage individual hunks | `git add -p` / [git-add--interactive](https://github.com/cwarden/git-add--interactive) |
| `jig checkout [paths...]` | Interactively discard file changes | `git restore` |
| `jig diff [revision]` | Interactive side-by-side diff viewer | [diffnav](https://github.com/dlvhdr/diffnav) |
| `jig fixup` | Interactively create a fixup commit | [forgit](https://github.com/wfxr/forgit) fixup |
| `jig hunk-checkout [paths...]` | Interactively discard individual hunks | `git checkout -p` |
| `jig hunk-reset [paths...]` | Interactively unstage individual hunks | `git reset -p` |
| `jig log [revision]` | Interactive commit log browser | [tig](https://github.com/jonas/tig) / `git log` |
| `jig rebase-interactive [revision]` | Interactive rebase todo editor | [git-interactive-rebase-tool](https://github.com/MitMaro/git-interactive-rebase-tool) |
| `jig reset [paths...]` | Interactively unstage files | [forgit](https://github.com/wfxr/forgit) reset |

### Command flags

| Command | Flag | Description |
|---------|------|-------------|
| `diff` | `--staged` | Show staged (cached) changes instead of working tree changes |
| `add` | `--interactive`, `-i` | Open TUI even when paths are given (default: stages paths directly) |
| `reset` | `--interactive`, `-i` | Open TUI even when paths are given (default: unstages paths directly) |
| `checkout` | `--direct`, `-d` | Discard files directly without opening TUI |

## Configuration

jig reads configuration from `~/.config/jig/config.yaml` (XDG) or `~/.jig.yaml` as a fallback. Unset fields use built-in defaults.

### Config file format

```yaml
diff:
  renderer: chroma   # chroma | delta | plain

log:
  commitLimit: 50    # number of commits shown in log

rebase:
  defaultBase: HEAD~10   # default base for rebase-interactive

ui:
  theme: dark        # dark | light (theme switching is plumbing only)
  showDiffPanel: true   # show diff panel on startup
  panelRatio: 40        # file list width as percentage [20-80]
  softWrap: true        # soft-wrap long diff lines
  showLineNumbers: true # show line numbers in diff view
```

### Environment variable overrides

Environment variables take precedence over the config file:

| Variable | Config field | Example |
|----------|-------------|---------|
| `JIG_DIFF_RENDERER` | `diff.renderer` | `JIG_DIFF_RENDERER=delta` |
| `JIG_LOG_COMMIT_LIMIT` | `log.commitLimit` | `JIG_LOG_COMMIT_LIMIT=100` |
| `JIG_REBASE_DEFAULT_BASE` | `rebase.defaultBase` | `JIG_REBASE_DEFAULT_BASE=main` |
| `JIG_UI_THEME` | `ui.theme` | `JIG_UI_THEME=light` |
| `JIG_SHOW_DIFF_PANEL` | `ui.showDiffPanel` | `JIG_SHOW_DIFF_PANEL=false` |
| `JIG_PANEL_RATIO` | `ui.panelRatio` | `JIG_PANEL_RATIO=50` |
| `JIG_SOFT_WRAP` | `ui.softWrap` | `JIG_SOFT_WRAP=false` |
| `JIG_SHOW_LINE_NUMBERS` | `ui.showLineNumbers` | `JIG_SHOW_LINE_NUMBERS=false` |

### Diff renderers

- `chroma` (default) — syntax-highlighted diffs rendered in-process
- `delta` — pipe through [delta](https://github.com/dandavison/delta) if installed
- `plain` — uncoloured plain text

## Keybindings

### Universal keys

Available in all two-panel commands (add, checkout, diff, fixup, hunk-add, hunk-checkout, hunk-reset, log, reset).

| Key | Action |
|-----|--------|
| `?` | Toggle help overlay |
| `Tab` | Switch focus between file list and diff panel |
| `D` | Toggle diff panel visibility |
| `F` | Toggle maximize diff panel (full-width) |
| `q` / `Esc` | Quit command (Esc also clears active search) |
| `w` | Toggle soft-wrap in diff panel (when diff is focused) |
| `[` / `]` | Shrink / grow file list panel by 5% (persisted to config) |
| `/` | Search in diff (when diff is focused) |
| `n` / `N` | Next / previous search match |

### File list navigation

When the file list panel is focused.

| Key | Action |
|-----|--------|
| `j` / `Down` | Move to next item |
| `k` / `Up` | Move to previous item |

In maximize mode, `j` and `k` are forwarded to the file list so you can still change files while viewing the full-width diff.

### Diff panel navigation

When the diff panel is focused. These are standard viewport keys.

| Key | Action |
|-----|--------|
| `Up` / `Down` | Scroll one line |
| `PgUp` / `PgDn` | Scroll one page |

### Context and editing keys

Available in most commands that show diffs (add, diff, fixup, hunk-add, log, reset).

| Key | Action | Commands |
|-----|--------|----------|
| `{` / `}` | Decrease / increase diff context lines | add, diff, fixup, hunk-add, log, reset, rebase-interactive |
| `e` | Edit selected diff in `$EDITOR` | add, diff, hunk-add, reset |

### File selection commands (add, reset, checkout)

| Key | Action |
|-----|--------|
| `Space` | Toggle file checked/unchecked |
| `Enter` | Apply action to checked files (stage, unstage, or discard) |
| `a` | Check all files |
| `d` | Uncheck all files |

### Commit keys (add, hunk-add)

| Key | Action |
|-----|--------|
| `c` | Commit staged changes |
| `C` | Commit staged changes with `--amend` |

### Hunk commands (hunk-add, hunk-reset, hunk-checkout)

| Key | Action |
|-----|--------|
| `Space` | Toggle hunk staged/unstaged |
| `j` / `Down` | Next hunk |
| `k` / `Up` | Previous hunk |
| `Enter` | Apply staged hunks (hunk-add: enter line-edit mode) |

hunk-add only:

| Key | Action |
|-----|--------|
| `w` | Apply staged hunks (when file list is focused) |
| `s` | Split current hunk into smaller hunks |

### rebase-interactive

| Key | Action |
|-----|--------|
| `Space` | Cycle through rebase actions (pick, reword, edit, squash, fixup, drop) |
| `p` | Set action to pick |
| `r` | Set action to reword |
| `e` | Set action to edit |
| `s` | Set action to squash |
| `f` | Set action to fixup |
| `d` | Set action to drop |
| `J` / `Ctrl+Down` | Move commit down in the todo list |
| `K` / `Ctrl+Up` | Move commit up in the todo list |
| `w` / `Enter` | Confirm and write the rebase todo |
| `W` | Toggle soft-wrap in diff panel |
| `q` / `Esc` | Abort rebase |

## Shell Completions

Generate and install completions for your shell:

### fish

```sh
jig completion fish > ~/.config/fish/completions/jig.fish
```

## Development

```sh
make build    # build binary to bin/jig
make test     # run tests with race detector and coverage check (90% threshold)
make lint     # run golangci-lint
make fmt      # run gofmt + goimports
make clean    # remove build artifacts
```

## License

[MIT](LICENSE)
