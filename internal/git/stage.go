package git

import (
	"context"
	"fmt"
	"strings"
)

// StatusFile represents a file with a working-tree status.
type StatusFile struct {
	Path   string
	Status FileStatus
}

// ListUnstagedFiles returns the list of working-tree modified files (unstaged changes)
// plus untracked files. It runs two git commands:
//  1. git diff --name-status (modified/deleted/renamed working-tree files)
//  2. git ls-files --others --exclude-standard (untracked files)
func ListUnstagedFiles(ctx context.Context, r Runner) ([]StatusFile, error) {
	// Working-tree changes (modified, deleted, renamed)
	nameStatus, err := r.Run(ctx, "diff", "--name-status")
	if err != nil {
		return nil, fmt.Errorf("git diff --name-status: %w", err)
	}

	var files []StatusFile
	for line := range strings.SplitSeq(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sf := parseNameStatusLine(line)
		files = append(files, sf)
	}

	// Untracked files
	untracked, err := r.Run(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("git ls-files --others: %w", err)
	}

	for line := range strings.SplitSeq(untracked, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, StatusFile{Path: line, Status: Added})
	}

	return files, nil
}

// ListUnstagedFilesFiltered returns unstaged files optionally filtered by paths.
// When paths is empty it behaves identically to ListUnstagedFiles.
// When paths is non-empty it appends "-- <paths>" to the git diff calls so only
// the specified files are included.
func ListUnstagedFilesFiltered(ctx context.Context, r Runner, paths []string) ([]StatusFile, error) {
	args := []string{"diff", "--name-status"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	nameStatus, err := r.Run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff --name-status: %w", err)
	}

	var files []StatusFile
	for line := range strings.SplitSeq(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sf := parseNameStatusLine(line)
		files = append(files, sf)
	}

	// Untracked files (only when no path filter; path-filtered untracked is complex)
	if len(paths) == 0 {
		untracked, err := r.Run(ctx, "ls-files", "--others", "--exclude-standard")
		if err != nil {
			return nil, fmt.Errorf("git ls-files --others: %w", err)
		}
		for line := range strings.SplitSeq(untracked, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			files = append(files, StatusFile{Path: line, Status: Added})
		}
	} else {
		// For path-filtered mode, check each path individually as untracked.
		untrackedArgs := append([]string{"ls-files", "--others", "--exclude-standard", "--"}, paths...)
		untracked, err := r.Run(ctx, untrackedArgs...)
		if err == nil {
			for line := range strings.SplitSeq(untracked, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				files = append(files, StatusFile{Path: line, Status: Added})
			}
		}
	}

	return files, nil
}

// ListModifiedFilesFiltered returns working-tree modified files filtered by paths.
// When paths is empty it behaves identically to ListModifiedFiles.
func ListModifiedFilesFiltered(ctx context.Context, r Runner, paths []string) ([]StatusFile, error) {
	args := []string{"diff", "--name-status"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	nameStatus, err := r.Run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff --name-status: %w", err)
	}

	var files []StatusFile
	for line := range strings.SplitSeq(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sf := parseNameStatusLine(line)
		files = append(files, sf)
	}

	return files, nil
}

// ListStagedFilesFiltered returns staged files optionally filtered by paths.
// When paths is empty it behaves identically to ListStagedFiles.
func ListStagedFilesFiltered(ctx context.Context, r Runner, paths []string) ([]StatusFile, error) {
	args := []string{"diff", "--cached", "--name-status"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	nameStatus, err := r.Run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff --cached --name-status: %w", err)
	}

	var files []StatusFile
	for line := range strings.SplitSeq(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sf := parseNameStatusLine(line)
		files = append(files, sf)
	}

	return files, nil
}

// ListModifiedFiles returns working-tree modified files (for checkout/discard).
// It only includes files tracked by git (not untracked), using git diff --name-status.
func ListModifiedFiles(ctx context.Context, r Runner) ([]StatusFile, error) {
	nameStatus, err := r.Run(ctx, "diff", "--name-status")
	if err != nil {
		return nil, fmt.Errorf("git diff --name-status: %w", err)
	}

	var files []StatusFile
	for line := range strings.SplitSeq(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sf := parseNameStatusLine(line)
		files = append(files, sf)
	}

	return files, nil
}

// parseNameStatusLine parses a single line from git diff --name-status.
// Format: <status>\t<path> or R<score>\t<old>\t<new> for renames.
func parseNameStatusLine(line string) StatusFile {
	parts := strings.Split(line, "\t")
	if len(parts) < 2 {
		return StatusFile{Path: line, Status: Modified}
	}

	code := parts[0]
	switch {
	case strings.HasPrefix(code, "R"):
		// Rename: R100\told\tnew
		newPath := parts[len(parts)-1]
		return StatusFile{Path: newPath, Status: Renamed}
	case code == "A":
		return StatusFile{Path: parts[1], Status: Added}
	case code == "D":
		return StatusFile{Path: parts[1], Status: Deleted}
	default:
		return StatusFile{Path: parts[1], Status: Modified}
	}
}

// StageFiles runs git add for the given file paths.
func StageFiles(ctx context.Context, r Runner, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, paths...)
	_, err := r.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return nil
}

// StageHunk applies a single hunk patch to the index via `git apply --cached`.
// patchHeader is the unified diff file header (e.g. "diff --git a/f b/f\n--- a/f\n+++ b/f\n")
// and hunkBody is the "@@ ... @@\n..." hunk text. They are concatenated to form
// a minimal patch that git apply can process.
func StageHunk(ctx context.Context, r Runner, patchHeader, hunkBody string) error {
	patch := patchHeader + "\n" + hunkBody + "\n"
	_, err := r.RunWithStdin(ctx, patch, "apply", "--cached")
	if err != nil {
		return fmt.Errorf("git apply --cached: %w", err)
	}
	return nil
}

// ListStagedFiles returns the list of files currently in the index (staged for commit).
// It uses git diff --cached --name-status to enumerate staged changes.
func ListStagedFiles(ctx context.Context, r Runner) ([]StatusFile, error) {
	nameStatus, err := r.Run(ctx, "diff", "--cached", "--name-status")
	if err != nil {
		return nil, fmt.Errorf("git diff --cached --name-status: %w", err)
	}

	var files []StatusFile
	for line := range strings.SplitSeq(nameStatus, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sf := parseNameStatusLine(line)
		files = append(files, sf)
	}

	return files, nil
}

// UnstageFiles runs git reset HEAD -- for the given file paths to remove them from the index.
func UnstageFiles(ctx context.Context, r Runner, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"reset", "HEAD", "--"}, paths...)
	_, err := r.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git reset HEAD: %w", err)
	}
	return nil
}

// UnstageHunk removes a single hunk from the index via `git apply --cached --reverse`.
// patch is the full unified diff patch (header + hunk body) to reverse-apply.
func UnstageHunk(ctx context.Context, r Runner, patch string) error {
	_, err := r.RunWithStdin(ctx, patch, "apply", "--cached", "--reverse")
	if err != nil {
		return fmt.Errorf("git apply --cached --reverse: %w", err)
	}
	return nil
}

// DiscardHunk reverts a single hunk in the working tree via `git apply --reverse`.
// patch is the full unified diff patch (header + hunk body) to reverse-apply.
func DiscardHunk(ctx context.Context, r Runner, patch string) error {
	_, err := r.RunWithStdin(ctx, patch, "apply", "--reverse")
	if err != nil {
		return fmt.Errorf("git apply --reverse: %w", err)
	}
	return nil
}

// DiscardFiles runs git checkout -- for the given file paths to restore them to HEAD.
func DiscardFiles(ctx context.Context, r Runner, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"checkout", "--"}, paths...)
	_, err := r.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git checkout --: %w", err)
	}
	return nil
}
