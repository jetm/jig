package git

import (
	"context"
	"fmt"
	"strings"
)

// RepoRoot returns the absolute path to the repository root.
func RepoRoot(ctx context.Context, r Runner) (string, error) {
	out, err := r.Run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// BranchName returns the current branch name, or "HEAD" if in detached HEAD state.
func BranchName(ctx context.Context, r Runner) (string, error) {
	out, err := r.Run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --abbrev-ref HEAD: %w", err)
	}
	return strings.TrimSpace(out), nil
}
