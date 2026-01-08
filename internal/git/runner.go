// Package git provides types and functions for executing git commands and
// querying repository state.
package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner executes git commands and returns stdout.
type Runner interface {
	Run(ctx context.Context, args ...string) (string, error)
	RunWithEnv(ctx context.Context, env []string, args ...string) (string, error)
	RunWithStdin(ctx context.Context, stdin string, args ...string) (string, error)
}

// ExecError wraps a non-zero exit from git with captured stderr.
type ExecError struct {
	Args     []string
	ExitCode int
	Stderr   string
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("git %s: exit %d: %s", e.Args[0], e.ExitCode, e.Stderr)
}

// ExecRunner implements Runner using os/exec.
type ExecRunner struct {
	gitPath string
	repoDir string
}

// NewExecRunner creates a Runner that executes git commands.
// It locates git via exec.LookPath and resolves the repo root via git rev-parse --show-toplevel.
func NewExecRunner(ctx context.Context) (*ExecRunner, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git not found: %w", err)
	}

	// Resolve repo root
	cmd := exec.CommandContext(ctx, gitPath, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	return &ExecRunner{
		gitPath: gitPath,
		repoDir: strings.TrimSpace(string(out)),
	}, nil
}

func (r *ExecRunner) run(ctx context.Context, env []string, stdin string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, r.gitPath, args...)
	cmd.Dir = r.repoDir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", &ExecError{
				Args:     args,
				ExitCode: exitErr.ExitCode(),
				Stderr:   strings.TrimSpace(stderr.String()),
			}
		}
		return "", fmt.Errorf("executing git: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Run executes git with the given args and returns trimmed stdout.
func (r *ExecRunner) Run(ctx context.Context, args ...string) (string, error) {
	return r.run(ctx, nil, "", args)
}

// RunWithEnv executes git with extra environment variables prepended to os.Environ().
func (r *ExecRunner) RunWithEnv(ctx context.Context, env []string, args ...string) (string, error) {
	return r.run(ctx, env, "", args)
}

// RunWithStdin executes git with the provided string piped to stdin.
func (r *ExecRunner) RunWithStdin(ctx context.Context, stdin string, args ...string) (string, error) {
	return r.run(ctx, nil, stdin, args)
}
