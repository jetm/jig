# gti

An interactive TUI for git workflows, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Installation

### From source

```sh
git clone https://github.com/jetm/gti
cd gti
make install   # installs to $GOPATH/bin/gti
```

### Pre-built binaries

Download from the [releases page](https://github.com/jetm/gti/releases), or use goreleaser:

```sh
make snapshot   # produces binaries in dist/
```

## Commands

| Command | Description |
|---------|-------------|
| `gti diff [revision]` | Interactive side-by-side diff viewer. Use `--staged` for cached changes. |
| `gti add` | Interactively stage files |
| `gti reset` | Interactively unstage files |
| `gti checkout` | Interactively discard file changes |
| `gti hunk-add` | Interactively stage individual hunks |
| `gti fixup` | Interactively create a fixup commit targeting a recent commit |
| `gti log [revision]` | Interactive commit log browser |
| `gti rebase-interactive [revision]` | Interactive rebase todo editor |

## Configuration

gti reads configuration from `~/.config/gti/config.yaml` (XDG) or `~/.gti.yaml` as a fallback. Unset fields use built-in defaults.

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
| `GTI_DIFF_RENDERER` | `diff.renderer` | `GTI_DIFF_RENDERER=delta` |
| `GTI_LOG_COMMIT_LIMIT` | `log.commitLimit` | `GTI_LOG_COMMIT_LIMIT=100` |
| `GTI_REBASE_DEFAULT_BASE` | `rebase.defaultBase` | `GTI_REBASE_DEFAULT_BASE=main` |
| `GTI_UI_THEME` | `ui.theme` | `GTI_UI_THEME=light` |

### Diff renderers

- `chroma` (default) — syntax-highlighted diffs rendered in-process
- `delta` — pipe through [delta](https://github.com/dandavison/delta) if installed
- `plain` — uncoloured plain text

## Shell Completions

Generate and install completions for your shell:

### bash

```sh
gti completion bash > /etc/bash_completion.d/gti
# or for the current user:
gti completion bash > ~/.local/share/bash-completion/completions/gti
```

### zsh

```sh
gti completion zsh > "${fpath[1]}/_gti"
```

### fish

```sh
gti completion fish > ~/.config/fish/completions/gti.fish
```

### PowerShell

```powershell
gti completion powershell | Out-String | Invoke-Expression
```

## Development

```sh
make build    # build binary to bin/gti
make test     # run tests with race detector and coverage check (90% threshold)
make lint     # run golangci-lint
make fmt      # run gofmt + goimports
make clean    # remove build artifacts
```
