package testhelper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// NewTempRepo creates a git repository in tb.TempDir() with one initial commit.
// Configures user.email and user.name locally so commits work in CI.
func NewTempRepo(tb testing.TB) string {
	tb.Helper()
	dir := tb.TempDir()

	run := func(args ...string) string {
		tb.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			tb.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test User")

	// Create initial commit (git needs at least one file)
	WriteFile(tb, dir, "README.md", "# test repo\n")
	run("add", ".")
	run("commit", "-m", "initial commit")

	return dir
}

// AddCommit stages all changes and creates a commit. Returns the short hash.
func AddCommit(tb testing.TB, repoPath, msg string) string {
	tb.Helper()
	runGit := func(args ...string) string {
		tb.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			tb.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit("add", ".")
	runGit("commit", "-m", msg)
	return runGit("rev-parse", "--short", "HEAD")
}

// WriteFile creates or overwrites repoPath/name using os.Root (Go 1.24+).
func WriteFile(tb testing.TB, repoPath, name, content string) {
	tb.Helper()
	root, err := os.OpenRoot(repoPath)
	if err != nil {
		tb.Fatalf("open root %s: %v", repoPath, err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			tb.Logf("closing root: %v", err)
		}
	}()

	// Ensure parent directory exists
	dir := filepath.Dir(name)
	if dir != "." {
		if err := os.MkdirAll(filepath.Join(repoPath, dir), 0o755); err != nil {
			tb.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	f, err := root.Create(name)
	if err != nil {
		tb.Fatalf("create %s: %v", name, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			tb.Logf("closing file %s: %v", name, err)
		}
	}()
	if _, err := f.Write([]byte(content)); err != nil {
		tb.Fatalf("write %s: %v", name, err)
	}
}

// StageFile runs git add <name> inside repoPath.
func StageFile(tb testing.TB, repoPath, name string) {
	tb.Helper()
	cmd := exec.Command("git", "add", name)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		tb.Fatalf("git add %s: %v\n%s", name, err, out)
	}
}

// CommitCount returns the number of commits reachable from HEAD.
func CommitCount(tb testing.TB, repoPath string) int {
	tb.Helper()
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		tb.Fatalf("git rev-list --count HEAD: %v\n%s", err, out)
	}
	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count); err != nil {
		tb.Fatalf("parsing commit count: %v", err)
	}
	return count
}
