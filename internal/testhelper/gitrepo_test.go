package testhelper_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jetm/gti/internal/testhelper"
)

func TestNewTempRepo(t *testing.T) {
	dir := testhelper.NewTempRepo(t)

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf(".git directory not found: %v", err)
	}

	if count := testhelper.CommitCount(t, dir); count != 1 {
		t.Fatalf("expected 1 commit, got %d", count)
	}
}

func TestAddCommit(t *testing.T) {
	dir := testhelper.NewTempRepo(t)

	testhelper.WriteFile(t, dir, "file.txt", "hello\n")
	hash := testhelper.AddCommit(t, dir, "add file")

	if len(hash) != 7 {
		t.Fatalf("expected 7-char short hash, got %q (len %d)", hash, len(hash))
	}

	if count := testhelper.CommitCount(t, dir); count != 2 {
		t.Fatalf("expected 2 commits, got %d", count)
	}
}

func TestWriteFile(t *testing.T) {
	dir := testhelper.NewTempRepo(t)

	testhelper.WriteFile(t, dir, "hello.txt", "world\n")

	content, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "world\n" {
		t.Fatalf("expected %q, got %q", "world\n", string(content))
	}
}
