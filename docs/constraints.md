# Constraints

- Never use goroutines inside `Update()` — async via `tea.Cmd` only
- Never use go-git for writes — `os/exec` via Runner
- Never call `os.Exit` outside `main.go`
- Never swallow errors — wrap with `fmt.Errorf("context: %w", err)`, surface as `errMsg`
- Never use `any`/`interface{}` when a concrete type or constrained generic is possible
- Go 1.26 minimum: use `slices`, `maps`, `cmp`, `errors.AsType[E]`, `new(expr)`, `min`/`max` builtins, `slog`
