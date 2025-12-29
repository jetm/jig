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
	for _, line := range strings.Split(nameStatus, "\n") {
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

	for _, line := range strings.Split(untracked, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, StatusFile{Path: line, Status: Added})
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
	for _, line := range strings.Split(nameStatus, "\n") {
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
