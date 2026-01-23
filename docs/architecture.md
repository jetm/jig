# Architecture

## Two-column layout

Every command renders the same layout: left panel (30% width, min 28 cols) + right diff viewport (70%). Both reflow on `tea.WindowSizeMsg`. Bottom status bar: left-aligned keyhints, right-aligned branch + mode.

## Navigation stack

`app.Model` is the root model managing a stack of command models via push/pop:
- `PushModelMsg` — push a new command onto the stack
- `PopModelMsg` — pop current command (every command emits this on `q`, never `tea.Quit`)
- `RefreshMsg` — re-fetch data after git mutations

## Runner interface

All git mutations go through `os/exec` via `internal/git/runner.go`. The `Runner` interface is the seam for testing — see `file:docs/test-helpers.md` for `FakeRunner`.

## Diff renderer chain

`delta` (external) -> `chroma` (native) -> `plain` fallback. Resolved once at startup via `config.DeltaPath`. All commands receive the resolved `Renderer` at construction.

## Command constructors

All commands receive `Runner`, `Config`, `Renderer`. Commands emit `PopModelMsg` to return, never `tea.Quit` directly.

## Mutation feedback

Every git-mutating action shows confirmation in status bar (`ColorGreen` + `IconSuccess`). Errors use `ColorRed` + `IconError`. Messages auto-clear after 3s or next keypress. After mutation, the left panel re-fetches its data source and rebuilds the list.
