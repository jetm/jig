# Testing

The `tdd` rule defines TDD workflow and test integrity requirements. Go-specific patterns:

**Table-driven tests:**
```go
tests := []struct {
    name string
    // fields
}{
    {"empty input", …},
    {"single file", …},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // test body
    })
}
```

**Assertion library:** `testify/require` for fatal checks, `testify/assert` for non-fatal. Use `require` for setup preconditions, `assert` for test assertions.

**Benchmarks:** Use `b.Loop()` (Go 1.26+), not `for i := 0; i < b.N; i++`. Use `b.ArtifactDir()` for benchmark output files.

**`t.Parallel()`** on every test and subtest unless it mutates shared git state.

**Commit messages:** kernel-style `subsystem: description` (e.g. `commands/add: fix cursor after staging last file`).

See `file:docs/test-helpers.md` for `FakeRunner`, `testhelper` package, and per-package coverage requirements.
