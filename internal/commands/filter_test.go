package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandGlobs_EmptyInput(t *testing.T) {
	t.Parallel()
	result := expandGlobs(nil)
	if result != nil {
		t.Errorf("expandGlobs(nil) = %v, want nil", result)
	}
	result = expandGlobs([]string{})
	if result != nil {
		t.Errorf("expandGlobs([]) = %v, want nil", result)
	}
}

func TestExpandGlobs_SingleLiteral(t *testing.T) {
	t.Parallel()
	result := expandGlobs([]string{"foo.go"})
	if len(result) != 1 || result[0] != "foo.go" {
		t.Errorf("expandGlobs([foo.go]) = %v, want [foo.go]", result)
	}
}

func TestExpandGlobs_GlobMatchingFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create test files.
	for _, name := range []string{"a.go", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatalf("creating test file: %v", err)
		}
	}

	pattern := filepath.Join(dir, "*.go")
	result := expandGlobs([]string{pattern})
	if len(result) != 2 {
		t.Errorf("expandGlobs([%s]) = %v, want 2 .go files", pattern, result)
	}
}

func TestExpandGlobs_GlobMatchingNoFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pattern := filepath.Join(dir, "*.xyz")
	result := expandGlobs([]string{pattern})
	if result != nil {
		t.Errorf("expandGlobs with no matches = %v, want nil", result)
	}
}

func TestExpandGlobs_MixedLiteralAndGlob(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), nil, 0o600); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	pattern := filepath.Join(dir, "*.go")
	result := expandGlobs([]string{"literal.go", pattern})
	if len(result) != 2 {
		t.Errorf("expandGlobs(mixed) = %v, want [literal.go, %s/a.go]", result, dir)
	}
	if result[0] != "literal.go" {
		t.Errorf("first element = %q, want literal.go", result[0])
	}
}

func TestExpandGlobs_MultipleGlobsAllMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.ts"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatalf("creating test file: %v", err)
		}
	}

	goGlob := filepath.Join(dir, "*.go")
	tsGlob := filepath.Join(dir, "*.ts")
	result := expandGlobs([]string{goGlob, tsGlob})
	if len(result) != 2 {
		t.Errorf("expandGlobs(two globs) = %v, want 2 files", result)
	}
}
