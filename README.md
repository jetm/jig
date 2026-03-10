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
```

### Environment variable overrides

Environment variables take precedence over the config file:

| Variable | Config field | Example |
|----------|-------------|---------|
| `JIG_DIFF_RENDERER` | `diff.renderer` | `JIG_DIFF_RENDERER=delta` |
| `JIG_LOG_COMMIT_LIMIT` | `log.commitLimit` | `JIG_LOG_COMMIT_LIMIT=100` |
| `JIG_REBASE_DEFAULT_BASE` | `rebase.defaultBase` | `JIG_REBASE_DEFAULT_BASE=main` |
| `JIG_UI_THEME` | `ui.theme` | `JIG_UI_THEME=light` |

### Diff renderers

- `chroma` (default) — syntax-highlighted diffs rendered in-process
- `delta` — pipe through [delta](https://github.com/dandavison/delta) if installed
- `plain` — uncoloured plain text

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
