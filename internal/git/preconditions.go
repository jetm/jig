package git

import (
	"context"
	"os"
	"path/filepath"
)

// IsRebaseInProgress returns true if .git/rebase-merge or .git/rebase-apply exists.
func IsRebaseInProgress(ctx context.Context, r Runner) bool {
	root, err := RepoRoot(ctx, r)
	if err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "rebase-merge")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(root, ".git", "rebase-apply")); err == nil {
		return true
	}
	return false
}

// HasStagedChanges returns true if git diff --cached --quiet exits non-zero.
func HasStagedChanges(ctx context.Context, r Runner) bool {
	_, err := r.Run(ctx, "diff", "--cached", "--quiet")
	return err != nil
}

// HasCommits returns true if git rev-parse HEAD succeeds.
func HasCommits(ctx context.Context, r Runner) bool {
	_, err := r.Run(ctx, "rev-parse", "HEAD")
	return err == nil
}

// IsMergeInProgress returns true if .git/MERGE_HEAD exists.
func IsMergeInProgress(ctx context.Context, r Runner) bool {
	root, err := RepoRoot(ctx, r)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(root, ".git", "MERGE_HEAD"))
	return err == nil
}
